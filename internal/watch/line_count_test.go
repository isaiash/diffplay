package watch

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestScanTruncatedTextLineCounts(t *testing.T) {
	root := t.TempDir()
	// Split UTF-8 at the captured prefix and at a streaming read boundary.
	text := append(bytes.Repeat([]byte{'x'}, ContentLimit-1), []byte("世界\n")...)
	text = append(text, bytes.Repeat([]byte("é\n"), 30000)...)
	text = append(text, []byte("last")...)
	put(t, root, "text", string(text))
	put(t, root, "late-binary", string(append(append([]byte{}, text...), 0)))
	put(t, root, "late-invalid", string(append(append([]byte{}, text...), 0xff)))
	put(t, root, "incomplete", string(append(append([]byte{}, text...), 0xe4)))
	put(t, root, "limit", string(bytes.Repeat([]byte("x\n"), HashLimit/2)))
	put(t, root, "sampled", string(bytes.Repeat([]byte("x\n"), HashLimit/2+1)))
	if err := os.Symlink("text", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	s, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"text": 30002, "limit": HashLimit / 2} {
		e := s[name]
		if !e.LinesKnown || e.Lines != want {
			t.Errorf("%s: lines=%d known=%v, want %d known", name, e.Lines, e.LinesKnown, want)
		}
		if !e.Truncated || len(e.Content) > ContentLimit {
			t.Errorf("%s: unbounded capture", name)
		}
	}
	if s["text"].Hash != sha256.Sum256(text) {
		t.Fatal("line counting changed full file hash")
	}
	for _, name := range []string{"late-binary", "late-invalid", "incomplete", "sampled", "link"} {
		if e := s[name]; e.LinesKnown || e.Lines != 0 {
			t.Errorf("%s must have unknown lines", name)
		}
	}
}

func TestScanTextLineCounts(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"empty": "", "terminated": "one\ntwo\n", "unterminated": "one\ntwo",
		"blank": "\n", "unicode": "olá\n世界", "binary": "a\x00b", "invalid": "a\xffb",
	} {
		put(t, root, name, text)
	}
	s, err := Scan(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"empty": 0, "terminated": 2, "unterminated": 2, "blank": 1, "unicode": 2} {
		if e := s[name]; !e.LinesKnown || e.Lines != want {
			t.Errorf("%s: lines=%d known=%v, want %d known", name, e.Lines, e.LinesKnown, want)
		}
	}
	for _, name := range []string{".", "binary", "invalid"} {
		if e := s[name]; e.LinesKnown || e.Lines != 0 {
			t.Errorf("%s must have unknown lines: %+v", name, e)
		}
	}
}
