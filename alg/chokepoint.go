package alg

import (
	"errors"
	"sort"

	"RueaES/typedef"
)

// ChokeReport aggregates impact for a single territory acting as a chokepoint.
type ChokeReport struct {
	Importance float64        `json:"importance"`
	Impacts    []SourceImpact `json:"impacts"`
}

// SourceImpact describes how a single source territory is affected when a chokepoint is taken.
type SourceImpact struct {
	Source         string `json:"source"`
	BaselinePaths  int    `json:"baselinePaths"`
	RemainingPaths int    `json:"remainingPaths"`
	LostPaths      int    `json:"lostPaths"`
}

var (
	ErrGuildTagEmpty = errors.New("guild tag is required")
	ErrNoHQForGuild  = errors.New("no HQ found for guild")
)

// ComputeChokepoints calculates chokepoints for a single guild, considering only that guild's territories and links.
// It returns a map keyed by territory name (for that guild) to its choke report. Territories with zero importance are omitted.
// Only own-guild routes are considered; paths through other guilds are ignored. emeraldWeight controls how strongly
// emerald production is weighted relative to other resources when scoring. includeDownstream toggles whether a source's
// weight includes the production of territories downstream (toward the map edge) from that source; when false, only the
// source territory's own production is used for weighting.
func ComputeChokepoints(guildTag string, territories map[string]*typedef.Territory, routes map[string][]string, emeraldWeight float64, includeDownstream bool) (map[string]ChokeReport, error) {
	if guildTag == "" {
		return nil, ErrGuildTagEmpty
	}
	if emeraldWeight <= 0 {
		emeraldWeight = 1
	}

	guildNodes, hqs := collectGuildNodes(guildTag, territories)
	if len(hqs) == 0 {
		return nil, ErrNoHQForGuild
	}

	baseValues := make(map[string]float64, len(guildNodes))
	for name, t := range guildNodes {
		baseValues[name] = territoryProductionValue(t, emeraldWeight)
	}

	weightedValues := baseValues
	if includeDownstream {
		weightedValues = computeDownstreamProduction(guildNodes, routes, hqs, baseValues)
	}

	sourceValues := make(map[string]float64)
	totalSourceValue := 0.0
	for name, t := range guildNodes {
		if t == nil || t.HQ {
			continue
		}
		val := weightedValues[name]
		if val <= 0 {
			val = 1
		}
		sourceValues[name] = val
		totalSourceValue += val
	}

	sources := pickSources(guildNodes)
	if len(sources) == 0 {
		return map[string]ChokeReport{}, nil
	}

	// Precompute baseline robustness for each source.
	baseline := make(map[string]int, len(sources))
	for _, src := range sources {
		baseline[src] = nodeDisjointPathCount(src, hqs, guildNodes, routes, "")
	}

	results := make(map[string]ChokeReport)

	for name, t := range guildNodes {
		if t == nil || t.HQ {
			continue // Do not consider HQ itself as a chokepoint target.
		}

		var impacts []SourceImpact
		var importance float64

		for _, src := range sources {
			base := baseline[src]
			if base <= 0 {
				continue
			}

			srcWeight := sourceValues[src]
			if srcWeight <= 0 {
				srcWeight = 1
			}

			remaining := nodeDisjointPathCount(src, hqs, guildNodes, routes, name)
			if remaining < base {
				lost := base - remaining
				fractionLost := float64(lost) / float64(base)
				importance += fractionLost * srcWeight
				impacts = append(impacts, SourceImpact{
					Source:         src,
					BaselinePaths:  base,
					RemainingPaths: remaining,
					LostPaths:      lost,
				})
			}
		}

		if importance > 0 {
			if totalSourceValue > 0 {
				importance /= totalSourceValue
			}
			results[name] = ChokeReport{
				Importance: importance,
				Impacts:    impacts,
			}
		}
	}

	return results, nil
}

