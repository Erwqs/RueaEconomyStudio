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
