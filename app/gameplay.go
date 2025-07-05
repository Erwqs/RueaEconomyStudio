package app

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// GameplayModule demonstrates advanced input handling patterns
type GameplayModule struct {
	keyEventCh         <-chan KeyEvent
	mouseButtonEventCh <-chan MouseButtonEvent
	active             bool
	playerX            float64
	playerY            float64
	moveSpeed          float64
	lastMoveTime       time.Time
	keyHoldStates      map[ebiten.Key]bool
	mapView            *MapView              // Added map view component
	guildManager       *EnhancedGuildManager // Added enhanced guild manager component
}

// NewGameplayModule creates a new gameplay module
func NewGameplayModule(inputManager *InputManager) *GameplayModule {
	// Create a new MapView
	mapView := NewMapView()

	// Set the global MapView singleton for overlay and territory access
	SetMapView(mapView)

	// Create a new EnhancedGuildManager
	guildManager := NewEnhancedGuildManager()
	guildManager.SetInputManager(inputManager)

	// Initialize the GuildClaimManager
	_ = GetGuildClaimManager() // Ensure the singleton is created

	module := &GameplayModule{
		keyEventCh:         inputManager.Subscribe(),
		mouseButtonEventCh: inputManager.SubscribeMouseEvents(),
		active:             false,
		playerX:            100,
		playerY:            100,
		moveSpeed:          2.0,
		lastMoveTime:       time.Now(),
		keyHoldStates:      make(map[ebiten.Key]bool),
		mapView:            mapView,
		guildManager:       guildManager,
	}

	// Set the guildManager in the territories manager
	if mapView != nil && mapView.territoriesManager != nil {
		mapView.territoriesManager.SetGuildManager(guildManager)

		// Set up callback for guild data changes to invalidate territory cache
		guildManager.SetGuildDataChangedCallback(func() {
			fmt.Printf("[GAMEPLAY] Guild data changed callback triggered, invalidating territory cache\n")
			mapView.territoriesManager.InvalidateTerritoryCache()
		})
	}

	// Set up the edit claim callback to connect guild manager to map view
	guildManager.SetEditClaimCallback(func(guildName, guildTag string) {
		if mapView != nil {
			// Hide the guild management menu
			guildManager.Hide()

			// Start claim editing mode
			mapView.StartClaimEditing(guildName, guildTag)
		}
	})

	return module
}

// SetActive enables or disables the gameplay module
func (gm *GameplayModule) SetActive(active bool) {
	gm.active = active
	if active {
		// fmt.Println("[GAMEPLAY] Gameplay module activated - Use WASD/Arrow keys to move, SHIFT to run, SPACE to jump")
		gm.lastMoveTime = time.Now()
	} else {
		// fmt.Println("[GAMEPLAY] Gameplay module deactivated")
		// Clear all key hold states when deactivating
		for key := range gm.keyHoldStates {
			gm.keyHoldStates[key] = false
		}
	}
}

// IsActive returns whether the gameplay module is active
func (gm *GameplayModule) IsActive() bool {
	return gm.active
}

// Update processes input and updates gameplay state
func (gm *GameplayModule) Update() {
	if !gm.active {
		return
	}

	// Process key events to track hold states
	for {
		select {
		case event := <-gm.keyEventCh:
			gm.handleKeyEvent(event)
		default:
			// No more events to process
			goto keyEventsProcessed
		}
	}
keyEventsProcessed:

	// Process mouse button events
	for {
		select {
		case event := <-gm.mouseButtonEventCh:
			gm.handleMouseButtonEvent(event)
		default:
			// No more events to process
			goto mouseEventsProcessed
		}
	}
mouseEventsProcessed:

	// Get screen dimensions for map positioning
	screenW, screenH := ebiten.WindowSize()

	// Update guild manager first and check if it handled the input
	inputHandled := false
	if gm.guildManager != nil {
		inputHandled = gm.guildManager.Update()
	}

	// Update loadout manager and check if it handled the input
	if !inputHandled {
		loadoutManager := GetLoadoutManager()
		if loadoutManager != nil && loadoutManager.IsVisible() {
			inputHandled = loadoutManager.Update()
		}
	}

	// Update map view if it exists and input wasn't handled by guild manager
	if gm.mapView != nil && !inputHandled {
		gm.mapView.Update(screenW, screenH)
	}
}

