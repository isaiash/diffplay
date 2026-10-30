package app

import (
	"strings"
	"testing"
	"time"
)

func TestListFilePercentageUsesCurrentLines(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		added, removed, lines int
		known, stats          bool
		want                  string
	}{
		{"mixed", 3, 2, 40, true, true, "12.5"},
		{"over total", 40, 10, 20, true, true, "250"},
		{"hundred", 100, 0, 100, true, true, "100"},
		{"ten point four", 104, 0, 1000, true, true, "10.4"},
		{"fraction", 23, 0, 10000, true, true, "0.23"},
		{"rounding carry", 99999, 0, 100000, true, true, "100"},
		{"large", 10000, 0, 1, true, true, ">999"},
		{"tiny", 1, 0, 1000000, true, true, "<.01"},
		{"unknown size", 3, 2, 40, false, true, "?"},
		{"unknown edits", 3, 2, 40, true, false, "?"},
		{"zero total", 3, 2, 0, true, true, "?"},
		{"empty", 0, 0, 0, true, true, "0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.recordChange("file", "modified", false, "", tc.added, tc.removed, tc.stats, time.Time{})
			u.graph.upsert("file", ".", false)
			n := u.graph.nodes["file"]
			n.lines, n.linesKnown = tc.lines, tc.known
			lines := strings.Split(plainList(u.renderList(100, 8, time.Now())), "\r\n")
			row := lines[2]
			removedEnd := strings.Index(row, "file")
			if removedEnd < 0 || !strings.Contains(row[:removedEnd], " "+tc.want+" ") {
				t.Fatalf("percentage %q missing before path: %q", tc.want, row)
			}
		})
	}
}

func TestListColumnTitlesAndMouseBaseline(t *testing.T) {
	u := newUI(newGraph(), "/long/path/actual-repo")
	u.recordChange("first", "modified", false, "", 2, 1, true, time.Time{})
	u.recordChange("second", "modified", false, "", 1, 0, true, time.Time{})
	u.width, u.height = 100, 8
	lines := strings.Split(plainList(u.renderList(100, 8, time.Now())), "\r\n")
	titles := lines[1]
	if strings.Index(titles, "actual-repo")-strings.Index(titles, "%") != 5 {
		t.Fatalf("percentage must occupy four cells plus one separator: %q", titles)
	}
	previous := -1
	for _, title := range []string{"HITS", "ADDED", "REMOVED", "%", "actual-repo"} {
		at := strings.Index(titles, title)
		if at <= previous {
			t.Fatalf("missing or overlapping title %q: %q", title, titles)
		}
		previous = at
	}
	if !strings.Contains(lines[2], "first") || u.listVisibleHeight(8) != 5 {
		t.Fatalf("data must start at zero-based row 2 with five rows: %q", lines[2])
	}
	u.listSelected = "second"
	u.listMouse(35, 2, 1, false)
	if u.listSelected != "second" {
		t.Fatal("column titles must not select data")
	}
	u.listMouse(35, 2, 2, false)
	if u.listSelected != "first" {
		t.Fatal("first data row must be selectable")
	}
	u.combined = true
	if u.listVisibleHeight(8) != 6 {
		t.Fatal("combined pane reserves only header and titles")
	}
}

func TestListLargeCountsRemainReadable(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("file", "modified", false, "", 123456, 1234567, true, time.Time{})
	row := strings.Split(plainList(u.renderList(100, 8, time.Now())), "\r\n")[2]
	if !strings.Contains(row, "+123456 ") || !strings.Contains(row, "-1234567 ") {
		t.Fatalf("column labels must not reduce count capacity: %q", row)
	}
}

func TestListPreviewUsesLastRowAboveFooter(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.recordChange("file", "modified", false, "one\ntwo\nthree", 1, 1, true, time.Time{})
	lines := strings.Split(plainList(u.renderList(160, 8, time.Now())), "\r\n")
	if !strings.Contains(lines[6], "three") {
		t.Fatalf("preview lost usable last row: %q", lines[6])
	}
}

func TestListPercentageMetadataAndWorkspaceLabels(t *testing.T) {
	u := newUI(newGraph(), "/workspace")
	u.repositoryLabels = map[string]string{"repo-1": "actual-one", "repo-2": "actual-two"}
	u.recordChange("repo-1/file", "modified", false, "", 3, 2, true, time.Time{})
	u.recordChange("repo-2/other", "modified", false, "", 1, 0, true, time.Time{})
	u.graph.upsert("repo-1/file", "repo-1", false)
	n := u.graph.nodes["repo-1/file"]
	n.lines, n.linesKnown = 40, true
	row := u.history.entries["repo-1/file"]
	if got := u.filePercentage(row); got != "12.5" {
		t.Fatal(got)
	}
	n.lines = 20
	if got := u.filePercentage(row); got != "25.0" {
		t.Fatalf("must follow current file metadata: %s", got)
	}
	for _, r := range u.changeRows() {
		if r.dir && u.filePercentage(r) != "" {
			t.Fatal("folder percentage must remain blank")
		}
	}
	row.kind = "deleted"
	if got := u.filePercentage(row); got != "?" {
		t.Fatalf("deleted file must not use stale line total: %s", got)
	}
	delete(u.graph.nodes, row.id)
	row.kind = "modified"
	if got := u.filePercentage(row); got != "?" {
		t.Fatalf("missing metadata must remain unknown: %s", got)
	}
	frame := plainList(u.renderList(100, 12, time.Now()))
	for _, label := range []string{"actual-one/", "actual-two/"} {
		if !strings.Contains(frame, label) {
			t.Fatalf("lost workspace repository label %q", label)
		}
	}
}
