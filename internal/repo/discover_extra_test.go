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
	if err != nil || len(got) != 0 {
		t.Fatalf("unexpected symlink traversal: %q %v", got, err)
	}
}

// macOS temporary paths have a symlinked ancestor (/var -> /private/var).
// Git reports the physical path, but discovery must retain the caller's path.
func TestDiscoverWithSymlinkedAncestor(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "repository"
		if linked {
			name = "linked worktree"
		}
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			physical := filepath.Join(parent, "physical")
			if err := os.MkdirAll(filepath.Join(physical, "projects"), 0755); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(parent, "alias")
			if err := os.Symlink(physical, alias); err != nil {
				t.Fatal(err)
			}
			alias = filepath.Join(alias, "projects")
			actual := filepath.Join(physical, "projects", name)
			if linked {
				root := repository(t)
				git(t, root, "commit", "--allow-empty", "-qm", "baseline")
				git(t, root, "worktree", "add", "--detach", actual)
			} else {
				if err := os.Mkdir(actual, 0755); err != nil {
					t.Fatal(err)
				}
				git(t, actual, "init", "-q")
			}
			want := filepath.Join(alias, name)
			for _, tc := range []struct {
				root  string
				depth int
			}{
				{want, 0},
				{filepath.Join(alias, "."), 1},
			} {
				got, err := Discover(tc.root, tc.depth)
				if err != nil || !reflect.DeepEqual(got, []string{want}) {
					t.Fatalf("Discover(%q, %d) = %q, %v; want %q", tc.root, tc.depth, got, err, []string{want})
				}
			}
		})
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
