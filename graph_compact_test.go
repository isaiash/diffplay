package main

import (
	"fmt"
	"testing"
)

func TestDirectFilesAdaptToWideViewport(t *testing.T) {
	g := newGraph()
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("file-%03d", i)
		g.upsert(id, ".", false)
		g.nodes[id].kind = "modified"
		g.nodes[id].radius = 1
	}
	for _, size := range [][2]int{{80, 20}, {240, 30}} {
		e := g.folderEnvelopes(size[0], size[1])[0]
		utilization := e.Width * e.Height / float64((size[0]-1)*(size[1]-1))
		t.Logf("viewport %v: frame utilization %.3f", size, utilization)
		if utilization < .82 {
			t.Errorf("viewport %v: frame utilization %.3f, want at least .82", size, utilization)
		}
	}
}

func TestMixedHierarchyAdaptsToViewport(t *testing.T) {
	g := newGraph()
	for i := 0; i < 180; i++ {
		id := fmt.Sprintf("file-%03d", i)
		parent := []string{"A", "A/long-child", "A/other", "B/deep/leaf", "C"}[i%5]
		g.upsert(id, parent, false)
		g.nodes[id].kind = "modified"
		g.nodes[id].radius = 1
	}
	for _, size := range [][2]int{{80, 20}, {240, 30}, {30, 100}} {
		e := g.folderEnvelopes(size[0], size[1])[0]
		utilization := e.Width * e.Height / float64((size[0]-1)*(size[1]-1))
		t.Logf("viewport %v: frame utilization %.3f", size, utilization)
		if utilization < .80 {
			t.Errorf("viewport %v: frame utilization %.3f, want at least .80", size, utilization)
		}
	}
}

func TestClusterRepackAfterRemovalAndResizeAtZeroTime(t *testing.T) {
	g := newGraph()
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("file-%02d", i)
		g.upsert(id, ".", false)
		g.nodes[id].kind = "modified"
		g.nodes[id].radius = 1
	}
	g.stepClusters(80, 20, 0)
	before := [2]float64{g.nodes["file-00"].x, g.nodes["file-00"].y}
	for i := 10; i < 30; i++ {
		delete(g.nodes, fmt.Sprintf("file-%02d", i))
	}
	g.dirty = true
	g.stepClusters(80, 20, 0)
	after := [2]float64{g.nodes["file-00"].x, g.nodes["file-00"].y}
	if before == after || len(g.clusters(80, 20)[0].Nodes) != 10 {
		t.Fatal("removal did not immediately repack files")
	}
	g.stepClusters(20, 80, 0)
	resized := [2]float64{g.nodes["file-00"].x, g.nodes["file-00"].y}
	if resized == after {
		t.Fatal("resize did not immediately repack files")
	}
	g.stepClusters(20, 80, 0)
	if resized != [2]float64{g.nodes["file-00"].x, g.nodes["file-00"].y} {
		t.Fatal("unchanged graph drifted after repacking")
	}
}

func TestSingleFileFolderAvoidsCircularEnvelopePadding(t *testing.T) {
	g := newGraph()
	g.upsert("a", "A", false)
	g.nodes["a"].radius = 3
	for _, e := range g.folderEnvelopes(120, 40) {
		if e.ID == "A" && e.Width/e.Scale > 11 {
			t.Fatalf("single-file folder too wide: native width=%g", e.Width/e.Scale)
		}
	}
}
