package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestAuthMappingModelRoundTripOmitsRoles(t *testing.T) {
	mapping := client.AuthMapping{Subject: "RCSClient", LocalUser: "internal/user/idm-provisioning"}
	hash, err := client.AuthMappingHash(mapping)
	if err != nil {
		t.Fatal(err)
	}
	m := authMappingToModel(mapping, hash)
	if !m.Roles.IsNull() || !m.ExecuteAugmentationScript.IsNull() {
		t.Fatalf("optional keys should be null: %#v", m)
	}
	got := modelToAuthMapping(m)
	if len(got.Roles) != 0 || got.ExecuteAugmentationScript != nil {
		t.Fatalf("model reintroduced optionals: %#v", got)
	}
	h2, err := client.AuthMappingHash(got)
	if err != nil {
		t.Fatal(err)
	}
	if h2 != hash {
		t.Fatalf("hash drifted %s -> %s", hash, h2)
	}
}

func TestAuthMappingModelPreservesExecuteFlag(t *testing.T) {
	flag := true
	mapping := client.AuthMapping{
		Subject:                   "test_service_C1",
		LocalUser:                 "internal/role/c1",
		Roles:                     []string{"internal/role/c1"},
		ExecuteAugmentationScript: &flag,
	}
	m := authMappingToModel(mapping, "abc")
	if m.ExecuteAugmentationScript.IsNull() || !m.ExecuteAugmentationScript.ValueBool() {
		t.Fatalf("flag = %#v", m.ExecuteAugmentationScript)
	}
	got := modelToAuthMapping(m)
	if got.ExecuteAugmentationScript == nil || !*got.ExecuteAugmentationScript {
		t.Fatalf("lost flag: %#v", got)
	}
}

func TestReadAuthMappingFindsByHash(t *testing.T) {
	mapping := client.AuthMapping{Subject: "probe", LocalUser: "internal/user/anonymous"}
	encoded := client.EncodeAuthMapping(mapping)
	hash, err := client.Digest(encoded)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"rsFilter": map[string]any{"staticUserMapping": []any{encoded}}}
	r := hashedRuleResource[authMappingModel, client.AuthMapping]{spec: authenticationMappingSpec()}
	got, err := r.read(doc, hash)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Subject.ValueString() != "probe" || got.ID.ValueString() != hash {
		t.Fatalf("%#v", got)
	}
}
