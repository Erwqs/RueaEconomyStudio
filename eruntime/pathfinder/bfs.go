package pathfinder

import (
	"RueaES/typedef"
)

// BFS finds the shortest path between two territories disregarding tax
func BFS(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string) ([]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}

	if start == target {
		return []*typedef.Territory{start}, nil
	}

	// Queue for BFS
	queue := []*typedef.Territory{start}
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	visited[start.Name] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Found target
		if current.Name == target.Name {
			return reconstructPath(previous, start, target), nil
		}

		// Explore neighbors
		neighbors := GetTerritoryConnections(current, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if visited[neighbor.Name] {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			visited[neighbor.Name] = true
			previous[neighbor.Name] = current
			queue = append(queue, neighbor)
		}
	}

	return nil, ErrNoPath
}
