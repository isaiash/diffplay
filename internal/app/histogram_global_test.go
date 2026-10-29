package app

import (
	"strings"
	"testing"
	"time"
)

// histogramCells captures the actual rendered meter, including glyph and ink.
func histogramCells(t *testing.T, c *canvas, name string) []cell {
	t.Helper()
	for y := 1; y < c.h-1; y++ {
		var line strings.Builder
		for x := 0; x < c.w; x++ {
			line.WriteRune(c.cells[y*c.w+x].ch)
		}
		if strings.Contains(line.String(), name) {
			// At width 100 the meter occupies columns 13 through 21.
			return append([]cell(nil), c.cells[y*c.w+13:y*c.w+22]...)
		}
	}
	t.Fatalf("row %q not visible", name)
	return nil
}

func TestHistogramScaleDoesNotChangeWhenScrolling(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("dominant", "modified", false, "", 100, 0, true, time.Time{})
	u.recordChange("target", "modified", false, "", 25, 50, true, time.Time{})
	u.recordChange("last", "modified", false, "", 10, 0, true, time.Time{})
	u.listSelected = "dominant"
	before := histogramCells(t, u.renderListCanvas(100, 4, time.Time{}), "target")
	u.listMove(2)
	after := histogramCells(t, u.renderListCanvas(100, 4, time.Time{}), "target")
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("target meter changed when dominant scrolled out at cell %d: before=%+v after=%+v", i, before, after)
		}
	}
	if before[3].ch != '█' || before[2].ch != 0 || before[5].ch != '█' || before[6].ch != '█' || before[7].ch != 0 {
		t.Fatalf("target must be normalized against 100 additions: %+v", before)
	}
}

func TestHistogramFolderAggregateDoesNotRescaleFiles(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("folder/a", "modified", false, "", 60, 0, true, time.Time{})
	u.recordChange("folder/b", "modified", false, "", 40, 0, true, time.Time{})
	u.recordChange("outside", "modified", false, "", 50, 0, true, time.Time{})
	u.listSelected = "outside"
	before := histogramCells(t, u.renderListCanvas(100, 10, time.Time{}), "outside")
	if before[3].ch != '█' || before[2].ch != '█' || before[1].ch != '█' || before[0].ch == 0 {
		t.Fatalf("file must use largest file (60) as scale: %+v", before)
	}
	u.open["folder"] = false
	c := u.renderListCanvas(100, 10, time.Time{})
	after := histogramCells(t, c, "outside")
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("outside meter changed when folder collapsed: before=%+v after=%+v", before, after)
		}
	}
	folder := histogramCells(t, c, "folder/")
	for i := 0; i < 4; i++ {
		if folder[i].ch != '█' || folder[i].color != green {
			t.Fatalf("collapsed aggregate must fill the scale in green: %+v", folder)
		}
	}
}

func TestHistogramScalesAreIndependentPerRepository(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	u.repositoryLabels = map[string]string{"repo-1": "large", "repo-2": "small"}
	u.recordChange("repo-1/large-file", "modified", false, "", 100, 0, true, time.Time{})
	u.recordChange("repo-1/quarter", "modified", false, "", 25, 0, true, time.Time{})
	u.recordChange("repo-2/small-file", "modified", false, "", 10, 0, true, time.Time{})
	u.recordChange("repo-2/half", "modified", false, "", 5, 0, true, time.Time{})
	u.listSelected = "repo-1/large-file"
	c := u.renderListCanvas(100, 12, time.Time{})
	quarter := histogramCells(t, c, "quarter")
	half := histogramCells(t, c, "half")
	if quarter[3].ch != '█' || quarter[2].ch != 0 || half[3].ch != '█' || half[2].ch != '█' || half[1].ch != 0 {
		t.Fatalf("repository scales must exclude group totals: quarter=%+v half=%+v", quarter, half)
	}
	small := histogramCells(t, c, "small-file")
	for i := 0; i < 4; i++ {
		if small[i].ch != '█' || small[i].color != green {
			t.Fatalf("small repository peak should fill its independent scale: %+v", small)
		}
	}
	u.listOffset = 0
	u.listSelected = "repo-1/large-file"
	before := histogramCells(t, u.renderListCanvas(100, 5, time.Time{}), "quarter")
	u.listSelected = "repo-2/small-file"
	after := histogramCells(t, u.renderListCanvas(100, 5, time.Time{}), "quarter")
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("repository meter changed when peak scrolled away: before=%+v after=%+v", before, after)
		}
	}
}

func TestHistogramIgnoresTemporaryUnchangedSelection(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("folder/a", "modified", false, "", 60, 0, true, time.Time{})
	u.recordChange("folder/b", "modified", false, "", 40, 0, true, time.Time{})
	u.recordChange("outside", "modified", false, "", 50, 0, true, time.Time{})
	u.graph.upsert("folder/quiet", "folder", false)
	u.listSelected = "outside"
	before := histogramCells(t, u.renderListCanvas(100, 10, time.Time{}), "outside")
	root := u.history.tree()
	u.selected = "folder/quiet"
	after := histogramCells(t, u.renderListCanvas(100, 10, time.Time{}), "outside")
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("unchanged row affected histogram: before=%+v after=%+v", before, after)
		}
	}
	if u.history.tree() != root || root.added != 150 || len(u.history.entries) != 3 {
		t.Fatalf("unchanged selection mutated history: root=%+v entries=%d", root, len(u.history.entries))
	}
}
