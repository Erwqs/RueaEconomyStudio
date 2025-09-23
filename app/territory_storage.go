//go:build !js || !wasm

package app

import (
	"encoding/json"
	"os"
)

// saveTerritoryClaims saves territory claims to file for non-WASM builds
func saveTerritoryClaims(claims map[string]TerritoryClaim) error {
	// Convert map to slice for JSON serialization
	claimsSlice := make([]TerritoryClaim, 0, len(claims))
	for _, claim := range claims {
		claimsSlice = append(claimsSlice, claim)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(claimsSlice, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile("territory_claims.json", data, 0644)
}

// loadTerritoryClaimsFromMemory loads territory claims from file for non-WASM builds
func loadTerritoryClaimsFromMemory() (map[string]TerritoryClaim, error) {
	// For non-WASM builds, this function should not be used
	// The regular file loading should be used instead
	return make(map[string]TerritoryClaim), nil
}
