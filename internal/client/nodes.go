package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) GetNode(ctx context.Context, realm, nodeType, id string) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s/%s", c.NodesPath(realm), url.PathEscape(nodeType), url.PathEscape(id))
	req, err := c.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PutNode(ctx context.Context, realm, nodeType, id string, body map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s/%s", c.NodesPath(realm), url.PathEscape(nodeType), url.PathEscape(id))
	req, err := c.NewRequest(ctx, http.MethodPut, path, StripServerFields(body))
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
	var echoed map[string]any
	if len(raw) > 0 && json.Unmarshal(raw, &echoed) == nil && echoed["_id"] != "" {
		return echoed, nil
	}
	got, err := c.GetNode(ctx, realm, nodeType, id)
	if err != nil {
		return nil, fmt.Errorf("put %s/%s succeeded (HTTP %d) but re-read failed: %w", nodeType, id, status, err)
	}
	return got, nil
}

func (c *Client) DeleteNode(ctx context.Context, realm, nodeType, id string) error {
	path := fmt.Sprintf("%s/%s/%s", c.NodesPath(realm), url.PathEscape(nodeType), url.PathEscape(id))
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

func (c *Client) NodeSchema(ctx context.Context, realm, nodeType string) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s?_action=schema", c.NodesPath(realm), url.PathEscape(nodeType))
	req, err := c.NewRequest(ctx, http.MethodPost, path, map[string]any{})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) NodeTemplate(ctx context.Context, realm, nodeType string) (map[string]any, error) {
	path := fmt.Sprintf("%s/%s?_action=template", c.NodesPath(realm), url.PathEscape(nodeType))
	req, err := c.NewRequest(ctx, http.MethodPost, path, map[string]any{})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
