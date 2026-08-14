package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

// ESV IDs are tenant-global and must match this pattern. The provider
// prefix is inserted after "esv-" (see prefix.ApplyESV) so copies stay valid.
var esvIDRe = regexp.MustCompile(`^esv-[a-z0-9_-]{1,124}$`)

func ValidateESVID(id string) error {
	if !esvIDRe.MatchString(id) {
		return fmt.Errorf("esv id %q does not match ^esv-[a-z0-9_-]{1,124}$", id)
	}
	return nil
}

type Variable struct {
	ID             string
	Description    string
	ExpressionType string
	Value          string // plaintext; wire form is valueBase64
	Loaded         bool
	LastChangeDate string
	LastChangedBy  string
}

type Secret struct {
	ID                string
	Description       string
	Encoding          string
	UseInPlaceholders bool
	ActiveVersion     string
	LoadedVersion     string
	Loaded            bool
	LastChangeDate    string
	LastChangedBy     string
}

type StartupStatus struct {
	RestartStatus string
}

type ESVPendingCount struct {
	Variables int
	Secrets   int
}

func (c *Client) ListVariables(ctx context.Context) ([]Variable, error) {
	raws, err := c.listESV(ctx, "/environment/variables")
	if err != nil {
		return nil, err
	}
	out := make([]Variable, 0, len(raws))
	for _, raw := range raws {
		v, err := decodeVariable(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, nil
}

func (c *Client) GetVariable(ctx context.Context, id string) (*Variable, error) {
	if err := ValidateESVID(id); err != nil {
		return nil, err
	}
	req, err := c.NewRequestVersion(ctx, http.MethodGet, "/environment/variables/"+url.PathEscape(id), ESVAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return decodeVariable(raw)
}

func (c *Client) PutVariable(ctx context.Context, id string, v Variable) (*Variable, error) {
	if err := ValidateESVID(id); err != nil {
		return nil, err
	}
	body := map[string]any{
		"valueBase64":    EncodeESVValue(v.Value),
		"expressionType": v.ExpressionType,
	}
	if v.Description != "" {
		body["description"] = v.Description
	}
	req, err := c.NewRequestVersion(ctx, http.MethodPut, "/environment/variables/"+url.PathEscape(id), ESVAPIVersion, body)
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
	// Re-read: create/update echo is usually complete, but loaded is the
	// field we must not invent, and a 200 body is not a durability guarantee
	// we want to trust over GET.
	return c.GetVariable(ctx, id)
}

func (c *Client) DeleteVariable(ctx context.Context, id string) error {
	if err := ValidateESVID(id); err != nil {
		return err
	}
	req, err := c.NewRequestVersion(ctx, http.MethodDelete, "/environment/variables/"+url.PathEscape(id), ESVAPIVersion, nil)
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

func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	raws, err := c.listESV(ctx, "/environment/secrets")
	if err != nil {
		return nil, err
	}
	out := make([]Secret, 0, len(raws))
	for _, raw := range raws {
		s, err := decodeSecret(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, nil
}

func (c *Client) GetSecret(ctx context.Context, id string) (*Secret, error) {
	if err := ValidateESVID(id); err != nil {
		return nil, err
	}
	req, err := c.NewRequestVersion(ctx, http.MethodGet, "/environment/secrets/"+url.PathEscape(id), ESVAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return decodeSecret(raw)
}

// CreateSecret is create-only. PUT on an existing id returns 400.
func (c *Client) CreateSecret(ctx context.Context, id string, s Secret, value string) (*Secret, error) {
	if err := ValidateESVID(id); err != nil {
		return nil, err
	}
	body := map[string]any{
		"encoding":          s.Encoding,
		"useInPlaceholders": s.UseInPlaceholders,
		"valueBase64":       EncodeESVValue(value),
	}
	if s.Description != "" {
		body["description"] = s.Description
	}
	req, err := c.NewRequestVersion(ctx, http.MethodPut, "/environment/secrets/"+url.PathEscape(id), ESVAPIVersion, body)
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
	return c.GetSecret(ctx, id)
}

func (c *Client) CreateSecretVersion(ctx context.Context, id, value string) (*Secret, error) {
	if err := ValidateESVID(id); err != nil {
		return nil, err
	}
	path := "/environment/secrets/" + url.PathEscape(id) + "/versions?_action=create"
	req, err := c.NewRequestVersion(ctx, http.MethodPost, path, ESVAPIVersion, map[string]any{
		"valueBase64": EncodeESVValue(value),
	})
	if err != nil {
		return nil, err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, &APIError{Status: status, Body: string(raw), Method: http.MethodPost, URL: req.URL.String()}
	}
	return c.GetSecret(ctx, id)
}

func (c *Client) SetSecretDescription(ctx context.Context, id, description string) error {
	if err := ValidateESVID(id); err != nil {
		return err
	}
	path := "/environment/secrets/" + url.PathEscape(id) + "?_action=setDescription"
	req, err := c.NewRequestVersion(ctx, http.MethodPost, path, ESVAPIVersion, map[string]any{
		"description": description,
	})
	if err != nil {
		return err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return err
	}
	// Success is 200 with an empty body.
	if status != http.StatusOK {
		return &APIError{Status: status, Body: string(raw), Method: http.MethodPost, URL: req.URL.String()}
	}
	return nil
}

func (c *Client) DeleteSecret(ctx context.Context, id string) error {
	if err := ValidateESVID(id); err != nil {
		return err
	}
	req, err := c.NewRequestVersion(ctx, http.MethodDelete, "/environment/secrets/"+url.PathEscape(id), ESVAPIVersion, nil)
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

func (c *Client) GetStartup(ctx context.Context) (*StartupStatus, error) {
	req, err := c.NewRequestVersion(ctx, http.MethodGet, "/environment/startup", ESVAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var body StartupStatus
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	return &body, nil
}

func (c *Client) PendingESVCount(ctx context.Context) (*ESVPendingCount, error) {
	req, err := c.NewRequestVersion(ctx, http.MethodGet, "/environment/count?_onlyPending=true", ESVAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Variables int `json:"variables"`
		Secrets   int `json:"secrets"`
	}
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	return &ESVPendingCount{Variables: body.Variables, Secrets: body.Secrets}, nil
}

func (c *Client) listESV(ctx context.Context, base string) ([]map[string]any, error) {
	var out []map[string]any
	cookie := ""
	for {
		q := url.Values{"_pageSize": {"100"}}
		if cookie != "" {
			q.Set("_pagedResultsCookie", cookie)
		}
		req, err := c.NewRequestVersion(ctx, http.MethodGet, base+"?"+q.Encode(), ESVAPIVersion, nil)
		if err != nil {
			return nil, err
		}
		var body struct {
			Result             []map[string]any `json:"result"`
			PagedResultsCookie string           `json:"pagedResultsCookie"`
		}
		if err := c.Do(req, http.StatusOK, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Result...)
		if body.PagedResultsCookie == "" {
			break
		}
		cookie = body.PagedResultsCookie
	}
	return out, nil
}

func decodeVariable(raw map[string]any) (*Variable, error) {
	id, _ := raw["_id"].(string)
	enc, _ := raw["valueBase64"].(string)
	val, err := DecodeESVValue(enc)
	if err != nil {
		return nil, fmt.Errorf("variable %s: %w", id, err)
	}
	expr, _ := raw["expressionType"].(string)
	if expr == "" {
		expr = "string"
	}
	loaded, _ := raw["loaded"].(bool)
	desc, _ := raw["description"].(string)
	changed, _ := raw["lastChangeDate"].(string)
	by, _ := raw["lastChangedBy"].(string)
	return &Variable{
		ID:             id,
		Description:    desc,
		ExpressionType: expr,
		Value:          val,
		Loaded:         loaded,
		LastChangeDate: changed,
		LastChangedBy:  by,
	}, nil
}

func decodeSecret(raw map[string]any) (*Secret, error) {
	id, _ := raw["_id"].(string)
	enc, _ := raw["encoding"].(string)
	if enc == "" {
		enc = "generic"
	}
	ph, _ := raw["useInPlaceholders"].(bool)
	desc, _ := raw["description"].(string)
	active, _ := raw["activeVersion"].(string)
	loadedV, _ := raw["loadedVersion"].(string)
	loaded, _ := raw["loaded"].(bool)
	changed, _ := raw["lastChangeDate"].(string)
	by, _ := raw["lastChangedBy"].(string)
	return &Secret{
		ID:                id,
		Description:       desc,
		Encoding:          enc,
		UseInPlaceholders: ph,
		ActiveVersion:     active,
		LoadedVersion:     loadedV,
		Loaded:            loaded,
		LastChangeDate:    changed,
		LastChangedBy:     by,
	}, nil
}

func EncodeESVValue(plain string) string {
	return base64.StdEncoding.EncodeToString([]byte(plain))
}

func DecodeESVValue(wire string) (string, error) {
	if wire == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(wire)
	if err != nil {
		return "", fmt.Errorf("valueBase64 is not standard base64: %w", err)
	}
	return string(raw), nil
}
