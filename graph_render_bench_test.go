package main

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkGraphRenderThousandFiles(b *testing.B) {
	u := newUI(newGraph(), "demo")
	u.combined = true
	at := time.Unix(100, 0)
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("src-%d/file-%04d", i%4, i)
		events := []changeEvent{{Path: id, Kind: "modified", Added: 3, StatsKnown: true, At: at}}
		applySnapshot(u.graph, nil, events, at)
		u.recordEvents(events)
	}
	u.width, u.height = 120, 30
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.renderGraphCanvas(120, 30, at.Add(time.Hour))
	}
}
