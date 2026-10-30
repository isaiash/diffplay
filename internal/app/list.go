package app

import (
	"fmt"
	"path"
	"strings"
	"time"
)

var green = rgb{r: 115, g: 211, b: 153}

func (u *ui) recordChange(id, kind string, dir bool, diff string, added, removed int, known bool, at time.Time) {
	u.history.record(id, kind, dir, diff, added, removed, known, at)
	// New branches reveal the complete changed path. Preserve deliberate collapses.
	if dir {
		if _, seen := u.open[id]; !seen {
			u.open[id] = true
		}
	}
	for p := path.Dir(id); ; p = path.Dir(p) {
		if _, seen := u.open[p]; !seen {
			u.open[p] = true
		}
		if p == "." {
			break
		}
	}
}
func (u *ui) changeRows() []*changeRow {
	var rows []*changeRow
	for _, child := range u.presentationTree().children {
		rows = append(rows, flattenChanges(child, u.open)...)
	}
	if len(rows) == 0 {
		return nil
	}
	for {
		for _, r := range rows {
			if r.id == u.listSelected {
				return rows
			}
		}
		if u.listSelected == "." {
			u.listSelected = rows[0].id
			return rows
		}
		u.listSelected = path.Dir(u.listSelected)
	}
}
func (u *ui) listMove(delta int) {
	defer u.syncListSelection()
	rows := u.changeRows()
	if len(rows) == 0 {
		return
	}
	index := 0
	for i, r := range rows {
		if r.id == u.listSelected {
			index = i
			break
		}
	}
	u.listSelected = rows[max(0, min(len(rows)-1, index+delta))].id
	u.scroll = 0
}
func (u *ui) listKey(key string) {
	defer u.syncListSelection()
	rows := u.changeRows()
	if len(rows) == 0 {
		return
	}
	index := 0
	for i, r := range rows {
		if r.id == u.listSelected {
			index = i
			break
		}
	}
	r := rows[index]
	switch key {
	case "A", "k":
		u.listMove(-1)
	case "B", "j":
		u.listMove(1)
	case "C", "l":
		if r.dir {
			if !u.open[r.id] {
				u.open[r.id] = true
			} else if len(r.children) > 0 {
				u.listSelected = r.children[0].id
			}
		}
	case "D", "h":
		if r.dir && u.open[r.id] {
			u.open[r.id] = false
		} else if path.Dir(r.id) != "." {
			u.listSelected = path.Dir(r.id)
		}
	case "enter":
		if r.dir {
			u.open[r.id] = !u.open[r.id]
		}
	case "H":
		u.listSelected = rows[0].id
	case "F":
		u.listSelected = rows[len(rows)-1].id
	case "5~":
		u.listMove(-max(1, u.listVisibleHeight(u.height)))
	case "6~":
		u.listMove(max(1, u.listVisibleHeight(u.height)))
	case "a":
		for id := range u.open {
			u.open[id] = true
		}
	case "c":
		for id := range u.open {
			u.open[id] = id == "."
		}
		u.listSelected = rows[0].id
	case "J":
		u.scroll++
	case "K":
		u.scroll = max(0, u.scroll-1)
	}
}
func (u *ui) listWidth(w int) int {
	if u.listSplitRatio > 0 && (u.combined && w >= 40 || !u.combined && w >= 120) {
		return max(20, min(w-20, int(u.listSplitRatio*float64(w)+.5)))
	}
	if u.combined && w >= 40 {
		return max(20, w/2-1)
	}
	if w >= 120 {
		return w - min(70, w/2) - 1
	}
	return w
}
func (u *ui) listVisibleHeight(h int) int {
	footer := 1
	if u.combined {
		footer = 0
	}
	return max(0, h-2-footer)
}
func (u *ui) listMouse(code, x, y int, release bool) {
	defer u.syncListSelection()
	if release {
		return
	}
	if code&64 != 0 {
		delta := 3
		if code&1 == 0 {
			delta = -3
		}
		if x >= u.listWidth(u.width) {
			u.scroll = max(0, u.scroll+delta)
			return
		}
		u.listMove(delta)
		return
	}
	if y < 2 || y >= 2+u.listVisibleHeight(u.height) || x >= u.listWidth(u.width) {
		return
	}
	rows := u.changeRows()
	index := u.listOffset + y - 2
	if index < 0 || index >= len(rows) {
		return
	}
	row := rows[index]
	if u.listSelected != row.id {
		u.scroll = 0
	}
	u.listSelected = row.id
	if code&32 == 0 && code&3 == 0 && row.dir {
		u.open[row.id] = !u.open[row.id]
	}
}
func countLabel(n int, unknown bool) string {
	if unknown {
		if n == 0 {
			return "?"
		}
		return fmt.Sprintf("%d?", n)
	}
	return fmt.Sprint(n)
}
func signedCount(sign string, n int, unknown bool) string {
	if n == 0 && !unknown {
		return ""
	}
	return sign + countLabel(n, unknown)
}
func textWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// paintHistogram uses eighth-cell blocks anchored against the center.
func paintHistogram(c *canvas, center, y, width, count, maximum, direction int, color rgb) {
	if count <= 0 || maximum <= 0 {
		return
	}
	units := max(1, int(float64(count)/float64(maximum)*float64(width*8)+0.5))
	units = min(width*8, units)
	for i := 0; i < (units+7)/8; i++ {
		x := center + direction*(i+1)
		level := min(8, units-i*8)
		glyph := '█'
		if level < 8 {
			glyph = []rune(" ▏▎▍▌▋▊▉")[level]
		}
		if x < 0 || x >= c.w || y < 0 || y >= c.h {
			continue
		}
		bg := c.cells[y*c.w+x].bg
		c.put(x, y, glyph, color)
		c.cells[y*c.w+x].bg = bg
		if direction < 0 && level < 8 && x >= 0 && x < c.w && y >= 0 && y < c.h {
			cell := &c.cells[y*c.w+x]
			cell.ch = []rune(" █▉▊▋▌▍▎▏")[level+1]
			cell.color, cell.bg = cell.bg, color
			if level == 4 {
				cell.ch, cell.color, cell.bg = '▐', color, bg
			}
		}
	}
}
func (u *ui) filePercentage(row *changeRow) string {
	if row.dir {
		return ""
	}
	n := u.graph.nodes[row.id]
	if row.unknown || n == nil || !n.linesKnown || row.kind == "deleted" || n.lines == 0 && (row.added != 0 || row.removed != 0) {
		return "?"
	}
	if n.lines == 0 {
		return "0.00"
	}
	percent := (float64(row.added) + float64(row.removed)) / float64(n.lines) * 100
	if percent > 0 && percent < .005 {
		return "<.01"
	}
	decimals := 2
	if percent >= 100 {
		decimals = 0
	} else if percent >= 10 {
		decimals = 1
	}
	for ; decimals >= 0; decimals-- {
		label := fmt.Sprintf("%.*f", decimals, percent)
		if len(label) <= 4 {
			return label
		}
	}
	return ">999"
}

