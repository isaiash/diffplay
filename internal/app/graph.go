package app

import (
	"hash/fnv"
	"math"
	"sort"
	"time"
)

const burstDuration = 4 * time.Second
const tombstoneDuration = 12 * time.Second

type node struct {
	id, parent string
	dir        bool
	x, y       float64
	kind, diff string
	changed    time.Time
	quiet      bool
	radius     float64

	added, removed int
	fresh, unknown bool
	lines          int
	linesKnown     bool
}

type graph struct {
	nodes         map[string]*node
	ordered       []*node
	width, height int
	dirty         bool
}

func newGraph() *graph { return &graph{nodes: make(map[string]*node), width: 80, height: 24} }
func (g *graph) upsert(id, parent string, dir bool) {
	if n := g.nodes[id]; n != nil {
		n.parent = parent
		n.dir = dir
		if n.kind == "deleted" {
			n.kind = ""
		}
		return
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	angle := float64(h.Sum32()%6283) / 1000
	x, y := float64(g.width)/2, float64(g.height)/2
	if p := g.nodes[parent]; p != nil {
		x, y = p.x, p.y
	}
	g.nodes[id] = &node{id: id, parent: parent, dir: dir, x: x + math.Cos(angle)*3, y: y + math.Sin(angle)*2}
	g.dirty = true
}
func (g *graph) list() []*node {
	if g.dirty {
		g.ordered = g.ordered[:0]
		for _, n := range g.nodes {
			g.ordered = append(g.ordered, n)
		}
		sort.Slice(g.ordered, func(i, j int) bool { return g.ordered[i].id < g.ordered[j].id })
		g.dirty = false
	}
	return g.ordered
}
func (g *graph) mark(id, kind, diff string, now time.Time) {
	if n := g.nodes[id]; n != nil {
		n.kind = kind
		n.diff = diff
		n.changed = now
		n.quiet = false
	}
}
func (g *graph) expire(now time.Time) {
	for id, n := range g.nodes {
		if n.kind == "deleted" && now.Sub(n.changed) > tombstoneDuration {
			delete(g.nodes, id)
			g.dirty = true
		}
	}
	g.pruneContext()
}

// Context-only directories disappear once their last changed descendant is gone.
func (g *graph) pruneContext() {
	for {
		parents := make(map[string]bool)
		for _, n := range g.nodes {
			parents[n.parent] = true
		}
		removed := false
		for id, n := range g.nodes {
			if n.dir && n.kind == "" && !parents[id] {
				delete(g.nodes, id)
				g.dirty = true
				removed = true
			}
		}
		if !removed {
			return
		}
	}
}
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
