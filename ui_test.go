package main

import (
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
	frame := u.render(100, 30, now)
	for _, want := range []string{"Diffplay", "+hello", "38;2;139;202;255", "38;2;244;92;105"} {
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
	for _, n := range g.nodes {
		n.x = 20
		n.y = 8
	}
	now := time.Now()
	g.mark("a-new.txt", "created", "+new", now)
	u := newUI(g, "demo")
	frame := u.render(100, 30, now)
	before, _, found := strings.Cut(frame, "█")
	lastColor := strings.LastIndex(before, "\x1b[38;2;")
	if !found || lastColor < 0 || !strings.HasPrefix(before[lastColor:], "\x1b[38;2;139;202;255m") {
		t.Fatal("unchanged node hides the new node in the same cell")
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
	frame := u.render(100, 30, now)
	if strings.Contains(frame, "tracked.txt") || strings.Contains(frame, "new.txt") {
		t.Fatal("nearby files must not reveal names without hover")
	}
	if !strings.Contains(frame, "38;2;139;202;255") {
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
	g.nodes["a"].x = 10
	g.nodes["a"].y = 5
	u := newUI(g, "demo")
	u.width = 80
	u.height = 24
	u.input([]byte("\x1b[<35;11;"))
	if u.selected != "" {
		t.Fatal("incomplete mouse report must wait")
	}
	u.input([]byte("8M"))
	if u.selected != "a" {
		t.Fatalf("hover selected %q", u.selected)
	}
	u.input([]byte("\x1b[<64;11;8M"))
	if u.zoom <= 1 {
		t.Fatal("wheel must zoom")
	}
	u.input([]byte("q"))
	if !u.quit {
		t.Fatal("q must quit")
	}
}
