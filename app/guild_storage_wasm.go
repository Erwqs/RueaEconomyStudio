//go:build js && wasm

package app

// In-memory storage for guild data in WASM
var wasmGuildData []EnhancedGuildData

// saveGuildsToMemory saves guild data to memory instead of file
func saveGuildsToMemory(guilds []EnhancedGuildData) error {
	wasmGuildData = make([]EnhancedGuildData, len(guilds))
	copy(wasmGuildData, guilds)
	// fmt.Printf("[GUILD_STORAGE] Saved %d guilds to WASM memory\n", len(guilds))
	return nil
}

// loadGuildsFromMemory loads guild data from memory instead of file
func loadGuildsFromMemory() ([]EnhancedGuildData, error) {
	if wasmGuildData == nil {
		// fmt.Println("[GUILD_STORAGE] No guild data in WASM memory, returning empty")
		return []EnhancedGuildData{}, nil
	}

	result := make([]EnhancedGuildData, len(wasmGuildData))
	copy(result, wasmGuildData)
	// fmt.Printf("[GUILD_STORAGE] Loaded %d guilds from WASM memory\n", len(result))
	return result, nil
}

// checkGuildsFileExists always returns false for WASM since we use memory
func checkGuildsFileExists() bool {
	exists := wasmGuildData != nil && len(wasmGuildData) > 0
	// fmt.Printf("[GUILD_STORAGE] WASM guild data exists: %v\n", exists)
	return exists
}
