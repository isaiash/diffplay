package main

import "testing"

func TestSelectionRecolorsAllExistingCells(t *testing.T) {
	for _, zoom := range []float64{0.5, 1, 3} {
		for _, tc := range []struct {
			name string
			node node
		}{
			{"sparse", node{id: "sparse", kind: "modified", added: 2}},
			{"dense", node{id: "dense", kind: "modified", added: 800, removed: 100}},
			{"context", node{id: "context"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				u := newUI(newGraph(), "demo")
				u.width, u.height, u.zoom = 100, 40, zoom
				n := tc.node
				// Put the node at the camera center at every zoom.
				n.x, n.y = 50, 19.5
				u.graphRadii = map[string]float64{n.id: 6}
				c := newCanvas(100, 40)
				u.drawFileDisk(c, &n, 6)
				before := append([]cell(nil), c.cells...)
				u.selected = n.id
				c = newCanvas(100, 40)
				u.drawFileDisk(c, &n, 6)
				whitened := 0
				for i, v := range c.cells {
					old := before[i]
					if v.dots != old.dots || v.ch != old.ch || v.bg != old.bg || v.strike != old.strike {
						t.Fatalf("zoom %g changed glyph or background at (%d,%d): %+v -> %+v", zoom, i%c.w, i/c.w, old, v)
					}
					if v.color != old.color {
						if old.dots == 0 || v.color != white {
							t.Fatalf("zoom %g recolored non-file cell (%d,%d)", zoom, i%c.w, i/c.w)
						}
						whitened++
					} else if v.dots != 0 && v.color != white {
						t.Fatalf("zoom %g left selected file cell colored", zoom)
					}
				}
				if whitened == 0 {
					t.Fatalf("zoom %g had no highlighted existing cells", zoom)
				}

			})
		}
	}
}
