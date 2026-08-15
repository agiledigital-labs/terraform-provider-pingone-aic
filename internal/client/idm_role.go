package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	internalRoleAPIVersion = "resource=1.0"
	internalRolePath       = "/openidm/internal/role"
)

var roleKeys = map[string]struct{}{
	"name": {}, "description": {}, "privileges": {},
	"condition": {}, "temporalConstraints": {},
}

var privilegeKeys = map[string]struct{}{
	"name": {}, "path": {}, "actions": {},
	"permissions": {}, "accessFlags": {}, "filter": {},
}

var accessFlagKeys = map[string]struct{}{
	"attribute": {}, "readOnly": {},
}

// Role is one /openidm/internal/role/{id}. _id is chosen on create
// (PUT /{id}); the admin console's POST path yields a random UUID instead.
type Role struct {
	ID               string
	Rev              string
	Name             string
	Description      string
	Condition        string
	Privileges       []Privilege
	conditionPresent bool
}

type Privilege struct {
	Name        string
	Path        string
	Actions     []string
	Permissions []string
	AccessFlags []AccessFlag
	Filter      string
}

type AccessFlag struct {
	Attribute string
	ReadOnly  bool
}

func DecodeRole(raw map[string]any) (*Role, error) {
	if err := rejectUnknown("internal role", raw, roleKeys); err != nil {
		return nil, err
	}
	if err := rejectNonEmptyTemporal(raw["temporalConstraints"]); err != nil {
		return nil, err
	}
	id, err := strictString(raw, "_id")
	if err != nil {
		return nil, err
	}
	r := &Role{
		ID:               id,
		conditionPresent: hasKey(raw, "condition"),
	}
	for key, dst := range map[string]*string{"_rev": &r.Rev, "name": &r.Name, "description": &r.Description} {
		*dst, err = strictString(raw, key)
		if err != nil {
			return nil, err
		}
	}
	cond, err := optionalStringField(raw, "condition")
	if err != nil {
		return nil, err
	}
	if cond != nil {
		r.Condition = *cond
	}
	privs, err := decodePrivileges(raw["privileges"])
	if err != nil {
		return nil, err
	}
	r.Privileges = privs
	return r, nil
}

func decodePrivileges(v any) ([]Privilege, error) {
	if v == nil {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("privileges is %T, want array", v)
	}
	out := make([]Privilege, 0, len(arr))
	for i, item := range arr {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("privileges[%d] is not an object", i)
		}
		p, err := decodePrivilege(o)
		if err != nil {
			return nil, fmt.Errorf("privileges[%d]: %w", i, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func decodePrivilege(raw map[string]any) (Privilege, error) {
	if err := rejectUnknown("role privilege", raw, privilegeKeys); err != nil {
		return Privilege{}, err
	}
	p := Privilege{}
	var err error
	p.Name, err = strictString(raw, "name")
	if err != nil {
		return Privilege{}, err
	}
	p.Path, err = strictString(raw, "path")
	if err != nil {
		return Privilege{}, err
	}
	if _, ok := raw["actions"]; !ok {
		return Privilege{}, fmt.Errorf("actions is required")
	}
	if _, ok := raw["permissions"]; !ok {
		return Privilege{}, fmt.Errorf("permissions is required")
	}
	if p.Actions, err = requireStringSlice(raw, "actions"); err != nil {
		return Privilege{}, err
	}
	if p.Permissions, err = requireStringSlice(raw, "permissions"); err != nil {
		return Privilege{}, err
	}
	if p.Filter, err = requireStringField(raw, "filter"); err != nil {
		return Privilege{}, err
	}
	flags, err := decodeAccessFlags(raw["accessFlags"])
	if err != nil {
		return Privilege{}, err
	}
	if len(flags) == 0 {
		return Privilege{}, fmt.Errorf("accessFlags cannot be empty")
	}
	p.AccessFlags = flags
	if p.Name == "" {
		return Privilege{}, fmt.Errorf("privilege name cannot be empty")
	}
	if p.Path == "" {
		return Privilege{}, fmt.Errorf("privilege path cannot be empty")
	}
	return p, nil
}

func decodeAccessFlags(v any) ([]AccessFlag, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("accessFlags is %T, want array", v)
	}
	out := make([]AccessFlag, 0, len(arr))
	for i, item := range arr {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("accessFlags[%d] is not an object", i)
		}
		if err := rejectUnknown("accessFlag", o, accessFlagKeys); err != nil {
			return nil, err
		}
		attr, err := strictString(o, "attribute")
		if err != nil {
			return nil, fmt.Errorf("accessFlags[%d]: %w", i, err)
		}
		if attr == "" {
			return nil, fmt.Errorf("accessFlags[%d] attribute cannot be empty", i)
		}
		ro, ok, err := strictBool(o, "readOnly")
		if err != nil {
			return nil, fmt.Errorf("accessFlags[%d]: %w", i, err)
		}
		if !ok {
			return nil, fmt.Errorf("accessFlags[%d] readOnly is required", i)
		}
		out = append(out, AccessFlag{Attribute: attr, ReadOnly: ro})
	}
	return out, nil
}

func rejectNonEmptyTemporal(v any) error {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("temporalConstraints is %T, want array", v)
	}
	if len(arr) > 0 {
		return fmt.Errorf("internal role has non-empty temporalConstraints — add them to internal/client/idm_role.go")
	}
	return nil
}

