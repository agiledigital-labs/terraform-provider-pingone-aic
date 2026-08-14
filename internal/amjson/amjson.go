// Package amjson decodes the loosely-typed JSON that AM returns into plain Go
// values.
//
// AM sends numbers as float64 and omits keys that hold their default, so every
// caller needs the same handful of coercions. Both internal/resources and
// internal/generate read the same tree bodies, so the rules live here rather
// than being reimplemented either side of the provider/CLI split.
//
// These helpers coerce; they do not validate. Rejecting unmodelled keys is
// client.TreeWriteBody's job.
package amjson

// String returns v as a string, or "" if it is absent or not a string.
func String(v any) string {
	s, _ := v.(string)
	return s
}

// StringAt returns m[k] as a string, or "" if it is absent or not a string.
func StringAt(m map[string]any, k string) string {
	return String(m[k])
}

// Bool returns v as a bool, falling back to def when AM omitted the key.
func Bool(v any, def bool) bool {
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

// Int64 returns v as an int64. AM sends JSON numbers as float64, but accept the
// integer types too so callers can pass values that never went through
// encoding/json. ok is false when the key was absent or held a non-number.
func Int64(v any) (value int64, ok bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// OptionalInt64 returns a pointer to v as an int64, or nil when the key was
// absent. Callers that emit HCL use nil to mean "don't write this attribute".
func OptionalInt64(v any) *int64 {
	n, ok := Int64(v)
	if !ok {
		return nil
	}
	return &n
}
