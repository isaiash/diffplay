package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentPrivateImmutableWindowsAndClose(t *testing.T) {
	d, err := NewDocument(strings.NewReader("one\n\t界́\nlast"))
	if err != nil {
		t.Fatal(err)
	}
	dir := d.dir
	for _, p := range []string{dir, filepath.Join(dir, "text"), filepath.Join(dir, "index")} {
		s, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if s.Mode().Perm()&0077 != 0 {
			t.Fatalf("non-private snapshot %s", p)
		}
	}
	if d.Len() != 3 || strings.Join(d.Window(1, 2), "|") != "\t界́|last" {
		t.Fatal(d.Window(0, 5))
	}
	if len(d.Window(3, 10)) != 0 {
		t.Fatal("EOF should be empty")
	}
	d.Close()
	d.Close()
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("snapshot leaked")
	}
}
func TestDocumentLongLineIsBounded(t *testing.T) {
	d, err := NewDocument(strings.NewReader(strings.Repeat("x", 1024*1024) + "\nlast\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p := d.Window(0, 2)
	if len(p) != 2 || len(p[0]) > maxDisplayLine+50 || p[1] != "last" {
		t.Fatal("long line page corrupted")
	}
	if !strings.Contains(p[0], "long line clipped") {
		t.Fatal("clipping must be explicit")
	}
}
func BenchmarkDocumentWindow(b *testing.B) {
	d, err := NewDocument(strings.NewReader(strings.Repeat("+line\n", 100000)))
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d.Window((i*37)%99900, 24)
	}
}
func TestDocumentHunkStatsAndOffsets(t *testing.T) {
	text := "--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n---old\n+++new\n context\n@@ -100,1 +100,1 @@\n-old\n+new\n\\ No newline at end of file\n"
	d, err := NewDocument(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Added != 2 || d.Removed != 2 || d.FirstEdit != 1 {
		t.Fatal(fmt.Sprint(d.Added, d.Removed, d.FirstEdit))
	}
	if !strings.Contains(strings.Join(d.Window(d.Offset(100), 7), "\n"), "+new") {
		t.Fatal("latest hunk was not located")
	}
	many, err := NewDocument(strings.NewReader("--- a/x\n+++ b/x\n@@ -1,12000 +1,12001 @@\n" + strings.Repeat("-old\n", 12000) + strings.Repeat("+new\n", 12000) + "+LATEST\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer many.Close()
	if !strings.Contains(strings.Join(many.Window(many.Offset(12001), 5), "\n"), "LATEST") {
		t.Fatal("follow offset does not account for removed lines")
	}
}
