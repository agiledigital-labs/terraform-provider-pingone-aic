package client

import (
	"strings"
	"testing"
)

func validTreeInternals() map[string]any {
	return map[string]any{
		"uiConfig":    map[string]any{"categories": "[]"},
		"staticNodes": map[string]any{"success": map[string]any{"x": float64(1), "y": float64(2)}},
		"nodes": map[string]any{"node-id": map[string]any{
			"connections": map[string]any{"true": "success"},
			"displayName": "Decision", "nodeType": "DecisionNode",
			"version": "2.0", "x": float64(1), "y": float64(2),
		}},
	}
}

func TestValidateTreeInternalsAcceptsKnownShape(t *testing.T) {
	if err := validateTreeInternals(validTreeInternals()); err != nil {
		t.Fatal(err)
	}
}

// Shapes taken from a live tenant (2026-08-14, alpha realm, 35 trees). Rejecting
// any of these breaks generate and Read for a real journey, so they are
// regression cases rather than hypotheticals.
func TestValidateTreeInternalsAcceptsLiveShapes(t *testing.T) {
	tests := map[string]func(map[string]any){
		// 6 of 35 trees carry the editor's canvas layout alongside categories.
		"uiConfig.annotations": func(tree map[string]any) {
			tree["uiConfig"] = map[string]any{
				"categories":  "[]",
				"annotations": `{"forNodes":{},"structural":[]}`,
			}
		},
		// 3 of 35 trees have no staticNodes key at all.
		"staticNodes absent": func(tree map[string]any) { delete(tree, "staticNodes") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			tree := validTreeInternals()
			mutate(tree)
			if err := validateTreeInternals(tree); err != nil {
				t.Fatalf("rejected a shape AM really returns: %v", err)
			}
		})
	}
}

// The nested check is only reachable through TreeWriteBody, so callers cannot
// half-enforce the fail-closed contract by forgetting a second call.
func TestTreeWriteBodyRejectsNestedUnknownFields(t *testing.T) {
	tree := validTreeInternals()
	tree["nodes"].(map[string]any)["node-id"].(map[string]any)["newField"] = true
	if _, err := TreeWriteBody(tree); err == nil || !strings.Contains(err.Error(), "newField") {
		t.Fatalf("got %v, want unknown-field error", err)
	}
}

func TestValidateTreeInternalsRejectsUnknownNestedFields(t *testing.T) {
	tests := map[string]func(map[string]any){
		"uiConfig": func(tree map[string]any) { tree["uiConfig"].(map[string]any)["newField"] = true },
		"node": func(tree map[string]any) {
			tree["nodes"].(map[string]any)["node-id"].(map[string]any)["newField"] = true
		},
		"staticNode": func(tree map[string]any) {
			tree["staticNodes"].(map[string]any)["success"].(map[string]any)["newField"] = true
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			tree := validTreeInternals()
			mutate(tree)
			if err := validateTreeInternals(tree); err == nil || !strings.Contains(err.Error(), "newField") {
				t.Fatalf("got %v, want unknown-field error", err)
			}
		})
	}
}
