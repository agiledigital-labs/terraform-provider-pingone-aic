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
	_ resource.Resource                = &idmEndpointResource{}
	_ resource.ResourceWithConfigure   = &idmEndpointResource{}
	_ resource.ResourceWithImportState = &idmEndpointResource{}
)

func NewIDMEndpointResource() resource.Resource { return &idmEndpointResource{} }

type idmEndpointResource struct{ client *client.Client }

type idmEndpointModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	RemoteName    types.String `tfsdk:"remote_name"`
	Type          types.String `tfsdk:"type"`
	Source        types.String `tfsdk:"source"`
	File          types.String `tfsdk:"file"`
	Description   types.String `tfsdk:"description"`
	Context       types.String `tfsdk:"context"`
	GlobalsObject types.String `tfsdk:"globals_object"`
	AllowedRoles  types.List   `tfsdk:"allowed_roles"`
}

func (r *idmEndpointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_idm_endpoint"
}

func (r *idmEndpointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IDM custom endpoint (`/openidm/config/endpoint/{name}`). Tenant-global; no realm. " +
			"`source` is plaintext JavaScript (not base64). File-backed product endpoints set `file` instead. " +
			"`name` is the logical id; the provider prepends `resource_prefix` on the wire.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Logical endpoint name without the `endpoint/` prefix."},
			"remote_name": schema.StringAttribute{Computed: true},
			"type":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("text/javascript")},
			"source":      schema.StringAttribute{Optional: true, MarkdownDescription: "Plaintext script body. Inline a string or, preferably, `source = file(\"${path.module}/endpoints/foo.js\")`."},
			"file":        schema.StringAttribute{Optional: true, MarkdownDescription: "IDM product file path for file-backed endpoints (not a local Terraform path)."},
			"description": schema.StringAttribute{Optional: true},
			"context":     schema.StringAttribute{Optional: true},
			"globals_object": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Raw `globalsObject` string some tenant endpoints store.",
			},
			"allowed_roles": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "From `globals.endpointConfig.allowedRoles`.",
			},
		},
	}
}

func (r *idmEndpointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *idmEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan idmEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, idmEndpointModel{})
	if err != nil {
		resp.Diagnostics.AddError("Create idm endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *idmEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state idmEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetEndpoint(ctx, applyRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString()))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read idm endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, endpointToModel(got, state.Name.ValueString(), r.client.Prefix))...)
}

func (r *idmEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior idmEndpointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, prior)
	if err != nil {
		resp.Diagnostics.AddError("Update idm endpoint", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *idmEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state idmEndpointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEndpoint(ctx, applyRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())); err != nil {
		resp.Diagnostics.AddError("Delete idm endpoint", err.Error())
	}
}

func (r *idmEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.TrimPrefix(strings.TrimSpace(req.ID), "endpoint/")
	if name == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the endpoint name (with or without the endpoint/ prefix).")
		return
	}
	remote := prefix.Apply(r.client.Prefix, name)
	if strings.HasPrefix(req.ID, "endpoint/") || strings.HasPrefix(req.ID, r.client.Prefix) {
		remote = strings.TrimPrefix(req.ID, "endpoint/")
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), remote)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), remote)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, remote))...)
}

func (r *idmEndpointResource) write(ctx context.Context, plan, prior idmEndpointModel) (idmEndpointModel, error) {
	remote := applyRemote(prior.ID, prior.RemoteName, r.client.Prefix, plan.Name.ValueString())
	got, err := r.client.PutEndpoint(ctx, remote, modelToEndpoint(plan))
	if err != nil {
		return idmEndpointModel{}, err
	}
	return endpointToModel(got, plan.Name.ValueString(), r.client.Prefix), nil
}

func modelToEndpoint(plan idmEndpointModel) client.Endpoint {
	return client.Endpoint{
		Name:          plan.Name.ValueString(),
		Type:          plan.Type.ValueString(),
		Source:        plan.Source.ValueString(),
		File:          plan.File.ValueString(),
		Description:   plan.Description.ValueString(),
		Context:       plan.Context.ValueString(),
		GlobalsObject: plan.GlobalsObject.ValueString(),
		AllowedRoles:  listStrings(plan.AllowedRoles),
	}
}

func endpointToModel(e *client.Endpoint, logical, pfx string) idmEndpointModel {
	name := logical
	if name == "" {
		name = prefix.Strip(pfx, e.Name)
	}
	return idmEndpointModel{
		ID:            types.StringValue(e.Name),
		Name:          types.StringValue(name),
		RemoteName:    types.StringValue(e.Name),
		Type:          types.StringValue(e.Type),
		Source:        stringOrNull(e.Source),
		File:          stringOrNull(e.File),
		Description:   stringOrNull(e.Description),
		Context:       stringOrNull(e.Context),
		GlobalsObject: stringOrNull(e.GlobalsObject),
		AllowedRoles:  stringListOrNull(e.AllowedRoles),
	}
}
