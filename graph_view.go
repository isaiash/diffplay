package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Circle area follows total edited lines, normalized independently per repository.
func (u *ui) updateGraphMetrics() {
	maximum := make(map[string]int)
	for _, n := range u.graph.list() {
		if n.dir {
			continue
		}
		if r := u.history.entries[n.id]; r != nil {
			n.added = r.added
			n.removed = r.removed
			n.fresh = r.fresh
			n.unknown = r.unknown
		}
		group := u.histogramGroup(n.id)
		maximum[group] = max(maximum[group], n.added+n.removed)
	}
	for _, n := range u.graph.list() {
		if n.dir {
			continue
		}
		total := n.added + n.removed
		peak := maximum[u.histogramGroup(n.id)]
		n.radius = 0.5
		if total > 0 && peak > 0 {
			n.radius = math.Sqrt(0.25 + 8.75*float64(total)/float64(peak))
		}
	}
}
func graphFileColor(n *node) rgb {
	outer, _, _ := bubblePalette(n)
	return outer
}
func (u *ui) fileNodes() []*node {
	var nodes []*node
	for _, n := range u.graph.list() {
		if !n.dir {
			nodes = append(nodes, n)
		}
	}
	return nodes
}
func (u *ui) drawFileDisk(c *canvas, n *node, radius float64) {
	u.drawSolidBubble(c, n, radius)
}
func (u *ui) renderGraph(w, h int, now time.Time) string {
	u.updateGraphMetrics()
	c := newCanvas(w, h)

	clusters := u.graph.clusters(w, max(1, h-4))
	u.applyGraphDrags(clusters)
	u.graphRadii = make(map[string]float64)
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
		if age < 0 || age >= burstDuration {
			continue
		}
		t := float64(age) / float64(burstDuration)
		x, y := u.screen(n)
		radius := (u.displayRadius(n) + t*7) * u.zoom
		color := blend(graphFileColor(n), background, t)
		for angle := 0.0; angle < math.Pi*2; angle += .1 {
			xx, yy := x+math.Cos(angle)*radius, y+math.Sin(angle)*radius/2
			if yy >= 2 && yy < float64(h-2) {
				c.dot(int(xx*2), int(yy*4), color)
			}
		}
	}
	for _, n := range nodes {
		u.drawFileDisk(c, n, u.displayRadius(n))
	}
	// Folders are headings for their file clusters, never graph nodes.
	envelopes := u.graph.folderEnvelopes(w, max(1, h-4))
	for _, envelope := range envelopes {
		if envelope.ID == "." && len(envelopes) > 1 {
			continue
		}
		title := u.displayPath(envelope.ID)
		title += "/"
		left := envelope.CenterX - envelope.Radius
		top := envelope.CenterY - envelope.Radius/2
		cx := float64(w) / 2
		cy := float64(max(1, h-4)) / 2
		x := int(math.Round((left-cx)*u.zoom + cx))
		y := int(math.Round((top-cy)*u.zoom+cy)) + 2
		if y >= 2 && y < h-2 {
			c.text(x+1, y, title, gray, max(0, min(w-x-2, int(2*envelope.Radius*u.zoom)-2)))
		}
	}
	for _, y := range []int{0, 1, h - 2, h - 1} {
		if y >= 0 && y < h {
			for x := 0; x < w; x++ {
				c.put(x, y, ' ', muted)
			}
		}
	}
	c.text(1, 0, "Diffplay  CLUSTERS", white, w-2)
	c.text(24, 0, u.root, gray, w-25)
	deleted := 0
	for _, n := range nodes {
		if n.kind == "deleted" {
			deleted++
		}
	}
	state := "LIVE"
	if u.paused {
		state = "LAYOUT PAUSED"
	}
	folders := len(envelopes)
	if folders > 1 {
		folders--
	} // The implicit root envelope is not displayed.
	c.text(1, 1, fmt.Sprintf("%s  %d files · %d folders · %d deleted  |  size: added + removed lines", state, len(nodes), folders, deleted), muted, w-2)
	if len(nodes) == 0 && h > 7 {
		c.text(2, 4, "No changed files.", gray, w-4)
	}
	c.text(1, h-2, "shell: dominant edits · core: minority edits · blue: new · size: total lines", gray, w-2)
	c.text(1, h-1, "Tab graph/list · hover name/diff · drag file · wheel/+/- zoom · n/p select · j/k diff · space freeze · q quit", muted, w-2)
	if u.warning != "" {
		c.text(1, h-2, "Warning: "+u.warning, red, w-2)
	}
	if n := u.graph.nodes[u.selected]; n != nil && !n.dir && w >= 24 && h >= 10 {
		pw := min(76, max(24, w/2))
		ph := min(h-4, 18)
		px, py := w-pw, 2
		for yy := py; yy < py+ph; yy++ {
			for xx := px; xx < w; xx++ {
				c.put(xx, yy, ' ', gray)
			}
		}
		c.text(px+1, py, u.displayPath(n.id), white, pw-2)
		c.text(px+1, py+1, n.kind+"  "+signedCount("+", n.added, n.unknown)+" "+signedCount("-", n.removed, n.unknown), muted, pw-2)
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
			c.text(px+1, py+3+i, line, color, pw-2)
		}
		c.text(px+1, py+ph-1, fmt.Sprintf("j/k scroll · %d/%d", u.scroll+1, len(lines)), muted, pw-2)
	}
	return c.ansi()
}
