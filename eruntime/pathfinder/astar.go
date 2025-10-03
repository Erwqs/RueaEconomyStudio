package pathfinder

import (
	"RueaES/typedef"
	"container/heap"
)

// Astar finds the shortest path between two territories using A* algorithm with graph-based heuristic
func Astar(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}

	if start == target {
		return []*typedef.Territory{start}, nil
	}

	// Priority queue for A* algorithm
	pq := &AstarPriorityQueue{}
	heap.Init(pq)

	// Maps to track costs and visited nodes
	gScore := make(map[string]float64)  // Cost from start to node
	fScore := make(map[string]float64)  // gScore + heuristic
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	// Precompute heuristic distances from target using BFS
	heuristic := computeHeuristic(target, territoryMap, tradingRoutes, sourceGuildTag)

	// Initialize starting node
	startNode := &AstarNode{
		Territory: start,
		GScore:    0,
		FScore:    heuristic[start.Name],
	}
	heap.Push(pq, startNode)
	gScore[start.Name] = 0
	fScore[start.Name] = heuristic[start.Name]

	for pq.Len() > 0 {
		currentNode := heap.Pop(pq).(*AstarNode)
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

			// Calculate cost for A* routing
			edgeCost := calculateAstarCost(current, neighbor, sourceGuildTag, allies)
			tentativeGScore := gScore[current.Name] + edgeCost

			if existingGScore, exists := gScore[neighbor.Name]; !exists || tentativeGScore < existingGScore {
				gScore[neighbor.Name] = tentativeGScore
				fScore[neighbor.Name] = tentativeGScore + heuristic[neighbor.Name]
				previous[neighbor.Name] = current

				neighborNode := &AstarNode{
					Territory: neighbor,
					GScore:    tentativeGScore,
					FScore:    fScore[neighbor.Name],
				}
				heap.Push(pq, neighborNode)
			}
		}
	}

	return nil, ErrNoPath
}

// AstarNode represents a node in A* pathfinding
type AstarNode struct {
	Territory *typedef.Territory
	GScore    float64 // Cost from start
	FScore    float64 // GScore + heuristic
	Index     int     // For heap implementation
}

// AstarPriorityQueue implements heap.Interface for A* pathfinding
type AstarPriorityQueue []*AstarNode

func (pq AstarPriorityQueue) Len() int { return len(pq) }

func (pq AstarPriorityQueue) Less(i, j int) bool {
	return pq[i].FScore < pq[j].FScore
}

func (pq AstarPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *AstarPriorityQueue) Push(x interface{}) {
	n := len(*pq)
	node := x.(*AstarNode)
	node.Index = n
	*pq = append(*pq, node)
}

func (pq *AstarPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.Index = -1
	*pq = old[0 : n-1]
	return node
}

// calculateAstarCost calculates the cost for A* routing mode
func calculateAstarCost(from, to *typedef.Territory, sourceGuildTag string, allies []string) float64 {
	// Base cost
	cost := 1.0

	// If the destination territory is not owned by the same guild, add tax cost
	if to.Guild.Tag != sourceGuildTag {
		tax := GetTaxForTerritory(to, sourceGuildTag, allies)
		cost += tax * 8.0 // Multiply by 8 to make tax significant but less than Dijkstra
	} else {
		// Prefer own guild territories
		cost = 0.2
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
			cost += 3.0 // Penalty for non-allied territories
		}
	}

	return cost
}

// computeHeuristic precomputes heuristic distances from target using reverse BFS
func computeHeuristic(target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string) map[string]float64 {
	heuristic := make(map[string]float64)
	queue := []*typedef.Territory{target}
	visited := make(map[string]bool)
	distances := make(map[string]int)

	visited[target.Name] = true
	distances[target.Name] = 0
	heuristic[target.Name] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Get all territories that connect to current (reverse connections)
		for _, territory := range territoryMap {
			if visited[territory.Name] {
				continue
			}

			// Check if territory connects to current
			connections := GetTerritoryConnections(territory, territoryMap, tradingRoutes)
			connected := false
			for _, conn := range connections {
				if conn.Name == current.Name {
					connected = true
					break
				}
			}

			if connected && CanPassThroughTerritory(territory, sourceGuildTag) {
				visited[territory.Name] = true
				distances[territory.Name] = distances[current.Name] + 1
				heuristic[territory.Name] = float64(distances[territory.Name])
				queue = append(queue, territory)
			}
		}
	}

	// Set a high heuristic for unreachable territories
	for name := range territoryMap {
		if _, exists := heuristic[name]; !exists {
			heuristic[name] = 1000.0
		}
	}

	return heuristic
}
