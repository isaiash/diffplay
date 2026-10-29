package main

import (
	"testing"
	"time"
)

func TestSelectedFileWhitesEntireCircleWithoutChangingGlyphs(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.combined = true
	at := time.Unix(100, 0)
	events := []changeEvent{{Path: "file", Kind: "modified", Added: 300, Removed: 100, StatsKnown: true, At: at}}
	applySnapshot(u.graph, nil, events, at)
	u.recordEvents(events)
	before := u.renderGraphCanvas(100, 40, at.Add(time.Hour))
	u.selected = "file"
	after := u.renderGraphCanvas(100, 40, at.Add(time.Hour))
	total, whiteCount := 0, 0
	for i, a := range after.cells {
		b := before.cells[i]
		if a.ch != b.ch || a.dots != b.dots {
			t.Fatalf("selection changed glyph or dots at %d", i)
		}
		if b.dots != 0 && (b.color == green || b.color == red) {
			total++
			if a.color == white {
				whiteCount++
			}
		}
	}
	if total < 10 || whiteCount != total {
		t.Fatalf("selected file has %d white / %d colored glyphs", whiteCount, total)
	}
}
