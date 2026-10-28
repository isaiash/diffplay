package main

import (
	"strings"
	"testing"
	"time"
)

func TestPickerTabTogglesWithoutMovingFocus(t *testing.T) {
	p := newRepositoryPicker([]string{"/a", "/b"})
	p.input([]byte("\t"))
	if !p.checked[0] || p.focus != 0 || p.done {
		t.Fatal("Tab must toggle the focused repository without confirming or moving")
	}
	p.input([]byte("\t"))
	if p.checked[0] {
		t.Fatal("second Tab must deselect")
	}
	p.input([]byte("\t\x1b[B\t\r"))
	if len(p.selectedPaths()) != 2 {
		t.Fatal("Tab must support multiple checked repositories")
	}
}

func TestGraphOnlyFileDisksAndClusterTitles(t *testing.T) {
	g := newGraph()
	g.upsert("src", ".", true)
	g.upsert("src/secret-name.go", "src", false)
	g.nodes["src/secret-name.go"].x = 25
	g.nodes["src/secret-name.go"].y = 10
	u := newUI(g, "/repo")
	now := time.Now()
	u.recordChange("src/secret-name.go", "modified", false, "+hello", 8, 2, true, now.Add(-time.Minute))
	frame := u.render(100, 30, now)
	if strings.Contains(frame, "secret-name.go") {
		t.Fatal("file names must remain hidden without hover")
	}
	if !strings.Contains(frame, "src/") {
		t.Fatal("folder must appear as the cluster title")
	}
	if strings.Contains(frame, "38;2;51;51;61") {
		t.Fatal("graph must not draw relationship lines")
	}
	if strings.Contains(frame, "●") {
		t.Fatal("directory nodes must not be rendered")
	}
	u.input([]byte("l"))
	if strings.Contains(u.render(100, 30, now), "secret-name.go") {
		t.Fatal("legacy label key must not reveal every filename")
	}
	u.input([]byte("\x1b[<35;26;13M"))
	if u.selected != "src/secret-name.go" {
		t.Fatalf("file hover selected %q", u.selected)
	}
	if !strings.Contains(u.render(100, 30, now), "secret-name.go") {
		t.Fatal("hover must reveal the file name and diff")
	}
}

func TestGraphRadiusAndMetricsAreRepositoryLocal(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	u.repositoryLabels = map[string]string{"repo-1": "large", "repo-2": "small"}
	now := time.Now()
	events := []changeEvent{{Path: "repo-1/a.txt", Kind: "modified", Added: 100, Removed: 20, StatsKnown: true, At: now}, {Path: "repo-1/b.txt", Kind: "created", Added: 1, StatsKnown: true, At: now}, {Path: "repo-2/c.txt", Kind: "created", Added: 2, StatsKnown: true, At: now}}
	applySnapshot(u.graph, nil, events, now)
	u.recordEvents(events)
	u.updateGraphMetrics()
	a, b, c := u.graph.nodes["repo-1/a.txt"], u.graph.nodes["repo-1/b.txt"], u.graph.nodes["repo-2/c.txt"]
	if a.radius <= b.radius || a.radius != c.radius {
		t.Fatalf("disk sizing must normalize per repository: %v %v %v", a.radius, b.radius, c.radius)
	}
	if a.added != 100 || a.removed != 20 || !b.fresh {
		t.Fatal("graph metrics must reuse list history")
	}
}

func TestGraphHitUsesDiskExtentAndNeverSelectsFolder(t *testing.T) {
	g := newGraph()
	g.upsert("src", ".", true)
	g.upsert("src/file", "src", false)
	g.nodes["src"].x = 10
	g.nodes["src"].y = 5
	g.nodes["src/file"].x = 30
	g.nodes["src/file"].y = 10
	g.nodes["src/file"].radius = 3
	u := newUI(g, "/repo")
	u.width = 100
	u.height = 30
	if u.hit(10, 7) != "" {
		t.Fatal("folder context must not be selectable as a file")
	}
	if u.hit(32, 12) != "src/file" {
		t.Fatal("hover must include the visible disk extent")
	}
}
