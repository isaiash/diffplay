package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSymlinkNodesDoNotFollowTargets(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	put(t, outside, "file", "outside secret")
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}
	before, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 3 || before["link"].IsDir || len(before["link"].Content) != 0 {
		t.Fatalf("followed symlink: %v", keys(before))
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside+"-other", link); err != nil {
		t.Fatal(err)
	}
	after, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	if before["link"].Hash == after["link"].Hash {
		t.Fatal("symlink target text was not fingerprinted")
	}
}

func TestScanReturnsPartialOnPermissions(t *testing.T) {
	root := t.TempDir()
	put(t, root, "a-readable", "ok")
	put(t, root, "z-denied/file", "secret")
	denied := filepath.Join(root, "z-denied")
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(denied, 0755)
	if _, err := os.ReadDir(denied); err == nil {
		t.Skip("current user bypasses directory permissions")
	}
	s, err := Scan(root, 5)
	if err == nil {
		t.Fatal("permission error suppressed")
	}
	if _, ok := s["a-readable"]; !ok {
		t.Fatal("partial snapshot discarded")
	}
	if _, err = Scan(filepath.Join(root, "missing"), 5); err == nil {
		t.Fatal("missing root error suppressed")
	}
}

func TestHashDetectsChangesBeyondCapturedPrefix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	data := make([]byte, 400*1024)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	before, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] = 'b'
	if err = os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, before["file"].ModTime, before["file"].ModTime); err != nil {
		t.Fatal(err)
	}
	after, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	events := Changes(before, after, time.Now())
	if len(events) != 1 || events[0].Path != "file" {
		t.Fatalf("missed tail edit: %v", events)
	}
}
