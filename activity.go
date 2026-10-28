package main

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
