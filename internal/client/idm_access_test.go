package client

import (
	"path/filepath"
	"testing"
)

func TestLiveAccessDocumentDigestMatchesAIC(t *testing.T) {
	// Pinned in docs/api/19-config-access.md — two independent implementations
	// (Python json.dumps and aic access) agreed on this value.
	doc := readJSONMap(t, filepath.Join("testdata", "access", "live.json"))
	got, err := Digest(doc)
	if err != nil {
		t.Fatal(err)
	}
	const want = "75189406f2cad0de785a306176deb50fb57291319015946e98a2ae9e5900cf7f"
	if got != want {
		t.Fatalf("document digest %s, want %s", got, want)
	}
}

func TestDecodeAllLiveAccessRules(t *testing.T) {
	doc := readJSONMap(t, filepath.Join("testdata", "access", "live.json"))
	rules, err := AccessRules(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 65 {
		t.Fatalf("rules = %d", len(rules))
	}
	seen := map[string]int{}
	var missingActions, emptyActions int
	for i, raw := range rules {
		got, err := DecodeAccessRule(raw)
		if err != nil {
			t.Fatalf("rules[%d]: %v", i, err)
		}
		h, err := AccessRuleHash(*got)
		if err != nil {
			t.Fatal(err)
		}
		live, err := Digest(raw)
		if err != nil {
			t.Fatal(err)
		}
		if h != live {
			t.Fatalf("rules[%d] encode changed hash %s -> %s", i, live, h)
		}
		seen[h]++
		if got.Actions == nil {
			missingActions++
		} else if *got.Actions == "" {
			emptyActions++
		}
	}
	if missingActions != 6 {
		t.Fatalf("rules omitting actions = %d, want 6", missingActions)
	}
	if emptyActions != 3 {
		t.Fatalf("rules with actions=\"\" = %d, want 3", emptyActions)
	}
	dups := 0
	for _, n := range seen {
		if n > 1 {
			dups++
		}
	}
	if len(seen) != 59 || dups != 1 {
		t.Fatalf("unique = %d dup-groups = %d", len(seen), dups)
	}
}

func TestEncodeAccessRuleOmitsOptionalKeys(t *testing.T) {
	body := EncodeAccessRule(AccessRule{Pattern: "managed/x", Roles: "*", Methods: "read"})
	if _, ok := body["actions"]; ok {
		t.Fatalf("synthesised actions: %#v", body)
	}
	if _, ok := body["customAuthz"]; ok {
		t.Fatalf("synthesised customAuthz: %#v", body)
	}
	empty := ""
	withEmpty := EncodeAccessRule(AccessRule{Pattern: "system/*", Roles: "*", Methods: "read", Actions: &empty})
	if v, ok := withEmpty["actions"]; !ok || v != "" {
		t.Fatalf("empty actions must stay present: %#v", withEmpty)
	}
}

func TestDecodeAccessRuleRejectsUnknownField(t *testing.T) {
	_, err := DecodeAccessRule(map[string]any{"pattern": "x", "roles": "*", "methods": "read", "brandNew": true})
	if err == nil {
		t.Fatal("expected unknown field")
	}
}

func TestAppendReplaceRemoveAccessRulePreservesSiblings(t *testing.T) {
	keep := map[string]any{"pattern": "keep", "roles": "*", "methods": "read"}
	doc := map[string]any{"_id": "access", "configs": []any{keep}}
	star := "*"
	next, hash, err := AppendAccessRule(doc, AccessRule{Pattern: "probe", Roles: "*", Methods: "query", Actions: &star})
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := AccessRules(next)
	if len(rules) != 2 {
		t.Fatalf("len = %d", len(rules))
	}
	if DigestMust(t, rules[0]) != DigestMust(t, keep) {
		t.Fatal("append rewrote sibling")
	}

	replaced, newHash, err := ReplaceAccessRule(next, hash, AccessRule{Pattern: "probe", Roles: "*", Methods: "query,read", Actions: &star})
	if err != nil {
		t.Fatal(err)
	}
	if newHash == hash {
		t.Fatal("update kept hash")
	}
	rules, _ = AccessRules(replaced)
	if DigestMust(t, rules[0]) != DigestMust(t, keep) {
		t.Fatal("replace rewrote sibling")
	}
	if rules[1]["methods"] != "query,read" {
		t.Fatalf("replace = %#v", rules[1])
	}

	removed, remaining, err := RemoveAccessRule(replaced, newHash)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d", remaining)
	}
	rules, _ = AccessRules(removed)
	if len(rules) != 1 || DigestMust(t, rules[0]) != DigestMust(t, keep) {
		t.Fatalf("remove = %#v", rules)
	}
}

func TestAppendAccessRuleRefusesDuplicateHash(t *testing.T) {
	rule := AccessRule{Pattern: "probe", Roles: "*", Methods: "read"}
	doc := map[string]any{"configs": []any{EncodeAccessRule(rule)}}
	_, _, err := AppendAccessRule(doc, rule)
	if err == nil {
		t.Fatal("expected duplicate refusal")
	}
}

func TestRemoveAccessRuleRemovesFirstDuplicateOnly(t *testing.T) {
	dup := map[string]any{"pattern": "x", "roles": "*", "methods": "read"}
	doc := map[string]any{"configs": []any{dup, map[string]any{"pattern": "y", "roles": "*", "methods": "read"}, dup}}
	h := DigestMust(t, dup)
	next, remaining, err := RemoveAccessRule(doc, h)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d", remaining)
	}
	rules, _ := AccessRules(next)
	if len(rules) != 2 {
		t.Fatalf("len = %d", len(rules))
	}
	if DigestMust(t, rules[0]) == h {
		t.Fatal("removed the wrong copy")
	}
	if DigestMust(t, rules[1]) != h {
		t.Fatal("removed both copies")
	}
}

func DigestMust(t *testing.T, v any) string {
	t.Helper()
	h, err := Digest(v)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
