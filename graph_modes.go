package main

import (
	"math"
	"sort"
)

var clusterTitles = [...]string{"CLUSTERS", "SPIRAL", "HEX"}
var clusterLegends = [...]string{
	"contact rings · shell/core: dominant/minority · size: cumulative lines",
	"golden-angle spiral · pie sectors: green + / red - · blue: new additions",
	"hex lattice · dots: left + / right - · scaled per file · hover exact unit",
}

func (u *ui) drawClusterSymbol(c *canvas, n *node, radius float64) {
	x, y := u.screen(n)
	x, y = x+.5, y+.5
	r := clamp(radius*u.zoom, .1, 12)
	addColor := green
	if n.fresh || n.kind == "created" {
		addColor = blue
	}
	if n.unknown || n.added+n.removed == 0 || r < 1 {
		symbol, color := '·', gray
		switch {
		case n.unknown:
			symbol = '?'
		case n.added > 0 && n.removed > 0:
			symbol = '±'
			color = white
		case n.removed > 0:
			symbol = '−'
			color = red
		case n.added > 0:
			symbol = '+'
			color = addColor
		}
		if y >= 2 && y < float64(c.h-2) {
			c.put(int(x), int(y), symbol, color)
		}
		return
	}
	if u.graphMode == 1 {
		fraction := float64(n.added) / float64(n.added+n.removed)
		for yy := int((y - r/2) * 2); yy <= int((y+r/2)*2); yy++ {
			if yy < 4 || yy >= (c.h-2)*2 {
				continue
			}
			for xx := int(x - r); xx <= int(x+r); xx++ {
				dx, dy := float64(xx)+.5-x, float64(yy)+.5-2*y
				if dx*dx+dy*dy > r*r {
					continue
				}
				angle := math.Mod(math.Atan2(dy, dx)+math.Pi/2+2*math.Pi, 2*math.Pi)
				color := red
				if angle < 2*math.Pi*fraction {
					color = addColor
				}
				c.solidPixel(xx, yy, color)
			}
		}
		return
	}
	// Braille dots are real subcell marks, not a text list. Separate terminal
	// columns keep additions and removals independently colored (one color/cell).
	type point struct {
		x, y     int
		distance float64
	}
	var halves [2][]point
	for yy := int((y - r/2) * 4); yy <= int((y+r/2)*4); yy++ {
		if yy < 8 || yy >= (c.h-2)*4 {
			continue
		}
		for xx := int((x - r) * 2); xx <= int((x+r)*2); xx++ {
			dx, dy := float64(xx)/2+.25-x, float64(yy)/2+.25-2*y
			if math.Abs(dx)+math.Abs(dy) > r {
				continue
			}
			side := 0
			if xx/2 >= int(x) {
				side = 1
			}
			halves[side] = append(halves[side], point{xx, yy, dx*dx + dy*dy})
		}
	}
	counts := [2]int{n.added, n.removed}
	colors := [2]rgb{addColor, red}
	unit := 1
	for side, points := range halves {
		if len(points) > 0 && counts[side] > 0 {
			unit = max(unit, 1+(counts[side]-1)/len(points))
		}
	}
	if u.graphDotUnits == nil {
		u.graphDotUnits = make(map[string]int)
	}
	u.graphDotUnits[n.id] = unit
	for side, points := range halves {
		sort.SliceStable(points, func(i, j int) bool { return points[i].distance < points[j].distance })
		dots := 0
		if counts[side] > 0 {
			dots = 1 + (counts[side]-1)/unit
		}
		for _, p := range points[:min(len(points), dots)] {
			c.dot(p.x, p.y, colors[side])
		}
	}
}

// Spatial hashing keeps collision tests local. Candidates follow a golden-angle
// spiral; no forces, randomness, or time-dependent relaxation are involved.
func packSpiral(circles []packedCircle) float64 {
	if len(circles) == 0 {
		return 0
	}
	largest := 0.0
	for _, c := range circles {
		largest = math.Max(largest, c.r)
	}
	cell := math.Max(.01, 2*largest)
	buckets := make(map[[2]int][]int)
	outer, candidate := 0.0, 0
	for i := range circles {
		c := &circles[i]
		for {
			angle := float64(candidate) * math.Pi * (3 - math.Sqrt(5))
			rho := largest * math.Sqrt(float64(candidate))
			candidate++
			x, y := rho*math.Cos(angle), rho*math.Sin(angle)
			key := [2]int{int(math.Floor(x / cell)), int(math.Floor(y / cell))}
			fits := true
			for dx := -1; dx <= 1 && fits; dx++ {
				for dy := -1; dy <= 1 && fits; dy++ {
					for _, j := range buckets[[2]int{key[0] + dx, key[1] + dy}] {
						other := circles[j]
						if math.Hypot(x-other.x, y-other.y)+1e-9 < c.r+other.r {
							fits = false
							break
						}
					}
				}
			}
			if fits {
				c.x, c.y = x, y
				buckets[key] = append(buckets[key], i)
				outer = math.Max(outer, rho+c.r)
				break
			}
		}
	}
	return outer
}

// Hexagonal axial rings provide a stable lattice with equal-size slots. Unlike
// size-sorted contact packing, changing a file's size never changes its index.
func packHex(circles []packedCircle) float64 {
	if len(circles) == 0 {
		return 0
	}
	largest := 0.0
	for _, c := range circles {
		largest = math.Max(largest, c.r)
	}
	circles[0].x, circles[0].y = 0, 0
	outer, index := largest, 1
	directions := [6][2]int{{1, 0}, {0, 1}, {-1, 1}, {-1, 0}, {0, -1}, {1, -1}}
	for ring := 1; index < len(circles); ring++ {
		q, r := 0, -ring
		for _, d := range directions {
			for step := 0; step < ring && index < len(circles); step++ {
				c := &circles[index]
				c.x = 2 * largest * (float64(q) + float64(r)/2)
				c.y = math.Sqrt(3) * largest * float64(r)
				outer = math.Max(outer, math.Hypot(c.x, c.y)+c.r)
				index++
				q, r = q+d[0], r+d[1]
			}
		}
	}
	return outer
}
