package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
	"github.com/isaiash/diffplay/internal/watch"
)

func monitorGit(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func monitorPut(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestMonitorForgetsVanishedUntrackedTree(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "temp/nested/a", "one\ntwo\n")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	u.applyScanUpdate(scanUpdate{baseline: true, snapshot: snapshot, events: events})
	m := newMonitor(root, r, snapshot, events, true)
	if err := os.RemoveAll(filepath.Join(root, "temp")); err != nil {
		t.Fatal(err)
	}
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(up)
	for _, id := range []string{"temp", "temp/nested", "temp/nested/a"} {
		if u.graph.nodes[id] != nil || u.history.entries[id] != nil {
			t.Errorf("untracked %q survived deletion: graph=%+v history=%+v events=%+v", id, u.graph.nodes[id], u.history.entries[id], up.events)
		}
	}
	if h := u.history.tree(); h.added != 0 || h.removed != 0 || h.unknown {
		t.Errorf("phantom totals: %+v", h)
	}
}

func TestMonitorForgetsVanishedUntrackedChildWhenDirectoryRemains(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	u.applyScanUpdate(scanUpdate{baseline: true, snapshot: snapshot})
	m := newMonitor(root, r, snapshot, nil, true)
	monitorPut(t, root, "dir/a", "one\n")
	monitorPut(t, root, "dir/b", "two\n")
	u.applyScanUpdate(m.poll(time.Now()))
	if err := os.Remove(filepath.Join(root, "dir/a")); err != nil {
		t.Fatal(err)
	}
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(up)
	if u.graph.nodes["dir/a"] != nil || u.history.entries["dir/a"] != nil {
		t.Fatal("vanished untracked file still appears")
	}
	if u.graph.nodes["dir"] == nil || u.graph.nodes["dir/b"] == nil || u.history.tree().added != 1 || u.history.tree().removed != 0 {
		t.Fatalf("remaining sibling or totals lost: %+v", u.history.tree())
	}
}

func TestMonitorKeepsTrackedDeletions(t *testing.T) {
	for _, scenario := range []string{"staged-add", "committed", "staged-remove", "committed-remove", "rename"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			monitorGit(t, root, "init", "-q")
			monitorGit(t, root, "config", "user.email", "test@example.invalid")
			monitorGit(t, root, "config", "user.name", "Test")
			monitorPut(t, root, "dir/a", "one\ntwo\n")
			if scenario != "staged-add" {
				monitorGit(t, root, "add", "dir/a")
				monitorGit(t, root, "commit", "-qm", "base")
			}
			r, err := gitrepo.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := r.Scan(5)
			if err != nil {
				t.Fatal(err)
			}
			m := newMonitor(root, r, snapshot, nil, true)
			if scenario == "staged-add" {
				monitorGit(t, root, "add", "dir/a")
			}
			switch scenario {
			case "staged-remove", "committed-remove":
				monitorGit(t, root, "rm", "-q", "dir/a")
				if scenario == "committed-remove" {
					monitorGit(t, root, "commit", "-qm", "remove")
				}
			case "rename":
				monitorGit(t, root, "mv", "dir/a", "dir/b")
			default:
				if err := os.Remove(filepath.Join(root, "dir/a")); err != nil {
					t.Fatal(err)
				}
			}
			up := m.poll(time.Now())
			if up.err != nil {
				t.Fatal(up.err)
			}
			found := false
			for _, e := range up.events {
				if e.Path == "dir/a" && e.Kind == "deleted" && e.StatsKnown && e.Removed == 2 {
					found = true
				}
			}
			if !found || !up.visible["dir/a"] {
				t.Fatalf("tracked deletion lost: %+v", up)
			}
		})
	}
}

