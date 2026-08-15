package resources

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestModelToTreePreservesTimeoutSettings(t *testing.T) {
	plan := journeyModel{
		Realm:              types.StringValue("alpha"),
		Enabled:            types.BoolValue(true),
		InnerTreeOnly:      types.BoolValue(false),
		MustRun:            types.BoolValue(false),
		NoSession:          types.BoolValue(false),
		TransactionalOnly:  types.BoolValue(false),
		MaximumIdleTime:    types.Int64Value(10),
		MaximumSessionTime: types.Int64Value(20),
		TreeTimeout:        types.Int64Value(30),
		EntryNode:          types.StringValue("entry"),
		Categories:         types.ListNull(types.StringType),
	}

	body, err := modelToTree(plan, "")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int64{
		"maximumIdleTime": 10, "maximumSessionTime": 20, "treeTimeout": 30,
	} {
		if got := body[key]; got != want {
			t.Errorf("%s = %#v, want %d", key, got, want)
		}
	}
}

func TestTreeToModelPreservesTimeoutSettings(t *testing.T) {
	raw := map[string]any{
		"_id": "Tree", "enabled": true, "entryNodeId": "entry",
		"identityResource": "managed/alpha_user", "innerTreeOnly": false,
		"mustRun": false, "noSession": false, "transactionalOnly": false,
		"maximumIdleTime": float64(10), "maximumSessionTime": float64(20),
		"treeTimeout": float64(30), "nodes": map[string]any{},
		"staticNodes": map[string]any{}, "uiConfig": map[string]any{},
	}
	plan := journeyModel{Realm: types.StringValue("alpha")}

	got, err := treeToModel(raw, plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.MaximumIdleTime.ValueInt64() != 10 || got.MaximumSessionTime.ValueInt64() != 20 || got.TreeTimeout.ValueInt64() != 30 {
		t.Fatalf("timeouts not preserved: %#v", got)
	}
}

// write() has to consult remoteName, or an Update after a resource_prefix
// change PUTs a second tree and orphans the one Read and Delete still address.
func TestJourneyWriteTargetsPersistedTreeAfterPrefixChange(t *testing.T) {
	var puts []string
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			puts = append(puts, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"_id":"OldPrefix_Tree","enabled":true,"entryNodeId":"entry",` +
					`"identityResource":"managed/alpha_user","nodes":{},` +
					`"staticNodes":{},"uiConfig":{}}`)),
			Header: make(http.Header),
		}, nil
	})
	c, err := client.New(client.Config{
		TenantURL: "https://tenant.example", AccessToken: "token",
		ResourcePrefix: "NewPrefix_", HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatal(err)
	}

	r := &journeyResource{client: c}
	prior := journeyModel{
		ID:         types.StringValue("OldPrefix_Tree"),
		RemoteName: types.StringValue("OldPrefix_Tree"),
		Name:       types.StringValue("Tree"),
		Realm:      types.StringValue("alpha"),
	}
	plan := journeyModel{
		Name: types.StringValue("Tree"), Realm: types.StringValue("alpha"),
		Enabled: types.BoolValue(true), InnerTreeOnly: types.BoolValue(false),
		MustRun: types.BoolValue(false), NoSession: types.BoolValue(false),
		TransactionalOnly: types.BoolValue(false),
		EntryNode:         types.StringValue("entry"),
		Categories:        types.ListNull(types.StringType),
	}
	if _, err := r.write(context.Background(), plan, prior); err != nil {
		t.Fatal(err)
	}

	if len(puts) != 1 {
		t.Fatalf("expected one PUT, got %v", puts)
	}
	if strings.Contains(puts[0], "NewPrefix_Tree") {
		t.Fatalf("Update wrote to %q — that creates a second tree and orphans OldPrefix_Tree", puts[0])
	}
	if !strings.HasSuffix(puts[0], "OldPrefix_Tree") {
		t.Fatalf("Update wrote to %q, want the tree Read/Delete address (OldPrefix_Tree)", puts[0])
	}
}

// Create has no prior state, so it must still honour the current prefix.
func TestJourneyWriteUsesPrefixedNameOnCreate(t *testing.T) {
	var puts []string
	httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			puts = append(puts, req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"_id":"NewPrefix_Tree","enabled":true,"entryNodeId":"entry",` +
					`"identityResource":"managed/alpha_user","nodes":{},` +
					`"staticNodes":{},"uiConfig":{}}`)),
			Header: make(http.Header),
		}, nil
	})
	c, err := client.New(client.Config{
		TenantURL: "https://tenant.example", AccessToken: "token",
		ResourcePrefix: "NewPrefix_", HTTPClient: httpClient,
	})
	if err != nil {
		t.Fatal(err)
	}

	r := &journeyResource{client: c}
	plan := journeyModel{
		Name: types.StringValue("Tree"), Realm: types.StringValue("alpha"),
		Enabled: types.BoolValue(true), InnerTreeOnly: types.BoolValue(false),
		MustRun: types.BoolValue(false), NoSession: types.BoolValue(false),
		TransactionalOnly: types.BoolValue(false),
		EntryNode:         types.StringValue("entry"),
		Categories:        types.ListNull(types.StringType),
	}
	if _, err := r.write(context.Background(), plan, journeyModel{}); err != nil {
		t.Fatal(err)
	}
	if len(puts) != 1 || !strings.HasSuffix(puts[0], "NewPrefix_Tree") {
		t.Fatalf("create wrote to %v, want the prefixed plan name", puts)
	}
}

// AM omits version on default nodes; the schema defaults it to "1.0". If decode
// disagreed, Terraform would abort with "inconsistent result after apply".
func TestTreeToModelDefaultsNodeVersionWhenAMOmitsIt(t *testing.T) {
	raw := map[string]any{
		"_id": "Tree", "enabled": true, "entryNodeId": "n1",
		"identityResource": "managed/alpha_user",
		"nodes": map[string]any{
			"n1": map[string]any{
				"nodeType":    "DecisionNode",
				"connections": map[string]any{"true": client.SuccessNodeID},
			},
		},
		"staticNodes": map[string]any{}, "uiConfig": map[string]any{},
	}
	got, err := treeToModel(raw, journeyModel{Realm: types.StringValue("alpha")}, "")
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Nodes[0].Version.ValueString(); v != client.DefaultNodeVersion {
		t.Fatalf("node version = %q, want the schema default %q", v, client.DefaultNodeVersion)
	}
}
