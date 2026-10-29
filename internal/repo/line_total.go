package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/isaiash/diffplay/internal/watch"
	"golang.org/x/sys/unix"
)

// lineTotalFileLimit bounds per-file reads. Larger files are explicitly unknown.
const lineTotalFileLimit = 64 << 20

type lineTotalEntry struct {
	info  os.FileInfo
	lines int
	known bool
}

type binaryDetector struct{ hasNUL bool }

func (d *binaryDetector) Write(p []byte) (int, error) {
	d.hasNUL = d.hasNUL || bytes.IndexByte(p, 0) >= 0
	return len(p), nil
}

// LineTotal counts current UTF-8 text in Git-allowed and HEAD files at every depth.
// Symlinks and NUL-containing binary assets are excluded. Invalid UTF-8,
// special nodes, nested repositories and >64 MiB files make the result
// incomplete. Missing tracked paths contribute nothing. Files are
// opened relative to directories with O_NOFOLLOW to avoid reading symlink targets.
// Filesystem/Git errors invalidate the result. Like a filesystem scan, this is
// not an atomic snapshot, and concurrent modifications may cause an error.
func (r *Repository) LineTotal() (int, bool, error) {
	paths, err := r.command("ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return 0, false, err
	}
	head, err := r.head()
	if err != nil {
		return 0, false, err
	}
	if head != "" {
		tree, err := r.command("ls-tree", "-r", "--name-only", "-z", head, "--", ".")
		if err != nil {
			return 0, false, err
		}
		paths = append(paths, tree...)
	}
	root, err := unix.Open(r.root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, false, err
	}
	defer unix.Close(root)
	total, known := 0, true
	nextCache := make(map[string]lineTotalEntry)
	seen := make(map[string]bool)
	for _, p := range strings.Split(string(paths), "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if !validPath(p) {
			known = false
			continue
		}
		entry, err := countPath(root, p, r.lineTotals[p])
		if err != nil {
			return 0, false, fmt.Errorf("count %s: %w", p, err)
		}
		if entry.info != nil {
			nextCache[p] = entry
		}
		if !entry.known || entry.lines > math.MaxInt-total {
			known = false
			continue
		}
		total += entry.lines
	}
	r.lineTotals = nextCache
	return total, known, nil
}

func countPath(root int, p string, cached lineTotalEntry) (lineTotalEntry, error) {
	parts := strings.Split(p, "/")
	dir := root
	for _, part := range parts[:len(parts)-1] {
		next, err := unix.Openat(dir, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if dir != root {
			unix.Close(dir)
		}
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP) {
			return lineTotalEntry{known: true}, nil
		}
		if err != nil {
			return lineTotalEntry{}, err
		}
		dir = next
	}
	if dir != root {
		defer unix.Close(dir)
	}
	fd, err := unix.Openat(dir, parts[len(parts)-1], unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) {
		return lineTotalEntry{known: true}, nil
	}
	if err != nil {
		return lineTotalEntry{}, err
	}
	f := os.NewFile(uintptr(fd), p)
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return lineTotalEntry{}, err
	}
	// shortcut: same-inode edits restoring size/mode/mtime evade this cache; use content hashing if needed.
	if cached.info != nil && sameLineTotalFile(cached.info, before) {
		return cached, nil
	}
	entry := lineTotalEntry{info: before}
	if !before.Mode().IsRegular() || before.Size() > lineTotalFileLimit {
		return entry, nil
	}
	detector := &binaryDetector{}
	n, known, err := watch.CountTextLines(io.TeeReader(io.LimitReader(f, lineTotalFileLimit+1), detector))
	if err != nil {
		return lineTotalEntry{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return lineTotalEntry{}, err
	}
	if !sameLineTotalFile(before, after) {
		return lineTotalEntry{}, fmt.Errorf("file changed during count")
	}
	if detector.hasNUL {
		entry.known = true
		return entry, nil
	}
	entry.lines, entry.known = n, known
	return entry, nil
}

func sameLineTotalFile(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
