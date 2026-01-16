package alg

import (
	"errors"
	"sort"
	"strings"

	"RueaES/typedef"
)

var ErrNoTerritoriesForGuild = errors.New("no territories found for guild")

// HQCandidate describes a territory scored for HQ placement suitability.
type HQCandidate struct {
	Name                string  `json:"name"`
	DirectConnections   int     `json:"directConnections"`
	ExternalConnections int     `json:"externalConnections"`
	Score               float64 `json:"score"`
}

// ComputeHQCandidates ranks a guild's territories by HQ suitability using connection-driven bonuses.
// Score approximates the tower multiplier: (1 + 0.3*connections) * (1.5 + 0.25*externals).
func ComputeHQCandidates(guildTag string, territories map[string]*typedef.Territory) ([]HQCandidate, error) {
	tag := strings.TrimSpace(guildTag)
	if tag == "" {
		return nil, ErrGuildTagEmpty
	}

	candidates := make([]HQCandidate, 0)

	for name, t := range territories {
		if t == nil {
			continue
		}

		t.Mu.RLock()
		tagMatch := t.Guild.Tag == tag
		direct := len(t.Links.Direct)
		externals := len(t.Links.Externals)
		t.Mu.RUnlock()

		if !tagMatch {
			continue
		}

		linkBonus := 1.0 + 0.3*float64(direct)
		externalBonus := 1.5 + 0.25*float64(externals)
		score := linkBonus * externalBonus

		candidates = append(candidates, HQCandidate{
			Name:                name,
			DirectConnections:   direct,
			ExternalConnections: externals,
			Score:               score,
		})
	}

	if len(candidates) == 0 {
		return nil, ErrNoTerritoriesForGuild
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].ExternalConnections == candidates[j].ExternalConnections {
				if candidates[i].DirectConnections == candidates[j].DirectConnections {
					return candidates[i].Name < candidates[j].Name
				}
				return candidates[i].DirectConnections > candidates[j].DirectConnections
			}
			return candidates[i].ExternalConnections > candidates[j].ExternalConnections
		}
		return candidates[i].Score > candidates[j].Score
	})

	return candidates, nil
}
