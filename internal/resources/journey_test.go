package resources

import (
	"testing"

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

func TestJourneyRemoteNameUsesPersistedIDAcrossPrefixChanges(t *testing.T) {
	state := journeyModel{
		ID:         types.StringValue("OldPrefix_Tree"),
		RemoteName: types.StringValue("OldPrefix_Tree"),
		Name:       types.StringValue("Tree"),
	}
	if got := journeyRemoteName(state, "NewPrefix_"); got != "OldPrefix_Tree" {
		t.Fatalf("got %q, want persisted remote id", got)
	}
}

func TestJourneyRemoteNameFallsBackForImport(t *testing.T) {
	state := journeyModel{
		ID:         types.StringNull(),
		RemoteName: types.StringNull(),
		Name:       types.StringValue("Tree"),
	}
	if got := journeyRemoteName(state, "Prefix_"); got != "Prefix_Tree" {
		t.Fatalf("got %q, want reconstructed import name", got)
	}
}
