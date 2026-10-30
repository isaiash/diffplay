// Package repo provides Git-aware, root-scoped filesystem snapshots.
package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/isaiash/diffplay/internal/watch"
)

var ErrNotRepository = errors.New("not a Git working tree")

type Repository struct {
	root            string
	depth           int
	lineTotals      map[string]lineTotalEntry
	baselineHead    string
	baselineTree    map[string]object
	baselineEntries map[object]watch.Entry
	previews        map[string]cachedPreview
}

// Open discovers a working tree while retaining root as the snapshot scope.
// Missing Git, inaccessible paths and other Git failures are not ErrNotRepository.
func Open(root string) (*Repository, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	r := &Repository{root: abs, depth: watch.AllDepth}
	out, err := r.command("rev-parse", "--is-inside-work-tree")
	if err != nil {
		if strings.Contains(err.Error(), "fatal: not a git repository") {
			return nil, fmt.Errorf("%w: %s", ErrNotRepository, abs)
		}
		return nil, err
	}
	if strings.TrimSpace(string(out)) != "true" {
		return nil, ErrNotRepository
	}
	return r, nil
}
func (r *Repository) command(args ...string) ([]byte, error) { return r.inputCommand("", args...) }
func (r *Repository) inputCommand(input string, args ...string) ([]byte, error) {
	base := []string{"--no-pager", "-c", "core.fsmonitor=false", "-C", r.root}
	if len(args) == 0 || args[0] != "check-ignore" {
		base = append(base, "--literal-pathspecs")
	}
	c := exec.Command("git", append(base, args...)...)
	c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stderr strings.Builder
	c.Stderr = &stderr
	c.Stdin = strings.NewReader(input)
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return out, nil
}
