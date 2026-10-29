package main

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestPackingRadiusHonorsLargeContextFiles(t *testing.T) {
	n := &node{radius: 6}
	if got := packingRadius(n); got != 6 {
		t.Fatalf("large context radius clipped: got %g, want 6", got)
	}
}

func TestFileClustersUseExactSortedParentsAndExcludeDirectories(t *testing.T) {
	g := newGraph()
	for _, id := range []string{"repo-b/src/z", "repo-a/src/b", "repo-a/src/a"} {
		parent := "repo-a/src"
		if id == "repo-b/src/z" {
			parent = "repo-b/src"
		}
		g.upsert(id, parent, false)
		g.nodes[id].kind = "modified"
	}
	g.upsert("empty", "repo-a", true)
	groups := g.clusters(80, 24)
	if len(groups) != 2 {
		t.Fatalf("got %d clusters, want 2", len(groups))
	}
	if groups[0].ID != "repo-a/src" || groups[1].ID != "repo-b/src" {
		t.Fatalf("parents conflated or unsorted: %+v", groups)
	}
	if got := []string{groups[0].Nodes[0].id, groups[0].Nodes[1].id}; !reflect.DeepEqual(got, []string{"repo-a/src/a", "repo-a/src/b"}) {
		t.Fatalf("unsorted files: %v", got)
	}
	if got := newGraph().clusters(0, 0); len(got) != 0 {
		t.Fatalf("empty graph has groups: %v", got)
	}
	if got := newGraph().folderEnvelopes(0, 0); len(got) != 0 {
		t.Fatalf("empty graph has envelopes: %v", got)
	}
}

func TestClusterPackingNeverOverlapsFileDisksAcrossAspectResizes(t *testing.T) {
	g := newGraph()
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("file-%03d", i)
		g.upsert(id, fmt.Sprintf("repo-%d/src/branch-%d", i%3, i%7), false)
		g.nodes[id].radius = .5 + float64(i%12)/2
		g.nodes[id].kind = "modified"
	}
	for _, size := range [][2]int{{240, 100}, {35, 12}, {24, 120}, {120, 24}, {4, 2}, {1, 1}, {0, -4}, {180, 60}} {
		for _, dt := range []float64{0, .05, math.NaN(), math.Inf(1), -.5} {
			g.stepClusters(size[0], size[1], dt)
			groups := g.clusters(size[0], size[1])
			scales := make(map[*node]float64)
			for _, c := range groups {
				if math.IsNaN(c.Scale) || math.IsInf(c.Scale, 0) || c.Scale < 0 {
					t.Fatalf("invalid scale: %+v", c)
				}
				for _, n := range c.Nodes {
					scales[n] = c.Scale
					if math.IsNaN(n.x) || math.IsNaN(n.y) || math.IsInf(n.x, 0) || math.IsInf(n.y, 0) || n.x < 0 || n.y < 0 || n.x > float64(max(1, size[0])-1) || n.y > float64(max(1, size[1])-1) {
						t.Fatalf("invalid position at %v: %+v", size, n)
					}
					r := (packingRadius(n) + .35) * c.Scale
					if n.x-r < c.Left-1e-8 || n.x+r > c.Left+c.Width+1e-8 || n.y-r/2 < c.Top-1e-8 || n.y+r/2 > c.Top+c.Height+1e-8 {
						t.Fatalf("file escapes rectangular direct-file zone at %v: %s", size, n.id)
					}
				}
			}
			files := g.list()
			for i, a := range files {
				for _, b := range files[:i] {
					if math.Hypot(a.x-b.x, 2*(a.y-b.y))+1e-8 < (a.radius+.35)*scales[a]+(b.radius+.35)*scales[b] {
						t.Fatalf("file collision at %v: %s %s", size, a.id, b.id)
					}
				}
			}
		}
	}
}

