package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
)

func TestDigestMatchesPythonCanonicalForm(t *testing.T) {
	// Same object, different insertion order, must hash equal — this is
	// the form docs/api/19-config-access.md pins against aic access.
	first := map[string]any{"pattern": "managed/x", "roles": "*", "methods": "read"}
	second := map[string]any{"methods": "read", "roles": "*", "pattern": "managed/x"}
	a, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("order-sensitive digest: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("want 64 hex chars, got %d %s", len(a), a)
	}
	if ShortHash(a) != a[:8] {
		t.Fatalf("short = %q", ShortHash(a))
	}
}

func TestReplaceRuleRefusesExistingContentHashBeforePut(t *testing.T) {
	accessOld := AccessRule{Pattern: "old", Roles: "*", Methods: "read"}
	accessNew := AccessRule{Pattern: "new", Roles: "*", Methods: "read"}
	authOld := AuthMapping{Subject: "old", LocalUser: "internal/user/anonymous"}
	authNew := AuthMapping{Subject: "new", LocalUser: "internal/user/anonymous"}

	tests := []struct {
		name       string
		errorLabel string
		doc        map[string]any
		mutate     func(context.Context, *Client) error
	}{
		{
			name:       "access",
			errorLabel: "access rule",
			doc: map[string]any{"_id": "access", "configs": []any{
				EncodeAccessRule(accessOld), EncodeAccessRule(accessNew),
			}},
			mutate: func(ctx context.Context, c *Client) error {
				oldHash, _ := AccessRuleHash(accessOld)
				return c.MutateAccess(ctx, func(doc map[string]any) (map[string]any, RuleConfirm, error) {
					return ReplaceAccessRule(doc, oldHash, accessNew)
				})
			},
		},
		{
			name:       "authentication",
			errorLabel: "authentication mapping",
			doc: map[string]any{"_id": "authentication", "rsFilter": map[string]any{"staticUserMapping": []any{
				EncodeAuthMapping(authOld), EncodeAuthMapping(authNew),
			}}},
			mutate: func(ctx context.Context, c *Client) error {
				oldHash, _ := AuthMappingHash(authOld)
				return c.MutateAuthentication(ctx, func(doc map[string]any) (map[string]any, RuleConfirm, error) {
					return ReplaceAuthMapping(doc, oldHash, authNew)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gets, puts int
			httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
				switch req.Method {
				case http.MethodGet:
					gets++
				case http.MethodPut:
					puts++
				}
				body, err := json.Marshal(tt.doc)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})
			c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token", HTTPClient: httpClient})
			if err != nil {
				t.Fatal(err)
			}

			started := time.Now()
			err = tt.mutate(context.Background(), c)
			if err == nil || !strings.Contains(err.Error(), "a different "+tt.errorLabel) || !strings.Contains(err.Error(), "already has this content hash") {
				t.Fatalf("error = %v", err)
			}
			if gets != 1 || puts != 0 {
				t.Fatalf("requests: GET=%d PUT=%d, want GET=1 PUT=0", gets, puts)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("duplicate check took %s", elapsed)
			}
		})
	}
}

func TestDigestDoesNotHTMLEscape(t *testing.T) {
	// encoding/json defaults would turn & into \u0026 and change the hash
	// relative to Python / serde_json.
	got, err := CanonicalJSON(map[string]any{"x": "a&b"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"x":"a&b"}` {
		t.Fatalf("got %s", got)
	}
}

func TestFindRuleHashesFullAndPrefix(t *testing.T) {
	rules := []map[string]any{
		{"pattern": "a", "roles": "*", "methods": "read"},
		{"pattern": "b", "roles": "*", "methods": "read"},
		{"pattern": "a", "roles": "*", "methods": "read"},
	}
	h, err := Digest(rules[0])
	if err != nil {
		t.Fatal(err)
	}
	idxs, err := FindRuleHashes(rules, h)
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 2 || idxs[0] != 0 || idxs[1] != 2 {
		t.Fatalf("full hash idxs = %v", idxs)
	}
	idxs, err = FindRuleHashes(rules, h[:8])
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 2 {
		t.Fatalf("prefix idxs = %v", idxs)
	}
}

func TestDigestOfEncodedJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"methods":"read","pattern":"info/*","roles":"*","actions":"*"}`)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	h1, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := DecodeAccessRule(m)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := AccessRuleHash(*rule)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("decode+encode changed hash: %s vs %s", h1, h2)
	}
}
