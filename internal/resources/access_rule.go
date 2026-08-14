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
	_ resource.Resource                = &accessRuleResource{}
	_ resource.ResourceWithConfigure   = &accessRuleResource{}
	_ resource.ResourceWithImportState = &accessRuleResource{}
)

func NewAccessRuleResource() resource.Resource { return &accessRuleResource{} }

type accessRuleResource struct{ client *client.Client }

type accessRuleModel struct {
	ID              types.String `tfsdk:"id"`
	Pattern         types.String `tfsdk:"pattern"`
	Roles           types.String `tfsdk:"roles"`
	Methods         types.String `tfsdk:"methods"`
	Actions         types.String `tfsdk:"actions"`
	CustomAuthz     types.String `tfsdk:"custom_authz"`
	ExcludePatterns types.String `tfsdk:"exclude_patterns"`
}

func (r *accessRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_rule"
}

func (r *accessRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "One grant in `/openidm/config/access` (`configs[]`). Tenant-global; no realm. " +
			"Rules have no server id — Terraform identifies the live entry by a SHA-256 of its canonical JSON " +
			"(the same digest `aic access list` prints). Other rules in the document are left untouched. " +
			"`resource_prefix` is not applied: there is no name to prefix. Creating a rule whose hash already " +
			"exists is refused (import it). Destroy removes only the first matching copy. " +
			"`configs` is a disjunction, so an appended grant cannot revoke access; editing or deleting a " +
			"grant can.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Canonical SHA-256 of the rule. Changes when any field changes.",
			},
			"pattern": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Route glob, e.g. `endpoint/foo` or `*`.",
			},
			"roles": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Comma-separated IDM role ids (`internal/role/…`) or `*`. A role *name* never matches.",
			},
			"methods": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Comma-separated methods (`read`, `query`, `create`, …) or `*`.",
			},
			"actions": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional. Omit the argument to leave the key off the rule (six sandbox rules). Set `\"\"` to store an empty string (three sandbox rules). These are different; do not send `*` just to fill it in.",
			},
			"custom_authz": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JS expression that can only deny (e.g. `ownDataOnly()`).",
			},
			"exclude_patterns": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Comma-separated globs subtracted from `pattern`. Semantics inferred, not probed.",
			},
		},
	}
}

func (r *accessRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accessRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule := modelToAccessRule(plan)
	hash, err := mutateHashed(ctx, r.client.MutateAccess, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, h, err := client.AppendAccessRule(doc, rule)
		return next, h, client.RuleConfirm{Hash: h, Count: 1}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Create access rule", err.Error())
		return
	}
	got := accessRuleToModel(rule, hash)
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func (r *accessRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	doc, err := r.client.GetAccess(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read access rule", err.Error())
		return
	}
	got, err := readAccessRule(doc, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read access rule", err.Error())
		return
	}
	if got == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

//nolint:dupl // Parallel of authentication_mapping.Update: same hash-keyed RMW, different document and typed rule.
func (r *accessRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior accessRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule := modelToAccessRule(plan)
	hash, err := mutateHashed(ctx, r.client.MutateAccess, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, h, err := client.ReplaceAccessRule(doc, prior.ID.ValueString(), rule)
		return next, h, client.RuleConfirm{Hash: h, Count: 1}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Update access rule", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, accessRuleToModel(rule, hash))...)
}

func (r *accessRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hash := state.ID.ValueString()
	_, err := mutateHashed(ctx, r.client.MutateAccess, func(doc map[string]any) (map[string]any, string, client.RuleConfirm, error) {
		next, remaining, err := client.RemoveAccessRule(doc, hash)
		return next, hash, client.RuleConfirm{Hash: hash, Count: remaining}, err
	})
	if err != nil {
		resp.Diagnostics.AddError("Delete access rule", err.Error())
	}
}

func (r *accessRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	id = strings.TrimPrefix(id, "access/")
	if id == "" {
		resp.Diagnostics.AddError("Invalid import id", "Use the rule hash (full SHA-256 or unique 8+ character prefix from `aic access list`).")
		return
	}
	doc, err := r.client.GetAccess(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Import access rule", err.Error())
		return
	}
	got, err := readAccessRule(doc, id)
	if err != nil {
		resp.Diagnostics.AddError("Import access rule", err.Error())
		return
	}
	if got == nil {
		resp.Diagnostics.AddError("Import access rule", fmt.Sprintf("no access rule has digest %q", id))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, got)...)
}

func readAccessRule(doc map[string]any, hash string) (*accessRuleModel, error) {
	rules, err := client.AccessRules(doc)
	if err != nil {
		return nil, err
	}
	idxs, err := client.FindRuleHashes(rules, hash)
	if err != nil {
		return nil, err
	}
	if len(idxs) == 0 {
		return nil, nil
	}
	decoded, err := client.DecodeAccessRule(rules[idxs[0]])
	if err != nil {
		return nil, err
	}
	full, err := client.Digest(rules[idxs[0]])
	if err != nil {
		return nil, err
	}
	m := accessRuleToModel(*decoded, full)
	return &m, nil
}

func modelToAccessRule(plan accessRuleModel) client.AccessRule {
	return client.AccessRule{
		Pattern:         plan.Pattern.ValueString(),
		Roles:           plan.Roles.ValueString(),
		Methods:         plan.Methods.ValueString(),
		Actions:         optionalString(plan.Actions),
		CustomAuthz:     optionalString(plan.CustomAuthz),
		ExcludePatterns: optionalString(plan.ExcludePatterns),
	}
}

func accessRuleToModel(r client.AccessRule, hash string) accessRuleModel {
	return accessRuleModel{
		ID:              types.StringValue(hash),
		Pattern:         types.StringValue(r.Pattern),
		Roles:           types.StringValue(r.Roles),
		Methods:         types.StringValue(r.Methods),
		Actions:         optionalStringAttr(r.Actions),
		CustomAuthz:     optionalStringAttr(r.CustomAuthz),
		ExcludePatterns: optionalStringAttr(r.ExcludePatterns),
	}
}

func optionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func optionalStringAttr(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}