// handleKeyEvent processes individual key events
func (gm *GameplayModule) handleKeyEvent(event KeyEvent) {
	// Update key hold states
	gm.keyHoldStates[event.Key] = event.Pressed

	// Handle one-time key press actions
	if event.Pressed {
		switch event.Key {
		case ebiten.KeySpace:
			// fmt.Printf("[GAMEPLAY] Player jumped!\n")
		case ebiten.KeyShift:
			gm.moveSpeed = 4.0 // Run speed
		case ebiten.KeyR:
			// Check if any text input is currently focused before resetting map view
			textInputFocused := false

			// Check if guild manager is open and has text input focused
			if gm.mapView != nil && gm.mapView.territoriesManager != nil {
				guildManager := gm.mapView.territoriesManager.guildManager
				if guildManager != nil && guildManager.IsVisible() && guildManager.HasTextInputFocused() {
					textInputFocused = true
				}
			}

			// Check if loadout manager is open and has text input focused
			loadoutManager := GetLoadoutManager()
			if loadoutManager != nil && loadoutManager.IsVisible() && loadoutManager.HasTextInputFocused() {
				textInputFocused = true
			}

			// Check if transit resource menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() && gm.mapView.transitResourceMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Check if tribute menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Reset map view only if no text input is focused
			if !textInputFocused && gm.mapView != nil {
				gm.mapView.ResetView()
			}
		case ebiten.KeyEqual, ebiten.KeyNumpadAdd:
			// Check if tribute menu is active and has text input focused
			textInputFocused := false
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}
			// Zoom in with smooth zoom
			if !textInputFocused && gm.mapView != nil {
				mx, my := ebiten.CursorPosition()
				gm.mapView.handleSmoothZoom(3.0, mx, my)
			}
		case ebiten.KeyMinus, ebiten.KeyNumpadSubtract:
			// Check if tribute menu is active and has text input focused
			textInputFocused := false
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}
			// Zoom out with smooth zoom
			if !textInputFocused && gm.mapView != nil {
				mx, my := ebiten.CursorPosition()
				gm.mapView.handleSmoothZoom(-3.0, mx, my)
			}
		case ebiten.KeyC:
			// Check if tribute menu is active and has text input focused
			textInputFocused := false
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}
			// Toggle coordinates
			if !textInputFocused && gm.mapView != nil {
				gm.mapView.ToggleCoordinates()
			}
		case ebiten.KeyM:
			// Check if tribute menu is active and has text input focused
			textInputFocused := false
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}
			// Add test marker at current mouse position
			if !textInputFocused && gm.mapView != nil {
				mx, my := ebiten.CursorPosition()
				mapX, mapY := gm.mapView.GetMapCoordinates(mx, my)
				markerName := fmt.Sprintf("M%d", int(mapX))
				gm.mapView.AddMarker(mapX, mapY, markerName, color.RGBA{255, 0, 0, 255}, 10)
			}
		case ebiten.KeyX:
			// Check if tribute menu is active and has text input focused
			textInputFocused := false
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}
			// Clear all markers
			if !textInputFocused && gm.mapView != nil {
				gm.mapView.ClearMarkers()
			}
		case ebiten.KeyG:
			// Check if any text input is currently focused before opening guild manager
			textInputFocused := false

			// Check if guild manager is open and has text input focused
			if gm.mapView != nil && gm.mapView.territoriesManager != nil {
				guildManager := gm.mapView.territoriesManager.guildManager
				if guildManager != nil && guildManager.IsVisible() && guildManager.HasTextInputFocused() {
					textInputFocused = true
				}
			}

			// Check if loadout manager is open and has text input focused
			loadoutManager := GetLoadoutManager()
			if loadoutManager != nil && loadoutManager.IsVisible() && loadoutManager.HasTextInputFocused() {
				textInputFocused = true
			}

			// Check if transit resource menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() && gm.mapView.transitResourceMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Check if tribute menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Only open guild manager if no text input is focused
			if !textInputFocused {
				// Open guild editor/management only if not already open
				if gm.mapView != nil && gm.mapView.territoriesManager != nil {
					// Check if guild manager is already visible
					if !gm.mapView.territoriesManager.guildManager.IsVisible() {
						// Hide transit resource menu when entering guild management mode
						if gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() {
							gm.mapView.transitResourceMenu.Hide()
						}
						gm.mapView.territoriesManager.OpenGuildManagement()
					}
				}
			}
		case ebiten.KeyL:
			// Check if any text input is currently focused before opening loadout manager
			textInputFocused := false

			// Check if guild manager is open and has text input focused
			if gm.mapView != nil && gm.mapView.territoriesManager != nil {
				guildManager := gm.mapView.territoriesManager.guildManager
				if guildManager.IsVisible() {
					if guildManager.HasTextInputFocused() {
						textInputFocused = true
					}
				}
			}

			// Check if loadout manager is open and has text input focused
			loadoutManager := GetLoadoutManager()
			if loadoutManager != nil && loadoutManager.IsVisible() {
				if loadoutManager.HasTextInputFocused() {
					textInputFocused = true
				}
			}

			// Check if transit resource menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() && gm.mapView.transitResourceMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Check if tribute menu is open and has text input focused
			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Only open loadout manager if no text input is focused
			if !textInputFocused {
				if loadoutManager != nil && !loadoutManager.IsVisible() {
					// Hide transit resource menu when entering loadout application mode
					if gm.mapView != nil && gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() {
						gm.mapView.transitResourceMenu.Hide()
					}
					loadoutManager.Show()
				}
			}
		}
	} else {
		// Handle key release actions
		switch event.Key {
		case ebiten.KeyShift:
			gm.moveSpeed = 2.0 // Walk speed
		}
	}
}

