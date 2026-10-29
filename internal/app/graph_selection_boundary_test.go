package app

import (
	"testing"
	"time"
)

func TestSelectionHighlightsExistingDotsDuringFlash(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.combined, u.paused = true, true
	at := time.Unix(100, 0)
	events := []changeEvent{{Path: "file", Kind: "modified", Added: 80, StatsKnown: true, At: at}}
	applySnapshot(u.graph, nil, events, at)
	u.recordEvents(events)
	now := at.Add(125 * time.Millisecond)
	before := u.renderGraphCanvas(100, 40, now)
	u.selected = "file"
	after := u.renderGraphCanvas(100, 40, now)
	whitened := 0
	for i, v := range after.cells {
		if v.dots != before.cells[i].dots {
			t.Fatalf("flash selection changed dots at (%d,%d)", i%after.w, i/after.w)
		}
		if v.dots != 0 && v.color == white && before.cells[i].color != white {
			whitened++
		}
	}
	if whitened == 0 {
		t.Fatal("selection did not highlight existing dots during flash")
	}
}

func TestSelectionPreservesSparseRenderedDots(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 100, 40
	n := &node{id: "sparse", x: 30, y: 10, radius: 6, kind: "modified", added: 2}
	u.graphRadii = map[string]float64{n.id: 6}
	before, after := newCanvas(100, 40), newCanvas(100, 40)
	u.drawFileDisk(before, n, 6)
	u.selected = n.id
	u.drawFileDisk(after, n, 6)
	whiteCount := 0
	for i, got := range after.cells {
		if got.dots != before.cells[i].dots || got.ch != before.cells[i].ch {
			t.Fatalf("selection changed glyph at (%d,%d)", i%after.w, i/after.w)
		}
		if got.dots != 0 {
			if got.color != white {
				t.Fatal("selected dot not white")
			}
			whiteCount++
		}
	}
	if whiteCount == 0 {
		t.Fatal("sparse file's existing dots were not highlighted")
	}
}

func TestSelectionDoesNotAddOrRemoveOverlappingDots(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 100, 40
	a := &node{id: "a", x: 30, y: 10, kind: "modified", added: 30}
	b := &node{id: "b", x: 30, y: 10, kind: "modified", removed: 30}
	before, after := newCanvas(100, 40), newCanvas(100, 40)
	u.drawFileDisk(before, b, 3)
	u.drawFileDisk(before, a, 3)
	u.selected = a.id
	u.drawFileDisk(after, b, 3)
	u.drawFileDisk(after, a, 3)
	for i, v := range after.cells {
		if v.ch != before.cells[i].ch || v.dots != before.cells[i].dots {
			t.Fatalf("overlapping selection changed symbol at %d", i)
		}
	}
}
