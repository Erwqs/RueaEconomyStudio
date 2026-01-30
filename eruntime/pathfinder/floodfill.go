package pathfinder

import (
	"RueaES/typedef"
	"container/heap"
)

// FloodFillResult contains the results of a flood fill operation
type FloodFillResult struct {
	ReachableTerritories []*typedef.Territory            // All territories reachable from start
	Distances            map[string]int                  // Distance from start to each territory
	TotalCost            map[string]float64              // Total cost to reach each territory
	Paths                map[string][]*typedef.Territory // Path from start to each territory
}

// FloodFill finds all territories reachable from a starting territory
// Returns territories organized by distance/cost with their paths
// Uses priority-based exploration to prefer own territories and avoid taxed routes
func FloodFill(start *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) (*FloodFillResult, error) {
	if start == nil {
		return nil, ErrNilTerritory
	}

	result := &FloodFillResult{
		ReachableTerritories: []*typedef.Territory{},
		Distances:            make(map[string]int),
		TotalCost:            make(map[string]float64),
		Paths:                make(map[string][]*typedef.Territory),
	}

	adjacency := buildAdjacency(territoryMap, tradingRoutes)

	bestCost := map[string]float64{start.Name: 0}
	bestDistance := map[string]int{start.Name: 0}
	previous := make(map[string]*typedef.Territory)
	settled := make(map[string]struct{})

	priorityQueue := &PriorityQueue{}
	heap.Init(priorityQueue)
	heap.Push(priorityQueue, &PathfindingNode{Territory: start, Cost: 0, Distance: 0})

	for priorityQueue.Len() > 0 {
		node := heap.Pop(priorityQueue).(*PathfindingNode)
		current := node.Territory

		if current == nil {
			continue
		}

		currentCost, ok := bestCost[current.Name]
		if !ok || node.Cost > currentCost {
			continue
		}

		if _, ok := settled[current.Name]; ok {
			continue
		}
		settled[current.Name] = struct{}{}

		result.ReachableTerritories = append(result.ReachableTerritories, current)
		result.Distances[current.Name] = bestDistance[current.Name]
		result.TotalCost[current.Name] = currentCost

		neighbors := adjacency[current.Name]
		for _, neighbor := range neighbors {
			if neighbor == nil {
				continue
			}

			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			edgeCost := calculateFloodFillCost(current, neighbor, sourceGuildTag, allies)
			newCost := currentCost + edgeCost
			oldCost, ok := bestCost[neighbor.Name]
			if !ok || newCost < oldCost {
				bestCost[neighbor.Name] = newCost
				bestDistance[neighbor.Name] = bestDistance[current.Name] + 1
				previous[neighbor.Name] = current
				heap.Push(priorityQueue, &PathfindingNode{Territory: neighbor, Cost: newCost, Distance: bestDistance[neighbor.Name]})
			}
		}
	}

	result.Paths = buildPathsForAll(start, previous, territoryMap)

	return result, nil
}

