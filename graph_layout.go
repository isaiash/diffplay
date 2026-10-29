package main

import (
	"math"
	"sort"
	"strings"
)

// All packing is done in physical horizontal-cell units. Only projection into
// terminal coordinates halves y; changing the viewport never distorts circles.
type fileCluster struct {
	ID                              string
	Nodes                           []*node
	Left, Top, Width, Height        float64
	CenterX, CenterY, Radius, Scale float64
}

// folderEnvelope includes ancestors without direct files. Its disk contains
// both its own direct-file zone and every descendant folder disk.
type folderEnvelope struct {
	ID                              string
	CenterX, CenterY, Radius, Scale float64
	Depth                           int
}

type packedCircle struct{ x, y, r float64 }
type packedFolder struct {
	id       string
	depth    int
	children []*packedFolder
	nodes    []*node
	files    []packedCircle
	zone     packedCircle
	circle   packedCircle
}

type hierarchyLayout struct {
	clusters  []fileCluster
	envelopes []folderEnvelope
	positions map[*node][2]float64
}

// packCircles uses size-ordered concentric contact rings, not viewport rows or
// columns. Each ring has a provable separation from both adjacent rings and its
// own neighbors. Sorting dominates O(n log n); there is no all-pairs relaxation
// or density-dependent retry loop, including for 100,000 coincident input files.
// The enclosing disk is deliberately conservative, avoiding numerical overlap.
func packCircles(circles []packedCircle) float64 {
	if len(circles) == 0 {
		return 0
	}
	order := make([]int, len(circles))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return circles[order[i]].r > circles[order[j]].r })
	if len(order) == 1 {
		circles[0].x, circles[0].y = 0, 0
		return circles[0].r
	}
	if len(order) == 2 {
		a, b := &circles[order[0]], &circles[order[1]]
		a.x, a.y = -b.r, 0
		b.x, b.y = a.r, 0
		return a.r + b.r
	}
	if len(order) <= 6 {
		r := circles[order[0]].r
		rho := r / math.Sin(math.Pi/float64(len(order)))
		for i, index := range order {
			angle := float64(i)*2*math.Pi/float64(len(order)) - .5*math.Pi
			circles[index].x, circles[index].y = rho*math.Cos(angle), rho*math.Sin(angle)
		}
		return rho + r
	}
	first := &circles[order[0]]
	first.x, first.y = 0, 0
	outer := first.r
	for start, ring := 1, 0; start < len(order); ring++ {
		r := circles[order[start]].r
		rho := outer + r
		capacity := max(1, int(math.Floor(math.Pi/math.Asin(math.Min(1, r/rho))+1e-10)))
		count := min(capacity, len(order)-start)
		for i := 0; i < count; i++ {
			angle := float64(i)*2*math.Pi/float64(count) + float64(ring)*.73
			c := &circles[order[start+i]]
			c.x, c.y = rho*math.Cos(angle), rho*math.Sin(angle)
		}
		outer = rho + r
		start += count
	}
	return outer
}

func packingRadius(n *node) float64 {
	r := n.radius
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return .5
	}
	return clamp(r, .5, 3)
}

// A title band is two physical units (one terminal row) before scaling, plus
// an extra unit of radial breathing room. Shift contents downward by one unit
// and expand the circle by three: no heading crosses any contained disk.
func (f *packedFolder) packWith(pack func([]packedCircle) float64) {
	parts := make([]packedCircle, 0, len(f.children)+1)
	if len(f.nodes) > 0 {
		f.files = make([]packedCircle, len(f.nodes))
		for i, n := range f.nodes {
			f.files[i].r = packingRadius(n) + .35
		}
		r := pack(f.files)
		for i := range f.files {
			f.files[i].y++
		}
		f.zone.r = r + 3
		parts = append(parts, f.zone)
	}
	for _, child := range f.children {
		child.packWith(pack)
		parts = append(parts, child.circle)
	}
	r := pack(parts)
	offset := 0
	if len(f.nodes) > 0 {
		f.zone = parts[0]
		f.zone.y++
		offset = 1
	}
	for i, child := range f.children {
		child.circle = parts[i+offset]
		child.circle.y++
	}
	f.circle.r = r + 3
}

