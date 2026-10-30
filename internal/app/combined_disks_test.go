package app

import (
	"math"
	"math/bits"
	"testing"
	"time"
)

func TestCombinedVisibilityRoundTrip(t *testing.T) {
	u := newUI(newGraph(), "repo")
	for _, id := range []string{"changed", "clean"} {
		u.graph.upsert(id, ".", false)
	}
	u.graph.nodes["changed"].kind = "modified"
	at := time.Unix(100, 0)
	for _, combined := range []bool{false, true, false, false, true} {
		if u.combined != combined {
			t.Fatal("Tab did not select the expected view")
		}
		u.renderGraphCanvas(100, 40, at)
		want := 2
		if combined {
			want = 1
		}
		if len(u.fileNodes()) != want || len(u.graphRadii) != want {
			t.Fatalf("combined=%v: files=%d packed=%d, want %d", combined, len(u.fileNodes()), len(u.graphRadii), want)
		}
		if combined {
			clean := u.graph.nodes["clean"]
			clean.x, clean.y = 0, 0
			if u.hit(0, 0) == "clean" {
				t.Fatal("hidden file remains hittable")
			}
			u.selected = "changed"
			u.cycleGraph(1)
			if u.selected != "changed" {
				t.Fatal("navigation reached unchanged context")
			}
		}
		u.input([]byte("	"))
	}
	if u.graph.nodes["clean"] == nil {
		t.Fatal("context destroyed")
	}
}

func TestCombinedDiskAreaUsesEditVolume(t *testing.T) {
	u := newUI(newGraph(), "repo")
	for id, edits := range map[string]int{"small": 100, "large": 400} {
		u.graph.upsert(id, ".", false)
		n := u.graph.nodes[id]
		n.kind, n.lines, n.linesKnown = "modified", 9999, true
		n.added, n.removed = edits/2, edits/2
	}
	for _, combined := range []bool{false, true, false, true} {
		u.combined = combined
		u.updateGraphMetrics()
		a, b := u.graph.nodes["small"].radius, u.graph.nodes["large"].radius
		want := 1.0
		if combined {
			want = 4
		}
		if math.Abs(b*b/(a*a)-want) > 1e-9 {
			t.Fatalf("combined=%v area ratio %g, want %g", combined, b*b/(a*a), want)
		}
		if combined {
			u.renderGraphCanvas(200, 100, time.Unix(100, 0))
			counts := map[string]int{}
			for _, n := range u.fileNodes() {
				c := newCanvas(200, 104)
				u.graphRendering = true
				u.drawFileDisk(c, n, u.displayRadius(n))
				u.graphRendering = false
				for _, v := range c.cells {
					counts[n.id] += bits.OnesCount8(v.dots)
				}
			}
			if ratio := float64(counts["large"]) / float64(counts["small"]); math.Abs(ratio-4) > .15 {
				t.Fatalf("painted area ratio = %g, want about 4", ratio)
			}
		}
	}
}

func TestCombinedMarkersAndRepositoryNormalization(t *testing.T) {
	u := newUI(newGraph(), "repo")
	u.combined = true
	u.repositoryLabels = map[string]string{"one": "one", "two": "two"}
	for _, tc := range []struct {
		id             string
		kind           string
		added, removed int
		unknown        bool
	}{
		{"one/large", "modified", 300, 100, false},
		{"one/empty", "modified", 0, 0, false},
		{"one/unknown", "modified", 9999, 0, true},
		{"one/deleted", "deleted", 0, 100, false},
		{"two/large", "modified", 75, 25, false},
	} {
		u.graph.upsert(tc.id, tc.id[:3], false)
		n := u.graph.nodes[tc.id]
		n.kind, n.added, n.removed, n.unknown = tc.kind, tc.added, tc.removed, tc.unknown
	}
	u.updateGraphMetrics()
	for _, id := range []string{"one/empty", "one/unknown"} {
		if u.graph.nodes[id].radius != .5 {
			t.Fatalf("%s invented an edit volume", id)
		}
	}
	if u.graph.nodes["one/deleted"].radius != 1.5 {
		t.Fatal("deleted disk must use removed edit volume")
	}
	if u.graph.nodes["one/large"].radius != u.graph.nodes["two/large"].radius {
		t.Fatal("normalization leaked across repositories")
	}
	v := newUI(newGraph(), "repo")
	v.combined = true
	v.graph.upsert("gone", ".", false)
	n := v.graph.nodes["gone"]
	n.kind, n.removed = "deleted", 100
	v.updateGraphMetrics()
	if n.radius != 3 {
		t.Fatalf("deleted-only radius = %g, want 3", n.radius)
	}
}

func TestCombinedIgnoresCurrentFileLines(t *testing.T) {
	u := newUI(newGraph(), "repo")
	u.combined = true
	for _, id := range []string{"changed", "other", "hidden"} {
		u.graph.upsert(id, ".", false)
		n := u.graph.nodes[id]
		n.lines, n.linesKnown = 100, true
		if id != "hidden" {
			n.kind, n.added, n.removed = "modified", 30, 10
		}
	}
	at := time.Unix(100, 0)
	baseline := u.renderGraphCanvas(160, 70, at)
	radius := u.graph.nodes["changed"].radius
	paintedRadius := u.graphRadii["changed"]
	for _, id := range []string{"changed", "other", "hidden"} {
		for _, lines := range []int{0, 1, 1000000} {
			for _, known := range []bool{true, false} {
				n := u.graph.nodes[id]
				n.lines, n.linesKnown = lines, known
				actual := u.renderGraphCanvas(160, 70, at)
				if u.graph.nodes["changed"].radius != radius || u.graphRadii["changed"] != paintedRadius {
					t.Fatal("current total lines changed radius")
				}
				for i, v := range actual.cells {
					if v != baseline.cells[i] {
						t.Fatalf("%s lines=%d known=%v changed mask/colors in cell %d", id, lines, known, i)
					}
				}
				n.lines, n.linesKnown = 100, true
			}
		}
	}
	if len(u.graphRadii) != 2 {
		t.Fatal("unchanged context was packed in the combined graph")
	}
}

func TestCombinedPausedModeSwitchReflows(t *testing.T) {
	u := newUI(newGraph(), "repo")
	u.paused = true
	for _, id := range []string{"a-clean", "z-change"} {
		u.graph.upsert(id, ".", false)
	}
	u.graph.nodes["z-change"].kind = "modified"
	u.renderGraphCanvas(100, 41, time.Unix(100, 0))
	u.combined = true
	u.renderGraphCanvas(100, 40, time.Unix(100, 0))
	layout := u.graph.packedHierarchy(100, 40, true)
	n := u.graph.nodes["z-change"]
	if p := layout.positions[n]; n.x != p[0] || n.y != p[1] {
		t.Fatal("paused mode switch retained stale standalone positions")
	}
}
