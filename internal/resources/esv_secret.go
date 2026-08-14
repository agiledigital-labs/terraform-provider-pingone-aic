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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &esvSecretResource{}
	_ resource.ResourceWithConfigure   = &esvSecretResource{}
	_ resource.ResourceWithImportState = &esvSecretResource{}
)

func NewESVSecretResource() resource.Resource { return &esvSecretResource{} }

type esvSecretResource struct {
	client *client.Client
}

type esvSecretModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	RemoteName        types.String `tfsdk:"remote_name"`
	Description       types.String `tfsdk:"description"`
	Encoding          types.String `tfsdk:"encoding"`
	UseInPlaceholders types.Bool   `tfsdk:"use_in_placeholders"`
	Value             types.String `tfsdk:"value"`
	Loaded            types.Bool   `tfsdk:"loaded"`
	ActiveVersion     types.String `tfsdk:"active_version"`
	LoadedVersion     types.String `tfsdk:"loaded_version"`
}

func (r *esvSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esv_secret"
}

func (r *esvSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An AIC environment secret (`/environment/secrets`). Tenant-global; no realm. " +
			"The value is write-only — AIC never returns it, and generate cannot copy one. " +
			"`encoding` and `use_in_placeholders` are immutable after create. " +
			"A value change creates a new secret version rather than replacing the secret. " +
			"This resource never restarts the tenant; `loaded` stays false until someone does, " +
			"when `use_in_placeholders` is true.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Logical ESV id (`esv-…`) without the provider prefix.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"remote_name": schema.StringAttribute{Computed: true},
			"description": schema.StringAttribute{Optional: true},
			"encoding": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("generic"),
				MarkdownDescription: "generic | pem | base64hmac | base64aes. Immutable after create.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"use_in_placeholders": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Required by AIC on create. true means `&{esv.…}` references work and a restart is needed before `loaded` becomes true.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Plaintext secret. Required on create. Changing it posts a new version. Never read back from AIC.",
			},
			"loaded":         schema.BoolAttribute{Computed: true},
			"active_version": schema.StringAttribute{Computed: true},
			"loaded_version": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *esvSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *esvSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan esvSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Value.IsNull() || plan.Value.ValueString() == "" {
		resp.Diagnostics.AddError("Create esv secret", "value is required on create (AIC never returns secret values).")
		return
	}
	remote := prefix.ApplyESV(r.client.Prefix, plan.Name.ValueString())
	got, err := r.client.CreateSecret(ctx, remote, client.Secret{
		ID:                remote,
		Description:       plan.Description.ValueString(),
		Encoding:          plan.Encoding.ValueString(),
		UseInPlaceholders: plan.UseInPlaceholders.ValueBool(),
	}, plan.Value.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Create esv secret", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, secretToModel(got, plan.Name.ValueString(), plan.Value.ValueString(), r.client.Prefix))...)
}

func (r *esvSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state esvSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetSecret(ctx, secretRemoteName(state, r.client.Prefix))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read esv secret", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, secretToModel(got, state.Name.ValueString(), state.Value.ValueString(), r.client.Prefix))...)
}

func (r *esvSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior esvSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := secretRemoteName(prior, r.client.Prefix)
	if plan.Description.ValueString() != prior.Description.ValueString() {
		if err := r.client.SetSecretDescription(ctx, remote, plan.Description.ValueString()); err != nil {
			resp.Diagnostics.AddError("Update esv secret description", err.Error())
			return
		}
	}
	value := plan.Value.ValueString()
	if !plan.Value.IsNull() && value != "" && value != prior.Value.ValueString() {
		if _, err := r.client.CreateSecretVersion(ctx, remote, value); err != nil {
			resp.Diagnostics.AddError("Update esv secret value", err.Error())
			return
		}
	}
	got, err := r.client.GetSecret(ctx, remote)
	if err != nil {
		resp.Diagnostics.AddError("Read esv secret after update", err.Error())
		return
	}
	if value == "" {
		value = prior.Value.ValueString()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, secretToModel(got, plan.Name.ValueString(), value, r.client.Prefix))...)
}

func (r *esvSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state esvSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSecret(ctx, secretRemoteName(state, r.client.Prefix)); err != nil {
		resp.Diagnostics.AddError("Delete esv secret", err.Error())
	}
}

func (r *esvSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the AIC ESV id (esv-…).")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.StripESV(r.client.Prefix, id))...)
}

func secretRemoteName(state esvSecretModel, pfx string) string {
	for _, v := range []types.String{state.ID, state.RemoteName} {
		if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			return v.ValueString()
		}
	}
	return prefix.ApplyESV(pfx, state.Name.ValueString())
}

func secretToModel(s *client.Secret, logical, value, pfx string) esvSecretModel {
	name := logical
	if name == "" {
		name = prefix.StripESV(pfx, s.ID)
	}
	desc := types.StringNull()
	if s.Description != "" {
		desc = types.StringValue(s.Description)
	}
	val := types.StringNull()
	if value != "" {
		val = types.StringValue(value)
	}
	return esvSecretModel{
		ID:                types.StringValue(s.ID),
		Name:              types.StringValue(name),
		RemoteName:        types.StringValue(s.ID),
		Description:       desc,
		Encoding:          types.StringValue(s.Encoding),
		UseInPlaceholders: types.BoolValue(s.UseInPlaceholders),
		Value:             val,
		Loaded:            types.BoolValue(s.Loaded),
		ActiveVersion:     types.StringValue(s.ActiveVersion),
		LoadedVersion:     types.StringValue(s.LoadedVersion),
	}
}
