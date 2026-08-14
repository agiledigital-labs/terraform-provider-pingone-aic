package nodetype

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

func TestDecodeRejectsUnknownField(t *testing.T) {
	spec, ok := Lookup("MessageNode")
	if !ok {
		t.Fatal("missing MessageNode")
	}
	_, err := DecodeAPI(spec, map[string]any{
		"message":       map[string]any{"en": "hi"},
		"messageYes":    map[string]any{},
		"messageNo":     map[string]any{},
		"brandNewField": true,
		"_id":           "x",
		"_rev":          "1",
	}, "Terraform_")
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestESVRoundTrip(t *testing.T) {
	spec, _ := Lookup("PersistentCookieDecisionNode")
	decoded, err := DecodeAPI(spec, map[string]any{
		"hmacSigningKey":       map[string]any{"$string": "&{esv.abc.datahmacsigningkey}"},
		"idleTimeout":          float64(1440),
		"persistentCookieName": "session-jwt",
		"sameSite":             "LAX",
		"enforceClientIp":      false,
		"useHttpOnlyCookie":    true,
		"useSecureCookie":      true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := decoded["hmac_signing_key"].(string)
	if got != "&{esv.abc.datahmacsigningkey}" {
		t.Fatalf("unwrap = %q", got)
	}
	body, err := EncodeAPI(spec, decoded, "")
	if err != nil {
		t.Fatal(err)
	}
	wrap, ok := body["hmacSigningKey"].(map[string]any)
	if !ok || wrap["$string"] != "&{esv.abc.datahmacsigningkey}" {
		t.Fatalf("wrap = %#v", body["hmacSigningKey"])
	}
}

func TestInnerTreePrefix(t *testing.T) {
	spec, _ := Lookup("InnerTreeEvaluatorNode")
	decoded, err := DecodeAPI(spec, map[string]any{
		"tree":                "Terraform__DealerMfa",
		"displayErrorOutcome": false,
	}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	if decoded["tree"] != "_DealerMfa" {
		t.Fatalf("strip = %#v", decoded["tree"])
	}
	body, err := EncodeAPI(spec, map[string]any{"tree": "_DealerMfa"}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	if body["tree"] != "Terraform__DealerMfa" {
		t.Fatalf("apply = %#v", body["tree"])
	}
	if prefix.Apply("Terraform_", "_DealerMfa") != "Terraform__DealerMfa" {
		t.Fatal("prefix helper")
	}
}

func TestScriptedDecisionDefaultsOmittedOnEncodeFill(t *testing.T) {
	spec, _ := Lookup("ScriptedDecisionNode")
	body, err := EncodeAPI(spec, map[string]any{
		"script_id": "abc",
		"outcomes":  []string{"ok", "noip"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	in, _ := body["inputs"].([]any)
	if len(in) != 1 || in[0] != "*" {
		t.Fatalf("inputs default = %#v", body["inputs"])
	}
}

func TestEqualDefault(t *testing.T) {
	spec, _ := Lookup("ScriptedDecisionNode")
	in, _ := spec.FieldByTF("inputs")
	if !EqualDefault(in, []string{"*"}) {
		t.Fatal("inputs [*] should be default")
	}
	if EqualDefault(in, []string{"username"}) {
		t.Fatal("custom inputs should not be default")
	}
}
