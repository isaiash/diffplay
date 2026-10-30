package app

import (
	"math"
	"time"
)

const listBurstDuration = time.Second

func listBurstStrength(age time.Duration) float64 {
	if age < 0 || age >= listBurstDuration {
		return 0
	}
	remaining := 1 - float64(age)/float64(listBurstDuration)
	return remaining * remaining
}

func (u *ui) listBurstBackground(base, event rgb, strength float64) rgb {
	if strength <= 0 {
		return base
	}
	target := base
	switch base {
	case background:
		target = u.terminalBackground
		// shortcut: unanswered OSC queries use dark-theme estimates; query support gives exact targets.
		if target == (rgb{}) {
			target = rgb{r: 24, g: 24, b: 24}
		}
	case selectedBackground:
		target = u.terminalSelectedBackground
		if target == (rgb{}) {
			target = rgb{r: 80, g: 80, b: 80}
		}
	}
	pastel := blend(event, white, .45)
	strength = clamp(strength, 0, 1)
	channel := func(a, b int) int { return int(math.Round(float64(a) + float64(b-a)*strength)) }
	color := rgb{r: channel(target.r, pastel.r), g: channel(target.g, pastel.g), b: channel(target.b, pastel.b)}
	color.explicit = color == (rgb{})
	return color
}
