package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverDepthAndSymlinkTraversal(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, nested, "init", "-q")
	got, err := Discover(root, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("depth cap: %q %v", got, err)
	}
	if err := os.Symlink(nested, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	got, err = Discover(root, 2)
	if err != nil || !reflect.DeepEqual(got, []string{nested}) {
		t.Fatalf("unexpected symlink traversal: %q %v", got, err)
	}
}

func TestDiscoverLinkedWorktreeMarker(t *testing.T) {
	root := repository(t)
	put(t, root, "file", "base\n")
	git(t, root, "add", ".")
	git(t, root, "-c", "user.name=Graph Test", "-c", "user.email=graph@example.invalid", "commit", "-qm", "baseline")
	worktree := filepath.Join(t.TempDir(), "linked worktree")
	git(t, root, "worktree", "add", "--detach", worktree)
	got, err := Discover(filepath.Dir(worktree), 2)
	if err != nil || !reflect.DeepEqual(got, []string{worktree}) {
		t.Fatalf("linked worktree discovery: %q %v", got, err)
	}
}
