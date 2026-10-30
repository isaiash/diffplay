package app

import (
	"strings"
	"testing"
	"time"
)

func assertStructure(t *testing.T, c *canvas, x, y int, glyph rune) {
	t.Helper()
	v := c.cells[y*c.w+x]
	if v.ch != glyph {
		t.Fatalf("cell (%d,%d) = %q, want %q", x, y, v.ch, glyph)
	}
	if v.bg != background && v.bg != (rgb{}) {
		t.Fatalf("fixture must use default background: %+v", v)
	}
	if v.color.sgr(false) != "\x1b[22;39m" {
		t.Errorf("%q at (%d,%d) foreground=%+v SGR=%q; want normal terminal foreground", glyph, x, y, v.color, v.color.sgr(false))
	}
}

func structureFixture() *ui {
	u := newUI(newGraph(), "/repo")
	u.recordChange("dir/nested/file", "modified", false, "+new\n-old", 8, 4, true, time.Unix(100, 0))
	u.recordChange("other", "modified", false, "", 1, 1, true, time.Unix(100, 0))
	u.listSelected = "other"
	return u
}

func TestStructureListTreeAndHistogram(t *testing.T) {
	u := structureFixture()
	c := u.renderListCanvas(140, 24, time.Unix(200, 0))
	rows := u.changeRows()
	for _, tc := range []struct {
		name, id string
		glyphs   string
	}{
		{"nested connector and folder icon", "dir/nested", "├─▾"},
		{"deep ancestor connector", "dir/nested/file", "│├─"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := -1
			for i, row := range rows {
				if row.id == tc.id {
					y = i + 2
				}
			}
			if y < 0 {
				t.Fatalf("missing row %s", tc.id)
			}
			for _, glyph := range tc.glyphs {
				x := -1
				for col := 0; col < u.listWidth(c.w); col++ {
					if c.cells[y*c.w+col].ch == glyph {
						x = col
						if glyph == '│' && col < 20 {
							x = -1
							continue
						}
						break
					}
				}
				if x < 0 {
					t.Fatalf("missing %q in row %s", glyph, tc.id)
				}
				assertStructure(t, c, x, y, glyph)
			}
		})
	}
	t.Run("histogram center", func(t *testing.T) {
		y := -1
		for i, row := range rows {
			if row.id == "dir/nested/file" {
				y = i + 2
			}
		}
		if y < 0 {
			t.Fatal("missing file row")
		}
		for x := 1; x < 20; x++ {
			if c.cells[y*c.w+x].ch == '│' && c.cells[y*c.w+x-1].ch == '█' && c.cells[y*c.w+x+1].ch == '█' {
				assertStructure(t, c, x, y, '│')
				return
			}
		}
		t.Fatal("missing histogram center with bars")
	})
	t.Run("list diff vertical divider", func(t *testing.T) { assertStructure(t, c, u.listWidth(c.w), 5, '│') })
}

func TestStructureCombinedDivider(t *testing.T) {
	u := structureFixture()
	u.combined = true
	c := u.renderCombined(140, 40, time.Unix(200, 0))
	assertStructure(t, c, 0, u.splitTop(40), '─')
	assertStructure(t, c, 139, u.splitTop(40), '─')
}

func TestStructureFolderFrames(t *testing.T) {
	for _, tc := range []struct {
		name, selected string
		want           rgb
	}{
		{"inactive", "", gray}, {"unrelated active", "other/file", gray}, {"selected ancestor", "dir/file", white},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "/repo")
			u.width, u.height, u.selected = 100, 40, tc.selected
			c := newCanvas(100, 40)
			u.drawFolderFrame(c, folderEnvelope{ID: "dir", Left: 5, Top: 3, Width: 35, Height: 10})
			for _, point := range []struct {
				x, y int
				ch   rune
			}{{5, 3, '┌'}, {40, 3, '┐'}, {5, 13, '└'}, {40, 13, '┘'}, {5, 5, '│'}, {20, 13, '─'}} {
				v := c.cells[point.y*c.w+point.x]
				if v.ch != point.ch {
					t.Fatalf("frame cell = %q, want %q", v.ch, point.ch)
				}
				if tc.want == gray {
					assertStructure(t, c, point.x, point.y, point.ch)
				} else if v.color != white {
					t.Errorf("selected frame %q = %+v, want white", v.ch, v.color)
				}
			}
		})
	}
}

func TestStructureForegroundANSI(t *testing.T) {
	c := newCanvas(1, 1)
	c.put(0, 0, '│', gray)
	if got := c.ansi(); !strings.Contains(got, "\x1b[22;39m│") {
		t.Fatalf("default foreground must emit intensity and foreground reset: %q", got)
	}
}