// FloodFillWithMaxDistance finds all territories reachable within a maximum distance
func FloodFillWithMaxDistance(start *typedef.Territory, maxDistance int, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) (*FloodFillResult, error) {
	if start == nil {
		return nil, ErrNilTerritory
	}

	result := &FloodFillResult{
		ReachableTerritories: []*typedef.Territory{},
		Distances:            make(map[string]int),
		TotalCost:            make(map[string]float64),
		Paths:                make(map[string][]*typedef.Territory),
	}

	adjacency := buildAdjacency(territoryMap, tradingRoutes)

	// Queue for BFS-style flood fill with distance tracking
	type queueItem struct {
		territory *typedef.Territory
		distance  int
	}

	queue := []queueItem{{territory: start, distance: 0}}
	queueHead := 0
	visited := make(map[string]struct{})
	previous := make(map[string]*typedef.Territory)

	// Initialize starting territory
	visited[start.Name] = struct{}{}
	result.Distances[start.Name] = 0
	result.TotalCost[start.Name] = 0
	result.ReachableTerritories = append(result.ReachableTerritories, start)

	for queueHead < len(queue) {
		current := queue[queueHead]
		queueHead++

		// Skip if we've reached maximum distance
		if current.distance >= maxDistance {
			continue
		}

		// Explore neighbors
		neighbors := adjacency[current.territory.Name]
		for _, neighbor := range neighbors {
			if neighbor == nil {
				continue
			}
			if _, ok := visited[neighbor.Name]; ok {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			newDistance := current.distance + 1
			visited[neighbor.Name] = struct{}{}
			previous[neighbor.Name] = current.territory
			result.Distances[neighbor.Name] = newDistance

			// Calculate cumulative cost
			edgeCost := calculateFloodFillCost(current.territory, neighbor, sourceGuildTag, allies)
			result.TotalCost[neighbor.Name] = result.TotalCost[current.territory.Name] + edgeCost

			result.ReachableTerritories = append(result.ReachableTerritories, neighbor)
			queue = append(queue, queueItem{territory: neighbor, distance: newDistance})
		}
	}

	result.Paths = buildPathsForAll(start, previous, territoryMap)

	return result, nil
}

// FloodFillWithMaxCost finds all territories reachable within a maximum cost threshold
func FloodFillWithMaxCost(start *typedef.Territory, maxCost float64, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) (*FloodFillResult, error) {
	if start == nil {
		return nil, ErrNilTerritory
	}

	result := &FloodFillResult{
		ReachableTerritories: []*typedef.Territory{},
		Distances:            make(map[string]int),
		TotalCost:            make(map[string]float64),
		Paths:                make(map[string][]*typedef.Territory),
	}

	adjacency := buildAdjacency(territoryMap, tradingRoutes)

	// Queue for BFS-style flood fill with cost tracking
	type queueItem struct {
		territory *typedef.Territory
		distance  int
		cost      float64
	}

	queue := []queueItem{{territory: start, distance: 0, cost: 0}}
	queueHead := 0
	visited := make(map[string]struct{})
	previous := make(map[string]*typedef.Territory)

	// Initialize starting territory
	visited[start.Name] = struct{}{}
	result.Distances[start.Name] = 0
	result.TotalCost[start.Name] = 0
	result.ReachableTerritories = append(result.ReachableTerritories, start)

	for queueHead < len(queue) {
		current := queue[queueHead]
		queueHead++

		// Explore neighbors
		neighbors := adjacency[current.territory.Name]
		for _, neighbor := range neighbors {
			if neighbor == nil {
				continue
			}
			if _, ok := visited[neighbor.Name]; ok {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			// Calculate cost to reach this neighbor
			edgeCost := calculateFloodFillCost(current.territory, neighbor, sourceGuildTag, allies)
			newCost := current.cost + edgeCost

			// Skip if cost exceeds maximum
			if newCost > maxCost {
				continue
			}

			newDistance := current.distance + 1
			visited[neighbor.Name] = struct{}{}
			previous[neighbor.Name] = current.territory
			result.Distances[neighbor.Name] = newDistance
			result.TotalCost[neighbor.Name] = newCost

			result.ReachableTerritories = append(result.ReachableTerritories, neighbor)
			queue = append(queue, queueItem{territory: neighbor, distance: newDistance, cost: newCost})
		}
	}

	result.Paths = buildPathsForAll(start, previous, territoryMap)

	return result, nil
}

// FloodFillByGuild finds all territories reachable from start, organized by guild ownership
func FloodFillByGuild(start *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) (map[string]*FloodFillResult, error) {
	if start == nil {
		return nil, ErrNilTerritory
	}

	// Get all reachable territories first
	allReachable, err := FloodFill(start, territoryMap, tradingRoutes, sourceGuildTag, allies)
	if err != nil {
		return nil, err
	}

	// Organize by guild
	resultsByGuild := make(map[string]*FloodFillResult)

	for _, territory := range allReachable.ReachableTerritories {
		guildTag := territory.Guild.Tag
		if guildTag == "" {
			guildTag = "NONE"
		}

		if resultsByGuild[guildTag] == nil {
			resultsByGuild[guildTag] = &FloodFillResult{
				ReachableTerritories: []*typedef.Territory{},
				Distances:            make(map[string]int),
				TotalCost:            make(map[string]float64),
				Paths:                make(map[string][]*typedef.Territory),
			}
		}

		result := resultsByGuild[guildTag]
		result.ReachableTerritories = append(result.ReachableTerritories, territory)
		result.Distances[territory.Name] = allReachable.Distances[territory.Name]
		result.TotalCost[territory.Name] = allReachable.TotalCost[territory.Name]
		result.Paths[territory.Name] = allReachable.Paths[territory.Name]
	}

	return resultsByGuild, nil
}

func buildAdjacency(territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string) map[string][]*typedef.Territory {
	adjacency := make(map[string][]*typedef.Territory, len(territoryMap))
	for territoryName, territory := range territoryMap {
		if territory == nil {
			continue
		}
		routes := tradingRoutes[territoryName]
		if len(routes) == 0 {
			continue
		}

		neighbors := make([]*typedef.Territory, 0, len(routes))
		for _, routeName := range routes {
			if neighbor, ok := territoryMap[routeName]; ok && neighbor != nil {
				neighbors = append(neighbors, neighbor)
			}
		}
		if len(neighbors) > 0 {
			adjacency[territoryName] = neighbors
		}
	}

	return adjacency
}

func buildPathsForAll(start *typedef.Territory, previous map[string]*typedef.Territory, territoryMap map[string]*typedef.Territory) map[string][]*typedef.Territory {
	paths := make(map[string][]*typedef.Territory)
	if start == nil {
		return paths
	}

	paths[start.Name] = []*typedef.Territory{start}

	var buildPathFor func(name string) []*typedef.Territory
	buildPathFor = func(name string) []*typedef.Territory {
		if path, ok := paths[name]; ok {
			return path
		}

		current := territoryMap[name]
		if current == nil {
			return nil
		}

		prev := previous[name]
		if prev == nil {
			return nil
		}

		prevPath := buildPathFor(prev.Name)
		if prevPath == nil {
			return nil
		}

		path := append(make([]*typedef.Territory, 0, len(prevPath)+1), prevPath...)
		path = append(path, current)
		paths[name] = path
		return path
	}

	for name := range previous {
		buildPathFor(name)
	}

	return paths
}

// calculateFloodFillCost calculates the cost for flood fill operations
func calculateFloodFillCost(from, to *typedef.Territory, sourceGuildTag string, allies []string) float64 {
	// Base cost
	cost := 1.0

	// If the destination territory is not owned by the same guild, add tax cost
	if to.Guild.Tag != sourceGuildTag {
		tax := GetTaxForTerritory(to, sourceGuildTag, allies)
		cost += tax * 5.0 // Multiply by 5 for flood fill cost calculation
	} else {
		// Own guild territories have minimal cost
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
			cost += 2.0 // Moderate penalty for non-allied territories
		}
	}

	return cost
}

// buildPath reconstructs the path from start to target using the previous map
