package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDiscoveryHelpDescribesImmediateChildren(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--help"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "immediate children") || strings.Contains(out.String(), "within 5 levels") {
		t.Fatalf("outdated discovery help: %s", out.String())
	}
}

func TestDiscoveryDoesNotOfferGrandchildRepository(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "one", "two")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	initTestRepository(t, nested)
	err := run([]string{"--snapshot", root}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "immediate children") {
		t.Fatalf("missing nonrecursive guidance: %v", err)
	}
}

func TestNoRepositoryProvidesEnglishGuidance(t *testing.T) {
	root := t.TempDir()
	err := run([]string{"--snapshot", root}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "open the application inside a Git repository") {
		t.Fatalf("missing repository guidance: %v", err)
	}
}
