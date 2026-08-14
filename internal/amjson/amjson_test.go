package amjson

import "testing"

func TestInt64AcceptsEveryNumericEncoding(t *testing.T) {
	tests := map[string]struct {
		in     any
		want   int64
		wantOK bool
	}{
		"json number": {float64(42), 42, true},
		"int64":       {int64(42), 42, true},
		"int":         {int(42), 42, true},
		"absent":      {nil, 0, false},
		"string":      {"42", 0, false},
		"bool":        {true, 0, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := Int64(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("Int64(%#v) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestOptionalInt64IsNilOnlyWhenAbsent(t *testing.T) {
	if got := OptionalInt64(float64(0)); got == nil || *got != 0 {
		t.Fatalf("got %#v, want a pointer to 0 — a present zero is not absent", got)
	}
	if got := OptionalInt64(nil); got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestBoolFallsBackWhenKeyAbsent(t *testing.T) {
	if !Bool(nil, true) || Bool(nil, false) {
		t.Fatal("absent key should yield the default")
	}
	if Bool(false, true) {
		t.Fatal("present false should beat a true default")
	}
}

func TestStringAtToleratesMissingAndMistypedKeys(t *testing.T) {
	m := map[string]any{"a": "x", "n": float64(1)}
	if got := StringAt(m, "a"); got != "x" {
		t.Fatalf("got %q, want %q", got, "x")
	}
	if got := StringAt(m, "n"); got != "" {
		t.Fatalf("non-string should decode to empty, got %q", got)
	}
	if got := StringAt(m, "missing"); got != "" {
		t.Fatalf("missing should decode to empty, got %q", got)
	}
}
