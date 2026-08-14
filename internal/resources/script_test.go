package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestScriptToModelDerivesLogicalNameAndNullDescription(t *testing.T) {
	model := scriptToModel(&client.Script{
		ID: "id", Name: "Prefix_Logical", Context: "CONTEXT",
		Language: "JAVASCRIPT", EvaluatorVersion: "2.0", Source: "source",
	}, "alpha", "Prefix_", "")
	if model.Name.ValueString() != "Logical" || model.RemoteName.ValueString() != "Prefix_Logical" {
		t.Fatalf("unexpected names: %#v", model)
	}
	if !model.Description.IsNull() {
		t.Fatalf("empty description should be null: %#v", model.Description)
	}
}
