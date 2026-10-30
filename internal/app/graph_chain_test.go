package app

import (
	"math"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"
)

func chainGraph(changed, quiet []string) *graph {
	g := newGraph()
	for _, id := range append(append([]string{}, changed...), quiet...) {
		g.upsert(id, path.Dir(id), false)
		g.nodes[id].radius = 1
	}
	for _, id := range changed {
		g.nodes[id].kind = "modified"
	}
	return g
}

func TestCompactFolderChainSelectionIncludesMergedAncestorsAndQuietFiles(t *testing.T) {
	g := chainGraph([]string{"a/b/c/d/file"}, []string{"a/quiet", "a/b/nearby", "a/cousin/file"})
	g.upsert("a/cousin", "a", true)
	g.upsert("a/b/cousin", "a/b", true)
	u := newUI(g, "/repo")
	u.width, u.height = 100, 40
	layout := g.packedHierarchy(100, 40)
	var frame folderEnvelope
	for _, f := range layout.envelopes {
		if f.ID == "a/b/c/d" {
			frame = f
		}
	}
	// Fixed geometry isolates frame color from layout quantization.
	frame.Left, frame.Top, frame.Width, frame.Height = 5, 3, 80, 20
	for _, selected := range []string{"a/b/c/d/file", "a/quiet", "a/b/nearby", "a", "a/b", "a/b/c", "a/b/c/d", "a/cousin", "a/b/cousin", "a/cousin/file", "a/b/cousin/file", "ab/file", ""} {
		u.selected = selected
		c := newCanvas(100, 40)
		u.drawFolderFrame(c, frame)
		want := rgb{r: 128, g: 128, b: 128}
		if selected != "" && !strings.Contains(selected, "cousin") && selected != "ab/file" {
			want = white
		}
		for _, xy := range [][2]int{{5, 5}, {7, 3}} {
			v := c.cells[xy[1]*c.w+xy[0]]
			if v.color != want {
				t.Errorf("selection %q at %v: color=%+v want %+v", selected, xy, v.color, want)
			}
		}
	}
}

