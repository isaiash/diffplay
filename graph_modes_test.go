package main

import (
	"fmt"
	"math"
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

func TestClusterAlgorithmsHaveDistinctStableNonoverlappingPositions(t *testing.T) {
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
	var previous string
	for mode := 0; mode < 3; mode++ {
		u.graphMode = mode
		frame := u.render(180, 60, at.Add(time.Minute))
		var positions strings.Builder
		nodes := u.fileNodes()
		for i, n := range nodes {
			fmt.Fprintf(&positions, "%.5f,%.5f;", n.x, n.y)
			for _, other := range nodes[:i] {
				if math.Hypot(n.x-other.x, 2*(n.y-other.y))+1e-8 < u.displayRadius(n)+u.displayRadius(other) {
					t.Fatalf("mode %d overlapping files %s / %s", mode, n.id, other.id)
				}
			}
		}
		if mode > 0 && positions.String() == previous {
			t.Fatalf("mode %d still uses previous packing algorithm", mode)
		}
		previous = positions.String()
		if frame != u.render(180, 60, at.Add(time.Hour)) {
			t.Fatalf("mode %d moves at rest", mode)
		}
		n := nodes[0]
		x, y := u.screen(n)
		if u.hit(int(math.Round(x)), int(math.Round(y))) != n.id {
			t.Fatalf("mode %d cannot hover file", mode)
		}
	}
}

func TestClusterSymbolsAreDistinctAndPreserveBothEditDirections(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width, u.height = 80, 30
	n := &node{id: "a", x: 35, y: 12, radius: 3, added: 8, removed: 3, kind: "modified"}
	frames := make(map[string]bool)
	for mode := 0; mode < 3; mode++ {
		u.graphMode = mode
		c := newCanvas(80, 30)
		u.drawFileDisk(c, n, 3)
		frame := c.ansi()
		if frames[frame] {
			t.Fatalf("mode %d reuses the previous symbol", mode)
		}
		frames[frame] = true
		if !strings.Contains(frame, "38;2;115;211;153") || !strings.Contains(frame, "38;2;244;92;105") {
			t.Fatalf("mode %d hides one edit direction", mode)
		}
	}
}

func TestHexKeepsSmallChangesVisibleBesideLargeChanges(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	at := time.Unix(100, 0)
	events := []changeEvent{{Path: "src/small", Kind: "modified", Added: 1, StatsKnown: true, At: at}, {Path: "src/large", Kind: "modified", Added: 100000, StatsKnown: true, At: at}}
	applySnapshot(u.graph, nil, events, at)
	u.recordEvents(events)
	u.graphMode = 2
	u.render(120, 36, at.Add(time.Minute))
	a, b := u.graph.nodes["src/small"], u.graph.nodes["src/large"]
	if u.displayRadius(a) != u.displayRadius(b) {
		t.Fatal("HEX must reserve equal glyph capacity, not shrink single-line edits")
	}
	if u.graphDotUnits[a.id] != 1 || u.graphDotUnits[b.id] <= 1 {
		t.Fatal("dot scaling must distinguish single-line and large edits")
	}
}

func TestAlternativeClusterPauseKeepsPositions(t *testing.T) {
	for mode := 1; mode < 3; mode++ {
		u := newUI(newGraph(), "/repo")
		u.graph.upsert("a", ".", false)
		u.graphMode = mode
		u.render(100, 30, time.Unix(100, 0))
		u.paused = true
		n := u.graph.nodes["a"]
		n.x += 1
		want := n.x
		u.render(100, 30, time.Unix(200, 0))
		if n.x != want {
			t.Fatalf("mode %d ignores layout pause", mode)
		}
	}
}

func TestTabVisitsEveryVisualization(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	for _, title := range []string{"CLUSTERS", "CHANGES", "SPIRAL", "HEX", "CLUSTERS"} {
		frame := plainList(u.render(120, 30, time.Unix(100, 0)))
		if !strings.Contains(frame, "Diffplay  "+title) {
			t.Fatalf("expected %s, got %s", title, frame)
		}
		u.input([]byte("\t"))
	}
}
