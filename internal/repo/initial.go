package repo

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isaiash/diffplay/internal/watch"
)

type object struct{ mode, oid string }
type cachedPreview struct {
	head  string
	entry watch.Entry
	event watch.Event
}

// HeadChanges provides live previews without changing interval event statistics.
func (r *Repository) HeadChanges(current watch.Snapshot, now time.Time) ([]watch.Event, error) {
	return r.InitialChanges(current, now)
}

func (r *Repository) tree(head string) (map[string]object, error) {
	if r.baselineTree != nil && r.baselineHead == head {
		return r.baselineTree, nil
	}
	out := map[string]object{}
	if head == "" {
		r.cacheBaseline(head, out)
		return out, nil
	}
	raw, err := r.command("ls-tree", "-r", "-z", head, "--", ".")
	if err != nil {
		return nil, err
	}
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		meta, p, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid Git tree record")
		}
		out[p] = object{fields[0], fields[2]}
	}
	r.cacheBaseline(head, out)
	return out, nil
}

func (r *Repository) cacheBaseline(head string, tree map[string]object) {
	for _, e := range r.baselineEntries {
		if e.Source != nil {
			e.Source.Close()
		}
	}
	r.baselineHead, r.baselineTree = head, tree
	r.baselineEntries = make(map[object]watch.Entry)
}

func (r *Repository) entry(p string, o object) (watch.Entry, error) {
	if e, ok := r.baselineEntries[o]; ok {
		e.Path = p
		return e, nil
	}
	e := watch.Entry{Path: p}
	if o.mode == "160000" {
		e.IsDir = true
		return e, nil
	}
	size, err := r.command("cat-file", "-s", o.oid)
	if err != nil {
		return e, err
	}
	e.Size, err = strconv.ParseInt(strings.TrimSpace(string(size)), 10, 64)
	if err != nil {
		return e, err
	}
	e.Truncated = e.Size > watch.ContentLimit
	c := exec.Command("git", "--no-pager", "-C", r.root, "cat-file", "blob", o.oid)
	c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stderr strings.Builder
	c.Stderr = &stderr
	pipe, err := c.StdoutPipe()
	if err != nil {
		return e, err
	}
	if err = c.Start(); err != nil {
		return e, err
	}
	data := make([]byte, min(e.Size, int64(watch.ContentLimit)))
	_, readErr := io.ReadFull(pipe, data)
	if e.Truncated {
		if e.Size <= watch.HashLimit && o.mode != "120000" && e.Content == nil && !bytes.ContainsRune(data, 0) {
			e.Source, readErr = watch.NewDocument(io.MultiReader(bytes.NewReader(data), pipe))
			if e.Source != nil && !e.Source.TextKnown {
				e.Source.Close()
				e.Source = nil
			}
		} else {
			_ = c.Process.Kill()
		}
	}
	waitErr := c.Wait()
	if readErr != nil {
		return e, fmt.Errorf("read Git blob %s: %w", o.oid, readErr)
	}
	if waitErr != nil && (!e.Truncated || e.Source != nil) {
		return e, fmt.Errorf("read Git blob: %w: %s", waitErr, stderr.String())
	}
	e.Hash = sha256.Sum256(data)
	if o.mode != "120000" && !bytes.ContainsRune(data, 0) {
		text := data
		if e.Truncated {
			for n := 0; n < 3 && len(text) > 0 && !utf8.Valid(text); n++ {
				text = text[:len(text)-1]
			}
		}
		if utf8.Valid(text) {
			e.Content = append([]byte{}, text...)
		}
	}
	// HEAD blobs are immutable; invalidate the bounded cache when HEAD changes.
	if r.baselineEntries != nil {
		r.baselineEntries[o] = e
	}
	return e, nil
}