func TestCompactFolderChainsKeepBranchPointsAndDirectChanges(t *testing.T) {
	for _, tc := range []struct {
		name          string
		changed, want []string
	}{
		{"branch", []string{"a/b/c/file", "a/b/d/file"}, []string{"a/b", "a/b/c", "a/b/d"}},
		{"direct parent", []string{"a/file", "a/b/c/file"}, []string{"a", "a/b/c"}},
		{"multiple roots", []string{"one/src/deep/file", "two/src/deep/file"}, []string{"one/src/deep", "two/src/deep"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := chainGraph(tc.changed, nil)
			got := []string{}
			for _, f := range g.packedHierarchy(240, 100).envelopes {
				if f.ID != "." {
					got = append(got, f.ID)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("frames=%v want %v", got, tc.want)
			}
		})
	}
}

func TestCompactFolderChainsRetainQuietContextAcrossModes(t *testing.T) {
	changed := []string{"a/b/c/d/file", "a/b/direct"}
	quiet := []string{"rootquiet", "a/quiet", "a/b/quiet", "a/b/c/quiet", "a/b/c/d/quiet"}
	g := chainGraph(changed, append(append([]string{}, quiet...), "a/inactive/file"))
	for _, combined := range []bool{false, true, false} {
		layout := g.packedHierarchy(240, 100, combined)
		want := len(changed)
		if !combined {
			want += len(quiet)
		}
		if len(layout.positions) != want {
			t.Fatalf("combined=%v positions=%d want %d", combined, len(layout.positions), want)
		}
		frames := []string{}
		seen := map[*node]bool{}
		for _, f := range layout.envelopes {
			if f.ID != "." {
				frames = append(frames, f.ID)
			}
		}
		if !reflect.DeepEqual(frames, []string{"a/b", "a/b/c/d"}) {
			t.Fatalf("quiet context altered compaction: %v", frames)
		}
		for _, cluster := range layout.clusters {
			for _, n := range cluster.Nodes {
				if seen[n] {
					t.Fatalf("duplicate node %s", n.id)
				}
				seen[n] = true
				if g.nodes[n.id] != n || n.parent != path.Dir(n.id) {
					t.Fatalf("changed model identity: %+v", n)
				}
			}
		}
		for _, id := range quiet {
			if _, ok := layout.positions[g.nodes[id]]; ok == combined {
				t.Errorf("combined=%v quiet position %s present=%v", combined, id, ok)
			}
		}
	}
	if len(g.nodes) != len(changed)+len(quiet)+1 {
		t.Fatal("compaction discarded scanned context")
	}
}

func TestCompactFolderChainsRepositoryLabelsStayIndependent(t *testing.T) {
	g := chainGraph([]string{"repo-1/src/deep/file", "repo-2/src/deep/file"}, nil)
	u := newUI(g, "/workspace")
	u.width, u.height = 240, 100
	u.repositoryLabels = map[string]string{"repo-1": "first", "repo-2": "other"}
	for _, f := range g.packedHierarchy(240, 100).envelopes {
		if f.ID == "." {
			continue
		}
		c := newCanvas(240, 100)
		u.selected = "repo-1/src/deep/file"
		u.drawFolderFrame(c, f)
		label := "first/src/deep/"
		want := white
		if strings.HasPrefix(f.ID, "repo-2/") {
			label = "other/src/deep/"
			want = rgb{r: 128, g: 128, b: 128}
		}
		if !strings.Contains(plainList(c.ansi()), label) {
			t.Errorf("missing repository label %s", label)
		}
		for _, v := range c.cells {
			if v.ch != 0 && v.color != want {
				t.Errorf("cross-repository selection color for %s", f.ID)
				break
			}
		}
	}
}

func TestCompactFolderChainsUnicodePackingAndContainment(t *testing.T) {
	g := chainGraph([]string{"日本語/long-parent/枝/葉/file", "日本語/long-parent/direct", "other/file"}, []string{"日本語/context"})
	for _, size := range [][2]int{{400, 200}, {240, 20}, {20, 120}, {4, 2}, {1, 1}, {0, -4}} {
		layout := g.packedHierarchy(size[0], size[1])
		frames := map[string]folderEnvelope{}
		for _, f := range layout.envelopes {
			frames[f.ID] = f
			for _, v := range []float64{f.Left, f.Top, f.Width, f.Height, f.Scale, f.Radius} {
				if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
					t.Fatalf("invalid geometry at %v: %+v", size, f)
				}
			}
			width := 4
			for _, r := range f.Label + "/" {
				width += runeWidth(r)
			}
			if f.Scale > 0 && f.Width/f.Scale < float64(width)-1e-8 {
				t.Fatalf("collapsed title not reserved: %+v need %d", f, width)
			}
		}
		outer, inner := frames["日本語/long-parent"], frames["日本語/long-parent/枝/葉"]
		if inner.Depth != outer.Depth+1 || inner.Left < outer.Left+outer.Scale-1e-8 || inner.Left+inner.Width > outer.Left+outer.Width-outer.Scale+1e-8 || inner.Top < outer.Top+2*outer.Scale-1e-8 || inner.Top+inner.Height > outer.Top+outer.Height-outer.Scale+1e-8 {
			t.Fatalf("collapsed child escapes parent at %v", size)
		}
		for _, cluster := range layout.clusters {
			f := frames[cluster.ID]
			for _, n := range cluster.Nodes {
				p := layout.positions[n]
				r := packingRadius(n) * f.Scale
				if p[0]-r < f.Left+f.Scale-1e-8 || p[0]+r > f.Left+f.Width-f.Scale+1e-8 || p[1]-r/2 < f.Top+2*f.Scale-1e-8 || p[1]+r/2 > f.Top+f.Height-f.Scale+1e-8 {
					t.Fatalf("node %s escapes merged frame at %v", n.id, size)
				}
			}
		}
		for a, pa := range layout.positions {
			for b, pb := range layout.positions {
				if a != b && math.Hypot(pa[0]-pb[0], 2*(pa[1]-pb[1]))+1e-8 < (packingRadius(a)+packingRadius(b))*outer.Scale {
					t.Fatalf("overlap %s %s at %v", a.id, b.id, size)
				}
			}
		}
	}
}

