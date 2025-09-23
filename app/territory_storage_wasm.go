//go:build js && wasm

package app

// In-memory storage for territory claims in WASM
var wasmTerritoryClaims map[string]TerritoryClaim

// saveTerritoryClaimsToMemory saves territory claims to memory instead of file
func saveTerritoryClaims(claims map[string]TerritoryClaim) error {
	wasmTerritoryClaims = make(map[string]TerritoryClaim)
	for k, v := range claims {
		wasmTerritoryClaims[k] = v
	}
	// fmt.Printf("[TERRITORY_STORAGE] Saved %d territory claims to WASM memory\n", len(claims))
	return nil
}

// loadTerritoryClaimsFromMemory loads territory claims from memory instead of file
func loadTerritoryClaimsFromMemory() (map[string]TerritoryClaim, error) {
	if wasmTerritoryClaims == nil {
		// fmt.Println("[TERRITORY_STORAGE] No territory claims in WASM memory, returning empty")
		return make(map[string]TerritoryClaim), nil
	}

	result := make(map[string]TerritoryClaim)
	for k, v := range wasmTerritoryClaims {
		result[k] = v
	}
	// fmt.Printf("[TERRITORY_STORAGE] Loaded %d territory claims from WASM memory\n", len(result))
	return result, nil
}
