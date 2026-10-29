package app

import (
	"strings"
	"testing"
	"time"
)

func TestModificationTotalsSumOnlyDescendants(t *testing.T) {
	h := newHistory()
	h.record("dir/a", "modified", false, "", 2, 1, true, time.Time{})
	h.record("dir/b", "modified", false, "", 3, 0, true, time.Time{})
	h.entries["dir/a"].modifications = 4
	h.entries["dir/b"].modifications = 2
	if h.tree().modifications != 6 || h.tree().children[0].modifications != 6 {
		t.Fatal("folder must sum actual descendant modifications")
	}
	u := newUI(newGraph(), "/repo")
	u.graph.upsert("quiet", ".", false)
	u.selected = "quiet"
	if u.presentationTree().children[0].modifications != 0 {
		t.Fatal("context must stay zero")
	}
}

func TestModificationCountsWhiteAndZeroBlank(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("a", "modified", false, "+preview", 2, 1, true, time.Time{})
	u.history.entries["a"].modifications = 7
	for _, width := range []int{20, 40, 100} {
		c := u.renderListCanvas(width, 12, time.Now())
		found := false
		for x := 0; x < width; x++ {
			v := c.cells[c.w+x]
			if v.ch == '7' && v.color == white {
				found = true
			}
		}
		if !found {
			t.Fatalf("width %d: modification count missing", width)
		}
	}
	c := u.renderListCanvas(100, 12, time.Now())
	countAt := -1
	for x := 0; x < 30; x++ {
		v := c.cells[c.w+x]
		if v.ch == '7' {
			countAt = x
			if v.color != white {
				t.Fatal("modification count not white")
			}
		}
	}
	if countAt < 0 {
		t.Fatal("modification count missing")
	}
	u.history.entries["a"].modifications = 0
	u.history.cached = nil
	c = u.renderListCanvas(100, 12, time.Now())
	if c.cells[c.w+countAt].ch != ' ' && c.cells[c.w+countAt].ch != 0 {
		t.Fatal("zero count must be blank")
	}
}

func TestNarrowModificationCountNeverLooksLikeDifferentCount(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("a", "modified", false, "", 2, 1, true, time.Time{})
	u.history.entries["a"].modifications = 1234567
	c := u.renderListCanvas(20, 8, time.Now())
	if c.cells[20+2].ch != '…' || c.cells[20+3].ch != '+' || c.cells[20+4].ch != '2' {
		t.Fatal("narrow count must signal truncation without overwriting added count")
	}
}

func TestCompactRemovedGapAndNarrowUnicodeRows(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("界界.txt", "modified", false, "+preview", 2, 1, true, time.Time{})
	for _, w := range []int{1, 2, 8, 12, 20, 40, 80, 100, 160} {
		c := u.renderListCanvas(w, 12, time.Now())
		if len(c.cells) != w*12 {
			t.Fatalf("width %d: canvas overflow", w)
		}
		lines := strings.Split(plainList(c.ansi()), "\r\n")
		for _, line := range lines {
			if textWidth(line) != w {
				t.Fatalf("width %d: serialized row width %d", w, textWidth(line))
			}
		}
		if w == 100 {
			line := lines[1]
			removed := strings.Index(line, "-1")
			name := strings.Index(line, "界")
			if removed < 0 || name < 0 || name-(removed+2) > 3 {
				t.Fatalf("excess removed padding: %q", line)
			}
		}
	}
}
