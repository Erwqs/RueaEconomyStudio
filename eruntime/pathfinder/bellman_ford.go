package pathfinder

import (
	"RueaES/typedef"
	"math"
)

type bfEdge struct {
	from *typedef.Territory
	to   *typedef.Territory
	cost float64
}

// BellmanFord finds the cheapest path between two territories using Bellman-Ford.
func BellmanFord(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
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

	edges := buildBellmanFordEdges(reachable, tradingRoutes, sourceGuildTag, allies)
	nodeCount := len(reachable)

	distances := make(map[string]float64, nodeCount)
	previous := make(map[string]*typedef.Territory, nodeCount)
	for name := range reachable {
		distances[name] = math.Inf(1)
	}
	distances[start.Name] = 0

	for i := 0; i < nodeCount-1; i++ {
		updated := false
		for _, edge := range edges {
			fromName := edge.from.Name
			toName := edge.to.Name
			if distances[fromName] == math.Inf(1) {
				continue
			}
			newCost := distances[fromName] + edge.cost
			if newCost < distances[toName] {
				distances[toName] = newCost
				previous[toName] = edge.from
				updated = true
			}
		}
		if !updated {
			break
		}
	}

	if distances[target.Name] == math.Inf(1) {
		return nil, ErrNoPath
	}

	return reconstructPath(previous, start, target), nil
}

func reachableTerritories(start *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string) map[string]*typedef.Territory {
	reachable := make(map[string]*typedef.Territory)
	queue := []*typedef.Territory{start}
	queueHead := 0
	reachable[start.Name] = start

	for queueHead < len(queue) {
		current := queue[queueHead]
		queueHead++

		routes := tradingRoutes[current.Name]
		for _, neighborName := range routes {
			neighbor := territoryMap[neighborName]
			if neighbor == nil {
				continue
			}
			if !CanPassThroughTerritory(neighbor, sourceGuildTag) {
				continue
			}
			if _, ok := reachable[neighbor.Name]; ok {
				continue
			}
			reachable[neighbor.Name] = neighbor
			queue = append(queue, neighbor)
		}
	}

	return reachable
}

func buildBellmanFordEdges(reachable map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) []bfEdge {
	edges := make([]bfEdge, 0)
	for fromName, from := range reachable {
		if from == nil {
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
			cost := calculateCheapestCost(from, to, sourceGuildTag, allies)
			edges = append(edges, bfEdge{from: from, to: to, cost: cost})
		}
	}
	return edges
}
