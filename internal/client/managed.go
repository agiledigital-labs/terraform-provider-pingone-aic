package client

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const managedConfigPath = "/openidm/config/managed"

// ManagedConfirm says what a config/managed write must observe before
// it is believed. A 200 on PUT is not evidence the change is stored.
type ManagedConfirm struct {
	Name   string
	Absent bool
}

func (c *Client) GetManaged(ctx context.Context) (map[string]any, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, managedConfigPath, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PutManaged(ctx context.Context, doc map[string]any) error {
	req, err := c.NewRequest(ctx, http.MethodPut, managedConfigPath, doc)
	if err != nil {
		return err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return &APIError{Status: status, Body: string(raw), Method: http.MethodPut, URL: req.URL.String()}
	}
	return nil
}

// MutateManaged serialises GET + mutate + confirmed PUT so two Terraform
// resources cannot lose each other's insert (the document is a single RMW).
func (c *Client) MutateManaged(ctx context.Context, mutate func(map[string]any) (map[string]any, []ManagedConfirm, error)) error {
	c.managedMu.Lock()
	defer c.managedMu.Unlock()
	doc, err := c.GetManaged(ctx)
	if err != nil {
		return err
	}
	next, expect, err := mutate(doc)
	if err != nil {
		return err
	}
	return c.replaceManagedConfirmedLocked(ctx, next, expect)
}

// ReplaceManagedConfirmed PUTs doc and re-reads until every condition
// holds, or fails after six attempts. Matches aic's replace_managed_confirmed
// (docs/api/10-managed-objects.md, Q14).
func (c *Client) ReplaceManagedConfirmed(ctx context.Context, doc map[string]any, expect []ManagedConfirm) error {
	c.managedMu.Lock()
	defer c.managedMu.Unlock()
	return c.replaceManagedConfirmedLocked(ctx, doc, expect)
}

func (c *Client) replaceManagedConfirmedLocked(ctx context.Context, doc map[string]any, expect []ManagedConfirm) error {
	if len(expect) == 0 {
		return fmt.Errorf("managed config write requires at least one confirmation condition")
	}
	const attempts = 6
	var last error
	for i := 0; i < attempts; i++ {
		if err := c.PutManaged(ctx, doc); err != nil {
			return err
		}
		got, err := c.GetManaged(ctx)
		if err != nil {
			return fmt.Errorf("re-read managed after write: %w", err)
		}
		if err := confirmManaged(got, expect); err == nil {
			return nil
		} else {
			last = err
		}
		if i+1 < attempts {
			delay := time.Duration(500*(1<<i)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return fmt.Errorf("managed config write was accepted but not persisted: %w; see docs/api/99-quirks-and-open-questions.md Q14", last)
}

func confirmManaged(doc map[string]any, expect []ManagedConfirm) error {
	objs, _ := doc["objects"].([]any)
	names := map[string]bool{}
	for _, raw := range objs {
		o, _ := raw.(map[string]any)
		if n, _ := o["name"].(string); n != "" {
			names[n] = true
		}
	}
	for _, e := range expect {
		has := names[e.Name]
		if e.Absent && has {
			return fmt.Errorf("expected managed object %q to be absent", e.Name)
		}
		if !e.Absent && !has {
			return fmt.Errorf("expected managed object %q to be present", e.Name)
		}
	}
	return nil
}

func ManagedObjects(doc map[string]any) ([]map[string]any, error) {
	raw, ok := doc["objects"].([]any)
	if !ok {
		return nil, fmt.Errorf("managed document has no objects array")
	}
	out := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("objects[%d] is not an object", i)
		}
		out = append(out, o)
	}
	return out, nil
}

func FindManagedObject(doc map[string]any, name string) (map[string]any, bool, error) {
	objs, err := ManagedObjects(doc)
	if err != nil {
		return nil, false, err
	}
	for _, o := range objs {
		if n, _ := o["name"].(string); n == name {
			return o, true, nil
		}
	}
	return nil, false, nil
}

// SetManagedObject replaces or appends an object by name and returns the
// mutated document. Other objects are preserved.
func SetManagedObject(doc map[string]any, obj map[string]any) (map[string]any, error) {
	name, _ := obj["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("managed object has no name")
	}
	objs, err := ManagedObjects(doc)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(objs)+1)
	replaced := false
	for _, o := range objs {
		if n, _ := o["name"].(string); n == name {
			out = append(out, obj)
			replaced = true
			continue
		}
		out = append(out, o)
	}
	if !replaced {
		out = append(out, obj)
	}
	next := cloneMap(doc)
	next["objects"] = out
	next["_id"] = "managed"
	return next, nil
}

func RemoveManagedObject(doc map[string]any, name string) (map[string]any, error) {
	objs, err := ManagedObjects(doc)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(objs))
	for _, o := range objs {
		if n, _ := o["name"].(string); n == name {
			continue
		}
		out = append(out, o)
	}
	next := cloneMap(doc)
	next["objects"] = out
	next["_id"] = "managed"
	return next, nil
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
