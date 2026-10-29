package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func changedUI() *ui {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	u.recordChange("src/small.go", "modified", false, "-old\n+new", 2, 1, true, now)
	u.recordChange("docs/new.md", "created", false, "+hello", 12, 0, true, now)
	u.recordChange("src/deleted.go", "deleted", false, "-bye", 0, 8, true, now)
	return u
}

func TestTabSwitchesBetweenGraphAndChangeList(t *testing.T) {
	u := changedUI()
	u.input([]byte("	"))
	if !u.combined {
		t.Fatal("Tab from graph must open combined view")
	}
	u.input([]byte("	"))
	if !u.listMode {
		t.Fatal("Tab must open changes view")
	}
	frame := u.render(100, 25, time.Now())
	for _, want := range []string{"repo ", "+14", "-9", "src", "small.go", "new.md", "deleted.go"} {
		if !strings.Contains(frame, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(frame, "\x1b[9m") {
		t.Fatal("deleted filename must use terminal strikethrough")
	}
	u.input([]byte("\t"))
	if u.listMode {
		t.Fatal("second Tab must restore graph")
	}
}

func TestTreeKeyboardMouseAndStableSelection(t *testing.T) {
	u := changedUI()
	u.input([]byte("\t"))
	u.render(100, 25, time.Now())
	u.input([]byte("\x1b[B"))
	if u.listSelected != "docs/new.md" {
		t.Fatalf("down selected %q", u.listSelected)
	}
	u.input([]byte("\x1b[A"))
	u.input([]byte("\x1b[D"))
	if u.open["docs"] {
		t.Fatal("left must collapse directory")
	}
	u.input([]byte("\x1b[C"))
	if !u.open["docs"] {
		t.Fatal("right must expand directory")
	}
	u.input([]byte("\x1b[C"))
	if u.listSelected != "docs/new.md" {
		t.Fatal("right in open directory must descend")
	}
	u.input([]byte("\x1b[D"))
	if u.listSelected != "docs" {
		t.Fatal("left from file must select parent")
	}
	u.recordChange("src/small.go", "modified", false, "+more", 30, 0, true, time.Now())
	u.render(100, 25, time.Now())
	if u.listSelected != "docs" {
		t.Fatal("live sorting must not move focus to a different path")
	}
	// The newly highest-ranked src directory is the first visible row (row 2).
	u.input([]byte("\x1b[<35;20;2M"))
	if u.listSelected != "src" {
		t.Fatalf("hover must select row, got %q", u.listSelected)
	}
	u.input([]byte("\x1b[<0;20;2M"))
	if u.open["src"] {
		t.Fatal("mouse click must collapse folder")
	}
	u.input([]byte("\x1b[<0;20;2M"))
	if !u.open["src"] {
		t.Fatal("mouse click must reopen folder")
	}
}

func TestListShowsOnlySessionChangesAndKeepsDeletedRows(t *testing.T) {
	u := changedUI()
	u.graph.upsert("untouched.txt", ".", false)
	u.graph.expire(time.Now().Add(time.Hour))
	u.input([]byte("\t"))
	frame := u.render(100, 25, time.Now())
	if strings.Contains(frame, "untouched.txt") || !strings.Contains(frame, "deleted.go") {
		t.Fatal("list must be session history, not live graph nodes")
	}
	for _, size := range [][2]int{{1, 1}, {12, 5}, {80, 24}, {180, 50}} {
		if u.render(size[0], size[1], time.Now()) == "" {
			t.Fatal("empty resize render")
		}
	}
}

func TestListScrollAndCollapsedBranchesSurviveLiveUpdates(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	for i := 0; i < 40; i++ {
		u.recordChange(fmt.Sprintf("src/file-%02d.go", i), "modified", false, "+x", i+1, 0, true, now)
	}
	u.input([]byte("\t"))
	u.render(80, 12, now)
	u.input([]byte("\x1b[F"))
	u.render(80, 12, now)
	if u.listOffset == 0 || u.listSelected != "src/file-00.go" {
		t.Fatal("End must scroll to the lowest-ranked changed file")
	}
	u.input([]byte("\x1b[5"))
	u.input([]byte("~"))
	if u.listSelected == "src/file-00.go" {
		t.Fatal("fragmented Page Up must navigate")
	}
	u.input([]byte("c"))
	if u.open["src"] {
		t.Fatal("collapse all must collapse folders")
	}
	u.recordChange("src/new.go", "created", false, "+new", 50, 0, true, now.Add(time.Second))
	u.render(80, 12, now)
	if u.open["src"] {
		t.Fatal("a live update must preserve a deliberate collapse")
	}
	u.input([]byte("a"))
	if !u.open["src"] {
		t.Fatal("expand all must reopen changed branches")
	}
	u.input([]byte("\x1b[<65;20;3M"))
	if u.listSelected == "." {
		t.Fatal("mouse wheel must move selection")
	}
}

func TestUnknownListCountsAreExplicit(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("binary.png", "modified", false, "Binary files differ", 0, 0, false, time.Now())
	u.input([]byte("\t"))
	frame := u.render(80, 24, time.Now())
	if !strings.Contains(frame, "+?") || !strings.Contains(frame, "-?") {
		t.Fatal("unknown statistics must not be displayed as zero")
	}
}

func TestListCountColorsAndChangeFlashFade(t *testing.T) {
	u := changedUI()
	u.input([]byte("\t"))
	now := time.Now()
	recent := u.render(100, 25, now)
	for _, color := range []string{green.sgr(false), blue.sgr(false), red.sgr(false)} {
		if !strings.Contains(recent, color) {
			t.Errorf("missing count color %s", color)
		}
	}
	stale := u.render(100, 25, now.Add(5*time.Second))
	if !strings.Contains(recent, "✦") || strings.Contains(stale, "✦") {
		t.Fatal("live flash must fade without dropping rows")
	}
}
