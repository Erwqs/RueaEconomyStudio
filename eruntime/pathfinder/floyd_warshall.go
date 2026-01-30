package pathfinder

import (
	"RueaES/typedef"
	"math"
)

// FloydWarshall finds the cheapest path between two territories using Floyd-Warshall.
func FloydWarshall(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}

	if start == target {
		return []*typedef.Territory{start}, nil
	}

	reachable := reachableTerritories(start, territoryMap, tradingRoutes, sourceGuildTag)
	if _, ok := reachable[target.Name]; !ok {
		return nil, ErrNoPath
	}

	const fwMaxNodes = 400
	if len(reachable) > fwMaxNodes {
		return Dijkstra(start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	}

	territories, indexOf := buildTerritoryIndex(reachable)
	startIndex, ok := indexOf[start.Name]
	if !ok {
		return nil, ErrNoPath
	}
	targetIndex, ok := indexOf[target.Name]
	if !ok {
		return nil, ErrNoPath
	}

	n := len(territories)
	dist := make([][]float64, n)
	next := make([][]int, n)
	for i := 0; i < n; i++ {
		dist[i] = make([]float64, n)
		next[i] = make([]int, n)
		for j := 0; j < n; j++ {
			dist[i][j] = math.Inf(1)
			next[i][j] = -1
		}
		dist[i][i] = 0
		next[i][i] = i
	}

	for fromName, from := range reachable {
		if from == nil {
			continue
		}
		fromIndex, ok := indexOf[fromName]
		if !ok {
			continue
		}
		routes := tradingRoutes[fromName]
		for _, toName := range routes {
			to, ok := reachable[toName]
			if !ok || to == nil {
				continue
			}
			if !CanPassThroughTerritory(to, sourceGuildTag) {
				continue
			}
			toIndex, ok := indexOf[toName]
			if !ok {
				continue
			}
			cost := calculateCheapestCost(from, to, sourceGuildTag, allies)
			if cost < dist[fromIndex][toIndex] {
				dist[fromIndex][toIndex] = cost
				next[fromIndex][toIndex] = toIndex
			}
		}
	}

	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			if dist[i][k] == math.Inf(1) {
				continue
			}
			for j := 0; j < n; j++ {
				if dist[k][j] == math.Inf(1) {
					continue
				}
				alt := dist[i][k] + dist[k][j]
				if alt < dist[i][j] {
					dist[i][j] = alt
					next[i][j] = next[i][k]
				}
			}
		}
	}

	if next[startIndex][targetIndex] == -1 {
		return nil, ErrNoPath
	}

	path := []*typedef.Territory{territories[startIndex]}
	current := startIndex
	for current != targetIndex {
		current = next[current][targetIndex]
		if current < 0 || current >= n {
			return nil, ErrNoPath
		}
		path = append(path, territories[current])
	}

	return path, nil
}

func buildTerritoryIndex(territoryMap map[string]*typedef.Territory) ([]*typedef.Territory, map[string]int) {
	territories := make([]*typedef.Territory, 0, len(territoryMap))
	indexOf := make(map[string]int, len(territoryMap))
	for name, territory := range territoryMap {
		if territory == nil {
			continue
		}
		indexOf[name] = len(territories)
		territories = append(territories, territory)
	}
	return territories, indexOf
}
