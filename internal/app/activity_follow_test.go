package app

import (
	"fmt"
	"testing"
	"time"
)

func TestLiveBurstRevealsCollapsedOffscreenFile(t *testing.T) {
	for _, combined := range []bool{false, true} {
		u := newUI(newGraph(), "/repo")
		u.listMode, u.combined = !combined, combined
		for i := 0; i < 30; i++ {
			u.recordChange(fmt.Sprintf("src/file%02d", i), "modified", false, "+old", 30-i, 0, true, time.Time{})
		}
		u.render(120, 16, time.Now())
		u.open["src"] = false
		u.scroll = 99
		u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "src/file29", Kind: "modified", Diff: "+latest", Added: 1, StatsKnown: true, At: time.Now()}}})
		if u.selected != "src/file29" || u.listSelected != u.selected || !u.open["src"] || u.listOffset == 0 || u.scroll != 0 {
			t.Fatalf("not revealed: selected=%q list=%q open=%v offset=%d scroll=%d", u.selected, u.listSelected, u.open, u.listOffset, u.scroll)
		}
	}
}

func TestLiveBurstQueuesEveryFileAndManualInputCancels(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	now := time.Now()
	events := []changeEvent{{Path: "a", Kind: "modified", Diff: "+a", StatsKnown: true, At: now}, {Path: "b", Kind: "modified", Diff: "+b", StatsKnown: true, At: now}, {Path: "c", Kind: "modified", Diff: "+c", StatsKnown: true, At: now}}
	u.applyScanUpdate(scanUpdate{events: events})
	if u.listSelected != "a" {
		t.Fatalf("first file not immediate: %q", u.listSelected)
	}
	u.render(120, 20, now.Add(300*time.Millisecond))
	if u.listSelected != "a" {
		t.Fatal("advanced too soon")
	}
	u.render(120, 20, now.Add(650*time.Millisecond))
	if u.listSelected != "b" {
		t.Fatalf("second file not shown: %q", u.listSelected)
	}
	u.scroll = 9
	u.applyScanUpdate(scanUpdate{events: events[1:2]})
	if u.listSelected != "b" || u.scroll != 0 {
		t.Fatal("same-file burst must reset immediately")
	}
	u.render(120, 20, time.Now().Add(650*time.Millisecond))
	if u.listSelected != "c" {
		t.Fatalf("pending file lost: %q", u.listSelected)
	}
	u.applyScanUpdate(scanUpdate{events: events})
	u.input([]byte("J"))
	selected := u.listSelected
	u.render(120, 20, time.Now().Add(2*time.Second))
	if u.listSelected != selected {
		t.Fatal("manual navigation must cancel follow")
	}
}

func TestQueuedFollowUsesLatestPreviewWithoutDuplicateVisits(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	events := []changeEvent{{Path: "a", Kind: "modified", Diff: "+a", StatsKnown: true}, {Path: "b", Kind: "modified", Diff: "+old", StatsKnown: true}}
	u.applyScanUpdate(scanUpdate{events: events})
	for i := 0; i < 10; i++ {
		u.applyScanUpdate(scanUpdate{events: events, previews: map[string]string{"b": "+latest HEAD"}})
	}
	if len(u.followPending) != 1 || u.followPending[0] != "b" {
		t.Fatalf("queue not deduplicated: %v", u.followPending)
	}
	u.render(120, 20, u.followAt)
	if u.listSelected != "b" || u.history.entries["b"].diff != "+latest HEAD" || len(u.followPending) != 0 {
		t.Fatal("queue did not show latest per-file state")
	}
	u.applyScanUpdate(scanUpdate{events: events})
	u.routeMouse(35, 1, 1, false)
	selected := u.selected
	u.render(120, 20, time.Now().Add(time.Second))
	if u.selected != selected {
		t.Fatal("mouse navigation failed to cancel queue")
	}
}

