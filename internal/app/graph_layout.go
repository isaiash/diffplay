package app

import (
	"math"
	"sort"
	"strings"
)

// All packing is done in physical horizontal-cell units. Only projection into
// terminal coordinates halves y; changing the viewport never distorts circles.
// Left/Top/Width/Height describe the direct-file rectangle. Radius is its
// physical circumscribed radius for legacy offset normalization, not a drag
// boundary: callers must constrain disks against all four rectangle edges.
type fileCluster struct {
	ID                              string
	Nodes                           []*node
	Left, Top, Width, Height        float64
	CenterX, CenterY, Radius, Scale float64
}

// Folder rectangles include ancestors without direct files. Extents describe
// border endpoints in terminal cells, not array counts. Circle fields remain
// available to legacy callers, but do not determine folder packing.
type folderEnvelope struct {
	ID                              string
	Left, Top, Width, Height        float64
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
	zone     packedRect
	rect     packedRect
}

type packedRect struct{ x, y, w, h float64 }

type hierarchyLayout struct {
	clusters  []fileCluster
	envelopes []folderEnvelope
	positions map[*node][2]float64
}

func packingRadius(n *node) float64 {
	r := n.radius
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return .5
	}
	return math.Max(.5, r)
}

// Rectangular shelves keep sibling borders adjacent, with a two-physical-unit
// gap. Trying a bounded range of shelf widths adapts packing to the viewport
// without quadratic work for folders with many children.
func tileFolders(parts []packedRect, aspect float64) (float64, float64) {
	if len(parts) == 0 {
		return 0, 0
	}
	if len(parts) == 1 {
		parts[0].x, parts[0].y = 0, 0
		return parts[0].w, parts[0].h
	}
	minWidth, maxWidth := 0.0, -2.0
	for _, p := range parts {
		minWidth = math.Max(minWidth, p.w)
		maxWidth += p.w + 2
	}
	best, bestW, bestH := math.Inf(1), 0.0, 0.0
	placements := make([]packedRect, len(parts))
	candidate := make([]packedRect, len(parts))
	for trial := 0; trial <= 48; trial++ {
		limit := minWidth + (maxWidth-minWidth)*float64(trial)/48
		x, y, rowH, width := 0.0, 0.0, 0.0, 0.0
		for i, p := range parts {
			if x > 0 && x+p.w > limit+1e-8 {
				y += rowH + 2
				x, rowH = 0, 0
			}
			p.x, p.y = x, y
			candidate[i] = p
			width = math.Max(width, x+p.w)
			x += p.w + 2
			rowH = math.Max(rowH, p.h)
		}
		height := y + rowH
		score := math.Max((width+2)/aspect, height+6)
		if score < best-1e-8 || (math.Abs(score-best) < 1e-8 && width*height < bestW*bestH) {
			best, bestW, bestH = score, width, height
			copy(placements, candidate)
		}
	}
	copy(parts, placements)
	return bestW, bestH
}

