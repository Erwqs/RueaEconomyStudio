package eruntime

import (
	"etools/typedef"
)

// ResourceTraversalAndTax is called every 60 ticks from update2()
func ResourceTraversalAndTax() {
	// 1. Find all HQs
	hqs := []*typedef.Territory{}
	for _, t := range st.territories {
		if t != nil && t.HQ {
			hqs = append(hqs, t)
		}
	}

	// 2. Traverse all territories for surplus/deficit
	for _, territory := range st.territories {
		if territory == nil {
			continue
		}
		territory.Mu.Lock()

		if !territory.HQ {
			// Non-HQ territories: send resources to HQ and request deficit
			handleSurplus(territory, hqs)
			// Deficit: HQ sends to territory
			if territory.Net.Emeralds < 0 || territory.Net.Ores < 0 || territory.Net.Wood < 0 || territory.Net.Fish < 0 || territory.Net.Crops < 0 {
				handleDeficit(territory, hqs)
			}
		}

		// All territories (including HQ) need to process incoming transit resources
		moveInTransit(territory)
		territory.Mu.Unlock()
	}
}

// Utility: Find a valid route from a territory to an HQ (returns route slice and HQ pointer)
func findRouteToHQ(territory *typedef.Territory, hqs []*typedef.Territory) ([]*typedef.Territory, *typedef.Territory) {
	for _, hq := range hqs {
		for _, route := range territory.TradingRoutes {
			if len(route) > 0 && route[len(route)-1] == hq {
				return route, hq
			}
		}
	}
	return nil, nil
}

// Utility: Find a valid route from HQ to a territory (returns route slice)
func findRouteFromHQToTerritory(hq *typedef.Territory, dest *typedef.Territory) []*typedef.Territory {
	for _, route := range hq.TradingRoutes {
		if len(route) > 0 && route[len(route)-1] == dest {
			return route
		}
	}
	return nil
}

// Surplus: Send resource to HQ
func handleSurplus(territory *typedef.Territory, hqs []*typedef.Territory) {
	if len(hqs) == 0 {
		return // No HQ to send to
	}
	// Find a valid route to an HQ
	route, hq := findRouteToHQ(territory, hqs)
	if route == nil || hq == nil {
		return // No valid route
	}

	// Send all resources currently in storage to HQ
	// This includes resources that exceed capacity due to manual edits
	resourcesToSend := territory.Storage.At

	// Check if there's anything to send
	if resourcesToSend.Emeralds <= 0 && resourcesToSend.Ores <= 0 && resourcesToSend.Wood <= 0 && resourcesToSend.Fish <= 0 && resourcesToSend.Crops <= 0 {
		return
	}

	// Remove all resources from storage (they're being sent to HQ)
	territory.Storage.At = typedef.BasicResources{}

	inTransit := typedef.InTransitResources{
		BasicResources: resourcesToSend,
		Origin:         territory,
		Destination:    hq,
		Next:           route[1],         // Next territory in the route
		NextTax:        route[1].Tax.Tax, // Tax for next territory (could check for ally)
		Route:          route,            // Store the full route
		RouteIndex:     0,                // Start at index 0 (origin territory)
		Moved:          true,             // Mark as moved to prevent double processing
	}
	// Deduct tax if next territory is not same guild
	if route[1].Guild.Tag != territory.Guild.Tag {
		inTransit.BasicResources = applyTax(inTransit.BasicResources, route[1].Tax.Tax)
	}

	// Safely add to next territory's transit resources
	// We need to be careful about lock ordering to avoid deadlocks
	nextTerritory := route[1]
	if nextTerritory != territory { // Only lock if it's a different territory
		nextTerritory.Mu.Lock()
		nextTerritory.TransitResource = append(nextTerritory.TransitResource, inTransit)
		nextTerritory.Mu.Unlock()
	} else {
		// Same territory, already locked
		nextTerritory.TransitResource = append(nextTerritory.TransitResource, inTransit)
	}
}

