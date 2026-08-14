package client

import (
	"strings"
	"testing"
)

// Script bodies are arbitrary user JavaScript, so the base64 wrapping has to be
// total: any string must survive the round trip byte for byte.
func FuzzScriptBodyRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"outcome = '✓';\n",
		"",
		"var a = 1;",
		"\x00\xff binary-ish",
		strings.Repeat("x", 1024),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		got, err := DecodeScriptBody(EncodeScriptBody(source))
		if err != nil {
			t.Fatalf("round trip of %q failed: %v", source, err)
		}
		if got != source {
			t.Fatalf("got %q, want %q", got, source)
		}
	})
}

func TestDecodeScriptCanonicalDefaults(t *testing.T) {
	script, err := decodeScript(map[string]any{
		"_id": "id", "name": "name", "description": nil,
		"script": EncodeScriptBody("source"), "language": "JAVASCRIPT",
		"context": "AUTHENTICATION_TREE_DECISION_NODE", "default": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if script.EvaluatorVersion != "1.0" || script.Description != "" || script.Source != "source" {
		t.Fatalf("unexpected decoded script: %#v", script)
	}
}

func TestCanonicalContext(t *testing.T) {
	if got := CanonicalContext("SCRIPTED_DECISION_NODE"); got != "AUTHENTICATION_TREE_DECISION_NODE" {
		t.Fatalf("got %q", got)
	}
	if got := CanonicalContext("OTHER"); got != "OTHER" {
		t.Fatalf("got %q", got)
	}
}