// collectGuildNodes returns guild-owned territories keyed by name, and the list of HQ names.
func collectGuildNodes(guildTag string, territories map[string]*typedef.Territory) (map[string]*typedef.Territory, []string) {
	guildNodes := make(map[string]*typedef.Territory)
	hqs := make([]string, 0)

	for name, t := range territories {
		if t == nil || t.Guild.Tag != guildTag {
			continue
		}
		guildNodes[name] = t
		if t.HQ {
			hqs = append(hqs, name)
		}
	}

	return guildNodes, hqs
}

// pickSources chooses source territories to evaluate: all non-HQ guild territories.
func pickSources(guildNodes map[string]*typedef.Territory) []string {
	sources := make([]string, 0, len(guildNodes))
	for name, t := range guildNodes {
		if t != nil && !t.HQ {
			sources = append(sources, name)
		}
	}
	return sources
}

func territoryProductionValue(t *typedef.Territory, emeraldWeight float64) float64 {
	if t == nil {
		return 0
	}

	t.Mu.RLock()
	gen := t.ResourceGeneration.At
	t.Mu.RUnlock()

	value := gen.Ores + gen.Wood + gen.Fish + gen.Crops
	value += gen.Emeralds * emeraldWeight

	if value <= 0 {
		return 1 // Avoid zero weights so every source still contributes
	}
	return value
}

// computeDownstreamProduction rolls up production for each territory by adding its own production and all production
// in territories further from any HQ (based on shortest-path distance). This approximates downstream weighting so that
// chokepoints near HQ inherit the production of territories beyond them.
func computeDownstreamProduction(guildNodes map[string]*typedef.Territory, routes map[string][]string, hqs []string, base map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(base))
	if len(base) == 0 {
		return result
	}

	// Multi-source BFS from all HQs to get minimum distance to an HQ for each node.
	dist := make(map[string]int, len(guildNodes))
	for name := range guildNodes {
		dist[name] = -1
	}

	queue := make([]string, 0, len(hqs))
	for _, h := range hqs {
		if _, ok := guildNodes[h]; !ok {
			continue
		}
		dist[h] = 0
		queue = append(queue, h)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range routes[cur] {
			if _, ok := guildNodes[nb]; !ok {
				continue
			}
			if dist[nb] == -1 {
				dist[nb] = dist[cur] + 1
				queue = append(queue, nb)
			}
		}
	}

	// Pick a single upstream parent (closest to HQ) for each node to avoid double counting.
	parent := make(map[string]string, len(guildNodes))
	for name := range guildNodes {
		if guildNodes[name] == nil || guildNodes[name].HQ || dist[name] <= 0 {
			continue
		}
		best := ""
		bestDist := dist[name]
		for _, nb := range routes[name] {
			nbDist, ok := dist[nb]
			if !ok || nbDist < 0 {
				continue
			}
			if nbDist >= dist[name] {
				continue // Only consider nodes closer to an HQ
			}
			if best == "" || nbDist < bestDist || (nbDist == bestDist && nb < best) {
				best = nb
				bestDist = nbDist
			}
		}
		if best != "" {
			parent[name] = best
		}
	}

	children := make(map[string][]string, len(guildNodes))
	for child, p := range parent {
		children[p] = append(children[p], child)
	}

	names := make([]string, 0, len(guildNodes))
	for name := range guildNodes {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		di := dist[names[i]]
		dj := dist[names[j]]
		if di == dj {
			return names[i] > names[j]
		}
		return di > dj // larger distance (further downstream) first
	})

	for _, name := range names {
		baseVal := base[name]
		if baseVal <= 0 {
			baseVal = 1
		}
		total := baseVal
		for _, child := range children[name] {
			total += result[child]
		}
		result[name] = total
	}

	return result
}

type edge struct {
	to  int
	rev int
	cap int
}

type graph [][]edge

func addEdge(g graph, from, to, cap int) {
	g[from] = append(g[from], edge{to: to, rev: len(g[to]), cap: cap})
	g[to] = append(g[to], edge{to: from, rev: len(g[from]) - 1, cap: 0})
}

const (
	capInfinity = 1_000_000_000
	nodeCap     = 1
)

