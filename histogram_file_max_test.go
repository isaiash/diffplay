package main

import (
	"testing"
	"time"
)

func TestHistogramScaleUsesLargestFileNotFolderSum(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.recordChange("folder/a", "modified", false, "", 60, 0, true, time.Time{})
	u.recordChange("folder/b", "modified", false, "", 40, 0, true, time.Time{})
	u.recordChange("outside", "modified", false, "", 50, 0, true, time.Time{})
	u.listSelected = "outside"
	before := histogramCells(t, u.renderListCanvas(100, 10, time.Time{}), "outside")
	reference := newUI(newGraph(), "demo")
	reference.recordChange("a", "modified", false, "", 60, 0, true, time.Time{})
	reference.recordChange("outside", "modified", false, "", 50, 0, true, time.Time{})
	reference.listSelected = "outside"
	want := histogramCells(t, reference.renderListCanvas(100, 10, time.Time{}), "outside")
	for i := range before {
		if before[i] != want[i] {
			t.Fatalf("folder sum changed file-relative scale at %d: got %+v want %+v", i, before, want)
		}
	}
	u.open["folder"] = false
	after := histogramCells(t, u.renderListCanvas(100, 10, time.Time{}), "outside")
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("folder collapse changed file-relative scale")
		}
	}
}
