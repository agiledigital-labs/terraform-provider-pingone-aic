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
	"regexp"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
	"github.com/hashicorp/hcl/v2/hclwrite"
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

// seedDir writes the named files (relative to dir) with dummy content.
func seedDir(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanGeneratedFilesRemovesOnlyOurOutput(t *testing.T) {
	dir := t.TempDir()
	generated := []string{"provider.tf", "scripts.tf", "oauth2_clients.tf", "esv_variables.tf", "esv_secrets.tf", "managed_objects.tf", "journey_old.tf", filepath.Join("scripts", "old.js")}
	seedDir(t, dir, append(generated, "notes.md", generatedMarker)...)

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

// The dangerous case: -out pointed at hand-written config. Without a marker we
// must refuse rather than delete, even though the names match our patterns.
func TestCleanGeneratedFilesRefusesUnmarkedDirectory(t *testing.T) {
	dir := t.TempDir()
	seedDir(t, dir, "provider.tf", "journey_handwritten.tf")

	err := cleanGeneratedFiles(dir)
	if err == nil || !strings.Contains(err.Error(), generatedMarker) {
		t.Fatalf("got %v, want a refusal naming the marker", err)
	}
	for _, name := range []string{"provider.tf", "journey_handwritten.tf"} {
		if _, statErr := os.Stat(filepath.Join(dir, name)); statErr != nil {
			t.Errorf("refused run deleted %q anyway: %v", name, statErr)
		}
	}
}

func TestCleanGeneratedFilesAcceptsFreshDirectory(t *testing.T) {
	dir := t.TempDir()
	seedDir(t, dir, "notes.md")
	if err := cleanGeneratedFiles(dir); err != nil {
		t.Fatalf("first run into a directory with no generated files: %v", err)
	}
}

// A run that fails after claiming the directory must still leave it cleanable,
// or the next invocation would refuse forever.
func TestWriteMarkerMakesDirectoryCleanable(t *testing.T) {
	dir := t.TempDir()
	seedDir(t, dir)
	if err := writeMarker(dir); err != nil {
		t.Fatal(err)
	}
	seedDir(t, dir, "journey_partial.tf")
	if err := cleanGeneratedFiles(dir); err != nil {
		t.Fatalf("marked directory should be cleanable: %v", err)
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

// sanitizeIdent feeds HCL resource labels, so a bad output is a syntax error in
// generated config. Both invariants hold for any input, not just the examples.
func FuzzSanitizeIdent(f *testing.F) {
	for _, seed := range []string{"", "Get IP", "1abc", "über-name", "__", "A.B/C", "journey_old"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := sanitizeIdent(s)
		if !validHCLIdent.MatchString(got) {
			t.Fatalf("sanitizeIdent(%q) = %q, which is not a valid HCL identifier", s, got)
		}
		if again := sanitizeIdent(got); again != got {
			t.Fatalf("not idempotent: sanitizeIdent(%q) = %q, then %q", s, got, again)
		}
	})
}

var validHCLIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// hclwrite.Format is the last thing to touch every emitted file; if it were not
// idempotent the generated tree would churn between otherwise identical runs.
func FuzzWriteTerraformFileFormatIdempotent(f *testing.F) {
	for _, seed := range []string{
		"resource \"a\" \"b\" {\n  x = 1\n  longer = 2\n}\n",
		"a = 1\n",
		"",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		once := hclwrite.Format([]byte(body))
		if twice := hclwrite.Format(once); !bytes.Equal(once, twice) {
			t.Fatalf("Format not idempotent for %q: %q then %q", body, once, twice)
		}
	})
}

func TestRunHonorsCancelledContext(t *testing.T) {
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
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

func TestDisplayConn(t *testing.T) {
	if displayConn("70e691a5-1e33-4ac3-a356-e7b6d60d92e0") != "success" {
		t.Fatal("success sentinel")
	}
	if displayConn("e301438c-0bd0-429c-ab0c-66126501069a") != "failure" {
		t.Fatal("failure sentinel")
	}
}

func TestWriteOAuth2ClientsOmitsDefaultsAndPassword(t *testing.T) {
	dir := t.TempDir()
	g := &gen{
		opt: Options{OutDir: dir, Realm: "alpha"},
		oauth2: []emittedOAuth2Client{{
			Name:  "service_C1",
			Label: "service_c1",
			Values: oauth2client.Values{
				"core": {
					"client_type":           "Confidential",
					"status":                "Active",
					"scopes":                []string{"c1"},
					"access_token_lifetime": int64(3600),
					"userpassword":          nil,
				},
				"advanced": {
					"grant_types":                []string{"client_credentials"},
					"token_endpoint_auth_method": "client_secret_basic",
					"is_consent_implied":         true,
					"subject_type":               "Public",
				},
				"override": {
					"provider_overrides_enabled": false,
				},
			},
		}},
	}
	if err := g.writeOAuth2Clients(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "oauth2_clients.tf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, `resource "pingoneaic_oauth2_client" "service_c1"`) {
		t.Fatalf("missing resource:\n%s", body)
	}
	if !strings.Contains(body, `name  = "service_C1"`) {
		t.Fatalf("missing name:\n%s", body)
	}
	if !strings.Contains(body, `scopes                 = ["c1"]`) && !strings.Contains(body, `scopes = ["c1"]`) {
		// formatted HCL may align equals
		if !strings.Contains(body, `["c1"]`) {
			t.Fatalf("missing scopes:\n%s", body)
		}
	}
	if strings.Contains(body, "userpassword =") {
		t.Fatal("must not emit userpassword")
	}
	if strings.Contains(body, `client_type`) {
		t.Fatalf("Confidential is the template default and should be omitted:\n%s", body)
	}
	if strings.Contains(body, "token_endpoint_auth_method") {
		t.Fatalf("client_secret_basic is the template default and should be omitted:\n%s", body)
	}
	if strings.Contains(body, "provider_overrides_enabled") {
		t.Fatalf("false override default should be omitted:\n%s", body)
	}
	if !strings.Contains(body, "is_consent_implied") || !strings.Contains(body, "subject_type") {
		t.Fatalf("non-default advanced fields missing:\n%s", body)
	}
}

func TestWriteVariablesEmitsPlaintextAndOmitsStringDefault(t *testing.T) {
	dir := t.TempDir()
	g := &gen{
		opt: Options{OutDir: dir},
		variables: []emittedVariable{{
			Name:  "esv-test11",
			Label: "esv_test11",
			Var: client.Variable{
				ID: "esv-test11", ExpressionType: "string", Value: "test1",
				Description: "Test variable",
			},
		}},
	}
	if err := g.writeVariables(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "esv_variables.tf"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if !strings.Contains(body, `resource "pingoneaic_esv_variable" "esv_test11"`) {
		t.Fatalf("missing resource:\n%s", body)
	}
	if strings.Contains(body, "expression_type") {
		t.Fatalf("string is the default and should be omitted:\n%s", body)
	}
	if !strings.Contains(body, `"test1"`) {
		t.Fatalf("missing value:\n%s", body)
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
