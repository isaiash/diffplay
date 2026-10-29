package main

import (
	"math"
	"time"
)

// repositoryLineTotal returns the summed complete current-content line count.
// Unknown means at least one selected repository has no complete count.
func (u *ui) repositoryLineTotal() (int, bool) {
	if len(u.repositoryTotals) == 0 {
		return 0, false
	}
	total := 0
	for _, value := range u.repositoryTotals {
		if !value.known || value.lines > math.MaxInt-total {
			return 0, false
		}
		total += value.lines
	}
	return total, true
}

// applyScanUpdate consumes only complete scans. Snapshot context never enters
// session history, including the first successful scan after startup failure.
func (u *ui) applyScanUpdate(up scanUpdate) {
	if up.err != nil {
		return
	}
	if up.total != nil {
		if u.repositoryTotals == nil {
			u.repositoryTotals = make(map[string]repositoryLineCount)
		}
		u.repositoryTotals[up.totalKey] = *up.total
	}
	if up.baseline {
		applySnapshot(u.graph, up.snapshot, nil, time.Now())
		u.seedEvents(up.events)
	} else {
		applySnapshot(u.graph, up.snapshot, up.events, time.Now())
		u.recordEvents(up.events)
	}
	for id, preview := range up.previews {
		if n := u.graph.nodes[id]; n != nil {
			n.diff = preview
		}
		if r := u.history.entries[id]; r != nil {
			r.diff = preview
		}
	}
	for id, count := range up.modifications {
		if r := u.history.entries[id]; r != nil {
			r.modifications = count
		}
	}
	for _, e := range up.events {
		if preview, ok := up.previews[e.Path]; ok {
			if r := u.history.entries[e.Path]; r != nil {
				r.previewScroll = previewOffset(preview, e.Diff)
			}
		}
	}
	u.history.cached = nil
	u.applyVisibility(up.visible)
	if !up.baseline {
		u.followEvents(up.events, up.visible, time.Now())
	}
}

func flashPriority(kind string) int {
	switch kind {
	case "deleted":
		return 3
	case "created":
		return 2
	case "modified":
		return 1
	}
	return 0
}

func (u *ui) recordEvents(events []changeEvent) {
	for _, e := range events {
		u.recordChange(e.Path, e.Kind, e.IsDir, e.Diff, e.Added, e.Removed, e.StatsKnown, e.At)
	}
}

// Pending work is bounded by distinct history paths, not the number of bursts.
func (u *ui) followEvents(events []changeEvent, visible map[string]bool, now time.Time) {
	first := ""
	for _, e := range events {
		if e.IsDir {
			continue
		}
		if allowed, ok := visible[e.Path]; ok && !allowed {
			continue
		}
		if first == "" {
			first = e.Path
		}
		queued := false
		for _, id := range u.followPending {
			if id == e.Path {
				queued = true
				break
			}
		}
		if !queued {
			u.followPending = append(u.followPending, e.Path)
		}
	}
	if first == "" {
		return
	}
	for i, id := range u.followPending {
		if id == first {
			u.followPending = append(u.followPending[:i], u.followPending[i+1:]...)
			break
		}
	}
	u.followFile(first)
	u.followAt = now.Add(600 * time.Millisecond)
}

func (u *ui) followFile(id string) {
	u.selected = id
	u.scroll = 0
	if r := u.history.entries[id]; r != nil {
		u.scroll = r.previewScroll
	}
	u.syncGraphSelection()
}

func (u *ui) advanceFollow(now time.Time) {
	if now.Before(u.followAt) {
		return
	}
	for len(u.followPending) > 0 {
		id := u.followPending[0]
		u.followPending = u.followPending[1:]
		r := u.history.entries[id]
		if r == nil || r.dir {
			continue
		}
		u.followFile(id)
		u.followAt = now.Add(600 * time.Millisecond)
		break
	}
}

// An ignore-rule update hides paths, rather than recording them as deletions.
func (u *ui) applyVisibility(visible map[string]bool) {
	for id, allowed := range visible {
		if allowed || id == "." {
			continue
		}
		if _, exists := u.graph.nodes[id]; exists {
			delete(u.graph.nodes, id)
			u.graph.dirty = true
		}
		if _, exists := u.history.entries[id]; exists {
			delete(u.history.entries, id)
			u.history.cached = nil
		}
	}
	u.graph.pruneContext()
}
