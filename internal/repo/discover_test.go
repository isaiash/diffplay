package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestDiscoverInvalidRootAndDepth(t *testing.T) {
	root := repository(t)
	put(t, root, "ordinary/file", "x")
	for _, path := range []string{filepath.Join(root, "missing"), filepath.Join(root, "ordinary/file")} {
		if _, err := Discover(path, 0); err == nil {
			t.Errorf("expected invalid root error for %s", path)
		}
	}
	if _, err := Discover(root, -1); err == nil {
		t.Error("negative depth must fail")
	}
}

func TestDiscoverRejectsFalseAndSymlinkMarkers(t *testing.T) {
	root := repository(t)
	put(t, root, "false/.git/not-git", "x")
	if err := os.MkdirAll(filepath.Join(root, "alias"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "alias/.git")); err != nil {
		t.Fatal(err)
	}
	got, _ := Discover(root, 5)
	if !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("false roots: %q", got)
	}
}

func TestDiscoverNestedSortedRoots(t *testing.T) {
	root := repository(t)
	nested := filepath.Join(root, "a spaced", "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, nested, "init", "-q")
	put(t, root, "ordinary/file", "hello")
	got, err := Discover(root, 5)
	want := []string{root, nested}
	sort.Strings(want)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover = %q, %v; want %q", got, err, want)
	}
}
