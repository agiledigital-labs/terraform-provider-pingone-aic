package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
)

func TestOAuth2EncodeDoesNotEmitUserpasswordFromEmptyState(t *testing.T) {
	body, err := oauth2client.EncodeAPI(oauth2client.Values{"core": {"client_type": "Confidential"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	core, _ := body["coreOAuth2ClientConfig"].(map[string]any)
	if core["userpassword"] != nil {
		t.Fatalf("userpassword = %#v", core["userpassword"])
	}
}
