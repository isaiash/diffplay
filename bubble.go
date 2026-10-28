package main

import "math"

func (u *ui) displayRadius(n *node) float64 {
	if radius, ok := u.graphRadii[n.id]; ok {
		return radius
	}
	return max(.5, n.radius)
}

// The minority occupies an area fraction of the bubble, not a split side.
// A created file is uniformly blue; deletion overrides earlier new-file status.
func bubblePalette(n *node) (outer, core rgb, coreFraction float64) {
	if n.kind == "deleted" {
		return red, red, 0
	}
	if n.fresh || n.kind == "created" {
		return blue, blue, 0
	}
	if n.added > 0 && n.removed > 0 {
		total := float64(n.added) + float64(n.removed)
		if n.removed > n.added {
			return red, green, float64(n.added) / total
		}
		return green, red, float64(n.removed) / total
	}
	if n.removed > 0 {
		return red, red, 0
	}
	if n.added > 0 || n.kind == "modified" {
		return green, green, 0
	}
	return gray, gray, 0
}

// Full and half blocks paint contiguous areas; Braille glyphs contain visible
// gaps even when all their dots are enabled, so they are used only for ripples.
func solidCellColors(cell cell) (upper, lower rgb) {
	bg := cell.bg
	if bg == (rgb{}) {
		bg = background
	}
	switch cell.ch {
	case '█':
		return cell.color, cell.color
	case '▀':
		return cell.color, bg
	case '▄':
		return bg, cell.color
	}
	return bg, bg
}
func (c *canvas) solidPixel(x, halfY int, color rgb) {
	y := halfY / 2
	if x < 0 || halfY < 0 || x >= c.w || y >= c.h {
		return
	}
	upper, lower := solidCellColors(c.cells[y*c.w+x])
	if halfY%2 == 0 {
		upper = color
	} else {
		lower = color
	}
	if upper == lower {
		c.cells[y*c.w+x] = cell{ch: '█', color: upper}
	} else {
		c.cells[y*c.w+x] = cell{ch: '▀', color: upper, bg: lower}
	}
}
func (u *ui) drawSolidBubble(c *canvas, n *node, radius float64) {
	x, y := u.screen(n)
	x += .5
	y += .5
	radius = clamp(radius*u.zoom, .1, 12)
	outer, core, fraction := bubblePalette(n)
	coreSquared := radius * radius * fraction
	painted := false
	for yy := int(math.Floor((y - radius/2) * 2)); yy <= int(math.Ceil((y+radius/2)*2)); yy++ {
		if yy < 4 || yy >= (c.h-2)*2 {
			continue
		}
		for xx := int(math.Floor(x - radius)); xx <= int(math.Ceil(x+radius)); xx++ {
			dx := float64(xx) + .5 - x
			dy := (float64(yy)/2 + .25 - y) * 2
			d2 := dx*dx + dy*dy
			if d2 > radius*radius {
				continue
			}
			color := outer
			if fraction > 0 && d2 <= coreSquared {
				color = core
			}
			c.solidPixel(xx, yy, color)
			painted = true
		}
	}
	// Preserve a selectable marker below half-cell resolution without adding a
	// hollow glyph to the center of an otherwise solid disk.
	if !painted {
		yy := int(math.Round((y - .25) * 2))
		if yy >= 4 && yy < (c.h-2)*2 {
			c.solidPixel(int(math.Floor(x)), yy, outer)
		}
	}
}
