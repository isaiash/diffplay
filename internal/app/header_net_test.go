package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRepositoryHeaderDiffBar(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 200, known: true}}
	u.recordChange("new", "created", false, "", 60, 0, true, time.Time{})
	u.recordChange("old", "modified", false, "", 40, 40, true, time.Time{})
	c := u.renderListCanvas(100, 5, time.Time{})
	for x, cell := range c.cells[:c.w] {
		want := rgb{}
		switch {
		case x < 43:
			want = blue
		case x < 72:
			want = green
		default:
			want = red
		}
		if cell.bg != want {
			t.Fatalf("cell %d background = %+v, want %+v", x, cell.bg, want)
		}
		if want != (rgb{}) && cell.color != headerInk {
			t.Fatalf("cell %d lacks contrast", x)
		}
	}
}

func TestRepositoryHeaderEdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		total                 int
		known                 bool
		fresh, added, removed int
		percent               string
		widths                [4]int
	}{
		{"unknown corpus", 100, false, 60, 40, 40, "?%", [4]int{43, 29, 28, 0}},
		{"empty", 0, true, 0, 0, 0, "0.00%", [4]int{0, 0, 0, 100}},
		{"zero corpus additions", 0, true, 1, 0, 0, "?%", [4]int{100, 0, 0, 0}},
		{"zero corpus removals", 0, true, 0, 0, 1, "?%", [4]int{0, 0, 100, 0}},
		{"zero corpus balanced", 0, true, 1, 0, 1, "0.00%", [4]int{50, 0, 50, 0}},
		{"negative net", 100, true, 5, 5, 30, "-20.00%", [4]int{13, 12, 75, 0}},
		{"saturated", 40, true, 40, 40, 20, "150.00%", [4]int{40, 40, 20, 0}},
		{"two decimals", 300, true, 1, 0, 0, "0.33%", [4]int{100, 0, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.repositoryTotals = map[string]repositoryLineCount{"": {lines: tt.total, known: tt.known}}
			u.recordChange("new", "created", false, "", tt.fresh, 0, true, time.Time{})
			u.recordChange("old", "modified", false, "", tt.added, tt.removed, true, time.Time{})
			c := u.renderListCanvas(100, 5, time.Time{})
			line := headerText(c)
			if !strings.Contains(line, tt.percent+" new lines") {
				t.Fatalf("percentage: %q", line)
			}
			label := "? total lines"
			if tt.known {
				label = fmt.Sprintf("%d total lines", tt.total)
			}
			if !strings.HasSuffix(line, label) {
				t.Fatalf("total must be right aligned: %q", line)
			}
			x := 0
			for i, n := range tt.widths {
				want := [4]rgb{blue, green, red, {}}[i]
				for j := 0; j < n; j++ {
					if c.cells[x].bg != want {
						t.Fatalf("cell %d background = %+v, want %+v", x, c.cells[x].bg, want)
					}
					x++
				}
			}
		})
	}
}

func TestRepositoryHeaderOverflowNet(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	maxInt := int(^uint(0) >> 1)
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: maxInt, known: true}}
	u.recordChange("new", "created", false, "", maxInt, 0, true, time.Time{})
	u.recordChange("old", "modified", false, "", maxInt, maxInt, true, time.Time{})
	c := u.renderListCanvas(100, 5, time.Time{})
	if line := headerText(c); !strings.Contains(line, "100.00% new lines") {
		t.Fatalf("overflow-safe net percentage: %q", line)
	}
	for x, cell := range c.cells[:c.w] {
		want := red
		if x < 34 {
			want = blue
		} else if x < 67 {
			want = green
		}
		if cell.bg != want {
			t.Fatalf("cell %d = %+v, want %+v", x, cell.bg, want)
		}
	}
}

func TestRepositoryHeaderNetText(t *testing.T) {
	u := newUI(newGraph(), "/parent/project")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 250, known: true}}
	u.recordChange("x", "modified", false, "", 50, 21, true, time.Time{})
	line := headerText(u.renderListCanvas(80, 5, time.Time{}))
	if !strings.HasPrefix(line, "+50 -21 11.60% new lines") || !strings.HasSuffix(line, "250 total lines") || strings.Contains(line, "/parent") {
		t.Fatalf("repository header: %q", line)
	}
}
