package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &internalRoleResource{}
	_ resource.ResourceWithConfigure   = &internalRoleResource{}
	_ resource.ResourceWithImportState = &internalRoleResource{}
)

func NewInternalRoleResource() resource.Resource { return &internalRoleResource{} }

type internalRoleResource struct{ client *client.Client }

type internalRoleModel struct {
	ID          types.String         `tfsdk:"id"`
	Name        types.String         `tfsdk:"name"`
	RemoteName  types.String         `tfsdk:"remote_name"`
	DisplayName types.String         `tfsdk:"display_name"`
	Description types.String         `tfsdk:"description"`
	Condition   types.String         `tfsdk:"condition"`
	Privileges  []rolePrivilegeModel `tfsdk:"privilege"`
}

type rolePrivilegeModel struct {
	Name        types.String    `tfsdk:"name"`
	Path        types.String    `tfsdk:"path"`
	Actions     types.Set       `tfsdk:"actions"`
	Permissions types.Set       `tfsdk:"permissions"`
	Filter      types.String    `tfsdk:"filter"`
	AccessFlags []roleFlagModel `tfsdk:"access_flag"`
}

type roleFlagModel struct {
	Attribute types.String `tfsdk:"attribute"`
	ReadOnly  types.Bool   `tfsdk:"read_only"`
}

func (r *internalRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_internal_role"
}

