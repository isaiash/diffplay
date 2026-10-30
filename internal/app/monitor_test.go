package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

type fixtureRepository struct {
	snapshot watch.Snapshot
	events   []changeEvent
	allowed  map[string]bool
	err      error
}

func (r *fixtureRepository) Scan(int) (watch.Snapshot, error) { return r.snapshot, r.err }
func (r *fixtureRepository) InitialChanges(watch.Snapshot, time.Time) ([]changeEvent, error) {
	return r.events, r.err
}
func (r *fixtureRepository) FilterPaths(paths []string) (map[string]bool, error) {
	out := make(map[string]bool)
	for _, p := range paths {
		allowed, ok := r.allowed[p]
		out[p] = !ok || allowed
	}
	return out, r.err
}

func TestMonitorHidesNewlyIgnoredPathsWithoutDeletionEvents(t *testing.T) {
	initial := watch.Snapshot{".": {Path: ".", IsDir: true}, "ignored.txt": {Path: "ignored.txt", Content: []byte("old\n")}}
	source := &fixtureRepository{snapshot: watch.Snapshot{".": initial["."]}, allowed: map[string]bool{"ignored.txt": false}}
	m := newMonitor("/repo", source, initial, nil, true)
	up := m.poll(time.Now())
	if up.err != nil || len(up.events) != 0 || up.visible["ignored.txt"] {
		t.Fatalf("ignore must hide, not delete: %+v", up)
	}
}

func TestFailedMonitorPollPreservesBaseline(t *testing.T) {
	initial := watch.Snapshot{"a": {Path: "a", Content: []byte("old\n")}}
	source := &fixtureRepository{err: errors.New("read failed")}
	m := newMonitor("/repo", source, initial, nil, true)
	if up := m.poll(time.Now()); up.err == nil {
		t.Fatal("poll failure must be reported")
	}
	if string(m.previous["a"].Content) != "old\n" {
		t.Fatal("failure must not advance baseline")
	}
	source.err = nil
	source.snapshot = watch.Snapshot{"a": {Path: "a", Content: []byte("new\n")}}
	if up := m.poll(time.Now()); len(up.events) != 1 || up.events[0].Kind != "modified" {
		t.Fatal("recovery must compare last complete snapshot")
	}
}

func TestFilesystemMonitorReusesUnchangedLargeSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "large.txt"), []byte(strings.Repeat("x", watch.ContentLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	initial, err := watch.Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.CloseSnapshot(initial)
	previous := initial["large.txt"].Source
	if previous == nil {
		t.Fatal("initial scan did not capture large source")
	}
	m := newMonitor(root, nil, initial, nil, true)
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	defer watch.CloseSnapshot(m.previous)
	if up.snapshot["large.txt"].Source != previous || !previous.Available() {
		t.Fatal("filesystem monitor did not retain unchanged source")
	}
}

type partialScanErrorRepository struct {
	*fixtureRepository
	partial watch.Snapshot
}

func (r *partialScanErrorRepository) ScanPrevious(int, watch.Snapshot) (watch.Snapshot, error) {
	return r.partial, errors.New("incomplete scan")
}

func TestFailedPreviousAwarePollClosesOnlyNewSources(t *testing.T) {
	previous, err := watch.NewDocument(strings.NewReader("previous\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer previous.Close()
	partial, err := watch.NewDocument(strings.NewReader("partial\n"))
	if err != nil {
		t.Fatal(err)
	}
	initial := watch.Snapshot{"large": {Source: previous}}
	repository := &partialScanErrorRepository{
		fixtureRepository: &fixtureRepository{},
		partial:           watch.Snapshot{"large": {Source: partial}},
	}
	m := newMonitor("/unused", repository, initial, nil, true)
	if up := m.poll(time.Now()); up.err == nil {
		t.Fatal("partial scan error was not reported")
	}
	if partial.Available() {
		t.Fatal("failed poll leaked its new source")
	}
	if !previous.Available() {
		t.Fatal("failed poll closed the baseline source")
	}
}
