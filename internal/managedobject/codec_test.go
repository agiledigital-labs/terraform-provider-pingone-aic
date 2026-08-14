package managedobject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeAllCustomObjectsFromTenantSweep(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "custom_objects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Objects []map[string]any `json:"objects"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Objects) == 0 {
		t.Fatal("empty fixture")
	}
	for _, obj := range doc.Objects {
		name, _ := obj["name"].(string)
		got, err := DecodeAPI(obj, "Terraform_")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.Name != name {
			t.Fatalf("%s: strip left %q", name, got.Name)
		}
	}
}

func TestRelationshipPathPrefixRoundTrip(t *testing.T) {
	decoded, err := DecodeAPI(map[string]any{
		"name": "Terraform_test_from",
		"schema": map[string]any{
			"type":  "object",
			"title": "From",
			"order": []any{"rel"},
			"properties": map[string]any{
				"rel": map[string]any{
					"type": "relationship",
					"resourceCollection": []any{
						map[string]any{"path": "managed/Terraform_test_to", "label": "to"},
					},
					"reversePropertyName": "back",
				},
			},
		},
	}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "test_from" {
		t.Fatalf("name = %q", decoded.Name)
	}
	if decoded.Properties[0].ResourcePath != "managed/test_to" {
		t.Fatalf("path = %q", decoded.Properties[0].ResourcePath)
	}
	body := EncodeAPI(*decoded, "Terraform_")
	if body["name"] != "Terraform_test_from" {
		t.Fatalf("encode name = %#v", body["name"])
	}
	schema := body["schema"].(map[string]any)
	rel := schema["properties"].(map[string]any)["rel"].(map[string]any)
	col := rel["resourceCollection"].([]any)[0].(map[string]any)
	if col["path"] != "managed/Terraform_test_to" {
		t.Fatalf("encode path = %#v", col["path"])
	}
}

func TestDecodeRejectsUnknownPropertyField(t *testing.T) {
	_, err := DecodeAPI(map[string]any{
		"name": "x",
		"schema": map[string]any{
			"properties": map[string]any{
				"a": map[string]any{"type": "string", "brandNew": true},
			},
		},
	}, "")
	if err == nil {
		t.Fatal("expected unknown field")
	}
}
