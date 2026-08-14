package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoleToModelNullsDisplayNameWhenItEqualsID(t *testing.T) {
	m := roleToModel(&client.Role{ID: "Terraform_probe", Name: "Terraform_probe", Description: "d"}, "probe", "Terraform_probe")
	if m.Name.ValueString() != "probe" || m.RemoteName.ValueString() != "Terraform_probe" {
		t.Fatalf("%#v", m)
	}
	if !m.DisplayName.IsNull() {
		t.Fatalf("display_name = %s", m.DisplayName.ValueString())
	}
}

func TestRoleToModelKeepsDistinctDisplayName(t *testing.T) {
	m := roleToModel(&client.Role{ID: "abc-uuid", Name: "identity-access-manager"}, "abc-uuid", "abc-uuid")
	if m.DisplayName.ValueString() != "identity-access-manager" {
		t.Fatalf("display_name = %s", m.DisplayName.ValueString())
	}
}

func TestModelToRoleUsesRemoteIDAsNameWhenDisplayOmitted(t *testing.T) {
	got, err := modelToRole(internalRoleModel{
		Name: types.StringValue("probe"),
		Privileges: []rolePrivilegeModel{{
			Name:        types.StringValue("n"),
			Path:        types.StringValue("managed/alpha_user"),
			Actions:     stringSetValue(nil),
			Permissions: stringSetValue([]string{"VIEW"}),
			AccessFlags: []roleFlagModel{{Attribute: types.StringValue("mail"), ReadOnly: types.BoolValue(true)}},
		}},
	}, "Terraform_probe")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Terraform_probe" {
		t.Fatalf("name = %s", got.Name)
	}
}

func TestModelToRoleRejectsUpdateWithoutWritableFlag(t *testing.T) {
	_, err := modelToRole(internalRoleModel{
		Name: types.StringValue("probe"),
		Privileges: []rolePrivilegeModel{{
			Name:        types.StringValue("n"),
			Path:        types.StringValue("managed/alpha_user"),
			Actions:     stringSetValue(nil),
			Permissions: stringSetValue([]string{"UPDATE"}),
			AccessFlags: []roleFlagModel{{Attribute: types.StringValue("mail"), ReadOnly: types.BoolValue(true)}},
		}},
	}, "Terraform_probe")
	if err == nil {
		t.Fatal("expected error")
	}
}
