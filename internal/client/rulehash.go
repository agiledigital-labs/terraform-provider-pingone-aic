package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// CanonicalJSON is the deterministic encoding the rule digest uses: object
// keys sorted, compact separators, no HTML escaping. It matches
// json.dumps(sort_keys=True, separators=(',',':')) and the sibling
// `aic access` digest (docs/api/19-config-access.md).
func CanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Digest is the lowercase hex SHA-256 of CanonicalJSON(v). Rules have no
// server id; Terraform uses this as the resource id.
func Digest(v any) (string, error) {
	raw, err := CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ShortDigest is the 8-character prefix `aic access list` prints.
func ShortDigest(v any) (string, error) {
	h, err := Digest(v)
	if err != nil {
		return "", err
	}
	return ShortHash(h), nil
}

func ShortHash(hash string) string {
	if len(hash) < 8 {
		return hash
	}
	return hash[:8]
}

// RuleObjects pulls a JSON array of objects from doc[key].
func RuleObjects(doc map[string]any, key string) ([]map[string]any, error) {
	raw, ok := doc[key].([]any)
	if !ok {
		return nil, fmt.Errorf("document has no %s array", key)
	}
	out := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] is not an object", key, i)
		}
		out = append(out, o)
	}
	return out, nil
}

func setRuleObjects(doc map[string]any, key string, rules []map[string]any) map[string]any {
	arr := make([]any, len(rules))
	for i, r := range rules {
		arr[i] = r
	}
	next := cloneMap(doc)
	next[key] = arr
	return next
}

// FindRuleHashes returns every index whose canonical digest equals hash
// (full hex or a unique prefix of at least 8 characters).
func FindRuleHashes(rules []map[string]any, hash string) ([]int, error) {
	want := strings.ToLower(strings.TrimSpace(hash))
	if want == "" {
		return nil, fmt.Errorf("empty rule hash")
	}
	var idxs []int
	for i, r := range rules {
		got, err := Digest(r)
		if err != nil {
			return nil, fmt.Errorf("hash rules[%d]: %w", i, err)
		}
		if got == want || (len(want) >= 8 && strings.HasPrefix(got, want)) {
			idxs = append(idxs, i)
		}
	}
	return idxs, nil
}

func removeFirstHash(rules []map[string]any, hash string) ([]map[string]any, int, error) {
	idxs, err := FindRuleHashes(rules, hash)
	if err != nil {
		return nil, 0, err
	}
	if len(idxs) == 0 {
		return rules, 0, nil
	}
	i := idxs[0]
	next := append([]map[string]any(nil), rules[:i]...)
	next = append(next, rules[i+1:]...)
	return next, len(idxs) - 1, nil
}

func countRuleHash(rules []map[string]any, hash string) (int, error) {
	idxs, err := FindRuleHashes(rules, hash)
	if err != nil {
		return 0, err
	}
	return len(idxs), nil
}

func requireStringField(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s is %T, want string", key, v)
	}
	return s, nil
}

func optionalStringField(m map[string]any, key string) (*string, error) {
	v, ok := m[key]
	if !ok {
		return nil, nil
	}
	if v == nil {
		s := ""
		return &s, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%s is %T, want string", key, v)
	}
	return &s, nil
}

func putOptionalString(body map[string]any, key string, v *string) {
	if v != nil {
		body[key] = *v
	}
}

func requireStringSlice(m map[string]any, key string) ([]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is %T, want array", key, v)
	}
	out := make([]string, 0, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] is %T, want string", key, i, item)
		}
		out = append(out, s)
	}
	return out, nil
}
