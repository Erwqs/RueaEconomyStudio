package eruntime

import (
	"RueaES/typedef"
	"fmt"
	"sync"
)

// Transit represents a single resource movement between territories
type Transit struct {
	ID             string                 `json:"id"`            // Unique identifier for this transit
	BasicResources typedef.BasicResources `json:"resources"`     // Resources being moved
	OriginID       string                 `json:"originId"`      // ID of origin territory
	DestinationID  string                 `json:"destinationId"` // ID of destination territory
	Route          []string               `json:"route"`         // Territory IDs along the route
	RouteIndex     int                    `json:"routeIndex"`    // Current position in route
	NextTax        float64                `json:"nextTax"`       // Tax for next territory
	CreatedAt      uint64                 `json:"createdAt"`     // Tick when transit was created
	Moved          bool                   `json:"moved"`         // Whether this transit has moved this tick
}

// TransitManager manages all resource transits in the system
type TransitManager struct {
	// All active transits indexed by ID
	transits map[string]*Transit

	// Fast lookup: which transits are currently at each territory
	atTerritory map[string][]*Transit // territory ID -> transits currently there

	// Mutex to protect concurrent access
	mu sync.RWMutex

	// Counter for generating unique transit IDs
	nextID uint64
}

// NewTransitManager creates a new transit manager
func NewTransitManager() *TransitManager {
	return &TransitManager{
		transits:    make(map[string]*Transit),
		atTerritory: make(map[string][]*Transit),
	}
}

// StartTransit creates a new transit from origin to destination
func (tm *TransitManager) StartTransit(resources typedef.BasicResources, originID, destID string, route []string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.nextID++
	transitID := fmt.Sprintf("transit_%d", tm.nextID)

	transit := &Transit{
		ID:             transitID,
		BasicResources: resources,
		OriginID:       originID,
		DestinationID:  destID,
		Route:          route,
		RouteIndex:     0,
		CreatedAt:      st.tick,
		Moved:          false,
	}

	// Calculate next tax if there's a next territory
	if len(route) > 1 {
		nextTerritory := st.getTerritoryByID(route[1])
		if nextTerritory != nil {
			originTerritory := st.getTerritoryByID(originID)
			if originTerritory != nil && nextTerritory.Guild.Tag != originTerritory.Guild.Tag {
				transit.NextTax = nextTerritory.Tax.Tax
			} else {
				transit.NextTax = nextTerritory.Tax.Ally
			}
		}
	}

	tm.transits[transitID] = transit

	// Add to current territory's list (first in route)
	if len(route) > 0 {
		currentTerritoryID := route[0]
		tm.atTerritory[currentTerritoryID] = append(tm.atTerritory[currentTerritoryID], transit)
	}

	return transitID
}

// ProcessAllTransits moves all transits forward one step
func (tm *TransitManager) ProcessAllTransits() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	completedTransits := []string{}

	// Reset moved flag for all transits
	for _, transit := range tm.transits {
		transit.Moved = false
	}

	// Process each transit
	for transitID, transit := range tm.transits {
		if tm.moveTransitForward(transit) {
			// Transit completed
			completedTransits = append(completedTransits, transitID)
		}
	}

	// Clean up completed transits
	for _, transitID := range completedTransits {
		tm.removeTransit(transitID)
	}
}

