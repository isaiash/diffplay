package main

import (
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
)

// sessionEdits counts only leaf records, not the directory aggregates in the tree.
func (u *ui) sessionEdits() ([3]*big.Int, bool) {
	counts := [3]*big.Int{new(big.Int), new(big.Int), new(big.Int)}
	unknown := false
	for _, row := range u.history.entries {
		if row.dir {
			continue
		}
		unknown = unknown || row.unknown
		group := 1
		if row.fresh {
			group = 0
		}
		if row.added > 0 {
			counts[group].Add(counts[group], big.NewInt(int64(row.added)))
		}
		if row.removed > 0 {
			counts[2].Add(counts[2], big.NewInt(int64(row.removed)))
		}
	}
	return counts, unknown
}

// headerWidths apportions terminal cells exactly using largest remainders;
// ties go to blue, then green, then red.
func headerWidths(width int, counts [4]*big.Int) [4]int {
	var widths [4]int
	total := new(big.Int)
	for _, count := range counts {
		total.Add(total, count)
	}
	if width <= 0 || total.Sign() == 0 {
		return widths
	}
	var rem [4]*big.Int
	used := 0
	for i := range counts {
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(new(big.Int).Mul(counts[i], big.NewInt(int64(width))), total, remainder)
		widths[i], rem[i] = int(quotient.Int64()), remainder
		used += widths[i]
	}
	for used < width {
		best := 0
		for i := 1; i < len(rem); i++ {
			if rem[i].Cmp(rem[best]) > 0 {
				best = i
			}
		}
		widths[best]++
		rem[best].SetInt64(-1)
		used++
	}
	return widths
}

// headerPath keeps the rightmost path components without splitting wide runes.
func headerPath(path string, available int) string {
	if available <= 0 {
		return ""
	}
	path = safeText(path)
	if textWidth(path) <= available {
		return path
	}
	if available == 1 {
		return "…"
	}
	runes := []rune(path)
	start, used := len(runes), 0
	for start > 0 {
		width := runeWidth(runes[start-1])
		if used+width > available-1 {
			break
		}
		start--
		used += width
	}
	return "…" + string(runes[start:])
}

// renderSessionHeader paints the list pane's shared first row.
func (u *ui) renderSessionHeader(c *canvas) {
	if c.h <= 0 || c.w <= 0 {
		return
	}
	counts, unknown := u.sessionEdits()
	added := new(big.Int).Add(counts[0], counts[1])
	metrics := ""
	for i, count := range []*big.Int{added, counts[2]} {
		if count.Sign() != 0 || unknown {
			sign := "+"
			if i == 1 {
				sign = "-"
			}
			label := count.String()
			if unknown {
				if count.Sign() == 0 {
					label = "?"
				} else {
					label += "?"
				}
			}
			metrics += " " + sign + label
		}
	}
	// The line-total provider supplies the current tracked/untracked text corpus.
	percent := "?%"
	denominator, known := u.repositoryLineTotal()
	net := new(big.Int).Sub(added, counts[2])
	if known && !unknown {
		if denominator > 0 {
			ratio := new(big.Rat).SetFrac(new(big.Int).Mul(net, big.NewInt(100)), big.NewInt(int64(denominator)))
			percent = ratio.FloatString(2) + "%"
		} else if net.Sign() == 0 {
			percent = "0.00%"
		}
	}
	percent += " new lines"
	totalLabel := "? total lines"
	if known {
		totalLabel = fmt.Sprintf("%d total lines", denominator)
	}
	available := c.w
	if textWidth(totalLabel)+1+textWidth(percent) <= c.w {
		available -= textWidth(totalLabel) + 1
		c.text(c.w-textWidth(totalLabel), 0, totalLabel, gray, textWidth(totalLabel))
	}
	metrics = strings.TrimSpace(metrics)
	if textWidth(metrics)+1+textWidth(percent) > available {
		metrics = ""
	}
	if metrics != "" {
		metrics += " "
	}
	metrics += percent
	name := headerPath(filepath.Base(u.root), available-textWidth(metrics)-1)
	header := metrics
	if name != "" {
		header = name + " " + header
	}
	if u.warning != "" && textWidth(header+"  Warning: "+u.warning) <= available {
		header += "  Warning: " + u.warning
	}
	c.text(0, 0, header, gray, available)
	if known && !unknown && denominator > 0 {
		positiveNet := new(big.Int).Set(net)
		if positiveNet.Sign() < 0 {
			positiveNet.SetInt64(0)
		}
		neutral := new(big.Int).Sub(big.NewInt(int64(denominator)), positiveNet)
		neutral.Sub(neutral, counts[2])
		if neutral.Sign() < 0 {
			// shortcut: cumulative edits can exceed the corpus; normalize colors until distinct-line coverage is tracked.
			neutral.SetInt64(0)
		}
		scale := new(big.Int).Set(added)
		if scale.Sign() == 0 {
			scale.SetInt64(1)
		}
		weights := [4]*big.Int{
			new(big.Int).Mul(positiveNet, counts[0]),
			new(big.Int).Mul(positiveNet, counts[1]),
			new(big.Int).Mul(counts[2], scale),
			new(big.Int).Mul(neutral, scale),
		}
		widths := headerWidths(c.w, weights)
		colors := [3]rgb{blue, green, red}
		x := 0
		for i, n := range widths[:3] {
			for j := 0; j < n; j++ {
				c.cells[x].bg = colors[i]
				c.cells[x].color = headerInk
				x++
			}
		}
	}
}
