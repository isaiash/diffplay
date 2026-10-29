package main

import (
	"fmt"
	"math"
	"math/bits"
	"strings"
	"testing"
	"time"
)

func TestIdlePollDoesNotRemoveOrRepackDeletedFiles(t *testing.T) {
	g := newGraph()
	at := time.Unix(100, 0)
	applySnapshot(g, nil, []changeEvent{{Path: "src/gone", Kind: "deleted", At: at}}, at)
	g.step(100, 26, 0)
	n := g.nodes["src/gone"]
	x, y := n.x, n.y
	applySnapshot(g, nil, nil, at.Add(time.Hour))
	g.step(100, 26, .05)
	if g.nodes["src/gone"] != n || n.x != x || n.y != y {
		t.Fatal("idle polling must not remove files or change their positions")
	}
}

func TestHexHasStableNonoverlappingPositions(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	at := time.Unix(100, 0)
	for i := 0; i < 36; i++ {
		id := fmt.Sprintf("src/group%d/file%02d", i/12, i)
		event := changeEvent{Path: id, Kind: "modified", Added: i + 1, Removed: 2, StatsKnown: true, At: at}
		applySnapshot(u.graph, nil, []changeEvent{event}, at)
		u.recordEvents([]changeEvent{event})
	}
	u.updateGraphMetrics()
	u.graph.step(180, 56, 0)
	for pass := 0; pass < 3; pass++ {
		frame := u.render(180, 60, at.Add(time.Minute))
		nodes := u.fileNodes()
		for i, n := range nodes {
			for _, other := range nodes[:i] {
				if math.Hypot(n.x-other.x, 2*(n.y-other.y))+1e-8 < u.displayRadius(n)+u.displayRadius(other) {
					t.Fatalf("overlapping files %s / %s", n.id, other.id)
				}
			}
		}
		if frame != u.render(180, 60, at.Add(time.Hour)) {
			t.Fatal("Hex moves at rest")
		}
		n := nodes[0]
		x, y := u.screen(n)
		if u.hit(int(math.Round(x)), int(math.Round(y))) != n.id {
			t.Fatal("cannot hover Hex file")
		}
	}
}

func TestHexSymbolsPreserveBothEditDirections(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width, u.height = 80, 30
	n := &node{id: "a", x: 35, y: 12, radius: 3, added: 8, removed: 3, kind: "modified"}
	for _, fresh := range []bool{false, true} {
		n.fresh = fresh
		c := newCanvas(80, 30)
		u.drawFileDisk(c, n, 3)
		frame := c.ansi()
		addedColor := green.sgr(false)
		if fresh {
			addedColor = blue.sgr(false)
		}
		if !strings.Contains(frame, addedColor) || !strings.Contains(frame, red.sgr(false)) {
			t.Fatalf("fresh=%v hides one edit direction", fresh)
		}
		for _, cell := range c.cells {
			if cell.ch != 0 {
				t.Fatal("Hex symbols must use Braille dots, not solid glyphs")
			}
		}
	}
}

func TestHexKeepsSmallChangesVisibleBesideLargeChanges(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	at := time.Unix(100, 0)
	events := []changeEvent{{Path: "src/small", Kind: "modified", Added: 1, StatsKnown: true, At: at}, {Path: "src/large", Kind: "modified", Added: 100000, StatsKnown: true, At: at}}
	applySnapshot(u.graph, nil, events, at)
	u.recordEvents(events)

	u.render(120, 36, at.Add(time.Minute))
	a, b := u.graph.nodes["src/small"], u.graph.nodes["src/large"]
	if u.displayRadius(a) != u.displayRadius(b) {
		t.Fatal("HEX must reserve equal glyph capacity, not shrink single-line edits")
	}
	if u.graphDotUnits[a.id] != 1 || u.graphDotUnits[b.id] <= 1 {
		t.Fatal("dot scaling must distinguish single-line and large edits")
	}
	density := func(n *node) int {
		c := newCanvas(120, 36)
		u.drawFileDisk(c, n, u.displayRadius(n))
		count := 0
		for _, cell := range c.cells {
			count += bits.OnesCount8(cell.dots)
		}
		return count
	}
	if small, large := density(a), density(b); small != 1 || large <= small {
		t.Fatalf("equal-capacity cells must retain sparse versus dense edits: %d vs %d dots", small, large)
	}
}

func TestHexPauseKeepsPositions(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.graph.upsert("a", ".", false)

	u.render(100, 30, time.Unix(100, 0))
	u.paused = true
	n := u.graph.nodes["a"]
	n.x += 1
	want := n.x
	u.render(100, 30, time.Unix(200, 0))
	if n.x != want {
		t.Fatal("Hex ignores layout pause")
	}
}

func TestTabVisitsEveryVisualization(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	for _, mode := range []string{"graph", "combined", "list", "graph", "combined"} {
		frame := plainList(u.render(120, 30, time.Unix(100, 0)))
		if strings.Contains(frame, "% new lines") != (mode != "graph") {
			t.Fatalf("wrong content for %s", mode)
		}
		if strings.Contains(frame, "Diffplay  ") {
			t.Fatal("obsolete top bar")
		}
		u.input([]byte("\t"))
	}
}