// moveTransitForward moves a single transit to its next position
// Returns true if transit is completed
func (tm *TransitManager) moveTransitForward(transit *Transit) bool {
	if transit.Moved {
		return false // Already moved this tick
	}

	// Mark as moved to prevent double processing
	transit.Moved = true

	// Get current and next territory
	if transit.RouteIndex >= len(transit.Route) {
		return true // Invalid state, remove transit
	}

	currentTerritoryID := transit.Route[transit.RouteIndex]
	currentTerritory := st.getTerritoryByID(currentTerritoryID)
	if currentTerritory == nil {
		return true // Territory doesn't exist, remove transit
	}

	// Check if we've reached destination
	if transit.RouteIndex == len(transit.Route)-1 {
		// Arrived at destination
		tm.deliverResources(transit, currentTerritory)
		return true
	}

	// Move to next territory
	transit.RouteIndex++
	nextTerritoryID := transit.Route[transit.RouteIndex]
	nextTerritory := st.getTerritoryByID(nextTerritoryID)

	if nextTerritory == nil {
		return true // Next territory doesn't exist, remove transit
	}

	// Check border status
	if nextTerritory.Border == typedef.BorderClosed &&
		currentTerritory.Guild.Tag != nextTerritory.Guild.Tag {
		// Border closed to different guild, void resources
		return true
	}

	// Apply tax if crossing guild boundary
	if currentTerritory.Guild.Tag != nextTerritory.Guild.Tag {
		// --- Begin new logic for immediate tax delivery ---
		preTax := transit.BasicResources
		transit.BasicResources = applyTax(transit.BasicResources, transit.NextTax)
		taxed := typedef.BasicResources{
			Emeralds: preTax.Emeralds - transit.BasicResources.Emeralds,
			Ores:     preTax.Ores - transit.BasicResources.Ores,
			Wood:     preTax.Wood - transit.BasicResources.Wood,
			Fish:     preTax.Fish - transit.BasicResources.Fish,
			Crops:    preTax.Crops - transit.BasicResources.Crops,
		}
		// Find HQ of the foreign guild (nextTerritory.Guild.Tag)
		var foreignHQ *typedef.Territory
		for _, t := range st.territories {
			if t != nil && t.HQ && t.Guild.Tag == nextTerritory.Guild.Tag {
				foreignHQ = t
				break
			}
		}
		if foreignHQ != nil {
			foreignHQ.Mu.Lock()
			foreignHQ.Storage.At.Emeralds += taxed.Emeralds
			foreignHQ.Storage.At.Ores += taxed.Ores
			foreignHQ.Storage.At.Wood += taxed.Wood
			foreignHQ.Storage.At.Fish += taxed.Fish
			foreignHQ.Storage.At.Crops += taxed.Crops
			foreignHQ.Mu.Unlock()
		}
		// --- End new logic ---
	}

	// Calculate next tax for the following territory
	if transit.RouteIndex+1 < len(transit.Route) {
		followingTerritoryID := transit.Route[transit.RouteIndex+1]
		followingTerritory := st.getTerritoryByID(followingTerritoryID)
		if followingTerritory != nil {
			if nextTerritory.Guild.Tag != followingTerritory.Guild.Tag {
				transit.NextTax = followingTerritory.Tax.Tax
			} else {
				transit.NextTax = followingTerritory.Tax.Ally
			}
		}
	}

	// Remove from current territory's list
	tm.removeTransitFromTerritory(currentTerritoryID, transit)

	// Add to next territory's list
	tm.atTerritory[nextTerritoryID] = append(tm.atTerritory[nextTerritoryID], transit)

	return false // Transit continues
}

// deliverResources delivers transit resources to the destination territory
func (tm *TransitManager) deliverResources(transit *Transit, territory *typedef.Territory) {
	territory.Mu.Lock()
	defer territory.Mu.Unlock()

	// Add resources to territory storage (with capacity limits)
	territory.Storage.At.Emeralds += transit.BasicResources.Emeralds
	territory.Storage.At.Ores += transit.BasicResources.Ores
	territory.Storage.At.Wood += transit.BasicResources.Wood
	territory.Storage.At.Fish += transit.BasicResources.Fish
	territory.Storage.At.Crops += transit.BasicResources.Crops

	// Check for overflow warnings (implementation would set warnings in territory)
	// This would be implemented based on your existing warning system
}

// removeTransit removes a transit from the manager
func (tm *TransitManager) removeTransit(transitID string) {
	transit, exists := tm.transits[transitID]
	if !exists {
		return
	}

	// Remove from territory list if still there
	if transit.RouteIndex < len(transit.Route) {
		currentTerritoryID := transit.Route[transit.RouteIndex]
		tm.removeTransitFromTerritory(currentTerritoryID, transit)
	}

	// Remove from main map
	delete(tm.transits, transitID)
}

