package app

import "testing"

func TestDottedCellsTranslateExactlyByWholeTerminalCells(t *testing.T) {
	for i := 0; i < 100; i++ {
		u := newUI(newGraph(), "demo")
		u.width, u.height = 120, 60
		n := &node{x: 40 + float64(i)/11, y: 15 + float64(i)/7}
		a, b := newCanvas(120, 60), newCanvas(120, 60)
		u.drawFileDisk(a, n, 7.1234)
		u.panX, u.panY = 3, 2
		u.drawFileDisk(b, n, 7.1234)
		for y := 2; y < 56; y++ {
			for x := 0; x < 117; x++ {
				if a.cells[y*a.w+x].dots != b.cells[(y+2)*b.w+x+3].dots {
					t.Fatalf("case %d: glyph did not translate at %d,%d", i, x, y)
				}
			}
		}
	}
}
