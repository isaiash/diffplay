package repo

import (
	"errors"
	"os/exec"
	"path"
	"strings"
)

func exitCode(err error, code int) bool {
	var e *exec.ExitError
	return errors.As(err, &e) && e.ExitCode() == code
}
func (r *Repository) head() (string, error) {
	out, err := r.command("rev-parse", "--verify", "--quiet", "HEAD")
	if exitCode(err, 1) {
		return "", nil
	}
	return strings.TrimSuffix(string(out), "\n"), err
}
func validPath(p string) bool {
	return p != "" && !strings.ContainsRune(p, 0) && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !metadata(p)
}

// TrackedPaths returns the union of index and HEAD paths and their ancestors.
func (r *Repository) TrackedPaths() (map[string]bool, error) {
	cached, err := r.command("ls-files", "--cached", "-z", "--", ".")
	if err != nil {
		return nil, err
	}
	tracked := parents(cached)
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
			tracked[p] = true
		}
	}
	return tracked, nil
}

// FilterPaths refreshes Git ignore rules and returns root-relative allowances.
// Missing paths remain allowed unless ignored, so real deletions survive. Paths
// tracked in the index or HEAD (including staged deletions) override ignores.
// Metadata and out-of-scope paths are false. This is independent of scan depth.
// On error callers must retain their previous state, not assume paths are hidden.
func (r *Repository) FilterPaths(paths []string) (map[string]bool, error) {
	tracked, err := r.TrackedPaths()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(paths))
	var input strings.Builder
	for _, p := range paths {
		out[p] = validPath(p)
		if out[p] && !tracked[p] {
			input.WriteString(p)
			input.WriteByte(0)
		}
	}
	if input.Len() == 0 {
		return out, nil
	}
	ignored, err := r.inputCommand(input.String(), "check-ignore", "--no-index", "-z", "--stdin")
	if err != nil && !exitCode(err, 1) {
		return nil, err
	}
	for _, p := range strings.Split(string(ignored), "\x00") {
		if p != "" {
			out[p] = false
		}
	}
	return out, nil
}