// removeTransitFromTerritory removes a specific transit from a territory's list
func (tm *TransitManager) removeTransitFromTerritory(territoryID string, targetTransit *Transit) {
	transits := tm.atTerritory[territoryID]
	for i, transit := range transits {
		if transit.ID == targetTransit.ID {
			// Remove by swapping with last element and truncating
			transits[i] = transits[len(transits)-1]
			tm.atTerritory[territoryID] = transits[:len(transits)-1]
			break
		}
	}
}

// GetTransitsAtTerritory returns all transits currently at a territory
func (tm *TransitManager) GetTransitsAtTerritory(territoryID string) []*Transit {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	transits := tm.atTerritory[territoryID]
	result := make([]*Transit, len(transits))
	copy(result, transits)
	return result
}

// GetAllTransits returns a copy of all active transits
func (tm *TransitManager) GetAllTransits() map[string]*Transit {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*Transit)
	for id, transit := range tm.transits {
		result[id] = transit
	}
	return result
}

// ClearAllTransits removes all transits (useful for testing or reset)
func (tm *TransitManager) ClearAllTransits() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.transits = make(map[string]*Transit)
	tm.atTerritory = make(map[string][]*Transit)
}

// LoadTransits restores transits from saved state
func (tm *TransitManager) LoadTransits(transits []*Transit) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Clear existing transits
	tm.transits = make(map[string]*Transit)
	tm.atTerritory = make(map[string][]*Transit)

	// Restore each transit
	for _, transit := range transits {
		if transit == nil {
			continue
		}

		// Add to main transit map
		tm.transits[transit.ID] = transit

		// Add to territory lookup map (current position in route)
		if transit.RouteIndex < len(transit.Route) {
			currentTerritoryID := transit.Route[transit.RouteIndex]
			tm.atTerritory[currentTerritoryID] = append(tm.atTerritory[currentTerritoryID], transit)
		}

		// Update nextID counter to avoid ID conflicts
		// Extract numeric part from transit ID (format: "transit_123")
		var idNum uint64
		fmt.Sscanf(transit.ID, "transit_%d", &idNum)
		if idNum >= tm.nextID {
			tm.nextID = idNum + 1
		}
	}

	fmt.Printf("[TRANSIT] Loaded %d transits into TransitManager\n", len(transits))
}

// Helper function to get territory by ID using O(1) map lookup
// Note: This function assumes the caller already holds appropriate locks
func (s *state) getTerritoryByID(id string) *typedef.Territory {
	return s.territoryMap[id]
}

// Updated transit functions using the new system

// ResourceTraversalAndTaxV2 is the new version using the decoupled transit system
func ResourceTraversalAndTaxV2() {
	debugf("ResourceTraversalAndTaxV2 called at tick %d\n", st.tick)

	// 1. Find all HQs
	hqs := []*typedef.Territory{}
	for _, t := range st.territories {
		if t != nil && t.HQ {
			hqs = append(hqs, t)
		}
	}

	debugf("Found %d HQs\n", len(hqs))

	// 2. Process all territories for surplus/deficit
	// First pass: calculate net values without calling functions that might lock other territories
	type territoryNetInfo struct {
		territory  *typedef.Territory
		hasDeficit bool
	}
	territoriesWithDeficit := make([]*territoryNetInfo, 0)

	for _, territory := range st.territories {
		if territory == nil {
			continue
		}
		territory.Mu.Lock()

		if !territory.HQ {
			// DEBUG: Calculate Net field manually as it might not be set properly
			// Net = Generation - Costs
			territory.Net.Emeralds = territory.ResourceGeneration.At.Emeralds - territory.Costs.Emeralds
			territory.Net.Ores = territory.ResourceGeneration.At.Ores - territory.Costs.Ores
			territory.Net.Wood = territory.ResourceGeneration.At.Wood - territory.Costs.Wood
			territory.Net.Fish = territory.ResourceGeneration.At.Fish - territory.Costs.Fish
			territory.Net.Crops = territory.ResourceGeneration.At.Crops - territory.Costs.Crops

			// Check for deficit while we have the lock
			hasDeficit := territory.Net.Emeralds < 0 || territory.Net.Ores < 0 || territory.Net.Wood < 0 || territory.Net.Fish < 0 || territory.Net.Crops < 0

			// Store info for later processing
			territoriesWithDeficit = append(territoriesWithDeficit, &territoryNetInfo{
				territory:  territory,
				hasDeficit: hasDeficit,
			})

			// Handle surplus while we have this territory's lock (surplus doesn't need HQ locks)
			handleSurplusV2(territory, hqs)
		}

		territory.Mu.Unlock()
	}

	// Second pass: handle deficits without holding territory locks to avoid deadlocks
	for _, info := range territoriesWithDeficit {
		if info.hasDeficit {
			// Now we can safely call handleDeficitV2 without risk of deadlock
			// because we're not holding any territory locks
			handleDeficitV2(info.territory, hqs)
		}
	}

	// 3. Process all transits using the new system
	debugf("Processing all transits\n")
	st.transitManager.ProcessAllTransits()
	debugf("ResourceTraversalAndTaxV2 completed\n")
}

