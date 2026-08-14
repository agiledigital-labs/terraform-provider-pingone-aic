package oauth2client

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/prefix"
)

// Server-owned top-level keys. AM rejects these on PUT
// (`400 Invalid attribute specified.`).
var serverOwned = map[string]struct{}{
	"_id": {}, "_rev": {}, "_type": {}, "_provider": {},
}

// EmptySentinel is AM's literal for "this script/tree pointer is unset".
const EmptySentinel = "[Empty]"

// Values is the Terraform-facing view of a client: group TF name → field TF
// name → decoded Go value. Missing keys mean "use the catalog default".
type Values map[string]map[string]any

// Template returns the live-template body as raw (unwrapped) group objects.
// A create PUT of this shape was verified 201 on 2026-08-15.
func Template() map[string]any {
	out := make(map[string]any, len(groups))
	for _, g := range groups {
		group := make(map[string]any, len(g.Fields))
		for _, f := range g.Fields {
			group[f.APIName] = cloneDefault(f.Default)
		}
		out[g.APIName] = group
	}
	return out
}

// DecodeAPI turns a raw OAuth2 client GET body into Terraform-facing values.
// Unknown top-level keys and unknown fields inside a modelled group are
// errors. Missing fields become catalog defaults so Computed defaults
// round-trip when AM omits them.
func DecodeAPI(raw map[string]any, resourcePrefix string) (Values, error) {
	var unknown []string
	out := Values{}
	seen := map[string]struct{}{}

	for k, v := range raw {
		if _, skip := serverOwned[k]; skip {
			continue
		}
		if strings.HasSuffix(k, "-encrypted") {
			continue
		}
		group, ok := LookupGroup(k)
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		seen[k] = struct{}{}
		obj, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("OAuth2Client.%s: expected object, got %T", k, v)
		}
		decoded, err := decodeGroup(group, obj, resourcePrefix)
		if err != nil {
			return nil, err
		}
		out[group.TFName] = decoded
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("OAuth2Client returned unmodelled fields %v — add them to internal/oauth2client/catalog.go", unknown)
	}

	// Groups AM omitted still need defaults so schema Default values match.
	for _, g := range groups {
		if _, ok := out[g.TFName]; ok {
			continue
		}
		out[g.TFName] = defaultsFor(g)
	}
	return out, nil
}

func decodeGroup(g Group, raw map[string]any, resourcePrefix string) (map[string]any, error) {
	var unknown []string
	out := defaultsFor(g)
	for k, v := range raw {
		if strings.HasSuffix(k, "-encrypted") {
			continue
		}
		field, ok := g.FieldByAPI(k)
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		unwrapped, inherited, err := unwrap(field, v)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", g.APIName, k, err)
		}
		if inherited {
			// Fall through to the catalog default (already in out).
			continue
		}
		decoded, err := decodeValue(field, unwrapped, resourcePrefix)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", g.APIName, k, err)
		}
		out[field.TFName] = decoded
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%s returned unmodelled fields %v — add them to internal/oauth2client/catalog.go", g.APIName, unknown)
	}
	return out, nil
}

// unwrap peels AM's {inherited,value} wrapper. inherited=true means the
// caller should treat the field as unset (use the catalog default).
func unwrap(field Field, v any) (value any, inherited bool, err error) {
	if v == nil {
		return nil, false, nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return v, false, nil
	}
	_, hasInherited := obj["inherited"]
	_, hasValue := obj["value"]
	if !hasInherited && !hasValue {
		return nil, false, fmt.Errorf("unexpected object keys %v", sortedKeys(obj))
	}
	if !field.Wrapped {
		return nil, false, fmt.Errorf("did not expect inherited wrapper, got keys %v", sortedKeys(obj))
	}
	if inh, _ := obj["inherited"].(bool); inh {
		return nil, true, nil
	}
	return obj["value"], false, nil
}

func decodeValue(field Field, v any, resourcePrefix string) (any, error) {
	if v == nil {
		switch field.Kind {
		case KindString:
			return nil, nil
		case KindStringList:
			return []string{}, nil
		default:
			return cloneDefault(field.Default), nil
		}
	}
	switch field.Kind {
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		if field.Prefixed && s != "" && s != EmptySentinel {
			return prefix.Strip(resourcePrefix, s), nil
		}
		return s, nil
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case KindInt:
		n, ok := asInt64(v)
		if !ok {
			return nil, fmt.Errorf("expected number, got %T", v)
		}
		return n, nil
	case KindStringList:
		arr, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("expected array, got %T", v)
		}
		out := make([]string, 0, len(arr))
		for i, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("[%d]: expected string, got %T", i, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unhandled field kind %d", field.Kind)
	}
}

