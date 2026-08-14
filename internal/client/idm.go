package client

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func (c *Client) NewIDMRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	// /openidm config does not want Accept-API-Version (docs/api/11-idm-endpoints.md).
	return c.NewRequestVersion(ctx, method, path, "", body)
}

func (c *Client) GetConfig(ctx context.Context, id string) (map[string]any, error) {
	req, err := c.NewIDMRequest(ctx, http.MethodGet, "/openidm/config/"+id, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PutConfig(ctx context.Context, id string, body map[string]any) (map[string]any, error) {
	req, err := c.NewIDMRequest(ctx, http.MethodPut, "/openidm/config/"+id, stripID(body))
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
	return c.GetConfig(ctx, id)
}

func (c *Client) DeleteConfig(ctx context.Context, id string) error {
	req, err := c.NewIDMRequest(ctx, http.MethodDelete, "/openidm/config/"+id, nil)
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

func (c *Client) ListConfigIDs(ctx context.Context, prefix string) ([]string, error) {
	req, err := c.NewIDMRequest(ctx, http.MethodGet, "/openidm/config?_queryFilter=true", nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	var ids []string
	for _, row := range body.Result {
		id, _ := row["_id"].(string)
		if prefix == "" || strings.HasPrefix(id, prefix) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func stripID(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k == "_id" {
			continue
		}
		out[k] = v
	}
	return out
}

func ConfigName(kind, id string) string {
	return strings.TrimPrefix(id, kind+"/")
}

func ConfigID(kind, name string) string {
	if strings.HasPrefix(name, kind+"/") {
		return name
	}
	return kind + "/" + name
}

func unknownKeys(raw map[string]any, known map[string]struct{}) []string {
	var unknown []string
	for k := range raw {
		if k == "_id" {
			continue
		}
		if _, ok := known[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	return unknown
}

func stringVal(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	s, _ := m[k].(string)
	return s
}

func boolVal(m map[string]any, k string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[k]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func intVal(m map[string]any, k string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	switch n := m[k].(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

func rejectUnknown(kind string, raw map[string]any, known map[string]struct{}) error {
	if u := unknownKeys(raw, known); len(u) > 0 {
		return fmt.Errorf("%s has unmodelled fields %v — add them to internal/client/idm.go", kind, u)
	}
	return nil
}

func asObject(v any) map[string]any {
	o, _ := v.(map[string]any)
	return o
}