// InitialChanges seeds Git's staged/unstaged/untracked status, comparing each
// path exactly once against HEAD, not summing index and worktree diffs. Renames
// are deletion/creation pairs. Unborn HEAD uses an empty baseline. Metadata-only
// and index-only status entries remain visible with zero known text edits.
// Deleted paths obey the most recent Scan depth (unlimited before the first Scan).
// Changed binary/symlink content and truncated blobs have unknown line statistics;
// identical fully hashed content can still prove a metadata-only zero.
func (r *Repository) InitialChanges(current watch.Snapshot, now time.Time) ([]watch.Event, error) {
	raw, err := r.command("status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames", "--ignore-submodules=none", "--", ".")
	if err != nil {
		return nil, err
	}
	prefix, err := r.command("rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	scope := strings.TrimSuffix(string(prefix), "\n")
	candidates := map[string]bool{}
	for _, record := range strings.Split(string(raw), "\x00") {
		if record == "" {
			continue
		}
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("invalid Git status record")
		}
		p := record[3:]
		if !strings.HasPrefix(p, scope) {
			continue
		}
		p = strings.TrimPrefix(p, scope)
		if validPath(p) && (r.depth == watch.AllDepth || strings.Count(p, "/")+1 <= r.depth) {
			candidates[p] = true
		}
	}
	head, err := r.head()
	if err != nil {
		return nil, err
	}
	tree, err := r.tree(head)
	if err != nil {
		return nil, err
	}
	before, after := watch.Snapshot{}, watch.Snapshot{}
	for p := range candidates {
		if o, ok := tree[p]; ok {
			e, err := r.entry(p, o)
			if err != nil {
				return nil, err
			}
			before[p] = e
		}
		if e, ok := current[p]; ok {
			after[p] = e
		}
	}
	var events []watch.Event
	cache := make(map[string]cachedPreview, len(candidates))
	for p := range candidates {
		b, bok := after[p]
		if saved, ok := r.previews[p]; ok && saved.head == head && (saved.event.Document == nil || saved.event.Document.Available()) && saved.entry.Hash == b.Hash && saved.entry.Size == b.Size && saved.entry.ModTime.Equal(b.ModTime) {
			e := saved.event
			e.At = now
			events = append(events, e)
			cache[p] = saved
			continue
		}
		aa, bb := watch.Snapshot{}, watch.Snapshot{}
		if a, ok := before[p]; ok {
			aa[p] = a
		}
		if bok {
			bb[p] = b
		}
		found := watch.Changes(aa, bb, now)
		events = append(events, found...)
		if len(found) > 0 {
			cache[p] = cachedPreview{head, b, found[0]}
		}
	}
	r.previews = cache
	seen := map[string]bool{}
	for i := range events {
		e := &events[i]
		a, aok := before[e.Path]
		b, bok := after[e.Path]
		if aok && bok && !a.Truncated && !b.Truncated && a.Size == b.Size && a.Hash == b.Hash {
			e.StatsKnown = true
			e.Added = 0
			e.Removed = 0
			e.Diff = "Git index or metadata changed (no net content edits)\n"
		}
		if !e.StatsKnown && head != "" && aok && bok && !a.IsDir && !b.IsDir {
			raw, err := r.command("diff", "--raw", "-z", "--no-abbrev", "--no-ext-diff", "--no-textconv", "--no-renames", head, "--", e.Path)
			if err != nil {
				return nil, err
			}
			header, _, _ := strings.Cut(string(raw), "\x00")
			fields := strings.Fields(header)
			if len(raw) == 0 || len(fields) == 5 && fields[2] == fields[3] {
				e.StatsKnown = true
				e.Added, e.Removed = 0, 0
				e.Diff = "Git index or metadata changed (no net content edits)\n"
			}
		}
		seen[e.Path] = true
	}
	for p := range candidates {
		if seen[p] {
			continue
		}
		a, aok := before[p]
		b, bok := after[p]
		known := !aok && !bok || aok && bok && !a.Truncated && !b.Truncated && a.Content != nil && b.Content != nil && bytes.Equal(a.Content, b.Content)
		events = append(events, watch.Event{Path: p, Kind: "modified", IsDir: b.IsDir, At: now, StatsKnown: known, Diff: "Git index or metadata changed (no net captured text edits)\n"})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Path < events[j].Path })
	return events, nil
}
