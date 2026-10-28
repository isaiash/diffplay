package repo

import (
	"path"
	"strings"

	"github.com/isaiash/diffplay/internal/watch"
)

func metadata(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}
func parents(paths []byte) map[string]bool {
	out := map[string]bool{".": true}
	for _, p := range strings.Split(string(paths), "\x00") {
		if p == "" || metadata(p) {
			continue
		}
		for {
			out[p] = true
			if p == "." {
				break
			}
			p = path.Dir(p)
		}
	}
	return out
}

// Scan includes allowed tracked and untracked files and their ancestor directories.
// Git metadata and ignored subtrees are pruned before capturing filesystem contents.
// Empty untracked directories are omitted, as Git does not enumerate them. Depth
// follows watch.Scan (root zero, clamped to 0..5). Errors invalidate the snapshot.
func (r *Repository) Scan(maxDepth int) (watch.Snapshot, error) {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if maxDepth > 5 {
		maxDepth = 5
	}
	r.depth = maxDepth
	out, err := r.command("ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return nil, err
	}
	allowed := parents(out)
	head, err := r.head()
	if err != nil {
		return nil, err
	}
	if head != "" {
		tree, err := r.command("ls-tree", "-r", "--name-only", "-z", head, "--", ".")
		if err != nil {
			return nil, err
		}
		for p := range parents(tree) {
			allowed[p] = true
		}
	}
	return watch.ScanFiltered(r.root, maxDepth, func(p string, _ bool) bool { return allowed[p] && !metadata(p) })
}
