package main

import (
	"testing"
)

func TestRenderedDisksGrowWithRadius(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width = 80
	u.height = 24
	n := &node{id: "file", x: 25, y: 8, kind: "modified"}
	renderedArea := func(radius float64) int {
		c := newCanvas(80, 24)
		u.drawFileDisk(c, n, radius)
		count := 0
		for _, cell := range c.cells {
			upper, lower := solidCellColors(cell)
			if upper != background {
				count++
			}
			if lower != background {
				count++
			}
		}
		return count
	}
	small, large := renderedArea(.5), renderedArea(3)
	if large <= small*4 {
		t.Fatalf("larger line counts must produce visibly larger disks: %d vs %d", large, small)
	}
}

func TestDiskRenderingDoesNotPaintHeaderOrFooter(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width = 20
	u.height = 10
	for _, pos := range [][2]float64{{0, 0}, {19, 5}, {10, 8}} {
		c := newCanvas(20, 10)
		n := &node{x: pos[0], y: pos[1], kind: "created"}
		u.drawFileDisk(c, n, 3)
		for _, y := range []int{0, 1, 8, 9} {
			for x := 0; x < c.w; x++ {
				cell := c.cells[y*c.w+x]
				if cell.ch != 0 || cell.dots != 0 {
					t.Fatalf("disk crossed reserved terminal row %d", y)
				}
			}
		}
	}
}
