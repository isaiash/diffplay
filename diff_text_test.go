package main

import (
	"strings"
	"testing"
	"time"
)

func TestDiffTabsExpandWithoutReplacingUnicodeOrAcceptingEscapes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"+\tif 日本語", "+       if 日本語"},
		{" \t\tx", "                x"},
		{"+日\tx", "+日     x"},
		{"+e\u0301	x", "+e\u0301      x"},
		{"+Windows line\r", "+Windows line"},
	} {
		if got := diffText(tc.input); got != tc.want {
			t.Errorf("diffText(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
	if got := diffText("+\ttext\x1b[2J"); strings.ContainsAny(got, "\x1b\t") {
		t.Fatalf("unsafe diff text: %q", got)
	}
}

func TestEveryDiffPreviewExpandsTabs(t *testing.T) {
	for _, mode := range []string{"list", "combined", "graph"} {
		u := newUI(newGraph(), "/repo")
		u.listMode, u.combined = mode == "list", mode == "combined"
		u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "file", Kind: "modified", Diff: "+	日本語 e\u0301\r", StatsKnown: true}}})
		frame := plainList(u.render(160, 40, time.Now().Add(time.Second)))
		if !strings.Contains(frame, "+       日本語 e\u0301") || strings.Contains(frame, "�") {
			t.Fatalf("%s preview lost tab/Unicode: %q", mode, frame)
		}
	}
}

func TestNativeSelectionModeReleasesMouseAndFreezesOnlyDrawing(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.combined = true
	u.render(120, 40, time.Now())
	u.input([]byte("m"))
	frame := u.render(120, 40, time.Now())
	if !strings.Contains(frame, "\x1b[?1003l") || !strings.Contains(frame, "\x1b[?1006l") || !strings.Contains(frame, "copy") {
		t.Fatal("native selection must release mouse and explain copying")
	}
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "live", Kind: "created", Diff: "+live", StatsKnown: true}}})
	if u.history.entries["live"] == nil {
		t.Fatal("selection mode stopped monitoring")
	}
	if frame := u.render(160, 50, time.Now()); frame != "" {
		t.Fatal("redraw would destroy native terminal selection")
	}
	u.input([]byte("m"))
	frame = u.render(160, 50, time.Now())
	if !strings.Contains(frame, "\x1b[?1003h") || !strings.Contains(frame, "\x1b[?1006h") || !strings.Contains(frame, "live") {
		t.Fatal("leaving selection must restore mouse and latest changes")
	}
}

func TestLivePreviewRevealsNewestLineInsideLongHEADHunk(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.listMode = true
	u.render(160, 14, time.Now())
	head := "--- a/file\n+++ b/file\n@@ -1,1 +1,42 @@\n baseline\n" + strings.Repeat("+earlier\n", 40) + "+latest\n"
	burst := "--- a/file\n+++ b/file\n@@ -38,4 +38,5 @@\n earlier\n earlier\n earlier\n earlier\n+latest\n"
	u.applyScanUpdate(scanUpdate{events: []changeEvent{{Path: "file", Kind: "modified", Diff: burst, Added: 1, StatsKnown: true}}, previews: map[string]string{"file": head}})
	frame := plainList(u.render(160, 14, time.Now()))
	if !strings.Contains(frame, "+latest") || u.history.entries["file"].diff != head {
		t.Fatal("new burst must reveal its line without replacing the HEAD-relative diff")
	}
}
