package kb

import "sort"

// Communities groups the graph's nodes with the Leiden algorithm (Traag,
// Waltman & van Eck 2019, "From Louvain to Leiden"), maximising modularity
// over all edges treated as undirected. It returns a community number per node
// key: 0 is the largest community. resolution <= 0 means 1; higher values give
// more, smaller communities.
func (s *Store) Communities(resolution float64) (map[string]int, error) {
	nodes, err := s.List(Filter{})
	if err != nil {
		return nil, err
	}
	links, err := s.AllLinks()
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(nodes))
	for i, n := range nodes {
		keys[i] = n.Key
	}
	return Leiden(keys, links, resolution), nil
}

// Leiden partitions keys by the links between them (see Store.Communities).
// It is deterministic: nodes are visited in key order and ties keep the
// current community, so the same graph always gets the same numbering.
func Leiden(keys []string, links []Link, resolution float64) map[string]int {
	if resolution <= 0 {
		resolution = 1
	}
	keys = append([]string(nil), keys...)
	sort.Strings(keys)
	idx := make(map[string]int, len(keys))
	uniq := keys[:0]
	for _, k := range keys {
		if _, ok := idx[k]; !ok {
			idx[k] = len(uniq)
			uniq = append(uniq, k)
		}
	}
	keys = uniq
	n := len(keys)
	w := make([]map[int]float64, n)
	for i := range w {
		w[i] = map[int]float64{}
	}
	for _, l := range links {
		u, ok1 := idx[l.From]
		v, ok2 := idx[l.To]
		if !ok1 || !ok2 || u == v {
			continue
		}
		w[u][v]++
		w[v][u]++
	}
	member := leiden(newWGraph(w), resolution)

	// Number communities by size (largest first), then by first key.
	size := map[int]int{}
	first := map[int]int{}
	for i, c := range member {
		if size[c] == 0 {
			first[c] = i
		}
		size[c]++
	}
	order := make([]int, 0, len(size))
	for c := range size {
		order = append(order, c)
	}
	sort.Slice(order, func(a, b int) bool {
		if size[order[a]] != size[order[b]] {
			return size[order[a]] > size[order[b]]
		}
		return first[order[a]] < first[order[b]]
	})
	rank := make(map[int]int, len(order))
	for r, c := range order {
		rank[c] = r
	}
	out := make(map[string]int, n)
	for i, k := range keys {
		out[k] = rank[member[i]]
	}
	return out
}

const leidenEps = 1e-12

type wedge struct {
	to int
	w  float64
}

// wgraph is a weighted undirected graph. adj excludes self loops, which are in
// self; deg[i] = self[i] + sum of adj[i]; total = sum of deg (= 2m).
type wgraph struct {
	adj   [][]wedge
	self  []float64
	deg   []float64
	total float64
}

func newWGraph(w []map[int]float64) *wgraph {
	n := len(w)
	g := &wgraph{adj: make([][]wedge, n), self: make([]float64, n), deg: make([]float64, n)}
	for i, row := range w {
		for j, x := range row {
			if i == j {
				g.self[i] = x
			} else {
				g.adj[i] = append(g.adj[i], wedge{j, x})
			}
			g.deg[i] += x
		}
		sort.Slice(g.adj[i], func(a, b int) bool { return g.adj[i][a].to < g.adj[i][b].to })
		g.total += g.deg[i]
	}
	return g
}

// leiden returns a community id per node of g.
func leiden(g *wgraph, gamma float64) []int {
	n0 := len(g.deg)
	level := make([]int, n0) // original node -> node of the current (aggregated) graph
	for i := range level {
		level[i] = i
	}
	part := make([]int, n0)
	for i := range part {
		part[i] = i
	}
	for iter := 0; iter < 100; iter++ {
		n := len(g.deg)
		moveNodesFast(g, part, gamma)
		if normalize(part) == n {
			break
		}
		refined := refine(g, part, gamma)
		by := refined
		k := normalize(refined)
		if k == n { // refinement merged nothing: aggregate by the partition itself
			by = append([]int(nil), part...)
			k = normalize(by)
		}
		w := make([]map[int]float64, k)
		for c := range w {
			w[c] = map[int]float64{}
		}
		next := make([]int, k)
		for i := 0; i < n; i++ {
			c := by[i]
			if g.self[i] != 0 {
				w[c][c] += g.self[i]
			}
			for _, e := range g.adj[i] {
				w[c][by[e.to]] += e.w
			}
			next[c] = part[i] // aggregated nodes start in their community
		}
		normalize(next)
		for o := range level {
			level[o] = by[level[o]]
		}
		g, part = newWGraph(w), next
	}
	out := make([]int, n0)
	for o := range out {
		out[o] = part[level[o]]
	}
	return out
}

