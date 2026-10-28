package main

import (
	"math"
	"testing"
)

// File IDs deliberately do not encode their directory: the parent is authoritative.
func hierarchyFixture() *graph {
	g := newGraph()
	for i, parent := range []string{"A/C", "A/D", "A/E", "B/X", "B/Y", "A"} {
		for j := 0; j < 5; j++ {
			id := string(rune('a'+i)) + string(rune('0'+j))
			g.upsert(id, parent, false)
			g.nodes[id].radius = .5 + float64(j)/2
		}
	}
	g.upsert("unused/empty", "unused", true)
	return g
}

func TestRecursivePackingContainsChildrenAndSeparatesSiblingFolders(t *testing.T) {
	g := hierarchyFixture()
	g.stepClusters(240, 100, 0)
	envelopes := make(map[string]folderEnvelope)
	for _, e := range g.folderEnvelopes(240, 100) {
		envelopes[e.ID] = e
	}
	for _, id := range []string{"A", "A/C", "A/D", "A/E", "B", "B/X", "B/Y"} {
		if _, ok := envelopes[id]; !ok {
			t.Fatalf("missing ancestor envelope %q", id)
		}
	}
	if _, ok := envelopes["unused"]; ok {
		t.Fatal("context-only directories changed layout")
	}
	for _, siblings := range [][]string{{"A", "B"}, {"A/C", "A/D", "A/E"}, {"B/X", "B/Y"}} {
		for i, id := range siblings {
			a := envelopes[id]
			for _, other := range siblings[:i] {
				b := envelopes[other]
				if math.Hypot(a.CenterX-b.CenterX, 2*(a.CenterY-b.CenterY))+1e-8 < a.Radius+b.Radius {
					t.Fatalf("sibling envelopes overlap: %s %s", id, other)
				}
			}
		}
	}
	for _, pair := range [][2]string{{"A/C", "A"}, {"A/D", "A"}, {"A/E", "A"}, {"B/X", "B"}, {"B/Y", "B"}} {
		c, p := envelopes[pair[0]], envelopes[pair[1]]
		if math.Hypot(c.CenterX-p.CenterX, 2*(c.CenterY-p.CenterY))+c.Radius > p.Radius+1e-8 {
			t.Fatalf("%s escapes %s", c.ID, p.ID)
		}
		if c.Depth != p.Depth+1 {
			t.Fatalf("wrong depth: %+v %+v", c, p)
		}
		// The parent heading occupies a band above every child disk.
		if 2*(c.CenterY-p.CenterY)-c.Radius < -p.Radius+2*p.Scale-1e-8 {
			t.Fatalf("child collides with parent title: %s", c.ID)
		}
	}
	for _, c := range g.clusters(240, 100) {
		e := envelopes[c.ID]
		if math.Hypot(c.CenterX-e.CenterX, 2*(c.CenterY-e.CenterY))+c.Radius > e.Radius+1e-8 {
			t.Fatalf("direct-file zone escapes folder %s", c.ID)
		}
		if math.Abs(c.Width-2*c.Radius) > 1e-8 || math.Abs(c.Height-c.Radius) > 1e-8 {
			t.Fatalf("not a physical circle: %+v", c)
		}
		if c.Scale != e.Scale {
			t.Fatal("nonuniform scale")
		}
		if c.ID == "A" {
			for _, id := range []string{"A/C", "A/D", "A/E"} {
				d := envelopes[id]
				if math.Hypot(c.CenterX-d.CenterX, 2*(c.CenterY-d.CenterY))+1e-8 < c.Radius+d.Radius {
					t.Fatalf("direct files collide with subdirectory %s", id)
				}
			}
		}
	}
}
