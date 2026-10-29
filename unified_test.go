package main

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func combinedFixture() *ui {
	u := newUI(newGraph(), "/repo")
	u.combined = true
	at := time.Unix(100, 0)
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("src/file%02d", i)
		u.graph.upsert("src", ".", true)
		u.graph.upsert(id, "src", false)
		u.recordChange(id, "modified", false, "+hello\n-world", 20-i, 1, true, at)
	}
	return u
}
func TestCombinedTabRestoresFullPaneInputDimensions(t *testing.T) {
	u := combinedFixture()
	u.render(80, 31, time.Unix(200, 0))
	u.input([]byte("\t"))
	u.input([]byte("\x1b[<35;1;20M"))
	if u.width != 80 || u.height != 31 {
		t.Fatalf("list input retained split dimensions: %dx%d", u.width, u.height)
	}
}

func TestCombinedTranslatedWheelAndCapturedDrag(t *testing.T) {
	u := combinedFixture()
	u.render(100, 41, time.Unix(200, 0))
	// Pointer is (23,11) in the lower pane; its world point must not move.
	anchor := &node{x: 23, y: 11}
	u.input([]byte("\x1b[<64;24;32M"))
	x, y := u.screen(anchor)
	if math.Abs(x-23) > 1e-9 || math.Abs(y-11) > 1e-9 {
		t.Fatalf("wheel used terminal coordinates: %g,%g", x, y)
	}
	beforeX, beforeY := u.panX, u.panY
	u.input([]byte("\x1b[<0;6;26M"))
	u.input([]byte("\x1b[<32;13;10M"))
	if u.panX != beforeX+7 || u.panY != beforeY-16 {
		t.Fatalf("drag lost capture crossing divider: %g,%g", u.panX, u.panY)
	}
	u.input([]byte("\x1b[<0;13;10m"))
	if u.dragging {
		t.Fatal("release above divider must stop graph drag")
	}
	x, y = u.panX, u.panY
	u.input([]byte("\x1b[<35;14;9M"))
	if u.panX != x || u.panY != y || u.graphFocus {
		t.Fatal("top hover after release must focus list, not pan")
	}
}

func TestCombinedFragmentedGraphHoverAndTopDiff(t *testing.T) {
	u := combinedFixture()
	u.render(80, 40, time.Unix(200, 0))
	n := u.graph.nodes["src/file00"]
	x, y := u.screen(n)
	report := fmt.Sprintf("\x1b[<35;%d;%dM", int(math.Round(x))+1, int(math.Round(y))+21)
	for _, b := range []byte(report) {
		u.input([]byte{b})
	}
	if u.selected != n.id || u.listSelected != n.id {
		t.Fatalf("fragmented translated hover: %q/%q", u.selected, u.listSelected)
	}
	frame := plainList(u.render(80, 40, time.Unix(200, 0)))
	lines := strings.Split(frame, "\r\n")
	if !strings.Contains(strings.Join(lines[:20], "\n"), "+hello") {
		t.Fatal("80-column combined diff pane does not show selected diff")
	}
	// Focus returns to the upper list on hover, then j drives shared selection.
	u.input([]byte("\x1b[<35;1;3M"))
	u.input([]byte("j"))
	if u.graphFocus || u.selected != u.listSelected {
		t.Fatal("list focus/selection not restored")
	}
}

func TestCombinedInitialListSelectionIsShared(t *testing.T) {
	u := combinedFixture()
	u.render(80, 30, time.Unix(200, 0))
	if u.selected != u.listSelected {
		t.Fatalf("initial selection diverged: %q/%q", u.selected, u.listSelected)
	}
}

func TestCombinedRenderHalfPanesAndResize(t *testing.T) {
	u := combinedFixture()
	for _, size := range [][2]int{{80, 31}, {120, 40}, {1, 1}, {2, 2}, {80, 20}} {
		w, h := size[0], size[1]
		frame := plainList(u.render(w, h, time.Unix(200, 0)))
		lines := strings.Split(frame, "\r\n")
		if len(lines) != h {
			t.Fatalf("%v: rows=%d", size, len(lines))
		}
		if w >= 80 {
			if !strings.Contains(lines[0], "% new lines") || strings.Count(lines[u.splitTop(h)], "─") != w {
				t.Fatalf("missing session totals or divider: %v", size)
			}
			if u.listWidth(w) >= w {
				t.Fatal("combined diff pane missing at usable width")
			}
		}
	}
}
func TestCombinedTabCycle(t *testing.T) {
	u := combinedFixture()
	for i, want := range []string{"combined", "list", "graph", "combined"} {
		got := "graph"
		if u.combined {
			got = "combined"
		} else if u.listMode {
			got = "list"
		}
		if got != want {
			t.Fatalf("step %d: %s want %s", i, got, want)
		}
		u.input([]byte("\t"))
	}
}
func TestCombinedListKeyboardSynchronizesGraph(t *testing.T) {
	u := combinedFixture()
	u.render(80, 30, time.Unix(200, 0))
	u.input([]byte("j"))
	if u.selected != u.listSelected || u.selected == "" {
		t.Fatalf("selection not shared: %q/%q", u.selected, u.listSelected)
	}
	u.input([]byte("J"))
	if u.scroll != 1 {
		t.Fatal("J must scroll diff")
	}
	u.input([]byte("n"))
	if u.selected != u.listSelected {
		t.Fatal("n graph cycle must synchronize list")
	}
}
func TestCombinedGraphMouseExpandsAndScrolls(t *testing.T) {
	u := combinedFixture()
	u.open["src"] = false
	u.render(100, 40, time.Unix(200, 0))
	n := u.graph.nodes["src/file19"]
	x, y := u.screen(n)
	u.input([]byte(fmt.Sprintf("\x1b[<0;%d;%dM", int(math.Round(x))+1, int(math.Round(y))+21)))
	if u.selected != n.id || u.listSelected != n.id || !u.open["src"] {
		t.Fatalf("graph selection not revealed: %q/%q", u.selected, u.listSelected)
	}
	if u.listOffset == 0 {
		t.Fatal("selection must scroll into view immediately")
	}
}

