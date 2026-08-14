package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &scriptResource{}
	_ resource.ResourceWithConfigure   = &scriptResource{}
	_ resource.ResourceWithImportState = &scriptResource{}
)

func NewScriptResource() resource.Resource { return &scriptResource{} }

type scriptResource struct {
	client *client.Client
}

type scriptModel struct {
	ID               types.String `tfsdk:"id"`
	Realm            types.String `tfsdk:"realm"`
	Name             types.String `tfsdk:"name"`
	RemoteName       types.String `tfsdk:"remote_name"`
	Description      types.String `tfsdk:"description"`
	Context          types.String `tfsdk:"context"`
	Language         types.String `tfsdk:"language"`
	EvaluatorVersion types.String `tfsdk:"evaluator_version"`
	Source           types.String `tfsdk:"source"`
}

func (r *scriptResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_script"
}

func (r *scriptResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AM script. `source` is plaintext JavaScript (or Groovy); the provider base64-encodes on the wire. " +
			"`name` is the logical name; the provider prepends `resource_prefix` when talking to AIC.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "AM script UUID. Generated on create.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"realm": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Logical script name (without the provider prefix).",
			},
			"remote_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name stored in AIC, including `resource_prefix`.",
			},
			"description": schema.StringAttribute{Optional: true},
			"context": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Script context. `SCRIPTED_DECISION_NODE` is accepted and stored as " +
					"`AUTHENTICATION_TREE_DECISION_NODE`.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"language": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("JAVASCRIPT"),
			},
			"evaluator_version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("2.0"),
				MarkdownDescription: "Always send this. Omitting it on the API creates a legacy v1 engine script.",
			},
			"source": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Plaintext script body.",
			},
		},
	}
}

func (r *scriptResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *scriptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scriptModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := uuid.NewString()
	got, err := r.write(ctx, id, plan)
	if err != nil {
		resp.Diagnostics.AddError("Create script", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *scriptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scriptModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetScript(ctx, state.Realm.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read script", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, scriptToModel(got, state.Realm.ValueString(), r.client.Prefix, state.Name.ValueString()))...)
}

func (r *scriptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scriptModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan.ID.ValueString(), plan)
	if err != nil {
		resp.Diagnostics.AddError("Update script", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *scriptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scriptModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteScript(ctx, state.Realm.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete script", err.Error())
	}
}

func (r *scriptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// realm/id  or  realm/name=<name>
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use realm/<script-uuid> or realm/name=<script-name>.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("realm"), parts[0])...)
	if strings.HasPrefix(parts[1], "name=") {
		name := strings.TrimPrefix(parts[1], "name=")
		got, err := r.client.FindScriptByName(ctx, parts[0], name)
		if err != nil {
			resp.Diagnostics.AddError("Import script", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), got.ID)...)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *scriptResource) write(ctx context.Context, id string, plan scriptModel) (scriptModel, error) {
	remoteName := prefix.Apply(r.client.Prefix, plan.Name.ValueString())
	s := client.Script{
		ID:               id,
		Name:             remoteName,
		Description:      plan.Description.ValueString(),
		Context:          plan.Context.ValueString(),
		Language:         plan.Language.ValueString(),
		EvaluatorVersion: plan.EvaluatorVersion.ValueString(),
		Source:           plan.Source.ValueString(),
	}
	got, err := r.client.PutScript(ctx, plan.Realm.ValueString(), id, s)
	if err != nil {
		return scriptModel{}, err
	}
	return scriptToModel(got, plan.Realm.ValueString(), r.client.Prefix, plan.Name.ValueString()), nil
}

func scriptToModel(s *client.Script, realm, pfx, logicalName string) scriptModel {
	name := logicalName
	if name == "" {
		name = prefix.Strip(pfx, s.Name)
	}
	desc := types.StringNull()
	if s.Description != "" {
		desc = types.StringValue(s.Description)
	}
	return scriptModel{
		ID:               types.StringValue(s.ID),
		Realm:            types.StringValue(realm),
		Name:             types.StringValue(name),
		RemoteName:       types.StringValue(s.Name),
		Description:      desc,
		Context:          types.StringValue(s.Context),
		Language:         types.StringValue(s.Language),
		EvaluatorVersion: types.StringValue(s.EvaluatorVersion),
		Source:           types.StringValue(s.Source),
	}
}
