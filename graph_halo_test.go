package main

import (
	"testing"
	"time"
)

func TestSelectedFileRecolorsExistingDotsWithoutChangingMasks(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height, u.combined, u.paused = 100, 40, true, true
	events := []changeEvent{{Path: "file", Kind: "modified", Added: 300, Removed: 100, StatsKnown: true, At: time.Unix(100, 0)}}
	applySnapshot(u.graph, nil, events, time.Unix(100, 0))
	u.recordEvents(events)
	before := u.renderGraphCanvas(100, 40, time.Unix(200, 0))
	u.selected = "file"
	after := u.renderGraphCanvas(100, 40, time.Unix(200, 0))
	whitened, retained := 0, 0
	for i, v := range after.cells {
		old := before.cells[i]
		if v.dots != old.dots {
			t.Fatalf("selection changed dot mask at (%d,%d)", i%after.w, i/after.w)
		}
		if v.dots != 0 && v.color == white && old.color != white {
			whitened++
		}
		if v.dots != 0 && v.color != white {
			retained++
		}
	}
	if whitened == 0 || retained != 0 {
		t.Fatalf("want all file dots white, got %d white and %d colored", whitened, retained)
	}
}
