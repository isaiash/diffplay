package main

import (
	"github.com/isaiash/diffplay/internal/watch"
	"sort"
	"time"
)

type repositorySource interface {
	Scan(int) (watch.Snapshot, error)
	InitialChanges(watch.Snapshot, time.Time) ([]watch.Event, error)
	FilterPaths([]string) (map[string]bool, error)
}
type scanUpdate struct {
	snapshot watch.Snapshot
	events   []changeEvent
	visible  map[string]bool
	err      error
	baseline bool
}
type monitor struct {
	root       string
	repository repositorySource
	previous   watch.Snapshot
	known      map[string]bool
	seeded     bool
}

func newMonitor(root string, repository repositorySource, initial watch.Snapshot, events []changeEvent, seeded bool) *monitor {
	m := &monitor{root: root, repository: repository, previous: initial, known: make(map[string]bool), seeded: seeded}
	for id := range initial {
		m.known[id] = true
	}
	for _, e := range events {
		m.known[e.Path] = true
	}
	return m
}
func (m *monitor) poll(now time.Time) scanUpdate {
	baseline := !m.seeded
	var next watch.Snapshot
	var err error
	if m.repository != nil {
		next, err = m.repository.Scan(5)
	} else {
		next, err = watch.Scan(m.root, 5)
	}
	if err != nil {
		return scanUpdate{err: err}
	}
	var events []changeEvent
	if !m.seeded && m.repository != nil {
		events, err = m.repository.InitialChanges(next, now)
	} else {
		events = watch.Changes(m.previous, next, now)
	}
	if err != nil {
		return scanUpdate{err: err}
	}
	known := make(map[string]bool, len(m.known)+len(next))
	for id := range m.known {
		known[id] = true
	}
	for id := range next {
		known[id] = true
	}
	for _, e := range events {
		known[e.Path] = true
	}
	var visible map[string]bool
	if m.repository != nil {
		paths := make([]string, 0, len(known))
		for id := range known {
			paths = append(paths, id)
		}
		sort.Strings(paths)
		visible, err = m.repository.FilterPaths(paths)
		if err != nil {
			return scanUpdate{err: err}
		}
		filtered := events[:0]
		for _, e := range events {
			if allowed, ok := visible[e.Path]; !ok || allowed {
				filtered = append(filtered, e)
			}
		}
		events = filtered
		for id, allowed := range visible {
			if !allowed {
				delete(known, id)
			}
		}
	}
	m.previous = next
	m.known = known
	m.seeded = true
	return scanUpdate{snapshot: next, events: events, visible: visible, baseline: baseline}
}
