package main

import (
	"strings"
	"testing"
	"time"
)

func TestClusterDisksReuseListColorsWithoutFadingToGray(t *testing.T) {
	now := time.Now()
	u := newUI(newGraph(), "/repo")
	events := []changeEvent{{Path: "src/existing.go", Kind: "modified", Added: 8, Removed: 4, StatsKnown: true, At: now.Add(-time.Minute)}, {Path: "src/new.go", Kind: "created", Added: 4, StatsKnown: true, At: now.Add(-time.Minute)}}
	applySnapshot(u.graph, nil, events, now)
	u.recordEvents(events)
	u.graph.nodes["src/existing.go"].x = 20
	u.graph.nodes["src/existing.go"].y = 10
	u.graph.nodes["src/new.go"].x = 50
	u.graph.nodes["src/new.go"].y = 10
	frame := u.render(100, 30, now)
	for _, color := range []string{"38;2;115;211;153", "38;2;139;202;255", "38;2;244;92;105"} {
		if !strings.Contains(frame, color) {
			t.Errorf("missing persistent list palette %s", color)
		}
	}
	if strings.Contains(frame, "existing.go") || strings.Contains(frame, "new.go") {
		t.Fatal("disk names must be hidden until inspection")
	}
	u.selected = "src/existing.go"
	if frame = u.render(100, 30, now); !strings.Contains(frame, "existing.go") {
		t.Fatal("keyboard inspection must remain accessible")
	}
}

func TestGraphKeyboardSelectionSkipsFolderContext(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.graph.upsert("src", ".", true)
	u.graph.upsert("src/a", "src", false)
	u.graph.upsert("src/b", "src", false)
	u.input([]byte("n"))
	if u.selected != "src/a" {
		t.Fatal("next must select a file, not the directory")
	}
	u.input([]byte("n"))
	if u.selected != "src/b" {
		t.Fatal("next file not selected")
	}
	u.input([]byte("p"))
	if u.selected != "src/a" {
		t.Fatal("previous file not selected")
	}
}
