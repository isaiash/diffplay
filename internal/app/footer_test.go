package app

import (
	"strings"
	"testing"
	"time"
)

func TestFooterHasRightAlignedSemanticLegend(t *testing.T) {
	for _, view := range []string{"list", "combined", "graph"} {
		u := newUI(newGraph(), "repo")
		u.listMode, u.combined = view == "list", view == "combined"
		var c *canvas
		switch view {
		case "list":
			c = u.renderListCanvas(160, 36, time.Time{})
		case "combined":
			c = u.renderCombined(160, 36, time.Time{})
		default:
			c = u.renderGraphCanvas(160, 36, time.Time{})
		}
		footer := string(canvasRunes(c, c.h-1))
		if !strings.HasSuffix(footer, "new edited removed ") || strings.Contains(footer, "terminal copy shortcut") {
			t.Fatalf("%s footer: %q", view, footer)
		}
		for _, part := range []struct {
			label string
			color rgb
		}{{"new", blue}, {"edited", green}, {"removed", red}} {
			x := len([]rune(footer[:strings.LastIndex(footer, part.label)]))
			for i := 0; i < len(part.label); i++ {
				if c.cells[(c.h-1)*c.w+x+i].color != part.color {
					t.Fatalf("%s legend %s lacks semantic color", view, part.label)
				}
			}
		}
	}
}

func canvasRunes(c *canvas, row int) []rune {
	out := make([]rune, c.w)
	for x := range out {
		out[x] = c.cells[row*c.w+x].ch
		if out[x] == 0 {
			out[x] = ' '
		}
	}
	return out
}
