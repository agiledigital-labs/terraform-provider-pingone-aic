package resources

import (
	"context"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewAccessRuleResource() resource.Resource {
	return &hashedRuleResource[accessRuleModel, client.AccessRule]{spec: accessRuleSpec()}
}

type accessRuleModel struct {
	ID              types.String `tfsdk:"id"`
	Pattern         types.String `tfsdk:"pattern"`
	Roles           types.String `tfsdk:"roles"`
	Methods         types.String `tfsdk:"methods"`
	Actions         types.String `tfsdk:"actions"`
	CustomAuthz     types.String `tfsdk:"custom_authz"`
	ExcludePatterns types.String `tfsdk:"exclude_patterns"`
}

func accessRuleSpec() hashedRuleSpec[accessRuleModel, client.AccessRule] {
	return hashedRuleSpec[accessRuleModel, client.AccessRule]{
		typeSuffix:   "access_rule",
		label:        "access rule",
		importPrefix: "access/",
		importHelp:   "Use the rule hash (full SHA-256 or unique 8+ character prefix from `aic access list`).",
		schema: schema.Schema{
			MarkdownDescription: "One grant in `/openidm/config/access` (`configs[]`). Tenant-global; no realm. " +
				"Rules have no server id — Terraform identifies the live entry by a SHA-256 of its canonical JSON " +
				"(the same digest `aic access list` prints). Other rules in the document are left untouched. " +
				"`resource_prefix` is not applied: there is no name to prefix. Creating a rule whose hash already " +
				"exists is refused (import it). Destroy removes only the first matching copy. " +
				"`configs` is a disjunction, so an appended grant cannot revoke access; editing or deleting a " +
				"grant can.",
			Attributes: map[string]schema.Attribute{
				"id":               schema.StringAttribute{Computed: true, MarkdownDescription: "Canonical SHA-256 of the rule. Changes when any field changes."},
				"pattern":          schema.StringAttribute{Required: true, MarkdownDescription: "Route glob, e.g. `endpoint/foo` or `*`."},
				"roles":            schema.StringAttribute{Required: true, MarkdownDescription: "Comma-separated IDM role ids (`internal/role/…`) or `*`. A role *name* never matches."},
				"methods":          schema.StringAttribute{Required: true, MarkdownDescription: "Comma-separated methods (`read`, `query`, `create`, …) or `*`."},
				"actions":          schema.StringAttribute{Optional: true, MarkdownDescription: "Optional. Omit the argument to leave the key off the rule (six sandbox rules). Set `\"\"` to store an empty string (three sandbox rules). These are different; do not send `*` just to fill it in."},
				"custom_authz":     schema.StringAttribute{Optional: true, MarkdownDescription: "JS expression that can only deny (e.g. `ownDataOnly()`)."},
				"exclude_patterns": schema.StringAttribute{Optional: true, MarkdownDescription: "Comma-separated globs subtracted from `pattern`. Semantics inferred, not probed."},
			},
		},
		get: func(ctx context.Context, c *client.Client) (map[string]any, error) {
			return c.GetAccess(ctx)
		},
		mutate: func(ctx context.Context, c *client.Client, mutation func(map[string]any) (map[string]any, client.RuleConfirm, error)) error {
			return c.MutateAccess(ctx, mutation)
		},
		objects:     client.AccessRules,
		decode:      client.DecodeAccessRule,
		append:      client.AppendAccessRule,
		replace:     client.ReplaceAccessRule,
		remove:      client.RemoveAccessRule,
		modelToRule: modelToAccessRule,
		ruleToModel: accessRuleToModel,
	}
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
