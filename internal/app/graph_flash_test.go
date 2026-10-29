package app

import (
	"math"
	"testing"
	"time"
)

func graphFlashFixture() (*ui, time.Time) {
	start := time.Unix(100, 0)
	u := newUI(newGraph(), "demo")
	u.combined = true
	events := []changeEvent{{Path: "file", Kind: "modified", Added: 80, Removed: 20, StatsKnown: true, At: start}}
	applySnapshot(u.graph, nil, events, start)
	u.recordEvents(events)
	return u, start
}

func TestGraphFileFlashTurnsDotsWhiteThenFadesQuickly(t *testing.T) {
	u, start := graphFlashFixture()
	whiteDots := func(at time.Time) int {
		c := u.renderGraphCanvas(120, 36, at)
		count := 0
		for _, cell := range c.cells {
			if cell.dots != 0 && cell.color == white {
				count++
			}
		}
		return count
	}
	if count := whiteDots(start); count == 0 {
		t.Fatal("first frame must flash actual file dots white")
	}
	if count := whiteDots(start.Add(400 * time.Millisecond)); count != 0 {
		t.Fatalf("white flash did not fade quickly: %d white cells", count)
	}
	u.graph.nodes["file"].quiet = true
	if count := whiteDots(start); count != 0 {
		t.Fatal("quiet startup must not flash")
	}
}

func TestGraphRippleExpandsQuicklyAndFades(t *testing.T) {
	u, start := graphFlashFixture()
	measure := func(age time.Duration) (float64, int, int) {
		c := u.renderGraphCanvas(120, 36, start.Add(age))
		n := u.graph.nodes["file"]
		x, y := u.screen(n)
		x += .5
		y += .5
		radius := u.displayRadius(n) * u.zoom
		far, brightness, count := 0.0, 0, 0
		for i, cell := range c.cells {
			if cell.dots == 0 {
				continue
			}
			d := math.Hypot(float64(i%c.w)+.5-x, 2*(float64(i/c.w)+.5-y))
			if d > radius+1.5 {
				far = math.Max(far, d)
				if cell.color != background {
					brightness += 2 - cell.color.b
				}
				count++
			}
		}
		if count > 0 {
			brightness /= count
		}
		return far, brightness, count
	}
	r1, b1, c1 := measure(150 * time.Millisecond)
	r2, b2, c2 := measure(800 * time.Millisecond)
	if c1 == 0 || c2 == 0 || r2-r1 < 4 || b2 >= b1 {
		t.Fatalf("ripple should expand rapidly while fading: early r=%g b=%d count=%d; late r=%g b=%d count=%d", r1, b1, c1, r2, b2, c2)
	}
	_, _, late := measure(1100 * time.Millisecond)
	if late != 0 {
		t.Fatalf("ripple lasted more than one second: %d cells", late)
	}
	u.graph.nodes["file"].quiet = true
	_, _, quiet := measure(150 * time.Millisecond)
	if quiet != 0 {
		t.Fatal("startup changes must not ripple")
	}
}
