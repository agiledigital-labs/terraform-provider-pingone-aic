package client

import (
	"context"
	"fmt"
)

var endpointKeys = map[string]struct{}{
	"type": {}, "source": {}, "file": {}, "description": {},
	"context": {}, "globals": {}, "globalsObject": {},
}

var endpointSourceKeys = map[string]struct{}{
	"source": {}, "type": {},
}

type Endpoint struct {
	Name               string
	Type               string
	Source             string
	File               string
	Description        string
	Context            string
	GlobalsObject      string
	AllowedRoles       []string
	NestedSource       bool
	EmptyGlobalsObject bool
}

func DecodeEndpoint(raw map[string]any) (*Endpoint, error) {
	if err := rejectUnknown("endpoint", raw, endpointKeys); err != nil {
		return nil, err
	}
	id, err := strictString(raw, "_id")
	if err != nil {
		return nil, err
	}
	e := &Endpoint{
		Name: ConfigName("endpoint", id),
	}
	for key, dst := range map[string]*string{
		"type": &e.Type, "file": &e.File, "description": &e.Description, "context": &e.Context,
	} {
		*dst, err = strictString(raw, key)
		if err != nil {
			return nil, err
		}
	}
	e.Source, e.NestedSource, err = decodeEndpointSource(raw["source"])
	if err != nil {
		return nil, err
	}
	globalsObj, err := decodeGlobalsObject(raw["globalsObject"])
	if err != nil {
		return nil, err
	}
	e.GlobalsObject = globalsObj
	if globals, ok := raw["globalsObject"].(map[string]any); ok && len(globals) == 0 {
		e.EmptyGlobalsObject = true
	}
	if g, err := strictObject(raw["globals"], "endpoint.globals"); err != nil {
		return nil, err
	} else if g != nil {
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
		if e.NestedSource {
			body["source"] = map[string]any{"source": e.Source, "type": e.Type}
		} else {
			body["source"] = e.Source
		}
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
	} else if e.EmptyGlobalsObject {
		body["globalsObject"] = map[string]any{}
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

func decodeEndpointSource(v any) (string, bool, error) {
	if v == nil {
		return "", false, nil
	}
	if source, ok := v.(string); ok {
		return source, false, nil
	}
	o, err := strictObject(v, "endpoint.source")
	if err != nil {
		return "", false, err
	}
	if err := rejectUnknown("endpoint.source", o, endpointSourceKeys); err != nil {
		return "", false, err
	}
	source, err := strictString(o, "source")
	if err != nil {
		return "", false, err
	}
	if _, err := strictString(o, "type"); err != nil {
		return "", false, err
	}
	return source, true, nil
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
	cfg, err := strictObject(globals["endpointConfig"], "endpoint.globals.endpointConfig")
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	for k := range cfg {
		if k != "allowedRoles" {
			return nil, fmt.Errorf("endpoint globals.endpointConfig has unmodelled fields [%s]", k)
		}
	}
	v, exists := cfg["allowedRoles"]
	if !exists || v == nil {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("endpoint.globals.endpointConfig.allowedRoles is %T, want array", v)
	}
	out := make([]string, 0, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("endpoint.globals.endpointConfig.allowedRoles[%d] is %T, want string", i, item)
		}
		out = append(out, s)
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
