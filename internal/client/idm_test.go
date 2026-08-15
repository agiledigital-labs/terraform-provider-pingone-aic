package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestIDMFixtureDecodeEncodePreservesRecursiveKeySet(t *testing.T) {
	cases := []struct {
		dir    string
		encode func(map[string]any) (map[string]any, error)
	}{
		{"endpoints", func(raw map[string]any) (map[string]any, error) {
			decoded, err := DecodeEndpoint(raw)
			if err != nil {
				return nil, err
			}
			return EncodeEndpoint(*decoded), nil
		}},
		{"schedules", func(raw map[string]any) (map[string]any, error) {
			decoded, err := DecodeSchedule(raw)
			if err != nil {
				return nil, err
			}
			return EncodeSchedule(*decoded), nil
		}},
		{"roles", func(raw map[string]any) (map[string]any, error) {
			decoded, err := DecodeRole(raw)
			if err != nil {
				return nil, err
			}
			return EncodeRole(*decoded)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.dir)
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) == 0 {
				t.Fatal("no fixtures")
			}
			for _, entry := range entries {
				t.Run(entry.Name(), func(t *testing.T) {
					raw := readJSONMap(t, filepath.Join(dir, entry.Name()))
					encoded, err := tc.encode(raw)
					if err != nil {
						t.Fatal(err)
					}
					want := recursiveKeySet(raw, true)
					got := recursiveKeySet(encoded, false)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("recursive key set\n got: %v\nwant: %v", got, want)
					}
				})
			}
		})
	}
}

// recursiveKeySet flattens every key path in a document so decode → encode can
// be compared for key-set equality: a key the decoder cannot carry is deleted
// from the tenant on the next apply, which is exactly the class of silent
// passthrough this provider exists to prevent.
//
// response marks the side that came off the wire, and only that side drops the
// three keys a request legitimately never carries: `_id` and `_rev` are server
// metadata, and `temporalConstraints` is stripped deliberately because IDM
// rejects it on write (see .ai/core.md). Every other difference is a finding.
func recursiveKeySet(v any, response bool) []string {
	var keys []string
	var walk func(any, string)
	walk = func(value any, path string) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if path == "" && (key == "_id" || key == "_rev" || response && key == "temporalConstraints") {
					continue
				}
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				keys = append(keys, childPath)
				walk(child, childPath)
			}
		case []any:
			for i, child := range value {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	walk(v, "")
	sort.Strings(keys)
	return keys
}

func TestDecodeEndpointNestedSourceAndAllowedRoles(t *testing.T) {
	got, err := DecodeEndpoint(map[string]any{
		"_id":  "endpoint/nested",
		"type": "text/javascript",
		"source": map[string]any{
			"type":   "text/javascript",
			"source": "return {};",
		},
		"globals": map[string]any{
			"endpointConfig": map[string]any{
				"allowedRoles": []any{"internal/role/openidm-authorized"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "return {};" {
		t.Fatalf("source = %q", got.Source)
	}
	if len(got.AllowedRoles) != 1 {
		t.Fatalf("roles = %#v", got.AllowedRoles)
	}
}

func TestDecodeEndpointRejectsUnknownField(t *testing.T) {
	_, err := DecodeEndpoint(map[string]any{"_id": "endpoint/x", "brandNew": true})
	if err == nil {
		t.Fatal("expected unknown field")
	}
}

func TestDecodeScheduleTaskscannerSource(t *testing.T) {
	raw := readJSONMap(t, filepath.Join("testdata", "schedules", "Test.json"))
	got, err := DecodeSchedule(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.InvokeService != "taskscanner" || got.Source == "" {
		t.Fatalf("%#v", got)
	}
	if got.ScanObject != "managed/alpha_user" {
		t.Fatalf("scan = %q", got.ScanObject)
	}
	if got.Globals == nil || len(got.Globals) != 0 {
		t.Fatalf("globals = %#v, want present empty map", got.Globals)
	}
	body := EncodeSchedule(*got)
	ic, _ := body["invokeContext"].(map[string]any)
	if _, ok := ic["task"]; !ok {
		t.Fatalf("taskscanner encode missing task: %#v", ic)
	}
}

func TestDecodeScheduleRejectsUnknownNestedField(t *testing.T) {
	_, err := DecodeSchedule(map[string]any{
		"invokeContext": map[string]any{
			"script": map[string]any{"source": "", "brandNew": true},
		},
	})
	if err == nil {
		t.Fatal("expected nested unknown field error")
	}
}

func TestDecodeScheduleDefaultsPersisted(t *testing.T) {
	got, err := DecodeSchedule(map[string]any{"_id": "schedule/x", "invokeService": "script"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Persisted {
		t.Fatal("persisted = false, want default true")
	}
}

func TestIDMDecodersRejectWrongTypes(t *testing.T) {
	for name, decode := range map[string]func() error{
		"schedule boolean": func() error { _, err := DecodeSchedule(map[string]any{"persisted": "true"}); return err },
		"schedule object":  func() error { _, err := DecodeSchedule(map[string]any{"invokeContext": []any{}}); return err },
		"endpoint string":  func() error { _, err := DecodeEndpoint(map[string]any{"source": true}); return err },
		"role string":      func() error { _, err := DecodeRole(map[string]any{"name": false}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode(); err == nil {
				t.Fatal("expected type error")
			}
		})
	}
}

func TestNewIDMRequestOmitsAPIVersion(t *testing.T) {
	c, err := New(Config{TenantURL: "https://tenant.example", AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewIDMRequest(context.Background(), "GET", "/openidm/config/endpoint/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Accept-API-Version"); got != "" {
		t.Fatalf("IDM request must omit Accept-API-Version, got %q", got)
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}
