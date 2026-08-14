package client

import (
	"context"
	"errors"
	"fmt"
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

// A 404 that some layer has wrapped with %w must still read as "gone" —
// otherwise Read() surfaces an error instead of dropping the resource from
// state, and Terraform never converges. See client.IsNotFound.
func TestIsNotFoundSeesThroughWrapping(t *testing.T) {
	notFound := &APIError{Status: http.StatusNotFound, Method: http.MethodGet, URL: "/gone"}
	cases := map[string]struct {
		err  error
		want bool
	}{
		"bare":          {notFound, true},
		"wrapped once":  {fmt.Errorf("read node: %w", notFound), true},
		"wrapped twice": {fmt.Errorf("journey: %w", fmt.Errorf("read node: %w", notFound)), true},
		"other status":  {&APIError{Status: http.StatusForbidden}, false},
		"wrapped 403":   {fmt.Errorf("read node: %w", &APIError{Status: http.StatusForbidden}), false},
		"unrelated":     {errors.New("boom"), false},
		"wrapped plain": {fmt.Errorf("read node: %w", errors.New("boom")), false},
		"nil":           {nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := IsNotFound(tc.err); got != tc.want {
				t.Fatalf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
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
