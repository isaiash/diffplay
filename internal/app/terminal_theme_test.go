package app

import (
	"fmt"
	"strings"
	"testing"
)

func TestTerminalThemePaletteIndices(t *testing.T) {
	for index := 0; index < 16; index++ {
		color := rgb{r: index + 1, g: -1}
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
	for _, want := range []string{"\x1b[7m", "\x1b[22;38;2;244;92;105m▀", "\x1b[27m", "\x1b[22;38;2;139;202;255mx"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing default-background half-cell SGR %q in %q", want, got)
		}
	}
}

func TestTerminalThemeFadeAndFlash(t *testing.T) {
	if got := blend(red, background, .5); got != (rgb{r: 244, g: 92, b: 105, dim: true}) {
		t.Fatalf("fade needs dim truecolor red, got %v", got)
	}
	if got := blend(red, background, 1); got != background {
		t.Fatalf("fade endpoint=%v", got)
	}
	if got := blend(red, white, .45); got != (rgb{r: 249, g: 165, b: 173}) {
		t.Fatalf("flash needs bright truecolor red, got %v", got)
	}
	if got := blend(background, red, 0); got != background {
		t.Fatalf("zero flash=%v", got)
	}
	c := newCanvas(3, 1)
	c.put(0, 0, 'a', blend(red, background, .5))
	c.put(1, 0, 'b', red)
	c.put(2, 0, 'c', blend(red, white, .45))
	got := c.ansi()
	for _, want := range []string{"\x1b[2;38;2;244;92;105ma", "\x1b[22;38;2;244;92;105mb", "\x1b[22;38;2;249;165;173mc"} {
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
	for _, want := range []string{"\x1b[49m", "\x1b[48;2;139;202;255m", "\x1b[22;38;2;244;92;105m", "\x1b[22;39m", "\x1b[22;97m", "\x1b[22;39md"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing theme SGR %q in %q", want, got)
		}
	}

	if !strings.Contains(got, "a\x1b[49m\x1b[22;39mb") {
		t.Fatalf("palette colors leaked into default cell: %q", got)
	}
}
