package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestLiveActivityUnbornDeletionAndCleanReset(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	m := activityMonitor(t, root)
	monitorPut(t, root, "dir/a", "first\n")
	up := m.poll(time.Now())
	if up.err != nil || up.warning != nil || up.modifications["dir/a"] != 1 || up.modifications["dir"] != 0 || !strings.Contains(up.previews["dir/a"], "+first") {
		t.Fatalf("unborn: %+v", up)
	}
	if err := os.Remove(filepath.Join(root, "dir/a")); err != nil {
		t.Fatal(err)
	}
	up = m.poll(time.Now())
	if n, ok := up.modifications["dir/a"]; !ok || n != 0 {
		t.Fatalf("deleted untracked count not explicitly cleared: %+v", up)
	}
	root = activityRepo(t)
	m = activityMonitor(t, root)
	monitorPut(t, root, "a", "edited\n")
	m.poll(time.Now())
	monitorPut(t, root, "a", "base\n")
	if up = m.poll(time.Now()); up.modifications["a"] != 0 {
		t.Fatal("clean reset failed")
	}
}

type failingHeadRepository struct {
	fixtureRepository
	headErr error
}

func (r *failingHeadRepository) HeadChanges(watch.Snapshot, time.Time) ([]watch.Event, error) {
	return nil, r.headErr
}

func TestLiveActivityHEADFailureDoesNotDiscardEvents(t *testing.T) {
	initial := watch.Snapshot{"a": {Path: "a", Content: []byte("before\n")}}
	source := &failingHeadRepository{fixtureRepository: fixtureRepository{snapshot: watch.Snapshot{"a": {Path: "a", Content: []byte("after\n")}}}, headErr: errors.New("HEAD unavailable")}
	m := newMonitor("/repo", source, initial, nil, true)
	up := m.poll(time.Now())
	if up.err != nil || up.warning == nil || len(up.events) != 1 || up.modifications["a"] != 1 {
		t.Fatalf("auxiliary failure lost event: %+v", up)
	}
	if !strings.Contains(up.previews["a"], "HEAD unavailable") {
		t.Fatalf("must not masquerade interval diff as HEAD preview: %+v", up)
	}
	source.headErr = nil
	up = m.poll(time.Now())
	if up.warning != nil || up.modifications["a"] != 0 {
		t.Fatalf("recovery: %+v", up)
	}
}

func TestLiveActivityStartupHEADFailureKeepsMonitoring(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := activityRepo(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/bin/sh\nfor arg do\n case \"$arg\" in status) echo status-unavailable >&2; exit 88;; esac\ndone\nexec \"$DIFFPLAY_REAL_GIT\" \"$@\"\n"
	if err = os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIFFPLAY_REAL_GIT", git)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	sessions, err := loadSessions(root, []string{root}, false)
	if err != nil {
		t.Fatalf("HEAD preview failure stopped startup: %v", err)
	}
	if sessions[0].activityErr == nil {
		t.Fatal("startup failure was silent")
	}
	monitorPut(t, root, "a", "base\nnew\n")
	up := sessions[0].newMonitor().poll(time.Now())
	if up.err != nil || up.warning == nil || len(up.events) == 0 || up.modifications["a"] != 1 {
		t.Fatalf("startup recovery lost events: %+v", up)
	}
}

func TestLiveActivityStorageOutageDoesNotResurrectCommittedCounts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := activityRepo(t)
	s := activitySession(t, root)
	m := s.newMonitor()
	monitorPut(t, root, "a", "base\none\n")
	if up := m.poll(time.Now()); up.warning != nil {
		t.Fatal(up.warning)
	}
	saved, err := os.ReadFile(s.store.path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(s.store.path); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(s.store.path, 0700); err != nil {
		t.Fatal(err)
	}
	monitorGit(t, root, "add", "a")
	monitorGit(t, root, "commit", "-qm", "during outage")
	up := m.poll(time.Now())
	if up.warning == nil || up.modifications["a"] != 0 {
		t.Fatalf("commit during outage: %+v", up)
	}
	monitorPut(t, root, "a", "base\none\ntwo\n")
	if up = m.poll(time.Now()); up.modifications["a"] != 1 {
		t.Fatalf("new burst: %+v", up)
	}
	if err = os.Remove(s.store.path); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(s.store.path, saved, 0600); err != nil {
		t.Fatal(err)
	}
	up = m.poll(time.Now())
	if up.warning != nil || up.modifications["a"] != 1 {
		t.Fatalf("committed count resurrected after repair: %+v", up)
	}
}

func TestLiveActivityStorageFailureRetriesWithoutLosingBursts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	root := activityRepo(t)
	s := activitySession(t, root)
	m := s.newMonitor()
	if err := os.Mkdir(s.store.path, 0700); err != nil {
		t.Fatal(err)
	}
	monitorPut(t, root, "a", "base\none\n")
	up := m.poll(time.Now())
	if up.err != nil || up.warning == nil || len(up.events) == 0 || up.modifications["a"] != 1 {
		t.Fatalf("storage failure froze monitoring: %+v", up)
	}
	monitorPut(t, root, "a", "base\none\ntwo\n")
	up = m.poll(time.Now())
	if up.err != nil || up.modifications["a"] != 2 {
		t.Fatalf("second failed-save burst: %+v", up)
	}
	if err := os.Remove(s.store.path); err != nil {
		t.Fatal(err)
	}
	up = m.poll(time.Now())
	if up.warning != nil || up.modifications["a"] != 2 {
		t.Fatalf("pending saves not recovered: %+v", up)
	}
	if s = activitySession(t, root); s.modifications["a"] != 2 {
		t.Fatal("recovered counts not saved")
	}
}
