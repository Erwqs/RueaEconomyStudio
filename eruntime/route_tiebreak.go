package eruntime

import (
	"RueaES/typedef"
	"fmt"
	"sort"
)

// AlternativeRoutes returns all optimal routes from the territory to its HQ.
func AlternativeRoutes(territoryName string) map[int][]*typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return alternativeRoutesToHQUnsafe(territoryName)
}

// AlternativeRoutesFromHQ returns all optimal routes from the guild HQ to the territory.
func AlternativeRoutesFromHQ(territoryName string) map[int][]*typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return alternativeRoutesFromHQUnsafe(territoryName)
}

// AlternativeRoutesJSON returns all optimal routes as territory names from the territory to its HQ.
func AlternativeRoutesJSON(territoryName string) map[int][]string {
	routes := AlternativeRoutes(territoryName)
	return routesToNamesMap(routes)
}

// AlternativeRoutesFromHQJSON returns all optimal routes as territory names from the HQ to the territory.
func AlternativeRoutesFromHQJSON(territoryName string) map[int][]string {
	routes := AlternativeRoutesFromHQ(territoryName)
	return routesToNamesMap(routes)
}

// SetTradingRoute selects the route (by ID) from a territory to its HQ.
func SetTradingRoute(territoryName string, routeID int) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.stateLoading {
		return nil
	}

	routes := alternativeRoutesToHQUnsafe(territoryName)
	if len(routes) == 0 {
		return fmt.Errorf("no alternative routes available for territory %s", territoryName)
	}
	if _, ok := routes[routeID]; !ok {
		return fmt.Errorf("invalid route id %d for territory %s", routeID, territoryName)
	}

	st.manualRouteToHQ[territoryName] = routeID
	st.updateRoute()
	return nil
}

// SetTradingRouteFromHQ selects the route (by ID) from the HQ to the territory.
func SetTradingRouteFromHQ(territoryName string, routeID int) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.stateLoading {
		return nil
	}

	routes := alternativeRoutesFromHQUnsafe(territoryName)
	if len(routes) == 0 {
		return fmt.Errorf("no alternative routes available from HQ to territory %s", territoryName)
	}
	if _, ok := routes[routeID]; !ok {
		return fmt.Errorf("invalid route id %d for territory %s", routeID, territoryName)
	}

	st.manualRouteFromHQ[territoryName] = routeID
	st.updateRoute()
	return nil
}

// GetSelectedTradingRouteID returns the current route ID for a territory's route to HQ.
func GetSelectedTradingRouteID(territoryName string) (int, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	routes := alternativeRoutesToHQUnsafe(territoryName)
	if len(routes) == 0 {
		return -1, false
	}

	if routeID, ok := st.manualRouteToHQ[territoryName]; ok {
		if _, exists := routes[routeID]; exists {
			return routeID, true
		}
	}

	territory := TerritoryMap[territoryName]
	if territory == nil {
		return minRouteID(routes), true
	}

	territory.Mu.RLock()
	var current []*typedef.Territory
	if len(territory.TradingRoutes) > 0 {
		current = territory.TradingRoutes[0]
	}
	territory.Mu.RUnlock()

	if current != nil {
		if id, ok := matchRouteID(routes, current); ok {
			return id, true
		}
	}

	return minRouteID(routes), true
}

// GetSelectedTradingRouteFromHQID returns the current route ID for the HQ -> territory route.
func GetSelectedTradingRouteFromHQID(territoryName string) (int, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	routes := alternativeRoutesFromHQUnsafe(territoryName)
	if len(routes) == 0 {
		return -1, false
	}

	if routeID, ok := st.manualRouteFromHQ[territoryName]; ok {
		if _, exists := routes[routeID]; exists {
			return routeID, true
		}
	}

	territory := TerritoryMap[territoryName]
	if territory == nil {
		return minRouteID(routes), true
	}

	territory.Mu.RLock()
	guildTag := territory.Guild.Tag
	territory.Mu.RUnlock()
	if guildTag == "" || guildTag == "NONE" {
		return minRouteID(routes), true
	}

	hq := getHQFromMap(guildTag)
	if hq == nil {
		return minRouteID(routes), true
	}

	hq.Mu.RLock()
	var current []*typedef.Territory
	for _, route := range hq.TradingRoutes {
		if len(route) > 0 && route[len(route)-1] != nil && route[len(route)-1].Name == territoryName {
			current = route
			break
		}
	}
	hq.Mu.RUnlock()

	if current != nil {
		if id, ok := matchRouteID(routes, current); ok {
			return id, true
		}
	}

	return minRouteID(routes), true
}

