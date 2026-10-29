package app

import (
	"strings"
	"testing"
	"time"
)

func TestSemanticColorsUseFixedTruecolor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		color  rgb
		fg, bg string
	}{
		{"new", blue, "\x1b[22;38;2;139;202;255m", "\x1b[48;2;139;202;255m"},
		{"modified", green, "\x1b[22;38;2;115;211;153m", "\x1b[48;2;115;211;153m"},
		{"deleted", red, "\x1b[22;38;2;244;92;105m", "\x1b[48;2;244;92;105m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.color.sgr(false); got != tc.fg {
				t.Errorf("foreground = %q, want %q", got, tc.fg)
			}
			if got := tc.color.sgr(true); got != tc.bg {
				t.Errorf("background = %q, want %q", got, tc.bg)
			}
		})
	}
}

func TestSemanticFlashAndFadeKeepHue(t *testing.T) {
	for _, tc := range []struct {
		name        string
		base, flash rgb
	}{
		{"new", blue, rgb{r: 191, g: 226, b: 255}},
		{"modified", green, rgb{r: 178, g: 231, b: 199}},
		{"deleted", red, rgb{r: 249, g: 165, b: 173}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := blend(tc.base, white, .45); got != tc.flash {
				t.Errorf("rounded flash = %+v, want %+v", got, tc.flash)
			}
			if tc.name == "new" && !(tc.flash.b > tc.flash.g && tc.flash.g > tc.flash.r) ||
				tc.name == "modified" && !(tc.flash.g > tc.flash.b && tc.flash.b > tc.flash.r) ||
				tc.name == "deleted" && !(tc.flash.r > tc.flash.b && tc.flash.b > tc.flash.g) {
				t.Errorf("flash lost %s hue: %+v", tc.name, tc.flash)
			}
			faded := tc.base
			faded.dim = true
			if got := blend(tc.base, background, .5); got != faded {
				t.Errorf("fade = %+v, want %+v", got, faded)
			}
			if got := blend(background, tc.base, .5); got != faded {
				t.Errorf("reverse fade = %+v, want %+v", got, faded)
			}
			if blend(tc.base, background, 1) != background || blend(tc.base, white, 1) != white {
				t.Error("fade/flash endpoint changed")
			}
		})
	}
}

func TestSemanticListDiffHeaderAndSelection(t *testing.T) {
	now := time.Unix(100, 0)
	for _, tc := range []struct {
		kind, addedSGR, flashBG string
	}{
		{"created", "\x1b[22;38;2;139;202;255m", "\x1b[48;2;191;226;255m"},
		{"modified", "\x1b[22;38;2;115;211;153m", "\x1b[48;2;178;231;199m"},
		{"deleted", "\x1b[22;38;2;115;211;153m", "\x1b[48;2;249;165;173m"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.recordChange("target", tc.kind, false, "+added\n-removed", 8, 4, true, now)
			u.listSelected = "target"
			frame := u.renderList(140, 20, now.Add(time.Second))
			for _, want := range []string{tc.addedSGR + "+added", "\x1b[22;38;2;244;92;105m-removed", "\x1b[22;97m›", "\x1b[22;39m", "\x1b[49m"} {
				if !strings.Contains(frame, want) {
					t.Errorf("missing %q in rendered list", want)
				}
			}
			if flash := u.renderList(140, 20, now); !strings.Contains(flash, tc.flashBG) {
				t.Errorf("selected flash missing %q", tc.flashBG)
			}
		})
	}
	u := newUI(newGraph(), "/repo")
	u.repositoryTotals = map[string]repositoryLineCount{"": {lines: 2, known: true}}
	u.recordChange("new", "created", false, "", 1, 0, true, now)
	u.recordChange("edit", "modified", false, "", 1, 1, true, now)
	frame := u.renderList(10, 5, now.Add(time.Second))
	for _, want := range []string{"\x1b[48;2;139;202;255m", "\x1b[48;2;115;211;153m", "\x1b[48;2;244;92;105m"} {
		if !strings.Contains(frame, want) {
			t.Errorf("header missing %q", want)
		}
	}
}

func TestSemanticGraphCellAndRipple(t *testing.T) {
	for _, tc := range []struct {
		name           string
		fresh          bool
		added, removed int
		body, core     string
	}{
		{"new", true, 300, 100, "\x1b[22;38;2;139;202;255m", "\x1b[22;38;2;244;92;105m"},
		{"edit", false, 300, 100, "\x1b[22;38;2;115;211;153m", "\x1b[22;38;2;244;92;105m"},
		{"deletion-dominant", false, 100, 300, "\x1b[22;38;2;244;92;105m", "\x1b[22;38;2;115;211;153m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "demo")
			u.width, u.height = 80, 30
			c := newCanvas(80, 30)
			u.drawFileDisk(c, &node{id: "a", x: 30, y: 10, kind: "modified", added: tc.added, removed: tc.removed, fresh: tc.fresh}, 6)
			frame := c.ansi()
			if !strings.Contains(frame, tc.body) || !strings.Contains(frame, tc.core) {
				t.Errorf("cell missing truecolor body/core")
			}
		})
	}
	for _, tc := range []struct {
		kind  string
		color rgb
		sgr   string
	}{
		{"created", blue, "\x1b[2;38;2;139;202;255m"},
		{"modified", green, "\x1b[2;38;2;115;211;153m"},
		{"deleted", red, "\x1b[2;38;2;244;92;105m"},
	} {
		t.Run("ripple-"+tc.kind, func(t *testing.T) {
			u, start := graphFlashFixture()
			u.graph.nodes["file"].kind = tc.kind
			u.history.entries["file"].fresh = tc.kind == "created"
			c := u.renderGraphCanvas(120, 36, start.Add(450*time.Millisecond))
			want := tc.color
			want.dim = true
			for _, v := range c.cells {
				if v.dots != 0 && v.color == want {
					if !strings.Contains(c.ansi(), tc.sgr) {
						t.Error("ripple missing dim truecolor SGR")
					}
					return
				}
			}
			t.Fatal("no dim semantic ripple dots")
		})
	}
}