// handleMouseButtonEvent processes mouse button events
func (gm *GameplayModule) handleMouseButtonEvent(event MouseButtonEvent) {
	if !event.Pressed {
		return // Only handle button presses, not releases
	}

	// Handle mouse button actions
	switch event.Button {
	case ebiten.MouseButton3: // Back button
		// Check if we're in claim editing mode first
		if gm.mapView != nil && gm.mapView.IsEditingClaims() {
			// Cancel claim editing
			gm.mapView.CancelClaimEditing()
		}
		// Otherwise, back button is handled in the InputManager by simulating an Escape key press
	case ebiten.MouseButton4: // Forward button
		// Forward button now opens state management menu (handled in map view)
		// No longer resets the map view
	}
}

// IsTerritoryMenuOpen returns whether a territory menu is currently open
func (gm *GameplayModule) IsTerritoryMenuOpen() bool {
	// Check if we have a map view and a territory manager with an open menu
	if gm.mapView != nil && gm.mapView.territoriesManager != nil {
		return gm.mapView.territoriesManager.IsSideMenuOpen()
	}
	return false
}

// GetMapView returns the map view
func (gm *GameplayModule) GetMapView() *MapView {
	return gm.mapView
}

// Draw renders the gameplay elements on the screen
func (gm *GameplayModule) Draw(screen *ebiten.Image) {
	if !gm.active {
		return
	}

	// Draw map view if it exists
	if gm.mapView != nil {
		gm.mapView.Draw(screen)
	}

	// Draw guild manager if it exists and is visible
	if gm.guildManager != nil && gm.guildManager.IsVisible() {
		gm.guildManager.Draw(screen)
	}

	// Draw loadout manager if it exists and is visible
	loadoutManager := GetLoadoutManager()
	if loadoutManager != nil && loadoutManager.IsVisible() {
		loadoutManager.Draw(screen)
	}
}
