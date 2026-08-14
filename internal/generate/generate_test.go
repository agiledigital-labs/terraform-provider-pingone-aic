package generate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeIdent(t *testing.T) {
	tests := map[string]string{
		"Get IP":                           "get_ip",
		"AIC-Rhino-Legacy-Probe":           "aic_rhino_legacy_probe",
		"_DealerMfa":                       "dealermfa",
		"OAuth2 Client Authorization Test": "oauth2_client_authorization_test",
		"2FA":                              "n_2fa",
	}
	for in, want := range tests {
		if got := sanitizeIdent(in); got != want {
			t.Fatalf("sanitizeIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSelectJourneysRejectsMissingNames(t *testing.T) {
	got, err := selectJourneys([]string{"One", "Two"}, []string{"Two", "Missing"})
	if err == nil || !strings.Contains(err.Error(), "Missing") {
		t.Fatalf("got %#v, %v; want missing-name error", got, err)
	}
}

func TestSelectJourneysPreservesTenantOrder(t *testing.T) {
	got, err := selectJourneys([]string{"One", "Two", "Three"}, []string{"Three", "One"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"One", "Three"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCleanGeneratedFilesLeavesUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	generated := []string{"provider.tf", "scripts.tf", "journey_old.tf", filepath.Join("scripts", "old.js")}
	for _, name := range append(generated, "notes.md") {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanGeneratedFiles(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range generated {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("generated file %q still exists", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}

func TestDisplayConn(t *testing.T) {
	if displayConn("70e691a5-1e33-4ac3-a356-e7b6d60d92e0") != "success" {
		t.Fatal("success sentinel")
	}
	if displayConn("e301438c-0bd0-429c-ab0c-66126501069a") != "failure" {
		t.Fatal("failure sentinel")
	}
}

func TestHclIdentOrQuote(t *testing.T) {
	if hclIdentOrQuote("ok") != "ok" {
		t.Fatal("ok")
	}
	if hclIdentOrQuote("true") != "true" {
		t.Fatal("true is a valid ident in HCL map keys")
	}
	if hclIdentOrQuote("no-ip") != `"no-ip"` {
		t.Fatalf("got %s", hclIdentOrQuote("no-ip"))
	}
}
