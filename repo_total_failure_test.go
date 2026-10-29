package main

import (
	"errors"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

type totalFailureSource struct {
	fixtureRepository
	fail bool
}

func (s *totalFailureSource) LineTotal() (int, bool, error) {
	if s.fail {
		return 0, false, errors.New("deep file inaccessible")
	}
	return 1, true, nil
}

func TestLineTotalFailureDoesNotBlockLiveEvents(t *testing.T) {
	before := watch.Snapshot{"a": {Path: "a", Content: []byte("old\n")}}
	source := &totalFailureSource{fixtureRepository: fixtureRepository{snapshot: watch.Snapshot{"a": {Path: "a", Content: []byte("new\n")}}}, fail: true}
	m := newMonitor("/repo", source, before, nil, true)
	u := newUI(newGraph(), "demo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 1, known: true}}
	up := m.poll(time.Now())
	if up.err != nil || len(up.events) != 1 {
		t.Fatalf("valid filesystem update was blocked by denominator failure: %+v", up)
	}
	u.applyScanUpdate(up)
	if _, known := u.repositoryLineTotal(); known {
		t.Fatal("failed count must not present stale ratio as exact")
	}
	if u.history.entries["a"] == nil {
		t.Fatal("live file change was lost")
	}
	source.fail = false
	u.applyScanUpdate(m.poll(time.Now()))
	if n, known := u.repositoryLineTotal(); !known || n != 1 {
		t.Fatal("line total did not recover")
	}
}
