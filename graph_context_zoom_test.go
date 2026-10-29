package main

import (
	"math/bits"
	"testing"
)

func TestGrayCircleZoomPreservesRelativeArea(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height, u.zoom = 240, 120, 6
	area := func(radius float64) int {
		c := newCanvas(240, 120)
		// The viewport center stays stationary under zoom.
		u.drawFileDisk(c, &node{x: 120, y: 58}, radius)
		count := 0
		for _, cell := range c.cells {
			count += bits.OnesCount8(cell.dots)
		}
		return count
	}
	small, large := area(2), area(4)
	if large < small*3 || large > small*5 {
		t.Fatalf("zoom flattened gray circle areas: small=%d large=%d", small, large)
	}
}
