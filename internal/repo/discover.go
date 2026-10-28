package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skipDiscoveryChild filters automatic candidates, never the explicit root.
func skipDiscoveryChild(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "Library", "System", "Applications", "Windows", "node_modules", "vendor", "__pycache__", "venv":
		return true
	}
	return false
}

// Discover finds worktree roots in root and its immediate children only.
// Zero depth inspects root only; positive depths are capped at one for compatibility.
// Symlinks and hidden/reserved children are skipped, but explicit roots are not filtered.
// Root and non-permission Git failures return any roots already found with an error.
func Discover(root string, maxDepth int) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if maxDepth < 0 {
		return nil, fmt.Errorf("discovery depth must not be negative")
	}
	if maxDepth > 1 {
		maxDepth = 1
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
		// Git wraps permission failures in an exit error rather than PathError.
		inaccessibleChild := func(err error) bool {
			message := strings.ToLower(err.Error())
			return depth > 0 && (errors.Is(err, os.ErrPermission) ||
				strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted"))
		}
		if marker, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			if marker.Mode()&os.ModeSymlink == 0 {
				if r, err := Open(dir); err == nil {
					top, err := r.command("rev-parse", "--show-toplevel")
					if err != nil && !inaccessibleChild(err) {
						failures = append(failures, err)
					} else if err == nil && filepath.Clean(strings.TrimSuffix(string(top), "\n")) == filepath.Clean(dir) {
						paths = append(paths, dir)
					}
				} else if !errors.Is(err, ErrNotRepository) && !inaccessibleChild(err) {
					failures = append(failures, err)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) && !inaccessibleChild(err) {
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
			if e.IsDir() && !skipDiscoveryChild(e.Name()) {
				visit(filepath.Join(dir, e.Name()), depth+1)
			}
		}
	}
	visit(abs, 0)
	sort.Strings(paths)
	return paths, errors.Join(failures...)
}
