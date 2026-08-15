package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestAccessRuleModelRoundTripOmitsActions(t *testing.T) {
	rule := client.AccessRule{Pattern: "managed/x", Roles: "*", Methods: "read"}
	hash, err := client.AccessRuleHash(rule)
	if err != nil {
		t.Fatal(err)
	}
	m := accessRuleToModel(rule, hash)
	if !m.Actions.IsNull() || !m.CustomAuthz.IsNull() {
		t.Fatalf("optional keys should be null: %#v", m)
	}
	got := modelToAccessRule(m)
	if got.Actions != nil || got.CustomAuthz != nil {
		t.Fatalf("model reintroduced optionals: %#v", got)
	}
	h2, err := client.AccessRuleHash(got)
	if err != nil {
		t.Fatal(err)
	}
	if h2 != hash {
		t.Fatalf("hash drifted %s -> %s", hash, h2)
	}
}

func TestReadAccessRuleFindsByShortHash(t *testing.T) {
	star := "*"
	rule := client.AccessRule{Pattern: "probe", Roles: "*", Methods: "query", Actions: &star}
	encoded := client.EncodeAccessRule(rule)
	hash, err := client.Digest(encoded)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"configs": []any{encoded}}
	r := hashedRuleResource[accessRuleModel, client.AccessRule]{spec: accessRuleSpec()}
	got, err := r.read(doc, hash[:8])
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID.ValueString() != hash || got.Pattern.ValueString() != "probe" {
		t.Fatalf("%#v", got)
	}
}

func TestReadAccessRuleMissingIsNil(t *testing.T) {
	doc := map[string]any{"configs": []any{}}
	r := hashedRuleResource[accessRuleModel, client.AccessRule]{spec: accessRuleSpec()}
	got, err := r.read(doc, "aaaaaaaa")
	if err != nil || got != nil {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestAccessRuleToModelUsesPassedHash(t *testing.T) {
	m := accessRuleToModel(client.AccessRule{Pattern: "x", Roles: "*", Methods: "read"}, "deadbeef")
	if m.ID.ValueString() != "deadbeef" {
		t.Fatalf("id = %s", m.ID.ValueString())
	}
	if m.Pattern.ValueString() != "x" || m.ID.IsNull() {
		t.Fatalf("%#v", m)
	}
}