func TestCompactFolderChainHitsKeepActualPaths(t *testing.T) {
	g := chainGraph([]string{"a/b/c/d/file", "a/b/direct", "other/file"}, []string{"a/quiet", "a/b/c/quiet"})
	u := newUI(g, "/repo")
	for _, combined := range []bool{false, true, false} {
		u.combined = combined
		u.zoom, u.panX, u.panY = 1, 0, 0
		u.renderGraphCanvas(240, 100, time.Unix(200, 0))
		for _, n := range g.visibleFiles(combined) {
			x, y := u.screen(n)
			if got := u.hit(int(math.Round(x)), int(math.Round(y))); got != n.id {
				t.Fatalf("combined=%v hit %s = %s", combined, n.id, got)
			}
		}
	}
}

func TestCompactFolderChainCameraClipsWithoutInventingBorders(t *testing.T) {
	frame := folderEnvelope{ID: "a/b/c", Start: "a", Label: "a/b/c", Left: 10, Top: 4, Width: 40, Height: 20}
	for _, tc := range []struct {
		name       string
		w, h       int
		zoom, x, y float64
	}{
		{"pan", 100, 50, 1, 3, 2}, {"zoom", 100, 50, 1.5, 20, 12}, {"clip", 100, 50, 1, -11, 0}, {"tiny", 12, 9, 1, -11, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.width, u.height, u.zoom, u.panX, u.panY = tc.w, tc.h, tc.zoom, tc.x, tc.y
			var baseline *canvas
			for _, selected := range []string{"", "a", ""} {
				u.selected = selected
				c := newCanvas(tc.w, tc.h)
				u.drawFolderFrame(c, frame)
				want := rgb{r: 128, g: 128, b: 128}
				if selected != "" {
					want = white
				}
				painted := 0
				left, _ := u.screen(&node{x: frame.Left, y: frame.Top})
				for i, v := range c.cells {
					if baseline != nil && (v.ch != baseline.cells[i].ch || v.dots != baseline.cells[i].dots) {
						t.Fatal("selection changed frame geometry")
					}
					if v.ch == 0 {
						continue
					}
					painted++
					if v.color != want {
						t.Fatalf("selection=%s wrong color: %+v", selected, v)
					}
					if y := i / c.w; y < 2 || y >= c.h-2 {
						t.Fatal("overwrote reserved rows")
					}
					if left < 0 && i%c.w == 0 && v.ch == '│' {
						t.Fatal("invented left border at clipped viewport edge")
					}
				}
				if painted == 0 {
					t.Fatal("camera fixture drew nothing")
				}
				baseline = c
			}
		})
	}
}

func TestCompactFolderChainsExactExample(t *testing.T) {
	g := chainGraph([]string{"a/b/c/d/file.txt", "a/b/file2.txt"}, nil)
	layout := g.packedHierarchy(240, 100)
	ids := []string{}
	for _, f := range layout.envelopes {
		if f.ID != "." {
			ids = append(ids, f.ID)
		}
	}
	if !reflect.DeepEqual(ids, []string{"a/b", "a/b/c/d"}) {
		t.Fatalf("frames=%v, want only a/b and a/b/c/d", ids)
	}
	u := newUI(g, "/repo")
	u.width, u.height = 240, 100
	c := newCanvas(240, 100)
	for _, f := range layout.envelopes {
		if f.ID != "." {
			u.drawFolderFrame(c, f)
		}
	}
	text := plainList(c.ansi())
	for _, label := range []string{"a/b/", "c/d/"} {
		if !strings.Contains(text, label) {
			t.Errorf("missing relative label %q", label)
		}
	}
	if len(layout.positions) != 2 || g.nodes["a/b/c/d/file.txt"].parent != "a/b/c/d" {
		t.Fatal("packing changed actual model paths")
	}
}
