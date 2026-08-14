package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConfigRemoteNamePersistsAcrossPrefixChange(t *testing.T) {
	if got := configRemoteName(types.StringValue("Old_test"), types.StringValue("Old_test"), "New_", "test"); got != "Old_test" {
		t.Fatalf("got %q", got)
	}
	if got := configRemoteName(types.StringNull(), types.StringNull(), "Terraform_", "test"); got != "Terraform_test" {
		t.Fatalf("create = %q", got)
	}
}

func TestEndpointToModelStripsPrefix(t *testing.T) {
	m := endpointToModel(&client.Endpoint{Name: "Terraform_probe", Type: "text/javascript", Source: "return {};"}, "", "Terraform_")
	if m.Name.ValueString() != "probe" || m.RemoteName.ValueString() != "Terraform_probe" {
		t.Fatalf("%#v", m)
	}
}
