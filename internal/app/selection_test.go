package app

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func selectionUI() *ui {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	u.recordChange("dir/a", "modified", false, "+alpha\n+beta", 1, 0, true, time.Time{})
	u.listSelected = "dir/a"
	u.draw(140, 20, time.Time{})
	return u
}

func TestPreviewFrozenWhenLiveDiffDisappears(t *testing.T) {
	u := selectionUI()
	x := u.listWidth(140) + 2
	u.routeMouse(0, x, 4, false)
	u.history.entries["dir/a"].diff = ""
	u.history.cached = nil
	frame := u.draw(140, 20, time.Time{})
	if !strings.Contains(frame, "alpha") {
		t.Fatal("live update erased captured preview")
	}
	u.routeMouse(0, x+5, 4, true)
	if !strings.Contains(u.draw(140, 20, time.Time{}), base64.StdEncoding.EncodeToString([]byte("+alpha"))) {
		t.Fatal("did not copy frozen diff")
	}
}

func TestClipboardRepliesNeverDispatchKeys(t *testing.T) {
	u := selectionUI()
	u.input([]byte("\x1b]52;c;qqq\a"))
	if u.quit {
		t.Fatal("clipboard response dispatched quit key")
	}
}

func TestClipboardPendingWriteBounded(t *testing.T) {
	u := selectionUI()
	x := u.listWidth(140) + 2
	for i := 0; i < 100; i++ {
		u.routeMouse(0, x, 4, false)
		u.routeMouse(0, x+5, 4, true)
	}
	if strings.Count(u.pendingControl, "\x1b]52;") != 1 {
		t.Fatal("queued clipboard writes without bound")
	}
}

func TestPreviewUnicodeAndSpaces(t *testing.T) {
	u := selectionUI()
	u.history.entries["dir/a"].diff = "+	界\u0301 e\u0301  \r\n+second\n"
	u.history.cached = nil
	u.draw(140, 20, time.Time{})
	x := u.listWidth(140) + 2
	u.routeMouse(0, x, 4, false)
	u.routeMouse(32, 10000, 10000, false)
	u.routeMouse(0, 10000, 10000, true)
	out := u.draw(140, 20, time.Time{})
	encoded := strings.Split(strings.TrimPrefix(out, "\x1b]52;c;"), "\a")[0]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || string(decoded) != "+       界\u0301 e\u0301  \n+second" {
		t.Fatalf("clipboard=%q err=%v", decoded, err)
	}
	// Either half of a wide cell retains the glyph and combining marks exactly once.
	u.routeMouse(0, x+9, 4, false)
	u.routeMouse(0, x+9, 4, true)
	if !strings.Contains(u.draw(140, 20, time.Time{}), base64.StdEncoding.EncodeToString([]byte("界\u0301"))) {
		t.Fatal("wide continuation lost glyph")
	}
}

func TestPreviewGeometryAndOtherDrags(t *testing.T) {
	for _, combined := range []bool{false, true} {
		u := selectionUI()
		u.listMode = !combined
		u.combined = combined
		u.listSplitRatio = .6
		u.draw(140, 30, time.Time{})
		x := u.listWidth(140) + 2
		// Header, kind/status, gutter, footer, and divider never start selection.
		for _, point := range [][2]int{{x, 1}, {x, 2}, {x - 1, 4}, {x, 29}} {
			u.routeMouse(0, point[0], point[1], false)
			if u.selection != nil {
				t.Fatal("selected preview chrome")
			}
			u.routeMouse(0, point[0], point[1], true)
		}
		u.routeMouse(35, x, 4, false)
		if u.listSelected != "dir/a" {
			t.Fatal("preview hover selected list row")
		}
		u.routeMouse(0, x, 4, false)
		u.routeMouse(32, -100, 4, false)
		if u.selection == nil || !u.selection.dragging {
			t.Fatal("lost cross-pane capture")
		}
		u.draw(140, 30, time.Time{})
		u.routeMouse(0, x+5, 4, true)
		if !strings.Contains(u.draw(140, 30, time.Time{}), base64.StdEncoding.EncodeToString([]byte("+alpha"))) {
			t.Fatal("wrong moved-divider copy")
		}
		if combined {
			y := u.splitTop(30) + 3
			u.routeMouse(0, 10, y, false)
			u.routeMouse(32, 15, y+2, false)
			u.routeMouse(0, 15, y+2, true)
			if u.panX != 5 || u.panY != 2 || u.selection != nil {
				t.Fatal("preview captured graph pan")
			}
		}
	}
}

func TestCombinedPreviewDragRestoresListKeyboardFocus(t *testing.T) {
	u := selectionUI()
	u.listMode, u.combined = false, true
	u.graph.upsert("dir", ".", true)
	u.graph.upsert("dir/a", "dir", false)
	u.graph.upsert("dir/b", "dir", false)
	u.draw(140, 30, time.Time{})
	graphY := u.splitTop(30) + 2
	u.routeMouse(64, 0, graphY, false)
	if !u.graphFocus {
		t.Fatal("graph interaction did not establish graph focus")
	}

	x := u.listWidth(140) + 2
	u.routeMouse(0, x, 4, false)
	if u.selection == nil || u.graphFocus {
		t.Fatal("preview press did not start selection with list focus")
	}
	u.routeMouse(32, 0, graphY, false)
	if u.graphFocus || u.listSelected != "dir/a" {
		t.Fatal("captured cross-pane motion changed focus or list selection")
	}
	u.routeMouse(0, 0, graphY, true)
	u.input([]byte("\x1b[C"))
	if u.graphFocus || u.selected != "dir/a" || u.listSelected != "dir/a" {
		t.Fatalf("Right after preview drag cycled graph: focus=%v selected=%q list=%q", u.graphFocus, u.selected, u.listSelected)
	}
	u.input([]byte("\x1b[D"))
	if u.listSelected != "dir" {
		t.Fatalf("Left did not navigate to parent: %q", u.listSelected)
	}
	u.input([]byte("\x1b[C"))
	if u.listSelected != "dir/a" || u.selected != "dir/a" {
		t.Fatalf("Right did not navigate to child: %q/%q", u.listSelected, u.selected)
	}
}

func TestPreviewSelectionCancelledOnViewSwitch(t *testing.T) {
	u := selectionUI()
	x := u.listWidth(140) + 2
	u.routeMouse(0, x, 4, false)
	u.input([]byte("\t"))
	u.routeMouse(0, x+5, 4, true)
	if u.selection != nil || u.pendingControl != "" {
		t.Fatal("view switch retained preview capture")
	}
}

func TestPreviewDragCopiesOnlyPreview(t *testing.T) {
	u := selectionUI()
	x := u.listWidth(140) + 2
	u.routeMouse(0, x+1, 4, false)
	u.routeMouse(32, -100, 5, false)
	u.routeMouse(0, -100, 5, true)
	output := u.draw(140, 20, time.Time{})
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("alpha\n+")) + "\a"
	if !strings.Contains(output, want) {
		t.Fatalf("clipboard missing %q in %q", want, output)
	}
	if u.listSelected != "dir/a" || !u.open["dir"] {
		t.Fatal("drag changed list selection/folder")
	}
	if strings.Contains(u.draw(140, 20, time.Time{}), "\x1b]52;") {
		t.Fatal("clipboard flushed twice")
	}
}
