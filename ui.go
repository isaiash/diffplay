package main

import (
	"fmt"
	"math"

	"strings"
	"time"
	"unicode"
)

type rgb struct{ r, g, b int }

var gray = rgb{145, 145, 156}
var white = rgb{241, 242, 248}
var blue = rgb{139, 202, 255}
var red = rgb{244, 92, 105}
var muted = rgb{79, 79, 92}
var background = rgb{23, 23, 27}

type cell struct {
	ch     rune
	color  rgb
	dots   uint8
	bg     rgb
	strike bool
}
type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas { return &canvas{w: w, h: h, cells: make([]cell, w*h)} }
func (c *canvas) put(x, y int, r rune, color rgb) {
	if x >= 0 && y >= 0 && x < c.w && y < c.h {
		c.cells[y*c.w+x] = cell{ch: r, color: color}
	}
}
func runeWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 || r >= 0xf900 && r <= 0xfaff || r >= 0xfe10 && r <= 0xfe6f || r >= 0xff00 && r <= 0xff60 || r >= 0x1f300 && r <= 0x1faff || r >= 0x20000) {
		return 2
	}
	return 1
}
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return '�'
		}
		return r
	}, s)
}
func (c *canvas) text(x, y int, s string, color rgb, limit int) {
	end := min(c.w, x+limit)
	for _, r := range safeText(s) {
		rw := runeWidth(r)
		if rw == 0 {
			continue
		}
		if x+rw > end {
			break
		}
		c.put(x, y, r, color)
		if rw == 2 {
			c.put(x+1, y, 0, color)
		}
		x += rw
	}
}
func blend(a, b rgb, t float64) rgb {
	t = clamp(t, 0, 1)
	return rgb{int(float64(a.r)*(1-t) + float64(b.r)*t), int(float64(a.g)*(1-t) + float64(b.g)*t), int(float64(a.b)*(1-t) + float64(b.b)*t)}
}
func (c *canvas) dot(x, y int, color rgb) {
	if x < 0 || y < 0 || x >= c.w*2 || y >= c.h*4 {
		return
	}
	i := (y/4)*c.w + x/2
	bits := [2][4]uint8{{1, 2, 4, 64}, {8, 16, 32, 128}}
	c.cells[i].dots |= bits[x%2][y%4]
	c.cells[i].color = color
}

