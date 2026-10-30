package app

import (
	"testing"
	"time"
)

func TestHeaderBarRepresentsOnlyDiffsRegardlessOfRepositoryTotal(t *testing.T) {
	for _, total := range []repositoryLineCount{{lines: 1000, known: true}, {lines: 1, known: true}, {known: false}} {
		u := newUI(newGraph(), "/repo")
		u.repositoryTotals = map[string]repositoryLineCount{"": total}
		u.recordChange("new", "created", false, "", 6, 0, true, time.Time{})
		u.recordChange("existing", "modified", false, "", 3, 1, true, time.Time{})
		c := u.renderListCanvas(100, 5, time.Time{})
		for x, cell := range c.cells[:c.w] {
			want := red
			if x < 60 {
				want = blue
			} else if x < 90 {
				want = green
			}
			if cell.bg != want {
				t.Fatalf("repository total %+v: cell %d = %+v, want %+v", total, x, cell.bg, want)
			}
		}
	}
}
