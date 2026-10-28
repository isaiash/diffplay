package main

import (
	"testing"
	"time"
)

func TestAncestorFlashUsesLatestDescendantChangeKind(t *testing.T) {
	h := newHistory()
	now := time.Now()
	h.record("src/new.go", "created", false, "+new", 2, 0, true, now)
	h.record("src/removed.go", "deleted", false, "-gone", 0, 1, true, now.Add(time.Second))
	folder := h.tree().children[0]
	if folder.flashKind != "deleted" || !folder.changed.Equal(now.Add(time.Second)) {
		t.Fatalf("folder flash must follow latest deletion: %+v", folder)
	}
	h.record("src/new.go", "modified", false, "+edit", 1, 0, true, now.Add(2*time.Second))
	if folder = h.tree().children[0]; folder.flashKind != "modified" {
		t.Fatalf("new file edits must flash as edits: %+v", folder)
	}
}

func TestSeedActivityAndHidePathsThatBecomeIgnored(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	u.graph.upsert(".", "", true)
	u.graph.upsert("src/a.go", "src", false)
	u.graph.upsert("cache/temp.txt", "cache", false)
	u.recordEvents([]changeEvent{{Path: "src/a.go", Kind: "modified", Diff: "+seed", Added: 4, Removed: 1, StatsKnown: true, At: now}, {Path: "cache/temp.txt", Kind: "created", Added: 5, StatsKnown: true, At: now}})
	if u.history.tree().added != 9 {
		t.Fatal("initial Git counts must seed totals")
	}
	u.recordEvents([]changeEvent{{Path: "src/a.go", Kind: "modified", Added: 2, StatsKnown: true, At: now.Add(time.Second)}})
	if u.history.entries["src/a.go"].added != 6 {
		t.Fatal("live edits must build on Git baseline exactly once")
	}
	u.applyVisibility(map[string]bool{"src/a.go": true, "cache/temp.txt": false})
	if u.graph.nodes["cache/temp.txt"] != nil || u.history.entries["cache/temp.txt"] != nil {
		t.Fatal("newly ignored paths must disappear rather than become deletions")
	}
	if u.history.tree().added != 6 || len(u.history.tree().children) != 1 {
		t.Fatal("ignored history must not leak into folder totals")
	}
}
