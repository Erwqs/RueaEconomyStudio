//go:build wasm
// +build wasm

package app

import (
	"etools/eruntime"
	"runtime"
)

// NewWebGame creates a new Game instance specifically configured for WebAssembly
// This version disables web-incompatible features and optimizes for browser environment
func NewWebGame() *Game {
	// Verify we're in the correct environment
	if runtime.GOOS != "js" || runtime.GOARCH != "wasm" {
		panic("NewWebGame should only be called in WebAssembly environment")
	}

	inputManager := NewInputManager()
	keyEventCh := inputManager.Subscribe()
	debugModule := NewDebugModule(inputManager)
	settingsModule := NewSettingsModule(inputManager)
	gameplayModule := NewGameplayModule(inputManager)

	// NOTE: File system manager is disabled for web - no native file dialogs
	// fileSystemManager := NewFileSystemManager(inputManager)

	game := &Game{
		state: &State{
			gameState:      StateSession, // Skip main menu, go straight to game
			inputManager:   inputManager,
			keyEventCh:     keyEventCh,
			debugModule:    debugModule,
			settingsModule: settingsModule,
			gameplayModule: gameplayModule,
			// fileSystemManager: nil, // Disabled for web
		},
	}

	// Initialize the menu and settings screen
	game.state.menu = game.createMenu()
	game.state.settingsScreen = NewSettingsScreen(settingsModule)

	// Since we're starting in StateSession, activate the gameplay module
	if game.state.gameplayModule != nil {
		game.state.gameplayModule.SetActive(true)
	}

	// Initialize session manager
	game.state.sessionManager = NewSessionManager(func() {
		// No menu return in web version - stay in session
	})

	// Set up the callback for returning to menu from settings
	game.state.settingsScreen.backToMenuCallback = func() {
		// No menu return in web version - stay in session
	}

	// Check if autosave was loaded on startup - if not, show welcome screen
	if !eruntime.WasAutoSaveLoadedOnStartup() {
		// No autosave file was found/loaded, show welcome screen
		welcomeScreen := GetWelcomeScreen()

		// Set up callback to access guild manager
		welcomeScreen.SetImportGuildsCallback(func() {
			if game.state.gameplayModule != nil && game.state.gameplayModule.guildManager != nil {
				// Disable file import in web version
				// Instead, we could provide alternative import methods
			}
		})

		// Show welcome screen (it's a modal overlay, not a separate state)
		welcomeScreen.Show()
	}

	return game
}
