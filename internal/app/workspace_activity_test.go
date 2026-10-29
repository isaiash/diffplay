package app

import "testing"

func TestNamespaceActivityMaps(t *testing.T) {
	original := scanUpdate{modifications: map[string]int{"a": 2, "clean": 0, ".": 9}, previews: map[string]string{"a": "head diff", ".": "skip"}}
	up := namespaceUpdate("repo-2", original)
	if up.modifications["repo-2/a"] != 2 || up.modifications["repo-2/clean"] != 0 || len(up.modifications) != 2 || up.previews["repo-2/a"] != "head diff" || len(up.previews) != 1 {
		t.Fatalf("unnamespaced activity: %+v", up)
	}
	if original.modifications["a"] != 2 || original.previews["a"] != "head diff" {
		t.Fatal("mutated original update")
	}
}
