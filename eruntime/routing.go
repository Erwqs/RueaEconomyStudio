package eruntime

import (
	"errors"
	"etools/eruntime/pathfinder"
	"etools/typedef"
	"fmt"
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
	// fmt.Printf("[ROUTING_DEBUG] updateRoute called - recalculating all trading routes\n")
	// Caches for HQs and allies per guild
	hqCache := make(map[string][]*typedef.Territory)
	alliesCache := make(map[string][]string)

	// Revert to sequential route calculation for each territory
	for _, t := range s.territories {
		if t == nil {
			continue
		}

		// Skip if no guild or guild is "No Guild"
		if t.Guild.Tag == "" || t.Guild.Tag == "NONE" {
			// Still reset route info for consistency
			t.TradingRoutes = nil
			t.NextTerritory = nil
			t.Destination = nil
			t.RouteTax = -1.0
			continue
		}

		// Reset route information
		t.TradingRoutes = nil
		t.NextTerritory = nil
		t.Destination = nil
		t.RouteTax = -1.0

		// Only populate links for valid guild territories
		populateTerritoryLinks(t)

		guildTag := t.Guild.Tag

		// Cache HQs and allies per guild
		if _, ok := hqCache[guildTag]; !ok {
			hqCache[guildTag] = findHQTerritories(guildTag)
		}
		if _, ok := alliesCache[guildTag]; !ok {
			alliesCache[guildTag] = getGuildAllies(guildTag)
		}
		hqTerritories := hqCache[guildTag]
		allies := alliesCache[guildTag]

		// Cache for pathfinding results between this territory and each HQ
		pathCache := make(map[string]struct {
			route []*typedef.Territory
			err   error
		})

		if len(hqTerritories) == 0 {
			// fmt.Printf("[ROUTING_DEBUG] No HQ found for guild %s, skipping territory %s\n", t.Guild.Tag, t.Name)
			continue
		}

		// Skip if this territory is already an HQ
		if t.HQ {
			// fmt.Printf("[ROUTING_DEBUG] Territory %s is HQ for guild %s, calling updateHQRoutes\n", t.Name, t.Guild.Tag)
			t.RouteTax = -1.0 // HQ has no route tax
			// Handle HQ routing to all other territories of the same guild
			updateHQRoutes(t, allies)
			continue
		}

		var bestRoutes [][]*typedef.Territory
		var bestHQ *typedef.Territory

		// Try to find route to each HQ and pick the best one
		for _, hq := range hqTerritories {
			cacheKey := hq.Name
			if cached, ok := pathCache[cacheKey]; ok {
				route, err := cached.route, cached.err
				if err != nil || len(route) == 0 {
					continue
				}
				if len(bestRoutes) == 0 || isBetterRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
					bestRoutes = [][]*typedef.Territory{route}
					bestHQ = hq
				} else if len(bestRoutes) > 0 && isEqualRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
					bestRoutes = append(bestRoutes, route)
				}
				continue
			}

			var route []*typedef.Territory
			var err error
			switch t.RoutingMode {
			case typedef.RoutingCheapest:
				route, err = pathfinder.Dijkstra(t, hq, TerritoryMap, TradingRoutesMap, t.Guild.Tag, allies)
			case typedef.RoutingFastest:
				route, err = pathfinder.BFS(t, hq, TerritoryMap, TradingRoutesMap, t.Guild.Tag)
			}
			// Store in cache
			pathCache[cacheKey] = struct {
				route []*typedef.Territory
				err   error
			}{route, err}

			if err != nil || len(route) == 0 {
				continue
			}
			if len(bestRoutes) == 0 || isBetterRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
				bestRoutes = [][]*typedef.Territory{route}
				bestHQ = hq
			} else if len(bestRoutes) > 0 && isEqualRoute(route, bestRoutes[0], t.RoutingMode, t.Guild.Tag, allies) {
				bestRoutes = append(bestRoutes, route)
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

// HQ management functions for fast lookups

// setHQInMap sets or removes an HQ in the map
func setHQInMap(territory *typedef.Territory, isHQ bool) {
	if territory == nil {
		return
	}

	guildTag := territory.Guild.Tag
	if guildTag == "" || guildTag == "NONE" {
		return
	}

	if isHQ {
		st.hqMap[guildTag] = territory
	} else {
		// Only remove if this territory was the HQ
		if currentHQ, exists := st.hqMap[guildTag]; exists && currentHQ == territory {
			delete(st.hqMap, guildTag)
		}
	}
}

// getHQFromMap gets the HQ territory for a guild tag
func getHQFromMap(guildTag string) *typedef.Territory {
	if guildTag == "" || guildTag == "NONE" {
		return nil
	}
	return st.hqMap[guildTag]
}

// rebuildHQMap rebuilds the HQ map from scratch by scanning all territories
func rebuildHQMap() {
	st.hqMap = make(map[string]*typedef.Territory)
	for _, territory := range st.territories {
		if territory != nil && territory.HQ {
			guildTag := territory.Guild.Tag
			if guildTag != "" && guildTag != "NONE" {
				st.hqMap[guildTag] = territory
			}
		}
	}
	fmt.Printf("[HQ_MAP] Rebuilt HQ map with %d entries\n", len(st.hqMap))
}

// findHQTerritories finds all HQ territories for a given guild using the fast HQ map
func findHQTerritories(guildTag string) []*typedef.Territory {
	var hqTerritories []*typedef.Territory

	// Use the fast HQ map lookup instead of scanning all territories
	if hq := getHQFromMap(guildTag); hq != nil {
		hqTerritories = append(hqTerritories, hq)
	}

	return hqTerritories
}

// getGuildAllies returns the ally guild tags for a given guild
func getGuildAllies(guildTag string) []string {
	for _, guild := range st.guilds {
		if guild != nil && guild.Tag == guildTag {
			// Return copy of AllyTags to avoid external modification
			allies := make([]string, len(guild.AllyTags))
			copy(allies, guild.AllyTags)
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

	// fmt.Printf("[ROUTING_DEBUG] updateHQRoutes called for HQ: %s, guild: %s [%s]\n", hq.Name, hq.Guild.Name, hq.Guild.Tag)

	// Find all territories of the same guild (excluding the HQ itself)
	var guildTerritories []*typedef.Territory
	for _, territory := range st.territories {
		if territory != nil &&
			territory.Guild.Tag == hq.Guild.Tag &&
			!territory.HQ &&
			territory.Guild.Tag != "" &&
			territory.Guild.Tag != "NONE" {
			guildTerritories = append(guildTerritories, territory)
			// fmt.Printf("[ROUTING_DEBUG] Found guild territory: %s\n", territory.Name)
		}
	}

	// fmt.Printf("[ROUTING_DEBUG] HQ %s found %d territories of guild %s to route to\n", hq.Name, len(guildTerritories), hq.Guild.Tag)

	// Reset HQ's trading routes
	hq.TradingRoutes = make([][]*typedef.Territory, 0, len(guildTerritories))

	// Calculate routes from HQ to each territory
	for _, target := range guildTerritories {
		// fmt.Printf("[ROUTING_DEBUG] Calculating route from HQ %s to %s\n", hq.Name, target.Name)

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
			// fmt.Printf("[ROUTING_DEBUG] No route found from HQ %s to %s: %v\n", hq.Name, target.Name, err)
			continue
		}

		// Pick one route randomly if multiple routes have the same cost/length
		selectedRoute := selectRandomRoute(routes)
		hq.TradingRoutes = append(hq.TradingRoutes, selectedRoute)
		// fmt.Printf("[ROUTING_DEBUG] Added route from HQ %s to %s (length: %d)\n", hq.Name, target.Name, len(selectedRoute))
	}

	// fmt.Printf("[ROUTING_DEBUG] HQ %s final route count: %d\n", hq.Name, len(hq.TradingRoutes))
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
	// Don't allow HQ modifications during state loading
	st.mu.RLock()
	if st.stateLoading {
		st.mu.RUnlock()
		fmt.Printf("[ERUNTIME] SetTerritoryHQ blocked during state loading for territory: %s\n", territoryName)
		return nil
	}
	st.mu.RUnlock()

	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	fmt.Printf("[HQ_DEBUG] SetTerritoryHQ called for territory %s, isHQ=%v\n", territoryName, isHQ)

	// If setting as HQ, unset other HQs for the same guild
	if isHQ {
		if oldHQ := getHQFromMap(territory.Guild.Tag); oldHQ != nil && oldHQ != territory {
			fmt.Printf("[HQ_DEBUG] Clearing old HQ %s for guild %s in SetTerritoryHQ\n", oldHQ.Name, oldHQ.Guild.Tag)
			oldHQ.HQ = false
			setHQInMap(oldHQ, false)
		}
	}

	territory.HQ = isHQ
	setHQInMap(territory, isHQ)
	fmt.Printf("[HQ_DEBUG] Set territory %s HQ status to %v\n", territoryName, isHQ)

	// Update all routes for this guild
	// fmt.Printf("[ROUTING_DEBUG] SetTerritoryHQ: Calling UpdateAllRoutes after setting HQ status\n")
	UpdateAllRoutes()

	// Notify UI components to update after HQ change
	// This ensures territory colors and HQ icons are refreshed
	go func() {
		// Add a small delay to ensure state is fully settled
		time.Sleep(50 * time.Millisecond)

		// Notify territory manager to update colors and visual state
		NotifyTerritoryColorsUpdate()

		// Notify only the specific guild that had its HQ changed for efficiency
		NotifyGuildSpecificUpdate(territory.Guild.Name)

		fmt.Printf("[HQ_DEBUG] SetTerritoryHQ notifications sent to UI components for guild: %s\n", territory.Guild.Name)
	}()

	// Trigger auto-save after user action
	TriggerAutoSave()

	return nil
}

// SetTerritoryRoutingMode sets the routing mode for a territory
func SetTerritoryRoutingMode(territoryName string, mode typedef.Routing) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	// Check if this actually changes the value
	if territory.RoutingMode == mode {
		return nil // No change needed
	}

	territory.RoutingMode = mode

	// Update routes for this territory
	UpdateAllRoutes()

	// Trigger territory change callback
	if territoryChangeCallback != nil {
		territoryChangeCallback(territoryName)
	}

	// Trigger auto-save after user action
	TriggerAutoSave()

	return nil
}

// SetTerritoryBorder sets the border status for a territory
func SetTerritoryBorder(territoryName string, border typedef.Border) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	// Check if this actually changes the value
	if territory.Border == border {
		return nil // No change needed
	}

	territory.Border = border

	// Update all routes as this might affect pathfinding
	UpdateAllRoutes()

	// Trigger territory change callback
	if territoryChangeCallback != nil {
		territoryChangeCallback(territoryName)
	}

	// Trigger auto-save after user action
	TriggerAutoSave()

	return nil
}