func alternativeRoutesToHQUnsafe(territoryName string) map[int][]*typedef.Territory {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return map[int][]*typedef.Territory{}
	}

	territory.Mu.RLock()
	guildTag := territory.Guild.Tag
	routingMode := territory.RoutingMode
	territory.Mu.RUnlock()

	if guildTag == "" || guildTag == "NONE" {
		return map[int][]*typedef.Territory{}
	}

	hq := getHQFromMap(guildTag)
	if hq == nil {
		return map[int][]*typedef.Territory{}
	}

	allies := getGuildAllies(guildTag)

	var routes [][]*typedef.Territory
	var err error
	switch routingMode {
	case typedef.RoutingCheapest:
		routes, err = findAllRoutesWithSameTax(territory, hq, guildTag, allies, true)
	case typedef.RoutingFastest:
		routes, err = findAllRoutesWithSameLength(territory, hq, guildTag)
	}

	if err != nil || len(routes) == 0 {
		return map[int][]*typedef.Territory{}
	}

	routes = sortRoutesDeterministically(routes)
	return mapRoutes(routes)
}

func alternativeRoutesFromHQUnsafe(territoryName string) map[int][]*typedef.Territory {
	territory := TerritoryMap[territoryName]
	if territory == nil {
		return map[int][]*typedef.Territory{}
	}

	territory.Mu.RLock()
	guildTag := territory.Guild.Tag
	territory.Mu.RUnlock()

	if guildTag == "" || guildTag == "NONE" {
		return map[int][]*typedef.Territory{}
	}

	hq := getHQFromMap(guildTag)
	if hq == nil {
		return map[int][]*typedef.Territory{}
	}

	hq.Mu.RLock()
	routingMode := hq.RoutingMode
	hq.Mu.RUnlock()

	allies := getGuildAllies(guildTag)

	var routes [][]*typedef.Territory
	var err error
	switch routingMode {
	case typedef.RoutingCheapest:
		routes, err = findAllRoutesWithSameTax(hq, territory, guildTag, allies, true)
	case typedef.RoutingFastest:
		routes, err = findAllRoutesWithSameLength(hq, territory, guildTag)
	}

	if err != nil || len(routes) == 0 {
		return map[int][]*typedef.Territory{}
	}

	routes = sortRoutesDeterministically(routes)
	return mapRoutes(routes)
}

func mapRoutes(routes [][]*typedef.Territory) map[int][]*typedef.Territory {
	result := make(map[int][]*typedef.Territory, len(routes))
	for i, route := range routes {
		result[i] = route
	}
	return result
}

func routesToNamesMap(routes map[int][]*typedef.Territory) map[int][]string {
	result := make(map[int][]string, len(routes))
	for id, route := range routes {
		result[id] = routeToNames(route)
	}
	return result
}

func routeToNames(route []*typedef.Territory) []string {
	out := make([]string, 0, len(route))
	for _, t := range route {
		if t != nil {
			out = append(out, t.Name)
		}
	}
	return out
}

func matchRouteID(routes map[int][]*typedef.Territory, current []*typedef.Territory) (int, bool) {
	currentKey := routeKey(current)
	for id, route := range routes {
		if routeKey(route) == currentKey {
			return id, true
		}
	}
	return -1, false
}

func minRouteID(routes map[int][]*typedef.Territory) int {
	ids := make([]int, 0, len(routes))
	for id := range routes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		return -1
	}
	return ids[0]
}
