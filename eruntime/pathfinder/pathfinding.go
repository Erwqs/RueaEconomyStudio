package pathfinder

import (
	"RueaES/typedef"
)

// FindPath finds a path using the specified pathfinding algorithm
func FindPath(algorithm typedef.PathfindingAlgorithm, start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
	switch start.RoutingMode {
	case typedef.RoutingCheapest:
		return FindPathCheapest(algorithm, start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	case typedef.RoutingFastest:
		return FindPathFastest(start, target, territoryMap, tradingRoutes, sourceGuildTag)
	default:
		// Default to cheapest if unknown
		return FindPathCheapest(algorithm, start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	}
}

// FindPathCheapest finds the cheapest path using the specified algorithm
// This is for when we specifically want cheapest routing - uses user-selected algorithm
func FindPathCheapest(algorithm typedef.PathfindingAlgorithm, start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string, allies []string) ([]*typedef.Territory, error) {
	switch algorithm {
	case typedef.PathfindingDijkstra:
		return Dijkstra(start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	case typedef.PathfindingAstar:
		return Astar(start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	case typedef.PathfindingFloodFill:
		// FloodFill does not find a single path but all reachable territories
		// We can use it to find the path to the target if reachable
		floodFillResult, err := FloodFill(start, territoryMap, tradingRoutes, sourceGuildTag, allies)
		if err != nil {
			return nil, err
		}
		path, ok := floodFillResult.Paths[target.Name]
		if !ok || len(path) == 0 {
			return nil, ErrNoPath
		}
		return path, nil
	default:
		// Default to Dijkstra if unknown algorithm
		return Dijkstra(start, target, territoryMap, tradingRoutes, sourceGuildTag, allies)
	}
}

// FindPathFastest finds the fastest path - always uses BFS regardless of user setting
// This is for when we specifically want fastest routing (shortest path, ignores cost)
func FindPathFastest(start, target *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string, sourceGuildTag string) ([]*typedef.Territory, error) {
	return BFS(start, target, territoryMap, tradingRoutes, sourceGuildTag)
}
