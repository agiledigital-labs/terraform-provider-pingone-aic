package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &authMappingResource{}
	_ resource.ResourceWithConfigure   = &authMappingResource{}
	_ resource.ResourceWithImportState = &authMappingResource{}
)

func NewAuthenticationMappingResource() resource.Resource { return &authMappingResource{} }

type authMappingResource struct{ client *client.Client }

type authMappingModel struct {
	ID                        types.String `tfsdk:"id"`
	Subject                   types.String `tfsdk:"subject"`
	LocalUser                 types.String `tfsdk:"local_user"`
	Roles                     types.List   `tfsdk:"roles"`
	UserRoles                 types.String `tfsdk:"user_roles"`
	ExecuteAugmentationScript types.Bool   `tfsdk:"execute_augmentation_script"`
}

func (r *authMappingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authentication_mapping"
}

func (r *authMappingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One `rsFilter.staticUserMapping[]` entry in `/openidm/config/authentication`. " +
			"Tenant-global; no realm. Mappings have no server id — Terraform identifies the live entry by a " +
			"SHA-256 of its canonical JSON. The rest of `rsFilter` (scopes, `subjectMapping`, " +
			"`anonymousUserMapping`, client credentials, …) is left untouched. `resource_prefix` is not " +
			"applied. Creating a mapping whose hash already exists is refused (import it). `roles` here is " +
			"an **array**, unlike `pingoneaic_access_rule.roles` which is a comma-separated string.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Canonical SHA-256 of the mapping. Changes when any field changes.",
			},
			"subject": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Token subject this mapping matches, e.g. an OAuth2 client id.",
			},
			"local_user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IDM identity path, usually `internal/user/…`. A synthetic `internal/role/…` is legal.",
			},
			"roles": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDM role ids granted to this subject. Omit the argument to leave the key off (one live mapping does).",
			},
			"user_roles": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Relationship pointer some mappings store, e.g. `authzRoles/*`.",
			},
			"execute_augmentation_script": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When set, written as `executeAugmentationScript`. Omit to leave the key off.",
			},
		},
	}
}

func (r *authMappingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *authMappingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authMappingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mapping := modelToAuthMapping(plan)
	hash, err := mutateHashed(ctx, r.client.MutateAuthentication, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, h, err := client.AppendAuthMapping(doc, mapping)
		return next, h, client.RuleConfirm{Hash: h, Count: 1}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Create authentication mapping", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, authMappingToModel(mapping, hash))...)
}

func (r *authMappingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state authMappingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	doc, err := r.client.GetAuthentication(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read authentication mapping", err.Error())
		return
	}
	got, err := readAuthMapping(doc, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read authentication mapping", err.Error())
		return
	}
	if got == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

//nolint:dupl // Parallel of access_rule.Update: same hash-keyed RMW, different document and typed mapping.
func (r *authMappingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior authMappingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mapping := modelToAuthMapping(plan)
	hash, err := mutateHashed(ctx, r.client.MutateAuthentication, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, h, err := client.ReplaceAuthMapping(doc, prior.ID.ValueString(), mapping)
		return next, h, client.RuleConfirm{Hash: h, Count: 1}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Update authentication mapping", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, authMappingToModel(mapping, hash))...)
}

func (r *authMappingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state authMappingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hash := state.ID.ValueString()
	_, err := mutateHashed(ctx, r.client.MutateAuthentication, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, remaining, err := client.RemoveAuthMapping(doc, hash)
		return next, hash, client.RuleConfirm{Hash: hash, Count: remaining}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Delete authentication mapping", err.Error())
	}
}

func (r *authMappingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	id = strings.TrimPrefix(id, "authentication/")
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the mapping hash (full SHA-256 or unique 8+ character prefix).")
		return
	}
	doc, err := r.client.GetAuthentication(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Import authentication mapping", err.Error())
		return
	}
	got, err := readAuthMapping(doc, id)
	if err != nil {
		resp.Diagnostics.AddError("Import authentication mapping", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Import authentication mapping", fmt.Sprintf("no authentication mapping has digest %q", id))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func readAuthMapping(doc map[string]any, hash string) (*authMappingModel, error) {
	maps, err := client.AuthMappings(doc)
	if err != nil {
		return nil, err
	}
	idxs, err := client.FindRuleHashes(maps, hash)
	if err != nil {
		return nil, err
	}
	if len(idxs) == 0 {
		return nil, nil
	}
	decoded, err := client.DecodeAuthMapping(maps[idxs[0]])
	if err != nil {
		return nil, err
	}
	full, err := client.Digest(maps[idxs[0]])
	if err != nil {
		return nil, err
	}
	m := authMappingToModel(*decoded, full)
	return &m, nil
}

func modelToAuthMapping(plan authMappingModel) client.AuthMapping {
	m := client.AuthMapping{
		Subject:   plan.Subject.ValueString(),
		LocalUser: plan.LocalUser.ValueString(),
		Roles:     listStrings(plan.Roles),
		UserRoles: plan.UserRoles.ValueString(),
	}
	if !plan.ExecuteAugmentationScript.IsNull() && !plan.ExecuteAugmentationScript.IsUnknown() {
		v := plan.ExecuteAugmentationScript.ValueBool()
		m.ExecuteAugmentationScript = &v
	}
	return m
}

func authMappingToModel(m client.AuthMapping, hash string) authMappingModel {
	return authMappingModel{
		ID:                        types.StringValue(hash),
		Subject:                   types.StringValue(m.Subject),
		LocalUser:                 types.StringValue(m.LocalUser),
		Roles:                     stringListOrNull(m.Roles),
		UserRoles:                 stringOrNull(m.UserRoles),
		ExecuteAugmentationScript: boolOrNull(m.ExecuteAugmentationScript),
	}
}
