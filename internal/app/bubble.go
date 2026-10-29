package app

func (u *ui) displayRadius(n *node) float64 {
	if radius, ok := u.graphRadii[n.id]; ok {
		return radius
	}
	return max(.5, n.radius)
}

// The minority occupies an area fraction of the bubble, not a split side.
// Deletion overrides earlier new-file status.
func bubblePalette(n *node) (outer, core rgb, coreFraction float64) {
	if n.kind == "deleted" {
		return red, red, 0
	}
	addition := green
	if n.fresh || n.kind == "created" {
		addition = blue
	}
	if n.added > 0 && n.removed > 0 {
		total := float64(n.added) + float64(n.removed)
		if n.removed > n.added {
			return red, addition, float64(n.added) / total
		}
		return addition, red, float64(n.removed) / total
	}
	if n.removed > 0 {
		return red, red, 0
	}
	if n.added > 0 || n.kind == "modified" {
		return addition, addition, 0
	}
	return gray, gray, 0
}
