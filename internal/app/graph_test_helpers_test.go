package app

func (g *graph) clusters(w, h int) []fileCluster { return g.packedHierarchy(w, h).clusters }
func (g *graph) folderEnvelopes(w, h int) []folderEnvelope {
	return g.packedHierarchy(w, h).envelopes
}
func (g *graph) step(w, h int, _ float64) {
	w, h = max(1, w), max(1, h)
	g.applyLayout(g.packedHierarchy(w, h), w, h)
}
func (g *graph) stepClusters(w, h int, dt float64) { g.step(w, h, dt) }