func TestCombinedContextSelectionIsTemporaryPresentationOnly(t *testing.T) {
	u := combinedFixture()
	u.graph.upsert("quiet", ".", true)
	u.graph.upsert("quiet/deep", "quiet", true)
	u.graph.upsert("quiet/deep/context", "quiet/deep", false)
	u.render(100, 40, time.Unix(200, 0))
	before := u.history.tree().added
	u.selected = "quiet/deep/context"
	u.syncGraphSelection()
	found := false
	for _, r := range u.changeRows() {
		if r.id == u.selected {
			found = true
			if r.added != 0 || r.removed != 0 || r.unknown {
				t.Fatal("context must have zero counts")
			}
		}
	}
	if !found || u.listSelected != u.selected {
		t.Fatal("graph context selection missing from list")
	}
	if len(u.history.entries) != 20 || u.history.tree().added != before {
		t.Fatal("context polluted history/totals")
	}
	u.selected = "src/file00"
	u.syncGraphSelection()
	for _, r := range u.changeRows() {
		if strings.HasPrefix(r.id, "quiet") {
			t.Fatal("stale context row or ancestor survived selection")
		}
	}
}

func TestChromeStartsWithSessionTotalsAndOneFooter(t *testing.T) {
	u := combinedFixture()
	frame := strings.Split(plainList(u.render(120, 40, time.Unix(200, 0))), "\r\n")
	if !strings.HasPrefix(frame[0], "repo ") || !strings.HasSuffix(frame[0], "total lines") {
		t.Fatalf("first row: %q", frame[0])
	}
	for i, line := range frame {
		if strings.Contains(line, "Diffplay  CHANGES") || strings.Contains(line, "Diffplay  HEX") || strings.Contains(line, "session activity") {
			t.Fatalf("obsolete chrome row %d: %q", i, line)
		}
		if i != len(frame)-1 && (strings.Contains(line, "Tab next view") || strings.Contains(line, "Tab graph/list")) {
			t.Fatalf("embedded footer row %d: %q", i, line)
		}
	}
	if !strings.Contains(frame[len(frame)-1], "Tab") {
		t.Fatal("missing bottom help")
	}
}

func TestCombinedDividerDragCaptureClampAndResize(t *testing.T) {
	u := combinedFixture()
	now := time.Unix(200, 0)
	frame := strings.Split(plainList(u.render(100, 40, now)), "\r\n")
	if strings.Count(frame[19], "─") < 90 {
		t.Fatalf("missing divider at row 20: %q", frame[19])
	}
	startX, startY := u.panX, u.panY
	u.input([]byte("\x1b[<0;3;20M\x1b[<32;3;11M"))
	frame = strings.Split(plainList(u.render(100, 40, now)), "\r\n")
	if strings.Count(frame[10], "─") < 90 || u.panX != startX || u.panY != startY {
		t.Fatal("divider drag up failed or panned graph")
	}
	u.input([]byte("\x1b[<32;3;38M"))
	frame = strings.Split(plainList(u.render(100, 40, now)), "\r\n")
	if strings.Count(frame[37], "─") >= 90 {
		t.Fatal("divider was not clamped")
	}
	u.input([]byte("\x1b[<32;3;16M\x1b[<0;3;16m"))
	frame = strings.Split(plainList(u.render(100, 80, now)), "\r\n")
	if strings.Count(frame[30], "─") < 90 {
		t.Fatal("resize lost divider proportion")
	}
	u.input([]byte("\x1b[<35;3;5M"))
	frame = strings.Split(plainList(u.render(100, 80, now)), "\r\n")
	if strings.Count(frame[30], "─") < 90 {
		t.Fatal("release failed to end divider capture")
	}
}

