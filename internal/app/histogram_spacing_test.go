package app

import (
	"strings"
	"testing"
	"time"
)

func TestHistogramHasEqualSpacingToBothCounts(t *testing.T) {
	for _, width := range []int{40, 60, 100, 160} {
		u := newUI(newGraph(), "/repo")
		u.recordChange("equal.txt", "modified", false, "", 12, 12, true, time.Now().Add(-time.Minute))
		rows := strings.Split(plainList(u.renderList(width, 24, time.Now())), "\r\n")
		row := []rune(rows[1])
		center := -1
		for i, r := range row {
			if r == '│' {
				center = i
				break
			}
		}
		if center < 0 {
			t.Fatalf("width %d: missing histogram", width)
		}
		left, right := center-1, center+1
		for left >= 0 && row[left] == '█' {
			left--
		}
		for right < len(row) && row[right] == '█' {
			right++
		}
		leftGap := 0
		for left >= 0 && row[left] == ' ' {
			leftGap++
			left--
		}
		rightGap := 0
		for right < len(row) && row[right] == ' ' {
			rightGap++
			right++
		}
		if leftGap != rightGap || leftGap != 1 {
			t.Errorf("width %d: unequal spacing: added=%d removed=%d, %q", width, leftGap, rightGap, rows[1])
		}
		if right >= len(row) || row[right] != '-' {
			t.Errorf("width %d: removed count should follow its single-cell gap", width)
		}
	}
}
