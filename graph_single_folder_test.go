package main

import (
	"math"
	"testing"
)

func TestSingleFolderPackingDoesNotAllocate(t *testing.T) {
	parts := []packedRect{{x: 7, y: 9, w: 10, h: 20}}
	for _, aspect := range []float64{0, .2, 2, math.NaN()} {
		allocations := testing.AllocsPerRun(10, func() {
			w, h := tileFolders(parts, aspect)
			if w != 10 || h != 20 || parts[0].x != 0 || parts[0].y != 0 {
				t.Fatalf("single folder placement at aspect %g: %v, %g x %g", aspect, parts, w, h)
			}
		})
		if allocations != 0 {
			t.Fatalf("single folder needs no packing search, got %g allocations", allocations)
		}
	}
}
