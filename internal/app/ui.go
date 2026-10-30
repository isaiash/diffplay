package app

import (
	"fmt"
	"math"

	"strings"
	"time"
	"unicode"
)

type rgb struct {
	r, g, b  int
	dim      bool
	explicit bool // Distinguish true RGB black from an unspecified color.
}

// A negative green channel marks a palette index; zero remains unspecified.
var gray = rgb{}
var white = rgb{r: 16, g: -1}
var blue = rgb{r: 139, g: 202, b: 255}
var red = rgb{r: 244, g: 92, b: 105}
var muted = rgb{dim: true}
var background = rgb{g: -1}
var headerInk = rgb{r: 1, g: -1}
var selectedBackground = rgb{r: 9, g: -1}

func (color rgb) sgr(bg bool) string {
	intensity := 22
	if color.dim {
		intensity = 2
	}
	if color.g >= 0 && (color.explicit || color.r != 0 || color.g != 0 || color.b != 0) {
		if bg {
			return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", color.r, color.g, color.b)
		}
		return fmt.Sprintf("\x1b[%d;38;2;%d;%d;%dm", intensity, color.r, color.g, color.b)
	}
	code := 39
	if bg {
		code = 49
	}
	if color.g == -1 && color.r > 0 && color.r <= 16 {
		index := color.r - 1
		code = 30 + index
		if index >= 8 {
			code = 90 + index - 8
		}
		if bg {
			code += 10
		}
	}
	if bg {
		return fmt.Sprintf("\x1b[%dm", code)
	}
	return fmt.Sprintf("\x1b[%d;%dm", intensity, code)
}

