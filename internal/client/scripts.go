package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CanonicalContext maps the friendly alias AM accepts on write onto the
// value it actually stores. Re-read after create; never trust the request
// form. See docs/api/04-scripts.md.
func CanonicalContext(context string) string {
	if context == "SCRIPTED_DECISION_NODE" {
		return "AUTHENTICATION_TREE_DECISION_NODE"
	}
	return context
}

type Script struct {
	ID               string
	Name             string
	Description      string
	Context          string
	Language         string
	EvaluatorVersion string
	Source           string // plaintext, never base64
	Default          bool
}

type scriptWire struct {
	ID               string `json:"_id,omitempty"`
	Name             string `json:"name"`
	Description      any    `json:"description,omitempty"`
	Script           string `json:"script"`
	Language         string `json:"language"`
	Context          string `json:"context"`
	EvaluatorVersion string `json:"evaluatorVersion,omitempty"`
	Default          bool   `json:"default"`
}

func decodeScript(raw map[string]any) (*Script, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var w scriptWire
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	src, err := DecodeScriptBody(w.Script)
	if err != nil {
		return nil, err
	}
	s := &Script{
		ID:               stringField(raw, "_id"),
		Name:             w.Name,
		Description:      descriptionFrom(raw["description"]),
		Context:          w.Context,
		Language:         w.Language,
		EvaluatorVersion: w.EvaluatorVersion,
		Source:           src,
		Default:          w.Default,
	}
	if s.EvaluatorVersion == "" {
		s.EvaluatorVersion = "1.0"
	}
	return s, nil
}

func descriptionFrom(v any) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	if s == "null" {
		return ""
	}
	return s
}

func stringField(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	s, _ := m[k].(string)
	return s
}

func DecodeScriptBody(wire string) (string, error) {
	if wire == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(wire)
	if err != nil {
		// Some historical notes claimed the server might accept raw JS.
		// We still refuse to guess: a non-base64 body is an error so a
		// provider upgrade is forced if AM ever changes encoding.
		return "", fmt.Errorf("script body is not standard base64 (AIC encoding changed?): %w", err)
	}
	return string(raw), nil
}

func EncodeScriptBody(src string) string {
	return base64.StdEncoding.EncodeToString([]byte(src))
}

func (c *Client) GetScript(ctx context.Context, realm, id string) (*Script, error) {
	path := fmt.Sprintf("%s/scripts/%s", c.RealmPath(realm), url.PathEscape(id))
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return decodeScript(raw)
}

func (c *Client) FindScriptByName(ctx context.Context, realm, name string) (*Script, error) {
	filter := url.QueryEscape(fmt.Sprintf(`name eq "%s"`, name))
	path := fmt.Sprintf("%s/scripts?_queryFilter=%s", c.RealmPath(realm), filter)
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	if len(body.Result) == 0 {
		return nil, &APIError{Status: http.StatusNotFound, Body: "script name not found: " + name, Method: http.MethodGet, URL: path}
	}
	if len(body.Result) > 1 {
		return nil, fmt.Errorf("script name %q matched %d records in realm %s", name, len(body.Result), realm)
	}
	return decodeScript(body.Result[0])
}

func (c *Client) ListScripts(ctx context.Context, realm string) ([]Script, error) {
	path := fmt.Sprintf("%s/scripts?_queryFilter=true", c.RealmPath(realm))
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	out := make([]Script, 0, len(body.Result))
	for _, raw := range body.Result {
		s, err := decodeScript(raw)
		if err != nil {
			return nil, fmt.Errorf("decode script %v: %w", raw["_id"], err)
		}
		// Product-internal scripts 403 on a subsequent GET of the body and
		// are marked only by this name prefix (docs/api/04-scripts.md).
		if strings.HasPrefix(s.Name, "ForgeRock Internal:") {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func (c *Client) PutScript(ctx context.Context, realm, id string, s Script) (*Script, error) {
	path := fmt.Sprintf("%s/scripts/%s", c.RealmPath(realm), url.PathEscape(id))
	desc := any(nil)
	if s.Description != "" {
		desc = s.Description
	}
	body := map[string]any{
		"name":             s.Name,
		"context":          s.Context,
		"language":         s.Language,
		"script":           EncodeScriptBody(s.Source),
		"evaluatorVersion": s.EvaluatorVersion,
		"description":      desc,
	}
	req, err := c.NewRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, &APIError{Status: status, Body: string(raw), Method: http.MethodPut, URL: req.URL.String()}
	}
	// Re-read: create echo can carry a write-only _rev and context may have
	// been normalised. The subsequent GET is the source of truth.
	return c.GetScript(ctx, realm, id)
}

func (c *Client) DeleteScript(ctx context.Context, realm, id string) error {
	path := fmt.Sprintf("%s/scripts/%s", c.RealmPath(realm), url.PathEscape(id))
	req, err := c.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status != http.StatusOK {
		return &APIError{Status: status, Body: string(raw), Method: http.MethodDelete, URL: req.URL.String()}
	}
	return nil
}
