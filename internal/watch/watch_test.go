package watch

import (
	"bytes"
	"os"

	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func put(t *testing.T, root, path, text string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func keys(s Snapshot) []string {
	var out []string
	for p := range s {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
func TestScanBoundedContent(t *testing.T) {
	root := t.TempDir()
	put(t, root, "large.txt", string(bytes.Repeat([]byte("x"), 300*1024)))
	put(t, root, "binary", "a\x00b")
	s, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s["large.txt"].Content) != 256*1024 || !s["large.txt"].Truncated {
		t.Fatalf("large capture len=%d truncated=%v", len(s["large.txt"].Content), s["large.txt"].Truncated)
	}
	if len(s["binary"].Content) != 0 || s["binary"].Hash == ([32]byte{}) {
		t.Fatal("binary must be hashed but not captured as text")
	}
	// Sparse files must not cause proportional I/O or allocations.
	f, err := os.Create(filepath.Join(root, "huge"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(1 << 40); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s, err = Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !s["huge"].Truncated {
		t.Fatal("huge file not marked truncated")
	}
}

func TestScanDepthHiddenAndContent(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".hidden", "secret\n")
	put(t, root, "a/b/c/d/e/f.txt", "deep")
	put(t, root, "a/note", "hello\n")
	s, err := Scan(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", ".hidden", "a", "a/b", "a/note"}
	if !reflect.DeepEqual(keys(s), want) {
		t.Fatalf("paths %v, want %v", keys(s), want)
	}
	if string(s["a/note"].Content) != "hello\n" || s["a/note"].Path != "a/note" || !s["a"].IsDir {
		t.Fatalf("bad entries: %#v", s)
	}
	s, err = Scan(root, 99)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s["a/b/c/d/e/f.txt"]; ok {
		t.Fatal("walk exceeded maximum depth five")
	}
	if _, ok := s["a/b/c/d/e"]; !ok {
		t.Fatal("depth five omitted")
	}
	s, err = Scan(root, 0)
	if err != nil || len(s) != 1 {
		t.Fatalf("depth zero: %v %v", s, err)
	}
}
