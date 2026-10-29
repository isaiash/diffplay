package app

import "testing"

func TestSelectedFileWhitensOnlyAncestorFrames(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 100, 40
	u.selected = "A/C/file"
	for _, tc := range []struct {
		id    string
		white bool
	}{{"A", true}, {"A/C", true}, {"A/C/child", false}, {"B", false}, {"A/Cousin", false}} {
		c := newCanvas(100, 40)
		u.drawFolderFrame(c, folderEnvelope{ID: tc.id, Left: 5, Top: 3, Width: 35, Height: 10})
		color := c.cells[5*c.w+5].color
		if (color == white) != tc.white {
			t.Errorf("frame %s white=%v want=%v", tc.id, color == white, tc.white)
		}
	}
}
