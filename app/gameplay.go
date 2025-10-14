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
	scriptManager      *ScriptManager        // Added script manager component
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

	// Create a new ScriptManager
	scriptManager := NewScriptManager()

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
		scriptManager:      scriptManager,
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

	// Check if welcome screen is visible - if so, update it and block other input
	welcomeScreen := GetWelcomeScreen()
	if welcomeScreen != nil && welcomeScreen.IsVisible() {
		welcomeScreen.Update()
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

	// Update script manager and check if it handled the input
	if !inputHandled && gm.scriptManager != nil && gm.scriptManager.IsVisible() {
		gm.scriptManager.Update()
		inputHandled = true
	}

	// Update loadout manager and check if it handled the input
	if !inputHandled {
		loadoutManager := GetLoadoutManager()
		if loadoutManager != nil && loadoutManager.IsVisible() {
			inputHandled = loadoutManager.Update()
		}
	}

	// Update welcome screen and check if it handled the input
	if !inputHandled {
		welcomeScreen := GetWelcomeScreen()
		if welcomeScreen != nil && welcomeScreen.IsVisible() {
			inputHandled = welcomeScreen.Update()
		}
	}

	// Update undo explorer and check if it handled the input
	if !inputHandled {
		undoExplorer := GetUndoExplorer()
		if undoExplorer != nil && undoExplorer.IsVisible() {
			inputHandled = undoExplorer.Update()
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
		case ebiten.KeyZ:
			// Check if any text input is focused before handling Z key
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

			// Only handle Z key if no text input is focused
			if !textInputFocused {
				// Check if Ctrl is held for undo, otherwise show undo explorer
				if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) {
					// Ctrl+Z: Undo last edit
					um := GetUndoManager()
					if territoryName, ok := um.Undo(); ok {
						// Show toast notification
						NewToast().
							Text(fmt.Sprintf("Undid edit for %s", territoryName), ToastOption{Colour: color.RGBA{255, 255, 255, 255}}).
							AutoClose(2 * time.Second).
							Show()
					} else {
						// Already at root
						NewToast().
							Text("Nothing to undo", ToastOption{Colour: color.RGBA{200, 200, 100, 255}}).
							AutoClose(2 * time.Second).
							Show()
					}
				} else {
					// Z: Toggle undo explorer
					explorer := GetUndoExplorer()
					if !explorer.IsVisible() {
						explorer.Show()
					} else {
						explorer.Hide()
					}
				}
			}
		case ebiten.KeyY:
			// Check if any text input is focused before handling Y key
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

			// Only handle Y key if no text input is focused
			if !textInputFocused {
				// Ctrl+Y: Redo
				if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) {
					um := GetUndoManager()
					if territoryName, ok := um.Redo(); ok {
						// Show toast notification
						NewToast().
							Text(fmt.Sprintf("Redid edit for %s", territoryName), ToastOption{Colour: color.RGBA{255, 255, 255, 255}}).
							AutoClose(2 * time.Second).
							Show()
					} else {
						// No redo available
						NewToast().
							Text("Nothing to redo", ToastOption{Colour: color.RGBA{200, 200, 100, 255}}).
							AutoClose(2 * time.Second).
							Show()
					}
				}
			}
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

			// Only open guild manager if no text input is focused and event editor is not open
			if !textInputFocused && !gm.mapView.IsEventEditorVisible() {
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

			// Only open loadout manager if no text input is focused and event editor is not open
			if !textInputFocused && !gm.mapView.IsEventEditorVisible() {
				if loadoutManager != nil && !loadoutManager.IsVisible() {
					// Hide transit resource menu when entering loadout application mode
					if gm.mapView != nil && gm.mapView.transitResourceMenu != nil && gm.mapView.transitResourceMenu.IsVisible() {
						gm.mapView.transitResourceMenu.Hide()
					}
					loadoutManager.Show()
				}
			}
		case ebiten.KeyS:
			// Check if any text input is currently focused before opening script manager
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

			// Only open script manager if no text input is focused and event editor is not open
			if !textInputFocused && !gm.mapView.IsEventEditorVisible() {
				if gm.scriptManager != nil && !gm.scriptManager.IsVisible() {
					gm.scriptManager.Show()
				}
			}
		case ebiten.KeyH:
			// Toggle route highlighting feature - only if no text input is focused
			textInputFocused := false

			// Check various text input sources for focus
			if gm.mapView != nil && gm.mapView.territoriesManager != nil {
				guildManager := gm.mapView.territoriesManager.guildManager
				if guildManager != nil && guildManager.IsVisible() && guildManager.HasTextInputFocused() {
					textInputFocused = true
				}
			}

			loadoutManager := GetLoadoutManager()
			if loadoutManager != nil && loadoutManager.IsVisible() && loadoutManager.HasTextInputFocused() {
				textInputFocused = true
			}

			if gm.mapView != nil && gm.mapView.tributeMenu != nil && gm.mapView.tributeMenu.IsVisible() && gm.mapView.tributeMenu.HasTextInputFocused() {
				textInputFocused = true
			}

			// Only toggle if no text input is focused
			if !textInputFocused {
				enabled := gm.ToggleRouteHighlighting()
				NewToast().Text(fmt.Sprintf("Route Highlighting %s", func() string {
					if enabled {
						return "Enabled"
					}
					return "Disabled"
				}()), ToastOption{Colour: color.RGBA{100, 150, 255, 255}}).
					AutoClose(time.Second * 2).
					Show()
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

	// Draw script manager if it exists and is visible
	if gm.scriptManager != nil && gm.scriptManager.IsVisible() {
		gm.scriptManager.Draw(screen)
	}

	// Draw loadout manager if it exists and is visible
	loadoutManager := GetLoadoutManager()
	if loadoutManager != nil && loadoutManager.IsVisible() {
		loadoutManager.Draw(screen)
	}

	// Draw welcome screen if it exists and is visible
	welcomeScreen := GetWelcomeScreen()
	if welcomeScreen != nil && welcomeScreen.IsVisible() {
		welcomeScreen.Draw(screen)
	}

	// Draw undo explorer if it exists and is visible (draw last so it's on top)
	undoExplorer := GetUndoExplorer()
	if undoExplorer != nil && undoExplorer.IsVisible() {
		undoExplorer.Draw(screen)
	}
}

// ToggleRouteHighlighting toggles the white route highlighting feature for hovered territories
func (gm *GameplayModule) ToggleRouteHighlighting() bool {
	if gm.mapView != nil && gm.mapView.territoriesManager != nil {
		currentState := gm.mapView.territoriesManager.GetShowHoveredRoutes()
		newState := !currentState
		gm.mapView.territoriesManager.SetShowHoveredRoutes(newState)
		if newState {
			fmt.Println("[FEATURE] Route highlighting enabled - hover over territories to see routes to HQ in white")
		} else {
			fmt.Println("[FEATURE] Route highlighting disabled")
		}
		return newState
	}
	return false
}

// SetRouteHighlighting sets the route highlighting feature state
func (gm *GameplayModule) SetRouteHighlighting(enabled bool) {
	if gm.mapView != nil && gm.mapView.territoriesManager != nil {
		gm.mapView.territoriesManager.SetShowHoveredRoutes(enabled)
		if enabled {
			fmt.Println("[FEATURE] Route highlighting enabled")
		} else {
			fmt.Println("[FEATURE] Route highlighting disabled")
		}
	}
}

// GetRouteHighlighting returns whether route highlighting is currently enabled
func (gm *GameplayModule) GetRouteHighlighting() bool {
	if gm.mapView != nil && gm.mapView.territoriesManager != nil {
		return gm.mapView.territoriesManager.GetShowHoveredRoutes()
	}
	return false
}
