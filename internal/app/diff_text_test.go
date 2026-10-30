package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDiffTabsExpandWithoutReplacingUnicodeOrAcceptingEscapes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"+\tif 日本語", "+       if 日本語"},
		{" \t\tx", "                x"},
		{"+日\tx", "+日     x"},
		{"+e\u0301	x", "+e\u0301      x"},
		{"+Windows line\r", "+Windows line"},
	} {
		if got := diffText(tc.input); got != tc.want {
			t.Errorf("diffText(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
	if got := diffText("+\ttext\x1b[2J"); strings.ContainsAny(got, "\x1b\t") {
		t.Fatalf("unsafe diff text: %q", got)
	}
}

func TestEveryDiffPreviewExpandsTabs(t *testing.T) {
	for _, mode := range []string{"list", "combined", "graph"} {
		u := newUI(newGraph(), "/repo")
		u.listMode, u.combined = mode == "list", mode == "combined"
		u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "file", Kind: "modified", Diff: "+	日本語 e\u0301\r", StatsKnown: true}}})
		frame := plainList(u.render(160, 40, time.Now().Add(time.Second)))
		if !strings.Contains(frame, "+       日本語 e\u0301") || strings.Contains(frame, "�") {
			t.Fatalf("%s preview lost tab/Unicode: %q", mode, frame)
		}
	}
}

func TestSelectionNeedsNoCopyModeOrFixedShortcut(t *testing.T) {
	for _, mode := range []string{"list", "combined", "graph"} {
		t.Run(mode, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.listMode, u.combined = mode == "list", mode == "combined"
			now := time.Unix(200, 0)
			before := u.render(180, 40, now)
			u.input([]byte("m"))
			after := u.render(180, 40, now)
			if after != before {
				t.Fatal("m must not change the frame or terminal mouse reporting")
			}
			footer := plainList(after)
			if mode != "graph" && !strings.Contains(footer, "drag diff select") {
				t.Fatal("footer must explain application diff selection")
			}
			if strings.Contains(footer, "terminal copy shortcut") {
				t.Fatal("obsolete copy guidance")
			}
			if strings.Contains(footer, "Ctrl+Shift+C") || strings.Contains(footer, "Cmd+C") || strings.Contains(footer, "m copy") {
				t.Fatal("copy guidance must not hardcode shortcuts or offer a copy mode")
			}
			u.input([]byte("	"))
			if mode == "combined" && !u.listMode || mode == "list" && u.listMode || mode == "graph" && !u.combined {
				t.Fatal("m must not disable subsequent view controls")
			}
		})
	}
}

func TestNativeSelectionIdleDrawingSkipsDuplicatesButRendersUpdates(t *testing.T) {
	for _, mode := range []string{"list", "combined", "graph"} {
		t.Run(mode, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.listMode, u.combined = mode == "list", mode == "combined"
			now := time.Unix(200, 0)
			if u.draw(160, 40, now) == "" {
				t.Fatal("initial frame missing")
			}
			for i := 1; i <= 20; i++ {
				if u.draw(160, 40, now.Add(time.Duration(i)*50*time.Millisecond)) != "" {
					t.Fatal("identical idle redraw would clear native selection")
				}
			}
			u.input([]byte("m"))
			u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "live", Kind: "created", Diff: "+live", StatsKnown: true, At: now}}})
			if frame := u.draw(160, 40, now); !strings.Contains(frame, "live") {
				t.Fatal("native selection must not freeze live updates")
			}
			if u.history.entries["live"] == nil {
				t.Fatal("monitoring stopped")
			}
			if u.draw(180, 50, now) == "" || u.draw(160, 40, now) == "" {
				t.Fatal("resize must invalidate the previous terminal frame")
			}
		})
	}
}

func TestNativeSelectionKeepsPaneDraggingAfterM(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.combined = true
	u.render(160, 40, time.Unix(200, 0))
	u.input([]byte("m"))
	x := u.listWidth(160)
	u.input([]byte(fmt.Sprintf("\x1b[<0;%d;4M\x1b[<32;61;31M\x1b[<0;61;31m", x+1)))
	if u.listWidth(160) != 60 || u.listSplitDragging || u.dragging {
		t.Fatal("native selection guidance must retain pane drag capture and release")
	}
	top := u.splitTop(40)
	u.input([]byte(fmt.Sprintf("\x1b[<0;10;%dM\x1b[<32;10;11M\x1b[<0;10;11m", top+1)))
	if u.splitTop(40) != 10 || u.splitDragging || u.dragging {
		t.Fatal("horizontal pane drag lost")
	}
}

func TestLivePreviewRevealsNewestLineInsideLongHEADHunk(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	u.render(160, 14, time.Now())
	head := "--- a/file\n+++ b/file\n@@ -1,1 +1,42 @@\n baseline\n" + strings.Repeat("+earlier\n", 40) + "+latest\n"
	burst := "--- a/file\n+++ b/file\n@@ -38,4 +38,5 @@\n earlier\n earlier\n earlier\n earlier\n+latest\n"
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "file", Kind: "modified", Diff: burst, Added: 1, StatsKnown: true}}, previews: map[string]string{"file": head}})
	frame := plainList(u.render(160, 14, time.Now()))
	if !strings.Contains(frame, "+latest") || u.history.entries["file"].diff != head {
		t.Fatal("new burst must reveal its line without replacing the HEAD-relative diff")
	}
}
