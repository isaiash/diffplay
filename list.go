package main

import (
	"fmt"
	"path"
	"strings"
	"time"
)

var green = rgb{115, 211, 153}

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
	if len(u.history.entries) == 0 {
		return nil
	}
	var rows []*changeRow
	for _, child := range u.history.tree().children {
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
		u.listMove(-max(1, u.height-4))
	case "6~":
		u.listMove(max(1, u.height-4))
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
	if w >= 120 {
		return w - min(70, w/2) - 1
	}
	return w
}
func (u *ui) listMouse(code, x, y int, release bool) {
	if release {
		return
	}
	if code&64 != 0 {
		delta := 3
		if code&1 == 0 {
			delta = -3
		}
		u.listMove(delta)
		return
	}
	if y < 2 || y >= u.height-2 || x >= u.listWidth(u.width) {
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
func (u *ui) renderList(w, h int, now time.Time) string {
	c := newCanvas(w, h)
	treeWidth := u.listWidth(w)
	c.text(1, 0, "Diffplay  CHANGES", white, w-2)
	c.text(1, 1, u.root+"  |  session activity · sorted by added lines ↓", gray, w-2)
	countWidth := max(1, min(9, (treeWidth-10)/4))
	// Reserve a third of the pane for paths before sizing the symmetric meter.
	barSide := max(0, min(4, (treeWidth-treeWidth/3-2*countWidth-8)/2))
	barX := 3 + countWidth + 1
	barWidth := 0
	if barSide > 0 {
		barWidth = 2*barSide + 1
	}
	removedX := barX + barWidth + 1
	treeX := removedX + countWidth + 2
	rows := u.changeRows()
	visible := max(0, h-4)
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
		c.text(2, 4, "No changes observed since startup.", gray, w-4)
	}
	maximum := make(map[string]int)
	for i := u.listOffset; i < len(rows) && i-u.listOffset < visible; i++ {
		r := rows[i]
		if !r.dir || !u.open[r.id] {
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
		c.text(3, y, fmt.Sprintf("%*s", countWidth, signedCount("+", row.added, row.unknown)), color, countWidth)
		// Align both counts toward the meter: one padding cell on either side.
		c.text(removedX, y, fmt.Sprintf("%-*s", countWidth, signedCount("-", row.removed, row.unknown)), red, countWidth)

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
		c.text(treeX, y, prefix+icon, muted, treeWidth-treeX)
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
		if !row.changed.IsZero() && age >= 0 && age < burstDuration {
			strength = 1 - float64(age)/float64(burstDuration)
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
			baseBG = rgb{38, 40, 49}
		}
		rowBG := blend(baseBG, blend(flashColor, white, 0.45), strength)
		for x := 0; x < min(treeWidth, w); x++ {
			c.cells[y*w+x].bg = rowBG
		}
		if barSide > 0 && (!row.dir || !u.open[row.id]) && (row.added > 0 || row.removed > 0) {
			center := barX + barSide
			c.put(center, y, '│', muted)
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
		for y := 2; y < h-2; y++ {
			c.put(treeWidth, y, '│', muted)
		}
		if focus != nil {
			px := treeWidth + 2
			pw := w - px - 1
			c.text(px, 2, u.displayPath(focus.id), white, pw)
			if focus.dir {
				c.text(px, 4, "Folder totals include changed descendants.", gray, pw)
				c.text(px, 6, "Right / click: expand", muted, pw)
				c.text(px, 7, "Left / click: collapse", muted, pw)
			} else {
				kind := focus.kind
				if focus.fresh && kind == "modified" {
					kind = "new file · modified"
				}
				c.text(px, 3, kind, muted, pw)
				lines := strings.Split(focus.diff, "\n")
				if focus.diff == "" {
					lines = []string{"No text diff available."}
				}
				n := max(0, h-8)
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
					c.text(px, 5+i, line, color, pw)
				}
				c.text(px, h-3, "J/K scroll diff", muted, pw)
			}
		}
	}
	root := u.history.tree()
	footer := "Session totals:"
	for _, count := range []string{signedCount("+", root.added, root.unknown), signedCount("-", root.removed, root.unknown)} {
		if count != "" {
			footer += " " + count
		}
	}
	if focus != nil {
		footer += "  |  " + u.displayPath(focus.id)
	}
	if u.warning != "" {
		footer = "Warning: " + u.warning
	}
	c.text(1, h-2, footer, gray, w-2)
	c.text(1, h-1, "Tab graph/list · ↑↓ move · → expand · ← collapse · click/hover select · wheel scroll · a expand all · c collapse · q quit", muted, w-2)
	return c.ansi()
}
