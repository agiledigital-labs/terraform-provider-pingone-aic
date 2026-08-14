package client

import (
	"encoding/json"
	"testing"
)

func TestDigestMatchesPythonCanonicalForm(t *testing.T) {
	// Same object, different insertion order, must hash equal — this is
	// the form docs/api/19-config-access.md pins against aic access.
	first := map[string]any{"pattern": "managed/x", "roles": "*", "methods": "read"}
	second := map[string]any{"methods": "read", "roles": "*", "pattern": "managed/x"}
	a, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("order-sensitive digest: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("want 64 hex chars, got %d %s", len(a), a)
	}
	if ShortHash(a) != a[:8] {
		t.Fatalf("short = %q", ShortHash(a))
	}
}

func TestDigestDoesNotHTMLEscape(t *testing.T) {
	// encoding/json defaults would turn & into \u0026 and change the hash
	// relative to Python / serde_json.
	got, err := CanonicalJSON(map[string]any{"x": "a&b"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"x":"a&b"}` {
		t.Fatalf("got %s", got)
	}
}

func TestFindRuleHashesFullAndPrefix(t *testing.T) {
	rules := []map[string]any{
		{"pattern": "a", "roles": "*", "methods": "read"},
		{"pattern": "b", "roles": "*", "methods": "read"},
		{"pattern": "a", "roles": "*", "methods": "read"},
	}
	h, err := Digest(rules[0])
	if err != nil {
		t.Fatal(err)
	}
	idxs, err := FindRuleHashes(rules, h)
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 2 || idxs[0] != 0 || idxs[1] != 2 {
		t.Fatalf("full hash idxs = %v", idxs)
	}
	idxs, err = FindRuleHashes(rules, h[:8])
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 2 {
		t.Fatalf("prefix idxs = %v", idxs)
	}
}

func TestDigestOfEncodedJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"methods":"read","pattern":"info/*","roles":"*","actions":"*"}`)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	h1, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := DecodeAccessRule(m)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := AccessRuleHash(*rule)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("decode+encode changed hash: %s vs %s", h1, h2)
	}
}
