package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
)

func TestValidateESVID(t *testing.T) {
	if err := ValidateESVID("esv-test11"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateESVID("Terraform_esv-test11"); err == nil {
		t.Fatal("uppercase / missing esv- marker must fail")
	}
}

func TestESVValueRoundTrip(t *testing.T) {
	for _, s := range []string{"", "hello", "0", "true", `["a","b"]`, "✓"} {
		got, err := DecodeESVValue(EncodeESVValue(s))
		if err != nil || got != s {
			t.Fatalf("%q → %q (%v)", s, got, err)
		}
	}
}

func TestPutVariableUsesESVVersionAndRejectsBadID(t *testing.T) {
	var gotVersion string
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		gotVersion = req.Header.Get("Accept-API-Version")
		body := `{"_id":"esv-terraform-test11","expressionType":"string","loaded":false,"valueBase64":"dGVzdDE="}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutVariable(context.Background(), "Nope", Variable{Value: "x", ExpressionType: "string"}); err == nil {
		t.Fatal("expected id rejection")
	}
	if _, err := c.PutVariable(context.Background(), "esv-terraform-test11", Variable{Value: "test1", ExpressionType: "string"}); err != nil {
		t.Fatal(err)
	}
	if gotVersion != ESVAPIVersion {
		t.Fatalf("version = %q", gotVersion)
	}
}

func TestListVariablesFollowsPageCookie(t *testing.T) {
	var calls int
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Query().Get("_pageSize") != "100" {
			t.Fatalf("pageSize = %q, want 100 (AIC rejects >100)", req.URL.Query().Get("_pageSize"))
		}
		if req.URL.Query().Get("_pagedResultsCookie") == "" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"result":[{"_id":"esv-a","expressionType":"string","valueBase64":"YQ==","loaded":true}],"pagedResultsCookie":"next"}`)),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"result":[{"_id":"esv-b","expressionType":"bool","valueBase64":"dHJ1ZQ==","loaded":true}]}`)),
			Header:     make(http.Header),
		}, nil
	})
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.ListVariables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 2 || got[0].Value != "a" || got[1].Value != "true" {
		t.Fatalf("calls=%d got=%#v", calls, got)
	}
}
