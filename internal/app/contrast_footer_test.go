package app

import (
	"testing"
	"time"
)

func TestHelpTextUsesTerminalForeground(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.combined = true
	c := u.renderCombined(80, 24, time.Unix(100, 0))
	v := c.cells[23*c.w+1]
	if v.ch != 'q' || v.color.sgr(false) != "\x1b[22;39m" {
		t.Fatalf("help must use the normal terminal foreground, got %+v (%q)", v, v.color.sgr(false))
	}
}
