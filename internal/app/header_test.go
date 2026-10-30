package app

import (
	"strings"
	"testing"
	"time"
)

func headerText(c *canvas) string {
	var b strings.Builder
	for _, v := range c.cells[:c.w] {
		if v.ch == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteRune(v.ch)
		}
	}
	return b.String()
}

func TestSessionHeaderNarrowWidthPrioritizesTotalsAndPercentage(t *testing.T) {
	u := newUI(newGraph(), "/some/long/path")
	u.recordChange("x", "modified", false, "", 3, 2, true, time.Time{})
	line := headerText(u.renderListCanvas(20, 5, time.Time{}))
	if !strings.Contains(line, "+3 -2 ?% new lines") {
		t.Fatalf("narrow header must retain totals and percent: %q", line)
	}
}

func TestSessionHeaderLargeTotalsDoNotOverflow(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	maxInt := int(^uint(0) >> 1)
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 100, known: true}}
	u.recordChange("new", "created", false, "", maxInt, 0, true, time.Time{})
	u.recordChange("existing", "modified", false, "", maxInt, 0, true, time.Time{})
	c := u.renderListCanvas(80, 5, time.Time{})
	for x, cell := range c.cells[:c.w] {
		want := green
		if x < 40 {
			want = blue
		}
		if cell.bg != want {
			t.Fatalf("overflow at %d: %+v", x, cell.bg)
		}
	}
	if strings.Contains(headerText(c), "+-2") {
		t.Fatalf("overflowed total: %q", headerText(c))
	}
}

func TestSessionHeaderPercentageTracksEditsAndRepositoryTotal(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 250, known: true}}
	u.recordChange("x", "modified", false, "", 50, 21, true, time.Time{})
	if line := headerText(u.renderListCanvas(80, 5, time.Time{})); !strings.Contains(line, "11.60%") {
		t.Fatalf("expected 11.60%%: %q", line)
	}
	u.recordChange("x", "modified", false, "", 300, 0, true, time.Time{})
	if line := headerText(u.renderListCanvas(80, 5, time.Time{})); !strings.Contains(line, "131.60%") {
		t.Fatalf("expected cumulative >100%%: %q", line)
	}
	u.repositoryTotals[""] = repositoryLineCount{lines: 100, known: true}
	if line := headerText(u.renderListCanvas(80, 5, time.Time{})); !strings.Contains(line, "329.00%") {
		t.Fatalf("repository update not reflected: %q", line)
	}
	u.repositoryTotals[""] = repositoryLineCount{lines: 0, known: true}
	if line := headerText(u.renderListCanvas(80, 5, time.Time{})); !strings.Contains(line, "?%") {
		t.Fatalf("zero denominator must be unknown: %q", line)
	}
}

func TestSessionHeaderUnknownEditsDoNotShowFalsePercentOrColors(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 100, known: true}}
	u.recordChange("binary.png", "created", false, "", 0, 0, false, time.Time{})
	c := u.renderListCanvas(80, 5, time.Time{})
	if line := headerText(c); !strings.Contains(line, "0.00% new lines") || strings.Contains(line, "?%") || strings.Contains(line, "+?") {
		t.Fatalf("unknown object must not affect global line metrics: %q", line)
	}
	for _, cell := range c.cells[:c.w] {
		if cell.bg != (rgb{}) {
			t.Fatalf("object file must not affect global bar: %+v", cell.bg)
		}
	}
	if got := u.filePercentage(u.history.entries["binary.png"]); got != "?" {
		t.Fatalf("object file row should remain unknown: %q", got)
	}
}

func TestUnknownObjectIsExcludedFromGlobalMetricsBesideTextEdits(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 100, known: true}}
	u.recordChange("binary.png", "created", false, "", 0, 0, false, time.Time{})
	u.recordChange("source.go", "modified", false, "", 4, 2, true, time.Time{})
	c := u.renderListCanvas(80, 5, time.Time{})
	line := headerText(c)
	if !strings.Contains(line, "+4 -2 2.00% new lines") || strings.Contains(line, "?%") {
		t.Fatalf("unknown binary contaminated known text totals: %q", line)
	}
	if got := u.filePercentage(u.history.entries["binary.png"]); got != "?" {
		t.Fatalf("binary row must keep unknown marker: %q", got)
	}
}

