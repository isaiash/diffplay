package app

import (
	"math"
	"sort"
)

func (u *ui) drawClusterSymbol(c *canvas, n *node, radius float64) {
	x, y := u.screen(n)
	x, y = x+.5, y+.5
	r := math.Max(.1, radius*u.zoom)
	body, core, fraction := bubblePalette(n)
	if n.kind == "" && n.added+n.removed == 0 && !n.unknown {
		body = muted
	}
	type point struct {
		x, y     int
		distance float64
	}
	var points []point
	for yy := int(math.Floor((y - r/2) * 4)); yy <= int(math.Ceil((y+r/2)*4)); yy++ {
		for xx := int((x - r) * 2); xx <= int((x+r)*2); xx++ {
			dx, dy := float64(xx)/2+.25-x, float64(yy)/2+.25-2*y
			if dx*dx+dy*dy <= r*r {
				points = append(points, point{xx, yy, dx*dx + dy*dy})
			}
		}
	}
	if len(points) == 0 {
		points = append(points, point{int(x * 2), int(y * 4), 0})
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].distance < points[j].distance })
	total := n.added + n.removed
	unit := max(1, 1+(total-1)/len(points))
	dots := len(points)
	if total > 0 {
		dots = min(dots, 1+(total-1)/unit)
	}
	// A terminal cell has a single foreground color. Assign whole cells by
	// radial distance so the nucleus stays central rather than bleeding into
	// a left/right split. The area ratio is quantized at terminal-cell resolution.
	var coreCells map[[2]int]bool
	coreBudget, bodyBudget := 0, dots
	if fraction > 0 {
		type dotCell struct {
			x, y, count int
			distance    float64
		}
		cells := make(map[[2]int]*dotCell)
		for _, p := range points {
			key := [2]int{p.x / 2, p.y / 4}
			if cells[key] == nil {
				dx, dy := float64(key[0])+.5-x, 2*(float64(key[1])+.5-y)
				cells[key] = &dotCell{x: key[0], y: key[1], distance: dx*dx + dy*dy}
			}
			cells[key].count++
		}
		ordered := make([]*dotCell, 0, len(cells))
		for _, cell := range cells {
			ordered = append(ordered, cell)
		}
		sort.Slice(ordered, func(i, j int) bool {
			if ordered[i].distance != ordered[j].distance {
				return ordered[i].distance < ordered[j].distance
			}
			if ordered[i].y != ordered[j].y {
				return ordered[i].y < ordered[j].y
			}
			return ordered[i].x < ordered[j].x
		})
		for {
			coreCells = make(map[[2]int]bool)
			coreCapacity := 0
			target := int(math.Round(float64(dots) * fraction))
			if fraction > 0 && dots > 1 {
				target = max(1, target)
			}
			for i, cell := range ordered {
				if target == 0 || i == len(ordered)-1 || coreCapacity >= target {
					break
				}
				coreCells[[2]int{cell.x, cell.y}] = true
				coreCapacity += cell.count
			}
			coreBudget = min(target, coreCapacity)
			bodyBudget = dots - coreBudget
			bodyCapacity := len(points) - coreCapacity
			if bodyBudget <= bodyCapacity {
				break
			}
			// Color-isolated cells can have less capacity than the full disk.
			// Increase aggregation instead of silently dropping counted dots.
			usable := max(1, bodyCapacity+coreBudget)
			unit = max(unit+1, 1+(total-1)/usable)
			dots = 1 + (total-1)/unit
		}
	}
	// A tiny minority must not become a whole eight-dot central character.
	if u.graphDotUnits == nil {
		u.graphDotUnits = make(map[string]int)
	}
	if total > 0 {
		u.graphDotUnits[n.id] = unit
	}
	for _, p := range points {
		color := body
		if coreCells[[2]int{p.x / 2, p.y / 4}] {
			if coreBudget == 0 {
				continue
			}
			coreBudget--
			color = core
		} else {
			if bodyBudget == 0 {
				continue
			}
			bodyBudget--
		}
		if p.y < 8 || p.y >= (c.h-2)*4 || p.x < 0 || p.x >= c.w*2 {
			continue
		}
		if u.selected == n.id && n.id != "" {
			color = white
		}
		c.dot(p.x, p.y, color)
	}
}
