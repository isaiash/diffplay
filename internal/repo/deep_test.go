package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestDeepGitStartupScopeAndDepth(t *testing.T) {
	root := repository(t)
	java := "CentralControlManager/src/astri/cc/CentralControlManagerImpl/CentralControlManagerImpl.java"
	deep := "CentralControlManager/a/b/c/d/e/f/g/"
	for _, p := range []string{java, deep + "deleted", deep + "index", "outside"} {
		put(t, root, p, "base\n")
	}
	put(t, root, ".gitignore", "ignored/\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	put(t, root, java, "changed\n")
	put(t, root, "CentralControlManager/test/README_TESTS.md", "shallow\n")
	put(t, root, deep+"new", "new\n")
	put(t, root, "ignored/a/b/c/d/e/f/file", "hidden\n")
	if err := os.Remove(filepath.Join(root, deep+"deleted")); err != nil {
		t.Fatal(err)
	}
	put(t, root, deep+"index", "staged\n")
	git(t, root, "add", deep+"index")
	put(t, root, deep+"index", "base\n")
	for _, scope := range []string{".", "CentralControlManager"} {
		t.Run(scope, func(t *testing.T) {
			r, err := Open(filepath.Join(root, scope))
			if err != nil {
				t.Fatal(err)
			}
			for _, depth := range []int{watch.AllDepth, 99, 5} {
				s, err := r.Scan(depth)
				if err != nil {
					t.Fatal(err)
				}
				events, err := r.InitialChanges(s, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]watch.Event{}
				for _, e := range events {
					got[e.Path] = e
				}
				for p, kind := range map[string]string{java: "modified", deep + "deleted": "deleted", deep + "index": "modified", deep + "new": "created", "CentralControlManager/test/README_TESTS.md": "created"} {
					if scope != "." {
						p = strings.TrimPrefix(p, scope+"/")
					}
					if depth == 5 && strings.Count(p, "/")+1 > depth {
						if _, ok := got[p]; ok {
							t.Errorf("bounded scan included %s", p)
						}
						continue
					}
					e, ok := got[p]
					if !ok || e.Kind != kind || !e.StatsKnown {
						t.Errorf("depth %d %s: %+v", depth, p, e)
					}
					if kind != "deleted" {
						if _, ok := s[p]; !ok {
							t.Errorf("depth %d snapshot omitted %s", depth, p)
						}
					}
					if strings.HasSuffix(p, "index") && (e.Added != 0 || e.Removed != 0) {
						t.Errorf("index-only: %+v", e)
					}
				}
				for p := range s {
					if metadata(p) || strings.HasPrefix(p, "ignored") || p == "outside" && scope != "." {
						t.Errorf("out-of-scope path %s", p)
					}
				}
			}
		})
	}
}
