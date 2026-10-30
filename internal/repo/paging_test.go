package repo

import (
	"github.com/isaiash/diffplay/internal/watch"
	"strings"
	"testing"
	"time"
)

func TestLargeHeadPreviewCacheAndInvalidation(t *testing.T) {
	root := repository(t)
	old := strings.Repeat("old line text\n", 30000)
	next := strings.Repeat("new line text\n", 30000)
	put(t, root, "large", old)
	git(t, root, "add", "large")
	git(t, root, "commit", "-qm", "base")
	put(t, root, "large", next)
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	current, err := r.Scan(watch.AllDepth)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(current, time.Time{})
	if err != nil || len(events) != 1 {
		t.Fatal(events, err)
	}
	first := events[0]
	if first.Document == nil || first.Added != 30000 || first.Removed != 30000 || !first.StatsKnown {
		t.Fatal("large HEAD diff is incomplete")
	}
	events, err = r.HeadChanges(current, time.Now())
	if err != nil || events[0].Document != first.Document {
		t.Fatal("unchanged preview was regenerated")
	}
	first.Document.Close()
	events, err = r.HeadChanges(current, time.Now())
	if err != nil || events[0].Document == first.Document || !events[0].Document.Available() {
		t.Fatal("closed preview cache was reused")
	}
	first = events[0]
	put(t, root, "large", next+"LATEST\n")
	changed, err := r.Scan(watch.AllDepth)
	if err != nil {
		t.Fatal(err)
	}
	events, err = r.HeadChanges(changed, time.Now())
	if err != nil || events[0].Document == first.Document {
		t.Fatal("changed source reused stale preview")
	}
	if !strings.Contains(strings.Join(events[0].Document.Window(events[0].Document.Len()-4, 4), "\n"), "LATEST") {
		t.Fatal("missing latest line")
	}
	interval := watch.Changes(current, changed, time.Now())
	if len(interval) != 1 || interval[0].Added != 1 || interval[0].Removed != 0 {
		t.Fatal("interval counts changed")
	}
	git(t, root, "add", "large")
	git(t, root, "commit", "-qm", "new HEAD")
	events, err = r.HeadChanges(changed, time.Now())
	if err != nil || len(events) != 0 {
		t.Fatal("HEAD change did not invalidate dirty preview")
	}
}

func TestRepositoryScanPreviousReusesLargeSource(t *testing.T) {
	root := repository(t)
	put(t, root, "large", strings.Repeat("large text line\n", 20000))
	git(t, root, "add", "large")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Scan(watch.AllDepth)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.CloseSnapshot(first)
	previous := first["large"].Source
	if previous == nil {
		t.Fatal("large source was not captured")
	}
	second, err := r.ScanPrevious(watch.AllDepth, first)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.CloseSnapshot(second)
	if second["large"].Source != previous || !previous.Available() {
		t.Fatal("Git scan did not reuse unchanged source")
	}
}