// handleSurplusV2 sends surplus resources to HQ using the new transit system
func handleSurplusV2(territory *typedef.Territory, hqs []*typedef.Territory) {
	if len(hqs) == 0 {
		return // No HQ to send to
	}

	// Find a valid route to an HQ
	route, hq := findRouteToHQV2(territory, hqs)
	if route == nil || hq == nil {
		return // No valid route
	}

	// Send all resources currently in storage to HQ
	resourcesToSend := territory.Storage.At

	// Check if there's anything to send
	if resourcesToSend.Emeralds <= 0 && resourcesToSend.Ores <= 0 && resourcesToSend.Wood <= 0 && resourcesToSend.Fish <= 0 && resourcesToSend.Crops <= 0 {
		return
	}

	// Remove all resources from storage (they're being sent to HQ)
	territory.Storage.At = typedef.BasicResources{}

	// Convert territory route to ID route
	routeIDs := make([]string, len(route))
	for i, t := range route {
		routeIDs[i] = t.ID
	}

	// Start transit using the new system
	st.transitManager.StartTransit(resourcesToSend, territory.ID, hq.ID, routeIDs)
}

// handleDeficitV2 sends resources from HQ to territory using the new transit system
func handleDeficitV2(territory *typedef.Territory, hqs []*typedef.Territory) {
	if len(hqs) == 0 {
		return // No HQ to send from
	}

	// DEBUG: Log when deficit handling is triggered (commented out for performance)
	// fmt.Printf("[DEBUG] Deficit handling for %s: E=%.2f, O=%.2f, W=%.2f, F=%.2f, C=%.2f\n",
	//	territory.Name, territory.Net.Emeralds, territory.Net.Ores, territory.Net.Wood, territory.Net.Fish, territory.Net.Crops)

	// Find a valid HQ and route
	var bestHQ *typedef.Territory
	var bestRoute []*typedef.Territory
	for _, hq := range hqs {
		route := findRouteFromHQToTerritoryV2(hq, territory)
		if route != nil {
			bestHQ = hq
			bestRoute = route
			break // For now, pick first valid
		}
	}
	if bestHQ == nil || bestRoute == nil {
		// fmt.Printf("[DEBUG] No valid route found from any HQ to %s\n", territory.Name)
		return // No valid route
	}

	// fmt.Printf("[DEBUG] Found route from %s to %s\n", bestHQ.Name, territory.Name)

	// Calculate how much is needed at destination (deficit)
	// The territory should have approximately 1 second worth of consumption left at :59
	// Transit delivery happens at :00, and next delivery is at next :00
	// So territory needs to survive for 59 seconds (from :01 to :59)
	// Formula: target_amount = net_consumption_per_hour * (59/3600)
	//          needed = target_amount - current_storage
	// Only send if needed > 0 (current storage is insufficient)

	// Get current storage to factor into calculation
	territory.Mu.RLock()
	currentStorage := territory.Storage.At
	territory.Mu.RUnlock()

	deficit := typedef.BasicResources{}

	if territory.Net.Emeralds < 0 {
		// Use NET consumption rate (deficit per hour)
		netDeficitPerHour := -territory.Net.Emeralds
		// Target: have enough for 59 seconds of NET consumption
		targetAmount := netDeficitPerHour * (61.0 / 3600.0)
		// Send the deficit: target amount minus what's already in storage
		needed := targetAmount - currentStorage.Emeralds
		if needed > 0 {
			deficit.Emeralds = needed
		}
	}
	if territory.Net.Ores < 0 {
		netDeficitPerHour := -territory.Net.Ores
		targetAmount := netDeficitPerHour * (61.0 / 3600.0)
		needed := targetAmount - currentStorage.Ores
		if needed > 0 {
			deficit.Ores = needed
		}
	}
	if territory.Net.Wood < 0 {
		netDeficitPerHour := -territory.Net.Wood
		targetAmount := netDeficitPerHour * (61.0 / 3600.0)
		needed := targetAmount - currentStorage.Wood
		if needed > 0 {
			deficit.Wood = needed
		}
	}
	if territory.Net.Fish < 0 {
		netDeficitPerHour := -territory.Net.Fish
		targetAmount := netDeficitPerHour * (61.0 / 3600.0)
		needed := targetAmount - currentStorage.Fish
		if needed > 0 {
			deficit.Fish = needed
		}
	}
	if territory.Net.Crops < 0 {
		netDeficitPerHour := -territory.Net.Crops
		targetAmount := netDeficitPerHour * (61.0 / 3600.0)
		needed := targetAmount - currentStorage.Crops
		if needed > 0 {
			deficit.Crops = needed
		}
	}

	// fmt.Printf("[DEBUG] Calculated deficit (per minute): E=%.2f, O=%.2f, W=%.2f, F=%.2f, C=%.2f\n",
	//	deficit.Emeralds, deficit.Ores, deficit.Wood, deficit.Fish, deficit.Crops)

	// Calculate total tax along the route
	totalTax := 1.0
	for i := 1; i < len(bestRoute); i++ {
		if bestRoute[i].Guild.Tag != bestHQ.Guild.Tag {
			totalTax *= (1.0 - bestRoute[i].Tax.Tax)
		}
	}

	// HQ must send extra to cover tax
	toSend := deficit
	if totalTax > 0 {
		toSend = scaleResources(deficit, 1.0/totalTax)
	}
	// Only send if HQ has enough - check and send each resource type individually
	bestHQ.Mu.Lock()
	defer bestHQ.Mu.Unlock()

	// Debug logging commented out for performance
	// fmt.Printf("[DEBUG] HQ %s storage: E=%.2f, O=%.2f, W=%.2f, F=%.2f, C=%.2f\n", ...)
	// fmt.Printf("[DEBUG] Trying to send: E=%.2f, O=%.2f, W=%.2f, F=%.2f, C=%.2f\n", ...)

	// Create actualSend with only resources that HQ can afford
	actualSend := typedef.BasicResources{}

	if toSend.Emeralds > 0 && bestHQ.Storage.At.Emeralds >= toSend.Emeralds {
		actualSend.Emeralds = toSend.Emeralds
		bestHQ.Storage.At.Emeralds -= toSend.Emeralds
	}
	if toSend.Ores > 0 && bestHQ.Storage.At.Ores >= toSend.Ores {
		actualSend.Ores = toSend.Ores
		bestHQ.Storage.At.Ores -= toSend.Ores
	}
	if toSend.Wood > 0 && bestHQ.Storage.At.Wood >= toSend.Wood {
		actualSend.Wood = toSend.Wood
		bestHQ.Storage.At.Wood -= toSend.Wood
	}
	if toSend.Fish > 0 && bestHQ.Storage.At.Fish >= toSend.Fish {
		actualSend.Fish = toSend.Fish
		bestHQ.Storage.At.Fish -= toSend.Fish
	}
	if toSend.Crops > 0 && bestHQ.Storage.At.Crops >= toSend.Crops {
		actualSend.Crops = toSend.Crops
		bestHQ.Storage.At.Crops -= toSend.Crops
	}

	// Debug logging commented out for performance
	// fmt.Printf("[DEBUG] Actually sending: E=%.2f, O=%.2f, W=%.2f, F=%.2f, C=%.2f\n", ...)

	// Check if there's anything to send
	if actualSend.Emeralds <= 0 && actualSend.Ores <= 0 && actualSend.Wood <= 0 && actualSend.Fish <= 0 && actualSend.Crops <= 0 {
		// Debug logging commented out for performance
		// fmt.Printf("[DEBUG] Nothing to send from %s to %s\n", bestHQ.Name, territory.Name)
		return // Nothing to send
	}

	// Convert territory route to ID route
	routeIDs := make([]string, len(bestRoute))
	for i, t := range bestRoute {
		routeIDs[i] = t.ID
	}

	// Start transit using the new system with actually sent resources
	_ = st.transitManager.StartTransit(actualSend, bestHQ.ID, territory.ID, routeIDs)
	// Debug logging commented out for performance
	// fmt.Printf("[DEBUG] Created transit %s from %s to %s\n", transitID, bestHQ.Name, territory.Name)
}

