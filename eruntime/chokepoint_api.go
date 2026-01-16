package eruntime

import (
	"RueaES/alg"
	"RueaES/typedef"
)

// ComputeChokepointsForGuild runs chokepoint analysis for a single guild on-demand.
// It does not run automatically; callers should invoke when needed (e.g., via API/UI action).
func ComputeChokepointsForGuild(guildTag string) (map[string]alg.ChokeReport, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	territories := make(map[string]*typedef.Territory, len(TerritoryMap))
	for name, t := range TerritoryMap {
		territories[name] = t
	}

	routes := make(map[string][]string, len(TradingRoutesMap))
	for name, adj := range TradingRoutesMap {
		copied := make([]string, len(adj))
		copy(copied, adj)
		routes[name] = copied
	}

	opts := st.runtimeOptions

	return alg.ComputeChokepoints(guildTag, territories, routes, opts.ChokepointEmeraldWeight, opts.ChokepointIncludeDownstream)
}
