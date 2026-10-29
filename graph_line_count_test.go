package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestSnapshotLineCountsRefreshWithoutChangeActivity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "clean")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0644); err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	scan := func() watch.Snapshot {
		t.Helper()
		s, err := watch.Scan(root, 1)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := scan()
	u.applyScanUpdate(scanUpdate{baseline: true, snapshot: s})
	n := u.graph.nodes["clean"]
	if n == nil || !n.linesKnown || n.lines != 2 {
		t.Fatalf("startup line count not copied: %+v", n)
	}
	source := &fixtureRepository{snapshot: s}
	m := newMonitor(root, source, s, nil, true)
	// Updating only sizing metadata must not synthesize an event or history row.
	e := s["clean"]
	e.Lines = 7
	source.snapshot = watch.Snapshot{".": s["."], "clean": e}
	up := m.poll(time.Now())
	if len(up.events) != 0 {
		t.Fatalf("metadata-only changes created events: %+v", up.events)
	}
	u.applyScanUpdate(up)
	if n.lines != 7 || !n.linesKnown || n.kind != "" || !n.changed.IsZero() || len(u.history.entries) != 0 || len(u.changeRows()) != 0 {
		t.Fatalf("metadata-only refresh changed activity or failed: %+v", n)
	}
	// Real filesystem edits must refresh metadata alongside the existing event.
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	source.snapshot = scan()
	up = m.poll(time.Now())
	if len(up.events) != 1 {
		t.Fatalf("live edit events: %+v", up.events)
	}
	u.applyScanUpdate(up)
	if n.lines != 3 || !n.linesKnown || len(u.history.entries) != 1 {
		t.Fatalf("live line count not refreshed: %+v", n)
	}
	// Known zero and unknown must replace old values, not leave stale counts.
	for _, text := range []string{"", "binary\x00"} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		applySnapshot(u.graph, scan(), nil, time.Now())
		if n.lines != 0 || n.linesKnown != (text == "") {
			t.Fatalf("stale count after %q: %+v", text, n)
		}
	}
}

func TestNamespacedSnapshotsPreserveLineCounts(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	s := watch.Snapshot{".": {Path: ".", IsDir: true}, "clean": {Path: "clean", Lines: 42, LinesKnown: true}}
	seedSessions(u, []repositorySession{{prefix: "repo-1", initial: s}, {prefix: "repo-2", initial: s}})
	for _, id := range []string{"repo-1/clean", "repo-2/clean"} {
		n := u.graph.nodes[id]
		if n == nil || n.lines != 42 || !n.linesKnown {
			t.Fatalf("%s lost metadata: %+v", id, n)
		}
	}
	if len(u.history.entries) != 0 || len(u.changeRows()) != 0 {
		t.Fatal("quiet counts seeded history")
	}
}
