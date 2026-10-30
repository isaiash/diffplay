package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const deepJava = "CentralControlManager/src/astri/cc/CentralControlManagerImpl/CentralControlManagerImpl.java"
const deeperFile = "CentralControlManager/a/b/c/d/e/f/g/file"
const shallowTests = "CentralControlManager/test/README_TESTS.md"

func deepRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorGit(t, root, "config", "user.email", "test@example.invalid")
	monitorGit(t, root, "config", "user.name", "Test")
	for _, p := range []string{deepJava, deeperFile, shallowTests} {
		monitorPut(t, root, p, "base\n")
	}
	monitorPut(t, root, ".gitignore", "ignored/\n")
	monitorGit(t, root, "add", ".")
	monitorGit(t, root, "commit", "-qm", "base")
	return root
}

func TestDeepGitSessionsStartupAndLive(t *testing.T) {
	for _, scope := range []string{".", "CentralControlManager"} {
		t.Run(scope, func(t *testing.T) {
			root := deepRepository(t)
			for _, p := range []string{deepJava, shallowTests} {
				monitorPut(t, root, p, "startup\n")
			}
			monitorPut(t, root, "ignored/a/b/c/d/e/f/file", "hidden\n")
			opened := filepath.Join(root, scope)
			sessions, err := loadSessions(opened, []string{opened}, false)
			if err != nil {
				t.Fatal(err)
			}
			s := sessions[0]
			if s.scanErr != nil {
				t.Fatal(s.scanErr)
			}
			relative := func(p string) string {
				if scope != "." {
					return strings.TrimPrefix(p, scope+"/")
				}
				return p
			}
			found := map[string]changeEvent{}
			for _, e := range s.events {
				found[e.Path] = e
			}
			for _, p := range []string{deepJava, shallowTests} {
				e := found[relative(p)]
				if e.Kind != "modified" || e.Added != 1 || e.Removed != 1 {
					t.Errorf("startup %s: %+v", p, e)
				}
			}
			if _, ok := s.initial[relative(deeperFile)]; !ok {
				t.Error("clean deep tracked context missing")
			}
			m := s.newMonitor()
			for _, p := range []string{deepJava, deeperFile, shallowTests} {
				monitorPut(t, root, p, "live\nextra\n")
			}
			created := "CentralControlManager/a/b/c/d/e/f/g/new"
			monitorPut(t, root, created, "new\n")
			up := m.poll(time.Now())
			if up.err != nil {
				t.Fatal(up.err)
			}
			got := map[string]changeEvent{}
			for _, e := range up.events {
				if !e.IsDir {
					got[e.Path] = e
				}
			}
			for _, p := range []string{deepJava, deeperFile, shallowTests, created} {
				id := relative(p)
				e := got[id]
				want := "modified"
				if p == created {
					want = "created"
				}
				if e.Kind != want || !e.StatsKnown || up.modifications[id] != 1 || !strings.Contains(up.previews[id], "+") {
					t.Errorf("live %s: event=%+v hits=%d preview=%q", p, e, up.modifications[id], up.previews[id])
				}
				if !up.snapshot[id].LinesKnown || up.snapshot[id].Lines < 1 {
					t.Errorf("line metadata missing: %s", id)
				}
			}
			for _, p := range []string{deepJava, created} {
				if err := os.Remove(filepath.Join(root, p)); err != nil {
					t.Fatal(err)
				}
			}
			up = m.poll(time.Now())
			if up.err != nil {
				t.Fatal(up.err)
			}
			deleted := false
			for _, e := range up.events {
				if e.Path == relative(deepJava) && e.Kind == "deleted" && e.Removed == 2 {
					deleted = true
				}
				if e.Path == relative(created) {
					t.Errorf("untracked removal retained: %+v", e)
				}
			}
			if !deleted || up.visible[relative(created)] {
				t.Errorf("deep removals: %+v", up)
			}
		})
	}
}

func TestDeepGitSnapshotCLI(t *testing.T) {
	root := deepRepository(t)
	var out bytes.Buffer
	if err := Run([]string{"--snapshot", root}, &out); err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, item := range items {
		found[item.Path] = true
	}
	for _, p := range []string{deepJava, deeperFile, shallowTests} {
		if !found[p] {
			t.Errorf("snapshot omitted %s", p)
		}
	}
}

func TestDeepGitWorkspaceSessions(t *testing.T) {
	roots := []string{deepRepository(t), deepRepository(t)}
	for _, root := range roots {
		monitorPut(t, root, deepJava, "dirty\n")
	}
	sessions, err := loadSessions(filepath.Dir(roots[0]), roots, true)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), "workspace")
	seedSessions(u, sessions)
	for _, s := range sessions {
		if s.scanErr != nil {
			t.Fatal(s.scanErr)
		}
		id := s.prefix + "/" + deepJava
		if u.graph.nodes[id] == nil || u.history.entries[id] == nil {
			t.Errorf("namespaced deep startup absent: %s", id)
		}
		monitorPut(t, s.root, deepJava, "live\n")
		up := s.newMonitor().poll(time.Now())
		if up.err != nil {
			t.Fatal(up.err)
		}
		u.applyScanUpdate(namespaceUpdate(s.prefix, up))
		if up.modifications[deepJava] != 1 {
			t.Errorf("workspace deep live hit missing: %+v", up.modifications)
		}
	}
}
