package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeAllLiveEndpointFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "endpoints")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) == 0 {
		t.Fatal("no fixtures")
	}
	for _, e := range ents {
		raw := readJSONMap(t, filepath.Join(dir, e.Name()))
		got, err := DecodeEndpoint(raw)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if got.Name == "" {
			t.Fatalf("%s: empty name", e.Name())
		}
	}
}

func TestDecodeAllLiveScheduleFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "schedules")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		raw := readJSONMap(t, filepath.Join(dir, e.Name()))
		got, err := DecodeSchedule(raw)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if got.Name == "" || got.InvokeService == "" {
			t.Fatalf("%s: %#v", e.Name(), got)
		}
	}
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
	body := EncodeSchedule(*got)
	ic, _ := body["invokeContext"].(map[string]any)
	if _, ok := ic["task"]; !ok {
		t.Fatalf("taskscanner encode missing task: %#v", ic)
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
