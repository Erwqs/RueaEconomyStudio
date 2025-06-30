package eruntime

import (
	"errors"
	"etools/eruntime/pathfinder"
	"etools/typedef"
	"math/rand"
	"time"
)

// populateTerritoryLinks populates the Links field for a territory with connected and external territories
func populateTerritoryLinks(territory *typedef.Territory) {
	if territory == nil {
		return
	}

	// Reset both maps before populating
	territory.Links.Direct = make(map[string]struct{})
	territory.Links.Externals = make(map[string]struct{})

	guildTag := territory.Guild.Tag

	// Skip if no guild or guild is "No Guild"
	if guildTag == "" || guildTag == "NONE" {
		return
	}

	// Get direct connections (1 territory away)
	connections := getDirectConnections(territory.Name, guildTag)

	// Add connections to the Links
	for _, conn := range connections {
		territory.Links.Direct[conn] = struct{}{}
		territory.Links.Externals[conn] = struct{}{} // Connections are also externals
	}

	// Get external territories (up to 3 territories away, excluding current territory)
	externals := getExternalTerritories(territory.Name, guildTag, 3)

	// Add externals to the Links
	for _, ext := range externals {
		territory.Links.Externals[ext] = struct{}{}
	}
}

// getDirectConnections returns territories that are directly connected to the given territory
// and owned by the same guild
func getDirectConnections(territoryName, guildTag string) []string {
	var connections []string

	// Get directly connected territories from TradingRoutesMap
	if connectedNames, exists := TradingRoutesMap[territoryName]; exists {
		for _, connectedName := range connectedNames {
			// Check if the connected territory is owned by the same guild
			if connectedTerritory := TerritoryMap[connectedName]; connectedTerritory != nil {
				if connectedTerritory.Guild.Tag == guildTag {
					connections = append(connections, connectedName)
				}
			}
		}
	}

	return connections
}

// getExternalTerritories returns territories that are up to maxDistance away from the given territory
// and owned by the same guild, excluding the current territory itself
func getExternalTerritories(territoryName, guildTag string, maxDistance int) []string {
	visited := make(map[string]bool)
	externals := make(map[string]bool)

	// BFS to find territories within maxDistance
	type queueItem struct {
		name     string
		distance int
	}

	queue := []queueItem{{name: territoryName, distance: 0}}
	visited[territoryName] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Skip if we've reached the maximum distance
		if current.distance >= maxDistance {
			continue
		}

		// Get connected territories
		if connectedNames, exists := TradingRoutesMap[current.name]; exists {
			for _, connectedName := range connectedNames {
				// Skip if already visited
				if visited[connectedName] {
					continue
				}

				// Check if the connected territory is owned by the same guild
				if connectedTerritory := TerritoryMap[connectedName]; connectedTerritory != nil {
					if connectedTerritory.Guild.Tag == guildTag {
						visited[connectedName] = true

						// Don't include the original territory in externals
						if connectedName != territoryName {
							externals[connectedName] = true
						}

						// Add to queue for further exploration
						queue = append(queue, queueItem{
							name:     connectedName,
							distance: current.distance + 1,
						})
					}
				}
			}
		}
	}

	// Convert map to slice
	var result []string
	for external := range externals {
		result = append(result, external)
	}

	return result
}

// updateRoute recalculates trading routes for each territory to its guild HQ.
// If no HQ is set, or guild owner is No Guild, then the territory will not have a route.
func (s *state) updateRoute() {
	for _, t := range s.territories {
		if t == nil {
			continue
		}

		// Reset route information
		t.TradingRoutes = nil
		t.NextTerritory = nil
		t.Destination = nil
		t.RouteTax = -1.0

		// Reset and populate Links
		t.Links.Direct = make(map[string]struct{})
		t.Links.Externals = make(map[string]struct{})
		populateTerritoryLinks(t)

		// Skip if no guild or guild is "No Guild"
		if t.Guild.Tag == "" || t.Guild.Tag == "NONE" {
			continue
		}

		// Find HQ territories for this guild
		hqTerritories := findHQTerritories(t.Guild.Tag)
		if len(hqTerritories) == 0 {
			continue
		}

		// Get allies for this guild
		allies := getGuildAllies(t.Guild.Tag)

		// Skip if this territory is already an HQ
		if t.HQ {
			t.RouteTax = -1.0 // HQ has no route tax
			// Handle HQ routing to all other territories of the same guild
			updateHQRoutes(t, allies)
			continue
		}

		var bestRoutes [][]*typedef.Territory
		var bestHQ *typedef.Territory

		// Try to find route to each HQ and pick the best one
		for _, hq := range hqTerritories {
			var route []*typedef.Territory
			var err error

			switch t.RoutingMode {
			case typedef.RoutingCheapest:
				route, err = pathfinder.Dijkstra(t, hq, TerritoryMap, TradingRoutesMap, t.Guild.Tag, allies)
			case typedef.RoutingFastest:
				route, err = pathfinder.BFS(t, hq, TerritoryMap, TradingRoutesMap, t.Guild.Tag)
			}

			if err != nil || len(route) == 0 {
				continue
			}

			// Check if this route is better, same, or worse than current best
			if len(bestRoutes) == 0 || isBetterRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
				// This route is better, replace all best routes
				bestRoutes = [][]*typedef.Territory{route}
				bestHQ = hq
			} else if len(bestRoutes) > 0 && isEqualRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
				// This route is equal to the best, add it to the list
				bestRoutes = append(bestRoutes, route)
				// Keep the same bestHQ for simplicity, or we could track multiple HQs
			}
		}

		// Set the best route if found
		if len(bestRoutes) > 0 {
			// Randomly select one route from the best routes
			selectedRoute := selectRandomRoute(bestRoutes)
			t.TradingRoutes = [][]*typedef.Territory{selectedRoute}
			t.Destination = bestHQ

			// Set next territory (second in path, or nil if direct connection)
			if len(selectedRoute) > 1 {
				t.NextTerritory = selectedRoute[1]
			}

			// Calculate route tax
			t.RouteTax = pathfinder.CalculateRouteTax(selectedRoute, t.Guild.Tag, allies)
		}
	}
}

