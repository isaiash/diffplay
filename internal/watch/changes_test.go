package watch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUnifiedDiffMergesNearbyChanges(t *testing.T) {
	a := "1\nold2\n3\n4\n5\n6\nold7\n8\n9\n10\n11\n12\n13\n14\n15\n16\nold17\n18\n19\n20\n"
	b := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(a, "old2", "new2"), "old7", "new7"), "old17", "new17")
	events := Changes(Snapshot{"f": {Content: []byte(a)}}, Snapshot{"f": {Content: []byte(b)}}, time.Time{})
	if len(events) != 1 {
		t.Fatal(events)
	}
	got := events[0].Diff
	if strings.Count(got, "@@ -") != 2 || !strings.Contains(got, "@@ -1,10 +1,10 @@") || !strings.Contains(got, "@@ -14,7 +14,7 @@") {
		t.Fatalf("overlapping or incorrect context hunks:\n%s", got)
	}
}

func TestUnifiedDiffContextAndBounds(t *testing.T) {
	now := time.Unix(1, 0)
	event := func(a, b Entry) string {
		t.Helper()
		e := Changes(Snapshot{"note": a}, Snapshot{"note": b}, now)
		if len(e) != 1 {
			t.Fatalf("events: %v", e)
		}
		return e[0].Diff
	}
	got := event(Entry{Content: []byte("one\ntwo\nold\nfour\nfive\n")}, Entry{Content: []byte("one\ntwo\nnew\nfour\nfive\n")})
	want := "--- a/note\n+++ b/note\n@@ -1,5 +1,5 @@\n one\n two\n-old\n+new\n four\n five\n"
	if got != want {
		t.Fatalf("diff:\n%s\nwant:\n%s", got, want)
	}
	got = event(Entry{Size: 2, Hash: [32]byte{1}}, Entry{Size: 2, Hash: [32]byte{2}})
	if !strings.Contains(got, "Binary") {
		t.Fatalf("missing binary marker: %q", got)
	}
	got = event(Entry{Content: []byte(strings.Repeat("old\n", 10000))}, Entry{Content: []byte(strings.Repeat("new\n", 10000))})
	if len(got) > 65536 || !strings.Contains(got, "omitted") {
		t.Fatalf("unbounded diff length %d", len(got))
	}
	got = event(Entry{Content: []byte("old\n"), Truncated: true}, Entry{Content: []byte("new\n"), Truncated: true})
	if !strings.Contains(got, "truncated") {
		t.Fatalf("missing truncation marker %q", got)
	}
	got = event(Entry{Content: []byte("old")}, Entry{Content: []byte("new")})
	if !strings.Contains(got, "\\ No newline at end of file") {
		t.Fatalf("missing newline marker %q", got)
	}
}

func TestChangesFilesystemLifecycle(t *testing.T) {
	root := t.TempDir()
	put(t, root, "edit", "old\n")
	put(t, root, "gone", "bye\n")
	put(t, root, "rename", "moved\n")
	put(t, root, "kept/child", "same\n")
	before, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, "edit", "new\n")
	if err = os.Chtimes(filepath.Join(root, "edit"), before["edit"].ModTime, before["edit"].ModTime); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, "gone"))
	os.Rename(filepath.Join(root, "rename"), filepath.Join(root, "renamed"))
	put(t, root, "newdir/new", "added\n")
	after, err := Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(123, 0)
	events := Changes(before, after, now)
	var got []string
	for _, e := range events {
		got = append(got, e.Path+":"+e.Kind)
		if !e.At.Equal(now) {
			t.Fatal("timestamp not preserved")
		}
		if e.Path == "newdir" && !e.IsDir {
			t.Fatal("missing directory flag")
		}
	}
	want := []string{"edit:modified", "gone:deleted", "newdir:created", "newdir/new:created", "rename:deleted", "renamed:created"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events %v want %v", got, want)
	}
	for _, pair := range []struct{ path, fragment string }{{"edit", "-old\n+new\n"}, {"gone", "-bye\n"}, {"newdir/new", "+added\n"}} {
		for _, e := range events {
			if e.Path == pair.path && !strings.Contains(e.Diff, pair.fragment) {
				t.Fatalf("diff %q missing %q", e.Diff, pair.fragment)
			}
		}
	}
	if got := Changes(after, after, now); len(got) != 0 {
		t.Fatalf("unchanged events: %v", got)
	}
	if got := Changes(Snapshot{"dir": {IsDir: true}}, Snapshot{}, now); len(got) != 1 || !got[0].IsDir || got[0].Kind != "deleted" {
		t.Fatalf("directory delete %v", got)
	}
}
