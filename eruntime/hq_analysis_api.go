package eruntime

import (
	"RueaES/alg"
	"RueaES/typedef"
	"maps"
)

// ComputeHQSuggestionsForGuild evaluates the best HQ placements for a guild using connection-driven scoring.
func ComputeHQSuggestionsForGuild(guildTag string) ([]alg.HQCandidate, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	territories := make(map[string]*typedef.Territory, len(TerritoryMap))
	maps.Copy(territories, TerritoryMap)

	return alg.ComputeHQCandidates(guildTag, territories)
}