// findHQTerritories finds all HQ territories for a given guild
func findHQTerritories(guildTag string) []*typedef.Territory {
	var hqTerritories []*typedef.Territory

	for _, territory := range st.territories {
		if territory != nil && territory.HQ && territory.Guild.Tag == guildTag {
			hqTerritories = append(hqTerritories, territory)
		}
	}

	return hqTerritories
}

// getGuildAllies returns the ally guild tags for a given guild
func getGuildAllies(guildTag string) []string {
	for _, guild := range st.guilds {
		if guild != nil && guild.Tag == guildTag {
			var allies []string
			for _, ally := range guild.Allies {
				if ally != nil {
					allies = append(allies, ally.Tag)
				}
			}
			return allies
		}
	}
	return []string{}
}

// isBetterRoute determines if route1 is better than route2 based on routing mode
func isBetterRoute(route1, route2 []*typedef.Territory, mode typedef.Routing, guildTag string, allies []string) bool {
	if mode == typedef.RoutingFastest {
		// For fastest mode, shorter route is always better
		return len(route1) < len(route2)
	}

	// For cheapest mode, prefer own guild territories first, then lower cost
	ownCount1 := pathfinder.CountOwnGuildTerritories(route1, guildTag)
	ownCount2 := pathfinder.CountOwnGuildTerritories(route2, guildTag)

	if ownCount1 != ownCount2 {
		return ownCount1 > ownCount2
	}

	// If same number of own territories, prefer lower tax
	tax1 := pathfinder.CalculateRouteTax(route1, guildTag, allies)
	tax2 := pathfinder.CalculateRouteTax(route2, guildTag, allies)

	return tax1 < tax2
}

