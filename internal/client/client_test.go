package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
)

func TestNewRequestSetsRequiredAMHeaders(t *testing.T) {
	c, err := New(Config{TenantURL: "tenant.example", AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewRequest(context.Background(), http.MethodPut, "/resource", map[string]any{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"Authorization":      "Bearer token",
		"Accept":             "application/json",
		"Accept-API-Version": AMAPIVersion,
		"X-Requested-With":   "XMLHttpRequest",
		"Content-Type":       "application/json",
	}
	for name, want := range checks {
		if got := req.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if req.URL.String() != "https://tenant.example/resource" {
		t.Fatalf("URL = %q", req.URL)
	}
}

func TestDoReturnsInspectableAPIError(t *testing.T) {
	httpClient := testutil.Client(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
		}, nil
	})
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewRequest(context.Background(), http.MethodGet, "/missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Do(req, http.StatusOK, nil)
	if !IsNotFound(err) {
		t.Fatalf("got %T %v, want not-found APIError", err, err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Method != http.MethodGet {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}

func TestNewValidatesAuthenticationInputs(t *testing.T) {
	if _, err := New(Config{TenantURL: "https://tenant.example"}); err == nil {
		t.Fatal("expected missing-authentication error")
	}
	if _, err := New(Config{AccessToken: "token"}); err == nil {
		t.Fatal("expected missing-tenant error")
	}
}
