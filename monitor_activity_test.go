package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
)

func activityRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorGit(t, root, "config", "user.email", "test@example.invalid")
	monitorGit(t, root, "config", "user.name", "Test")
	monitorPut(t, root, "a", "base\n")
	monitorGit(t, root, "add", ".")
	monitorGit(t, root, "commit", "-qm", "base")
	return root
}
func activityMonitor(t *testing.T, root string) *monitor {
	t.Helper()
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := r.Scan(5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := r.InitialChanges(initial, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return newMonitor(root, r, initial, events, true)
}
func TestLiveActivityCachesHEADBaseline(t *testing.T) {
	root := activityRepo(t)
	m := activityMonitor(t, root)
	monitorPut(t, root, "a", "base\none\n")
	if up := m.poll(time.Now()); up.warning != nil {
		t.Fatal(up.warning)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/bin/sh\nfor arg do\n case \"$arg\" in cat-file) echo unexpected-baseline-read >&2; exit 88;; esac\ndone\nexec \"$DIFFPLAY_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIFFPLAY_REAL_GIT", git)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+originalPath)
	monitorPut(t, root, "a", "base\none\ntwo\n")
	up := m.poll(time.Now())
	if up.err != nil || up.warning != nil || !strings.Contains(up.previews["a"], "+one") {
		t.Fatalf("unchanged HEAD reread blobs: %+v", up)
	}
	t.Setenv("PATH", originalPath)
	monitorGit(t, root, "add", "a")
	monitorGit(t, root, "commit", "-qm", "new baseline")
	monitorPut(t, root, "a", "base\none\ntwo\nthree\n")
	up = m.poll(time.Now())
	if up.err != nil || up.warning != nil || strings.Contains(up.previews["a"], "+one") || !strings.Contains(up.previews["a"], "+three") {
		t.Fatalf("HEAD cache not invalidated: %+v", up)
	}
}

func activitySession(t *testing.T, root string) repositorySession {
	t.Helper()
	sessions, err := loadSessions(root, []string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].activityErr != nil {
		t.Fatal(sessions[0].activityErr)
	}
	return sessions[0]
}
func TestLiveActivityPersistsAcrossRestartAndPartialCommit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := activityRepo(t)
	monitorPut(t, root, "startup", "already dirty\n")
	s := activitySession(t, root)
	if s.modifications["startup"] != 0 {
		t.Fatal("startup diff counted")
	}
	m := s.newMonitor()
	monitorPut(t, root, "a", "base\none\n")
	m.poll(time.Now())
	monitorPut(t, root, "a", "base\none\ntwo\n")
	monitorPut(t, root, "other", "keep me\n")
	up := m.poll(time.Now())
	if up.warning != nil || up.modifications["a"] != 2 || up.modifications["other"] != 1 {
		t.Fatalf("live counts: %+v", up)
	}
	s = activitySession(t, root)
	if s.modifications["a"] != 2 || s.modifications["other"] != 1 || s.modifications["startup"] != 0 {
		t.Fatalf("restart counts: %+v", s.modifications)
	}
	m = s.newMonitor()
	monitorGit(t, root, "add", "a")
	monitorGit(t, root, "commit", "-qm", "partial")
	up = m.poll(time.Now())
	if up.warning != nil || len(up.events) != 0 || up.modifications["a"] != 0 || up.modifications["other"] != 1 {
		t.Fatalf("commit without filesystem change: %+v", up)
	}
	s = activitySession(t, root)
	if s.modifications["a"] != 0 || s.modifications["other"] != 1 {
		t.Fatalf("reset did not persist: %+v", s.modifications)
	}
	monitorGit(t, root, "add", "other")
	monitorGit(t, root, "commit", "-qm", "remaining")
	if s = activitySession(t, root); s.modifications["other"] != 0 {
		t.Fatal("offline commit did not reset count")
	}
}

func TestLiveActivityCountsBurstsAndPreviewsHEAD(t *testing.T) {
	root := activityRepo(t)
	m := activityMonitor(t, root)
	monitorPut(t, root, "a", "base\none\n")
	first := m.poll(time.Now())
	if first.err != nil || first.modifications["a"] != 1 {
		t.Fatalf("first burst: %+v", first)
	}
	monitorPut(t, root, "a", "base\none\ntwo\n")
	second := m.poll(time.Now())
	if second.err != nil || second.modifications["a"] != 2 {
		t.Fatalf("second burst: %+v", second)
	}
	if !strings.Contains(second.previews["a"], "+one") || !strings.Contains(second.previews["a"], "+two") {
		t.Fatalf("preview is not HEAD relative: %q", second.previews["a"])
	}
	for _, event := range second.events {
		if event.Path == "a" && (event.Added != 1 || event.Removed != 0) {
			t.Fatalf("interval stats replaced: %+v", event)
		}
	}
	if first.modifications["a"] != 1 {
		t.Fatal("previous update map mutated")
	}
	idle := m.poll(time.Now())
	if idle.modifications["a"] != 2 || len(idle.previews) != 0 {
		t.Fatalf("idle poll counted an event: %+v", idle)
	}
}
