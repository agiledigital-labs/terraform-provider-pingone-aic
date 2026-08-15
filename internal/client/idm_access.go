package client

import (
	"context"
	"fmt"
	"time"
)

const accessConfigID = "access"

var accessRuleKeys = map[string]struct{}{
	"pattern": {}, "roles": {}, "methods": {},
	"actions": {}, "customAuthz": {}, "excludePatterns": {},
}

// AccessRule is one configs[] grant in /openidm/config/access.
// There is no per-rule endpoint and no rule id; identity is Digest of the
// encoded object. Other rules in the document are left untouched.
type AccessRule struct {
	Pattern         string
	Roles           string
	Methods         string
	Actions         *string // nil omits the key; &"" is a live form
	CustomAuthz     *string
	ExcludePatterns *string
}

func DecodeAccessRule(raw map[string]any) (*AccessRule, error) {
	if err := rejectUnknown("access rule", raw, accessRuleKeys); err != nil {
		return nil, err
	}
	r := &AccessRule{}
	var err error
	if r.Pattern, err = requireStringField(raw, "pattern"); err != nil {
		return nil, err
	}
	if r.Roles, err = requireStringField(raw, "roles"); err != nil {
		return nil, err
	}
	if r.Methods, err = requireStringField(raw, "methods"); err != nil {
		return nil, err
	}
	if r.Actions, err = optionalStringField(raw, "actions"); err != nil {
		return nil, err
	}
	if r.CustomAuthz, err = optionalStringField(raw, "customAuthz"); err != nil {
		return nil, err
	}
	if r.ExcludePatterns, err = optionalStringField(raw, "excludePatterns"); err != nil {
		return nil, err
	}
	if r.Pattern == "" {
		return nil, fmt.Errorf("access rule pattern cannot be empty")
	}
	if r.Roles == "" {
		return nil, fmt.Errorf("access rule roles cannot be empty")
	}
	if r.Methods == "" {
		return nil, fmt.Errorf("access rule methods cannot be empty")
	}
	return r, nil
}

// EncodeAccessRule omits optional keys that were never set. Absent `actions`
// and `actions: ""` are different live forms — do not collapse them
// (docs/api/19-config-access.md).
func EncodeAccessRule(r AccessRule) map[string]any {
	body := map[string]any{
		"pattern": r.Pattern,
		"roles":   r.Roles,
		"methods": r.Methods,
	}
	putOptionalString(body, "actions", r.Actions)
	putOptionalString(body, "customAuthz", r.CustomAuthz)
	putOptionalString(body, "excludePatterns", r.ExcludePatterns)
	return body
}

func AccessRuleHash(r AccessRule) (string, error) {
	return Digest(EncodeAccessRule(r))
}

func AccessRules(doc map[string]any) ([]map[string]any, error) {
	return RuleObjects(doc, "configs")
}

func SetAccessRules(doc map[string]any, rules []map[string]any) map[string]any {
	next := setRuleObjects(doc, "configs", rules)
	next["_id"] = accessConfigID
	return next
}

func (c *Client) GetAccess(ctx context.Context) (map[string]any, error) {
	return c.GetConfig(ctx, accessConfigID)
}

// RuleConfirm is what a whole-document rule write must observe on read-back.
// Count is the exact number of rules that must hash to Hash.
type RuleConfirm struct {
	Hash  string
	Count int
}

// MutateAccess serialises GET + mutate + confirmed PUT so two Terraform
// resources cannot lose each other's change. Unmanaged rules stay the
// same map values they arrived as — they are never decoded and re-encoded.
func (c *Client) MutateAccess(ctx context.Context, mutate func(map[string]any) (map[string]any, RuleConfirm, error)) error {
	c.accessMu.Lock()
	defer c.accessMu.Unlock()
	doc, err := c.GetAccess(ctx)
	if err != nil {
		return err
	}
	next, expect, err := mutate(doc)
	if err != nil {
		return err
	}
	return c.replaceAccessConfirmedLocked(ctx, next, expect)
}

func (c *Client) replaceAccessConfirmedLocked(ctx context.Context, doc map[string]any, expect RuleConfirm) error {
	if expect.Hash == "" {
		return fmt.Errorf("access write requires a rule-hash confirmation")
	}
	const attempts = 6
	var last error
	for i := 0; i < attempts; i++ {
		got, err := c.PutConfig(ctx, accessConfigID, doc)
		if err != nil {
			return err
		}
		if err := confirmAccess(got, expect); err == nil {
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
	return fmt.Errorf("config/access write was accepted but not persisted: %w", last)
}

func confirmAccess(doc map[string]any, expect RuleConfirm) error {
	rules, err := AccessRules(doc)
	if err != nil {
		return err
	}
	n, err := countRuleHash(rules, expect.Hash)
	if err != nil {
		return err
	}
	if n != expect.Count {
		return fmt.Errorf("expected %d access rule(s) with hash %s, found %d", expect.Count, ShortHash(expect.Hash), n)
	}
	return nil
}

func AppendAccessRule(doc map[string]any, rule AccessRule) (map[string]any, RuleConfirm, error) {
	encoded := EncodeAccessRule(rule)
	hash, err := Digest(encoded)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	rules, err := AccessRules(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	n, err := countRuleHash(rules, hash)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	if n > 0 {
		return nil, RuleConfirm{}, fmt.Errorf("access rule %s already exists; import it instead of creating a duplicate", ShortHash(hash))
	}
	return SetAccessRules(doc, append(rules, encoded)), RuleConfirm{Hash: hash, Count: 1}, nil
}

func ReplaceAccessRule(doc map[string]any, oldHash string, rule AccessRule) (map[string]any, RuleConfirm, error) {
	encoded := EncodeAccessRule(rule)
	newHash, err := Digest(encoded)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	rules, err := AccessRules(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	index, err := replacementRuleIndex(rules, oldHash, newHash, "access rule")
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	next := append([]map[string]any(nil), rules...)
	next[index] = encoded
	return SetAccessRules(doc, next), RuleConfirm{Hash: newHash, Count: 1}, nil
}

// RemoveAccessRule deletes the first rule matching hash. Its confirmation
// expects however many copies remain (zero if it was unique or already gone).
func RemoveAccessRule(doc map[string]any, hash string) (map[string]any, RuleConfirm, error) {
	rules, err := AccessRules(doc)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	next, remaining, err := removeFirstHash(rules, hash)
	if err != nil {
		return nil, RuleConfirm{}, err
	}
	return SetAccessRules(doc, next), RuleConfirm{Hash: hash, Count: remaining}, nil
}
