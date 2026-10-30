package app

import (
	"math"
	"path"
	"strings"
)

func folderName(id string) string { return path.Base(id) }

// Project the same camera transform as file glyphs. Clip drawing, not frame
// geometry, so panning an edge offscreen never invents a border at the viewport.
func (u *ui) drawFolderFrame(c *canvas, folder folderEnvelope) {
	color := rgb{r: 128, g: 128, b: 128}
	titleColor := color
	inRange := func(id string) bool {
		return folder.Start != "" && (id == folder.Start || strings.HasPrefix(id, folder.Start+"/")) &&
			(id == folder.ID || strings.HasPrefix(folder.ID, id+"/"))
	}
	mergedFile := false
	if n := u.graph.nodes[u.selected]; n != nil && !n.dir {
		mergedFile = inRange(n.parent)
	}
	if u.selected != "" && (u.selected == folder.ID || strings.HasPrefix(u.selected, folder.ID+"/") || inRange(u.selected) || mergedFile) {
		color, titleColor = white, white
	}
	left, top := u.screen(&node{x: folder.Left, y: folder.Top})
	right, bottom := u.screen(&node{x: folder.Left + folder.Width, y: folder.Top + folder.Height})
	x0, y0 := int(math.Round(left)), int(math.Round(top))
	x1, y1 := int(math.Round(right)), int(math.Round(bottom))
	if x1 <= x0 || y1 <= y0 {
		return
	}
	put := func(x, y int, ch rune) {
		if y >= 2 && y < c.h-2 {
			c.put(x, y, ch, color)
		}
	}
	for x := max(0, x0+1); x < min(c.w, x1); x++ {
		put(x, y0, '─')
		put(x, y1, '─')
	}
	for y := max(2, y0+1); y < min(c.h-2, y1); y++ {
		put(x0, y, '│')
		put(x1, y, '│')
	}
	put(x0, y0, '┌')
	put(x1, y0, '┐')
	put(x0, y1, '└')
	put(x1, y1, '┘')
	if y0 >= 2 && y0 < c.h-2 {
		title := folder.Label
		if title == "" {
			title = u.displayName(folder.ID)
		} else if label, ok := u.repositoryLabels[folder.Start]; ok {
			title = label + strings.TrimPrefix(title, folderName(folder.Start))
		}
		title += "/"
		// Crop by glyph width rather than byte length, preserving Unicode paths.
		x := x0 + 2
		for _, ch := range safeText(title) {
			width := runeWidth(ch)
			if width == 0 {
				continue
			}
			if x+width > x1 {
				break
			}
			if x >= 0 {
				c.text(x, y0, string(ch), titleColor, width)
			}
			x += width
		}
	}
}
