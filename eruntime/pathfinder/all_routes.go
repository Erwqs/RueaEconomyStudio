package pathfinder

import (
	"RueaES/typedef"
	"container/heap"
	"math"
)

const maxAlternativeRoutes = 256

// FindAllCheapestRoutes returns all routes with the minimum pathfinding cost between two territories.
// The routes are computed using the same edge cost model as the cheapest pathfinder.
func FindAllCheapestRoutes(algorithm typedef.PathfindingAlgorithm, start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([][]*typedef.Territory, error) {
	_ = algorithm
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}
	if start == target {
		return [][]*typedef.Territory{{start}}, nil
	}

	// Use Dijkstra-style traversal to gather all optimal predecessors.
	pq := &PriorityQueue{}
	heap.Init(pq)

	const epsilon = 1e-9
	visited := make(map[string]bool)
	distances := make(map[string]float64)
	predecessors := make(map[string][]*typedef.Territory)

	heap.Push(pq, &PathfindingNode{Territory: start, Cost: 0, Distance: 0})
	distances[start.Name] = 0

	for pq.Len() > 0 {
		currentNode := heap.Pop(pq).(*PathfindingNode)
		current := currentNode.Territory

		if visited[current.Name] {
			continue
		}
		visited[current.Name] = true

		neighbors := GetTerritoryConnections(current, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			edgeCost := calculateCheapestCost(current, neighbor, sourceGuildTag, allies)
			newDistance := distances[current.Name] + edgeCost

			existingDistance, exists := distances[neighbor.Name]
			if !exists || newDistance < existingDistance-epsilon {
				distances[neighbor.Name] = newDistance
				predecessors[neighbor.Name] = []*typedef.Territory{current}
				heap.Push(pq, &PathfindingNode{Territory: neighbor, Cost: newDistance, Distance: currentNode.Distance + 1})
			} else if math.Abs(newDistance-existingDistance) < epsilon {
				predecessors[neighbor.Name] = append(predecessors[neighbor.Name], current)
			}
		}
	}

	if _, ok := distances[target.Name]; !ok {
		return nil, ErrNoPath
	}

	routes := buildRoutesFromPredecessors(start, target, predecessors, maxAlternativeRoutes)
	if len(routes) == 0 {
		return nil, ErrNoPath
	}
	return routes, nil
}

// FindAllFastestRoutes returns all routes with the minimum number of steps between two territories.
func FindAllFastestRoutes(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string) ([][]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, ErrNilTerritory
	}
	if start == target {
		return [][]*typedef.Territory{{start}}, nil
	}

	distance := make(map[string]int)
	predecessors := make(map[string][]*typedef.Territory)
	queue := []*typedef.Territory{start}
	distance[start.Name] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		neighbors := GetTerritoryConnections(current, territoryMap, tradingRoutes)
		for _, neighbor := range neighbors {
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}

			newDistance := distance[current.Name] + 1
			existingDistance, exists := distance[neighbor.Name]
			if !exists {
				distance[neighbor.Name] = newDistance
				predecessors[neighbor.Name] = []*typedef.Territory{current}
				queue = append(queue, neighbor)
			} else if newDistance == existingDistance {
				predecessors[neighbor.Name] = append(predecessors[neighbor.Name], current)
			}
		}
	}

	if _, ok := distance[target.Name]; !ok {
		return nil, ErrNoPath
	}

	routes := buildRoutesFromPredecessors(start, target, predecessors, maxAlternativeRoutes)
	if len(routes) == 0 {
		return nil, ErrNoPath
	}
	return routes, nil
}

func buildRoutesFromPredecessors(start, target *typedef.Territory, predecessors map[string][]*typedef.Territory, limit int) [][]*typedef.Territory {
	var routes [][]*typedef.Territory
	var path []*typedef.Territory
	visited := make(map[string]bool)

	var dfs func(node *typedef.Territory)
	dfs = func(node *typedef.Territory) {
		if node == nil || len(routes) >= limit {
			return
		}
		if visited[node.Name] {
			return
		}

		visited[node.Name] = true
		path = append(path, node)

		if node.Name == start.Name {
			reversed := make([]*typedef.Territory, len(path))
			for i := range path {
				reversed[i] = path[len(path)-1-i]
			}
			routes = append(routes, reversed)
		} else {
			for _, prev := range predecessors[node.Name] {
				dfs(prev)
			}
		}

		path = path[:len(path)-1]
		visited[node.Name] = false
	}

	dfs(target)
	return routes
}
