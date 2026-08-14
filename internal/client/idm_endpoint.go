package client

import (
	"context"
	"fmt"
)

var endpointKeys = map[string]struct{}{
	"type": {}, "source": {}, "file": {}, "description": {},
	"context": {}, "globals": {}, "globalsObject": {},
}

type Endpoint struct {
	Name          string
	Type          string
	Source        string
	File          string
	Description   string
	Context       string
	GlobalsObject string
	AllowedRoles  []string
}

func DecodeEndpoint(raw map[string]any) (*Endpoint, error) {
	if err := rejectUnknown("endpoint", raw, endpointKeys); err != nil {
		return nil, err
	}
	id, _ := raw["_id"].(string)
	e := &Endpoint{
		Name:          ConfigName("endpoint", id),
		Type:          stringVal(raw, "type"),
		File:          stringVal(raw, "file"),
		Description:   stringVal(raw, "description"),
		Context:       stringVal(raw, "context"),
		Source:        decodeSource(raw["source"]),
		GlobalsObject: "",
	}
	globalsObj, err := decodeGlobalsObject(raw["globalsObject"])
	if err != nil {
		return nil, err
	}
	e.GlobalsObject = globalsObj
	if g := asObject(raw["globals"]); g != nil {
		roles, err := decodeAllowedRoles(g)
		if err != nil {
			return nil, err
		}
		e.AllowedRoles = roles
	}
	if e.Type == "" {
		e.Type = "text/javascript"
	}
	return e, nil
}

func EncodeEndpoint(e Endpoint) map[string]any {
	body := map[string]any{
		"type": e.Type,
	}
	if e.Source != "" {
		body["source"] = e.Source
	}
	if e.File != "" {
		body["file"] = e.File
	}
	if e.Description != "" {
		body["description"] = e.Description
	}
	if e.Context != "" {
		body["context"] = e.Context
	}
	if e.GlobalsObject != "" {
		body["globalsObject"] = e.GlobalsObject
	}
	if len(e.AllowedRoles) > 0 {
		body["globals"] = map[string]any{
			"endpointConfig": map[string]any{
				"allowedRoles": stringSliceAny(e.AllowedRoles),
			},
		}
	}
	return body
}

func (c *Client) ListEndpoints(ctx context.Context) ([]string, error) {
	ids, err := c.ListConfigIDs(ctx, "endpoint/")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, ConfigName("endpoint", id))
	}
	return names, nil
}

func (c *Client) GetEndpoint(ctx context.Context, name string) (*Endpoint, error) {
	raw, err := c.GetConfig(ctx, ConfigID("endpoint", name))
	if err != nil {
		return nil, err
	}
	return DecodeEndpoint(raw)
}

func (c *Client) PutEndpoint(ctx context.Context, name string, e Endpoint) (*Endpoint, error) {
	raw, err := c.PutConfig(ctx, ConfigID("endpoint", name), EncodeEndpoint(e))
	if err != nil {
		return nil, err
	}
	return DecodeEndpoint(raw)
}

func (c *Client) DeleteEndpoint(ctx context.Context, name string) error {
	return c.DeleteConfig(ctx, ConfigID("endpoint", name))
}

func decodeSource(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["source"].(string); ok {
			return s
		}
	}
	return ""
}

func decodeGlobalsObject(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case map[string]any:
		if len(t) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("endpoint globalsObject is a non-empty object — add it to internal/client/idm_endpoint.go")
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("endpoint globalsObject: unexpected %T", v)
	}
}

func decodeAllowedRoles(globals map[string]any) ([]string, error) {
	var unknown []string
	for k := range globals {
		if k != "endpointConfig" {
			unknown = append(unknown, "globals."+k)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("endpoint has unmodelled fields %v — add them to internal/client/idm_endpoint.go", unknown)
	}
	cfg := asObject(globals["endpointConfig"])
	if cfg == nil {
		return nil, nil
	}
	for k := range cfg {
		if k != "allowedRoles" {
			return nil, fmt.Errorf("endpoint globals.endpointConfig has unmodelled fields [%s]", k)
		}
	}
	arr, _ := cfg["allowedRoles"].([]any)
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, _ := item.(string)
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func stringSliceAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}
