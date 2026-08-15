package client

import (
	"path/filepath"
	"testing"
)

func TestLiveAuthenticationDocumentDigest(t *testing.T) {
	doc := readJSONMap(t, filepath.Join("testdata", "authentication", "live.json"))
	got, err := Digest(doc)
	if err != nil {
		t.Fatal(err)
	}
	const want = "4fabd82ccc9aa358e4e466af81532191562807ccde0292721b84539e6630258f"
	if got != want {
		t.Fatalf("document digest %s, want %s", got, want)
	}
}

func TestDecodeAllLiveAuthMappings(t *testing.T) {
	doc := readJSONMap(t, filepath.Join("testdata", "authentication", "live.json"))
	maps, err := AuthMappings(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 5 {
		t.Fatalf("mappings = %d", len(maps))
	}
	var omittedRoles int
	for i, raw := range maps {
		got, err := DecodeAuthMapping(raw)
		if err != nil {
			t.Fatalf("mappings[%d]: %v", i, err)
		}
		h, err := AuthMappingHash(*got)
		if err != nil {
			t.Fatal(err)
		}
		live, err := Digest(raw)
		if err != nil {
			t.Fatal(err)
		}
		if h != live {
			t.Fatalf("mappings[%d] encode changed hash %s -> %s", i, live, h)
		}
		if len(got.Roles) == 0 {
			omittedRoles++
		}
	}
	if omittedRoles != 1 {
		t.Fatalf("mappings omitting roles = %d, want 1 (RCSClient)", omittedRoles)
	}
}

func TestEncodeAuthMappingOmitsEmptyRoles(t *testing.T) {
	body := EncodeAuthMapping(AuthMapping{Subject: "x", LocalUser: "internal/user/anonymous"})
	if _, ok := body["roles"]; ok {
		t.Fatalf("synthesised roles: %#v", body)
	}
	if _, ok := body["executeAugmentationScript"]; ok {
		t.Fatalf("synthesised executeAugmentationScript: %#v", body)
	}
}

func TestDecodeAuthMappingRejectsUnknownField(t *testing.T) {
	_, err := DecodeAuthMapping(map[string]any{"subject": "x", "localUser": "internal/user/anonymous", "brandNew": true})
	if err == nil {
		t.Fatal("expected unknown field")
	}
}

func TestAppendReplaceRemoveAuthMappingPreservesSiblingsAndRSFilter(t *testing.T) {
	keep := map[string]any{"subject": "amadmin", "localUser": "internal/user/openidm-admin", "roles": []any{"internal/role/openidm-admin"}}
	doc := map[string]any{
		"_id": "authentication",
		"rsFilter": map[string]any{
			"scopes":            []any{"fr:idm:*"},
			"staticUserMapping": []any{keep},
		},
	}
	next, appendConfirm, err := AppendAuthMapping(doc, AuthMapping{
		Subject:   "Terraform_probe",
		LocalUser: "internal/user/anonymous",
		Roles:     []string{"internal/role/Terraform_probe"},
	})
	if err != nil {
		t.Fatal(err)
	}
	maps, _ := AuthMappings(next)
	if len(maps) != 2 {
		t.Fatalf("len = %d", len(maps))
	}
	if DigestMust(t, maps[0]) != DigestMust(t, keep) {
		t.Fatal("append rewrote sibling")
	}
	rs := asObject(next["rsFilter"])
	scopes, _ := rs["scopes"].([]any)
	if len(scopes) != 1 || scopes[0] != "fr:idm:*" {
		t.Fatalf("rsFilter.scopes rewritten: %#v", rs["scopes"])
	}

	flag := true
	hash := appendConfirm.Hash
	replaced, replaceConfirm, err := ReplaceAuthMapping(next, hash, AuthMapping{
		Subject:                   "Terraform_probe",
		LocalUser:                 "internal/user/anonymous",
		Roles:                     []string{"internal/role/Terraform_probe"},
		ExecuteAugmentationScript: &flag,
	})
	if err != nil {
		t.Fatal(err)
	}
	newHash := replaceConfirm.Hash
	if newHash == hash {
		t.Fatal("update kept hash")
	}
	maps, _ = AuthMappings(replaced)
	if DigestMust(t, maps[0]) != DigestMust(t, keep) {
		t.Fatal("replace rewrote sibling")
	}

	removed, removeConfirm, err := RemoveAuthMapping(replaced, newHash)
	if err != nil {
		t.Fatal(err)
	}
	if removeConfirm.Count != 0 {
		t.Fatalf("remaining = %d", removeConfirm.Count)
	}
	maps, _ = AuthMappings(removed)
	if len(maps) != 1 || DigestMust(t, maps[0]) != DigestMust(t, keep) {
		t.Fatalf("remove = %#v", maps)
	}
	rs = asObject(removed["rsFilter"])
	scopes, _ = rs["scopes"].([]any)
	if len(scopes) != 1 || scopes[0] != "fr:idm:*" {
		t.Fatalf("remove rewrote scopes: %#v", rs["scopes"])
	}
}

func TestAppendAuthMappingRefusesDuplicateHash(t *testing.T) {
	m := AuthMapping{Subject: "x", LocalUser: "internal/user/anonymous"}
	doc := map[string]any{"rsFilter": map[string]any{"staticUserMapping": []any{EncodeAuthMapping(m)}}}
	_, _, err := AppendAuthMapping(doc, m)
	if err == nil {
		t.Fatal("expected duplicate refusal")
	}
}
