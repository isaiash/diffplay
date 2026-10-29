package app

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
	if outer != blue || inner != red || ratio != .25 {
		t.Fatal("a fresh mixed file must retain its red minority nucleus")
	}
	n.kind = "deleted"
	outer, inner, ratio = bubblePalette(n)
	if outer != red || inner != red || ratio != 0 {
		t.Fatal("deletion must override a formerly new file")
	}
}

func TestNewBubblePixelsAreEntirelyBlue(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width = 80
	u.height = 30
	n := &node{x: 30, y: 10, kind: "created", fresh: true, added: 30}
	c := newCanvas(80, 30)
	u.drawFileDisk(c, n, 4)
	painted := 0
	for _, cell := range c.cells {
		if cell.dots != 0 {
			painted++
			if cell.color != blue {
				t.Fatalf("new-file bubble has a non-blue cell: %+v", cell)
			}
		}
	}
	if painted == 0 {
		t.Fatal("bubble was not drawn")
	}
}