// isEqualRoute determines if route1 is equal to route2 based on routing mode
func isEqualRoute(route1, route2 []*typedef.Territory, mode typedef.Routing, guildTag string, allies []string) bool {
	if mode == typedef.RoutingFastest {
		// For fastest mode, routes are equal if they have the same length
		return len(route1) == len(route2)
	}

	// For cheapest mode, routes are equal if they have the same own guild count and tax
	ownCount1 := pathfinder.CountOwnGuildTerritories(route1, guildTag)
	ownCount2 := pathfinder.CountOwnGuildTerritories(route2, guildTag)

	if ownCount1 != ownCount2 {
		return false
	}

	// Check if taxes are equal
	tax1 := pathfinder.CalculateRouteTax(route1, guildTag, allies)
	tax2 := pathfinder.CalculateRouteTax(route2, guildTag, allies)

	// Use a small epsilon for floating point comparison
	const epsilon = 1e-9
	return abs(tax1-tax2) < epsilon
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// updateHQRoutes calculates routes from HQ to all other territories of the same guild
func updateHQRoutes(hq *typedef.Territory, allies []string) {
	if hq == nil || !hq.HQ {
		return
	}

	// Find all territories of the same guild (excluding the HQ itself)
	var guildTerritories []*typedef.Territory
	for _, territory := range st.territories {
		if territory != nil &&
			territory.Guild.Tag == hq.Guild.Tag &&
			!territory.HQ &&
			territory.Guild.Tag != "" &&
			territory.Guild.Tag != "NONE" {
			guildTerritories = append(guildTerritories, territory)
		}
	}

	// Reset HQ's trading routes
	hq.TradingRoutes = make([][]*typedef.Territory, 0, len(guildTerritories))

	// Calculate routes from HQ to each territory
	for _, target := range guildTerritories {
		var routes [][]*typedef.Territory
		var err error

		switch hq.RoutingMode {
		case typedef.RoutingCheapest:
			routes, err = findAllRoutesWithSameTax(hq, target, hq.Guild.Tag, allies, true)
		case typedef.RoutingFastest:
			routes, err = findAllRoutesWithSameLength(hq, target, hq.Guild.Tag)
		}

		if err != nil || len(routes) == 0 {
			// No route found to this territory (e.g., foreign guild closed borders)
			continue
		}

		// Pick one route randomly if multiple routes have the same cost/length
		selectedRoute := selectRandomRoute(routes)
		hq.TradingRoutes = append(hq.TradingRoutes, selectedRoute)
	}
}

// findAllRoutesWithSameTax finds all routes with the same minimum tax
func findAllRoutesWithSameTax(start, target *typedef.Territory, sourceTag string, allies []string, useCheapest bool) ([][]*typedef.Territory, error) {
	// First find the best route using Dijkstra
	bestRoute, err := pathfinder.Dijkstra(start, target, TerritoryMap, TradingRoutesMap, sourceTag, allies)
	if err != nil || len(bestRoute) == 0 {
		return nil, err
	}

	bestTax := pathfinder.CalculateRouteTax(bestRoute, sourceTag, allies)
	_ = bestTax // Will be used when implementing multiple route finding

	// For now, return just the best route
	// In a more complex implementation, we could use multiple pathfinding runs
	// or implement a modified Dijkstra that tracks all optimal paths
	return [][]*typedef.Territory{bestRoute}, nil
}

// findAllRoutesWithSameLength finds all routes with the same minimum length
func findAllRoutesWithSameLength(start, target *typedef.Territory, sourceTag string) ([][]*typedef.Territory, error) {
	// First find the shortest route using BFS
	bestRoute, err := pathfinder.BFS(start, target, TerritoryMap, TradingRoutesMap, sourceTag)
	if err != nil || len(bestRoute) == 0 {
		return nil, err
	}

	// For now, return just the best route
	// In a more complex implementation, we could use multiple BFS runs
	// or implement a modified BFS that tracks all shortest paths
	return [][]*typedef.Territory{bestRoute}, nil
}

// selectRandomRoute randomly selects one route from a slice of routes
func selectRandomRoute(routes [][]*typedef.Territory) []*typedef.Territory {
	if len(routes) == 0 {
		return nil
	}
	if len(routes) == 1 {
		return routes[0]
	}

	// Initialize random seed if not already done
	rand.Seed(time.Now().UnixNano())

	// Select a random route
	randomIndex := rand.Intn(len(routes))
	return routes[randomIndex]
}

// UpdateAllRoutes is a public function to trigger route updates
func UpdateAllRoutes() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.updateRoute()
}

// SetTerritoryHQ sets a territory as HQ and updates routes
func SetTerritoryHQ(territoryName string, isHQ bool) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	// If setting as HQ, unset other HQs for the same guild
	if isHQ {
		for _, t := range st.territories {
			if t != nil && t.Guild.Tag == territory.Guild.Tag && t.HQ {
				t.HQ = false
			}
		}
	}

	territory.HQ = isHQ

	// Update all routes for this guild
	UpdateAllRoutes()

	return nil
}

// SetTerritoryRoutingMode sets the routing mode for a territory
func SetTerritoryRoutingMode(territoryName string, mode typedef.Routing) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	territory.RoutingMode = mode

	// Update routes for this territory
	UpdateAllRoutes()

	return nil
}

// SetTerritoryBorder sets the border status for a territory
func SetTerritoryBorder(territoryName string, border typedef.Border) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	territory.Border = border

	// Update all routes as this might affect pathfinding
	UpdateAllRoutes()

	return nil
}

// SetTerritoryTax sets the tax rates for a territory
func SetTerritoryTax(territoryName string, normalTax, allyTax float64) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	territory.Tax.Tax = normalTax
	territory.Tax.Ally = allyTax

	// Update all routes as tax affects routing calculations
	UpdateAllRoutes()

	return nil
}

// GetTerritoryRoute returns the route information for a territory
func GetTerritoryRoute(territoryName string) (*typedef.Territory, []*typedef.Territory, error) {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return nil, nil, errors.New("territory not found")
	}

	if len(territory.TradingRoutes) == 0 {
		return territory, nil, nil
	}

	return territory, territory.TradingRoutes[0], nil
}

// GetAllTerritories returns all territories
func GetAllTerritories() []*typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*typedef.Territory, len(st.territories))
	copy(result, st.territories)
	return result
}
