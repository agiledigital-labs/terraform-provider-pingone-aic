package client

import "testing"

func TestSetAndRemoveManagedObjectPreserveSiblings(t *testing.T) {
	doc := map[string]any{
		"_id": "managed",
		"objects": []any{
			map[string]any{"name": "keep"},
			map[string]any{"name": "test_from"},
		},
	}
	next, err := SetManagedObject(doc, map[string]any{"name": "Terraform_test_from", "schema": map[string]any{"type": "object"}})
	if err != nil {
		t.Fatal(err)
	}
	objs, _ := ManagedObjects(next)
	if len(objs) != 3 {
		t.Fatalf("len = %d", len(objs))
	}
	if _, ok, _ := FindManagedObject(next, "keep"); !ok {
		t.Fatal("lost sibling")
	}
	removed, err := RemoveManagedObject(next, "Terraform_test_from")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := FindManagedObject(removed, "Terraform_test_from"); ok {
		t.Fatal("still present")
	}
	if _, ok, _ := FindManagedObject(removed, "keep"); !ok {
		t.Fatal("remove dropped sibling")
	}
}

func TestConfirmManaged(t *testing.T) {
	content := map[string]any{"name": "x", "schema": map[string]any{"title": "new"}}
	doc := map[string]any{"objects": []any{content}}
	if err := confirmManaged(doc, []ManagedConfirm{{Name: "x"}}); err != nil {
		t.Fatal(err)
	}
	if err := confirmManaged(doc, []ManagedConfirm{{Name: "x", Content: content}}); err != nil {
		t.Fatal(err)
	}
	stale := map[string]any{"name": "x", "schema": map[string]any{"title": "old"}}
	if err := confirmManaged(doc, []ManagedConfirm{{Name: "x", Content: stale}}); err == nil {
		t.Fatal("expected stale content failure")
	}
	if err := confirmManaged(doc, []ManagedConfirm{{Name: "x", Absent: true}}); err == nil {
		t.Fatal("expected absent failure")
	}
}
