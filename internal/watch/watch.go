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
}
type Snapshot map[string]Entry

// ContentLimit bounds captured text; HashLimit bounds the fully hashed prefix.
// Files larger than HashLimit are sampled (prefix plus final ContentLimit bytes),
// so changes solely in the unsampled middle with unchanged metadata can be missed.
const ContentLimit = 256 * 1024
const HashLimit = 8 * 1024 * 1024

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
	if _, err = io.Copy(h, io.LimitReader(f, HashLimit-int64(len(prefix)))); err != nil {
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
