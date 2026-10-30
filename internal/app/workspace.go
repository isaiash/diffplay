package app

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

// Startup Git data establishes counts but must never look like fresh live edits.
func (u *ui) seedEvents(events []changeEvent) {
	applySnapshot(u.graph, nil, events, time.Now())
	for _, e := range events {
		u.recordChange(e.Path, e.Kind, e.IsDir, e.Diff, e.Added, e.Removed, e.StatsKnown, time.Time{})
		if r := u.history.entries[e.Path]; r != nil {
			r.document = e.Document
		}
		if n := u.graph.nodes[e.Path]; n != nil {
			n.quiet = true
		}
	}
}
func namespaceUpdate(prefix string, up scanUpdate) scanUpdate {
	up.totalKey = prefix
	if prefix == "" {
		return up
	}
	events := make([]changeEvent, 0, len(up.events))
	for _, e := range up.events {
		if e.Path == "." {
			continue
		}
		e.Path = path.Join(prefix, e.Path)
		events = append(events, e)
	}
	visible := make(map[string]bool, len(up.visible))
	for id, v := range up.visible {
		if id != "." {
			visible[path.Join(prefix, id)] = v
		}
	}
	var snapshot watch.Snapshot
	if up.snapshot != nil {
		snapshot = make(watch.Snapshot, len(up.snapshot))
		for id, entry := range up.snapshot {
			entry.Path = path.Join(prefix, id)
			snapshot[entry.Path] = entry
		}
	}
	modifications := make(map[string]int, len(up.modifications))
	for id, n := range up.modifications {
		if id != "." {
			modifications[path.Join(prefix, id)] = n
		}
	}
	previews := make(map[string]string, len(up.previews))
	for id, diff := range up.previews {
		if id != "." {
			previews[path.Join(prefix, id)] = diff
		}
	}
	documents := make(map[string]*watch.Document, len(up.documents))
	for id, d := range up.documents {
		if id != "." {
			documents[path.Join(prefix, id)] = d
		}
	}
	up.documents = documents
	up.modifications, up.previews = modifications, previews
	up.snapshot, up.events, up.visible = snapshot, events, visible
	return up
}
func (u *ui) displayPath(id string) string {
	group, rest, found := strings.Cut(id, "/")
	if label, ok := u.repositoryLabels[group]; ok {
		if found {
			return label + "/" + rest
		}
		return label
	}
	return id
}
func (u *ui) displayName(id string) string {
	if label, ok := u.repositoryLabels[id]; ok {
		return label
	}
	return path.Base(id)
}
func (u *ui) histogramGroup(id string) string {
	group, _, _ := strings.Cut(id, "/")
	if _, ok := u.repositoryLabels[group]; ok {
		return group
	}
	return ""
}
func groupKey(index int) string { return fmt.Sprintf("repo-%d", index+1) }
