package main

import (
	"fmt"
	"math"
	"testing"
)

func TestGraphWheelZoomAnchorsPointer(t *testing.T) {
	u := newUI(newGraph(), "demo")
	u.width, u.height = 100, 40
	anchor := &node{x: 23, y: 11}
	for _, code := range []int{64, 65, 65, 64} {
		u.input([]byte(fmt.Sprintf("\x1b[<%d;24;12M", code)))
		x, y := u.screen(anchor)
		if math.Abs(x-23) > 1e-9 || math.Abs(y-11) > 1e-9 {
			t.Fatalf("wheel %d moved anchor to (%g, %g)", code, x, y)
		}
	}
}

func TestGraphDragPansEntireView(t *testing.T) {
	for _, onFile := range []bool{false, true} {
		t.Run(fmt.Sprintf("onFile=%v", onFile), func(t *testing.T) {
			g := newGraph()
			g.upsert("a", "", false)
			n := g.nodes["a"]
			n.x, n.y = 40, 15
			u := newUI(g, "demo")
			u.width, u.height, u.zoom = 100, 40, 2
			x, y := u.screen(n)
			px, py := 5, 5
			if onFile {
				px, py = int(x), int(y)
			}
			u.input([]byte(fmt.Sprintf("\x1b[<0;%d;%dM", px+1, py+1)))
			u.input([]byte(fmt.Sprintf("\x1b[<32;%d;%dM", px+8, py+5)))
			xx, yy := u.screen(n)
			if math.Abs(xx-x-7) > 1e-9 || math.Abs(yy-y-4) > 1e-9 {
				t.Fatalf("drag moved view by (%g,%g), want (7,4)", xx-x, yy-y)
			}
			if n.x != 40 || n.y != 15 {
				t.Fatal("drag changed file layout")
			}
			if u.hit(int(xx), int(yy)) != "a" {
				t.Fatal("hit testing did not follow pan")
			}
			u.input([]byte(fmt.Sprintf("\x1b[<0;%d;%dm", px+8, py+5)))
			u.input([]byte("\x1b[<35;1;1M"))
			xxx, yyy := u.screen(n)
			if xxx != xx || yyy != yy || u.dragging {
				t.Fatal("view kept dragging after release")
			}
			u.input([]byte("r"))
			rx, ry := u.screen(n)
			if u.zoom != 1 || rx != 40 || ry != 15 {
				t.Fatal("reset must restore zoom and position")
			}
		})
	}
}
