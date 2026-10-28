package main

import (
	"errors"
	"github.com/isaiash/diffplay/internal/watch"
	"testing"
	"time"
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
