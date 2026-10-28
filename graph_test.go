package main

import (
	"math"
	"testing"
	"time"
)

func TestGraphGroupsChildrenAndFitsResize(t *testing.T) {
	g := newGraph()
	g.upsert(".", "", true)
	g.upsert("src", ".", true)
	g.upsert("src/main.go", "src", false)
	for i := 0; i < 100; i++ {
		g.step(100, 30, 0.05)
	}
	if len(g.nodes) != 3 {
		t.Fatal("all nodes must exist")
	}
	for _, size := range [][2]int{{100, 30}, {30, 10}, {4, 2}, {180, 50}} {
		for i := 0; i < 30; i++ {
			g.step(size[0], size[1], 0.05)
		}
		for _, n := range g.nodes {
			if math.IsNaN(n.x) || n.x < 0 || n.x > float64(size[0]-1) || n.y < 0 || n.y > float64(size[1]-1) {
				t.Fatalf("node outside resized viewport: %+v", n)
			}
		}
	}
}

func TestDeletedNodesFadeButDiffSurvives(t *testing.T) {
	g := newGraph()
	g.upsert("a", ".", false)
	now := time.Now()
	g.mark("a", "modified", "-old\n+new", now)
	if g.nodes["a"].diff != "-old\n+new" {
		t.Fatal("missing diff")
	}
	g.mark("a", "deleted", "-new", now)
	g.expire(now.Add(3 * time.Second))
	if g.nodes["a"] == nil {
		t.Fatal("deletion should remain visible during burst")
	}
	g.expire(now.Add(13 * time.Second))
	if g.nodes["a"] != nil {
		t.Fatal("deleted tombstone must expire")
	}
}
