package main

import (
	"math"
	"testing"
)

func TestBubbleDominantShellAndMinorityCore(t *testing.T) {
	for _, tc := range []struct {
		added, removed int
		outer, inner   rgb
		ratio          float64
	}{
		{30, 10, green, red, .25}, {10, 30, red, green, .25}, {30, 0, green, green, 0}, {0, 30, red, red, 0},
	} {
		n := &node{kind: "modified", added: tc.added, removed: tc.removed}
		outer, inner, ratio := bubblePalette(n)
		if outer != tc.outer || inner != tc.inner || math.Abs(ratio-tc.ratio) > 1e-9 {
			t.Errorf("+%d -%d palette/core=%v %v %v", tc.added, tc.removed, outer, inner, ratio)
		}
	}
	n := &node{kind: "modified", fresh: true, added: 30, removed: 10}
	outer, inner, ratio := bubblePalette(n)
	if outer != blue || inner != blue || ratio != 0 {
		t.Fatal("a new file must stay entirely blue even after edits")
	}
	n.kind = "deleted"
	outer, inner, ratio = bubblePalette(n)
	if outer != red || inner != red || ratio != 0 {
		t.Fatal("deletion must override a formerly new file")
	}
}

func TestSolidBubbleHasNoInteriorGlyphHoles(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width = 80
	u.height = 30
	n := &node{x: 30, y: 10, kind: "modified", added: 30, removed: 10}
	c := newCanvas(80, 30)
	u.drawFileDisk(c, n, 6)
	// Actual render center is centered within the terminal cell.
	for yy := 0; yy < c.h; yy++ {
		for xx := 0; xx < c.w; xx++ {
			cell := c.cells[yy*c.w+xx]
			for half := 0; half < 2; half++ {
				dx := float64(xx) - 30
				dy := (float64(yy) + .25 + float64(half)*.5 - 12.5) * 2
				if dx*dx+dy*dy >= 25 {
					continue
				}
				upper, lower := solidCellColors(cell)
				color := upper
				if half == 1 {
					color = lower
				}
				if color != green && color != red {
					t.Fatalf("interior hole at %d,%d/%d: %+v", xx, yy, half, cell)
				}
			}
		}
	}
	centerUpper, centerLower := solidCellColors(c.cells[12*c.w+30])
	if centerUpper != red || centerLower != red {
		t.Fatal("minority red must fill the nucleus")
	}
	outerUpper, _ := solidCellColors(c.cells[12*c.w+35])
	if outerUpper != green {
		t.Fatal("dominant green must form the shell")
	}
}

func TestNewBubblePixelsAreEntirelyBlue(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width = 80
	u.height = 30
	n := &node{x: 30, y: 10, kind: "created", fresh: true, added: 30, removed: 10}
	c := newCanvas(80, 30)
	u.drawFileDisk(c, n, 4)
	painted := 0
	for _, cell := range c.cells {
		upper, lower := solidCellColors(cell)
		for _, color := range []rgb{upper, lower} {
			if color != background {
				painted++
				if color != blue {
					t.Fatalf("new-file bubble has a non-blue pixel: %+v", color)
				}
			}
		}
	}
	if painted == 0 {
		t.Fatal("bubble was not drawn")
	}
}