// SetTerritoryTax sets the tax rates for a territory
func SetTerritoryTax(territoryName string, normalTax, allyTax float64) error {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return errors.New("territory not found")
	}

	// Check if this actually changes the values
	if territory.Tax.Tax == normalTax && territory.Tax.Ally == allyTax {
		return nil // No change needed
	}

	territory.Tax.Tax = normalTax
	territory.Tax.Ally = allyTax

	// Update all routes as tax affects routing calculations
	UpdateAllRoutes()

	// Trigger territory change callback
	if territoryChangeCallback != nil {
		territoryChangeCallback(territoryName)
	}

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

// FindTributeRoute calculates the optimal route between two HQ territories for tribute transfer
// This function considers tax rates, border closures, and guild relationships
func FindTributeRoute(fromHQ, toHQ *typedef.Territory) ([]string, error) {
	if fromHQ == nil || toHQ == nil {
		return nil, fmt.Errorf("both HQ territories must be non-nil")
	}

	// If both HQs are the same territory, no route needed
	if fromHQ.Name == toHQ.Name {
		return []string{fromHQ.ID}, nil
	}

	debugf("Finding tribute route from %s HQ (%s) to %s HQ (%s)\n",
		fromHQ.Guild.Name, fromHQ.Name, toHQ.Guild.Name, toHQ.Name)

	sourceGuildTag := fromHQ.Guild.Tag
	allies := getGuildAllies(sourceGuildTag)

	// Try different pathfinding strategies based on routing mode preference
	// For tributes, we prefer the cheapest route to minimize tax costs
	var bestRoute []*typedef.Territory
	var bestCost float64 = -1
	var err error

	// Try cheapest route first (Dijkstra with tax consideration)
	route, err := pathfinder.Dijkstra(fromHQ, toHQ, TerritoryMap, TradingRoutesMap, sourceGuildTag, allies)
	if err == nil && len(route) > 0 {
		cost := pathfinder.CalculateRouteTax(route, sourceGuildTag, allies)
		if bestCost < 0 || cost < bestCost {
			bestRoute = route
			bestCost = cost
		}
		debugf("Dijkstra route found: %d territories, tax cost: %.2f\n", len(route), cost)
	} else {
		debugf("Dijkstra failed: %v\n", err)
	}

	// Try fastest route as fallback (BFS)
	route, err = pathfinder.BFS(fromHQ, toHQ, TerritoryMap, TradingRoutesMap, sourceGuildTag)
	if err == nil && len(route) > 0 {
		cost := pathfinder.CalculateRouteTax(route, sourceGuildTag, allies)
		if bestCost < 0 || cost < bestCost {
			bestRoute = route
			bestCost = cost
		}
		debugf("BFS route found: %d territories, tax cost: %.2f\n", len(route), cost)
	} else {
		debugf("BFS failed: %v\n", err)
	}

	// If no route found, return error
	if len(bestRoute) == 0 {
		debugf("No tribute route found from %s to %s\n", fromHQ.Name, toHQ.Name)
		return nil, fmt.Errorf("no route found between HQs")
	}

	// Convert territory path to ID strings
	routeIDs := make([]string, len(bestRoute))
	routeNames := make([]string, len(bestRoute))
	for i, territory := range bestRoute {
		routeIDs[i] = territory.ID
		routeNames[i] = territory.Name
	}

	debugf("Best tribute route found: %v (cost: %.2f, %d territories)\n",
		routeNames, bestCost, len(bestRoute))

	// Validate that route doesn't pass through closed borders
	for i := 1; i < len(bestRoute); i++ {
		territory := bestRoute[i]
		if !pathfinder.CanPassThroughTerritory(territory, sourceGuildTag) {
			debugf("Route validation failed: cannot pass through %s (border closed)\n", territory.Name)
			return nil, fmt.Errorf("route blocked by closed border at %s", territory.Name)
		}
	}

	debugf("Route validation successful\n")
	return routeIDs, nil
}
