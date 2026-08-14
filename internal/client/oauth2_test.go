package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
)

func TestNewRequestVersionOverridesAPIVersion(t *testing.T) {
	c, err := New(Config{TenantURL: "tenant.example", AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewRequestVersion(context.Background(), http.MethodGet, "/oauth", OAuth2APIVersion, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Accept-API-Version"); got != OAuth2APIVersion {
		t.Fatalf("version = %q, want %q", got, OAuth2APIVersion)
	}
	if req.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		t.Fatal("missing X-Requested-With")
	}
}

func TestPutOAuth2ClientUsesProtocol21AndRejectsPathSeparators(t *testing.T) {
	var gotVersion, gotPath string
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		gotVersion = req.Header.Get("Accept-API-Version")
		gotPath = req.URL.Path
		status := http.StatusOK
		if req.Method == http.MethodPut {
			status = http.StatusCreated
		}
		body := `{"_id":"Terraform_Probe","coreOAuth2ClientConfig":{"clientType":{"inherited":false,"value":"Confidential"}}}`
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutOAuth2Client(context.Background(), "alpha", "a/b", map[string]any{}); err == nil {
		t.Fatal("expected path-separator rejection")
	}
	if _, err := c.PutOAuth2Client(context.Background(), "alpha", "Terraform_Probe", map[string]any{
		"_id":  "nope",
		"_rev": "1",
		"coreOAuth2ClientConfig": map[string]any{
			"clientType":             "Confidential",
			"userpassword-encrypted": "AQIC",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if gotVersion != OAuth2APIVersion {
		t.Fatalf("Accept-API-Version = %q", gotVersion)
	}
	if !strings.HasSuffix(gotPath, "/realm-config/agents/OAuth2Client/Terraform_Probe") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestListOAuth2ClientsFollowsPageCookie(t *testing.T) {
	var calls int
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Query().Get("_pagedResultsCookie") == "" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"result":[{"_id":"one"}],"pagedResultsCookie":"next"}`)),
				Header: make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"result":[{"_id":"two"}]}`)),
			Header:     make(http.Header),
		}, nil
	})
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := c.ListOAuth2Clients(context.Background(), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(ids) != 2 || ids[0] != "one" || ids[1] != "two" {
		t.Fatalf("ids = %v", ids)
	}
}
