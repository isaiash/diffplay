package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCompactListHasNoColumnHeaderRow(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("only.txt", "modified", false, "", 8, 4, true, time.Now())
	frame := plainList(u.renderList(100, 24, time.Now()))
	for _, label := range []string{"ADDED", "REMOVED", "CHANGED PATHS"} {
		if strings.Contains(frame, label) {
			t.Fatalf("unexpected section heading %s", label)
		}
	}
	rows := strings.Split(frame, "\r\n")
	if !strings.Contains(rows[1], "only.txt") {
		t.Fatal("first file must occupy reclaimed header row")
	}
	row := []rune(rows[1])
	center := -1
	for i, r := range row {
		if r == '│' {
			center = i
			break
		}
	}
	if center < 0 {
		t.Fatal("missing histogram")
	}
	left, right := 0, 0
	for i := center - 1; i >= 0 && row[i] != ' '; i-- {
		left++
	}
	for i := center + 1; i < len(row) && row[i] != ' '; i++ {
		right++
	}
	if left > 4 || right > 4 || left <= right {
		t.Fatalf("histogram should be compact, with additions on the left: %d/%d", left, right)
	}
}

func TestDefaultCLIViewIsCombined(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--help"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `(default "combined")`) {
		t.Fatal("default view must be combined")
	}
}

func TestGraphIncludesScannedContextAlongsideChanges(t *testing.T) {
	g := newGraph()
	now := time.Now()
	s := mapEntries(".", "src", "src/changed.go", "src/untouched.go", "docs", "docs/untouched.md")
	applySnapshot(g, s, nil, now)
	if len(g.nodes) != 5 {
		t.Fatal("clean scanned files and folders must populate graph, excluding root")
	}
	applySnapshot(g, s, []changeEvent{{Path: "src/changed.go", Kind: "modified", At: now}}, now)
	if len(g.nodes) != 5 || g.nodes["src/changed.go"].kind != "modified" {
		t.Fatalf("expected changed file alongside quiet context, got %v", g.nodes)
	}
	applySnapshot(g, s, nil, now.Add(time.Second))
	if len(g.nodes) != 5 {
		t.Fatal("unchanged polls must preserve quiet context")
	}
	applySnapshot(g, mapEntries("."), []changeEvent{{Path: "src/changed.go", Kind: "deleted", At: now}}, now)
	g.expire(now.Add(13 * time.Second))
	if g.nodes["src/changed.go"] != nil || g.nodes["src/untouched.go"] == nil {
		t.Fatal("expire only real tombstones, never infer deletions from missing snapshot entries")
	}
}
