package resources

import (
	"context"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewAuthenticationMappingResource() resource.Resource {
	return &hashedRuleResource[authMappingModel, client.AuthMapping]{spec: authenticationMappingSpec()}
}

type authMappingModel struct {
	ID                        types.String `tfsdk:"id"`
	Subject                   types.String `tfsdk:"subject"`
	LocalUser                 types.String `tfsdk:"local_user"`
	Roles                     types.List   `tfsdk:"roles"`
	UserRoles                 types.String `tfsdk:"user_roles"`
	ExecuteAugmentationScript types.Bool   `tfsdk:"execute_augmentation_script"`
}

func authenticationMappingSpec() hashedRuleSpec[authMappingModel, client.AuthMapping] {
	return hashedRuleSpec[authMappingModel, client.AuthMapping]{
		typeSuffix:   "authentication_mapping",
		label:        "authentication mapping",
		importPrefix: "authentication/",
		importHelp:   "Use the mapping hash (full SHA-256 or unique 8+ character prefix).",
		schema: schema.Schema{
			MarkdownDescription: "One `rsFilter.staticUserMapping[]` entry in `/openidm/config/authentication`. " +
				"Tenant-global; no realm. Mappings have no server id — Terraform identifies the live entry by a " +
				"SHA-256 of its canonical JSON. The rest of `rsFilter` (scopes, `subjectMapping`, " +
				"`anonymousUserMapping`, client credentials, …) is left untouched. `resource_prefix` is not " +
				"applied. Creating a mapping whose hash already exists is refused (import it). `roles` here is " +
				"an **array**, unlike `pingoneaic_access_rule.roles` which is a comma-separated string.",
			Attributes: map[string]schema.Attribute{
				"id":                          schema.StringAttribute{Computed: true, MarkdownDescription: "Canonical SHA-256 of the mapping. Changes when any field changes."},
				"subject":                     schema.StringAttribute{Required: true, MarkdownDescription: "Token subject this mapping matches, e.g. an OAuth2 client id."},
				"local_user":                  schema.StringAttribute{Required: true, MarkdownDescription: "IDM identity path, usually `internal/user/…`. A synthetic `internal/role/…` is legal."},
				"roles":                       schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "IDM role ids granted to this subject. Omit the argument to leave the key off (one live mapping does)."},
				"user_roles":                  schema.StringAttribute{Optional: true, MarkdownDescription: "Relationship pointer some mappings store, e.g. `authzRoles/*`."},
				"execute_augmentation_script": schema.BoolAttribute{Optional: true, MarkdownDescription: "When set, written as `executeAugmentationScript`. Omit to leave the key off."},
			},
		},
		get: func(ctx context.Context, c *client.Client) (map[string]any, error) {
			return c.GetAuthentication(ctx)
		},
		mutate: func(ctx context.Context, c *client.Client, mutation func(map[string]any) (map[string]any, client.RuleConfirm, error)) error {
			return c.MutateAuthentication(ctx, mutation)
		},
		objects:     client.AuthMappings,
		decode:      client.DecodeAuthMapping,
		append:      client.AppendAuthMapping,
		replace:     client.ReplaceAuthMapping,
		remove:      client.RemoveAuthMapping,
		modelToRule: modelToAuthMapping,
		ruleToModel: authMappingToModel,
	}
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