// nodeDisjointPathCount computes the maximum number of internally node-disjoint paths
// from a single source to any HQ using only guild-owned nodes. Optional exclude removes
// that node from the graph to simulate capture/loss.
func nodeDisjointPathCount(source string, hqs []string, guildNodes map[string]*typedef.Territory, routes map[string][]string, exclude string) int {
	// Build list of nodes to include.
	nodes := make([]string, 0, len(guildNodes))
	for name := range guildNodes {
		if name == exclude {
			continue
		}
		nodes = append(nodes, name)
	}

	// Quick exits.
	if len(nodes) == 0 {
		return 0
	}
	if _, ok := guildNodes[source]; !ok || source == exclude {
		return 0
	}

	hqSet := make(map[string]struct{}, len(hqs))
	for _, h := range hqs {
		if h != exclude {
			hqSet[h] = struct{}{}
		}
	}
	if len(hqSet) == 0 {
		return 0
	}

	// Map node -> split indices.
	type split struct{ in, out int }
	splits := make(map[string]split, len(nodes))

	// Each node gets two indices; add 2 for super source/sink.
	totalNodes := len(nodes)*2 + 2
	superSource := totalNodes - 2
	superSink := totalNodes - 1
	g := make(graph, totalNodes)

	idx := 0
	for _, name := range nodes {
		in := idx
		out := idx + 1
		idx += 2
		splits[name] = split{in: in, out: out}

		capThrough := nodeCap
		if name == source || isHQ(name, hqSet) {
			capThrough = capInfinity // sources and HQs are not capacity-limited
		}
		addEdge(g, in, out, capThrough)
	}

	// Connect adjacency (guild-only edges) using out->in with high capacity.
	for _, name := range nodes {
		neighbors := routes[name]
		if len(neighbors) == 0 {
			continue
		}
		fromSplit, ok := splits[name]
		if !ok {
			continue
		}
		for _, nb := range neighbors {
			if nb == exclude {
				continue
			}
			if _, ok := splits[nb]; !ok {
				continue
			}
			toSplit := splits[nb]
			addEdge(g, fromSplit.out, toSplit.in, capInfinity)
		}
	}

	// Wire super source and sink.
	if s, ok := splits[source]; ok {
		addEdge(g, superSource, s.in, capInfinity)
	} else {
		return 0
	}

	for h := range hqSet {
		if sp, ok := splits[h]; ok {
			addEdge(g, sp.out, superSink, capInfinity)
		}
	}

	return maxFlow(g, superSource, superSink)
}

func isHQ(name string, hqSet map[string]struct{}) bool {
	_, ok := hqSet[name]
	return ok
}

// Edmonds-Karp max flow (BFS-based) for small graphs.
func maxFlow(g graph, s, t int) int {
	flow := 0
	for {
		prevNode := make([]int, len(g))
		prevEdge := make([]int, len(g))
		for i := range prevNode {
			prevNode[i] = -1
			prevEdge[i] = -1
		}

		queue := []int{s}
		prevNode[s] = s

		for len(queue) > 0 && prevNode[t] == -1 {
			v := queue[0]
			queue = queue[1:]
			for ei, e := range g[v] {
				if e.cap > 0 && prevNode[e.to] == -1 {
					prevNode[e.to] = v
					prevEdge[e.to] = ei
					queue = append(queue, e.to)
					if e.to == t {
						break
					}
				}
			}
		}

		if prevNode[t] == -1 {
			break // no augmenting path
		}

		// Find bottleneck
		bottleneck := capInfinity
		v := t
		for v != s {
			u := prevNode[v]
			eIdx := prevEdge[v]
			if eIdx < 0 {
				bottleneck = 0
				break
			}
			if g[u][eIdx].cap < bottleneck {
				bottleneck = g[u][eIdx].cap
			}
			v = u
		}
		if bottleneck == 0 || bottleneck == capInfinity {
			// capInfinity may appear if graph is tiny; still fine to push.
		}

		// Apply flow
		v = t
		for v != s {
			u := prevNode[v]
			eIdx := prevEdge[v]
			rev := g[u][eIdx].rev
			g[u][eIdx].cap -= bottleneck
			g[v][rev].cap += bottleneck
			v = u
		}

		flow += bottleneck
	}

	return flow
}
