package repo

import (
	"path/filepath"
	"testing"
)

func TestTrackedPathsIncludesIndexAndHEADWithinScope(t *testing.T) {
	root := repository(t)
	put(t, root, "sub/committed", "one\n")
	put(t, root, "outside", "outside\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	git(t, root, "rm", "-q", "sub/committed")
	put(t, root, "sub/added", "two\n")
	put(t, root, "sub/untracked", "three\n")
	git(t, root, "add", "sub/added")
	r, err := Open(filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	tracked, err := r.TrackedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if !tracked["committed"] || !tracked["added"] || !tracked["."] || tracked["untracked"] || tracked["outside"] || tracked["sub/committed"] {
		t.Fatalf("wrong scope or HEAD/index union: %v", tracked)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := r.TrackedPaths(); err == nil {
		t.Fatal("Git error was not reported")
	}
}
