package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

func TestVariableToModelStripsESVPrefixAndKeepsUnloaded(t *testing.T) {
	m := variableToModel(&client.Variable{
		ID: "esv-terraform-test11", ExpressionType: "string", Value: "test1", Loaded: false,
	}, "", "Terraform_")
	if m.Name.ValueString() != "esv-test11" || m.RemoteName.ValueString() != "esv-terraform-test11" {
		t.Fatalf("names: %#v", m)
	}
	if m.Loaded.ValueBool() {
		t.Fatal("fresh write must report loaded=false; a restart is a separate operator action")
	}
	if prefix.ApplyESV("Terraform_", "esv-test11") != "esv-terraform-test11" {
		t.Fatal("prefix helper")
	}
}
