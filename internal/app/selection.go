package app

import (
	"encoding/base64"
	"strings"
)

// Bound terminal clipboard writes; oversized selections leave the clipboard alone.
const maxClipboardBytes = 64 * 1024

type previewSelection struct {
	x, y       int
	text       *canvas
	start, end int
	dragging   bool
}

// The snapshot is made from rendered cells, never ANSI or source escape sequences.
func (u *ui) previewCells(c *canvas, x, y, w, h int) {
	if s := u.selection; s != nil && s.dragging {
		if s.x == x && s.y == y && s.text.w == w && s.text.h <= c.h-y {
			bottom := c.h
			if !u.combined {
				bottom--
			}
			for row := y; row < bottom; row++ {
				for col := x; col < x+w; col++ {
					c.cells[row*c.w+col] = cell{}
				}
			}
			for row := 0; row < s.text.h; row++ {
				copy(c.cells[(y+row)*c.w+x:(y+row)*c.w+x+w], s.text.cells[row*w:(row+1)*w])
			}
			u.paintPreviewSelection(c)
			return
		}
		u.selection = nil
	}
	if w <= 0 || h <= 0 {
		u.preview = nil
		return
	}
	p := &previewSelection{x: x, y: y, text: newCanvas(w, h)}
	for row := 0; row < h; row++ {
		copy(p.text.cells[row*w:(row+1)*w], c.cells[(y+row)*c.w+x:(y+row)*c.w+x+w])
	}
	u.preview = p
}

func (u *ui) selectionMouse(code, x, y int, release bool) bool {
	if s := u.selection; s != nil && s.dragging {
		if release || code&32 != 0 && code&3 == 0 {
			col := max(0, min(s.text.w-1, x-s.x))
			row := max(0, min(s.text.h-1, y-s.y))
			s.end = row*s.text.w + col
			if release {
				s.dragging = false
				text := s.selectedText()
				if text != "" && len(text) <= maxClipboardBytes {
					u.pendingControl = "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
				}
				u.selection = nil
			}
		}
		return true
	}
	p := u.preview
	if p == nil || release || code&(32|64) != 0 || code&3 != 0 || x < p.x || x >= p.x+p.text.w || y < p.y || y >= p.y+p.text.h {
		return false
	}
	s := *p
	s.start = (y-p.y)*p.text.w + x - p.x
	s.end = s.start
	s.dragging = true
	u.selection = &s
	u.graphFocus = false
	return true
}

func (s *previewSelection) bounds() (int, int) {
	a, b := s.start, s.end
	if a > b {
		a, b = b, a
	}
	// Selecting either half of a wide character selects its entire glyph.
	if a%s.text.w > 0 && s.text.cells[a].ch == 0 && runeWidth(s.text.cells[a-1].ch) == 2 {
		a--
	}
	if runeWidth(s.text.cells[b].ch) == 2 && b%s.text.w < s.text.w-1 {
		b++
	}
	return a, b
}
func (s *previewSelection) selectedText() string {
	a, b := s.bounds()
	var lines []string
	for row := a / s.text.w; row <= b/s.text.w; row++ {
		lo, hi := max(a, row*s.text.w), min(b+1, (row+1)*s.text.w)
		// Zero cells are renderer padding, whereas explicit spaces belong to the diff.
		for hi > lo && s.text.cells[hi-1].ch == 0 {
			if hi > row*s.text.w+1 && runeWidth(s.text.cells[hi-2].ch) == 2 {
				break
			}
			hi--
		}
		var line strings.Builder
		for i := lo; i < hi; i++ {
			v := s.text.cells[i]
			if v.ch != 0 {
				line.WriteRune(v.ch)
				line.WriteString(v.combining)
			}
		}
		lines = append(lines, line.String())
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
func (u *ui) paintPreviewSelection(c *canvas) {
	s := u.selection
	a, b := s.bounds()
	for i := a; i <= b; i++ {
		row, col := i/s.text.w, i%s.text.w
		v := &c.cells[(s.y+row)*c.w+s.x+col]
		v.bg = selectedBackground
		v.color = white
	}
}
