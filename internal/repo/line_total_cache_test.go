package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLineTotalCachesUnchangedFiles(t *testing.T) {
	root := repository(t)
	for p, text := range map[string]string{"text": "one\ntwo\n", "binary": "\x00binary\n", "invalid": "\xff\n"} {
		put(t, root, p, text)
	}
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if n, known, err := r.LineTotal(); err != nil || known || n != 2 {
		t.Fatalf("first total: %d, %v, %v", n, known, err)
	}
	previous := r.lineTotals
	if len(previous) != 3 {
		t.Fatalf("cached %d files, want 3", len(previous))
	}
	if _, _, err := r.LineTotal(); err != nil {
		t.Fatal(err)
	}
	for p, entry := range previous {
		if entry.info == nil || r.lineTotals[p].info != entry.info {
			t.Errorf("unchanged %s was recounted", p)
		}
	}
}

func TestLineTotalCacheChecksMtimeAndMode(t *testing.T) {
	root := repository(t)
	put(t, root, "text", "one\n")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.LineTotal(); err != nil {
		t.Fatal(err)
	}
	previous := r.lineTotals["text"]
	put(t, root, "text", "\n\n\n\n")
	stamp := previous.info.ModTime().Add(time.Second)
	if err := os.Chtimes(filepath.Join(root, "text"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if n, known, err := r.LineTotal(); err != nil || !known || n != 4 {
		t.Fatalf("mtime change: %d, %v, %v", n, known, err)
	}
	previous = r.lineTotals["text"]
	if err := os.Chmod(filepath.Join(root, "text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.LineTotal(); err != nil {
		t.Fatal(err)
	}
	if r.lineTotals["text"].info == previous.info {
		t.Fatal("mode change reused cached entry")
	}
}

func TestLineTotalCacheRefreshesAndPrunes(t *testing.T) {
	root := repository(t)
	put(t, root, "text", "one\n")
	put(t, root, "hidden", "hidden\n")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		if n, known, err := r.LineTotal(); err != nil || !known || n != want {
			t.Fatalf("total = %d, %v, %v; want %d, true, nil", n, known, err, want)
		}
	}
	check(2)
	previous := r.lineTotals["text"]
	put(t, root, "text", "one\ntwo\n")
	check(3)
	if r.lineTotals["text"].info == previous.info {
		t.Fatal("size change reused cached entry")
	}
	previous = r.lineTotals["text"]
	replacement := filepath.Join(root, "replacement")
	put(t, root, "replacement", "\n\n\n\n\n\n\n\n")
	if err := os.Chtimes(replacement, previous.info.ModTime(), previous.info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filepath.Join(root, "text")); err != nil {
		t.Fatal(err)
	}
	check(9)
	if r.lineTotals["text"].info == previous.info {
		t.Fatal("replacement with identical size and mtime reused cached entry")
	}
	put(t, root, ".gitignore", "hidden\n")
	check(9)
	if _, ok := r.lineTotals["hidden"]; ok {
		t.Fatal("ignored file retained in cache")
	}
	if err := os.Remove(filepath.Join(root, "text")); err != nil {
		t.Fatal(err)
	}
	check(1)
	if _, ok := r.lineTotals["text"]; ok {
		t.Fatal("deleted file retained in cache")
	}
	put(t, root, ".gitignore", "")
	check(1)
	if _, ok := r.lineTotals["hidden"]; !ok {
		t.Fatal("newly allowed file not recounted")
	}
	other, err := Open(repository(t))
	if err != nil {
		t.Fatal(err)
	}
	if n, known, err := other.LineTotal(); err != nil || !known || n != 0 {
		t.Fatalf("other repository: %d, %v, %v", n, known, err)
	}
}

func TestLineTotalKeepsIgnoredHEADFileRemovedFromIndex(t *testing.T) {
	root := repository(t)
	const p = "a/b/c/d/e/f/kept"
	put(t, root, p, "one\ntwo\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "baseline")
	git(t, root, "rm", "--cached", p)
	put(t, root, ".gitignore", "a/\n")
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Scan(0); err != nil {
		t.Fatal(err)
	}
	if n, known, err := r.LineTotal(); err != nil || !known || n != 3 {
		t.Fatalf("kept HEAD file: got %d, %v, %v; want 3, true, nil", n, known, err)
	}
	if err := os.Remove(filepath.Join(root, p)); err != nil {
		t.Fatal(err)
	}
	if n, known, err := r.LineTotal(); err != nil || !known || n != 1 {
		t.Fatalf("missing HEAD file: got %d, %v, %v; want 1, true, nil", n, known, err)
	}
}
