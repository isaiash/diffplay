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
			g.nodes[id].kind = "modified"
		}
	}
	g.upsert("unused/empty", "unused", true)
	return g
}

func TestFolderFramesReserveFullUnicodeTitleWidth(t *testing.T) {
	g := newGraph()
	id := "very-long-parent/日本語日本語日本語日本語"
	g.upsert("file", id, false)
	g.nodes["file"].kind = "modified"
	for _, e := range g.folderEnvelopes(400, 200) {
		if e.ID != id {
			continue
		}
		want := 4
		for _, ch := range folderName(id) + "/" {
			want += runeWidth(ch)
		}
		if e.Width/e.Scale < float64(want)-1e-8 {
			t.Fatalf("native title width=%g, need %d", e.Width/e.Scale, want)
		}
	}
}

func TestFolderTilingUsesViewportAspectAndConsistentGaps(t *testing.T) {
	for _, aspect := range []float64{.2, 5} {
		parts := []packedRect{{w: 10, h: 10}, {w: 10, h: 10}, {w: 10, h: 10}, {w: 10, h: 10}}
		w, h := tileFolders(parts, aspect)
		if aspect < 1 && (w != 10 || h != 46) {
			t.Fatalf("tall viewport not compact: %g x %g", w, h)
		}
		if aspect > 1 && (w != 46 || h != 10) {
			t.Fatalf("wide viewport not compact: %g x %g", w, h)
		}
		for i := 1; i < len(parts); i++ {
			a, b := parts[i-1], parts[i]
			if aspect < 1 && b.y-(a.y+a.h) != 2 || aspect > 1 && b.x-(a.x+a.w) != 2 {
				t.Fatalf("inconsistent sibling gap: %+v %+v", a, b)
			}
		}
	}
}

func TestFolderFramesRemainFiniteInsideTinyViewports(t *testing.T) {
	g := hierarchyFixture()
	for _, size := range [][2]int{{0, -4}, {1, 1}, {2, 1}, {1, 2}, {4, 2}, {20, 120}, {240, 20}} {
		for _, e := range g.folderEnvelopes(size[0], size[1]) {
			for _, v := range []float64{e.Left, e.Top, e.Width, e.Height, e.Scale, e.Radius} {
				if math.IsNaN(v) || math.IsInf(v, 0) || v < -1e-8 {
					t.Fatalf("nonfinite frame at %v: %+v", size, e)
				}
			}
			if e.Left+e.Width > float64(max(1, size[0])-1)+1e-8 || e.Top+e.Height > float64(max(1, size[1])-1)+1e-8 {
				t.Fatalf("frame outside viewport %v: %+v", size, e)
			}
		}
	}
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
				if a.Left < b.Left+b.Width-1e-8 && b.Left < a.Left+a.Width-1e-8 && a.Top < b.Top+b.Height-1e-8 && b.Top < a.Top+a.Height-1e-8 {
					t.Fatalf("sibling envelopes overlap: %s %s", id, other)
				}
			}
		}
	}
	for _, pair := range [][2]string{{"A/C", "A"}, {"A/D", "A"}, {"A/E", "A"}, {"B/X", "B"}, {"B/Y", "B"}} {
		c, p := envelopes[pair[0]], envelopes[pair[1]]
		if c.Left < p.Left+p.Scale-1e-8 || c.Left+c.Width > p.Left+p.Width-p.Scale+1e-8 || c.Top < p.Top+2*p.Scale-1e-8 || c.Top+c.Height > p.Top+p.Height-p.Scale+1e-8 {
			t.Fatalf("%s escapes %s", c.ID, p.ID)
		}
		if c.Depth != p.Depth+1 {
			t.Fatalf("wrong depth: %+v %+v", c, p)
		}
		// Parent title and border occupy two terminal rows before scaling.
		if c.Top < p.Top+2*p.Scale-1e-8 {
			t.Fatalf("child collides with parent title: %s", c.ID)
		}
	}
	for _, c := range g.clusters(240, 100) {
		e := envelopes[c.ID]
		if c.Left < e.Left+e.Scale-1e-8 || c.Left+c.Width > e.Left+e.Width-e.Scale+1e-8 || c.Top < e.Top+2*e.Scale-1e-8 || c.Top+c.Height > e.Top+e.Height-e.Scale+1e-8 {
			t.Fatalf("direct-file zone escapes folder %s", c.ID)
		}
		if math.Abs(c.Radius-math.Hypot(c.Width/2, c.Height)) > 1e-8 {
			t.Fatalf("incorrect physical bounding radius: %+v", c)
		}
		if c.Scale != e.Scale {
			t.Fatal("nonuniform scale")
		}
		if c.ID == "A" {
			for _, id := range []string{"A/C", "A/D", "A/E"} {
				d := envelopes[id]
				if c.Left < d.Left+d.Width-1e-8 && d.Left < c.Left+c.Width-1e-8 && c.Top < d.Top+d.Height-1e-8 && d.Top < c.Top+c.Height-1e-8 {
					t.Fatalf("direct files collide with subdirectory %s", id)
				}
			}
		}
	}
	for _, n := range g.list() {
		if n.dir {
			continue
		}
		e := envelopes[n.parent]
		r := packingRadius(n) * e.Scale
		if n.x-r < e.Left+e.Scale-1e-8 || n.x+r > e.Left+e.Width-e.Scale+1e-8 || n.y-r/2 < e.Top+2*e.Scale-1e-8 || n.y+r/2 > e.Top+e.Height-e.Scale+1e-8 {
			t.Fatalf("file %s escapes folder content", n.id)
		}
	}
}
