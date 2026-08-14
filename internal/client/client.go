// Package client is a thin AIC HTTP client for the Terraform provider.
// Paths, headers, and token behaviour follow pingone-aic-manager's verified
// docs (docs/api/00-auth.md, 01, 02, 04, 09).
package client

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AMAPIVersion       = "protocol=2.0,resource=1.0"
	OAuth2APIVersion   = "protocol=2.1,resource=1.0"
	ESVAPIVersion      = "resource=1.0"
	tokenTTLSkew       = 60 * time.Second
	assertionTTL       = 180 * time.Second
	defaultHTTPTimeout = 60 * time.Second
	saScopes           = "fr:idm:* fr:am:* fr:idc:esv:* fr:idc:cookie-domain:*"
)

// SuccessNodeID and FailureNodeID are AM's built-in static outcome nodes.
// They are not separately fetchable; trees just point at these UUIDs.
const (
	SuccessNodeID = "70e691a5-1e33-4ac3-a356-e7b6d60d92e0"
	FailureNodeID = "e301438c-0bd0-429c-ab0c-66126501069a"
)

type Config struct {
	TenantURL        string
	ServiceAccountID string
	JWK              string
	AccessToken      string
	ResourcePrefix   string
	HTTPClient       *http.Client
}

type Client struct {
	base        string
	saID        string
	key         *rsa.PrivateKey
	staticToken string
	Prefix      string
	http        *http.Client

	mu        sync.Mutex
	token     string
	tokenExp  time.Time
	managedMu sync.Mutex
	accessMu  sync.Mutex
	authMu    sync.Mutex
}

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(cfg.TenantURL, "/")
	if base == "" {
		return nil, fmt.Errorf("tenant_url is required")
	}
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		base = "https://" + base
	}

	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultHTTPTimeout}
	}

	c := &Client{
		base:        base,
		saID:        cfg.ServiceAccountID,
		staticToken: strings.TrimSpace(cfg.AccessToken),
		Prefix:      cfg.ResourcePrefix,
		http:        hc,
	}

	if c.staticToken == "" {
		if cfg.ServiceAccountID == "" || cfg.JWK == "" {
			return nil, fmt.Errorf("either access_token, or both service_account_id and jwk, is required")
		}
		key, err := parseRSAJWK(cfg.JWK)
		if err != nil {
			return nil, fmt.Errorf("parse service account jwk: %w", err)
		}
		c.key = key
	}
	return c, nil
}

func (c *Client) BaseURL() string { return c.base }

func (c *Client) RealmPath(realm string) string {
	return fmt.Sprintf("/am/json/realms/root/realms/%s", realm)
}

func (c *Client) TreesPath(realm string) string {
	return c.RealmPath(realm) + "/realm-config/authentication/authenticationtrees"
}

func (c *Client) NodesPath(realm string) string {
	return c.TreesPath(realm) + "/nodes"
}

func (c *Client) bearer(ctx context.Context) (string, error) {
	if c.staticToken != "" {
		return c.staticToken, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Add(tokenTTLSkew).Before(c.tokenExp) {
		return c.token, nil
	}

	assertion, err := c.signAssertion()
	if err != nil {
		return "", err
	}
	form := url.Values{
		"client_id":  {"service-account"},
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
		"scope":      {saScopes},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/am/oauth2/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := c.doJSON(req, http.StatusOK, &body); err != nil {
		return "", fmt.Errorf("mint access token: %w", err)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned empty access_token")
	}
	c.token = body.AccessToken
	exp := body.ExpiresIn
	if exp <= 0 {
		exp = 898
	}
	c.tokenExp = time.Now().Add(time.Duration(exp) * time.Second)
	return c.token, nil
}

func (c *Client) signAssertion() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": c.saID,
		"sub": c.saID,
		"aud": c.base + "/am/oauth2/access_token",
		"exp": now.Add(assertionTTL).Unix(),
		"jti": uuid.NewString(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(c.key)
}

func (c *Client) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	return c.NewRequestVersion(ctx, method, path, AMAPIVersion, body)
}

// NewRequestVersion is NewRequest with an explicit Accept-API-Version.
// OAuth2 clients and the OIDC provider service require protocol=2.1
// (see docs/api/02-headers-and-versioning.md).
func (c *Client) NewRequestVersion(ctx context.Context, method, path, version string, body any) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return nil, err
	}
	token, err := c.bearer(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if version != "" {
		req.Header.Set("Accept-API-Version", version)
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) Do(req *http.Request, want int, dest any) error {
	return c.doJSON(req, want, dest)
}

func (c *Client) DoStatus(req *http.Request) (int, []byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

func (c *Client) doJSON(req *http.Request, want int, dest any) error {
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return err
	}
	if status != want {
		return &APIError{Status: status, Body: string(raw), Method: req.Method, URL: req.URL.String()}
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode %s %s: %w\nbody: %s", req.Method, req.URL, err, truncate(raw, 400))
	}
	return nil
}

type APIError struct {
	Status int
	Body   string
	Method string
	URL    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("AIC %s %s: HTTP %d: %s", e.Method, e.URL, e.Status, truncate([]byte(e.Body), 500))
}

func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	// errors.As, not a type assertion: a 404 that any layer has wrapped with
	// %w must still read as "gone", or Read() reports an error instead of
	// dropping the resource from state.
	var as *APIError
	return errors.As(err, &as) && as.NotFound()
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

type rsaJWK struct {
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	D   string `json:"d"`
	P   string `json:"p"`
	Q   string `json:"q"`
	Dp  string `json:"dp"`
	Dq  string `json:"dq"`
	Qi  string `json:"qi"`
}

func parseRSAJWK(raw string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	var j rsaJWK
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		return nil, fmt.Errorf("jwk is not JSON: %w", err)
	}
	if !strings.EqualFold(j.Kty, "RSA") {
		return nil, fmt.Errorf("jwk kty %q is not RSA", j.Kty)
	}
	n, err := b64int(j.N)
	if err != nil {
		return nil, fmt.Errorf("jwk n: %w", err)
	}
	e, err := b64int(j.E)
	if err != nil {
		return nil, fmt.Errorf("jwk e: %w", err)
	}
	d, err := b64int(j.D)
	if err != nil {
		return nil, fmt.Errorf("jwk d: %w", err)
	}
	if e.BitLen() > 64 {
		return nil, fmt.Errorf("jwk e is too large")
	}
	key := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())},
		D:         d,
	}
	if j.P != "" && j.Q != "" {
		p, err := b64int(j.P)
		if err != nil {
			return nil, fmt.Errorf("jwk p: %w", err)
		}
		q, err := b64int(j.Q)
		if err != nil {
			return nil, fmt.Errorf("jwk q: %w", err)
		}
		key.Primes = []*big.Int{p, q}
		if j.Dp != "" {
			if dp, err := b64int(j.Dp); err == nil {
				key.Precomputed.Dp = dp
			}
		}
		if j.Dq != "" {
			if dq, err := b64int(j.Dq); err == nil {
				key.Precomputed.Dq = dq
			}
		}
		if j.Qi != "" {
			if qi, err := b64int(j.Qi); err == nil {
				key.Precomputed.Qinv = qi
			}
		}
		key.Precompute()
	}
	if err := key.Validate(); err != nil {
		return nil, fmt.Errorf("jwk does not form a valid RSA key: %w", err)
	}
	return key, nil
}

func b64int(s string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("empty")
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	return new(big.Int).SetBytes(raw), nil
}

func StripServerFields(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k == "_id" || k == "_rev" || k == "_type" || k == "_outcomes" {
			continue
		}
		out[k] = v
	}
	return out
}
