package main

import (
	"errors"
	"github.com/isaiash/diffplay/internal/watch"
	"sort"
	"time"
)

type repositorySource interface {
	Scan(int) (watch.Snapshot, error)
	InitialChanges(watch.Snapshot, time.Time) ([]watch.Event, error)
	FilterPaths([]string) (map[string]bool, error)
}

// Non-Git sources retain their ordinary filesystem deletion behavior.
type trackingSource interface {
	TrackedPaths() (map[string]bool, error)
}

// Optional so filesystem sources and lightweight fixtures need not model Git.
type headChangesSource interface {
	HeadChanges(watch.Snapshot, time.Time) ([]watch.Event, error)
}
type lineTotalSource interface {
	LineTotal() (int, bool, error)
}

type repositoryLineCount struct {
	lines int
	known bool
}

type scanUpdate struct {
	snapshot      watch.Snapshot
	events        []changeEvent
	visible       map[string]bool
	err           error
	baseline      bool
	total         *repositoryLineCount
	totalKey      string
	modifications map[string]int
	previews      map[string]string
	warning       error
}
type monitor struct {
	root          string
	repository    repositorySource
	previous      watch.Snapshot
	known         map[string]bool
	tracked       map[string]bool
	seeded        bool
	modifications map[string]int
	store         *activityStore
	storageErr    error
	pending       map[string]int
	resets        map[string]bool
}

func newMonitor(root string, repository repositorySource, initial watch.Snapshot, events []changeEvent, seeded bool) *monitor {
	m := &monitor{root: root, repository: repository, previous: initial, known: make(map[string]bool), seeded: seeded, modifications: make(map[string]int), pending: make(map[string]int), resets: make(map[string]bool)}
	for id := range initial {
		m.known[id] = true
	}
	for _, e := range events {
		m.known[e.Path] = true
	}
	if source, ok := repository.(trackingSource); ok && seeded {
		var err error
		m.tracked, err = source.TrackedPaths()
		if err != nil {
			// Unknown startup classification must not erase a potentially tracked
			// deletion. A successful poll replaces this conservative baseline.
			m.tracked = make(map[string]bool, len(m.known))
			for id := range m.known {
				m.tracked[id] = true
			}
		}
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
	var tracked map[string]bool
	if source, ok := m.repository.(trackingSource); ok {
		tracked, err = source.TrackedPaths()
		if err != nil {
			return scanUpdate{err: err}
		}
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
			if tracked != nil && e.Kind == "deleted" && !tracked[e.Path] && !m.tracked[e.Path] {
				visible[e.Path] = false
				continue
			}
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
	var total *repositoryLineCount
	if source, ok := m.repository.(lineTotalSource); ok {
		n, complete, err := source.LineTotal()
		if err != nil {
			// The full-depth percentage is auxiliary: an inaccessible file
			// outside the monitored snapshot must not freeze live activity.
			// Mark the ratio unavailable until a complete count succeeds.
			complete = false
		}
		total = &repositoryLineCount{lines: n, known: complete}
	}
	m.previous = next
	m.known = known
	m.tracked = tracked
	m.seeded = true
	up := scanUpdate{snapshot: next, events: events, visible: visible, baseline: baseline, total: total}
	m.activity(&up, now)
	return up
}

func (m *monitor) activity(up *scanUpdate, now time.Time) {
	up.modifications = make(map[string]int)
	for p := range m.modifications {
		up.modifications[p] = 0
	}
	var dirty map[string]string
	if source, ok := m.repository.(headChangesSource); ok {
		events, err := source.HeadChanges(up.snapshot, now)
		if err != nil {
			up.warning = err
		} else {
			dirty = make(map[string]string)
			for _, e := range events {
				if !e.IsDir {
					dirty[e.Path] = e.Diff
				}
			}
		}
	}
	up.previews = make(map[string]string)
	if !up.baseline {
		for _, e := range up.events {
			if e.IsDir {
				continue
			}
			m.modifications[e.Path]++
			if m.store != nil {
				m.pending[e.Path]++
			}
			up.modifications[e.Path] = 0
			if dirty != nil {
				up.previews[e.Path] = dirty[e.Path]
			} else if up.warning != nil {
				up.previews[e.Path] = "HEAD preview unavailable: " + up.warning.Error() + "\n"
			}
		}
	}
	if dirty != nil {
		for p := range m.modifications {
			if _, ok := dirty[p]; !ok {
				delete(m.modifications, p)
				delete(m.pending, p)
				if m.store != nil {
					m.resets[p] = true
				}
			}
		}
	}
	if m.store != nil {
		counts, err := m.store.apply(dirty, m.pending, m.resets)
		m.storageErr = err
		if err == nil {
			m.modifications = counts
			clear(m.pending)
			clear(m.resets)
		}
	}
	up.warning = errors.Join(up.warning, m.storageErr)
	for p, n := range m.modifications {
		up.modifications[p] = n
	}
}
