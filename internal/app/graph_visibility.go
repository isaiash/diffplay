package app

import "path"

// Keep scanned context in the model, but expose only files directly inside a
// changed folder or one of its ancestors. A quiet child folder stays hidden
// even when its parent contains changes; its first real event reveals it.
func (g *graph) visibleFiles(changedOnly ...bool) []*node {
	active := map[string]bool{".": true, "": true}
	for _, n := range g.list() {
		if n.dir || n.kind == "" && n.added+n.removed == 0 && !n.unknown {
			continue
		}
		p := n.parent
		for {
			active[p] = true
			if p == "." {
				break
			}
			p = path.Dir(p)
		}
	}
	var files []*node
	for _, n := range g.list() {
		if len(changedOnly) > 0 && changedOnly[0] && n.kind == "" && n.added+n.removed == 0 && !n.unknown {
			continue
		}
		if !n.dir && active[n.parent] {
			files = append(files, n)
		}
	}
	return files
}
