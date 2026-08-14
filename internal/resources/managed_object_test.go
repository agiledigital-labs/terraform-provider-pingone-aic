package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/managedobject"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestManagedHookRoundTrip(t *testing.T) {
	plan := managedObjectModel{
		Name:  types.StringValue("lifecycle_probe"),
		Title: types.StringValue("Lifecycle probe"),
		Properties: []managedPropertyModel{{
			Name: types.StringValue("note"),
			Type: types.StringValue("string"),
		}},
		Hooks: []managedHookModel{
			{Event: types.StringValue("onCreate"), Type: types.StringValue("text/javascript"), Source: types.StringValue("return;")},
			{Event: types.StringValue("onDelete"), Type: types.StringValue("text/javascript"), File: types.StringValue("roles/onDelete-roles.js")},
		},
	}
	obj := modelToManaged(plan)
	if len(obj.Hooks) != 2 || obj.Hooks[0].Event != "onCreate" || obj.Hooks[1].File != "roles/onDelete-roles.js" {
		t.Fatalf("%#v", obj.Hooks)
	}
	got := managedToModel(&managedobject.Object{
		Name:       "lifecycle_probe",
		Title:      "Lifecycle probe",
		Properties: []managedobject.Property{{Name: "note", Type: "string"}},
		Hooks:      obj.Hooks,
	}, "lifecycle_probe", "Terraform_lifecycle_probe")
	if len(got.Hooks) != 2 || got.Hooks[0].Source.ValueString() != "return;" || got.Hooks[1].File.ValueString() != "roles/onDelete-roles.js" {
		t.Fatalf("%#v", got.Hooks)
	}
}
