package main

import "math"

// Drag offsets are relative to a leaf zone, so a resize moves them with their
// folder. Invalid offsets are dropped after structural edits rather than
// permitting bubbles to cross hierarchy boundaries or overlap other files.
func (u *ui) validGraphDrag(n *node, x, y float64, cluster fileCluster) bool {
	radius := max(.1, n.radius*cluster.Scale)
	limit := max(0, cluster.Radius-radius-cluster.Scale)
	if math.Hypot(x-cluster.CenterX, 2*(y-cluster.CenterY)) > limit+1e-8 {
		return false
	}
	for _, other := range cluster.Nodes {
		if other == n {
			continue
		}
		separation := radius + max(.1, other.radius*cluster.Scale) + .1*cluster.Scale
		if math.Hypot(x-other.x, 2*(y-other.y)) < separation-1e-8 {
			return false
		}
	}
	return true
}
func (u *ui) tryGraphDrag(n *node, x, y float64) bool {
	if n == nil || n.dir || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return false
	}
	for _, cluster := range u.graph.clusters(u.width, max(1, u.height-4)) {
		if cluster.ID != n.parent || cluster.Radius <= 0 {
			continue
		}
		if !u.validGraphDrag(n, x, y, cluster) {
			return false
		}
		if u.graphDrags == nil {
			u.graphDrags = make(map[string][2]float64)
		}
		u.graphDrags[n.id] = [2]float64{(x - cluster.CenterX) / cluster.Radius, 2 * (y - cluster.CenterY) / cluster.Radius}
		n.x = x
		n.y = y
		n.vx = 0
		n.vy = 0
		return true
	}
	return false
}
func (u *ui) applyGraphDrags(clusters []fileCluster) {
	for _, cluster := range clusters {
		for _, n := range cluster.Nodes {
			offset, ok := u.graphDrags[n.id]
			if !ok {
				continue
			}
			x, y := cluster.CenterX+offset[0]*cluster.Radius, cluster.CenterY+offset[1]*cluster.Radius/2
			if u.validGraphDrag(n, x, y, cluster) {
				n.x = x
				n.y = y
			} else {
				delete(u.graphDrags, n.id)
			}
		}
	}
	for id := range u.graphDrags {
		if u.graph.nodes[id] == nil {
			delete(u.graphDrags, id)
		}
	}
}
