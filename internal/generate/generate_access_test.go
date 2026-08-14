package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestWriteAccessEmitsReviewFilesNotTf(t *testing.T) {
	dir := t.TempDir()
	g := &gen{
		opt: Options{OutDir: dir},
		accessRules: []emittedAccessRule{{
			Label:   "info",
			Hash:    "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd",
			Copies:  1,
			Indices: []int{0},
			Rule:    client.AccessRule{Pattern: "info/*", Roles: "*", Methods: "read", Actions: strptr("*")},
		}},
		authMappings: []emittedAuthMapping{{
			Label:   "amadmin",
			Hash:    "1111111111111111111111111111111111111111111111111111111111111111",
			Mapping: client.AuthMapping{Subject: "amadmin", LocalUser: "internal/user/openidm-admin", Roles: []string{"internal/role/openidm-admin"}},
		}},
	}
	if err := g.writeAccess(); err != nil {
		t.Fatal(err)
	}
	access := filepath.Join(dir, "access_rules.tf.review")
	auth := filepath.Join(dir, "authentication_mappings.tf.review")
	for _, path := range []string{access, auth} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if !strings.Contains(body, "NOT loaded by Terraform") {
			t.Fatalf("%s missing review warning", path)
		}
		if !strings.Contains(body, "terraform import") {
			t.Fatalf("%s missing import hint", path)
		}
	}
	ab, _ := os.ReadFile(access)
	if !strings.Contains(string(ab), "abcdabcd") || !strings.Contains(string(ab), `pattern = "info/*"`) {
		t.Fatalf("access body:\n%s", ab)
	}
	if _, err := os.Stat(filepath.Join(dir, "access_rules.tf")); !os.IsNotExist(err) {
		t.Fatal("emitted applyable access_rules.tf")
	}
}

func strptr(s string) *string { return &s }
