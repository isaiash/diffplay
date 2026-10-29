package app

import (
	"testing"

	gitrepo "github.com/isaiash/diffplay/internal/repo"
)

func TestRepositoryLineTotalSkipsNonTextAssets(t *testing.T) {
	root := t.TempDir()
	monitorGit(t, root, "init", "-q")
	monitorPut(t, root, "code.txt", "first\nsecond\n")
	monitorPut(t, root, "image.bin", "\x89PNG\x00\n\xff")
	r, err := gitrepo.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	n, known, err := r.LineTotal()
	if err != nil || !known || n != 2 {
		t.Fatalf("text total=%d known=%v err=%v; binary assets have no text lines", n, known, err)
	}
}