func (r *internalRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An IDM internal role (`/openidm/internal/role/{id}`). Tenant-global; no realm. " +
			"`name` is the `_id` you choose — the string `config/access` and `config/authentication` must " +
			"reference as `internal/role/{id}`. The provider prepends `resource_prefix` on the wire so apply " +
			"creates a copy. PUT is a destructive replace: omitting `privilege` blocks empties privileges. " +
			"Updates send `If-Match` with the live `_rev`. `temporalConstraints` is stripped on write " +
			"(IDM rejects it, even as `[]`). Privilege order is not significant.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "Logical `_id` without the prefix. Access rules must use this id, never `display_name`."},
			"remote_name": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"display_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The role `name` field when it differs from `_id` (console-created UUID roles). Omit to set `name` equal to the remote `_id`.",
			},
			"description": schema.StringAttribute{Optional: true},
			"condition":   schema.StringAttribute{Optional: true, MarkdownDescription: "Optional query filter. Null on every live sandbox role."},
		},
		Blocks: map[string]schema.Block{
			"privilege": schema.SetNestedBlock{
				MarkdownDescription: "One privilege. `name`, `path`, `actions`, `permissions`, and at least one `access_flag` are mandatory. `actions` may be empty; `access_flag` may not. A non-VIEW permission requires at least one `read_only = false` flag.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Required: true},
						"path":        schema.StringAttribute{Required: true, MarkdownDescription: "Usually `managed/<type>`. Not prefixed."},
						"actions":     schema.SetAttribute{Required: true, ElementType: types.StringType},
						"permissions": schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Typically VIEW, CREATE, UPDATE, DELETE, ACTION. Unknown values are sent as-is — IDM publishes no enum."},
						"filter":      schema.StringAttribute{Optional: true},
					},
					Blocks: map[string]schema.Block{
						"access_flag": schema.SetNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"attribute": schema.StringAttribute{Required: true},
									"read_only": schema.BoolAttribute{Required: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *internalRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *internalRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan internalRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, internalRoleModel{})
	if err != nil {
		resp.Diagnostics.AddError("Create internal role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *internalRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state internalRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := applyRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())
	got, err := r.client.GetRole(ctx, remote)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read internal role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, roleToModel(got, state.Name.ValueString(), remote))...)
}

func (r *internalRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior internalRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.write(ctx, plan, prior)
	if err != nil {
		resp.Diagnostics.AddError("Update internal role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *internalRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state internalRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote := applyRemote(state.ID, state.RemoteName, r.client.Prefix, state.Name.ValueString())
	if err := r.client.DeleteRole(ctx, remote); err != nil {
		resp.Diagnostics.AddError("Delete internal role", err.Error())
	}
}

func (r *internalRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	id = strings.TrimPrefix(id, "internal/role/")
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the role `_id` (with or without the internal/role/ prefix).")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote_name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), prefix.Strip(r.client.Prefix, id))...)
}

func (r *internalRoleResource) write(ctx context.Context, plan, prior internalRoleModel) (internalRoleModel, error) {
	remote := applyRemote(prior.ID, prior.RemoteName, r.client.Prefix, plan.Name.ValueString())
	var rev string
	if persistedRemote(prior.ID, prior.RemoteName) == "" {
		if _, err := r.client.GetRole(ctx, remote); err == nil {
			return internalRoleModel{}, fmt.Errorf("internal role %q already exists", remote)
		} else if !client.IsNotFound(err) {
			return internalRoleModel{}, err
		}
	} else {
		live, err := r.client.GetRole(ctx, remote)
		if err != nil {
			return internalRoleModel{}, err
		}
		rev = live.Rev
	}
	role, err := modelToRole(ctx, plan, remote)
	if err != nil {
		return internalRoleModel{}, err
	}
	got, err := r.client.PutRole(ctx, remote, rev, role)
	if err != nil {
		return internalRoleModel{}, err
	}
	return roleToModel(got, plan.Name.ValueString(), remote), nil
}

func modelToRole(ctx context.Context, plan internalRoleModel, remote string) (client.Role, error) {
	display := plan.DisplayName.ValueString()
	if display == "" {
		display = remote
	}
	role := client.Role{
		Name:        display,
		Description: plan.Description.ValueString(),
		Condition:   plan.Condition.ValueString(),
	}
	for _, p := range plan.Privileges {
		actions, err := setStrings(ctx, p.Actions)
		if err != nil {
			return client.Role{}, fmt.Errorf("privilege %q actions: %w", p.Name.ValueString(), err)
		}
		permissions, err := setStrings(ctx, p.Permissions)
		if err != nil {
			return client.Role{}, fmt.Errorf("privilege %q permissions: %w", p.Name.ValueString(), err)
		}
		priv := client.Privilege{
			Name:        p.Name.ValueString(),
			Path:        p.Path.ValueString(),
			Actions:     actions,
			Permissions: permissions,
			Filter:      p.Filter.ValueString(),
		}
		for _, f := range p.AccessFlags {
			priv.AccessFlags = append(priv.AccessFlags, client.AccessFlag{
				Attribute: f.Attribute.ValueString(),
				ReadOnly:  f.ReadOnly.ValueBool(),
			})
		}
		role.Privileges = append(role.Privileges, priv)
	}
	if _, err := client.EncodeRole(role); err != nil {
		return client.Role{}, err
	}
	return role, nil
}

func roleToModel(r *client.Role, logical, remote string) internalRoleModel {
	name := logical
	if name == "" {
		name = r.ID
	}
	display := types.StringNull()
	if r.Name != "" && r.Name != r.ID {
		display = types.StringValue(r.Name)
	}
	m := internalRoleModel{
		ID:          types.StringValue(r.ID),
		Name:        types.StringValue(name),
		RemoteName:  types.StringValue(remote),
		DisplayName: display,
		Description: stringOrNull(r.Description),
		Condition:   stringOrNull(r.Condition),
	}
	for _, p := range r.Privileges {
		pm := rolePrivilegeModel{
			Name:        types.StringValue(p.Name),
			Path:        types.StringValue(p.Path),
			Actions:     stringSetValue(p.Actions),
			Permissions: stringSetValue(p.Permissions),
			Filter:      stringOrNull(p.Filter),
		}
		for _, f := range p.AccessFlags {
			pm.AccessFlags = append(pm.AccessFlags, roleFlagModel{
				Attribute: types.StringValue(f.Attribute),
				ReadOnly:  types.BoolValue(f.ReadOnly),
			})
		}
		m.Privileges = append(m.Privileges, pm)
	}
	return m
}

// stringSetValue cannot fail: every element is constructed as types.String
// immediately below, matching the declared element type, and `actions` /
// `permissions` are Required so an empty set is the correct zero — never null.
func stringSetValue(items []string) types.Set {
	els := make([]attr.Value, len(items))
	for i, s := range items {
		els[i] = types.StringValue(s)
	}
	return types.SetValueMust(types.StringType, els)
}

func setStrings(ctx context.Context, v types.Set) ([]string, error) {
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	var out []string
	if diags := v.ElementsAs(ctx, &out, false); diags.HasError() {
		d := diags.Errors()[0]
		return nil, fmt.Errorf("%s: %s", d.Summary(), d.Detail())
	}
	return out, nil
}