type cell struct {
	ch        rune
	combining string
	color     rgb
	dots      uint8
	bg        rgb
	strike    bool
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
	start := x
	end := min(c.w, x+limit)
	for _, r := range safeText(s) {
		rw := runeWidth(r)
		if rw == 0 {
			if x > max(0, start) && x <= c.w && y >= 0 && y < c.h {
				i := y*c.w + x - 1
				if c.cells[i].ch == 0 && x > 1 && x-start > 1 && runeWidth(c.cells[i-1].ch) == 2 {
					i--
				}
				c.cells[i].combining += string(r)
			}
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
	if a == b || t == 0 {
		return a
	}
	if t == 1 {
		return b
	}
	// shortcut: unknown theme RGB makes background fades discrete; use dim steps.
	if b == background {
		if t >= .85 {
			return b
		}
		if t >= .25 {
			a.dim = true
		}
		return a
	}
	if a == background {
		if t < .15 {
			return a
		}
		if t < .65 {
			b.dim = true
		}
		return b
	}
	if b == white {
		if a.g >= 0 && (a.r != 0 || a.g != 0 || a.b != 0) {
			a.r = int(math.Round(float64(a.r) + (255-float64(a.r))*t))
			a.g = int(math.Round(float64(a.g) + (255-float64(a.g))*t))
			a.b = int(math.Round(float64(a.b) + (255-float64(a.b))*t))
			a.dim = false
			return a
		}
		if t < .25 {
			return a
		}
		if t >= .75 {
			return b
		}
		if a.g == -1 && a.r >= 1 && a.r <= 8 {
			a.r += 8
		}
		a.dim = false
		return a
	}
	if t < .5 {
		return a
	}
	return b
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
	b.WriteString("\x1b[H\x1b[0m\x1b[49m")
	last := rgb{r: -1, g: -1, b: -1}
	lastBG := background
	struck := false
	reversed := false
	concealed := false
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
			conceal := v.color == background && bg == background
			if conceal != concealed {
				if conceal {
					b.WriteString("\x1b[8m")
				} else {
					b.WriteString("\x1b[28m")
				}
				concealed = conceal
			}
			reverse := v.color == background && !conceal
			if reverse {
				// Reverse video can paint the terminal default background as glyph ink.
				v.color, bg = bg, background
			}
			if reverse != reversed {
				if reverse {
					b.WriteString("\x1b[7m")
				} else {
					b.WriteString("\x1b[27m")
				}
				reversed = reverse
			}
			if bg != lastBG {
				b.WriteString(bg.sgr(true))
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
				b.WriteString(v.color.sgr(false))
				last = v.color
			}
			b.WriteRune(r)
			b.WriteString(v.combining)
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
	panX, panY              float64
	paused, quit            bool
	pending                 []byte
	terminalOSC             []byte
	terminalOSCActive       bool
	terminalOSCStarted      time.Time
	scroll                  int
	dragging                bool
	dragX, dragY            int

	history                          *history
	listMode                         bool
	combined                         bool
	graphFocus                       bool
	splitRatio                       float64
	splitDragging                    bool
	listSplitRatio                   float64
	listSplitDragging                bool
	dividerHover, dividerX, dividerY int
	graphRendering                   bool
	graphLayoutCombined              bool
	terminalWidth, terminalHeight    int
	lastFrame                        string
	preview, selection               *previewSelection
	pendingControl                   string
	lastFrameWidth, lastFrameHeight  int

	terminalBackground, terminalSelectedBackground rgb

	listSelected     string
	listOffset       int
	followPending    []string
	followAt         time.Time
	open             map[string]bool
	repositoryLabels map[string]string
	repositoryTotals map[string]repositoryLineCount
	graphRadii       map[string]float64
	graphDotUnits    map[string]int
}

func newUI(g *graph, root string) *ui {
	return &ui{graph: g, root: root, zoom: 1, history: newHistory(), open: map[string]bool{".": true}, listSelected: "."}
}
func (u *ui) graphContentHeight() int {
	h := u.height
	if !u.combined {
		h--
	}
	return max(1, h)
}
func (u *ui) screen(n *node) (float64, float64) {
	w, h := float64(u.width), float64(u.graphContentHeight())
	y := (n.y-h/2)*u.zoom + h/2 + u.panY
	if u.graphRendering {
		y += 2
	}
	return (n.x-w/2)*u.zoom + w/2 + u.panX, y
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
	// A truncated reply must not capture subsequent keyboard input indefinitely.
	if u.terminalOSCActive && time.Since(u.terminalOSCStarted) >= time.Second {
		u.terminalOSCActive, u.terminalOSC = false, nil
	}
	u.pending = append(u.pending, data...)
	for len(u.pending) > 0 {
		if u.terminalOSCActive {
			// Quit keys cannot occur in either requested color reply.
			if (u.pending[0] == 'q' || u.pending[0] == 3) && !strings.HasPrefix(string(u.terminalOSC), "52;") {
				u.terminalOSCActive, u.terminalOSC = false, nil
				continue
			}
			end := 0
			if u.pending[0] == 7 {
				end = 1
			} else if u.pending[0] == 27 {
				if len(u.pending) == 1 {
					return
				}
				if u.pending[1] != '\\' {
					u.terminalOSCActive, u.terminalOSC = false, nil
					continue // Preserve a new escape sequence after an interrupted reply.
				}
				end = 2
			}
			if end > 0 {
				if len(u.terminalOSC) <= 128 {
					u.terminalColorReply(string(u.terminalOSC))
				}
				u.terminalOSCActive, u.terminalOSC = false, nil
				u.pending = u.pending[end:]
				continue
			}
			// Keep draining oversized replies without retaining them or dispatching keys.
			if len(u.terminalOSC) <= 128 {
				u.terminalOSC = append(u.terminalOSC, u.pending[0])
			}
			u.pending = u.pending[1:]
			continue
		}
		if len(u.pending) >= 2 && u.pending[0] == 27 && u.pending[1] == ']' {
			u.terminalOSCActive = true
			u.terminalOSCStarted = time.Now()
			u.pending = u.pending[2:]
			continue
		}
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
				u.followPending = nil
				_, err := fmt.Sscanf(s[3:end], "%d;%d;%d", &code, &x, &y)
				if err == nil {
					x--
					y--
					u.routeMouse(code, x, y, s[end] == 'm')
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
				u.followPending = nil
				u.keyboardDimensions()
				if u.listMode || u.combined && !u.graphFocus {
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
			u.followPending = nil
			u.selected = ""
			u.dragging = false
			u.pending = u.pending[1:]
			continue
		}
		u.followPending = nil
		key := u.pending[0]
		u.pending = u.pending[1:]
		if key == '	' {
			u.preview, u.selection = nil, nil
			if u.combined {
				u.combined, u.listMode = false, true
			} else if u.listMode {
				u.listMode = false
			} else {
				u.combined = true
			}
			u.graphFocus = false
			u.dragging = false
			u.splitDragging = false
			u.listSplitDragging = false
			u.dividerHover = 0
			u.scroll = 0
			continue
		}
		u.keyboardDimensions()
		if u.combined && (key == 'n' || key == 'p') {
			if key == 'n' {
				u.cycleGraph(1)
			} else {
				u.cycleGraph(-1)
			}
			continue
		}
		if u.combined && (key == 'J' || key == 'K') {
			if key == 'J' {
				u.scroll++
			} else {
				u.scroll = max(0, u.scroll-1)
			}
			continue
		}
		if (u.listMode || u.combined && !u.graphFocus) && key != 'q' && key != 3 {
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
			u.panX, u.panY = 0, 0
			u.dragging = false
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
	defer u.syncGraphSelection()
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

// Identical output clears native terminal selections even when nothing changed.
func (u *ui) draw(w, h int, now time.Time) string {
	w, h = max(1, w), max(1, h)
	frame := u.render(w, h, now)
	control := u.pendingControl
	u.pendingControl = ""
	if frame == u.lastFrame && w == u.lastFrameWidth && h == u.lastFrameHeight {
		return control
	}
	u.lastFrame, u.lastFrameWidth, u.lastFrameHeight = frame, w, h
	return control + frame
}

func (u *ui) render(w, h int, now time.Time) string {
	w = max(1, w)
	h = max(1, h)
	u.width = w
	u.height = h
	u.terminalWidth, u.terminalHeight = w, h
	u.advanceFollow(now)
	if u.selection != nil && (w != u.lastFrameWidth || h != u.lastFrameHeight || !u.listMode && !u.combined) {
		u.selection = nil
	}
	u.preview = nil
	var c *canvas
	if u.combined {
		c = u.renderCombined(w, h, now)
	} else if u.listMode {
		c = u.renderListCanvas(w, h, now)
	} else {
		c = u.renderGraphCanvas(w, h, now)
	}
	return c.ansi()
}
