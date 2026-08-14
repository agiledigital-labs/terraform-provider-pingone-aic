package provider

import (
	"context"
	"os"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/nodetype"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &aicProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &aicProvider{version: version}
	}
}

type aicProvider struct {
	version string
}

type providerModel struct {
	TenantURL        types.String `tfsdk:"tenant_url"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	JWK              types.String `tfsdk:"jwk"`
	AccessToken      types.String `tfsdk:"access_token"`
	ResourcePrefix   types.String `tfsdk:"resource_prefix"`
}

func (p *aicProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pingoneaic"
	resp.Version = p.version
}

func (p *aicProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage PingOne Advanced Identity Cloud (AIC) tenant configuration. " +
			"Resources are **typed** — we do not accept raw JSON blobs. An AIC upgrade that adds or " +
			"renames a field is a provider bug until someone teaches the catalog about it.\n\n" +
			"`resource_prefix` is prepended to every name we create (scripts, journeys, inner-tree " +
			"references) so terraform-managed copies cannot collide with the originals they were " +
			"generated from. Default is `Terraform_`. Set to `\"\"` to write names as given.",
		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Tenant base URL, e.g. `https://openam-example.forgeblocks.com`. Env: `PINGONEAIC_TENANT_URL`.",
			},
			"service_account_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service-account UUID (`iss`/`sub` of the JWT). Env: `PINGONEAIC_SERVICE_ACCOUNT_ID`.",
			},
			"jwk": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Service-account RSA private JWK as a JSON string. Env: `PINGONEAIC_JWK`.",
			},
			"access_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Pre-minted bearer token. Useful for experiments (`aic whoami --token`). Env: `PINGONEAIC_ACCESS_TOKEN`. Takes precedence over JWT minting.",
			},
			"resource_prefix": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Prepended to created script and journey names. Default `Terraform_`. Env: `PINGONEAIC_RESOURCE_PREFIX`. Set to empty to disable.",
			},
		},
	}
}

func (p *aicProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tenant := first(cfg.TenantURL.ValueString(), os.Getenv("PINGONEAIC_TENANT_URL"))
	saID := first(cfg.ServiceAccountID.ValueString(), os.Getenv("PINGONEAIC_SERVICE_ACCOUNT_ID"))
	jwk := first(cfg.JWK.ValueString(), os.Getenv("PINGONEAIC_JWK"))
	token := first(cfg.AccessToken.ValueString(), os.Getenv("PINGONEAIC_ACCESS_TOKEN"))

	prefix := "Terraform_"
	if !cfg.ResourcePrefix.IsNull() && !cfg.ResourcePrefix.IsUnknown() {
		prefix = cfg.ResourcePrefix.ValueString()
	} else if v, ok := os.LookupEnv("PINGONEAIC_RESOURCE_PREFIX"); ok {
		prefix = v
	}

	if tenant == "" {
		resp.Diagnostics.AddError("Missing tenant_url", "Set tenant_url on the provider or PINGONEAIC_TENANT_URL.")
		return
	}

	c, err := client.New(client.Config{
		TenantURL:        tenant,
		ServiceAccountID: saID,
		JWK:              jwk,
		AccessToken:      token,
		ResourcePrefix:   prefix,
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *aicProvider) Resources(_ context.Context) []func() resource.Resource {
	out := []func() resource.Resource{
		resources.NewScriptResource,
		resources.NewJourneyResource,
	}
	for _, spec := range nodetype.All() {
		s := spec
		out = append(out, func() resource.Resource {
			return resources.NewNodeResource(s)
		})
	}
	return out
}

func (p *aicProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