// findRouteToHQV2 finds a valid route from a territory to an HQ
func findRouteToHQV2(territory *typedef.Territory, hqs []*typedef.Territory) ([]*typedef.Territory, *typedef.Territory) {
	for _, hq := range hqs {
		for _, route := range territory.TradingRoutes {
			if len(route) > 0 && route[len(route)-1] == hq {
				return route, hq
			}
		}
	}
	return nil, nil
}

// findRouteFromHQToTerritoryV2 finds a valid route from HQ to a territory
func findRouteFromHQToTerritoryV2(hq *typedef.Territory, dest *typedef.Territory) []*typedef.Territory {
	for _, route := range hq.TradingRoutes {
		if len(route) > 0 && route[len(route)-1] == dest {
			return route
		}
	}
	return nil
}

// Compatibility functions for UI and other systems that need transit information

// GetTransitResourcesForTerritory returns transit resources at a territory in the old format
// This is for backward compatibility with UI code
func GetTransitResourcesForTerritory(territory *typedef.Territory) []typedef.InTransitResources {
	transits := st.transitManager.GetTransitsAtTerritory(territory.ID)
	result := make([]typedef.InTransitResources, len(transits))

	for i, transit := range transits {
		// Convert new Transit format back to old InTransitResources format
		origin := st.getTerritoryByID(transit.OriginID)
		destination := st.getTerritoryByID(transit.DestinationID)

		var next *typedef.Territory
		if transit.RouteIndex+1 < len(transit.Route) {
			next = st.getTerritoryByID(transit.Route[transit.RouteIndex+1])
		}

		// Convert route IDs back to territory pointers
		route := make([]*typedef.Territory, len(transit.Route))
		for j, routeID := range transit.Route {
			route[j] = st.getTerritoryByID(routeID)
		}

		result[i] = typedef.InTransitResources{
			BasicResources: transit.BasicResources,
			Origin:         origin,
			Destination:    destination,
			Next:           next,
			NextTax:        transit.NextTax,
			Route:          route,
			RouteIndex:     transit.RouteIndex,
			Moved:          transit.Moved,
		}
	}

	return result
}

func scaleResources(res typedef.BasicResources, factor float64) typedef.BasicResources {
	return typedef.BasicResources{
		Emeralds: res.Emeralds * factor,
		Ores:     res.Ores * factor,
		Wood:     res.Wood * factor,
		Fish:     res.Fish * factor,
		Crops:    res.Crops * factor,
	}
}

func applyTax(res typedef.BasicResources, tax float64) typedef.BasicResources {
	return typedef.BasicResources{
		Emeralds: res.Emeralds * (1.0 - tax),
		Ores:     res.Ores * (1.0 - tax),
		Wood:     res.Wood * (1.0 - tax),
		Fish:     res.Fish * (1.0 - tax),
		Crops:    res.Crops * (1.0 - tax),
	}
}
