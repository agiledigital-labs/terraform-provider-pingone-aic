package resources

import (
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOAuth2RemoteNamePersistsAcrossPrefixChange(t *testing.T) {
	id := types.StringValue("OldPrefix_App")
	remote := types.StringValue("OldPrefix_App")
	if got := oauth2RemoteName(id, remote, "NewPrefix_", "App"); got != "OldPrefix_App" {
		t.Fatalf("got %q", got)
	}
	if got := oauth2RemoteName(types.StringNull(), remote, "NewPrefix_", "App"); got != "OldPrefix_App" {
		t.Fatalf("remote_name fallback = %q", got)
	}
	if got := oauth2RemoteName(types.StringNull(), types.StringNull(), "Prefix_", "App"); got != "Prefix_App" {
		t.Fatalf("create fallback = %q", got)
	}
}

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
