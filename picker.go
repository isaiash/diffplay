package main

import (
	"fmt"
	"strings"
)

type repositoryPicker struct {
	paths                        []string
	checked                      []bool
	focus, offset, width, height int
	pending                      []byte
	done, cancelled              bool
}

func newRepositoryPicker(paths []string) *repositoryPicker {
	return &repositoryPicker{paths: append([]string(nil), paths...), checked: make([]bool, len(paths))}
}
func (p *repositoryPicker) selectedPaths() []string {
	var selected []string
	for i, checked := range p.checked {
		if checked {
			selected = append(selected, p.paths[i])
		}
	}
	if len(selected) == 0 && len(p.paths) > 0 {
		selected = append(selected, p.paths[p.focus])
	}
	return selected
}
func (p *repositoryPicker) move(delta int) { p.focus = max(0, min(len(p.paths)-1, p.focus+delta)) }
func (p *repositoryPicker) input(data []byte) {
	p.pending = append(p.pending, data...)
	for len(p.pending) > 0 && !p.done && !p.cancelled {
		if p.pending[0] == 27 {
			s := string(p.pending)
			if len(s) < 2 {
				return
			}
			if strings.HasPrefix(s, "\x1b[<") {
				end := strings.IndexAny(s, "Mm")
				if end < 0 {
					if len(s) > 80 {
						p.pending = nil
					}
					return
				}
				var code, x, y int
				_, err := fmt.Sscanf(s[3:end], "%d;%d;%d", &code, &x, &y)
				if err == nil && s[end] == 'M' {
					if code&64 != 0 {
						if code&1 == 0 {
							p.move(-3)
						} else {
							p.move(3)
						}
					} else if y >= 4 && y <= p.height-2 {
						index := p.offset + y - 4
						if index >= 0 && index < len(p.paths) {
							p.focus = index
							if code&32 == 0 && code&3 == 0 {
								p.checked[index] = !p.checked[index]
							}
						}
					}
				}
				p.pending = p.pending[end+1:]
				continue
			}
			if s == "\x1b[" {
				return
			}
			if s[1] == '[' {
				end := 2
				for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
					end++
				}
				if end == len(s) {
					return
				}
				switch s[2 : end+1] {
				case "A":
					p.move(-1)
				case "B":
					p.move(1)
				case "H":
					p.focus = 0
				case "F":
					p.focus = max(0, len(p.paths)-1)
				case "5~":
					p.move(-max(1, p.height-5))
				case "6~":
					p.move(max(1, p.height-5))
				}
				p.pending = p.pending[end+1:]
				continue
			}
			p.pending = p.pending[1:]
			continue
		}
		key := p.pending[0]
		p.pending = p.pending[1:]
		switch key {
		case 'q', 3:
			p.cancelled = true
		case '\r', '\n':
			p.done = len(p.paths) > 0
		case 'j':
			p.move(1)
		case 'k':
			p.move(-1)
		case ' ', '	':
			if len(p.paths) > 0 {
				p.checked[p.focus] = !p.checked[p.focus]
			}
		case 'a':
			for i := range p.checked {
				p.checked[i] = true
			}
		case 'c':
			for i := range p.checked {
				p.checked[i] = false
			}
		}
	}
}
func (p *repositoryPicker) render(w, h int) string {
	w = max(1, w)
	h = max(1, h)
	p.width = w
	p.height = h
	c := newCanvas(w, h)
	c.text(1, 0, "Diffplay  SELECT REPOSITORIES", white, w-2)
	c.text(1, 1, "Choose one or more Git repositories to monitor.", gray, w-2)
	count := 0
	for _, selected := range p.checked {
		if selected {
			count++
		}
	}
	c.text(1, 2, fmt.Sprintf("%d repositories found · %d selected", len(p.paths), count), blue, w-2)
	visible := max(0, h-5)
	if p.focus < p.offset {
		p.offset = p.focus
	}
	if visible > 0 && p.focus >= p.offset+visible {
		p.offset = p.focus - visible + 1
	}
	p.offset = max(0, min(p.offset, max(0, len(p.paths)-visible)))
	for i := p.offset; i < len(p.paths) && i-p.offset < visible; i++ {
		y := 3 + i - p.offset
		mark := "[ ] "
		color := gray
		if p.checked[i] {
			mark = "[x] "
			color = blue
		}
		c.text(2, y, mark+p.paths[i], color, w-3)
		if i == p.focus {
			c.put(0, y, '›', white)
			for x := 0; x < w; x++ {
				c.cells[y*w+x].bg = rgb{38, 40, 49}
			}
		}
	}
	c.text(1, h-2, "Enter: open checked repositories (or focused row if none checked)", gray, w-2)
	c.text(1, h-1, "↑↓ move · Tab/Space/click toggle · a all · c clear · q cancel", muted, w-2)
	return c.ansi()
}