func TestScanUsesHEADPreviewAndSuppliedModificationCounts(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "a", Kind: "modified", Diff: "+burst", Added: 2, StatsKnown: true}}, previews: map[string]string{"a": "+HEAD"}, modifications: map[string]int{"a": 7}})
	r := u.history.entries["a"]
	if n := u.graph.nodes["a"]; n == nil || n.diff != "+HEAD" {
		t.Fatalf("graph HEAD preview lost: %+v", n)
	}
	if r.diff != "+HEAD" || r.modifications != 7 || r.added != 2 {
		t.Fatalf("HEAD preview/counts lost: %+v", r)
	}
	u.history.tree()
	u.scroll = 7
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "a", Kind: "modified", Diff: "-burst", Removed: 1, StatsKnown: true}}, previews: map[string]string{"a": ""}, modifications: map[string]int{"a": 0}})
	if r.diff != "" || r.modifications != 0 || r.added != 2 || r.removed != 1 || u.history.tree().modifications != 0 || u.scroll != 0 {
		t.Fatalf("clear must retain cumulative line counts: %+v", r)
	}
}

func TestLiveFollowSkipsHiddenAndDirectoryEvents(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "hidden", Kind: "modified"}, {Path: "dir", Kind: "modified", IsDir: true}, {Path: "shown", Kind: "modified"}}, visible: map[string]bool{"hidden": false}})
	if u.selected != "shown" || u.listSelected != "shown" {
		t.Fatalf("focused invisible/directory path: %q/%q", u.selected, u.listSelected)
	}
	if len(u.followPending) != 0 {
		t.Fatal("hidden/directory queued")
	}
}

func TestDividerResizeCaptureHoverAndProportion(t *testing.T) {
	for _, combined := range []bool{false, true} {
		u := newUI(newGraph(), "/repo")
		u.combined, u.listMode = combined, !combined
		u.render(160, 40, time.Now())
		x := u.listWidth(160)
		u.routeMouse(35, x, 3, false)
		c := u.renderListCanvas(160, 20, time.Now())
		if c.cells[3*160+x].ch != '↔' || c.cells[3*160+x].color != white {
			t.Fatal("vertical resize hover hint missing")
		}
		u.routeMouse(0, x, 3, false)
		u.routeMouse(32, 60, 30, false)
		if u.listWidth(160) != 60 || u.dragging {
			t.Fatal("vertical capture lost across panes")
		}
		u.routeMouse(0, 60, 30, true)
		u.render(240, 60, time.Now())
		if u.listWidth(240) != 90 {
			t.Fatalf("split proportion not preserved: %d", u.listWidth(240))
		}
		u.routeMouse(0, 90, 3, false)
		u.routeMouse(32, -20, 3, false)
		if u.listWidth(240) < 20 {
			t.Fatal("list pane unclamped")
		}
		u.routeMouse(32, 1000, 3, false)
		if 240-u.listWidth(240) < 20 {
			t.Fatal("preview pane unclamped")
		}
		u.routeMouse(0, 1000, 3, true)
		for _, w := range []int{1, 12, 39} {
			u.render(w, 6, time.Now())
			if u.listWidth(w) != w {
				t.Fatal("narrow fallback lost")
			}
		}
	}
}

func TestHorizontalDividerHintAndIntersectionPriority(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.combined = true
	u.render(120, 40, time.Now())
	top := u.splitTop(40)
	x := u.listWidth(120)
	u.routeMouse(35, x, top, false)
	c := u.renderCombined(120, 40, time.Now())
	if c.cells[top*120+x].ch != '↕' {
		t.Fatal("horizontal resize hint missing")
	}
	u.routeMouse(0, x, top, false)
	u.routeMouse(32, x, 10, false)
	if u.splitTop(40) != 10 || u.dragging {
		t.Fatal("intersection must drag horizontal, not graph")
	}
	u.routeMouse(0, x, 10, true)
}

func TestSeedEventsNeverFollow(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.selected = "existing"
	u.listSelected = "existing"
	u.scroll = 7
	u.seedEvents([]changeEvent{{Path: "new", Kind: "modified", Diff: "+seed", StatsKnown: true, At: time.Now()}})
	if u.selected != "existing" || u.listSelected != "existing" || u.scroll != 7 {
		t.Fatal("startup moved focus")
	}
}
