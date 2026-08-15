package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestEndpointToModelStripsPrefix(t *testing.T) {
	m := endpointToModel(&client.Endpoint{Name: "Terraform_probe", Type: "text/javascript", Source: "return {};"}, "", "Terraform_")
	if m.Name.ValueString() != "probe" || m.RemoteName.ValueString() != "Terraform_probe" {
		t.Fatalf("%#v", m)
	}
}
