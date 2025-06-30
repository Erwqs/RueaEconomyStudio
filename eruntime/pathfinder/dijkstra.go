package pathfinder

import (
	"container/heap"
	"etools/typedef"
)

// Dijkstra finds the cheapest path between two territories considering tax and guild preferences
func Dijkstra(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}

	if start == target {
		return []*typedef.Territory{start}, nil
	}

	// Priority queue for Dijkstra's algorithm
	pq := &PriorityQueue{}
	heap.Init(pq)

	// Maps to track visited nodes and distances
	distances := make(map[string]float64)
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	// Initialize starting node
	startNode := &PathfindingNode{
		Territory: start,
		Cost:      0,
		Distance:  0,
		Previous:  nil,
	}
	heap.Push(pq, startNode)
	distances[start.Name] = 0

	for pq.Len() > 0 {
		currentNode := heap.Pop(pq).(*PathfindingNode)
		current := currentNode.Territory

		if visited[current.Name] {
			continue
		}
		visited[current.Name] = true

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

			// Calculate cost for cheapest routing
			edgeCost := calculateCheapestCost(current, neighbor, sourceGuildTag, allies)
			newDistance := distances[current.Name] + edgeCost

			if existingDistance, exists := distances[neighbor.Name]; !exists || newDistance < existingDistance {
				distances[neighbor.Name] = newDistance
				previous[neighbor.Name] = current

				neighborNode := &PathfindingNode{
					Territory: neighbor,
					Cost:      newDistance,
					Distance:  currentNode.Distance + 1,
					Previous:  currentNode,
				}
				heap.Push(pq, neighborNode)
			}
		}
	}

	return nil, ErrNoPath
}

// calculateCheapestCost calculates the cost for cheapest routing mode
func calculateCheapestCost(from, to *typedef.Territory, sourceGuildTag string, allies []string) float64 {
	// Base cost - prefer own guild territories
	cost := 1.0

	// If the destination territory is not owned by the same guild, add tax cost
	if to.Guild.Tag != sourceGuildTag {
		tax := GetTaxForTerritory(to, sourceGuildTag, allies)
		cost += tax * 10.0 // Multiply by 10 to make tax significant in pathfinding
	} else {
		// Prefer own guild territories by giving them lower cost
		cost = 0.1
	}

	// Additional penalty for passing through non-allied territories
	if to.Guild.Tag != sourceGuildTag {
		isAlly := false
		for _, ally := range allies {
			if to.Guild.Tag == ally {
				isAlly = true
				break
			}
		}
		if !isAlly {
			cost += 5.0 // Heavy penalty for non-allied territories
		}
	}

	return cost
}

// reconstructPath rebuilds the path from the previous map
func reconstructPath(previous map[string]*typedef.Territory, start, target *typedef.Territory) []*typedef.Territory {
	path := []*typedef.Territory{}
	current := target

	for current != nil {
		path = append([]*typedef.Territory{current}, path...)
		current = previous[current.Name]
	}

	return path
}