func TestMonitorForgetsNamespacedUntrackedWithoutAffectingAnotherRepo(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "a", "one\n")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	u.applyScanUpdate(namespaceUpdate("repo-1", scanUpdate{baseline: true, snapshot: snapshot, events: events}))
	u.applyScanUpdate(namespaceUpdate("repo-2", scanUpdate{baseline: true, snapshot: snapshot, events: events}))
	m := newMonitor(root, r, snapshot, events, true)
	if err := os.Remove(filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(namespaceUpdate("repo-1", up))
	if u.graph.nodes["repo-1/a"] != nil || u.history.entries["repo-1/a"] != nil || u.graph.nodes["repo-2/a"] == nil || u.history.entries["repo-2/a"] == nil {
		t.Fatal("untracked removal crossed repository namespace")
	}
}

type failingTrackingRepository struct {
	fixtureRepository
	tracked     map[string]bool
	trackingErr error
}

func (r *failingTrackingRepository) TrackedPaths() (map[string]bool, error) {
	return r.tracked, r.trackingErr
}

func TestMonitorTrackingFailurePreservesLastCompleteScan(t *testing.T) {
	initial := watch.Snapshot{"a": {Path: "a", Content: []byte("old\n")}}
	source := &failingTrackingRepository{fixtureRepository: fixtureRepository{snapshot: initial}, tracked: map[string]bool{"a": true}}
	m := newMonitor("/repo", source, initial, nil, true)
	source.snapshot = watch.Snapshot{}
	source.tracked = map[string]bool{}
	source.trackingErr = errors.New("tracking failed")
	if up := m.poll(time.Now()); up.err == nil {
		t.Fatal("tracking failure accepted")
	}
	if !m.tracked["a"] || len(m.previous) != 1 {
		t.Fatal("tracking failure advanced baseline")
	}
	source.trackingErr = nil
	up := m.poll(time.Now())
	if up.err != nil || len(up.events) != 1 || up.events[0].Kind != "deleted" {
		t.Fatalf("recovery lost last known tracked deletion: %+v", up)
	}
}

func TestMonitorUnknownInitialTrackingDoesNotDiscardPossibleTrackedDeletion(t *testing.T) {
	initial := watch.Snapshot{"a": {Path: "a", Content: []byte("old\n")}}
	source := &failingTrackingRepository{
		fixtureRepository: fixtureRepository{snapshot: watch.Snapshot{}},
		trackingErr:       errors.New("initial tracking unavailable"),
	}
	m := newMonitor("/repo", source, initial, nil, true)
	source.trackingErr = nil
	source.tracked = map[string]bool{}
	up := m.poll(time.Now())
	if up.err != nil || len(up.events) != 1 || up.events[0].Kind != "deleted" {
		t.Fatalf("uncertain tracked deletion was discarded: %+v", up)
	}
}

func TestMonitorRefreshesTrackingBeforeCommittedRemoval(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorGit(t, root, "config", "user.email", "test@example.invalid")
	monitorGit(t, root, "config", "user.name", "Test")
	monitorPut(t, root, "a", "one\n")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	m := newMonitor(root, r, snapshot, nil, true)
	monitorGit(t, root, "add", "a")
	if up := m.poll(time.Now()); up.err != nil {
		t.Fatal(up.err)
	}
	monitorGit(t, root, "commit", "-qm", "add")
	monitorGit(t, root, "rm", "-q", "a")
	monitorGit(t, root, "commit", "-qm", "remove")
	up := m.poll(time.Now())
	if up.err != nil || len(up.events) != 1 || up.events[0].Path != "a" || up.events[0].Kind != "deleted" {
		t.Fatalf("tracked state not refreshed: %+v", up)
	}
}

func TestMonitorForgetsVanishedBinary(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "binary", "a\x00b")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(snapshot, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	u.applyScanUpdate(scanUpdate{baseline: true, snapshot: snapshot, events: events})
	m := newMonitor(root, r, snapshot, events, true)
	if err := os.Remove(filepath.Join(root, "binary")); err != nil {
		t.Fatal(err)
	}
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(up)
	if u.graph.nodes["binary"] != nil || u.history.tree().unknown || u.history.tree().removed != 0 {
		t.Fatal("binary untracked deletion left unknown or removed statistics")
	}
}
