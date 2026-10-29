package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isaiash/diffplay/internal/watch"
)

func TestUnchangedCircleAreaFollowsFileLineCount(t *testing.T) {
	root := t.TempDir()
	for name, lines := range map[string]int{"small": 10, "large": 40} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(strings.Repeat("line\n", lines)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := watch.Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	applySnapshot(u.graph, s, nil, time.Now())
	u.updateGraphMetrics()
	small, large := u.graph.nodes["small"], u.graph.nodes["large"]
	ratio := large.radius * large.radius / (small.radius * small.radius)
	if math.Abs(ratio-4) > 1e-9 {
		t.Fatalf("40-line/10-line circle area ratio=%g, want 4", ratio)
	}
	if len(u.history.entries) != 0 {
		t.Fatal("context sizes must not enter change totals")
	}
}

func TestContextSizeNormalizationIsRepositoryLocal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("a\nb\nc\nd\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := watch.Scan(root, 5)
	if err != nil {
		t.Fatal(err)
	}
	u := newUI(newGraph(), root)
	u.repositoryLabels = map[string]string{"repo-1": "a", "repo-2": "b"}
	for _, prefix := range []string{"repo-1", "repo-2"} {
		applySnapshot(u.graph, namespaceUpdate(prefix, scanUpdate{snapshot: s}).snapshot, nil, time.Now())
	}
	u.updateGraphMetrics()
	a, b := u.graph.nodes["repo-1/file"], u.graph.nodes["repo-2/file"]
	if a.radius != b.radius || a.radius <= 0 {
		t.Fatal("equal files in separate repos must retain their local scale")
	}
}
