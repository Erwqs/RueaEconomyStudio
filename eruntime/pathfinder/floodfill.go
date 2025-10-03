package pathfinder

import "RueaES/typedef"

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

	// Use separate queues for different priority levels
	ownTerritoryQueue := []*typedef.Territory{}     // Own guild territories (highest priority)
	allyTerritoryQueue := []*typedef.Territory{}    // Allied territories (medium priority)
	neutralTerritoryQueue := []*typedef.Territory{} // Neutral/other territories (lowest priority)

	visited := make(map[string]struct{})
	previous := make(map[string]*typedef.Territory)

	// Initialize starting territory
	visited[start.Name] = struct{}{}
	result.Distances[start.Name] = 0
	result.TotalCost[start.Name] = 0
	result.Paths[start.Name] = []*typedef.Territory{start}
	result.ReachableTerritories = append(result.ReachableTerritories, start)

	// Add initial territory to appropriate queue
	neighbors := GetTerritoryConnections(start, territoryMap, tradingRoutes)
	for _, neighbor := range neighbors {
		if _, ok := visited[neighbor.Name]; ok {
			continue
		}

		// Check if we can pass through this territory
		if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
			continue
		}

		// Add to appropriate priority queue
		if neighbor.Guild.Tag == sourceGuildTag {
			ownTerritoryQueue = append(ownTerritoryQueue, neighbor)
		} else if isAllyTerritory(neighbor, allies) {
			allyTerritoryQueue = append(allyTerritoryQueue, neighbor)
		} else {
			neutralTerritoryQueue = append(neutralTerritoryQueue, neighbor)
		}
	}

	// Process queues in priority order
	for len(ownTerritoryQueue) > 0 || len(allyTerritoryQueue) > 0 || len(neutralTerritoryQueue) > 0 {
		var current *typedef.Territory

		// Process own territories first
		if len(ownTerritoryQueue) > 0 {
			current = ownTerritoryQueue[0]
			ownTerritoryQueue = ownTerritoryQueue[1:]
		} else if len(allyTerritoryQueue) > 0 {
			current = allyTerritoryQueue[0]
			allyTerritoryQueue = allyTerritoryQueue[1:]
		} else {
			current = neutralTerritoryQueue[0]
			neutralTerritoryQueue = neutralTerritoryQueue[1:]
		}

		if _, ok := visited[current.Name]; ok {
			continue
		}

		// Find the best previous territory (lowest cost path)
		bestPrevious := findBestPreviousTerritory(current, visited, result.TotalCost, territoryMap, tradingRoutes, sourceGuildTag, allies)
		if bestPrevious == nil {
			continue
		}

		visited[current.Name] = struct{}{}
		previous[current.Name] = bestPrevious
		result.Distances[current.Name] = result.Distances[bestPrevious.Name] + 1

		// Calculate cumulative cost
		edgeCost := calculateFloodFillCost(bestPrevious, current, sourceGuildTag, allies)
		result.TotalCost[current.Name] = result.TotalCost[bestPrevious.Name] + edgeCost

		// Build path to this territory
		result.Paths[current.Name] = buildPath(previous, start, current)

		result.ReachableTerritories = append(result.ReachableTerritories, current)

		// Add neighbors to appropriate queues
		neighbors := GetTerritoryConnections(current, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if _, ok := visited[neighbor.Name]; ok {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			// Add to appropriate priority queue if not already queued
			if !isInQueue(neighbor, ownTerritoryQueue) && !isInQueue(neighbor, allyTerritoryQueue) && !isInQueue(neighbor, neutralTerritoryQueue) {
				if neighbor.Guild.Tag == sourceGuildTag {
					ownTerritoryQueue = append(ownTerritoryQueue, neighbor)
				} else if isAllyTerritory(neighbor, allies) {
					allyTerritoryQueue = append(allyTerritoryQueue, neighbor)
				} else {
					neutralTerritoryQueue = append(neutralTerritoryQueue, neighbor)
				}
			}
		}
	}

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

	// Queue for BFS-style flood fill with distance tracking
	type queueItem struct {
		territory *typedef.Territory
		distance  int
	}

	queue := []queueItem{{territory: start, distance: 0}}
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	// Initialize starting territory
	visited[start.Name] = true
	result.Distances[start.Name] = 0
	result.TotalCost[start.Name] = 0
	result.Paths[start.Name] = []*typedef.Territory{start}
	result.ReachableTerritories = append(result.ReachableTerritories, start)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Skip if we've reached maximum distance
		if current.distance >= maxDistance {
			continue
		}

		// Explore neighbors
		neighbors := GetTerritoryConnections(current.territory, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if visited[neighbor.Name] {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			newDistance := current.distance + 1
			visited[neighbor.Name] = true
			previous[neighbor.Name] = current.territory
			result.Distances[neighbor.Name] = newDistance

			// Calculate cumulative cost
			edgeCost := calculateFloodFillCost(current.territory, neighbor, sourceGuildTag, allies)
			result.TotalCost[neighbor.Name] = result.TotalCost[current.territory.Name] + edgeCost

			// Build path to this territory
			result.Paths[neighbor.Name] = buildPath(previous, start, neighbor)

			result.ReachableTerritories = append(result.ReachableTerritories, neighbor)
			queue = append(queue, queueItem{territory: neighbor, distance: newDistance})
		}
	}

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

	// Queue for BFS-style flood fill with cost tracking
	type queueItem struct {
		territory *typedef.Territory
		distance  int
		cost      float64
	}

	queue := []queueItem{{territory: start, distance: 0, cost: 0}}
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	// Initialize starting territory
	visited[start.Name] = true
	result.Distances[start.Name] = 0
	result.TotalCost[start.Name] = 0
	result.Paths[start.Name] = []*typedef.Territory{start}
	result.ReachableTerritories = append(result.ReachableTerritories, start)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Explore neighbors
		neighbors := GetTerritoryConnections(current.territory, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if visited[neighbor.Name] {
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
			visited[neighbor.Name] = true
			previous[neighbor.Name] = current.territory
			result.Distances[neighbor.Name] = newDistance
			result.TotalCost[neighbor.Name] = newCost

			// Build path to this territory
			result.Paths[neighbor.Name] = buildPath(previous, start, neighbor)

			result.ReachableTerritories = append(result.ReachableTerritories, neighbor)
			queue = append(queue, queueItem{territory: neighbor, distance: newDistance, cost: newCost})
		}
	}

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
func buildPath(previous map[string]*typedef.Territory, start, target *typedef.Territory) []*typedef.Territory {
	path := []*typedef.Territory{}
	current := target

	for current != nil {
		path = append([]*typedef.Territory{current}, path...)
		if current.Name == start.Name {
			break
		}
		current = previous[current.Name]
	}

	return path
}

// isAllyTerritory checks if a territory belongs to an allied guild
func isAllyTerritory(territory *typedef.Territory, allies []string) bool {
	for _, ally := range allies {
		if territory.Guild.Tag == ally {
			return true
		}
	}
	return false
}

// isInQueue checks if a territory is already in a queue
func isInQueue(territory *typedef.Territory, queue []*typedef.Territory) bool {
	for _, t := range queue {
		if t.Name == territory.Name {
			return true
		}
	}
	return false
}

// findBestPreviousTerritory finds the best (lowest cost) path to reach a territory
func findBestPreviousTerritory(target *typedef.Territory, visited map[string]struct{}, costs map[string]float64, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) *typedef.Territory {
	var bestPrevious *typedef.Territory
	var bestCost float64 = -1

	// Check all possible connections to this territory
	for _, territory := range territoryMap {
		if _, ok := visited[territory.Name]; !ok {
			continue
		}

		// Check if this territory connects to target
		connections := GetTerritoryConnections(territory, territoryMap, tradingRoutes)
		connected := false
		for _, conn := range connections {
			if conn.Name == target.Name {
				connected = true
				break
			}
		}

		if connected {
			edgeCost := calculateFloodFillCost(territory, target, sourceGuildTag, allies)
			totalCost := costs[territory.Name] + edgeCost

			if bestCost < 0 || totalCost < bestCost {
				bestCost = totalCost
				bestPrevious = territory
			}
		}
	}

	return bestPrevious
}