func (f *packedFolder) packRect(aspect float64) {
	parts := make([]packedRect, 0, len(f.children)+1)
	if len(f.nodes) > 0 {
		if cap(f.files) < len(f.nodes) {
			f.files = make([]packedCircle, len(f.nodes))
		} else {
			f.files = f.files[:len(f.nodes)]
		}
		largest := 0.0
		for i, n := range f.nodes {
			f.files[i].r = packingRadius(n) + .35
			largest = math.Max(largest, f.files[i].r)
		}
		// Equal-sized hexagonal slots keep even the largest disk separated.
		// Search possible row lengths using only the bounds, then place once.
		best, columns := math.Inf(1), 1
		n := len(f.files)
		for cols := 1; cols <= n; cols++ {
			rows := (n + cols - 1) / cols
			last := n - (rows-1)*cols
			width := 2 * largest * float64(cols)
			if rows > 1 && (rows > 2 || last == cols) {
				width += largest
			} else if rows == 2 {
				width = math.Max(width, 2*largest*float64(last)+largest)
			}
			height := 2*largest + float64(rows-1)*math.Sqrt(3)*largest
			score := math.Max((width+4)/aspect, height+8)
			if score < best {
				best, columns = score, cols
				f.zone.w, f.zone.h = width+2, height+2
			}
		}
		for i := range f.files {
			row, col := i/columns, i%columns
			f.files[i].x = 1 + largest + 2*largest*float64(col) + largest*float64(row%2)
			f.files[i].y = 1 + largest + math.Sqrt(3)*largest*float64(row)
		}
		parts = append(parts, packedRect{w: f.zone.w, h: f.zone.h})
	}
	for _, child := range f.children {
		child.packRect(aspect)
		parts = append(parts, child.rect)
	}
	w, h := tileFolders(parts, aspect)
	// One horizontal cell and one terminal row of border inset; two rows
	// above contents preserve the title band, even for nested frames.
	titleWidth := 4
	for _, ch := range folderName(f.id) + "/" {
		titleWidth += runeWidth(ch)
	}
	f.rect.w = math.Max(w+2, float64(titleWidth))
	f.rect.h = h + 6
	offset := 0
	if len(f.nodes) > 0 {
		f.zone.x = parts[0].x + 1
		f.zone.y = parts[0].y + 4
		offset = 1
	}
	for i, child := range f.children {
		child.rect.x = parts[i+offset].x + 1
		child.rect.y = parts[i+offset].y + 4
	}
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
	for _, n := range g.visibleFiles() {
		parent := n.parent
		if parent == "" {
			parent = "."
		}
		f := ensure(parent)
		f.nodes = append(f.nodes, n)
		count++
	}
	if count == 0 {
		return result
	}
	for _, f := range folders {
		sort.Slice(f.children, func(i, j int) bool { return f.children[i].id < f.children[j].id })
	}
	width, height := float64(max(1, w)-1), float64(max(1, h)-1)
	// A child's best aspect need not equal the final viewport aspect: siblings,
	// ancestor titles and border insets consume space too. Try several shared
	// targets and keep the complete hierarchy with the highest uniform scale.
	aspect := math.Max(.05, float64(max(1, w-1))/float64(2*max(1, h-1)))
	bestAspect, bestScale := aspect, -1.0
	for trial := -6; trial <= 6; trial++ {
		candidate := aspect * math.Pow(2, float64(trial)/6)
		root.packRect(candidate)
		s := math.Min(width/root.rect.w, 2*height/root.rect.h)
		if s > bestScale {
			bestAspect, bestScale = candidate, s
		}
	}
	root.packRect(bestAspect)
	scale := bestScale
	// Expand the rectangular hierarchy into both available axes while keeping
	// every file disk's radius at the uniform, non-distorting fit scale. The
	// larger projection never compresses existing gaps, so files and folder
	// borders remain disjoint even in very wide, shallow terminal panes.
	scaleX, scaleY := width/root.rect.w, 2*height/root.rect.h
	var project func(*packedFolder, float64, float64)
	project = func(f *packedFolder, x, y float64) {
		x += f.rect.x
		y += f.rect.y
		l, t := x*scaleX, y*scaleY/2
		fw, fh := f.rect.w*scaleX, f.rect.h*scaleY/2
		result.envelopes = append(result.envelopes, folderEnvelope{ID: f.id, Left: l, Top: t, Width: fw, Height: fh, CenterX: l + fw/2, CenterY: t + fh/2, Radius: math.Hypot(fw/2, fh), Scale: scale, Depth: f.depth})
		if len(f.nodes) > 0 {
			zx, zy := x+f.zone.x, y+f.zone.y
			l, t := zx*scaleX, zy*scaleY/2
			cw, ch := f.zone.w*scaleX, f.zone.h*scaleY/2
			centerX, centerY := l+cw/2, t+ch/2
			r := math.Hypot(cw/2, ch)
			result.clusters = append(result.clusters, fileCluster{ID: f.id, Nodes: f.nodes, Left: l, Top: t, Width: cw, Height: ch, CenterX: centerX, CenterY: centerY, Radius: r, Scale: scale})
			for i, n := range f.nodes {
				result.positions[n] = [2]float64{(zx + f.files[i].x) * scaleX, (zy + f.files[i].y) * scaleY / 2}
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

func (g *graph) applyLayout(layout hierarchyLayout, w, h int) {
	// Hidden context has no packing position, but its retained model coordinates
	// must remain finite and inside the resized viewport as well.
	for _, n := range g.list() {
		if _, projected := layout.positions[n]; projected {
			continue
		}
		if math.IsNaN(n.x) || math.IsInf(n.x, 0) {
			n.x = 0
		}
		if math.IsNaN(n.y) || math.IsInf(n.y, 0) {
			n.y = 0
		}
		n.x, n.y = clamp(n.x, 0, float64(w-1)), clamp(n.y, 0, float64(h-1))

	}
	for n, p := range layout.positions {
		n.x, n.y = p[0], p[1]

	}
	g.width, g.height = w, h
}
