package oauth2client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogHas115FieldsAndUniqueTFNames(t *testing.T) {
	if got := FieldCount(); got != 115 {
		t.Fatalf("FieldCount = %d, want 115", got)
	}
	if len(AllGroups()) != 6 {
		t.Fatalf("groups = %d, want 6", len(AllGroups()))
	}
	seen := map[string]string{}
	for _, g := range AllGroups() {
		for _, f := range g.Fields {
			key := g.TFName + "." + f.TFName
			if prev, ok := seen[key]; ok {
				t.Fatalf("duplicate TF name %s (also %s)", key, prev)
			}
			seen[key] = f.APIName
		}
	}
}

func TestDecodeRejectsUnknownTopLevelAndGroupField(t *testing.T) {
	_, err := DecodeAPI(map[string]any{
		"coreOAuth2ClientConfig": map[string]any{
			"clientType":    map[string]any{"inherited": false, "value": "Confidential"},
			"brandNewField": true,
		},
	}, "")
	if err == nil {
		t.Fatal("expected unknown field error")
	}

	_, err = DecodeAPI(map[string]any{
		"brandNewGroup": map[string]any{},
	}, "")
	if err == nil {
		t.Fatal("expected unknown group error")
	}
}

func TestDecodeUnwrapsInheritedAndFillsDefaults(t *testing.T) {
	vals, err := DecodeAPI(map[string]any{
		"_id":  "Probe",
		"_rev": "1",
		"_type": map[string]any{
			"_id": "OAuth2Client",
		},
		"coreOAuth2ClientConfig": map[string]any{
			"clientType": map[string]any{"inherited": false, "value": "Public"},
			"scopes":     map[string]any{"inherited": false, "value": []any{"openid"}},
		},
		"overrideOAuth2ClientConfig": map[string]any{
			"providerOverridesEnabled": true,
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	core := vals["core"]
	if core["client_type"] != "Public" {
		t.Fatalf("client_type = %#v", core["client_type"])
	}
	scopes, _ := core["scopes"].([]string)
	if len(scopes) != 1 || scopes[0] != "openid" {
		t.Fatalf("scopes = %#v", core["scopes"])
	}
	if core["status"] != "Active" {
		t.Fatalf("missing-key status default = %#v", core["status"])
	}
	if vals["override"]["provider_overrides_enabled"] != true {
		t.Fatalf("raw override bool = %#v", vals["override"]["provider_overrides_enabled"])
	}
	if _, ok := vals["advanced"]; !ok {
		t.Fatal("omitted group should still be filled with defaults")
	}
}

func TestDecodeInheritedTrueUsesDefault(t *testing.T) {
	vals, err := DecodeAPI(map[string]any{
		"coreOAuth2ClientConfig": map[string]any{
			"clientType": map[string]any{"inherited": true, "value": "Public"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if vals["core"]["client_type"] != "Confidential" {
		t.Fatalf("inherited:true should fall back to default, got %#v", vals["core"]["client_type"])
	}
}

func TestEncodeFillsTemplateAndWritesRawValues(t *testing.T) {
	body, err := EncodeAPI(Values{
		"core": {
			"client_type": "Public",
			"scopes":      []string{"openid"},
		},
		"advanced": {
			"grant_types":                []string{"client_credentials"},
			"token_endpoint_auth_method": "client_secret_post",
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["_id"]; ok {
		t.Fatal("PUT body must not include _id")
	}
	core, _ := body["coreOAuth2ClientConfig"].(map[string]any)
	if core["clientType"] != "Public" {
		t.Fatalf("raw clientType = %#v", core["clientType"])
	}
	if _, wrapped := core["clientType"].(map[string]any); wrapped {
		t.Fatal("create body must write raw values, not inherited wrappers")
	}
	if core["status"] != "Active" {
		t.Fatalf("unset fields should keep the template default, got status=%#v", core["status"])
	}
	adv, _ := body["advancedOAuth2ClientConfig"].(map[string]any)
	if adv["tokenEndpointAuthMethod"] != "client_secret_post" {
		t.Fatalf("tokenEndpointAuthMethod = %#v", adv["tokenEndpointAuthMethod"])
	}
}

func TestEncodeRejectsUnknownTFAttribute(t *testing.T) {
	_, err := EncodeAPI(Values{"core": {"not_a_field": "x"}}, "")
	if err == nil {
		t.Fatal("expected unknown attribute error")
	}
	_, err = EncodeAPI(Values{"nope": {}}, "")
	if err == nil {
		t.Fatal("expected unknown group error")
	}
}

func TestTreeNamePrefixRoundTrip(t *testing.T) {
	vals, err := DecodeAPI(map[string]any{
		"advancedOAuth2ClientConfig": map[string]any{
			"treeName": map[string]any{"inherited": false, "value": "Terraform_GetIP"},
		},
	}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	if vals["advanced"]["tree_name"] != "GetIP" {
		t.Fatalf("strip = %#v", vals["advanced"]["tree_name"])
	}
	body, err := EncodeAPI(Values{"advanced": {"tree_name": "GetIP"}}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	adv, _ := body["advancedOAuth2ClientConfig"].(map[string]any)
	if adv["treeName"] != "Terraform_GetIP" {
		t.Fatalf("apply = %#v", adv["treeName"])
	}

	empty, err := EncodeAPI(Values{"advanced": {"tree_name": EmptySentinel}}, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	adv, _ = empty["advancedOAuth2ClientConfig"].(map[string]any)
	if adv["treeName"] != EmptySentinel {
		t.Fatalf("[Empty] must not be prefixed, got %#v", adv["treeName"])
	}
}

func TestSanitizeWriteStripsServerAndEncryptedFields(t *testing.T) {
	got := SanitizeWrite(map[string]any{
		"_id":       "x",
		"_rev":      "1",
		"_type":     map[string]any{"_id": "OAuth2Client"},
		"_provider": map[string]any{},
		"coreOAuth2ClientConfig": map[string]any{
			"userpassword":           nil,
			"userpassword-encrypted": "AQIC...",
			"nested": map[string]any{
				"foo-encrypted": "secret",
				"keep":          "ok",
			},
		},
	})
	if _, ok := got["_id"]; ok {
		t.Fatal("left _id")
	}
	if _, ok := got["_rev"]; ok {
		t.Fatal("left _rev")
	}
	if _, ok := got["_type"]; ok {
		t.Fatal("left _type")
	}
	if _, ok := got["_provider"]; ok {
		t.Fatal("left _provider")
	}
	core, _ := got["coreOAuth2ClientConfig"].(map[string]any)
	if _, ok := core["userpassword-encrypted"]; ok {
		t.Fatal("left encrypted sibling")
	}
	nested, _ := core["nested"].(map[string]any)
	if _, ok := nested["foo-encrypted"]; ok {
		t.Fatal("left nested encrypted field")
	}
	if nested["keep"] != "ok" {
		t.Fatalf("stripped too much: %#v", nested)
	}
}

func TestEncodeSkipsEmptyUserpassword(t *testing.T) {
	body, err := EncodeAPI(Values{"core": {"userpassword": ""}}, "")
	if err != nil {
		t.Fatal(err)
	}
	core, _ := body["coreOAuth2ClientConfig"].(map[string]any)
	if core["userpassword"] != nil {
		t.Fatalf("empty password should stay template null, got %#v", core["userpassword"])
	}
}

func TestEqualDefaultOmitsTemplateValues(t *testing.T) {
	g, _ := LookupGroup("coreOAuth2ClientConfig")
	clientType, _ := g.FieldByTF("client_type")
	if !EqualDefault(clientType, "Confidential") {
		t.Fatal("Confidential is the template default")
	}
	if EqualDefault(clientType, "Public") {
		t.Fatal("Public is not the default")
	}
	scopes, _ := g.FieldByTF("scopes")
	if !EqualDefault(scopes, []string{}) {
		t.Fatal("empty scopes is the default")
	}
	adv, _ := LookupGroup("advancedOAuth2ClientConfig")
	custom, _ := adv.FieldByTF("custom_properties")
	if !EqualDefault(custom, []string{}) {
		t.Fatal("AM stores customProperties as [], not the template [\"\"]")
	}
}

func TestDecodeLiveShapedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "client_get.json"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	vals, err := DecodeAPI(body, "Terraform_")
	if err != nil {
		t.Fatal(err)
	}
	if vals["core"]["client_type"] != "Confidential" {
		t.Fatalf("client_type = %#v", vals["core"]["client_type"])
	}
	grants, _ := vals["advanced"]["grant_types"].([]string)
	if len(grants) != 2 || grants[0] != "authorization_code" {
		t.Fatalf("grant_types = %#v", grants)
	}
	if vals["override"]["provider_overrides_enabled"] != true {
		t.Fatalf("provider_overrides_enabled = %#v", vals["override"]["provider_overrides_enabled"])
	}
	if vals["override"]["access_token_modification_plugin_type"] != "SCRIPTED" {
		t.Fatalf("plugin type = %#v", vals["override"]["access_token_modification_plugin_type"])
	}
}
