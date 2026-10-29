package app

import (
	"path"
	"sort"
	"strings"
	"time"
)

// history is session activity, independent of the graph's expiring tombstones.
// Counts accumulate observed edits, rather than representing a Git diff or net delta.
type history struct {
	entries map[string]*changeRow
	cached  *changeRow
}

type changeRow struct {
	id, kind, diff      string
	flashKind           string
	dir, fresh, unknown bool
	added, removed      int
	modifications       int
	previewScroll       int
	changed             time.Time
	children            []*changeRow
	depth               int
}

func newHistory() *history { return &history{entries: make(map[string]*changeRow)} }
func (h *history) record(id, kind string, dir bool, diff string, added, removed int, known bool, at time.Time) {
	r := h.entries[id]
	if r == nil {
		r = &changeRow{id: id}
		h.entries[id] = r
	}
	r.kind = kind
	r.flashKind = kind
	r.dir = dir
	r.diff = diff
	r.changed = at
	if kind == "created" {
		r.fresh = true
	}
	r.added += added
	r.removed += removed
	r.unknown = r.unknown || !known
	h.cached = nil
}
func (h *history) tree() *changeRow {
	if h.cached != nil {
		return h.cached
	}
	nodes := map[string]*changeRow{".": {id: ".", dir: true}}
	for id, e := range h.entries {
		r := *e
		r.children = nil
		nodes[id] = &r
	}
	for id := range h.entries {
		for parent := path.Dir(id); id != "."; parent = path.Dir(parent) {
			if nodes[parent] == nil {
				nodes[parent] = &changeRow{id: parent, dir: true}
			}
			if parent == "." {
				break
			}
		}
	}
	for id, r := range nodes {
		if id == "." {
			continue
		}
		r.depth = strings.Count(id, "/") + 1
		p := nodes[path.Dir(id)]
		p.dir = true
		p.children = append(p.children, r)
	}
	var aggregate func(*changeRow)
	aggregate = func(r *changeRow) {
		if r.dir {
			r.modifications = 0
		}
		allFresh := len(r.children) > 0
		for _, child := range r.children {
			aggregate(child)
			r.added += child.added
			r.removed += child.removed
			r.modifications += child.modifications
			r.unknown = r.unknown || child.unknown
			allFresh = allFresh && child.fresh
			if child.changed.After(r.changed) || child.changed.Equal(r.changed) && flashPriority(child.flashKind) > flashPriority(r.flashKind) {
				r.changed = child.changed
				r.flashKind = child.flashKind
			}
		}
		if len(r.children) > 0 {
			r.fresh = allFresh
		}
		sort.Slice(r.children, func(i, j int) bool {
			a, b := r.children[i], r.children[j]
			if a.added != b.added {
				return a.added > b.added
			}
			if a.removed != b.removed {
				return a.removed > b.removed
			}
			return a.id < b.id
		})
	}
	aggregate(nodes["."])
	h.cached = nodes["."]
	return h.cached
}
func flattenChanges(root *changeRow, open map[string]bool) []*changeRow {
	rows := []*changeRow{}
	var walk func(*changeRow)
	walk = func(r *changeRow) {
		rows = append(rows, r)
		if r.dir && open[r.id] {
			for _, child := range r.children {
				walk(child)
			}
		}
	}
	walk(root)
	return rows
}
