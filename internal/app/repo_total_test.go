package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
)

func TestRepositoryTotalLiveUnknownAndRecovery(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "text", "one\n")
	sessions, err := loadSessions(root, []string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	seedSessions(u, sessions)
	m := newMonitor(root, sessions[0].source, sessions[0].initial, sessions[0].events, true)
	monitorPut(t, root, "binary", "\xff")
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(up)
	if _, known := u.repositoryLineTotal(); known {
		t.Fatal("invalid text must make aggregate unknown")
	}
	if err := os.Remove(filepath.Join(root, "binary")); err != nil {
		t.Fatal(err)
	}
	monitorPut(t, root, "text", "one\ntwo")
	up = m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(up)
	if n, known := u.repositoryLineTotal(); !known || n != 2 {
		t.Fatalf("recovered total=%d known=%v", n, known)
	}
}

func TestRepositoryTotalUnknownFilesAndRootScope(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "outside", "outside\n")
	monitorPut(t, root, "sub/plain", "one\ntwo")
	monitorPut(t, root, "sub/empty", "")
	monitorPut(t, root, "sub/binary", "a\x00b\n")
	r, err := gitrepo.Open(filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	n, known, err := r.LineTotal()
	if n != 2 || !known || err != nil {
		t.Fatalf("binary total=%d known=%v err=%v", n, known, err)
	}
	if err := os.Remove(filepath.Join(root, "sub/binary")); err != nil {
		t.Fatal(err)
	}
	n, known, err = r.LineTotal()
	if n != 2 || !known || err != nil {
		t.Fatalf("text total=%d known=%v err=%v", n, known, err)
	}
	monitorPut(t, root, "sub/invalid", "\xff")
	_, known, err = r.LineTotal()
	if known || err != nil {
		t.Fatalf("invalid UTF-8 known=%v err=%v", known, err)
	}
	if err := os.Remove(filepath.Join(root, "sub/invalid")); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(root, "sub/huge"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	_, known, err = r.LineTotal()
	if known || err != nil {
		t.Fatalf("oversize known=%v err=%v", known, err)
	}
}

func TestRepositoryTotalSkipsTrackedTreeReplacedBySymlink(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "tracked/file", "tracked\n")
	monitorGit(t, root, "add", "tracked/file")
	if err := os.RemoveAll(filepath.Join(root, "tracked")); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	monitorPut(t, external, "file", "outside\noutside\n")
	if err := os.Symlink(external, filepath.Join(root, "tracked")); err != nil {
		t.Fatal(err)
	}
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	n, known, err := r.LineTotal()
	if n != 0 || !known || err != nil {
		t.Fatalf("total=%d known=%v err=%v", n, known, err)
	}
}

func TestRepositoryTotalsSeedPollAndPreserveFailedRepository(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, root := range []string{a, b} {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		monitorGit(t, root, "init", "-q")
	}
	monitorPut(t, a, "a/b/c/d/e/f/x", "one\ntwo")
	monitorPut(t, b, "x", "three\n")
	sessions, err := loadSessions(base, []string{a, b}, true)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), base)
	seedSessions(u, sessions)
	counter, ok := any(u).(interface{ repositoryLineTotal() (int, bool) })
	if !ok {
		t.Fatal("UI does not provide repository total")
	}
	check := func(want int, known bool) {
		t.Helper()
		n, k := counter.repositoryLineTotal()
		if n != want || k != known {
			t.Fatalf("total=%d known=%v; want %d %v", n, k, want, known)
		}
	}
	check(3, true)
	s := sessions[0]
	m := newMonitor(s.root, s.source, s.initial, s.events, s.scanErr == nil)
	monitorPut(t, a, "a/b/c/d/e/f/x", "one\ntwo\nthree\nfour\n")
	up := m.poll(time.Now())
	if up.err != nil {
		t.Fatal(up.err)
	}
	u.applyScanUpdate(namespaceUpdate(s.prefix, up))
	check(5, true)
	if err := os.Rename(filepath.Join(a, ".git"), filepath.Join(a, ".git.bak")); err != nil {
		t.Fatal(err)
	}
	up = m.poll(time.Now())
	if up.err == nil {
		t.Fatal("expected failed poll")
	}
	u.applyScanUpdate(namespaceUpdate(s.prefix, up))
	check(5, true)
}

func TestRepositoryTotalIncludesDeepGitAllowedText(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, ".gitignore", "ignored\n")
	monitorPut(t, root, "ignored", "not\ncounted\n")
	monitorPut(t, root, "a/b/c/d/e/f/deep", "one\ntwo")
	monitorPut(t, root, "empty", "")
	monitorPut(t, root, "tracked", "three\n")
	monitorGit(t, root, "add", "tracked")
	if err := os.Symlink(filepath.Join(root, "tracked"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	counter, ok := any(r).(interface{ LineTotal() (int, bool, error) })
	if !ok {
		t.Fatal("repository does not provide a full line total")
	}
	n, known, err := counter.LineTotal()
	if err != nil || !known || n != 4 {
		t.Fatalf("total=%d known=%v err=%v; want 4 known", n, known, err)
	}
}