func (g *graph) packedHierarchy(w, h int) hierarchyLayout {
	result := hierarchyLayout{positions: make(map[*node][2]float64)}
	root := &packedFolder{id: "."}
	folders := map[string]*packedFolder{".": root}
	var ensure func(string) *packedFolder
	ensure = func(id string) *packedFolder {
		if f := folders[id]; f != nil {
			return f
		}
		parent := "."
		if at := strings.LastIndex(id, "/"); at >= 0 && id != "/" {
			parent = id[:at]
			if parent == "" {
				parent = "/"
			}
		}
		p := ensure(parent)
		f := &packedFolder{id: id, depth: p.depth + 1}
		folders[id] = f
		p.children = append(p.children, f)
		return f
	}
	count := 0
	for _, n := range g.list() {
		if !n.dir {
			f := ensure(n.parent)
			f.nodes = append(f.nodes, n)
			count++
		}
	}
	if count == 0 {
		return result
	}
	for _, f := range folders {
		sort.Slice(f.children, func(i, j int) bool { return f.children[i].id < f.children[j].id })
	}
	pack := packCircles
	switch g.layoutMode {
	case 1:
		pack = packSpiral
	case 2:
		pack = packHex
	}
	root.packWith(pack)
	width, height := float64(max(1, w)-1), float64(max(1, h)-1)
	scale := math.Min(1, math.Min(width/(2*root.circle.r), height/root.circle.r))
	var project func(*packedFolder, float64, float64)
	project = func(f *packedFolder, x, y float64) {
		x += f.circle.x
		y += f.circle.y
		cx, cy := width/2+x*scale, height/2+y*scale/2
		result.envelopes = append(result.envelopes, folderEnvelope{ID: f.id, CenterX: cx, CenterY: cy, Radius: f.circle.r * scale, Scale: scale, Depth: f.depth})
		if len(f.nodes) > 0 {
			zx, zy := x+f.zone.x, y+f.zone.y
			centerX, centerY := width/2+zx*scale, height/2+zy*scale/2
			r := f.zone.r * scale
			result.clusters = append(result.clusters, fileCluster{ID: f.id, Nodes: f.nodes, Left: centerX - r, Top: centerY - r/2, Width: 2 * r, Height: r, CenterX: centerX, CenterY: centerY, Radius: r, Scale: scale})
			for i, n := range f.nodes {
				result.positions[n] = [2]float64{width/2 + (zx+f.files[i].x)*scale, height/2 + (zy+f.files[i].y)*scale/2}
			}
		}
		for _, child := range f.children {
			project(child, x, y)
		}
	}
	project(root, 0, 0)
	sort.Slice(result.clusters, func(i, j int) bool { return result.clusters[i].ID < result.clusters[j].ID })
	sort.Slice(result.envelopes, func(i, j int) bool { return result.envelopes[i].ID < result.envelopes[j].ID })
	return result
}

func (g *graph) clusters(w, h int) []fileCluster           { return g.packedHierarchy(w, h).clusters }
func (g *graph) folderEnvelopes(w, h int) []folderEnvelope { return g.packedHierarchy(w, h).envelopes }

// Deterministic contact positions are also the initial positions: files never
// spend their first frames interpenetrating while a force simulation settles.
// A structural update or resize repacks immediately, even at dt=0. With an
// unchanged structure dt=0 is stationary. Directory context is never animated.
func (g *graph) stepClusters(w, h int, dt float64) {
	w, h = max(1, w), max(1, h)
	layout := g.packedHierarchy(w, h)
	// Context nodes have no role in packing, but keep legacy model coordinates
	// finite and within the resized viewport for callers inspecting the graph.
	for _, n := range g.list() {
		if !n.dir {
			continue
		}
		if math.IsNaN(n.x) || math.IsInf(n.x, 0) {
			n.x = 0
		}
		if math.IsNaN(n.y) || math.IsInf(n.y, 0) {
			n.y = 0
		}
		n.x, n.y = clamp(n.x, 0, float64(w-1)), clamp(n.y, 0, float64(h-1))
		if math.IsNaN(n.vx) || math.IsInf(n.vx, 0) {
			n.vx = 0
		}
		if math.IsNaN(n.vy) || math.IsInf(n.vy, 0) {
			n.vy = 0
		}
	}
	for n, p := range layout.positions {
		n.x, n.y = p[0], p[1]
		n.initialized = true
		if dt > 0 || math.IsNaN(n.vx) || math.IsInf(n.vx, 0) {
			n.vx = 0
		}
		if dt > 0 || math.IsNaN(n.vy) || math.IsInf(n.vy, 0) {
			n.vy = 0
		}
	}
	g.width, g.height = w, h
}
