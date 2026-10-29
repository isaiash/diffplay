package app

import "testing"

func TestTinySelectedFileHasVisibleOutline(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 80, 24
	u.graphRadii = map[string]float64{"f": .1}
	n := &node{id: "f", x: 30, y: 8, kind: "modified", added: 1}
	c := newCanvas(80, 24)
	u.drawFileDisk(c, n, .1)
	before := append([]cell(nil), c.cells...)
	u.selected = n.id
	c = newCanvas(80, 24)
	u.drawFileDisk(c, n, .1)
	whiteDots := 0
	for i, cell := range c.cells {
		if cell.dots != before[i].dots {
			t.Fatalf("tiny selection changed mask at cell %d", i)
		}
		if cell.color == white && cell.dots != 0 {
			whiteDots++
		}
	}
	if whiteDots == 0 {
		t.Fatal("tiny selected file has no visible white marker")
	}
}
