package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestTerminalThemePaletteIndices(t *testing.T) {
	for index := 0; index < 16; index++ {
		color := rgb{index + 1, -1, 0}
		code := 30 + index
		if index >= 8 {
			code = 90 + index - 8
		}
		if got, want := color.sgr(false), fmt.Sprintf("\x1b[22;%dm", code); got != want {
			t.Errorf("foreground %d = %q, want %q", index, got, want)
		}
		if got, want := color.sgr(true), fmt.Sprintf("\x1b[%dm", code+10); got != want {
			t.Errorf("background %d = %q, want %q", index, got, want)
		}
	}
	if (rgb{}).sgr(false) != "\x1b[22;39m" || (rgb{}).sgr(true) != "\x1b[49m" {
		t.Fatal("unset colors must retain terminal defaults")
	}
	if (rgb{38, 40, 49}).sgr(true) != "\x1b[49m" {
		t.Fatal("legacy RGB must fall back to terminal default")
	}
}

func TestTerminalThemeFadeDisappearsIntoDefaultBackground(t *testing.T) {
	c := newCanvas(2, 1)
	c.put(0, 0, '⠁', background)
	c.put(1, 0, 'x', red)
	got := c.ansi()
	if !strings.Contains(got, "\x1b[8m") || !strings.Contains(got, "\x1b[28m") || strings.Contains(got, "\x1b[7m") {
		t.Fatalf("faded glyph must conceal without inverting its gaps: %q", got)
	}
}

func TestTerminalThemeBackgroundAsInk(t *testing.T) {
	c := newCanvas(2, 1)
	c.cells[0] = cell{ch: '▀', color: background, bg: red}
	c.put(1, 0, 'x', blue)
	got := c.ansi()
	for _, want := range []string{"\x1b[7m", "\x1b[22;31m▀", "\x1b[27m", "\x1b[22;34mx"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing default-background half-cell SGR %q in %q", want, got)
		}
	}
}

func TestTerminalThemeFadeAndFlash(t *testing.T) {
	if got := blend(red, background, .5); got == red || got == background || got.g != -1 || got.b != 1 {
		t.Fatalf("fade needs dim theme red, got %v", got)
	}
	if got := blend(red, background, 1); got != background {
		t.Fatalf("fade endpoint=%v", got)
	}
	if got := blend(red, white, .45); got == red || got == white || got.g != -1 {
		t.Fatalf("flash needs bright theme red, got %v", got)
	}
	if got := blend(background, red, 0); got != background {
		t.Fatalf("zero flash=%v", got)
	}
	c := newCanvas(3, 1)
	c.put(0, 0, 'a', blend(red, background, .5))
	c.put(1, 0, 'b', red)
	c.put(2, 0, 'c', blend(red, white, .45))
	got := c.ansi()
	for _, want := range []string{"\x1b[2;31ma", "\x1b[22;31mb", "\x1b[22;91mc"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing fade SGR %q in %q", want, got)
		}
	}
}

func TestTerminalThemeCanvasUsesDefaultsAndPalette(t *testing.T) {
	c := newCanvas(4, 1)
	c.put(0, 0, 'a', red)
	c.cells[0].bg = blue
	c.put(1, 0, 'b', rgb{})
	c.put(2, 0, 'c', white)
	c.put(3, 0, 'd', gray)
	got := c.ansi()
	for _, want := range []string{"\x1b[49m", "\x1b[44m", "\x1b[22;31m", "\x1b[22;39m", "\x1b[22;97m", "\x1b[22;39md"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing theme SGR %q in %q", want, got)
		}
	}
	if strings.Contains(got, ";2;") || strings.Contains(got, ";5;") {
		t.Fatalf("non-theme color in %q", got)
	}
	if !strings.Contains(got, "a\x1b[49m\x1b[22;39mb") {
		t.Fatalf("palette colors leaked into default cell: %q", got)
	}
}
