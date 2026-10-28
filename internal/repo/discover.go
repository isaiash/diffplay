package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover finds worktree roots, including nested repositories, through maxDepth
// (root is depth zero). It never follows symlinks or enters hidden descendants.
// An explicitly supplied root is still inspected even if its name starts with a dot.
// A traversal or Git failure returns any roots already found alongside an error.
func Discover(root string, maxDepth int) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if maxDepth < 0 {
		return nil, fmt.Errorf("discovery depth must not be negative")
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("discovery root is not a directory: %s", abs)
	}
	var paths []string
	var failures []error
	var visit func(string, int)
	visit = func(dir string, depth int) {
		if marker, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			if marker.Mode()&os.ModeSymlink == 0 {
				if r, err := Open(dir); err == nil {
					top, err := r.command("rev-parse", "--show-toplevel")
					if err != nil {
						failures = append(failures, err)
					} else if filepath.Clean(strings.TrimSuffix(string(top), "\n")) == filepath.Clean(dir) {
						paths = append(paths, dir)
					}
				} else if !errors.Is(err, ErrNotRepository) {
					failures = append(failures, err)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			failures = append(failures, err)
		}
		if depth >= maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			failures = append(failures, fmt.Errorf("discover %s: %w", dir, err))
			return
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				visit(filepath.Join(dir, e.Name()), depth+1)
			}
		}
	}
	visit(abs, 0)
	sort.Strings(paths)
	return paths, errors.Join(failures...)
}