// Deficit: HQ sends resource to territory
func handleDeficit(territory *typedef.Territory, hqs []*typedef.Territory) {
	if len(hqs) == 0 {
		return // No HQ to send from
	}
	// Find a valid HQ and route
	var bestHQ *typedef.Territory
	var bestRoute []*typedef.Territory
	for _, hq := range hqs {
		route := findRouteFromHQToTerritory(hq, territory)
		if route != nil {
			bestHQ = hq
			bestRoute = route
			break // For now, pick first valid
		}
	}
	if bestHQ == nil || bestRoute == nil {
		return // No valid route
	}
	// Calculate how much is needed at destination (deficit)
	// Send 59 ticks worth of deficit resources

	deficit := typedef.BasicResources{}
	if territory.Net.Emeralds < 0 {
		deficit.Emeralds = -territory.Net.Emeralds * 59 // 60 ticks worth
	}
	if territory.Net.Ores < 0 {
		deficit.Ores = -territory.Net.Ores * 59
	}
	if territory.Net.Wood < 0 {
		deficit.Wood = -territory.Net.Wood * 59
	}
	if territory.Net.Fish < 0 {
		deficit.Fish = -territory.Net.Fish * 59
	}
	if territory.Net.Crops < 0 {
		deficit.Crops = -territory.Net.Crops * 59
	}
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
	// Only send if HQ has enough
	if bestHQ.Storage.At.Emeralds < toSend.Emeralds || bestHQ.Storage.At.Ores < toSend.Ores || bestHQ.Storage.At.Wood < toSend.Wood || bestHQ.Storage.At.Fish < toSend.Fish || bestHQ.Storage.At.Crops < toSend.Crops {
		return // Not enough resource
	}
	// Remove from HQ storage
	bestHQ.Storage.At.Emeralds -= toSend.Emeralds
	bestHQ.Storage.At.Ores -= toSend.Ores
	bestHQ.Storage.At.Wood -= toSend.Wood
	bestHQ.Storage.At.Fish -= toSend.Fish
	bestHQ.Storage.At.Crops -= toSend.Crops
	// Create InTransit and add to next territory
	inTransit := typedef.InTransitResources{
		BasicResources: toSend,
		Origin:         bestHQ,
		Destination:    territory,
		Next:           bestRoute[1],
		NextTax:        bestRoute[1].Tax.Tax,
		Route:          bestRoute, // Store the full route
		RouteIndex:     0,         // Start at index 0 (HQ)
	}
	if bestRoute[1].Guild.Tag != bestHQ.Guild.Tag {
		inTransit.BasicResources = applyTax(inTransit.BasicResources, bestRoute[1].Tax.Tax)
	}

	// Safely add to next territory's transit resources with proper locking
	nextTerritory := bestRoute[1]
	if nextTerritory != bestHQ && nextTerritory != territory { // Only lock if it's a different territory
		nextTerritory.Mu.Lock()
		nextTerritory.TransitResource = append(nextTerritory.TransitResource, inTransit)
		nextTerritory.Mu.Unlock()
	} else {
		// Same territory as HQ or destination, handle carefully
		// Note: HQ is not locked in this context, so we need to lock it
		if nextTerritory == bestHQ {
			bestHQ.Mu.Lock()
			nextTerritory.TransitResource = append(nextTerritory.TransitResource, inTransit)
			bestHQ.Mu.Unlock()
		} else {
			// nextTerritory == territory, already locked
			nextTerritory.TransitResource = append(nextTerritory.TransitResource, inTransit)
		}
	}
}

