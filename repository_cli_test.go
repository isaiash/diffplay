package main

import (
	"io"
	"os/exec"
	"strings"
	"testing"
)

func initTestRepository(t *testing.T, root string) {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

func TestNoRepositoryProvidesEnglishGuidance(t *testing.T) {
	root := t.TempDir()
	err := run([]string{"--snapshot", root}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "open the application inside a Git repository") {
		t.Fatalf("missing repository guidance: %v", err)
	}
}
