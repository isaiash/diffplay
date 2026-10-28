package main

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// Startup Git data establishes counts but must never look like fresh live edits.
func (u *ui) seedEvents(events []changeEvent) {
	applySnapshot(u.graph, nil, events, time.Now())
	for _, e := range events {
		u.recordChange(e.Path, e.Kind, e.IsDir, e.Diff, e.Added, e.Removed, e.StatsKnown, time.Time{})
		if n := u.graph.nodes[e.Path]; n != nil {
			n.quiet = true
		}
	}
}
func namespaceUpdate(prefix string, up scanUpdate) scanUpdate {
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
	return scanUpdate{events: events, visible: visible, err: up.err, baseline: up.baseline}
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
