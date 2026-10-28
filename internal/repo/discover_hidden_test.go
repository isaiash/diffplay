package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverSkipsAllHiddenDirectorySubtrees(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"visible", ".hidden-repository", ".cache/nested-repository", "ordinary/.private/repository"} {
		dir := filepath.Join(root, relative)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "init", "-q")
	}
	// An invalid marker inside a hidden subtree must never be inspected either.
	broken := filepath.Join(root, ".broken")
	if err := os.MkdirAll(broken, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: missing-directory\n"), 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := Discover(root, 5)
	if err != nil || !reflect.DeepEqual(paths, []string{filepath.Join(root, "visible")}) {
		t.Fatalf("hidden discovery leaked paths or errors: %q, %v", paths, err)
	}
}

func TestExplicitHiddenDiscoveryRootStillWorks(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".explicit")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	paths, err := Discover(root, 5)
	if err != nil || !reflect.DeepEqual(paths, []string{root}) {
		t.Fatalf("explicit root must not be filtered: %q %v", paths, err)
	}
}
