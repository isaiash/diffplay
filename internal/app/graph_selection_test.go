package app

import "testing"

func TestSelectedFileWhitensOnlyAncestorFrames(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 100, 40
	for _, selected := range []string{"A/C/file", "", "A/C", "A", ""} {
		u.selected = selected
		for _, id := range []string{"A", "A/C", "A/C/child", "B", "A/Cousin"} {
			t.Run(selected+"/"+id, func(t *testing.T) {
				want := "\x1b[22;38;2;128;128;128m"
				if selected == "A/C/file" && (id == "A" || id == "A/C") || selected == "A/C" && (id == "A" || id == "A/C") || selected == "A" && id == "A" {
					want = "\x1b[22;97m"
				}
				c := newCanvas(100, 40)
				u.drawFolderFrame(c, folderEnvelope{ID: id, Left: 5, Top: 3, Width: 35, Height: 10})
				for _, point := range []struct {
					x, y int
					ch   rune
				}{{5, 5, '│'}, {7, 3, rune(folderName(id)[0])}} {
					v := c.cells[point.y*c.w+point.x]
					if v.ch != point.ch || v.color.sgr(false) != want {
						t.Errorf("selected=%q frame=%q cell=(%d,%d): glyph=%q SGR=%q; want %q %q", selected, id, point.x, point.y, v.ch, v.color.sgr(false), point.ch, want)
					}
				}
			})
		}
	}
}
