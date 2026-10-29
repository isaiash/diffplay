package main

import (
	"math"
	"math/bits"
	"strings"
	"testing"
	"time"
)

func TestHexCellDotsHaveDominantBodyAndMinorityNucleus(t *testing.T) {
	for _, tc := range []struct {
		added, removed int
		fresh          bool
		body, core     rgb
	}{
		{300, 100, false, green, red}, {100, 300, false, red, green},
		{8, 3, false, green, red}, {3, 8, false, red, green},
		{300, 100, true, blue, red}, {100, 300, true, red, blue},
	} {
		u := newUI(newGraph(), "demo")
		u.width, u.height = 80, 30
		n := &node{id: "a", x: 30, y: 10, kind: "modified", added: tc.added, removed: tc.removed, fresh: tc.fresh}
		c := newCanvas(80, 30)
		u.drawFileDisk(c, n, 6)
		body, core := 0, 0
		farthestCore, nearestBody := 0.0, math.Inf(1)
		for i, cell := range c.cells {
			if cell.ch != 0 {
				t.Fatal("definitive view must render Braille dots, not solid blocks")
			}
			if cell.dots == 0 {
				continue
			}
			d := math.Hypot(float64(i%c.w)-30, 2*(float64(i/c.w)-10))
			switch cell.color {
			case tc.body:
				body += bits.OnesCount8(cell.dots)
				nearestBody = math.Min(nearestBody, d)
			case tc.core:
				core += bits.OnesCount8(cell.dots)
				farthestCore = math.Max(farthestCore, d)
			default:
				t.Fatalf("unexpected dot color %+v", cell.color)
			}
		}
		if body <= core || core == 0 {
			t.Fatalf("body=%d core=%d, want majority body and visible nucleus", body, core)
		}
		if farthestCore > nearestBody+1e-9 {
			t.Fatal("minority must be a central nucleus, not a left/right split")
		}
	}
}

func TestHexCellUnchangedContextUsesFaintDots(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 80, 30
	c := newCanvas(80, 30)
	u.drawFileDisk(c, &node{id: "clean", x: 30, y: 10}, 3)
	dots := 0
	for _, cell := range c.cells {
		if cell.dots != 0 {
			dots += bits.OnesCount8(cell.dots)
			if cell.color != muted {
				t.Fatalf("context should be faint gray, got %+v", cell.color)
			}
		}
	}
	if dots < 8 {
		t.Fatalf("unchanged file must have a contextual cell, got %d dots", dots)
	}
}

func TestTabCyclesCombinedListAndHex(t *testing.T) {
	u := newUI(newGraph(), "demo")
	for _, mode := range []string{"graph", "combined", "list", "graph", "combined"} {
		frame := plainList(u.render(120, 30, time.Unix(100, 0)))
		hasTotals := strings.Contains(frame, "% new lines")
		if hasTotals != (mode != "graph") {
			t.Fatalf("view %s did not render expected content", mode)
		}
		if strings.Contains(frame, "Diffplay  ") {
			t.Fatal("obsolete title bar")
		}
		u.input([]byte("\t"))
	}
}

func TestHexCellDotUnitMatchesRenderedCount(t *testing.T) {
	for _, radius := range []float64{1, 2, 3, 6} {
		for _, counts := range [][2]int{{8, 3}, {100, 3}, {100, 100}, {100000, 10000}} {
			u := newUI(newGraph(), "demo")
			u.width, u.height = 80, 30
			n := &node{id: "a", x: 30, y: 10, kind: "modified", added: counts[0], removed: counts[1]}
			c := newCanvas(80, 30)
			u.drawFileDisk(c, n, radius)
			dots := 0
			for _, cell := range c.cells {
				dots += bits.OnesCount8(cell.dots)
			}
			unit := u.graphDotUnits[n.id]
			want := 1 + (n.added+n.removed-1)/unit
			if dots != want {
				t.Fatalf("radius=%g counts=%v dots=%d want=%d (unit=%d)", radius, counts, dots, want, unit)
			}
		}
	}
}
