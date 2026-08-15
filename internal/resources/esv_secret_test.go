package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AIC never returns a secret value, so state is the only copy — but Terraform
// still checks the value written back equals the one planned. Collapsing null
// and "" into each other here is what produces "Provider produced inconsistent
// result after apply".
func TestSecretToModelPreservesPlannedValueExactly(t *testing.T) {
	secret := &client.Secret{ID: "esv-terraform-test11", Encoding: "generic"}

	tests := []struct {
		name  string
		value types.String
		want  types.String
	}{
		{"set", types.StringValue("s3cret"), types.StringValue("s3cret")},
		{"absent from config", types.StringNull(), types.StringNull()},
		{"explicitly empty", types.StringValue(""), types.StringValue("")},
		{"unknown becomes null", types.StringUnknown(), types.StringNull()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := secretToModel(secret, "test11", tt.value, "Terraform_")
			if !got.Value.Equal(tt.want) {
				t.Fatalf("value %v round-tripped to %v, want %v", tt.value, got.Value, tt.want)
			}
		})
	}
}
