package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestESVRemoteNamePersistsAcrossPrefixChange(t *testing.T) {
	state := esvVariableModel{
		ID:         types.StringValue("esv-oldprefix_test11"),
		RemoteName: types.StringValue("esv-oldprefix_test11"),
		Name:       types.StringValue("esv-test11"),
	}
	if got := esvRemoteName(state, "NewPrefix_"); got != "esv-oldprefix_test11" {
		t.Fatalf("got %q", got)
	}
	if got := esvRemoteName(esvVariableModel{Name: types.StringValue("esv-test11")}, "Terraform_"); got != "esv-terraform_test11" {
		t.Fatalf("create fallback = %q", got)
	}
}

func TestVariableToModelStripsESVPrefixAndKeepsUnloaded(t *testing.T) {
	m := variableToModel(&client.Variable{
		ID: "esv-terraform_test11", ExpressionType: "string", Value: "test1", Loaded: false,
	}, "", "Terraform_")
	if m.Name.ValueString() != "esv-test11" || m.RemoteName.ValueString() != "esv-terraform_test11" {
		t.Fatalf("names: %#v", m)
	}
	if m.Loaded.ValueBool() {
		t.Fatal("fresh write must report loaded=false; a restart is a separate operator action")
	}
	if prefix.ApplyESV("Terraform_", "esv-test11") != "esv-terraform_test11" {
		t.Fatal("prefix helper")
	}
}
