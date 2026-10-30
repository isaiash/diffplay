package app

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

func TestFolderFrameContrastSurvivesCameraAndTinyViews(t *testing.T) {
	for _, tc := range []struct {
		name       string
		w, h       int
		zoom, x, y float64
		folder     folderEnvelope
	}{
		{"pan", 80, 30, 1, 3, 2, folderEnvelope{ID: "A", Left: 10, Top: 4, Width: 20, Height: 8}},
		{"zoom", 80, 30, 1.5, 20, 12, folderEnvelope{ID: "A", Left: 10, Top: 4, Width: 20, Height: 8}},
		{"clipped", 80, 30, 1, -11, 0, folderEnvelope{ID: "A", Left: 10, Top: 4, Width: 20, Height: 8}},
		{"tiny", 12, 9, 1, 0, 0, folderEnvelope{ID: "A", Left: 1, Top: 2, Width: 9, Height: 4}},
		{"reserved rows only", 3, 4, 1, 0, 0, folderEnvelope{ID: "A", Left: 0, Top: 0, Width: 2, Height: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.width, u.height, u.zoom, u.panX, u.panY = tc.w, tc.h, tc.zoom, tc.x, tc.y
			var baseline *canvas
			for _, selected := range []string{"", "A/file", ""} {
				u.selected = selected
				c := newCanvas(tc.w, tc.h)
				u.drawFolderFrame(c, tc.folder)
				want := "\x1b[22;38;2;128;128;128m"
				if selected != "" {
					want = "\x1b[22;97m"
				}
				borders, titles := 0, 0
				for i, v := range c.cells {
					if baseline != nil && (v.ch != baseline.cells[i].ch || v.dots != baseline.cells[i].dots) {
						t.Fatalf("selection changed frame glyph at cell %d", i)
					}
					if v.ch == 0 {
						continue
					}
					if y := i / c.w; y < 2 || y >= c.h-2 {
						t.Fatalf("frame overwrote reserved row %d", y)
					}
					if v.color.sgr(false) != want {
						t.Errorf("selected=%q glyph=%q SGR=%q, want %q", selected, v.ch, v.color.sgr(false), want)
					}
					if v.ch == 'A' || v.ch == '/' {
						titles++
					} else {
						borders++
					}
				}
				if tc.h > 4 && (borders == 0 || titles != 2) {
					t.Fatalf("fixture missing border/title: borders=%d titles=%d", borders, titles)
				}
				baseline = c
			}
		})
	}
}