// Move resources in transit to next territory, deducting tax and handling edge cases
func moveInTransit(territory *typedef.Territory) {
	newTransit := []typedef.InTransitResources{}
	for _, tr := range territory.TransitResource {
		if tr.Next == nil || tr.Destination == nil {
			continue // Invalid
		}
		// Edge case: border closed - void resource if can't pass through
		if tr.Next.Border == typedef.BorderClosed && tr.Next.Guild.Tag != territory.Guild.Tag {
			continue // Border closed, void resource
		}
		if tr.Next.Border == typedef.BorderClosed && tr.Next.Guild.Tag != territory.Guild.Tag {
			continue // Border closed, void resource
		}
		// Move to next territory
		next := tr.Next

		// Avoid deadlock: don't lock if next is the same as current territory
		needsLock := (next != territory)
		if needsLock {
			next.Mu.Lock()
		}
		// Deduct tax if next territory is not same guild
		if next.Guild.Tag != territory.Guild.Tag {
			// --- Begin new logic for immediate tax delivery ---
			preTax := tr.BasicResources
			tr.BasicResources = applyTax(tr.BasicResources, next.Tax.Tax)
			taxed := typedef.BasicResources{
				Emeralds: preTax.Emeralds - tr.BasicResources.Emeralds,
				Ores:     preTax.Ores - tr.BasicResources.Ores,
				Wood:     preTax.Wood - tr.BasicResources.Wood,
				Fish:     preTax.Fish - tr.BasicResources.Fish,
				Crops:    preTax.Crops - tr.BasicResources.Crops,
			}
			// Find HQ of the foreign guild (next.Guild.Tag)
			var foreignHQ *typedef.Territory
			for _, t := range st.territories {
				if t != nil && t.HQ && t.Guild.Tag == next.Guild.Tag {
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
		// If next is destination, check if it's still the correct destination
		if next == tr.Destination {
			// Check if this is an HQ that was captured (different guild than origin)
			if tr.Destination.HQ && (tr.Destination.Guild.Tag != tr.Origin.Guild.Tag) {
				// HQ was captured! Try to reroute to the new guild's HQ
				newGuildHQs := []*typedef.Territory{}
				for _, t := range st.territories {
					if t != nil && t.HQ && t.Guild.Tag == tr.Destination.Guild.Tag {
						newGuildHQs = append(newGuildHQs, t)
					}
				}

				// Try to find a route from current location to new guild's HQ
				newRoute, newHQ := findRouteToHQ(next, newGuildHQs)
				if newRoute != nil && newHQ != nil && len(newRoute) > 1 {
					// Valid route found to new guild's HQ, reroute the transit
					tr.Destination = newHQ
					tr.Next = newRoute[1]
					tr.Route = newRoute
					tr.RouteIndex = 0

					// Apply tax if next territory is not same guild as current
					if newRoute[1].Guild.Tag != next.Guild.Tag {
						tr.BasicResources = applyTax(tr.BasicResources, newRoute[1].Tax.Tax)
					}

					// Check if border is closed
					if newRoute[1].Border == typedef.BorderClosed && newRoute[1].Guild.Tag != next.Guild.Tag {
						// Border closed, void resource
						continue
					}

					// Continue transit to new destination
					newRoute[1].Mu.Lock()
					newRoute[1].TransitResource = append(newRoute[1].TransitResource, tr)
					newRoute[1].Mu.Unlock()
					continue
				}
				// No valid route to new guild's HQ, void the resource
				continue
			}

			// Normal destination arrival - add to storage (capped at capacity)
			currentStorage := next.Storage.At
			maxStorage := next.Storage.Capacity

			potentialEmeralds := currentStorage.Emeralds + tr.BasicResources.Emeralds
			potentialOres := currentStorage.Ores + tr.BasicResources.Ores
			potentialWood := currentStorage.Wood + tr.BasicResources.Wood
			potentialFish := currentStorage.Fish + tr.BasicResources.Fish
			potentialCrops := currentStorage.Crops + tr.BasicResources.Crops

			next.Storage.At.Emeralds = min(potentialEmeralds, maxStorage.Emeralds)
			next.Storage.At.Ores = min(potentialOres, maxStorage.Ores)
			next.Storage.At.Wood = min(potentialWood, maxStorage.Wood)
			next.Storage.At.Fish = min(potentialFish, maxStorage.Fish)
			next.Storage.At.Crops = min(potentialCrops, maxStorage.Crops)

			// Set overflow warnings if any resource was capped
			if potentialEmeralds > maxStorage.Emeralds {
				next.Warning |= typedef.WarningOverflowEmerald
			}
			if potentialOres > maxStorage.Ores || potentialWood > maxStorage.Wood ||
				potentialFish > maxStorage.Fish || potentialCrops > maxStorage.Crops {
				next.Warning |= typedef.WarningOverflowResources
			}
		} else {
			// Otherwise, continue transit using the stored route
			if tr.Route != nil && tr.RouteIndex < len(tr.Route)-2 {
				// Move to next position in the route
				tr.RouteIndex++
				tr.Next = tr.Route[tr.RouteIndex+1] // Next territory after current position
				if tr.Next != nil {
					next.TransitResource = append(next.TransitResource, tr)
				}
			} else {
				// No valid next territory in route, resource is voided
			}
		}
		if needsLock {
			next.Mu.Unlock()
		}
	}
	territory.TransitResource = newTransit // Clear all after moving
}

// Utility: Apply tax to resources
func applyTax(res typedef.BasicResources, tax float64) typedef.BasicResources {
	return typedef.BasicResources{
		Emeralds: res.Emeralds * (1.0 - tax),
		Ores:     res.Ores * (1.0 - tax),
		Wood:     res.Wood * (1.0 - tax),
		Fish:     res.Fish * (1.0 - tax),
		Crops:    res.Crops * (1.0 - tax),
	}
}

// Utility: Scale resources by a factor
func scaleResources(res typedef.BasicResources, factor float64) typedef.BasicResources {
	return typedef.BasicResources{
		Emeralds: res.Emeralds * factor,
		Ores:     res.Ores * factor,
		Wood:     res.Wood * factor,
		Fish:     res.Fish * factor,
		Crops:    res.Crops * factor,
	}
}
