package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestConfirmedWriteRetryPolicy(t *testing.T) {
	instant := &Client{confirmDelay: func(int) time.Duration { return 0 }}

	t.Run("retries until confirm accepts", func(t *testing.T) {
		var writes int
		err := instant.confirmedWrite(context.Background(), "config/access write",
			func() (map[string]any, error) {
				writes++
				return map[string]any{"n": writes}, nil
			},
			func(got map[string]any) error {
				if got["n"] != 3 {
					return errors.New("still stale")
				}
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if writes != 3 {
			t.Fatalf("writes = %d, want 3", writes)
		}
	})

	t.Run("exhausts attempts and names the document", func(t *testing.T) {
		stale := errors.New("still stale")
		var writes int
		err := instant.confirmedWrite(context.Background(), "config/access write",
			func() (map[string]any, error) {
				writes++
				return map[string]any{}, nil
			},
			func(map[string]any) error { return stale },
		)
		if writes != 6 {
			t.Fatalf("writes = %d, want 6", writes)
		}
		if !errors.Is(err, stale) {
			t.Fatalf("err = %v, want wrapped still-stale", err)
		}
		msg := err.Error()
		for _, want := range []string{
			"config/access write was accepted but not persisted",
			"Q14",
		} {
			if !strings.Contains(msg, want) {
				t.Fatalf("error %q does not contain %q", msg, want)
			}
		}
	})

	t.Run("write errors are not retried", func(t *testing.T) {
		boom := errors.New("transport")
		var writes int
		err := instant.confirmedWrite(context.Background(), "config/access write",
			func() (map[string]any, error) {
				writes++
				return nil, boom
			},
			func(map[string]any) error {
				t.Fatal("confirm called after write error")
				return nil
			},
		)
		if writes != 1 || !errors.Is(err, boom) {
			t.Fatalf("writes=%d err=%v", writes, err)
		}
	})

	t.Run("cancelled context returns instead of sleeping", func(t *testing.T) {
		// Production first delay is 500ms. Cancel after the first stale
		// confirm so a missing ctx check would sleep that whole slot.
		c := &Client{}
		ctx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		err := c.confirmedWrite(ctx, "config/access write",
			func() (map[string]any, error) {
				cancel()
				return map[string]any{}, nil
			},
			func(map[string]any) error { return errors.New("still stale") },
		)
		elapsed := time.Since(start)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if elapsed > 200*time.Millisecond {
			t.Fatalf("slept %s; should have returned on cancel", elapsed)
		}
	})
}

func TestConfirmedWriteBackoffSchedule(t *testing.T) {
	c := &Client{}
	want := []time.Duration{
		500 * time.Millisecond,
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
	}
	for i, d := range want {
		if got := c.confirmRetryDelay(i); got != d {
			t.Fatalf("attempt %d delay = %s, want %s", i, got, d)
		}
	}
}
