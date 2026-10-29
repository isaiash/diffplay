// Package watch captures bounded filesystem snapshots and compares them.
package watch

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Entry is a single filesystem node. Content is captured UTF-8 text (nil for
// binary or non-regular nodes). Hash is SHA-256 for regular files, sampled only
// above HashLimit; symlinks hash their own link text, never target contents.
// Truncated means the file exceeded ContentLimit, regardless of its encoding.
type Entry struct {
	Path      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
	Content   []byte
	Hash      [32]byte
	Truncated bool
	// Lines counts newlines plus an unterminated final line (empty text is zero).
	// LinesKnown requires a complete regular UTF-8 file without NUL bytes, read
	// within HashLimit. Larger sampled files and non-text nodes remain unknown;
	// a truncated Content prefix alone never supplies a full-file line count.
	Lines      int
	LinesKnown bool
}
type Snapshot map[string]Entry

// ContentLimit bounds captured text; HashLimit bounds the fully hashed prefix.
// Files larger than HashLimit are sampled (prefix plus final ContentLimit bytes),
// so changes solely in the unsampled middle with unchanged metadata can be missed.
const ContentLimit = 256 * 1024
const HashLimit = 8 * 1024 * 1024

// textCounter validates UTF-8 across read boundaries without retaining file data.
// At most one incomplete rune is carried to the next Write.
type textCounter struct {
	lines   int
	bytes   int64
	last    byte
	invalid bool
	pending []byte
}

func (c *textCounter) Write(p []byte) (int, error) {
	n := len(p)
	c.bytes += int64(n)
	c.lines += bytes.Count(p, []byte{'\n'})
	if n > 0 {
		c.last = p[n-1]
	}
	if bytes.IndexByte(p, 0) >= 0 {
		c.invalid = true
	}
	if c.invalid {
		return n, nil
	}
	if len(c.pending) > 0 {
		for len(p) > 0 && !utf8.FullRune(c.pending) {
			c.pending = append(c.pending, p[0])
			p = p[1:]
		}
		if !utf8.FullRune(c.pending) {
			return n, nil
		}
		r, size := utf8.DecodeRune(c.pending)
		if r == utf8.RuneError && size == 1 {
			c.invalid = true
			return n, nil
		}
		c.pending = c.pending[:0]
	}
	for len(p) > 0 {
		if p[0] < utf8.RuneSelf {
			p = p[1:]
			continue
		}
		if !utf8.FullRune(p) {
			c.pending = append(c.pending[:0], p...)
			break
		}
		r, size := utf8.DecodeRune(p)
		if r == utf8.RuneError && size == 1 {
			c.invalid = true
			break
		}
		p = p[size:]
	}
	return n, nil
}

func capture(path string, e *Entry) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	prefix, err := io.ReadAll(io.LimitReader(f, ContentLimit))
	if err != nil {
		return err
	}
	h.Write(prefix)
	counter := &textCounter{}
	_, _ = counter.Write(prefix)
	if _, err = io.Copy(io.MultiWriter(h, counter), io.LimitReader(f, HashLimit-int64(len(prefix)))); err != nil {
		return err
	}
	if e.Size > HashLimit {
		if _, err = f.Seek(-ContentLimit, io.SeekEnd); err != nil {
			return err
		}
		if _, err = io.Copy(h, io.LimitReader(f, ContentLimit)); err != nil {
			return err
		}
	}
	copy(e.Hash[:], h.Sum(nil))
	e.Truncated = e.Size > ContentLimit
	// A text prefix may end in the middle of a UTF-8 encoding.
	text := prefix
	if e.Truncated {
		for n := 0; n < 3 && len(text) > 0 && !utf8.Valid(text); n++ {
			text = text[:len(text)-1]
		}
	}
	if !bytes.ContainsRune(prefix, 0) && utf8.Valid(text) {
		e.Content = append([]byte{}, text...)
	}
	// Count only fully read and validated text. Sampled files deliberately stay
	// unknown: neither prefix counts nor byte-size estimates describe their lines.
	if e.Size <= HashLimit && counter.bytes == e.Size && !counter.invalid && len(counter.pending) == 0 {
		e.LinesKnown = true
		e.Lines = counter.lines
		if counter.bytes > 0 && counter.last != '\n' {
			e.Lines++
		}
	}
	return nil
}

// Scan includes root (depth zero) and descendants up to maxDepth, capped at five.
// Negative depths mean root only. Paths are relative and slash-separated,
// including hidden nodes. Symlinks and special nodes are not traversed or read.
// On the first filesystem error Scan returns the partial snapshot and error;
// callers must not treat that partial result as a complete set of deletions.
// Like filesystem walking generally, the scan is not an atomic transaction.
func Scan(root string, maxDepth int) (Snapshot, error) {
	return ScanFiltered(root, maxDepth, nil)
}

// ScanFiltered applies allow to slash-separated root-relative paths before
// reading metadata or contents. A rejected directory is pruned without reading
// its children. Root is always included; nil allows every path.
func ScanFiltered(root string, maxDepth int, allow func(path string, isDir bool) bool) (Snapshot, error) {
	if maxDepth < 0 {
		maxDepth = 0
	}
	if maxDepth > 5 {
		maxDepth = 5
	}
	out := Snapshot{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, "/") + 1
		}
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel != "." && allow != nil && !allow(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := Entry{Path: rel, IsDir: d.IsDir(), Size: info.Size(), ModTime: info.ModTime()}
		if info.Mode().IsRegular() {
			if err = capture(path, &e); err != nil {
				return err
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			e.Hash = sha256.Sum256([]byte(target))
		}
		out[rel] = e
		if d.IsDir() && depth == maxDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return out, err
}
