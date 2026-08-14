package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type Tree struct {
	Name              string
	Description       string
	Enabled           bool
	EntryNodeID       string
	IdentityResource  string
	InnerTreeOnly     bool
	MustRun           bool
	NoSession         bool
	TransactionalOnly bool
	Nodes             map[string]TreeNode
	StaticNodes       map[string]StaticNode
	UIConfig          map[string]any
	RawExtra          map[string]any // keys the provider does not model
}

type TreeNode struct {
	Connections map[string]string
	DisplayName string
	NodeType    string
	Version     string
	X           float64
	Y           float64
}

type StaticNode struct {
	X float64
	Y float64
}

func (c *Client) ListTrees(ctx context.Context, realm string) ([]string, error) {
	path := fmt.Sprintf("%s/trees?_queryFilter=true&_pageSize=1000", c.TreesPath(realm))
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
	names := make([]string, 0, len(body.Result))
	for _, t := range body.Result {
		if id, _ := t["_id"].(string); id != "" {
			names = append(names, id)
		}
	}
	return names, nil
}

func (c *Client) GetTree(ctx context.Context, realm, name string) (map[string]any, error) {
	path := fmt.Sprintf("%s/trees/%s", c.TreesPath(realm), url.PathEscape(name))
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

func (c *Client) PutTree(ctx context.Context, realm, name string, body map[string]any) (map[string]any, error) {
	path := fmt.Sprintf("%s/trees/%s", c.TreesPath(realm), url.PathEscape(name))
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
	return c.GetTree(ctx, realm, name)
}

func (c *Client) DeleteTree(ctx context.Context, realm, name string) error {
	path := fmt.Sprintf("%s/trees/%s", c.TreesPath(realm), url.PathEscape(name))
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

// known tree attributes AM accepts on PUT (docs/api/09-journeys.md).
var treeWriteAttrs = map[string]struct{}{
	"description": {}, "enabled": {}, "entryNodeId": {}, "identityResource": {},
	"innerTreeOnly": {}, "maximumIdleTime": {}, "maximumSessionTime": {},
	"mustRun": {}, "noSession": {}, "nodes": {}, "staticNodes": {},
	"transactionalOnly": {}, "treeTimeout": {}, "uiConfig": {},
}

func TreeWriteBody(raw map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(treeWriteAttrs))
	var unknown []string
	for k, v := range raw {
		if k == "_id" || k == "_rev" {
			continue
		}
		if _, ok := treeWriteAttrs[k]; !ok {
			unknown = append(unknown, k)
			continue
		}
		out[k] = v
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("tree has unmodelled attributes %v — add them to the provider (see internal/client/journeys.go treeWriteAttrs)", unknown)
	}
	return out, nil
}
