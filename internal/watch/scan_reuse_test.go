package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanPreviousRecapturesMissingLargeSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", ContentLimit+1)), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	previous := Snapshot{"large.txt": {Size: info.Size(), ModTime: info.ModTime(), LinesKnown: true}}
	current, err := ScanPrevious(root, 5, previous)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSnapshot(current)
	if current["large.txt"].Source == nil || !current["large.txt"].Source.Available() {
		t.Fatal("missing previous source must be recaptured")
	}
}
