package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const graphRippleDuration = 900 * time.Millisecond
const graphFlashDuration = 250 * time.Millisecond

// Combined disks follow edit volume per repository; standalone retains sparse
// equal-capacity edit disks and size-proportional unchanged context.
func (u *ui) updateGraphMetrics() {
	for _, n := range u.graph.list() {
		if r := u.history.entries[n.id]; r != nil {
			n.added = r.added
			n.removed = r.removed
			n.fresh = r.fresh
			n.unknown = r.unknown
		}
	}
	maximum := make(map[string]int)
	for _, n := range u.fileNodes() {
		lines := n.lines
		known := n.linesKnown
		if u.combined {
			lines, known = max(0, n.added)+max(0, n.removed), !n.unknown
		}
		if !n.dir && known && (u.combined || n.kind == "" && u.history.entries[n.id] == nil) {
			group := u.histogramGroup(n.id)
			maximum[group] = max(maximum[group], lines)
		}
	}
	for _, n := range u.graph.list() {
		if n.dir {
			continue
		}
		n.radius = 3
		if u.combined {
			n.radius = .5
			edits := max(0, n.added) + max(0, n.removed)
			if peak := maximum[u.histogramGroup(n.id)]; peak > 0 && edits > 0 && !n.unknown {
				n.radius = max(.5, 3*math.Sqrt(float64(edits)/float64(peak)))
			}
			continue
		}
		if n.kind == "" && n.added+n.removed == 0 && !n.unknown {
			n.radius = .6
			if n.linesKnown {
				n.radius = .5
				if peak := maximum[u.histogramGroup(n.id)]; peak > 0 && n.lines > 0 {
					n.radius = max(.5, 3*math.Sqrt(float64(n.lines)/float64(peak)))
				}
			}
		}
	}
}
func graphFileColor(n *node) rgb {
	outer, _, _ := bubblePalette(n)
	return outer
}
func (u *ui) fileNodes() []*node {
	return u.graph.visibleFiles(u.combined)
}
func (u *ui) drawFileDisk(c *canvas, n *node, radius float64) {
	u.drawClusterSymbol(c, n, radius)
}
func (u *ui) renderGraph(w, h int, now time.Time) string {
	return u.renderGraphCanvas(w, h, now).ansi()
}
func (u *ui) renderGraphCanvas(w, h int, now time.Time) *canvas {
	u.width, u.height = w, h
	u.updateGraphMetrics()
	contentHeight := u.graphContentHeight()
	u.graphRendering = true
	defer func() { u.graphRendering = false }()

	layout := u.graph.packedHierarchy(w, contentHeight, u.combined)
	if !u.paused || w != u.graph.width || contentHeight != u.graph.height || u.graphLayoutCombined != u.combined {
		u.graph.applyLayout(layout, w, contentHeight)
	}
	u.graphLayoutCombined = u.combined
	c := newCanvas(w, contentHeight+4)

	clusters := layout.clusters

	u.graphRadii = make(map[string]float64)
	u.graphDotUnits = make(map[string]int)
	for _, cluster := range clusters {
		for _, n := range cluster.Nodes {
			u.graphRadii[n.id] = n.radius * cluster.Scale
		}
	}
	nodes := u.fileNodes()
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].changed.Before(nodes[j].changed) })
	// Bursts are transient decoration, not edges or relationships.
	for _, n := range nodes {
		if n.quiet || n.changed.IsZero() {
			continue
		}
		age := now.Sub(n.changed)
		if age < 0 || age >= graphRippleDuration {
			continue
		}
		t := float64(age) / float64(graphRippleDuration)
		x, y := u.screen(n)
		radius := (u.displayRadius(n) + t*10) * u.zoom
		color := blend(graphFileColor(n), background, t)
		for angle := 0.0; angle < math.Pi*2; angle += .1 {
			xx, yy := x+math.Cos(angle)*radius, y+math.Sin(angle)*radius/2
			if yy >= 2 && yy < float64(contentHeight+2) {
				c.dot(int(xx*2), int(yy*4), color)
			}
		}
	}
	var flashLayer *canvas
	for _, n := range nodes {
		age := now.Sub(n.changed)
		if !n.quiet && !n.changed.IsZero() && age >= 0 && age < graphFlashDuration {
			// Isolate the file's dots so the flash never recolors the ripple
			// or a neighboring file; startup data is explicitly quiet.
			if flashLayer == nil {
				flashLayer = newCanvas(c.w, c.h)
			}
			u.drawFileDisk(flashLayer, n, u.displayRadius(n))
			strength := 1 - float64(age)/float64(graphFlashDuration)
			x, y := u.screen(n)
			radius := math.Max(.1, u.displayRadius(n)*u.zoom)
			for yy := max(0, int(math.Floor(y+.5-radius/2))-1); yy < min(c.h, int(math.Ceil(y+.5+radius/2))+2); yy++ {
				for xx := max(0, int(math.Floor(x+.5-radius))-1); xx < min(c.w, int(math.Ceil(x+.5+radius))+2); xx++ {
					i := yy*c.w + xx
					v := flashLayer.cells[i]
					if v.dots != 0 {
						color := blend(v.color, white, strength)

						c.cells[i].dots |= v.dots
						c.cells[i].color = color
						flashLayer.cells[i] = cell{}
					}
				}
			}
		} else {
			u.drawFileDisk(c, n, u.displayRadius(n))
		}
	}

	// Nested folder frames express containment without relationship edges.
	envelopes := layout.envelopes
	for _, envelope := range envelopes {
		if envelope.ID == "." {
			continue
		}
		u.drawFolderFrame(c, envelope)
	}
	// Existing rasterizers clip two virtual rows at each edge. Crop those
	// rows to expose the entire graph viewport without parsing ANSI.
	output := newCanvas(w, h)
	copy(output.cells, c.cells[2*w:(2+min(contentHeight, h))*w])
	c = output
	u.graphRendering = false
	if len(nodes) == 0 && h > 4 {
		c.text(2, 2, "No files to display.", gray, w-4)
	}
	if !u.combined {
		for x := 0; x < w; x++ {
			c.put(x, h-1, ' ', muted)
		}
		state := "LIVE"
		if u.paused {
			state = "PAUSED"
		}
		help := fmt.Sprintf("%s %d files · q quit · Tab view · drag pan · wheel · j/k diff · Shift+drag select · terminal copy shortcut", state, len(nodes))
		if u.warning != "" {
			help += " · Warning: " + u.warning
		}
		c.text(1, h-1, help, gray, w-2)
	}
	if n := u.graph.nodes[u.selected]; !u.combined && n != nil && !n.dir && w >= 24 && h >= 10 {
		pw := min(76, max(24, w/2))
		ph := min(h-4, 18)
		px, py := w-pw, 2
		for yy := py; yy < py+ph; yy++ {
			for xx := px; xx < w; xx++ {
				c.put(xx, yy, ' ', gray)
			}
		}
		c.text(px+1, py, u.displayPath(n.id), white, pw-2)
		kind := n.kind
		if kind == "" {
			kind = "unchanged"
		}
		c.text(px+1, py+1, kind+"  "+signedCount("+", n.added, n.unknown)+" "+signedCount("-", n.removed, n.unknown), gray, pw-2)
		if unit := u.graphDotUnits[n.id]; unit > 0 {
			c.text(px+1, py+2, fmt.Sprintf("1 dot ≤ %d cumulative lines · rounded up", unit), gray, pw-2)
		} else if kind == "unchanged" {
			label := "File line count unavailable"
			if n.linesKnown {
				label = fmt.Sprintf("%d file lines · gray area: file size", n.lines)
			}
			c.text(px+1, py+2, label, gray, pw-2)
		}
		lines := strings.Split(n.diff, "\n")
		if n.diff == "" {
			lines = []string{"No text diff available."}
		}
		visible := max(1, ph-4)
		u.scroll = min(u.scroll, max(0, len(lines)-visible))
		for i := 0; i < visible && i+u.scroll < len(lines); i++ {
			line := lines[i+u.scroll]
			color := gray
			if strings.HasPrefix(line, "+") {
				color = green
				if n.fresh || n.kind == "created" {
					color = blue
				}
			} else if strings.HasPrefix(line, "-") {
				color = red
			}
			c.text(px+1, py+3+i, diffText(line), color, pw-2)
		}
		c.text(px+1, py+ph-1, fmt.Sprintf("%d/%d", u.scroll+1, len(lines)), gray, pw-2)
	}
	return c
}
