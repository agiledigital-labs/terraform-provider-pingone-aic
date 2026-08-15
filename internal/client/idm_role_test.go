package client

import (
	"path/filepath"
	"testing"
)

func TestDecodeRoleRejectsUnknownField(t *testing.T) {
	_, err := DecodeRole(map[string]any{"_id": "x", "name": "x", "brandNew": true})
	if err == nil {
		t.Fatal("expected unknown field")
	}
}

func TestDecodeRoleRejectsNonEmptyTemporal(t *testing.T) {
	_, err := DecodeRole(map[string]any{
		"_id": "x", "name": "x",
		"temporalConstraints": []any{map[string]any{"duration": "1"}},
	})
	if err == nil {
		t.Fatal("expected temporalConstraints error")
	}
}

func TestEncodeRoleOmitsTemporalAndRev(t *testing.T) {
	body, err := EncodeRole(Role{
		ID:   "x",
		Rev:  "stale",
		Name: "x",
		Privileges: []Privilege{{
			Name: "n", Path: "managed/alpha_user",
			Actions:     []string{},
			Permissions: []string{"VIEW"},
			AccessFlags: []AccessFlag{{Attribute: "mail", ReadOnly: true}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["temporalConstraints"]; ok {
		t.Fatalf("encoded temporalConstraints: %#v", body)
	}
	if _, ok := body["_rev"]; ok {
		t.Fatalf("encoded _rev: %#v", body)
	}
	if _, ok := body["_id"]; ok {
		t.Fatalf("encoded _id: %#v", body)
	}
}

func TestEncodeRoleRefusesUpdateWithoutWritableFlag(t *testing.T) {
	_, err := EncodeRole(Role{Name: "x", Privileges: []Privilege{{
		Name: "n", Path: "managed/alpha_user",
		Permissions: []string{"VIEW", "UPDATE"},
		AccessFlags: []AccessFlag{{Attribute: "mail", ReadOnly: true}},
	}}})
	if err == nil {
		t.Fatal("expected write-flag error")
	}
}

func TestEncodeRoleSendsEmptyPrivileges(t *testing.T) {
	body, err := EncodeRole(Role{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	privs, ok := body["privileges"].([]any)
	if !ok || len(privs) != 0 {
		t.Fatalf("privileges = %#v", body["privileges"])
	}
}

func TestRoleLooksLikeUUID(t *testing.T) {
	if !RoleLooksLikeUUID("f5897e85-208b-42cf-b400-d4e5d4baa7c8") {
		t.Fatal("uuid")
	}
	if RoleLooksLikeUUID("openidm-admin") || RoleLooksLikeUUID("Terraform_probe") {
		t.Fatal("non-uuid")
	}
}

func TestDecodePrivilegeRequiresFilterOptional(t *testing.T) {
	raw := readJSONMap(t, filepath.Join("testdata", "roles", "f5897e85-208b-42cf-b400-d4e5d4baa7c8.json"))
	got, err := DecodeRole(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Privileges) != 2 || got.Privileges[0].Filter == "" || got.Privileges[1].Filter != "" {
		t.Fatalf("%#v", got.Privileges)
	}
}
