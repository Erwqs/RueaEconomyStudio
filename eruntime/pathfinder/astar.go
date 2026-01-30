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

	// Build adjacency lists once to avoid repeated map lookups and allocations
	connections := buildConnections(territoryMap, tradingRoutes)
	reverseConnections := buildReverseConnections(territoryMap, connections)
	allySet := buildAllySet(allies)

	// Priority queue for A* algorithm
	pq := &AstarPriorityQueue{}
	heap.Init(pq)

	// Maps to track costs and visited nodes
	gScore := make(map[string]float64) // Cost from start to node
	fScore := make(map[string]float64) // gScore + heuristic
	visited := make(map[string]bool)
	previous := make(map[string]*typedef.Territory)

	// Precompute heuristic distances from target using reverse BFS
	heuristic := computeHeuristic(target, reverseConnections, sourceGuildTag)

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

		// Explore neighbors using prebuilt adjacency
		neighbors := connections[current.Name]
		for _, neighbor := range neighbors {
			if visited[neighbor.Name] {
				continue
			}

			// Check if we can pass through this territory
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			// Calculate cost for A* routing
			edgeCost := calculateAstarCost(current, neighbor, sourceGuildTag, allies, allySet)
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

func (pq *AstarPriorityQueue) Push(x any) {
	n := len(*pq)
	node := x.(*AstarNode)
	node.Index = n
	*pq = append(*pq, node)
}

func (pq *AstarPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.Index = -1
	*pq = old[0 : n-1]
	return node
}

// buildConnections creates a forward adjacency list once per search to avoid repeated allocations
func buildConnections(territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string) map[string][]*typedef.Territory {
	connections := make(map[string][]*typedef.Territory, len(territoryMap))

	for name := range territoryMap {
		routes := tradingRoutes[name]
		neighbors := make([]*typedef.Territory, 0, len(routes))
		for _, routeName := range routes {
			if connectedTerritory, exists := territoryMap[routeName]; exists {
				neighbors = append(neighbors, connectedTerritory)
			}
		}
		connections[name] = neighbors
	}

	return connections
}

// buildReverseConnections builds an incoming adjacency list for heuristic BFS
func buildReverseConnections(territoryMap map[string]*typedef.Territory, connections map[string][]*typedef.Territory) map[string][]*typedef.Territory {
	reverse := make(map[string][]*typedef.Territory, len(territoryMap))

	for name := range territoryMap {
		reverse[name] = nil
	}

	for fromName, neighbors := range connections {
		from := territoryMap[fromName]
		for _, neighbor := range neighbors {
			reverse[neighbor.Name] = append(reverse[neighbor.Name], from)
		}
	}

	return reverse
}

// buildAllySet converts ally slice into a set for O(1) membership checks
func buildAllySet(allies []string) map[string]struct{} {
	allySet := make(map[string]struct{}, len(allies))
	for _, ally := range allies {
		allySet[ally] = struct{}{}
	}
	return allySet
}

// calculateAstarCost calculates the cost for A* routing mode
func calculateAstarCost(from, to *typedef.Territory, sourceGuildTag string, allies []string, allySet map[string]struct{}) float64 {
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
		if _, isAlly := allySet[to.Guild.Tag]; !isAlly {
			cost += 3.0 // Penalty for non-allied territories
		}
	}

	return cost
}

// computeHeuristic precomputes heuristic distances from target using reverse BFS on prebuilt edges
func computeHeuristic(target *typedef.Territory, reverseConnections map[string][]*typedef.Territory, sourceGuildTag string) map[string]float64 {
	heuristic := make(map[string]float64, len(reverseConnections))
	queue := []*typedef.Territory{target}
	visited := make(map[string]bool, len(reverseConnections))
	distances := make(map[string]int, len(reverseConnections))

	visited[target.Name] = true
	distances[target.Name] = 0
	heuristic[target.Name] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, previous := range reverseConnections[current.Name] {
			if visited[previous.Name] {
				continue
			}

			if !CanPassThroughTerritory(previous, sourceGuildTag) {
				continue
			}

			visited[previous.Name] = true
			distances[previous.Name] = distances[current.Name] + 1
			heuristic[previous.Name] = float64(distances[previous.Name])
			queue = append(queue, previous)
		}
	}

	// Set a high heuristic for unreachable territories
	for name := range reverseConnections {
		if _, exists := heuristic[name]; !exists {
			heuristic[name] = 1000.0
		}
	}

	return heuristic
}
