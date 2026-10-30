package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanPreviousReusesAvailableUnchangedSource(t *testing.T) {
	root := t.TempDir()
	content := strings.Repeat("x", ContentLimit+1) + "\n"
	path := filepath.Join(root, "large.txt")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(first)
	old := first["large.txt"].Source
	if old == nil || !old.Available() {
		t.Fatal("large UTF-8 source was not captured")
	}

	second, err := ScanPrevious(root, 1, first)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(second)
	if second["large.txt"].Source != old {
		t.Fatal("unchanged scan replaced disk-backed source")
	}
	CloseSnapshotExcept(first, second)
	if !old.Available() {
		t.Fatal("closing prior snapshot closed transferred source")
	}
	CloseSnapshot(second)
	if old.Available() {
		t.Fatal("closing owning snapshot left source available")
	}
}

func TestScanPreviousHashInvalidatesSameMetadataSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	write := func(char byte) {
		t.Helper()
		content := []byte(strings.Repeat("a", ContentLimit+1))
		content[len(content)-1] = char
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write('a')
	first, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(first)
	old := first["large.txt"].Source
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	write('b')
	if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	second, err := ScanPrevious(root, 1, first)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(second)
	if first["large.txt"].Size != second["large.txt"].Size || !first["large.txt"].ModTime.Equal(second["large.txt"].ModTime) {
		t.Fatal("test did not preserve size and modification time")
	}
	if first["large.txt"].Hash == second["large.txt"].Hash || second["large.txt"].Source == old {
		t.Fatal("full hash did not reject changed source")
	}
}

func TestScanPreviousRecapturesUnavailableSourceAndLegacyScanStillCaptures(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", ContentLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	old := first["large.txt"].Source
	unknown := Snapshot{"large.txt": first["large.txt"]}
	entry := unknown["large.txt"]
	entry.LinesKnown = false
	unknown["large.txt"] = entry
	withoutKnown, err := ScanPrevious(root, 1, unknown)
	if err != nil {
		CloseSnapshot(first)
		t.Fatal(err)
	}
	if withoutKnown["large.txt"].Source == old {
		t.Fatal("source with unknown previous line count was reused")
	}
	CloseSnapshot(withoutKnown)
	old.Close()
	second, err := ScanPrevious(root, 1, first)
	if err != nil {
		CloseSnapshot(first)
		t.Fatal(err)
	}
	defer CloseSnapshot(second)
	if second["large.txt"].Source == old || !second["large.txt"].Source.Available() {
		t.Fatal("unavailable previous source was not recaptured")
	}
	legacy, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(legacy)
	if legacy["large.txt"].Source == nil || !legacy["large.txt"].Source.Available() {
		t.Fatal("legacy finite Scan no longer captures large source")
	}
}

func TestLargeOutputPages(t *testing.T) {
	text := strings.Repeat("added line\n", 20000)
	e := Changes(nil, Snapshot{"f": {Content: []byte(text), Size: int64(len(text))}}, time.Time{})[0]
	if e.Document == nil {
		t.Fatal("large output has no paging backend")
	}
	defer e.Document.Close()
	if e.Document.Len() < 20000 || e.Added != 20000 || !e.StatsKnown {
		t.Fatalf("incomplete output: %+v", e)
	}
	for _, offset := range []int{0, 10000, 19995} {
		page := e.Document.Window(offset, 5)
		if len(page) != 5 {
			t.Fatalf("page %d: %v", offset, page)
		}
	}
	if len(e.Document.cache) > 53 {
		t.Fatalf("unbounded cache %d", len(e.Document.cache))
	}
	if len(e.Diff) > DiffLimit {
		t.Fatal("compat text is not bounded")
	}
}

func TestLargeFewEditsRemainAvailable(t *testing.T) {
	old := strings.Repeat("context\n", 4000) + "old\n"
	next := strings.Repeat("context\n", 4000) + "new\n"
	e := Changes(Snapshot{"f": {Content: []byte(old)}}, Snapshot{"f": {Content: []byte(next)}}, time.Time{})[0]
	if !e.StatsKnown || e.Added != 1 || e.Removed != 1 || !strings.Contains(e.Diff, "+new") {
		t.Fatalf("large sparse diff unavailable: %+v", e)
	}
}
