package app

import (
	"math"
	"path"
	"time"
)

// Each renderer sees only its pane's dimensions. The terminal dimensions belong
// to the router, never to graph screen transforms or list hit testing.
// A divider and a global footer each occupy a row when space permits.
func (u *ui) splitTop(h int) int {
	if h < 3 {
		return max(0, h-1)
	}
	top := (h - 2) / 2
	if u.splitRatio > 0 {
		top = int(math.Round(u.splitRatio * float64(h)))
	}
	minimum := 3
	if h < 8 {
		minimum = 1
	}
	return max(minimum, min(h-2-minimum, top))
}
func (u *ui) renderCombined(w, h int, now time.Time) *canvas {
	c := newCanvas(w, h)
	top := u.splitTop(h)
	if top > 0 {
		u.width, u.height = w, top
		list := u.renderListCanvas(w, top, now)
		if !u.graphFocus {
			u.selected = u.listSelected
		}
		copy(c.cells, list.cells)
	}
	if h >= 3 {
		for x := 0; x < w; x++ {
			c.put(x, top, '─', muted)
		}
		if u.dividerHover == 2 {
			c.put(u.dividerX, top, '↕', white)
		}
		graphHeight := max(0, h-top-2)
		u.width, u.height = w, graphHeight
		graph := u.renderGraphCanvas(w, graphHeight, now)
		copy(c.cells[(top+1)*w:], graph.cells)
	}
	c.text(1, h-1, "q quit · Tab view · ↑↓ · ←→ tree · drag split/pan · wheel · J/K diff · m copy", muted, w-2)
	return c
}
func (u *ui) keyboardDimensions() {
	if !u.combined {
		if u.terminalWidth > 0 {
			u.width, u.height = u.terminalWidth, u.terminalHeight
		}
		return
	}
	top := u.splitTop(u.terminalHeight)
	u.width, u.height = u.terminalWidth, top
	if u.graphFocus {
		u.height = max(0, u.terminalHeight-top-2)
	}
}
func (u *ui) routeMouse(code, x, y int, release bool) {
	u.followPending = nil
	u.dividerHover = 0
	u.dividerX, u.dividerY = x, y
	if u.listSplitDragging {
		if release {
			u.listSplitDragging = false
			return
		}
		if code&32 != 0 && code&3 == 0 {
			w := u.terminalWidth
			if u.listWidth(w) < w {
				u.listSplitRatio = float64(max(20, min(w-20, x))) / float64(w)
			}
		}
		u.dividerHover = 1
		return
	}
	listHeight := u.terminalHeight - 1
	if u.combined {
		listHeight = u.splitTop(u.terminalHeight)
	}
	if !u.splitDragging && !u.dragging && (u.listMode || u.combined) && u.listWidth(u.terminalWidth) < u.terminalWidth && x == u.listWidth(u.terminalWidth) && y >= 1 && y < listHeight {
		if !release {
			u.dividerHover = 1
			if code&64 == 0 && code&32 == 0 && code&3 == 0 {
				u.listSplitDragging = true
			}
		}
		return
	}
	if !u.combined {
		u.keyboardDimensions()
	}
	if u.combined {
		top := u.splitTop(u.terminalHeight)
		if u.splitDragging {
			if release {
				u.splitDragging = false
				return
			}
			if code&32 != 0 && code&3 == 0 {
				minHeight := 3
				if u.terminalHeight < 8 {
					minHeight = 1
				}
				top = max(minHeight, min(u.terminalHeight-2-minHeight, y))
				u.splitRatio = float64(top) / float64(u.terminalHeight)
			}
			u.dividerHover = 2
			return
		}
		if !u.dragging && y == top && u.terminalHeight >= 3 {
			if !release {
				u.dividerHover = 2
			}
			if !release && code&64 == 0 && code&32 == 0 && code&3 == 0 {
				u.splitDragging = true
			}
			return
		}
		u.width = u.terminalWidth
		if y > top || u.dragging {
			if y >= u.terminalHeight-1 && !u.dragging {
				return
			}
			u.graphFocus = true
			u.height = max(0, u.terminalHeight-top-2)
			u.graphMouse(code, x, y-top-1, release)
		} else {
			u.graphFocus = false
			u.height = top
			u.listMouse(code, x, y, release)
		}
	} else if u.listMode {
		u.listMouse(code, x, y, release)
	} else {
		u.graphMouse(code, x, y, release)
	}
}
func (u *ui) graphMouse(code, x, y int, release bool) {
	if release {
		u.dragging = false
		return
	}
	if code&64 != 0 {
		old := u.zoom
		if code&1 == 0 {
			u.zoom = clamp(u.zoom*1.15, .2, 6)
		} else {
			u.zoom = clamp(u.zoom/1.15, .2, 6)
		}
		cx, cy := float64(u.width)/2, float64(u.graphContentHeight())/2
		ratio := u.zoom / old
		u.panX = float64(x) - cx - (float64(x)-cx-u.panX)*ratio
		u.panY = float64(y) - cy - (float64(y)-cy-u.panY)*ratio
	} else if code&3 == 0 && code&32 == 0 {
		u.selected = u.hit(x, y)
		u.dragging = true
		u.dragX, u.dragY = x, y
		u.scroll = 0
		u.syncGraphSelection()
	} else if code&32 != 0 {
		if u.dragging && code&3 == 0 {
			u.panX += float64(x - u.dragX)
			u.panY += float64(y - u.dragY)
			u.dragX, u.dragY = x, y
		} else {
			id := u.hit(x, y)
			if id != u.selected {
				u.scroll = 0
			}
			u.selected = id
			u.syncGraphSelection()
		}
	}
}
func (u *ui) syncListSelection() {
	u.selected = u.listSelected
	u.revealListSelection()
}
func (u *ui) syncGraphSelection() {
	if u.selected == "" {
		return
	}
	u.listSelected = u.selected
	for p := path.Dir(u.selected); ; p = path.Dir(p) {
		u.open[p] = true
		if p == "." {
			break
		}
	}
	u.revealListSelection()
}

// presentationTree adds only the selected quiet path. It never records activity
// or modifies cached history aggregates.
func (u *ui) presentationTree() *changeRow {
	root := u.history.tree()
	n := u.graph.nodes[u.selected]
	if n == nil || u.selected == "" {
		return root
	}
	var contains func(*changeRow) bool
	contains = func(r *changeRow) bool {
		if r.id == u.selected {
			return true
		}
		for _, child := range r.children {
			if contains(child) {
				return true
			}
		}
		return false
	}
	if contains(root) {
		return root
	}
	h := newHistory()
	for id, r := range u.history.entries {
		h.entries[id] = r
	}
	h.entries[n.id] = &changeRow{id: n.id, dir: n.dir, kind: "unchanged"}
	return h.tree()
}

func (u *ui) revealListSelection() {
	rows := u.changeRows()
	h := u.height
	if u.combined {
		h = u.splitTop(u.terminalHeight)
	}
	visible := max(1, u.listVisibleHeight(h))
	for i, r := range rows {
		if r.id != u.listSelected {
			continue
		}
		if i < u.listOffset {
			u.listOffset = i
		}
		if i >= u.listOffset+visible {
			u.listOffset = i - visible + 1
		}
		break
	}
}
