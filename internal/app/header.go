package app

import (
	"fmt"
	"math/big"
	"strings"
)

// sessionEdits sums known leaf line edits, excluding directory and unknown records.
func (u *ui) sessionEdits() [3]*big.Int {
	counts := [3]*big.Int{new(big.Int), new(big.Int), new(big.Int)}
	for _, row := range u.history.entries {
		if row.dir || row.unknown {
			continue
		}
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
	return counts
}

// headerWidths apportions terminal cells exactly using largest remainders;
// ties go to blue, then green, then red.
func headerWidths(width int, counts [3]*big.Int) [3]int {
	var widths [3]int
	total := new(big.Int)
	for _, count := range counts {
		total.Add(total, count)
	}
	if width <= 0 || total.Sign() == 0 {
		return widths
	}
	var rem [3]*big.Int
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
	counts := u.sessionEdits()
	added := new(big.Int).Add(counts[0], counts[1])
	metrics := ""
	for i, count := range []*big.Int{added, counts[2]} {
		if count.Sign() != 0 {
			sign := "+"
			if i == 1 {
				sign = "-"
			}
			label := count.String()
			metrics += " " + sign + label
		}
	}
	// The line-total provider supplies the current tracked/untracked text corpus.
	percent := "?%"
	denominator, known := u.repositoryLineTotal()
	net := new(big.Int).Sub(added, counts[2])
	if known {
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
	header := metrics
	if u.warning != "" && textWidth(header+"  Warning: "+u.warning) <= available {
		header += "  Warning: " + u.warning
	}
	c.text(0, 0, header, gray, available)
	widths := headerWidths(c.w, counts)
	colors := [3]rgb{blue, green, red}
	x := 0
	for i, n := range widths {
		for j := 0; j < n; j++ {
			c.cells[x].bg = colors[i]
			c.cells[x].color = headerInk
			x++
		}
	}
}
