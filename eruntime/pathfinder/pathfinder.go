package pathfinder

import (
	"RueaES/typedef"
	"errors"
)

var (
	ErrNilTerritory = errors.New("nil territory")
	ErrNoPath       = errors.New("cannot find path from t1 to t2")
)

// PathfindingNode represents a node in pathfinding algorithms
type PathfindingNode struct {
	Territory *typedef.Territory
	Cost      float64
	Distance  int
	Previous  *PathfindingNode
	Index     int // For heap implementation
}

// PriorityQueue implements heap.Interface for pathfinding
type PriorityQueue []*PathfindingNode

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Cost < pq[j].Cost
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	node := x.(*PathfindingNode)
	node.Index = n
	*pq = append(*pq, node)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.Index = -1
	*pq = old[0 : n-1]
	return node
}

// GetTerritoryConnections returns the territories connected to the given territory
func GetTerritoryConnections(territory *typedef.Territory, territoryMap map[string]*typedef.Territory, tradingRoutes map[string][]string) []*typedef.Territory {
	if territory == nil {
		return nil
	}

	var connections []*typedef.Territory

	// Get trading routes for this territory
	if routes, exists := tradingRoutes[territory.Name]; exists {
		for _, routeName := range routes {
			if connectedTerritory, exists := territoryMap[routeName]; exists {
				connections = append(connections, connectedTerritory)
			}
		}
	}

	return connections
}

// CanPassThroughTerritory checks if we can pass through a territory based on border status and guild ownership
func CanPassThroughTerritory(territory *typedef.Territory, sourceGuildTag string) bool {
	if territory == nil {
		return false
	}

	// Can always pass through own guild's territory
	if territory.Guild.Tag == sourceGuildTag {
		return true
	}

	// Cannot pass through other guild's territory if border is closed
	if territory.Border == typedef.BorderClosed {
		return false
	}

	return true
}

// GetTaxForTerritory returns the tax rate for passing through a territory
func GetTaxForTerritory(territory *typedef.Territory, sourceGuildTag string, allies []string) float64 {
	if territory == nil {
		return 0.0
	}

	// No tax for own guild's territory
	if territory.Guild.Tag == sourceGuildTag {
		return 0.0
	}

	// Check if ally
	for _, ally := range allies {
		if territory.Guild.Tag == ally {
			return territory.Tax.Ally
		}
	}

	// Normal tax for non-allied guilds
	return territory.Tax.Tax
}

// CalculateRouteTax calculates the compound tax for a route path
func CalculateRouteTax(path []*typedef.Territory, sourceGuildTag string, allies []string) float64 {
	if len(path) <= 2 {
		return 0.0 // No intermediate territories
	}

	combinedTaxReduction := 1.0

	// Calculate for intermediate territories (exclude start and end)
	for i := 1; i < len(path)-1; i++ {
		territory := path[i]

		// Only calculate tax for territories not owned by the same guild
		if territory.Guild.Tag != sourceGuildTag {
			tax := GetTaxForTerritory(territory, sourceGuildTag, allies)
			combinedTaxReduction *= (1.0 - tax)
		}
	}

	return 1.0 - combinedTaxReduction
}

// CountOwnGuildTerritories counts how many territories in the path belong to the source guild
func CountOwnGuildTerritories(path []*typedef.Territory, sourceGuildTag string) int {
	count := 0
	for _, territory := range path {
		if territory.Guild.Tag == sourceGuildTag {
			count++
		}
	}
	return count
}
