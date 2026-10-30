package app

import (
	"github.com/isaiash/diffplay/internal/watch"
	"strings"
	"testing"
	"time"
)

func TestPagedPreviewLiveFollowAndCommitInvalidation(t *testing.T) {
	root := activityRepo(t)
	monitorPut(t, root, "large", strings.Repeat("old line\n", 30000))
	monitorGit(t, root, "add", "large")
	monitorGit(t, root, "commit", "-qm", "large baseline")
	m := activityMonitor(t, root)
	u := newUI(newGraph(), root)
	monitorPut(t, root, "large", strings.Repeat("new line\n", 30000))
	up := m.poll(time.Now())
	u.applyScanUpdate(up)
	if up.documents["large"] == nil {
		t.Fatal("HEAD document not propagated")
	}
	monitorPut(t, root, "large", strings.Repeat("new line\n", 30000)+"LATEST\n")
	up = m.poll(time.Now())
	u.applyScanUpdate(up)
	if !strings.Contains(u.renderList(120, 30, time.Now()), "LATEST") {
		t.Fatal("auto follow missed latest line after large replacement")
	}
	monitorGit(t, root, "add", "large")
	monitorGit(t, root, "commit", "-qm", "commit without changing worktree")
	up = m.poll(time.Now())
	u.applyScanUpdate(up)
	if len(up.events) != 0 || u.history.entries["large"].document != nil || u.history.entries["large"].diff != "" {
		t.Fatal("HEAD-only change left stale preview")
	}
}

func TestPagedPreviewReachesEnd(t *testing.T) {
	e := watch.Changes(nil, watch.Snapshot{"f": {Content: []byte(strings.Repeat("line\n", 20000) + "LAST\n")}}, time.Time{})[0]
	u := newUI(newGraph(), "repo")
	u.seedEvents([]changeEvent{e})
	u.listSelected = "f"
	u.selected = "f"
	u.scroll = 1000000
	frame := u.renderList(120, 30, time.Time{})
	if !strings.Contains(frame, "LAST") {
		t.Fatal("last page is inaccessible")
	}
	u.scroll = 1000000
	frame = u.renderGraph(120, 30, time.Time{})
	if !strings.Contains(frame, "LAST") {
		t.Fatal("graph does not page")
	}
}
