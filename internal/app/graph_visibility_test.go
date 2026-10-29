package app

import (
	"strings"
	"testing"
	"time"
)

func TestGraphHidesInactiveFolderBranchesRecursively(t *testing.T) {
	g := newGraph()
	for _, f := range []struct{ id, parent string }{{"root-clean", "."}, {"A/clean", "A"}, {"A/C/change", "A/C"}, {"A/C/nearby", "A/C"}, {"A/quiet/deep/file", "A/quiet/deep"}, {"B/quiet", "B"}} {
		g.upsert(f.id, f.parent, false)
	}
	g.mark("A/C/change", "modified", "+edit", time.Unix(100, 0))
	u := newUI(g, "demo")
	got := map[string]bool{}
	for _, n := range u.fileNodes() {
		got[n.id] = true
	}
	for _, id := range []string{"root-clean", "A/clean", "A/C/change", "A/C/nearby"} {
		if !got[id] {
			t.Errorf("missing active-branch context %s", id)
		}
	}
	for _, id := range []string{"A/quiet/deep/file", "B/quiet"} {
		if got[id] {
			t.Errorf("inactive folder leaked: %s", id)
		}
	}
	for _, e := range g.folderEnvelopes(160, 44) {
		if strings.Contains(e.ID, "quiet") || e.ID == "B" {
			t.Errorf("inactive frame leaked: %s", e.ID)
		}
	}
	g.mark("B/quiet", "modified", "+live", time.Unix(101, 0))
	got = map[string]bool{}
	for _, n := range u.fileNodes() {
		got[n.id] = true
	}
	if !got["B/quiet"] {
		t.Fatal("first change must reveal previously hidden folder")
	}
}

func TestGraphOmitsRootFrameAndUsesLocalFolderNames(t *testing.T) {
	g := newGraph()
	applySnapshot(g, nil, []changeEvent{{Path: "parent/child/file", Kind: "modified", At: time.Unix(100, 0)}}, time.Unix(100, 0))
	u := newUI(g, "demo")
	frame := plainList(u.render(160, 50, time.Unix(200, 0)))
	if strings.Contains(frame, "./") || strings.Contains(frame, "parent/child/") {
		t.Fatal("redundant root/path in nested frame labels")
	}
	for _, label := range []string{"parent/", "child/"} {
		if !strings.Contains(frame, label) {
			t.Fatalf("missing local label %s", label)
		}
	}
	if strings.Count(frame, "┌") != 2 {
		t.Fatal("expected only parent and child frames")
	}
}
