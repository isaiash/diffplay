package app

import (
	"errors"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestSnapshotAddsQuietGraphContextWithoutHistory(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	s := watch.Snapshot{
		".":            {Path: ".", IsDir: true},
		"src":          {Path: "src", IsDir: true},
		"src/clean.go": {Path: "src/clean.go"},
	}
	applySnapshot(u.graph, s, nil, time.Now())
	n := u.graph.nodes["src/clean.go"]
	if n == nil || n.kind != "" || !n.changed.IsZero() {
		t.Fatal("scanned clean file must exist without change activity")
	}
	if u.graph.nodes["."] != nil || len(u.changeRows()) != 0 || len(u.history.entries) != 0 {
		t.Fatal("context must not add root nodes or list/history entries")
	}
}

func TestSessionsSeedGraphContextAcrossRepositories(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	s := watch.Snapshot{".": {Path: ".", IsDir: true}, "clean": {Path: "clean"}}
	seedSessions(u, []repositorySession{
		{prefix: "repo-1", label: "a", initial: s},
		{prefix: "repo-2", label: "b", initial: s},
		{prefix: "repo-3", initial: s, scanErr: errors.New("partial scan")},
	})
	if u.graph.nodes["repo-1/clean"] == nil || u.graph.nodes["repo-2/clean"] == nil {
		t.Fatal("startup snapshots must seed collision-free repository context")
	}
	if u.graph.nodes["repo-3/clean"] != nil || len(u.history.entries) != 0 {
		t.Fatal("failed scans and quiet context must not seed history")
	}
}

func TestRecoverySnapshotSeedsQuietContextAndPreservesOtherRepositories(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	u.graph.upsert("repo-2/keep", "repo-2", false)
	up := namespaceUpdate("repo-1", scanUpdate{
		baseline: true,
		snapshot: watch.Snapshot{"clean": {Path: "clean"}},
		events:   []changeEvent{{Path: "changed", Kind: "modified", Added: 3, StatsKnown: true, At: time.Now()}},
	})
	u.applyScanUpdate(up)
	if u.graph.nodes["repo-1/clean"] == nil || u.graph.nodes["repo-2/keep"] == nil {
		t.Fatal("recovery must add its context without replacing another repository")
	}
	if len(u.history.entries) != 1 || u.history.entries["repo-1/changed"] == nil || !u.graph.nodes["repo-1/changed"].quiet {
		t.Fatal("recovery must seed only real changes quietly")
	}
	u.applyScanUpdate(scanUpdate{err: errors.New("failed scan"), visible: map[string]bool{"repo-2/keep": false}})
	if u.graph.nodes["repo-2/keep"] == nil {
		t.Fatal("failed updates must not mutate graph visibility")
	}
}

func TestLiveContextIgnoreAndDeletionLifecycle(t *testing.T) {
	now := time.Now()
	u := newUI(newGraph(), "/repo")
	initial := watch.Snapshot{".": {Path: ".", IsDir: true}, "clean": {Path: "clean"}}
	u.applyScanUpdate(scanUpdate{snapshot: initial})
	source := &fixtureRepository{snapshot: watch.Snapshot{".": initial["."]}, allowed: map[string]bool{"clean": false}}
	m := newMonitor("/repo", source, initial, nil, true)
	u.applyScanUpdate(m.poll(now))
	if u.graph.nodes["clean"] != nil || len(u.history.entries) != 0 {
		t.Fatal("ignore must hide quiet context without a fake deletion")
	}
	source.allowed = nil
	source.snapshot = initial
	u.applyScanUpdate(m.poll(now))
	if u.graph.nodes["clean"] == nil {
		t.Fatal("unignored context must reappear")
	}
	source.snapshot = watch.Snapshot{".": initial["."]}
	u.applyScanUpdate(m.poll(now))
	if u.graph.nodes["clean"].kind != "deleted" || u.history.entries["clean"] == nil {
		t.Fatal("real deletion must create tombstone and history")
	}
	u.applyScanUpdate(m.poll(now.Add(time.Hour)))
	if u.graph.nodes["clean"] == nil {
		t.Fatal("idle scans must preserve tombstones")
	}
	u.graph.expire(now.Add(time.Hour))
	if u.graph.nodes["clean"] != nil || u.history.entries["clean"] == nil {
		t.Fatal("expired tombstone must retain changed-only history")
	}
}
