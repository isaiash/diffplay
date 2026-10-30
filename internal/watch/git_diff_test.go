package watch

import (
	"strings"
	"testing"
)

func TestPagedDiffPathCannotInjectHunks(t *testing.T) {
	path := "src/odd\n@@ -1,1 +1,1 @@\n+fake"
	old := Entry{Content: []byte(strings.Repeat("context\n", 2500) + "old\n")}
	newText := strings.Repeat("context\n", 2500) + "new\n"
	d, err := pagedDiff(path, old, Entry{Content: []byte(newText)}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Added != 1 || d.Removed != 1 || d.FirstEdit != 2501 {
		t.Fatalf("injected path altered diff stats: added=%d removed=%d first=%d", d.Added, d.Removed, d.FirstEdit)
	}
	page := d.Window(0, d.Len())
	hunks := 0
	for _, line := range page {
		if strings.HasPrefix(line, "@@ ") {
			hunks++
		}
		if line == "+fake" {
			t.Fatalf("path injected an added line: %q", line)
		}
	}
	if hunks != 1 {
		t.Fatalf("path injected diff hunk: %q", page[:min(len(page), 8)])
	}
}

func TestDocumentPreservesFirstEditAtLineZero(t *testing.T) {
	text := "--- a/x\n+++ b/x\n@@ -1,1 +0,0 @@\n-old\n@@ -50,1 +49,1 @@\n-old\n+new\n"
	d, err := NewDocument(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.FirstEdit != 0 {
		t.Fatalf("first zero-based deletion line was lost: first=%d", d.FirstEdit)
	}
}