func TestClusterStepZeroTimeIsStationaryAndSanitizesInvalidInputs(t *testing.T) {
	g := hierarchyFixture()
	g.stepClusters(120, 40, .05)
	before := make(map[*node][2]float64)
	for _, n := range g.list() {
		before[n] = [2]float64{n.x, n.y}
	}
	g.stepClusters(120, 40, 0)
	for n, p := range before {
		if n.x != p[0] || n.y != p[1] {
			t.Fatalf("dt=0 moved %s", n.id)
		}
	}
	for _, n := range g.list() {
		n.x, n.y, n.radius = math.NaN(), math.Inf(1), math.NaN()
	}
	g.stepClusters(4, 2, 0)
	for _, n := range g.list() {
		if math.IsNaN(n.x) || math.IsInf(n.x, 0) || math.IsNaN(n.y) || math.IsInf(n.y, 0) {
			t.Fatalf("invalid state: %+v", n)
		}
	}
	if g.width != 4 || g.height != 2 {
		t.Fatal("viewport dimensions not updated")
	}
	newGraph().stepClusters(0, 0, 0)
}

func TestClusterPackingIsIndependentOfInsertionOrderAndContext(t *testing.T) {
	a, b := newGraph(), newGraph()
	for i := 0; i < 80; i++ {
		a.upsert(fmt.Sprintf("f-%03d", i), fmt.Sprintf("repo/src-%d", i%4), false)
		a.nodes[fmt.Sprintf("f-%03d", i)].kind = "modified"
		j := 79 - i
		b.upsert(fmt.Sprintf("f-%03d", j), fmt.Sprintf("repo/src-%d", j%4), false)
		b.nodes[fmt.Sprintf("f-%03d", j)].kind = "modified"
	}
	b.upsert("unrelated/empty/directory", "unrelated/empty", true)
	a.stepClusters(120, 40, .05)
	b.stepClusters(120, 40, .05)
	if !reflect.DeepEqual(a.folderEnvelopes(120, 40), b.folderEnvelopes(120, 40)) {
		t.Fatal("insertion order or context changed hierarchy")
	}
	positions := make(map[[2]float64]bool)
	for _, n := range a.list() {
		o := b.nodes[n.id]
		if n.x != o.x || n.y != o.y {
			t.Fatalf("nondeterministic placement: %s", n.id)
		}
		positions[[2]float64{n.x, n.y}] = true
	}
	if len(positions) != 80 {
		t.Fatalf("initial packing piles up files: %d positions", len(positions))
	}
}

func TestClusterPackingRebuildsAfterNewFilesAndParentChanges(t *testing.T) {
	g := newGraph()
	g.upsert("arbitrary-id", "repo-a/src", false)
	g.nodes["arbitrary-id"].kind = "modified"
	g.stepClusters(100, 40, 0)
	g.upsert("new-file", "repo-b/src", false)
	g.nodes["new-file"].kind = "modified"
	g.upsert("arbitrary-id", "repo-a/src/deep", false)
	g.stepClusters(100, 40, 0)
	groups := g.clusters(100, 40)
	if len(groups) != 2 || groups[0].ID != "repo-a/src/deep" || groups[1].ID != "repo-b/src" {
		t.Fatalf("stale hierarchy: %+v", groups)
	}
	a, b := g.nodes["arbitrary-id"], g.nodes["new-file"]
	if a.x == b.x && a.y == b.y {
		t.Fatal("new file was not packed")
	}
}

// Contact-ring packing is O(n log n), regardless of coincident input positions.
func BenchmarkClusterStepDense(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("files-%d", count), func(b *testing.B) {
			g := newGraph()
			for i := 0; i < count; i++ {
				g.upsert(fmt.Sprintf("file-%06d", i), "root", false)
				g.nodes[fmt.Sprintf("file-%06d", i)].kind = "modified"
			}
			g.list()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				g.stepClusters(80, 24, .05)
			}
		})
	}
}
