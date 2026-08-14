package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
)

func TestWriteRolesUsesDisplayNameForUUIDIds(t *testing.T) {
	dir := t.TempDir()
	g := &gen{
		opt: Options{OutDir: dir},
		roles: []emittedRole{{
			Name:  "identity-access-manager",
			Label: "identity_access_manager",
			Role: client.Role{
				ID:          "f5897e85-208b-42cf-b400-d4e5d4baa7c8",
				Name:        "identity-access-manager",
				Description: "Customer One",
				Privileges: []client.Privilege{{
					Name: "orgs", Path: "managed/alpha_organization",
					Actions:     []string{},
					Permissions: []string{"VIEW"},
					Filter:      `/name eq "Inactive Users to Review"`,
					AccessFlags: []client.AccessFlag{{Attribute: "name", ReadOnly: true}},
				}},
			},
		}},
	}
	if err := g.writeRoles(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "internal_roles.tf"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, `name = "identity-access-manager"`) {
		t.Fatalf("missing chosen id:\n%s", s)
	}
	if !strings.Contains(s, `display_name = "identity-access-manager"`) {
		t.Fatalf("missing display_name:\n%s", s)
	}
	if !strings.Contains(s, "access_flag") || !strings.Contains(s, "filter") {
		t.Fatalf("missing privilege fields:\n%s", s)
	}
}
