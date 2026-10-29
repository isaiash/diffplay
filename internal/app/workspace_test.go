package app

import (
	"strings"
	"testing"
	"time"
)

func TestInitialChangesDoNotFlashButLiveChangesDo(t *testing.T) {
	now := time.Now()
	u := newUI(newGraph(), "/repo")
	events := []changeEvent{{Path: "a.txt", Kind: "modified", Added: 5, StatsKnown: true, At: now}, {Path: "gone.txt", Kind: "deleted", Removed: 2, StatsKnown: true, At: now}}
	u.seedEvents(events)
	frame := u.renderList(100, 24, now)
	if strings.Contains(frame, "✦") || u.history.tree().added != 5 {
		t.Fatal("Git startup counts must be visible without flashes")
	}
	u.graph.expire(now.Add(time.Second))
	if u.graph.nodes["gone.txt"] == nil {
		t.Fatal("initial deleted marker should survive the initial render")
	}
	u.recordEvents([]changeEvent{{Path: "a.txt", Kind: "modified", Added: 1, StatsKnown: true, At: now}})
	if !strings.Contains(u.renderList(100, 24, now), "✦") {
		t.Fatal("a subsequent live event must flash")
	}
}

func TestRepositoryHistogramsHaveIndependentScale(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	u.repositoryLabels = map[string]string{"repo-1": "large", "repo-2": "small"}
	now := time.Now().Add(-time.Minute)
	u.recordChange("repo-1/large.txt", "modified", false, "", 1000, 0, true, now)
	u.recordChange("repo-2/small.txt", "modified", false, "", 10, 0, true, now)
	frame := plainList(u.renderList(100, 24, time.Now()))
	var large, small string
	for _, row := range strings.Split(frame, "\r\n") {
		if strings.Contains(row, "large.txt") {
			large = row
		}
		if strings.Contains(row, "small.txt") {
			small = row
		}
	}
	if large == "" || small == "" || strings.Count(large, "█") != 4 || strings.Count(small, "█") != 4 {
		t.Fatalf("each repository must fill its own scale: %q / %q", large, small)
	}
	if !strings.Contains(frame, "large/") || !strings.Contains(frame, "small/") {
		t.Fatal("repository groups must use readable labels")
	}
}

func TestRepositoryUpdateNamespacingIsCollisionFree(t *testing.T) {
	up := scanUpdate{events: []changeEvent{{Path: "same.txt", Kind: "modified"}}, visible: map[string]bool{"same.txt": false}}
	a := namespaceUpdate("repo-1", up)
	b := namespaceUpdate("repo-2", up)
	if a.events[0].Path != "repo-1/same.txt" || b.events[0].Path != "repo-2/same.txt" {
		t.Fatal("same relative paths must not collide between repositories")
	}
	if allowed, ok := a.visible["repo-1/same.txt"]; !ok || allowed {
		t.Fatal("ignore visibility must be namespaced too")
	}
	if up.events[0].Path != "same.txt" {
		t.Fatal("namespacing must not mutate original updates")
	}
}

func TestRepositoryPickerSingleAndMultiple(t *testing.T) {
	p := newRepositoryPicker([]string{"/workspace/a", "/workspace/b"})
	p.input([]byte("\x1b[B\r"))
	if !p.done || strings.Join(p.selectedPaths(), ",") != "/workspace/b" {
		t.Fatal("Enter must choose the focused repository")
	}
	p = newRepositoryPicker([]string{"/workspace/a", "/workspace/b"})
	p.input([]byte(" \x1b[B \r"))
	if !p.done || len(p.selectedPaths()) != 2 {
		t.Fatal("Space must allow multiple repositories")
	}
	p = newRepositoryPicker([]string{"/workspace/a"})
	p.input([]byte("q"))
	if !p.cancelled {
		t.Fatal("q must cancel")
	}
}
