package app

import (
	"math"
	"math/bits"
	"testing"
)

func TestCombinedConcentricFilledDisks(t *testing.T) {
	for _, tc := range []struct {
		name           string
		added, removed int
		fresh          bool
		weights        map[rgb]int
		order          []rgb
	}{
		{"small-edits", 20, 10, false, map[rgb]int{green: 20, red: 10}, []rgb{green, red}},
		{"green", 70, 10, false, map[rgb]int{green: 70, red: 10}, []rgb{green, red}},
		{"blue", 70, 10, true, map[rgb]int{blue: 70, red: 10}, []rgb{blue, red}},
		{"created", 70, 10, false, map[rgb]int{blue: 70, red: 10}, []rgb{blue, red}},
		{"red", 10, 70, false, map[rgb]int{red: 70, green: 10}, []rgb{red, green}},
		{"new-red", 10, 70, true, map[rgb]int{red: 70, blue: 10}, []rgb{red, blue}},
		{"overflow", 120, 80, false, map[rgb]int{green: 120, red: 80}, []rgb{green, red}},
		{"ties", 50, 50, false, map[rgb]int{green: 50, red: 50}, []rgb{green, red}},
		{"negative", -50, -20, false, map[rgb]int{green: 100}, []rgb{green}},
		{"zero", 0, 0, false, map[rgb]int{green: 100}, []rgb{green}},
		{"unknown", 30, 10, false, map[rgb]int{green: 30, red: 10}, []rgb{green, red}},
		{"empty", 30, 10, false, map[rgb]int{green: 30, red: 10}, []rgb{green, red}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newUI(newGraph(), "repo")
			u.combined = true
			u.width, u.height = 100, 50
			n := &node{id: "file", x: 50, y: 24, kind: "modified", lines: 100, linesKnown: true, added: tc.added, removed: tc.removed, fresh: tc.fresh}
			if tc.name == "created" {
				n.kind = "created"
			}
			if tc.name == "unknown" {
				n.linesKnown = false
			}
			if tc.name == "empty" {
				n.lines = 0
			}
			c := newCanvas(100, 50)
			u.drawFileDisk(c, n, 12)
			expected := newCanvas(100, 50)
			for yy := 0; yy < 200; yy++ {
				for xx := 0; xx < 200; xx++ {
					dx, dy := float64(xx)/2+.25-50.5, float64(yy)/2+.25-49
					if dx*dx+dy*dy <= 144 {
						expected.dot(xx, yy, white)
					}
				}
			}
			counts := map[rgb]int{}
			ranges := map[rgb][2]float64{}
			total := 0
			for i, v := range c.cells {
				if v.dots != expected.cells[i].dots {
					t.Fatalf("disk has missing/extra dots in cell %d: %08b vs %08b", i, v.dots, expected.cells[i].dots)
				}
				if v.dots == 0 {
					continue
				}
				count := bits.OnesCount8(v.dots)
				counts[v.color] += count
				total += count
				dx, dy := float64(i%100)+.5-50.5, 2*(float64(i/100)+.5-24.5)
				d := dx*dx + dy*dy
				r, ok := ranges[v.color]
				if !ok {
					r = [2]float64{d, d}
				}
				r[0] = math.Min(r[0], d)
				r[1] = math.Max(r[1], d)
				ranges[v.color] = r
			}
			sum := 0
			for _, w := range tc.weights {
				sum += w
			}
			for color, weight := range tc.weights {
				if math.Abs(float64(counts[color])-float64(total*weight)/float64(sum)) > 16 {
					t.Fatalf("color %v has %d/%d dots, want weight %d/%d", color, counts[color], total, weight, sum)
				}
			}
			for i := 1; i < len(tc.order); i++ {
				if ranges[tc.order[i-1]][0] < ranges[tc.order[i]][1] {
					t.Fatal("larger layer is not entirely outside smaller layer")
				}
			}
			u.selected = n.id
			selected := newCanvas(100, 50)
			u.drawFileDisk(selected, n, 12)
			for i, v := range selected.cells {
				if v.dots != c.cells[i].dots || v.dots != 0 && v.color != white {
					t.Fatal("selection altered mask or left nonwhite cells")
				}
			}
		})
	}
}

func TestCombinedUnknownMarkersUseKnownKindColor(t *testing.T) {
	for _, tc := range []struct {
		kind  string
		color rgb
	}{{"created", blue}, {"modified", green}, {"deleted", red}} {
		t.Run(tc.kind, func(t *testing.T) {
			u := newUI(newGraph(), "repo")
			u.combined, u.width, u.height = true, 100, 50
			u.graph.upsert("file", ".", false)
			n := u.graph.nodes["file"]
			n.kind, n.unknown, n.added, n.removed = tc.kind, true, 300, 900
			n.x, n.y = 50, 24
			u.updateGraphMetrics()
			if n.radius != .5 {
				t.Fatal("unknown counts must use a minimum marker")
			}
			c := newCanvas(100, 50)
			u.drawFileDisk(c, n, n.radius)
			dots := 0
			for _, v := range c.cells {
				dots += bits.OnesCount8(v.dots)
				if v.dots != 0 && v.color != tc.color {
					t.Fatal("unknown marker used unavailable counts instead of the known change kind")
				}
			}
			if dots == 0 {
				t.Fatal("unknown marker disappeared")
			}
		})
	}
}

func TestCombinedDeletedDiskDoesNotInventCurrentGrayLines(t *testing.T) {
	u := newUI(newGraph(), "repo")
	u.combined, u.width, u.height = true, 100, 50
	n := &node{id: "gone", kind: "deleted", lines: 1000, linesKnown: true, removed: 100, x: 50, y: 24}
	c := newCanvas(100, 50)
	u.drawFileDisk(c, n, 12)
	for _, v := range c.cells {
		if v.dots != 0 && v.color != red {
			t.Fatal("last scanned lines of a deleted file became current gray coverage")
		}
	}
}