func TestGraphUsesFirstRowAndFullFooterWithoutHeader(t *testing.T) {
	u := combinedFixture()
	u.combined = false
	now := time.Unix(200, 0)
	u.render(100, 30, now)
	n := u.graph.nodes["src/file00"]
	_, y := u.screen(n)
	u.panY -= y
	c := u.renderGraphCanvas(100, 30, now)
	seen := false
	for _, cell := range c.cells[:100] {
		if cell.dots != 0 {
			seen = true
		}
	}
	if !seen {
		t.Fatal("graph content clipped from first row")
	}
	lines := strings.Split(plainList(c.ansi()), "\r\n")
	if !strings.Contains(lines[29], "Tab") || strings.Contains(strings.Join(lines[:29], "\n"), "Diffplay") {
		t.Fatal("graph chrome remains")
	}
}

func TestDividerClampAndPointerRoutingAfterMove(t *testing.T) {
	u := combinedFixture()
	now := time.Unix(200, 0)
	u.render(100, 40, now)
	u.input([]byte("\x1b[<0;1;20M\x1b[<32;1;1M"))
	if got := u.splitTop(40); got != 3 {
		t.Fatalf("upper clamp = %d", got)
	}
	u.input([]byte("\x1b[<32;1;40M"))
	if got := u.splitTop(40); got != 35 {
		t.Fatalf("lower clamp = %d", got)
	}
	u.input([]byte("\x1b[<32;1;13M\x1b[<0;1;13m"))
	u.render(100, 40, now)
	u.input([]byte("\x1b[<35;5;3M"))
	if u.listSelected != "src/file00" {
		t.Fatalf("list hover after divider move: %q", u.listSelected)
	}
	// Wheel at a pane-local point must leave the world point beneath it fixed.
	point := &node{x: 23, y: 9}
	x0, y0 := u.screen(point)
	// Put focus in the graph first so screen dimensions match its viewport.
	u.input([]byte("\x1b[<35;1;20M"))
	x0, y0 = u.screen(point)
	u.input([]byte(fmt.Sprintf("\x1b[<64;%d;%dM", int(x0)+1, int(y0)+14)))
	x1, y1 := u.screen(point)
	if math.Abs(x1-x0) > 1e-9 || math.Abs(y1-y0) > 1e-9 {
		t.Fatalf("translated wheel moved point: (%g,%g) to (%g,%g)", x0, y0, x1, y1)
	}
}

func TestFullListFirstRowsAndFooter(t *testing.T) {
	u := combinedFixture()
	u.combined = false
	u.listMode = true
	frame := strings.Split(plainList(u.render(100, 20, time.Unix(200, 0))), "\r\n")
	if !strings.HasPrefix(frame[0], "repo ") || !strings.HasSuffix(frame[0], "total lines") {
		t.Fatalf("first row: %q", frame[0])
	}
	if !strings.Contains(frame[1], "src/") {
		t.Fatalf("first change missing: %q", frame[1])
	}
	if !strings.Contains(frame[19], "Tab") {
		t.Fatal("missing full-list footer")
	}
}

func TestFullGraphFooterIncludesCompactState(t *testing.T) {
	u := combinedFixture()
	u.combined = false
	u.paused = true
	frame := strings.Split(plainList(u.render(120, 30, time.Unix(200, 0))), "\r\n")
	if !strings.Contains(frame[29], "PAUSED") || !strings.Contains(frame[29], "Tab") || !strings.Contains(frame[29], "q quit") {
		t.Fatalf("graph footer: %q", frame[29])
	}
}

func TestGraphPreviewHasNoEmbeddedHelp(t *testing.T) {
	u := combinedFixture()
	u.combined = false
	u.selected = "src/file00"
	frame := strings.Split(plainList(u.render(120, 30, time.Unix(200, 0))), "\r\n")
	for i, line := range frame[:len(frame)-1] {
		if strings.Contains(line, "j/k scroll") {
			t.Fatalf("embedded help row %d", i)
		}
	}
	if !strings.Contains(frame[len(frame)-1], "j/k") {
		t.Fatal("footer lacks diff help")
	}
}

func TestCommonWidthFootersExposeQuitAndCoreControls(t *testing.T) {
	for _, mode := range []string{"combined", "list", "graph"} {
		u := combinedFixture()
		u.combined = mode == "combined"
		u.listMode = mode == "list"
		lines := strings.Split(plainList(u.render(80, 24, time.Unix(200, 0))), "\r\n")
		footer := lines[len(lines)-1]
		labels := []string{"Tab", "q quit", "wheel", "J/K"}
		if mode == "graph" {
			labels[3] = "j/k"
		}
		for _, label := range labels {
			if !strings.Contains(footer, label) {
				t.Fatalf("%s footer missing %s: %q", mode, label, footer)
			}
		}
	}
}
