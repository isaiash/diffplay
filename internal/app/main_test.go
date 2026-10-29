package app

import (
	"bytes"
	"github.com/isaiash/diffplay/internal/watch"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mapEntries(paths ...string) watch.Snapshot {
	s := make(watch.Snapshot)
	for _, p := range paths {
		s[p] = watch.Entry{Path: p, IsDir: !strings.Contains(p, ".") || p == "."}
	}
	return s
}

func TestCLIUsesDiffplayName(t *testing.T) {
	for _, tc := range []struct {
		flag string
		want string
	}{
		{"--help", "Diffplay\n\nUsage: diffplay [options] [directory]"},
		{"--version", "Diffplay " + version + "\n"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run([]string{tc.flag}, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("expected %q in output, got %q", tc.want, out.String())
			}
			if strings.Contains(strings.ToLower(out.String()), "hermes-graph") {
				t.Fatal("output contains the previous application name")
			}
		})
	}
}

func TestCLIHelpAndSnapshotWithoutTerminal(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"--help"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Diffplay") {
		t.Fatal("missing help")
	}
	root := t.TempDir()
	initTestRepository(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"--snapshot", root}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a.txt") {
		t.Fatal("snapshot missing file")
	}
	if err := Run([]string{"--view", "invalid", root}, &out); err == nil || !strings.Contains(err.Error(), "--view") {
		t.Fatal("invalid view must produce an actionable validation error")
	}
	if err := Run([]string{"--view", "list", "--snapshot", root}, &out); err != nil {
		t.Fatal("list option must coexist with headless snapshot:", err)
	}
	if err := Run([]string{"--interval", "0s", root}, &out); err == nil {
		t.Fatal("zero polling interval accepted")
	}
}

func TestSyncRetainsDeletedNodesAndAddsParents(t *testing.T) {
	g := newGraph()
	now := time.Now()
	applySnapshot(g, mapEntries(".", "src", "src/a.txt"), []changeEvent{{Path: "src/a.txt", Kind: "modified", At: now}}, now)
	if len(g.nodes) != 2 || g.nodes["src/a.txt"].parent != "src" {
		t.Fatal("missing directory grouping")
	}
	applySnapshot(g, mapEntries("."), []changeEvent{{Path: "src/a.txt", Kind: "deleted", Diff: "-hello", At: now}, {Path: "src", Kind: "deleted", IsDir: true, At: now}}, now)
	if g.nodes["src/a.txt"].kind != "deleted" {
		t.Fatal("deleted node not retained")
	}
}
