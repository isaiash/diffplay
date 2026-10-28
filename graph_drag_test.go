package main

import (
	"math"
	"testing"
	"time"
)

func TestPackedBubbleDraggingPersistsWithoutCrossingFolderBoundary(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	now := time.Now()
	events := []changeEvent{{Path: "src/file", Kind: "modified", Added: 5, StatsKnown: true, At: now}}
	applySnapshot(u.graph, nil, events, now)
	u.recordEvents(events)
	u.updateGraphMetrics()
	u.graph.step(100, 26, .05)
	u.render(100, 30, now)
	n := u.graph.nodes["src/file"]
	cluster := u.graph.clusters(100, 26)[0]
	x, y := cluster.CenterX+1.5, cluster.CenterY
	if !u.tryGraphDrag(n, x, y) {
		t.Fatal("valid movement within a packed group should work")
	}
	u.graph.step(100, 26, .05)
	u.render(100, 30, now)
	if math.Abs(n.x-x) > 1e-8 || math.Abs(n.y-y) > 1e-8 {
		t.Fatal("packing reset the user's drag")
	}
	if u.tryGraphDrag(n, cluster.CenterX+cluster.Radius*2, cluster.CenterY) {
		t.Fatal("drag must not let a file leave its folder envelope")
	}
}
