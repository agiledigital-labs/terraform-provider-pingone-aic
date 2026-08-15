package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &esvVariableResource{}
	_ resource.ResourceWithConfigure   = &esvVariableResource{}
	_ resource.ResourceWithImportState = &esvVariableResource{}
)

func NewESVVariableResource() resource.Resource { return &esvVariableResource{} }

type esvVariableResource struct {
	client *client.Client
}

type esvVariableModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	RemoteName     types.String `tfsdk:"remote_name"`
	Description    types.String `tfsdk:"description"`
	ExpressionType types.String `tfsdk:"expression_type"`
	Value          types.String `tfsdk:"value"`
	Loaded         types.Bool   `tfsdk:"loaded"`
}

func (r *esvVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esv_variable"
}

func (r *esvVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AIC environment variable (`/environment/variables`). Tenant-global; no realm. " +
			"`name` is the logical ESV id (`esv-…`). The provider inserts a sanitised `resource_prefix` " +
			"after `esv-` (so `Terraform_` + `esv-test11` becomes `esv-terraform-test11`) because AIC " +
			"rejects ids that do not match `^esv-[a-z0-9_-]{1,124}$`.\n\n" +
			"`loaded` is computed. A create or update leaves the variable pending until someone restarts " +
			"the tenant; this resource never triggers that restart.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `remote_name` (the AIC ESV id).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Logical ESV id, including the `esv-` marker, without the provider prefix.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"remote_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Id stored in AIC, including the sanitised prefix.",
			},
			"description": schema.StringAttribute{Optional: true},
			"expression_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("string"),
				MarkdownDescription: "AIC expressionType. Changing it RequiresReplace (in-place type changes are rejected).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Plaintext value. The provider base64-encodes on the wire. Arrays and numbers are the decoded string form (JSON text, decimal, `true`/`false`).",
			},
			"loaded": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the running tenant has picked this value up. False after write until a tenant restart.",
			},
		},
	}
}

func (r *esvVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("%T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *esvVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan esvVariableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, esvVariableModel{})
	if err != nil {
		resp.Diagnostics.AddError("Create esv variable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *esvVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state esvVariableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := applyESVRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())
	got, err := r.client.GetVariable(ctx, remote)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read esv variable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, variableToModel(got, state.Name.ValueString(), r.client.Prefix))...)
}

func (r *esvVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior esvVariableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, prior)
	if err != nil {
		resp.Diagnostics.AddError("Update esv variable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *esvVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state esvVariableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteVariable(ctx, applyESVRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("Delete esv variable", err.Error())
	}
}

func (r *esvVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the AIC ESV id (esv-…).")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.StripESV(r.client.Prefix, id))...)
}

func (r *esvVariableResource) write(ctx context.Context, plan, prior esvVariableModel) (esvVariableModel, error) {
	remote := applyESVRemote(prior.ID, prior.RemoteName, r.client.Prefix, plan.Name.ValueString())
	got, err := r.client.PutVariable(ctx, remote, client.Variable{
		ID:             remote,
		Description:    plan.Description.ValueString(),
		ExpressionType: plan.ExpressionType.ValueString(),
		Value:          plan.Value.ValueString(),
	})
	if err != nil {
		return esvVariableModel{}, err
	}
	return variableToModel(got, plan.Name.ValueString(), r.client.Prefix), nil
}

func variableToModel(v *client.Variable, logical, pfx string) esvVariableModel {
	name := logical
	if name == "" {
		name = prefix.StripESV(pfx, v.ID)
	}
	desc := types.StringNull()
	if v.Description != "" {
		desc = types.StringValue(v.Description)
	}
	return esvVariableModel{
		ID:             types.StringValue(v.ID),
		Name:           types.StringValue(name),
		RemoteName:     types.StringValue(v.ID),
		Description:    desc,
		ExpressionType: types.StringValue(v.ExpressionType),
		Value:          types.StringValue(v.Value),
		Loaded:         types.BoolValue(v.Loaded),
	}
}