// EncodeAPI builds a PUT body from Terraform-facing values. Unset fields
// take catalog defaults so the body is a complete template overlay. Values
// are written raw (no inherited wrapper) — that is the shape AM accepted
// on create.
func EncodeAPI(tf Values, resourcePrefix string) (map[string]any, error) {
	out := Template()
	for groupTF, fields := range tf {
		var group Group
		found := false
		for _, g := range groups {
			if g.TFName == groupTF {
				group = g
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("OAuth2Client: unknown terraform group %q", groupTF)
		}
		rawGroup, _ := out[group.APIName].(map[string]any)
		for tfName, v := range fields {
			field, ok := group.FieldByTF(tfName)
			if !ok {
				return nil, fmt.Errorf("%s: unknown terraform attribute %q", group.APIName, tfName)
			}
			if field.Sensitive && isEmpty(v) {
				// Leave the template null. AM treats null userpassword as
				// "keep the existing secret" on update.
				continue
			}
			encoded, err := encodeValue(field, v, resourcePrefix)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", group.APIName, field.TFName, err)
			}
			rawGroup[field.APIName] = encoded
		}
	}
	return SanitizeWrite(out), nil
}

func encodeValue(field Field, v any, resourcePrefix string) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch field.Kind {
	case KindString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", v)
		}
		if field.Prefixed && s != "" && s != EmptySentinel {
			return prefix.Apply(resourcePrefix, s), nil
		}
		return s, nil
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", v)
		}
		return b, nil
	case KindInt:
		n, ok := asInt64(v)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", v)
		}
		return n, nil
	case KindStringList:
		switch t := v.(type) {
		case []string:
			out := make([]any, len(t))
			for i, s := range t {
				out[i] = s
			}
			return out, nil
		case []any:
			return t, nil
		default:
			return nil, fmt.Errorf("expected string list, got %T", v)
		}
	default:
		return nil, fmt.Errorf("unhandled field kind %d", field.Kind)
	}
}

// SanitizeWrite strips server-owned top-level keys and any `*-encrypted`
// field at any depth. Sending those back corrupts client secrets.
func SanitizeWrite(v any) map[string]any {
	cleaned := stripEncrypted(v)
	obj, ok := cleaned.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any, len(obj))
	for k, val := range obj {
		if _, skip := serverOwned[k]; skip {
			continue
		}
		if strings.HasSuffix(k, "-encrypted") {
			continue
		}
		out[k] = val
	}
	return out
}

func stripEncrypted(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if strings.HasSuffix(k, "-encrypted") {
				continue
			}
			out[k] = stripEncrypted(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = stripEncrypted(val)
		}
		return out
	default:
		return v
	}
}

// EqualDefault reports whether v is the catalog default (so generate can omit it).
func EqualDefault(field Field, v any) bool {
	if field.Default == nil {
		if field.Kind == KindStringList {
			switch t := v.(type) {
			case []string:
				return len(t) == 0
			case []any:
				return len(t) == 0
			case nil:
				return true
			}
		}
		if field.Kind == KindString {
			if v == nil {
				return true
			}
			s, ok := v.(string)
			return ok && s == ""
		}
		return v == nil
	}
	got, _ := json.Marshal(normalizeForCompare(v))
	want, _ := json.Marshal(normalizeForCompare(field.Default))
	return string(got) == string(want)
}

func defaultsFor(g Group) map[string]any {
	out := make(map[string]any, len(g.Fields))
	for _, f := range g.Fields {
		out[f.TFName] = decodeDefault(f)
	}
	return out
}

func decodeDefault(f Field) any {
	if f.Default == nil {
		if f.Kind == KindStringList {
			return []string{}
		}
		return nil
	}
	switch f.Kind {
	case KindStringList:
		switch t := f.Default.(type) {
		case []any:
			out := make([]string, 0, len(t))
			for _, item := range t {
				s, _ := item.(string)
				out = append(out, s)
			}
			return out
		case []string:
			return append([]string(nil), t...)
		}
	case KindInt:
		n, _ := asInt64(f.Default)
		return n
	}
	return cloneDefault(f.Default)
}

func cloneDefault(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func normalizeForCompare(v any) any {
	switch t := v.(type) {
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return v
	}
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
