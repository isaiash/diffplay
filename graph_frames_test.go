package main

import (
	"strings"
	"testing"
	"time"
)

func TestGraphFramesEveryNestedFolder(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	for _, id := range []string{"A/C/file", "A/D/file", "B/file"} {
		applySnapshot(u.graph, nil, []changeEvent{{Path: id, Kind: "modified", Added: 30, Removed: 10, StatsKnown: true, At: time.Unix(100, 0)}}, time.Unix(100, 0))
	}
	u.recordEvents([]changeEvent{{Path: "A/C/file", Kind: "modified", Added: 30, Removed: 10, StatsKnown: true}})
	frame := plainList(u.render(200, 70, time.Unix(200, 0)))
	for _, glyph := range []string{"┌", "┐", "└", "┘", "─", "│"} {
		if !strings.Contains(frame, glyph) {
			t.Fatalf("missing folder frame glyph %s", glyph)
		}
	}
	for _, title := range []string{"A/", "C/", "D/", "B/"} {
		if !strings.Contains(frame, title) {
			t.Fatalf("missing framed folder %s", title)
		}
	}
	if count := strings.Count(frame, "┌"); count != 4 {
		t.Fatalf("want 4 nested folder frames without root, got %d", count)
	}
}

func TestFolderFramesFollowCameraAndClipReservedRows(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.width, u.height = 80, 30
	u.panX, u.panY = 3, 2
	c := newCanvas(80, 30)
	u.drawFolderFrame(c, folderEnvelope{ID: "A", Left: 10, Top: 4, Width: 20, Height: 8})
	if c.cells[6*c.w+13].ch != '┌' || c.cells[14*c.w+33].ch != '┘' {
		t.Fatal("folder frame did not follow pan")
	}
	u.panX, u.panY = -15, -12
	c = newCanvas(80, 30)
	u.drawFolderFrame(c, folderEnvelope{ID: "A", Left: 10, Top: 4, Width: 20, Height: 30})
	for _, y := range []int{0, 1, 28, 29} {
		for x := 0; x < c.w; x++ {
			if c.cells[y*c.w+x].ch != 0 {
				t.Fatal("folder border overwrote header/footer")
			}
		}
	}
	if c.cells[10*c.w].ch != 0 {
		t.Fatal("clipping invented a left border at viewport edge")
	}
}
