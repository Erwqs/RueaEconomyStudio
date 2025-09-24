//go:build !js || !wasm

package app

// saveGuildsToWebStorage is a no-op on non-web platforms
func (gm *EnhancedGuildManager) saveGuildsToWebStorage() {
	// No-op for non-WASM builds - use file system instead
}

// loadGuildsFromWebStorage is a no-op on non-web platforms
func (gm *EnhancedGuildManager) loadGuildsFromWebStorage() {
	// No-op for non-WASM builds - use file system instead
}

// saveGuildsToMemory stub for non-WASM platforms (should never be called)
func saveGuildsToMemory(guilds []EnhancedGuildData) error {
	// This should never be called on non-WASM platforms
	// The code checks runtime.GOOS == "js" before calling this
	panic("saveGuildsToMemory called on non-WASM platform")
}

// loadGuildsFromMemory stub for non-WASM platforms (should never be called)
func loadGuildsFromMemory() ([]EnhancedGuildData, error) {
	// This should never be called on non-WASM platforms
	// The code checks runtime.GOOS == "js" before calling this
	panic("loadGuildsFromMemory called on non-WASM platform")
}