// normalize renumbers ids to 0..k-1 in order of first appearance and returns k.
func normalize(ids []int) int {
	m := map[int]int{}
	for i, c := range ids {
		r, ok := m[c]
		if !ok {
			r = len(m)
			m[c] = r
		}
		ids[i] = r
	}
	return len(m)
}

// moveNodesFast moves single nodes to the neighbouring community with the
// best modularity gain, revisiting only neighbours of nodes that moved.
// part ids must be in 0..n-1.
func moveNodesFast(g *wgraph, part []int, gamma float64) {
	n := len(g.deg)
	if g.total == 0 {
		return
	}
	commW := make([]float64, n)
	size := make([]int, n)
	for i := 0; i < n; i++ {
		commW[part[i]] += g.deg[i]
		size[part[i]]++
	}
	var empty []int
	for c := n - 1; c >= 0; c-- {
		if size[c] == 0 {
			empty = append(empty, c)
		}
	}
	queue := make([]int, n)
	inQ := make([]bool, n)
	for i := range queue {
		queue[i] = i
		inQ[i] = true
	}
	neighW := make([]float64, n)
	seen := make([]bool, n)
	var touched []int
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		inQ[i] = false
		ci := part[i]

		touched = touched[:0]
		for _, e := range g.adj[i] {
			c := part[e.to]
			if !seen[c] {
				seen[c] = true
				touched = append(touched, c)
			}
			neighW[c] += e.w
		}
		commW[ci] -= g.deg[i]
		size[ci]--
		gain := func(c int) float64 { return neighW[c] - gamma*g.deg[i]*commW[c]/g.total }
		best, bestGain := ci, gain(ci)
		for _, c := range touched {
			if c != ci {
				if x := gain(c); x > bestGain+leidenEps {
					best, bestGain = c, x
				}
			}
		}
		if bestGain < -leidenEps && size[ci] > 0 { // alone is better
			best = empty[len(empty)-1]
			empty = empty[:len(empty)-1]
		}
		commW[best] += g.deg[i]
		size[best]++
		for _, c := range touched {
			neighW[c], seen[c] = 0, false
		}
		if best == ci {
			continue
		}
		if size[ci] == 0 {
			empty = append(empty, ci)
		}
		part[i] = best
		for _, e := range g.adj[i] {
			if j := e.to; !inQ[j] && part[j] != best {
				inQ[j] = true
				queue = append(queue, j)
			}
		}
	}
}

// refine splits each community of part into well-connected subcommunities,
// starting from singletons and greedily merging nodes within the community.
// This is the step that guarantees Leiden communities are connected.
func refine(g *wgraph, part []int, gamma float64) []int {
	n := len(g.deg)
	ref := make([]int, n)
	for i := range ref {
		ref[i] = i
	}
	if g.total == 0 {
		return ref
	}
	commW := make([]float64, n) // K_C per community of part
	for i := 0; i < n; i++ {
		commW[part[i]] += g.deg[i]
	}
	refW := append([]float64(nil), g.deg...) // K_T per refined community
	refSize := make([]int, n)
	ext := make([]float64, n) // E(T, C \ T)
	for i := 0; i < n; i++ {
		refSize[i] = 1
		for _, e := range g.adj[i] {
			if part[e.to] == part[i] {
				ext[i] += e.w
			}
		}
	}
	wellConnected := func(t, c int) bool {
		return ext[t] >= gamma*refW[t]*(commW[c]-refW[t])/g.total-leidenEps
	}
	neighW := make([]float64, n)
	seen := make([]bool, n)
	var touched []int
	for i := 0; i < n; i++ {
		c := part[i]
		if refSize[ref[i]] != 1 || !wellConnected(i, c) {
			continue
		}
		touched = touched[:0]
		for _, e := range g.adj[i] {
			if part[e.to] != c {
				continue
			}
			t := ref[e.to]
			if !seen[t] {
				seen[t] = true
				touched = append(touched, t)
			}
			neighW[t] += e.w
		}
		best, bestGain := -1, -leidenEps
		for _, t := range touched {
			if t == i || !wellConnected(t, c) {
				continue
			}
			if x := neighW[t] - gamma*g.deg[i]*refW[t]/g.total; x > bestGain+leidenEps {
				best, bestGain = t, x
			}
		}
		if best >= 0 {
			ext[best] += ext[i] - 2*neighW[best]
			refW[best] += g.deg[i]
			refSize[best]++
			refSize[i]--
			ref[i] = best
		}
		for _, t := range touched {
			neighW[t], seen[t] = 0, false
		}
	}
	return ref
}
