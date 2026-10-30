package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestUnlimitedGitScanPreservesTraversalBoundaries(t *testing.T) {
	root := repository(t)
	module := repository(t)
	put(t, module, "a/b/c/d/e/f/g/private", "module content\n")
	git(t, module, "add", ".")
	git(t, module, "commit", "-qm", "base")
	git(t, root, "-c", "protocol.file.allow=always", "submodule", "add", "-q", module, "a/b/c/d/e/f/g/module")
	put(t, root, ".gitignore", "ignored/\n")
	put(t, root, "ignored/a/b/c/d/e/f/g/private", "ignored content\n")
	outside := t.TempDir()
	put(t, outside, "private", "outside content\n")
	link := "a/b/c/d/e/f/g/link"
	if err := os.Symlink(outside, filepath.Join(root, link)); err != nil {
		t.Fatal(err)
	}
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(watch.AllDepth)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s[link]; !ok {
		t.Error("symlink node missing")
	}
	if !s["a/b/c/d/e/f/g/module"].IsDir {
		t.Error("submodule node missing")
	}
	for p := range s {
		if metadata(p) || strings.HasPrefix(p, "ignored") || strings.HasPrefix(p, link+"/") || strings.HasPrefix(p, "a/b/c/d/e/f/g/module/") {
			t.Errorf("crossed traversal boundary: %s", p)
		}
	}
}