func (u *ui) renderList(w, h int, now time.Time) string {
	return u.renderListCanvas(w, h, now).ansi()
}
func (u *ui) renderListCanvas(w, h int, now time.Time) *canvas {
	c := newCanvas(w, h)
	treeWidth := u.listWidth(w)
	u.renderSessionHeader(c)
	countWidth := max(1, min(9, (treeWidth-10)/4))
	hitsWidth := 1
	if treeWidth >= 40 {
		hitsWidth = 4
		countWidth = 5
	}
	removedWidth := 0
	var measureCounts func(*changeRow)
	measureCounts = func(r *changeRow) {
		removedWidth = max(removedWidth, textWidth(signedCount("-", r.removed, r.unknown)))
		if treeWidth >= 40 {
			countWidth = max(countWidth, min(9, textWidth(signedCount("+", r.added, r.unknown))))
		}
		for _, child := range r.children {
			measureCounts(child)
		}
	}
	for _, child := range u.history.tree().children {
		measureCounts(child)
	}
	addedX := 2 + hitsWidth
	if treeWidth >= 40 {
		addedX++
	}
	// Reserve path space before sizing the symmetric meter.
	barSide := max(0, min(4, (treeWidth-treeWidth/3-2*countWidth-8)/2))
	if treeWidth >= 40 {
		barSide = max(0, min(4, (treeWidth-max(20, treeWidth/4)-hitsWidth-countWidth-7-4-9)/2))
	}
	barX := addedX + countWidth + 1
	barWidth := 0
	if barSide > 0 {
		barWidth = 2*barSide + 1
	}
	removedX := barX + barWidth + 1
	if treeWidth >= 40 {
		removedWidth = max(7, min(9, removedWidth))
	} else {
		removedWidth = min(countWidth, removedWidth)
	}
	treeX := removedX + removedWidth + 1
	if treeWidth < 20 {
		treeX = 0
		barSide = 0
	}
	percentX := treeX
	percentWidth := 0
	if treeWidth >= 40 {
		percentWidth = 4
		treeX += percentWidth + 1
	}
	if treeWidth >= 20 {
		c.text(2, 1, "HITS", gray, hitsWidth)
		c.text(addedX, 1, "ADDED", gray, countWidth)
		c.text(removedX, 1, "REMOVED", gray, removedWidth)
		c.text(percentX, 1, "%", gray, percentWidth)
	}
	c.text(treeX, 1, headerPath(path.Base(u.root), treeWidth-treeX), white, treeWidth-treeX)
	rows := u.changeRows()
	visible := u.listVisibleHeight(h)
	selected := 0
	for i, r := range rows {
		if r.id == u.listSelected {
			selected = i
			break
		}
	}
	if selected < u.listOffset {
		u.listOffset = selected
	}
	if visible > 0 && selected >= u.listOffset+visible {
		u.listOffset = selected - visible + 1
	}
	u.listOffset = max(0, min(u.listOffset, max(0, len(rows)-visible)))
	if len(rows) == 0 {
		if visible > 0 {
			c.text(2, 2, "No changes observed since startup.", gray, treeWidth-4)
		}
	}
	// Every file is normalized against the largest individual file edit in
	// its repository, whether or not that file is currently visible. Collapsed
	// folder sums may exceed the scale and are capped by the meter renderer.
	maximum := make(map[string]int)
	for _, r := range u.history.entries {
		if !r.dir {
			group := u.histogramGroup(r.id)
			maximum[group] = max(maximum[group], r.added, r.removed)
		}
	}
	for i := u.listOffset; i < len(rows) && i-u.listOffset < visible; i++ {
		row := rows[i]
		y := 2 + i - u.listOffset
		color := green
		if row.fresh {
			color = blue
		}
		addedLabel := signedCount("+", row.added, row.unknown)
		if treeWidth >= 20 {
			if row.modifications > 0 {
				spare := hitsWidth
				label := fmt.Sprint(row.modifications)
				if len(label) > spare && spare > 0 {
					label = label[:spare-1] + "…"
				}
				c.text(2, y, label, white, spare)
			}
			c.text(addedX+max(0, countWidth-textWidth(addedLabel)), y, addedLabel, color, min(countWidth, textWidth(addedLabel)))
			// Align both counts toward the meter: one padding cell on either side.
			c.text(removedX, y, signedCount("-", row.removed, row.unknown), red, removedWidth)
		}

		c.text(percentX, y, u.filePercentage(row), gray, percentWidth)
		prefix := ""
		if row.depth > 1 {
			prefix = strings.Repeat("│ ", row.depth-2) + "├─ "
		}
		icon := "  "
		if row.dir {
			icon = "▸ "
			if u.open[row.id] {
				icon = "▾ "
			}
		}
		c.text(treeX, y, prefix+icon, gray, treeWidth-treeX)
		name := u.displayName(row.id)
		if row.id == "." {
			name = path.Base(u.root) + "/"
		} else if row.dir {
			name += "/"
		}
		age := now.Sub(row.changed)
		strength := 0.0
		flashKind := row.flashKind
		if flashKind == "" {
			flashKind = row.kind
		}
		flashColor := green
		switch flashKind {
		case "deleted":
			flashColor = red
		case "created":
			flashColor = blue
		case "modified":
			flashColor = green
		default:
			if row.fresh {
				flashColor = blue
			}
		}
		if !row.changed.IsZero() {
			strength = listBurstStrength(age)
		}
		if strength > 0 {
			c.put(1, y, '✦', blend(background, flashColor, strength))
		}
		nameColor := blend(gray, white, strength)
		// Switch to dark ink while the pastel flash is bright; white-on-pastel
		// loses the path precisely when a new event should be most noticeable.
		if strength >= 0.5 {
			nameColor = background
		}
		if row.kind == "deleted" {
			nameColor = red
			if strength >= 0.5 {
				nameColor = blend(red, background, 0.85)
			}
		}
		nameX := treeX + textWidth(prefix+icon)
		c.text(nameX, y, name, nameColor, treeWidth-nameX-1)
		if row.kind == "deleted" {
			for x := max(0, nameX); x < min(treeWidth-1, nameX+textWidth(safeText(name))); x++ {
				c.cells[y*w+x].strike = true
			}
		}
		baseBG := background
		if row.id == u.listSelected {
			c.put(0, y, '›', white)
			baseBG = selectedBackground
		}
		rowBG := u.listBurstBackground(baseBG, flashColor, strength)
		for x := 0; x < min(treeWidth, w); x++ {
			c.cells[y*w+x].bg = rowBG
		}
		if barSide > 0 && (!row.dir || !u.open[row.id]) && (row.added > 0 || row.removed > 0) {
			center := barX + barSide
			c.put(center, y, '│', gray)
			c.cells[y*w+center].bg = rowBG
			paintHistogram(c, center, y, barSide, row.added, maximum[u.histogramGroup(row.id)], -1, color)
			paintHistogram(c, center, y, barSide, row.removed, maximum[u.histogramGroup(row.id)], 1, red)
		}
	}
	var focus *changeRow
	for _, r := range rows {
		if r.id == u.listSelected {
			focus = r
			break
		}
	}
	if treeWidth < w {
		previewRows := 0
		for y := 1; y < 2+visible; y++ {
			c.put(treeWidth, y, '│', gray)
		}
		if u.dividerHover == 1 && u.dividerY >= 1 && u.dividerY < 2+visible {
			c.put(treeWidth, u.dividerY, '↔', white)
		}
		if focus != nil {
			px := treeWidth + 2
			pw := w - px - 1
			c.text(px, 1, u.displayPath(focus.id), white, pw)
			if focus.dir {
				c.text(px, 3, "Folder totals include changed descendants.", gray, pw)

			} else {
				kind := focus.kind
				if focus.fresh && kind == "modified" {
					kind = "new file · modified"
				}
				c.text(px, 2, kind, gray, pw)
				lines := strings.Split(focus.diff, "\n")
				if focus.diff == "" {
					lines = []string{"No text diff available."}
				}
				n := max(0, visible-2)
				u.scroll = min(u.scroll, max(0, len(lines)-n))
				for i := 0; i < n && i+u.scroll < len(lines); i++ {
					line := lines[i+u.scroll]
					color := gray
					if strings.HasPrefix(line, "+") {
						color = green
						if focus.fresh {
							color = blue
						}
					} else if strings.HasPrefix(line, "-") {
						color = red
					}
					c.text(px, 4+i, diffText(line), color, pw)
				}
				if focus.diff != "" {
					previewRows = min(n, len(lines)-u.scroll)
				}
			}
		}
		u.previewCells(c, treeWidth+2, 4, w-treeWidth-3, previewRows)
	}
	if !u.combined {
		renderFooter(c, "q quit · Tab view · ↑↓ · ←→ · wheel · J/K diff · drag diff select")
	}
	return c
}
