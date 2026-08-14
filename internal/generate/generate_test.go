package generate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
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

func TestWriteTerraformFileFormatsHCL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.tf")
	body := []byte("resource \"example\" \"test\" {\n  short = 1\n  longer_name = 2\n}\n")
	if err := writeTerraformFile(path, body); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "resource \"example\" \"test\" {\n  short       = 1\n  longer_name = 2\n}\n"
	if string(got) != want {
		t.Fatalf("formatted HCL:\n%s\nwant:\n%s", got, want)
	}
}

func TestProgressfIsOptionalAndReadable(t *testing.T) {
	progressf(Options{}, "ignored %d", 1)
	var out bytes.Buffer
	progressf(Options{Progress: &out}, "read %d journey", 1)
	if got, want := out.String(), "read 1 journey\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOptionalInt64(t *testing.T) {
	if got := optionalInt64(float64(42)); got == nil || *got != 42 {
		t.Fatalf("got %#v, want 42", got)
	}
	if got := optionalInt64(nil); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestRunHonorsCancelledContext(t *testing.T) {
	httpClient := &http.Client{Transport: generateRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	c, err := client.New(client.Config{
		TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Run(ctx, c, Options{OutDir: t.TempDir(), Progress: io.Discard})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
}

type generateRoundTripFunc func(*http.Request) (*http.Response, error)

func (f generateRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