func TestSessionHeaderEmptyAndRedOnly(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 100, known: true}}
	c := u.renderListCanvas(40, 5, time.Time{})
	if !strings.Contains(headerText(c), "0.00%") {
		t.Fatal(headerText(c))
	}
	for _, cell := range c.cells[:c.w] {
		if cell.bg != (rgb{}) {
			t.Fatal("empty history must have no color")
		}
	}
	u.recordChange("x", "deleted", false, "", 0, 5, true, time.Time{})
	c = u.renderListCanvas(40, 5, time.Time{})
	for _, cell := range c.cells[:c.w] {
		if cell.bg != red {
			t.Fatalf("red-only background: %+v", cell.bg)
		}
	}
}

func TestSessionHeaderLargestRemainder(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 2, known: true}}
	u.recordChange("a", "created", false, "", 1, 0, true, time.Time{})
	u.recordChange("b", "modified", false, "", 1, 1, true, time.Time{})
	c := u.renderListCanvas(10, 5, time.Time{})
	for x, cell := range c.cells[:c.w] {
		want := red
		if x < 4 {
			want = blue
		} else if x < 7 {
			want = green
		}
		if cell.bg != want {
			t.Fatalf("cell %d = %+v, want %+v", x, cell.bg, want)
		}
	}
}

func TestSessionHeaderSplitAndStandaloneMatch(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("x", "modified", false, "", 3, 2, true, time.Time{})
	standalone := u.renderListCanvas(80, 5, time.Time{})
	u.combined = true
	split := u.renderCombined(80, 12, time.Time{})
	for x := 0; x < 80; x++ {
		if standalone.cells[x] != split.cells[x] {
			t.Fatalf("header mismatch at %d", x)
		}
	}
}

func TestSessionHeaderOmitsRepositoryName(t *testing.T) {
	u := newUI(newGraph(), "/very/long/path/项目/目录/file.go")
	u.recordChange("x", "modified", false, "", 3, 2, true, time.Time{})
	c := u.renderListCanvas(48, 5, time.Time{})
	line := headerText(c)
	if !strings.HasPrefix(line, "+3 -2 ?% new lines") || strings.Contains(line, "file.go") || !strings.HasSuffix(line, "? total lines") {
		t.Fatalf("must omit repository name but retain edits, percentage and right-aligned repository total: %q", line)
	}
	for x, cell := range c.cells[:c.w] {
		if runeWidth(cell.ch) == 2 && (x+1 >= c.w || c.cells[x+1].ch != 0) {
			t.Fatalf("wide character truncated at %d", x)
		}
	}
}

func TestSessionHeaderUnknownPercentage(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("x", "modified", false, "", 3, 2, true, time.Time{})
	line := headerText(u.renderListCanvas(80, 5, time.Time{}))
	if !strings.Contains(line, "?%") {
		t.Fatalf("unavailable repository denominator must show ?%%: %q", line)
	}
}

func TestSessionHeaderColorsUseLeafEditsOnce(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 10, known: true}}
	u.recordChange("folder", "modified", true, "", 90, 90, true, time.Time{})
	u.recordChange("folder/new", "created", false, "", 2, 0, true, time.Time{})
	u.recordChange("folder/new", "modified", false, "", 0, 1, true, time.Time{})
	u.recordChange("folder/existing", "modified", false, "", 3, 4, true, time.Time{})
	c := u.renderListCanvas(100, 5, time.Time{})
	for x := 0; x < c.w; x++ {
		want := red
		if x < 20 {
			want = blue
		} else if x < 50 {
			want = green
		}
		if got := c.cells[x].bg; got != want {
			t.Fatalf("cell %d background = %+v, want %+v", x, got, want)
		}
		if c.cells[x].color != headerInk {
			t.Fatalf("cell %d foreground must contrast with the bright background", x)
		}
	}
	if line := headerText(c); !strings.Contains(line, "+5 -5") {
		t.Fatalf("directory counts must not enter totals: %q", line)
	}
}

func TestSessionHeaderAlignsRepositoryTotalWithoutSeparator(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("x", "modified", false, "", 3, 2, true, time.Time{})
	line := headerText(u.renderListCanvas(80, 5, time.Time{}))
	if !strings.HasPrefix(line, "+3 -2") || strings.Contains(line, "|") || !strings.HasSuffix(line, "? total lines") {
		t.Fatalf("header must align metrics left and total right without a pipe: %q", line)
	}
}