// EncodeRole is the writable body: no _id, _rev, or temporalConstraints.
// Omitting privileges empties them on the server, so the key is always sent.
func EncodeRole(r Role) (map[string]any, error) {
	privs := make([]any, 0, len(r.Privileges))
	for i, p := range r.Privileges {
		enc, err := encodePrivilege(p)
		if err != nil {
			return nil, fmt.Errorf("privileges[%d]: %w", i, err)
		}
		privs = append(privs, enc)
	}
	body := map[string]any{
		"privileges": privs,
	}
	if r.Name != "" {
		body["name"] = r.Name
	}
	if r.Description != "" {
		body["description"] = r.Description
	}
	if r.Condition != "" {
		body["condition"] = r.Condition
	} else if r.conditionPresent {
		body["condition"] = nil
	}
	return body, nil
}

func hasKey(raw map[string]any, key string) bool {
	_, ok := raw[key]
	return ok
}

func encodePrivilege(p Privilege) (map[string]any, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	if len(p.AccessFlags) == 0 {
		return nil, fmt.Errorf("accessFlags cannot be empty")
	}
	if err := privilegeWriteFlags(p); err != nil {
		return nil, err
	}
	flags := make([]any, 0, len(p.AccessFlags))
	for _, f := range p.AccessFlags {
		if f.Attribute == "" {
			return nil, fmt.Errorf("accessFlag attribute cannot be empty")
		}
		flags = append(flags, map[string]any{
			"attribute": f.Attribute,
			"readOnly":  f.ReadOnly,
		})
	}
	actions := make([]any, len(p.Actions))
	for i, a := range p.Actions {
		actions[i] = a
	}
	perms := make([]any, len(p.Permissions))
	for i, perm := range p.Permissions {
		perms[i] = perm
	}
	body := map[string]any{
		"name":        p.Name,
		"path":        p.Path,
		"actions":     actions,
		"permissions": perms,
		"accessFlags": flags,
	}
	if p.Filter != "" {
		body["filter"] = p.Filter
	}
	return body, nil
}

func privilegeWriteFlags(p Privilege) error {
	needsWrite := false
	for _, perm := range p.Permissions {
		if perm != "VIEW" {
			needsWrite = true
			break
		}
	}
	if !needsWrite {
		return nil
	}
	for _, f := range p.AccessFlags {
		if !f.ReadOnly {
			return nil
		}
	}
	return fmt.Errorf("permissions %v require at least one access_flag with read_only = false", p.Permissions)
}

func (c *Client) ListRoles(ctx context.Context) ([]string, error) {
	req, err := c.NewRequestVersion(ctx, http.MethodGet, internalRolePath+"?_queryFilter=true&_fields=_id,name", internalRoleAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.Do(req, http.StatusOK, &body); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(body.Result))
	for _, row := range body.Result {
		id, _ := row["_id"].(string)
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (c *Client) GetRole(ctx context.Context, id string) (*Role, error) {
	req, err := c.NewRequestVersion(ctx, http.MethodGet, roleURL(id), internalRoleAPIVersion, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return DecodeRole(raw)
}

func (c *Client) PutRole(ctx context.Context, id, rev string, role Role) (*Role, error) {
	body, err := EncodeRole(role)
	if err != nil {
		return nil, err
	}
	req, err := c.NewRequestVersion(ctx, http.MethodPut, roleURL(id), internalRoleAPIVersion, body)
	if err != nil {
		return nil, err
	}
	if rev != "" {
		req.Header.Set("If-Match", rev)
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, &APIError{Status: status, Body: string(raw), Method: http.MethodPut, URL: req.URL.String()}
	}
	return c.GetRole(ctx, id)
}

func (c *Client) DeleteRole(ctx context.Context, id string) error {
	req, err := c.NewRequestVersion(ctx, http.MethodDelete, roleURL(id), internalRoleAPIVersion, nil)
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

func roleURL(id string) string {
	return internalRolePath + "/" + url.PathEscape(id)
}

func RoleLooksLikeUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}
