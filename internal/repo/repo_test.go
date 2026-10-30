package repo

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return string(out)
}
func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "user.name", "Test")
	return root
}
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
func TestScanRespectsGitIgnoresAndDepth(t *testing.T) {
	root := repository(t)
	put(t, root, ".gitignore", "ignored/\n*.tmp\n")
	put(t, root, "ignored/tracked.tmp", "kept\n")
	git(t, root, "add", "-f", "ignored/tracked.tmp")
	put(t, root, "ignored/hidden", "hidden")
	put(t, root, "a.tmp", "hidden")
	put(t, root, "nested/.gitignore", "local\n")
	put(t, root, "nested/local", "hidden")
	put(t, root, "nested/visible", "visible")
	put(t, root, "excluded", "hidden")
	put(t, root, ".git/info/exclude", "excluded\n")
	global := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(global, []byte("global\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "config", "core.excludesFile", global)
	put(t, root, "global", "hidden")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".", ".gitignore", "ignored", "ignored/tracked.tmp", "nested/visible"} {
		if _, ok := s[p]; !ok {
			t.Errorf("missing %q", p)
		}
	}
	for _, p := range []string{".git", "ignored/hidden", "a.tmp", "nested/local", "excluded", "global"} {
		if _, ok := s[p]; ok {
			t.Errorf("included %q", p)
		}
	}
	sub, err := Open(filepath.Join(root, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	s, err = sub.Scan(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s["visible"]; !ok {
		t.Fatal("subdirectory-relative path absent")
	}
	if _, ok := s["local"]; ok {
		t.Fatal("nested ignore lost")
	}
	s, err = r.Scan(0)
	if err != nil || len(s) != 1 {
		t.Fatalf("depth zero: %v, %v", s, err)
	}
}

func TestScanKeepsIgnoredHEADFileRemovedFromIndex(t *testing.T) {
	root := repository(t)
	put(t, root, ".gitignore", "ignored/\n")
	put(t, root, "ignored/nested/tracked", "base\n")
	git(t, root, "add", ".gitignore")
	git(t, root, "add", "-f", "ignored/nested/tracked")
	git(t, root, "commit", "-qm", "base")
	git(t, root, "rm", "--cached", "-q", "ignored/nested/tracked")
	put(t, root, "ignored/untracked", "hidden\n")

	for _, scope := range []string{".", "ignored"} {
		t.Run(scope, func(t *testing.T) {
			r, err := Open(filepath.Join(root, scope))
			if err != nil {
				t.Fatal(err)
			}
			s, err := r.Scan(5)
			if err != nil {
				t.Fatal(err)
			}
			p := "nested/tracked"
			if scope == "." {
				p = "ignored/" + p
			}
			for ancestor := p; ; ancestor = filepath.ToSlash(filepath.Dir(ancestor)) {
				if _, ok := s[ancestor]; !ok {
					t.Errorf("snapshot missing HEAD-tracked path or ancestor %q", ancestor)
				}
				if ancestor == "." {
					break
				}
			}
			if _, ok := s["untracked"]; ok {
				t.Error("included ignored untracked file")
			}
			if _, ok := s["ignored/untracked"]; ok {
				t.Error("included ignored untracked file")
			}
			events, err := r.InitialChanges(s, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 || events[0].Path != p || events[0].Kind != "modified" || !events[0].StatsKnown || events[0].Added != 0 || events[0].Removed != 0 {
				t.Errorf("index-only removal should have no net content edits: %+v", events)
			}
		})
	}
}

func TestFilterPathsRefreshesIgnoresAndKeepsTrackedDeletions(t *testing.T) {
	root := repository(t)
	put(t, root, "tracked", "x")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	git(t, root, "rm", "-q", "tracked")
	put(t, root, "free", "x")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"free", "tracked", "missing", ".git/config", "../escape"}
	allowed, err := r.FilterPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed["free"] || !allowed["tracked"] || !allowed["missing"] || allowed[".git/config"] || allowed["../escape"] {
		t.Fatalf("initial filter: %v", allowed)
	}
	put(t, root, ".gitignore", "free\ntracked\nmissing\n")
	allowed, err = r.FilterPaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if allowed["free"] || allowed["missing"] || !allowed["tracked"] {
		t.Fatalf("updated filter: %v", allowed)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err = r.FilterPaths(paths); err == nil {
		t.Fatal("Git failure treated as clean")
	}
}

func TestInitialChangesUsesHEADToWorktree(t *testing.T) {
	root := repository(t)
	for p, text := range map[string]string{"both": "base\n", "deleted": "gone\n", "rename old": "move\n", "zero": "same\n", "clean": "clean\n"} {
		put(t, root, p, text)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	put(t, root, "both", "staged\n")
	put(t, root, "zero", "staged\n")
	git(t, root, "add", "both", "zero")
	put(t, root, "both", "final\nextra\n")
	put(t, root, "zero", "same\n")
	if err := os.Remove(filepath.Join(root, "deleted")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "mv", "rename old", "rename\nnew")
	put(t, root, ":(glob) new\nfile", "new\n")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	events, err := r.InitialChanges(s, now)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]watch.Event{}
	for _, e := range events {
		if _, ok := got[e.Path]; ok {
			t.Fatalf("duplicate %q", e.Path)
		}
		got[e.Path] = e
		if !e.At.Equal(now) {
			t.Fatal("timestamp lost")
		}
	}
	if len(got) != 6 {
		t.Fatalf("events: %+v", got)
	}
	for p, kind := range map[string]string{"both": "modified", "deleted": "deleted", "rename old": "deleted", "rename\nnew": "created", ":(glob) new\nfile": "created", "zero": "modified"} {
		if got[p].Kind != kind {
			t.Errorf("%q: %+v", p, got[p])
		}
	}
	if e := got["both"]; !e.StatsKnown || e.Added != 2 || e.Removed != 1 {
		t.Errorf("net stats: %+v", e)
	}
	if e := got["zero"]; !e.StatsKnown || e.Added != 0 || e.Removed != 0 {
		t.Errorf("index-only stats: %+v", e)
	}
}

func TestInitialChangesUnbornKeepsIndexOnlyChanges(t *testing.T) {
	root := repository(t)
	put(t, root, "staged", "one\n")
	put(t, root, "removed", "gone\n")
	git(t, root, "add", ".")
	if err := os.Remove(filepath.Join(root, "removed")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "untracked", "two\n")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
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
	if len(got) != 3 || got["staged"].Kind != "created" || got["untracked"].Added != 1 || !got["removed"].StatsKnown || got["removed"].Added != 0 || got["removed"].Removed != 0 {
		t.Fatalf("unborn status: %+v", got)
	}
}

func TestInitialChangesModeOnlyBinaryHasKnownZeroStats(t *testing.T) {
	root := repository(t)
	put(t, root, "binary", "a\x00b")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	git(t, root, "update-index", "--chmod=+x", "binary")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].StatsKnown || events[0].Added != 0 || events[0].Removed != 0 {
		t.Fatalf("metadata-only binary: %+v", events)
	}
}

func TestInitialChangesScopeDepthAndFailure(t *testing.T) {
	root := repository(t)
	for _, p := range []string{"outside", "sub/top", "sub/deep/file", "sub/a/b/c/d/e/file"} {
		put(t, root, p, "base\n")
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	for _, p := range []string{"outside", "sub/top", "sub/deep/file", "sub/a/b/c/d/e/file"} {
		if err := os.Remove(filepath.Join(root, p)); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(1)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Path != "top" || events[0].Kind != "deleted" {
		t.Fatalf("scoped shallow deletions: %+v", events)
	}
	s, err = r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err = r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("explicit depth five bound: %+v", events)
	}
	allowed, err := r.FilterPaths([]string{"top", "deep/file"})
	if err != nil || !allowed["top"] || !allowed["deep/file"] {
		t.Fatalf("scoped tracked deletion filter: %v %v", allowed, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err = r.Scan(5); err == nil {
		t.Fatal("scan swallowed Git failure")
	}
	if _, err = r.InitialChanges(s, time.Now()); err == nil {
		t.Fatal("initial status swallowed Git failure")
	}
}

func TestInitialChangesBinarySymlinkAndTruncation(t *testing.T) {
	root := repository(t)
	put(t, root, "binary", "base\x00data")
	put(t, root, "large", string(make([]byte, watch.ContentLimit+1)))
	put(t, root, "target", "one\n")
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	put(t, root, "binary", "new\x00data")
	put(t, root, "large", "small\n")
	if err := os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("special-file events: %+v", events)
	}
	for _, e := range events {
		if e.StatsKnown {
			t.Errorf("uncaptured content has known stats: %+v", e)
		}
	}
}

func TestInitialChangesLargeIndexOnlyHasKnownZeroStats(t *testing.T) {
	root := repository(t)
	text := string(bytes.Repeat([]byte("line\n"), watch.ContentLimit/5+2))
	put(t, root, "large", text)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "base")
	put(t, root, "large", "intermediate\n")
	git(t, root, "add", "large")
	put(t, root, "large", text)
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].StatsKnown || events[0].Added != 0 || events[0].Removed != 0 {
		t.Fatalf("large index-only status: %+v", events)
	}
	git(t, root, "reset", "--hard", "-q", "HEAD")
	git(t, root, "update-index", "--chmod=+x", "large")
	if err := os.Chmod(filepath.Join(root, "large"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err = r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err = r.InitialChanges(s, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].StatsKnown {
		t.Fatalf("large mode-only status: %+v", events)
	}
}

func TestOpen(t *testing.T) {
	root := repository(t)
	put(t, root, "sub/file", "x")
	if _, err := Open(filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotRepository) {
		t.Fatalf("not a repository: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := Open(root); err == nil || errors.Is(err, ErrNotRepository) {
		t.Fatalf("missing git classified as non-repository: %v", err)
	}
}
