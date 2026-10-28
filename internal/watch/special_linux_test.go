//go:build linux

package watch

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestScanSpecialFilesWithoutOpeningThem(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := s["pipe"]; !ok || e.IsDir || len(e.Content) != 0 {
		t.Fatalf("FIFO entry missing or read: %#v", e)
	}
	// Even a root that is itself a FIFO must return without waiting for a writer.
	s, err = Scan(filepath.Join(root, "pipe"), 0)
	if err != nil || len(s) != 1 {
		t.Fatalf("FIFO root: %v %v", s, err)
	}
	// A symlink root is a node, never a traversal of its destination.
	if err = os.Symlink(root, filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
	s, err = Scan(filepath.Join(root, "self"), 5)
	if err != nil || len(s) != 1 || s["."].IsDir {
		t.Fatalf("symlink root: %v %v", s, err)
	}
}