func (c *canvas) ansi() string {
	var b strings.Builder
	b.Grow(c.w * c.h * 3)
	b.WriteString("\x1b[H\x1b[48;2;23;23;27m")
	last := rgb{-1, -1, -1}
	lastBG := background
	struck := false
	for y := 0; y < c.h; y++ {
		if y > 0 {
			b.WriteString("\r\n")
		}
		for x := 0; x < c.w; x++ {
			v := c.cells[y*c.w+x]
			r := v.ch
			if r == 0 {
				if x > 0 && runeWidth(c.cells[y*c.w+x-1].ch) == 2 {
					continue
				}
				if v.dots != 0 {
					r = rune(0x2800 + int(v.dots))
				} else {
					r = ' '
				}
			}
			bg := v.bg
			if bg == (rgb{}) {
				bg = background
			}
			if bg != lastBG {
				fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm", bg.r, bg.g, bg.b)
				lastBG = bg
			}
			if v.strike != struck {
				if v.strike {
					b.WriteString("\x1b[9m")
				} else {
					b.WriteString("\x1b[29m")
				}
				struck = v.strike
			}
			if v.color != last {
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", v.color.r, v.color.g, v.color.b)
				last = v.color
			}
			b.WriteRune(r)
		}
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

type ui struct {
	graph                   *graph
	root, selected, warning string
	width, height           int
	zoom                    float64
	paused, quit            bool
	pending                 []byte
	scroll                  int
	dragging                bool

	history          *history
	listMode         bool
	listSelected     string
	listOffset       int
	open             map[string]bool
	repositoryLabels map[string]string
	graphRadii       map[string]float64
	graphDrags       map[string][2]float64
}

func newUI(g *graph, root string) *ui {
	return &ui{graph: g, root: root, zoom: 1, history: newHistory(), open: map[string]bool{".": true}, listSelected: "."}
}
func (u *ui) screen(n *node) (float64, float64) {
	w, h := float64(u.width), float64(max(1, u.height-4))
	return (n.x-w/2)*u.zoom + w/2, (n.y-h/2)*u.zoom + h/2 + 2
}
func (u *ui) hit(x, y int) string {
	best := math.Inf(1)
	id := ""
	for _, n := range u.fileNodes() {
		nx, ny := u.screen(n)
		d := math.Hypot(nx-float64(x), (ny-float64(y))*2)
		radius := max(.5, u.displayRadius(n)*u.zoom)
		if d <= radius+.5 && d/radius < best {
			best = d / radius
			id = n.id
		}
	}
	return id
}
func (u *ui) input(data []byte) {
	u.pending = append(u.pending, data...)
	for len(u.pending) > 0 {
		if u.pending[0] == 27 {
			if len(u.pending) == 1 {
				return
			}
			s := string(u.pending)
			if strings.HasPrefix(s, "\x1b[<") {
				end := strings.IndexAny(s, "Mm")
				if end < 0 {
					if len(s) > 80 {
						u.pending = nil
					}
					return
				}
				var code, x, y int
				_, err := fmt.Sscanf(s[3:end], "%d;%d;%d", &code, &x, &y)
				if err == nil {
					x--
					y--
					if u.listMode {
						u.listMouse(code, x, y, s[end] == 'm')
						u.pending = u.pending[end+1:]
						continue
					}
					if code&64 != 0 {
						if code&1 == 0 {
							u.zoom = clamp(u.zoom*1.15, 0.2, 6)
						} else {
							u.zoom = clamp(u.zoom/1.15, 0.2, 6)
						}
					} else if s[end] == 'm' {
						u.dragging = false
					} else if code&3 == 0 && code&32 == 0 {
						u.selected = u.hit(x, y)
						u.dragging = u.selected != ""
						u.scroll = 0
					} else if code&32 != 0 {
						if u.dragging {
							if n := u.graph.nodes[u.selected]; n != nil {
								xx := (float64(x)-float64(u.width)/2)/u.zoom + float64(u.width)/2
								yy := (float64(y-2)-float64(max(1, u.height-4))/2)/u.zoom + float64(max(1, u.height-4))/2
								u.tryGraphDrag(n, xx, yy)
							}
						} else {
							id := u.hit(x, y)
							if id != u.selected {
								u.scroll = 0
							}
							u.selected = id
						}
					}
				}
				u.pending = u.pending[end+1:]
				continue
			}
			if s == "\x1b[" || strings.HasPrefix("\x1b[<", s) {
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
				key := s[2 : end+1]
				if u.listMode {
					u.listKey(key)
				} else {
					switch key {
					case "A":
						u.scroll = max(0, u.scroll-1)
					case "B":
						u.scroll++
					case "C":
						u.cycleGraph(1)
					case "D":
						u.cycleGraph(-1)
					}
				}
				u.pending = u.pending[end+1:]
				continue
			}
			u.selected = ""
			u.dragging = false
			u.pending = u.pending[1:]
			continue
		}
		key := u.pending[0]
		u.pending = u.pending[1:]
		if key == '	' {
			u.listMode = !u.listMode
			u.dragging = false
			u.scroll = 0
			continue
		}
		if u.listMode && key != 'q' && key != 3 {
			if key == '\r' || key == '\n' {
				u.listKey("enter")
			} else {
				u.listKey(string(key))
			}
			continue
		}
		switch key {
		case 'q', 3:
			u.quit = true
		case ' ':
			u.paused = !u.paused

		case '+', '=':
			u.zoom = clamp(u.zoom*1.15, 0.2, 6)
		case '-':
			u.zoom = clamp(u.zoom/1.15, 0.2, 6)
		case 'r':
			u.zoom = 1
		case 'j':
			u.scroll++
		case 'k':
			u.scroll = max(0, u.scroll-1)
		case 'n':
			u.cycleGraph(1)
		case 'p':
			u.cycleGraph(-1)
		}
	}
}
func (u *ui) cycleGraph(delta int) {
	nodes := u.fileNodes()
	if len(nodes) == 0 {
		return
	}
	index := -1
	for i, n := range nodes {
		if n.id == u.selected {
			index = i
			break
		}
	}
	if index < 0 {
		if delta < 0 {
			index = 0
		} else {
			index = -1
		}
	}
	index = (index + delta + len(nodes)) % len(nodes)
	u.selected = nodes[index].id
	u.scroll = 0
}
func (u *ui) render(w, h int, now time.Time) string {
	w = max(1, w)
	h = max(1, h)
	u.width = w
	u.height = h
	if u.listMode {
		return u.renderList(w, h, now)
	}
	return u.renderGraph(w, h, now)
}
