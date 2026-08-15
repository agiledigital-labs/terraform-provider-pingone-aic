package client

import (
	"context"
	"fmt"
	"time"
)

const authenticationConfigID = "authentication"

var authMappingKeys = map[string]struct{}{
	"subject": {}, "localUser": {}, "roles": {},
	"userRoles": {}, "executeAugmentationScript": {},
}

// AuthMapping is one rsFilter.staticUserMapping[] entry in
// /openidm/config/authentication. Identity is Digest of the encoded object;
// the rest of rsFilter (scopes, subjectMapping, anonymousUserMapping, …)
// is never rewritten.
type AuthMapping struct {
	Subject                   string
	LocalUser                 string
	Roles                     []string
	UserRoles                 string
	ExecuteAugmentationScript *bool
}

func DecodeAuthMapping(raw map[string]any) (*AuthMapping, error) {
	if err := rejectUnknown("authentication mapping", raw, authMappingKeys); err != nil {
		return nil, err
	}
	m := &AuthMapping{}
	var err error
	if m.Subject, err = requireStringField(raw, "subject"); err != nil {
		return nil, err
	}
	if m.LocalUser, err = requireStringField(raw, "localUser"); err != nil {
		return nil, err
	}
	if m.UserRoles, err = requireStringField(raw, "userRoles"); err != nil {
		return nil, err
	}
	if m.Roles, err = requireStringSlice(raw, "roles"); err != nil {
		return nil, err
	}
	if v, ok := raw["executeAugmentationScript"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("executeAugmentationScript is %T, want bool", v)
		}
		m.ExecuteAugmentationScript = &b
	}
	if m.Subject == "" {
		return nil, fmt.Errorf("authentication mapping subject cannot be empty")
	}
	if m.LocalUser == "" {
		return nil, fmt.Errorf("authentication mapping localUser cannot be empty")
	}
	return m, nil
}

func EncodeAuthMapping(m AuthMapping) map[string]any {
	body := map[string]any{
		"subject":   m.Subject,
		"localUser": m.LocalUser,
	}
	if len(m.Roles) > 0 {
		body["roles"] = stringSliceAny(m.Roles)
	}
	if m.UserRoles != "" {
		body["userRoles"] = m.UserRoles
	}
	if m.ExecuteAugmentationScript != nil {
		body["executeAugmentationScript"] = *m.ExecuteAugmentationScript
	}
	return body
}

func AuthMappingHash(m AuthMapping) (string, error) {
	return Digest(EncodeAuthMapping(m))
}

func AuthMappings(doc map[string]any) ([]map[string]any, error) {
	rs := asObject(doc["rsFilter"])
	if rs == nil {
		return nil, fmt.Errorf("config/authentication has no rsFilter object")
	}
	return RuleObjects(rs, "staticUserMapping")
}

func SetAuthMappings(doc map[string]any, mappings []map[string]any) (map[string]any, error) {
	rs := asObject(doc["rsFilter"])
	if rs == nil {
		return nil, fmt.Errorf("config/authentication has no rsFilter object")
	}
	next := cloneMap(doc)
	rsNext := cloneMap(rs)
	arr := make([]any, len(mappings))
	for i, m := range mappings {
		arr[i] = m
	}
	rsNext["staticUserMapping"] = arr
	next["rsFilter"] = rsNext
	next["_id"] = authenticationConfigID
	return next, nil
}

func (c *Client) GetAuthentication(ctx context.Context) (map[string]any, error) {
	return c.GetConfig(ctx, authenticationConfigID)
}

func (c *Client) MutateAuthentication(ctx context.Context, mutate func(map[string]any) (map[string]any, RuleConfirm, error)) error {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	doc, err := c.GetAuthentication(ctx)
	if err != nil {
		return err
	}
	next, expect, err := mutate(doc)
	if err != nil {
		return err
	}
	return c.replaceAuthConfirmedLocked(ctx, next, expect)
}

func (c *Client) replaceAuthConfirmedLocked(ctx context.Context, doc map[string]any, expect RuleConfirm) error {
	if expect.Hash == "" {
		return fmt.Errorf("authentication write requires a mapping-hash confirmation")
	}
	const attempts = 6
	var last error
	for i := 0; i < attempts; i++ {
		got, err := c.PutConfig(ctx, authenticationConfigID, doc)
		if err != nil {
			return err
		}
		if err := confirmAuth(got, expect); err == nil {
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
	return fmt.Errorf("config/authentication write was accepted but not persisted: %w", last)
}

func confirmAuth(doc map[string]any, expect RuleConfirm) error {
	maps, err := AuthMappings(doc)
	if err != nil {
		return err
	}
	n, err := countRuleHash(maps, expect.Hash)
	if err != nil {
		return err
	}
	if n != expect.Count {
		return fmt.Errorf("expected %d authentication mapping(s) with hash %s, found %d", expect.Count, ShortHash(expect.Hash), n)
	}
	return nil
}

func AppendAuthMapping(doc map[string]any, mapping AuthMapping) (map[string]any, RuleConfirm, error) {
	encoded := EncodeAuthMapping(mapping)
	hash, err := Digest(encoded)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	maps, err := AuthMappings(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	n, err := countRuleHash(maps, hash)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	if n > 0 {
		return nil, RuleConfirm{}, fmt.Errorf("authentication mapping %s already exists; import it instead of creating a duplicate", ShortHash(hash))
	}
	next, err := SetAuthMappings(doc, append(maps, encoded))
	return next, RuleConfirm{Hash: hash, Count: 1}, err
}

func ReplaceAuthMapping(doc map[string]any, oldHash string, mapping AuthMapping) (map[string]any, RuleConfirm, error) {
	encoded := EncodeAuthMapping(mapping)
	newHash, err := Digest(encoded)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	maps, err := AuthMappings(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	index, err := replacementRuleIndex(maps, oldHash, newHash, "authentication mapping")
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	nextMaps := append([]map[string]any(nil), maps...)
	nextMaps[index] = encoded
	next, err := SetAuthMappings(doc, nextMaps)
	return next, RuleConfirm{Hash: newHash, Count: 1}, err
}

func RemoveAuthMapping(doc map[string]any, hash string) (map[string]any, RuleConfirm, error) {
	maps, err := AuthMappings(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	next, remaining, err := removeFirstHash(maps, hash)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	out, err := SetAuthMappings(doc, next)
	return out, RuleConfirm{Hash: hash, Count: remaining}, err
}
