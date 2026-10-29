package app

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestRenderChangeColorsAndDiff(t *testing.T) {
	g := newGraph()
	g.upsert(".", "", true)
	g.upsert("new.txt", ".", false)
	g.upsert("old.txt", ".", false)
	now := time.Now()
	g.mark("new.txt", "created", "+hello", now)
	g.mark("old.txt", "deleted", "-bye", now)
	g.nodes["new.txt"].x = 20
	g.nodes["new.txt"].y = 10
	g.nodes["old.txt"].x = 40
	g.nodes["old.txt"].y = 10
	u := newUI(g, "demo")
	u.selected = "new.txt"
	frame := u.render(100, 30, now.Add(time.Second))
	for _, want := range []string{"Tab view", "+hello", blue.sgr(false), red.sgr(false)} {
		if !strings.Contains(frame, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, size := range [][2]int{{1, 1}, {10, 4}, {80, 24}, {200, 60}} {
		if s := u.render(size[0], size[1], now); s == "" {
			t.Fatal("empty resized frame")
		}
	}
}

func TestRecentChangesWinOverlappingCells(t *testing.T) {
	g := newGraph()
	g.upsert("a-new.txt", "", false)
	g.upsert("z-old.txt", "", false)
	now := time.Now()
	g.mark("a-new.txt", "created", "+new", now)
	u := newUI(g, "demo")
	u.recordChange("a-new.txt", "created", true, "+new", 1, 0, true, now)
	u.render(100, 30, now.Add(time.Minute))
	u.paused = true
	for _, n := range g.nodes {
		n.x, n.y = 20, 8
	}
	frame := u.render(100, 30, now.Add(time.Minute))
	// Inspect the overlapping glyph itself, not a header's palette or a burst.
	x, y := u.screen(g.nodes["a-new.txt"])
	line := strings.Split(frame, "\n")[int(y)]
	before := strings.Split(line, blue.sgr(false))
	if len(before) != 2 || !strings.ContainsFunc(before[1], func(r rune) bool { return r >= '\u2801' && r <= '\u28ff' }) {
		t.Fatal("unchanged node hides the new node in the same cell")
	}
	if x != 20 || y != 8 {
		t.Fatal("paused overlap fixture moved")
	}
}

func TestNearbyFileNamesStayHidden(t *testing.T) {
	g := newGraph()
	g.upsert("tracked.txt", "", false)
	g.upsert("new.txt", "", false)
	g.nodes["tracked.txt"].x = 20
	g.nodes["tracked.txt"].y = 8
	g.nodes["new.txt"].x = 24
	g.nodes["new.txt"].y = 8
	now := time.Now()
	g.mark("new.txt", "created", "+new", now)
	u := newUI(g, "demo")
	u.recordChange("new.txt", "created", true, "+new", 1, 0, true, now)
	frame := u.render(100, 30, now)
	if strings.Contains(frame, "tracked.txt") || strings.Contains(frame, "new.txt") {
		t.Fatal("nearby files must not reveal names without hover")
	}
	if !strings.Contains(frame, blue.sgr(false)) {
		t.Fatal("new file color must remain visible")
	}
}

func TestTerminalTextCannotInjectEscapes(t *testing.T) {
	if got := safeText("file\x1b[2J\n\t.txt"); strings.ContainsAny(got, "\x1b\n\t") {
		t.Fatalf("unsafe text: %q", got)
	}
}

func TestMouseHoverAndFragmentedInput(t *testing.T) {
	g := newGraph()
	g.upsert("a", "", false)
	u := newUI(g, "demo")
	u.render(80, 24, time.Now())
	x, y := u.screen(g.nodes["a"])
	px, py := int(math.Round(x))+1, int(math.Round(y))+1
	u.input([]byte(fmt.Sprintf("\x1b[<35;%d;", px)))
	if u.selected != "" {
		t.Fatal("incomplete mouse report must wait")
	}
	u.input([]byte(fmt.Sprintf("%dM", py)))
	if u.selected != "a" {
		t.Fatalf("hover selected %q", u.selected)
	}
	u.input([]byte(fmt.Sprintf("\x1b[<64;%d;%dM", px, py)))
	if u.zoom <= 1 {
		t.Fatal("wheel must zoom")
	}
	u.input([]byte("q"))
	if !u.quit {
		t.Fatal("q must quit")
	}
}
