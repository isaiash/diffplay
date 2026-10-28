package main

import (
	"reflect"
	"testing"
	"time"
)

func TestChangeTreeOnlyChangedPathsSortedByAddedLines(t *testing.T) {
	h := newHistory()
	now := time.Now()
	h.record("src/small.go", "modified", false, "", 2, 9, true, now)
	h.record("docs/large.md", "created", false, "", 20, 0, true, now)
	h.record("src/remove.go", "deleted", false, "", 0, 30, true, now)
	rows := h.tree()
	if rows.id != "." || rows.added != 22 || rows.removed != 39 {
		t.Fatalf("bad root totals: %+v", rows)
	}
	if len(rows.children) != 2 || rows.children[0].id != "docs" || rows.children[1].id != "src" {
		t.Fatal("folders must sort by descendant additions")
	}
	if rows.children[1].children[0].id != "src/small.go" {
		t.Fatal("file sort must prioritize added lines, not total churn")
	}
	if !rows.children[0].children[0].fresh {
		t.Fatal("new file classification lost")
	}
}

func TestChangeHistoryAccumulatesAndRetainsDeletion(t *testing.T) {
	h := newHistory()
	now := time.Now()
	h.record("new.txt", "created", false, "+a", 1, 0, true, now)
	h.record("new.txt", "modified", false, "+b", 2, 1, true, now.Add(time.Second))
	h.record("new.txt", "deleted", false, "-a", 0, 2, true, now.Add(2*time.Second))
	r := h.tree().children[0]
	if r.added != 3 || r.removed != 3 || r.kind != "deleted" || !r.fresh || r.diff != "-a" {
		t.Fatalf("lost session history: %+v", r)
	}
	if len(h.tree().children) != 1 {
		t.Fatal("deletions must not expire with graph tombstones")
	}
}

func TestChangeTreeUnknownStatsAndEmptyDirectory(t *testing.T) {
	h := newHistory()
	now := time.Now()
	h.record("assets/image.png", "modified", false, "Binary files differ", 0, 0, false, now)
	h.record("empty", "created", true, "", 0, 0, true, now)
	r := h.tree()
	if !r.unknown || len(r.children) != 2 {
		t.Fatal("unknown counts and changed empty directories must survive")
	}
	if got := flattenChanges(r, map[string]bool{".": true, "assets": true}); !reflect.DeepEqual(changeIDs(got), []string{".", "assets", "assets/image.png", "empty"}) {
		t.Fatalf("unexpected visible tree: %v", changeIDs(got))
	}
}

func changeIDs(rows []*changeRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.id
	}
	return ids
}
