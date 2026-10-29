package app

import (
	"testing"
	"time"
)

func TestSmallGraphUsesViewportAfterChange(t *testing.T) {
	g := newGraph()
	at := time.Unix(100, 0)
	applySnapshot(g, nil, []changeEvent{{Path: "active/nested/one", Kind: "modified", Added: 3, StatsKnown: true, At: at}, {Path: "active/nested/two", Kind: "modified", Added: 2, StatsKnown: true, At: at}}, at)
	g.upsert("rootfile", ".", false)
	g.nodes["rootfile"].radius = 1
	for _, n := range g.visibleFiles() {
		if n.kind != "" {
			n.radius = 3
		}
	}
	for _, size := range [][2]int{{80, 18}, {160, 24}, {80, 40}} {
		e := g.folderEnvelopes(size[0], size[1])[0]
		ratio := e.Width * e.Height / float64((size[0]-1)*(size[1]-1))
		t.Logf("small graph size=%v overall envelope fraction=%.3f", size, ratio)
		if ratio < .70 {
			t.Errorf("small graph wastes viewport: %.3f", ratio)
		}
	}
}
