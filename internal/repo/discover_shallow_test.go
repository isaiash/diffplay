package repo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverSkipsReservedChildrenButAcceptsExplicitRoots(t *testing.T) {
	root := t.TempDir()
	names := []string{"Library", "System", "Applications", "Windows", "node_modules", "vendor", "__pycache__", "venv"}
	for _, name := range names {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "init", "-q")
		paths, err := Discover(dir, 5)
		if err != nil || !reflect.DeepEqual(paths, []string{dir}) {
			t.Fatalf("explicit %s: %q %v", name, paths, err)
		}
	}
	visible := filepath.Join(root, "project")
	if err := os.Mkdir(visible, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, visible, "init", "-q")
	paths, err := Discover(root, 5)
	if err != nil || !reflect.DeepEqual(paths, []string{visible}) {
		t.Fatalf("reserved children: %q %v", paths, err)
	}
}

func TestDiscoverSkipsInaccessibleChild(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod permission denial is not enforced for root")
	}
	root := repository(t)
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, blocked, "init", "-q")
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0755) })
	if _, err := os.Stat(filepath.Join(blocked, ".git")); !os.IsPermission(err) {
		t.Fatalf("fixture must actually deny access: %v", err)
	}
	got, err := Discover(root, 5)
	if err != nil || !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("inaccessible child: %q %v", got, err)
	}
}

func TestDiscoverSkipsUnreadableChildGitConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod permission denial is not enforced for root")
	}
	root := repository(t)
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, child, "init", "-q")
	config := filepath.Join(child, ".git", "config")
	if err := os.Chmod(config, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(config, 0644) })
	if _, err := os.ReadFile(config); !os.IsPermission(err) {
		t.Fatalf("fixture must actually deny access: %v", err)
	}
	got, err := Discover(root, 5)
	if err != nil || !reflect.DeepEqual(got, []string{root}) {
		t.Fatalf("unreadable child Git config: %q %v", got, err)
	}
}

func TestDiscoverZeroAndPositiveDepths(t *testing.T) {
	root := repository(t)
	child := filepath.Join(root, "child")
	grandchild := filepath.Join(child, "grandchild")
	if err := os.MkdirAll(grandchild, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, child, "init", "-q")
	git(t, grandchild, "init", "-q")
	for _, depth := range []int{0, 1, 2, 5, 100} {
		want := []string{root}
		if depth > 0 {
			want = append(want, child)
		}
		got, err := Discover(root, depth)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("depth %d: %q %v; want %q", depth, got, err, want)
		}
	}
}
