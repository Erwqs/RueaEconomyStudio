package eruntime

import (
	"errors"
	"sync"

	"RueaES/eruntime/pathfinder"
	"RueaES/typedef"
)

// PathfinderTerritory represents the minimal territory data shared with plugin pathfinders.
type PathfinderTerritory struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	GuildTag    string          `json:"guildTag"`
	RoutingMode typedef.Routing `json:"routingMode"`
	Links       []string        `json:"links"`
}

// PathfinderGraph is the partial map view sent to plugin pathfinders.
type PathfinderGraph struct {
	Territories map[string]PathfinderTerritory `json:"territories"`
}

// PathfinderResolver resolves routes between src and dst territory IDs.
type PathfinderResolver func(graph PathfinderGraph, src, dst string) ([]string, error)

var (
	pfResolverMu      sync.RWMutex
	pfResolver        PathfinderResolver
	errNoPathResolver = errors.New("no pathfinder resolver")
)

// SetPathfinderResolver installs the resolver used for plugin-based pathfinding.
func SetPathfinderResolver(resolver PathfinderResolver) {
	pfResolverMu.Lock()
	pfResolver = resolver
	pfResolverMu.Unlock()
}

// ClearPathfinderResolver removes the current resolver.
func ClearPathfinderResolver() {
	pfResolverMu.Lock()
	pfResolver = nil
	pfResolverMu.Unlock()
}

func hasPathfinderResolver() bool {
	pfResolverMu.RLock()
	defer pfResolverMu.RUnlock()
	return pfResolver != nil
}

func invokePathfinderResolver(graph PathfinderGraph, src, dst string) ([]string, error) {
	pfResolverMu.RLock()
	resolver := pfResolver
	pfResolverMu.RUnlock()
	if resolver == nil {
		return nil, errNoPathResolver
	}
	return resolver(graph, src, dst)
}

// buildPathfinderGraph performs a bounded crawl from src across the trading routes,
// producing a partial graph suitable for plugin pathfinding.
func buildPathfinderGraph(src, dst string) PathfinderGraph {
	graph := PathfinderGraph{Territories: make(map[string]PathfinderTerritory)}
	if src == "" {
		return graph
	}

	visited := make(map[string]struct{})
	queue := []string{src}
	visited[src] = struct{}{}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		terr := TerritoryMap[node]
		if terr == nil {
			continue
		}

		neighbors := TradingRoutesMap[node]
		graph.Territories[node] = PathfinderTerritory{
			ID:          terr.ID,
			Name:        terr.Name,
			GuildTag:    terr.Guild.Tag,
			RoutingMode: terr.RoutingMode,
			Links:       append([]string(nil), neighbors...),
		}

		// Continue crawling outward so plugins get a connected subgraph; stop adding once discovered.
		for _, nbr := range neighbors {
			if _, seen := visited[nbr]; seen {
				continue
			}
			visited[nbr] = struct{}{}
			queue = append(queue, nbr)
			// Fast path: keep exploring even after reaching dst to provide alternates, but dst ensures inclusion.
		}
	}

	// Ensure destination is present if known, even if unreachable from src.
	if dst != "" {
		if _, ok := graph.Territories[dst]; !ok {
			if terr := TerritoryMap[dst]; terr != nil {
				graph.Territories[dst] = PathfinderTerritory{
					ID:          terr.ID,
					Name:        terr.Name,
					GuildTag:    terr.Guild.Tag,
					RoutingMode: terr.RoutingMode,
					Links:       append([]string(nil), TradingRoutesMap[dst]...),
				}
			}
		}
	}

	return graph
}

// resolvePathWithPlugin attempts to resolve a route via the registered pathfinder resolver.
func resolvePathWithPlugin(start, target *typedef.Territory) ([]*typedef.Territory, error) {
	if start == nil || target == nil {
		return nil, errors.New("nil territory")
	}
	graph := buildPathfinderGraph(start.Name, target.Name)
	ids, err := invokePathfinderResolver(graph, start.Name, target.Name)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, pathfinder.ErrNoPath
	}
	route := make([]*typedef.Territory, 0, len(ids))
	for _, id := range ids {
		terr := TerritoryMap[id]
		if terr == nil {
			return nil, errors.New("pathfinder returned unknown territory: " + id)
		}
		route = append(route, terr)
	}
	return route, nil
}
