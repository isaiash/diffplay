package app

import (
	"strings"
	"testing"
	"time"
)

func TestClusterDisksReuseListColorsWithoutFadingToGray(t *testing.T) {
	now := time.Now()
	u := newUI(newGraph(), "/repo")
	events := []changeEvent{{Path: "src/existing.go", Kind: "modified", Added: 8, Removed: 4, StatsKnown: true, At: now.Add(-time.Minute)}, {Path: "src/new.go", Kind: "created", Added: 4, StatsKnown: true, At: now.Add(-time.Minute)}}
	applySnapshot(u.graph, nil, events, now)
	u.recordEvents(events)
	u.graph.nodes["src/existing.go"].x = 20
	u.graph.nodes["src/existing.go"].y = 10
	u.graph.nodes["src/new.go"].x = 50
	u.graph.nodes["src/new.go"].y = 10
	frame := u.render(100, 30, now)
	for _, color := range []string{green.sgr(false), blue.sgr(false), red.sgr(false)} {
		if !strings.Contains(frame, color) {
			t.Errorf("missing persistent list palette %s", color)
		}
	}
	if strings.Contains(frame, "existing.go") || strings.Contains(frame, "new.go") {
		t.Fatal("disk names must be hidden until inspection")
	}
	u.selected = "src/existing.go"
	if frame = u.render(100, 30, now); !strings.Contains(frame, "existing.go") {
		t.Fatal("keyboard inspection must remain accessible")
	}
}

func TestGraphKeyboardSelectionSkipsFolderContext(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.graph.upsert("src", ".", true)
	u.graph.upsert("src/a", "src", false)
	u.graph.upsert("src/b", "src", false)
	u.graph.nodes["src/a"].kind = "modified"
	u.graph.nodes["src/b"].kind = "modified"
	u.input([]byte("n"))
	if u.selected != "src/a" {
		t.Fatal("next must select a file, not the directory")
	}
	u.input([]byte("n"))
	if u.selected != "src/b" {
		t.Fatal("next file not selected")
	}
	u.input([]byte("p"))
	if u.selected != "src/a" {
		t.Fatal("previous file not selected")
	}
}

func TestUnchangedHexContextUsesMutedDots(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width, u.height = 80, 30
	n := &node{id: "unchanged", x: 30, y: 10}
	c := newCanvas(80, 30)
	u.drawFileDisk(c, n, 3)
	painted := 0
	for _, cell := range c.cells {
		if cell.dots == 0 {
			continue
		}
		painted++
		if cell.color != muted {
			t.Fatalf("unchanged context must be muted gray, got %+v", cell.color)
		}
	}
	if painted == 0 {
		t.Fatal("unchanged context must remain visible")
	}
	if _, ok := u.graphDotUnits[n.id]; ok {
		t.Fatal("unchanged context must not claim cumulative edit units")
	}
}
