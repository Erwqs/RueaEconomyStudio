package app

import (
	"fmt"
	"image/color"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"etools/eruntime" // Add eruntime import
	"etools/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// parseResourceValue parses a resource value string to integer
func parseResourceValue(valueStr string) int {
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return 0
}

// Marker represents a location marker on the map
type Marker struct {
	X         float64
	Y         float64
	Label     string
	Color     color.RGBA
	Size      float64
	IsVisible bool
}

// MapView represents the map rendering and interaction component
type MapView struct {
	scale           float64
	offsetX         float64
	offsetY         float64
	dragging        bool
	lastMouseX      int
	lastMouseY      int
	minScale        float64
	maxScale        float64
	screenW         int
	screenH         int
	showCoordinates bool      // Toggle coordinate display
	selectedRegionX int       // X coordinate of selected region
	selectedRegionY int       // Y coordinate of selected region
	markers         []Marker  // List of markers on the map
	showTerritories bool      // Toggle territory display
	lastClickTime   time.Time // Time of last click for double-click detection
	lastUpdateTime  time.Time // Time of last update for delta time calculation

	// Map loading from Wynntils GitHub
	mapManager *MapManager // Manager for loading map data
	isLoading  bool        // Whether the map is currently loading

	// Territories manager for handling territory data
	territoriesManager *TerritoriesManager

	targetScale       float64 // Target scale for animation
	targetOffsetX     float64 // Target offset X for animation
	targetOffsetY     float64 // Target offset Y for animation
	startScale        float64 // Starting scale for animation
	startOffsetX      float64 // Starting offset X for animation
	startOffsetY      float64 // Starting offset Y for animation
	isAnimating       bool    // Whether an animation is in progress
	animProgress      float64 // Animation progress (0.0 to 1.0)
	animSpeed         float64 // Animation speed multiplier
	originalAnimSpeed float64 // Original animation speed to restore after animation

	// Shift+Space tick advancement fields
	shiftSpaceHoldTimer float64 // Timer for held Shift+Space key
	shiftSpaceInterval  float64 // Interval between tick advances when holding

	// Context menu for right-click interactions
	contextMenu *SelectionAnywhere

	// EdgeMenu for territory details
	edgeMenu *EdgeMenu

	// EdgeMenu for transit resource inspector
	transitResourceMenu   *EdgeMenu
	transitResourceFilter string // Current filter string for transit resources

	// State management menu for tick controls
	stateManagementMenu        *StateManagementMenu
	stateManagementMenuVisible bool // Whether the state management menu is visible

	// Hover functionality for territory display
	hoveredTerritory string // Name of currently hovered territory

	// Territory view switcher for different coloring modes
	territoryViewSwitcher *TerritoryViewSwitcher

	// Tribute menu for managing tribute system
	tributeMenu *TributeMenu

	// Claim editing functionality
	isEditingClaims  bool            // Whether we're in claim editing mode
	editingGuildName string          // Name of the guild being edited
	editingGuildTag  string          // Tag of the guild being edited
	guildClaims      map[string]bool // Territory name -> claimed status
	editingUIVisible bool            // Whether editing UI is visible

	// Input handling flags
	justHandledEscKey        bool // Whether we just handled an ESC key in claim editing mode
	tributeMenuHandledEscKey bool // Whether tribute menu just handled an ESC key

	// Transit resources auto-refresh tracking
	lastTransitRefreshTick uint64    // Last tick when transit resources were refreshed
	lastTransitRefreshTime time.Time // Last real time when transit resources were refreshed

	// Transit resource inspector tracking
	lastTransitMenuRefreshTick uint64 // Last tick when transit menu was refreshed

	// Transit resources caching to avoid expensive recalculations
	cachedTransitResources map[string][]typedef.InTransitResources // Territory name -> cached transit resources
	cachedTransitTick      uint64                                  // Last tick when transit resources were cached
	transitCacheValid      bool                                    // Whether the cached transit data is still valid

	// Area selection mode for territory claims
	isAreaSelecting         bool            // Whether area selection mode is active
	areaSelectStartX        int             // Starting X coordinate of area selection
	areaSelectStartY        int             // Starting Y coordinate of area selection
	areaSelectEndX          int             // Current X coordinate of area selection
	areaSelectEndY          int             // Current Y coordinate of area selection
	areaSelectDragging      bool            // Whether user is currently dragging for area selection
	areaSelectTempHighlight map[string]bool // Temporarily highlighted territories during area selection
	areaSelectIsDeselecting bool            // Whether we're in deselection mode (right-click)

	// Event Editor GUI
	eventEditor *EventEditorGUI // Event editor interface
}

// NewMapView creates a new map view component
func NewMapView() *MapView {
	// Create a new MapManager
	mapManager := NewMapManager()

	// Create a new TerritoriesManager
	territoriesManager := NewTerritoriesManager()

	// Create MapView with default values
	mapView := &MapView{
		scale:           0.3,   // Start with a smaller scale to see more of the map
		offsetX:         0,     // Will be adjusted after map loads
		offsetY:         0,     // Will be adjusted after map loads
		minScale:        0.1,   // Allow zooming out more to see full map
		maxScale:        3.0,   // Allow zooming in more for detail
		showCoordinates: false, // Coordinates disabled by default
		showTerritories: true,  // Territories enabled by default
		selectedRegionX: -1,
		selectedRegionY: -1,
		markers:         make([]Marker, 0),
		lastClickTime:   time.Time{}, // Initialize lastClickTime

		// Initialize map manager
		mapManager: mapManager,
		isLoading:  true,

		// Initialize territories manager
		territoriesManager: territoriesManager,

		// Initialize animation fields
		targetScale:   0.3,
		targetOffsetX: 0,
		targetOffsetY: 0,
		startScale:    0.3,
		startOffsetX:  0,
		startOffsetY:  0,
		isAnimating:   false,
		animProgress:  0,
		animSpeed:     4.0, // Animation speed (higher = faster) - made 2x slower

		// Initialize context menu
		contextMenu: NewSelectionAnywhere(),

		// Initialize EdgeMenu (hidden by default)
		edgeMenu: NewEdgeMenu("Territory Details", DefaultEdgeMenuOptions()),

		// Initialize transit resource menu (hidden by default) with bottom position
		transitResourceMenu: NewEdgeMenu("In transit Resource Inspector", func() EdgeMenuOptions {
			opts := DefaultEdgeMenuOptions()
			opts.Position = EdgeMenuBottom
			opts.Width = 0                // Full screen width
			opts.Height = 300             // Initial height, will be updated to 1/3 screen height dynamically
			opts.HorizontalScroll = false // Let the Container handle horizontal scrolling
			return opts
		}()),
		transitResourceFilter: "", // Initialize empty filter string

		// Initialize state management menu (hidden by default)
		stateManagementMenu: NewStateManagementMenu(),

		// Initialize territory view switcher
		territoryViewSwitcher: NewTerritoryViewSwitcher(),

		// Initialize tribute menu
		tributeMenu: NewTributeMenu(),

		// Initialize event editor (will be set after mapView is created)
		eventEditor: nil,

		// Initialize transit resource cache
		cachedTransitResources: make(map[string][]typedef.InTransitResources),
		cachedTransitTick:      0,
		transitCacheValid:      false,
	}

	// Initialize event editor after mapView is created
	mapView.eventEditor = NewEventEditorGUI(mapView)

	// EdgeMenu already handles its own refresh mechanism via refreshMenuData()
	// No need to set a refresh callback that would rebuild the entire menu

	// Start loading map data asynchronously
	mapManager.LoadMapAsync()

	// Start loading territories data asynchronously
	territoriesManager.LoadTerritoriesAsync()

	// Set the global MapView instance
	SetMapView(mapView)

	// Set up EdgeMenu callbacks
	mapView.edgeMenu.SetTerritoryNavCallback(func(territoryName string) {
		// Center map on clicked territory and open its menu
		mapView.CenterTerritory(territoryName)
		mapView.populateTerritoryMenu(territoryName)
	})

	// Register the territory change callback to refresh menu when territory settings change
	eruntime.SetTerritoryChangeCallback(mapView.onTerritoryChanged)

	// Register state change callback to refresh territory colors when state is reset/loaded
	eruntime.SetStateChangeCallback(func() {
		// fmt.Println("[MAP] State change callback triggered")

		// Get all territories from eruntime and update their guild assignments
		territories := eruntime.GetTerritories()
		claimManager := GetGuildClaimManager()

		if claimManager == nil {
			// fmt.Println("[MAP] Warning: GuildClaimManager is nil")
			return
		}

		// fmt.Printf("[MAP] Updating %d territories with their current guild assignments\n", len(territories))

		// Suspend redraws during batch update
		claimManager.suspendRedraws = true

		// Batch update all territory claims for performance
		claims := make([]GuildClaim, 0, len(territories))
		for _, territory := range territories {
			if territory != nil {
				territory.Mu.RLock()
				guildName := territory.Guild.Name
				guildTag := territory.Guild.Tag
				isHQ := territory.HQ
				territory.Mu.RUnlock()

				// If territory has no guild, use the hidden "None" guild
				if guildName == "" || guildName == "No Guild" {
					guildName = "None"
					guildTag = "NONE"
				}

				fmt.Printf("[MAP] Setting territory %s to guild %s [%s], HQ: %v\n", territory.Name, guildName, guildTag, isHQ)
				claims = append(claims, GuildClaim{
					TerritoryName: territory.Name,
					GuildName:     guildName,
					GuildTag:      guildTag,
				})

				// Also update HQ status in territories manager if available
				if mapView.territoriesManager != nil {
					mapView.territoriesManager.UpdateTerritoryHQStatus(territory.Name, isHQ)
				}
			}
		}
		claimManager.AddClaimsBatch(claims)

		// Re-enable redraws and trigger update
		claimManager.suspendRedraws = false
		claimManager.TriggerRedraw()

		// fmt.Println("[MAP] Territory guild assignments updated after state change")
	})

	return mapView
}

// Update handles map interaction (drag, zoom)
func (m *MapView) Update(screenW, screenH int) {
	// Store screen dimensions for drawing calculations
	m.screenW = screenW
	m.screenH = screenH

	// Calculate delta time for frame rate independent updates
	now := time.Now()
	var deltaTime float64 = 1.0 / 60.0 // Default fallback
	if !m.lastUpdateTime.IsZero() {
		deltaTime = now.Sub(m.lastUpdateTime).Seconds()
		// Clamp delta time to prevent huge jumps when paused/resumed
		if deltaTime > 0.1 { // Max 100ms per frame
			deltaTime = 0.1
		}
	}
	m.lastUpdateTime = now

	// Reset the ESC key flag at the start of each frame if it's been set
	if m.justHandledEscKey {
		fmt.Printf("[MAP] Resetting ESC key flag\n")
		m.justHandledEscKey = false
	}

	// Reset tribute menu ESC key flag at the start of each frame
	if m.tributeMenuHandledEscKey {
		m.tributeMenuHandledEscKey = false
	}

	// Force clear these flags to prevent accumulation
	m.justHandledEscKey = false
	m.tributeMenuHandledEscKey = false

	// Check for mouse back button - handle it before anything else if in claim edit mode
	if m.isEditingClaims && inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		m.CancelClaimEditing()
		return
	}

	// Update loading status and center map when first loaded
	if m.isLoading && m.mapManager.IsLoaded() {
		m.isLoading = false
		m.centerMapView() // Center the map when it finishes loading
	}

	// Update animations
	m.updateAnimations(deltaTime)

	// Check if we need to refresh transit resources (every 60 ticks)
	currentTick := eruntime.Elapsed()
	currentTime := time.Now()

	// Rate limiting: Only update if:
	// 1. It's a tick that's divisible by 60 (game logic requirement)
	// 2. We haven't updated this exact tick before
	// 3. At least 500ms have passed since last update (prevents excessive CPU/GPU usage)
	timeSinceLastRefresh := currentTime.Sub(m.lastTransitRefreshTime)
	if currentTick > 0 && currentTick%60 == 0 &&
		currentTick != m.lastTransitRefreshTick &&
		timeSinceLastRefresh >= 500*time.Millisecond {
		// Only refresh if a territory menu is currently open
		// Use the EdgeMenu's internal refresh mechanism instead of rebuilding the entire menu
		if m.edgeMenu.IsVisible() && m.edgeMenu.GetCurrentTerritory() != "" {
			// The EdgeMenu already handles periodic updates via refreshMenuData() in its Update method
			// No need to call populateTerritoryMenu here as it completely rebuilds the menu
		}
		m.lastTransitRefreshTick = currentTick
		m.lastTransitRefreshTime = currentTime
	}

	// Check if transit resource menu needs refreshing (every 60 ticks = 1 minute)
	if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() &&
		(m.lastTransitMenuRefreshTick == 0 || currentTick-m.lastTransitMenuRefreshTick >= 60) {
		m.populateTransitResourceMenu()
		m.lastTransitMenuRefreshTick = currentTick
	}

	// Update menus in top-to-bottom order (transit menu should be on top when visible)
	transitMenuHandledInput := false
	edgeMenuHandledInput := false
	stateMenuHandledInput := false

	// Update transit resource menu first - it should be on top when visible
	if m.transitResourceMenu != nil {
		// Update the dimensions to be full width and 1/3 of screen height before updating
		m.transitResourceMenu.options.Width = screenW
		m.transitResourceMenu.options.Height = screenH / 3
		transitMenuHandledInput = m.transitResourceMenu.Update(screenW, screenH, deltaTime)
	}

	// Update EdgeMenu second - only if transit menu didn't handle input
	if !transitMenuHandledInput && m.edgeMenu != nil {
		edgeMenuHandledInput = m.edgeMenu.Update(screenW, screenH, deltaTime)
	}

	// Update state management menu last - if it handles input, don't process map input
	if !transitMenuHandledInput && !edgeMenuHandledInput && m.stateManagementMenu != nil {
		stateMenuHandledInput = m.stateManagementMenu.menu.Update(screenW, screenH, deltaTime)
		// Update stats periodically
		m.stateManagementMenu.Update(deltaTime)
	}

	// Update tribute menu - it should be above the map but below other menus
	tributeMenuHandledInput := false
	if m.tributeMenu != nil {
		// Clear the tribute menu's ESC flag at the start of the frame
		m.tributeMenu.ClearEscKeyFlag()

		tributeMenuHandledInput = m.tributeMenu.Update()

		// Check if tribute menu handled ESC and set our flag
		if m.tributeMenu.JustHandledEscKey() {
			m.tributeMenuHandledEscKey = true
		}
	}

	// Update territories manager (for blinking animations)
	if m.territoriesManager != nil {
		m.territoriesManager.Update(deltaTime) // Pass real delta time
	}

	// Update territory view switcher BEFORE checking for early returns
	if m.territoryViewSwitcher != nil {
		m.territoryViewSwitcher.Update()
	}

	// Handle event editor input first (E key to open)
	if !tributeMenuHandledInput && m.HandleEventEditorInput() {
		return // Don't process other input this frame
	}

	// Update event editor if visible (handles ESC and mouse back button to close)
	if m.IsEventEditorVisible() {
		eventEditorHandledInput := m.UpdateEventEditor(screenW, screenH)
		if eventEditorHandledInput {
			return // Event editor handled input, don't process map input
		}
	}

	// Handle P key and MouseButton4 BEFORE checking if input was handled
	// Toggle state management menu with P key
	if !tributeMenuHandledInput && inpututil.IsKeyJustPressed(ebiten.KeyP) {
		// Check if any text input is currently focused before opening state management menu
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only toggle state management menu if no text input is focused and event editor is not visible
		if !textInputFocused && !m.IsEventEditorVisible() {
			m.ToggleStateManagementMenu()
			return // Don't process other input this frame
		}
	}

	// Handle MouseButton4 (forward button) to open state management menu
	if !tributeMenuHandledInput && inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) {
		// Check if any text input is currently focused before opening state management menu
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only toggle state management menu if no text input is focused and event editor is not visible
		if !textInputFocused && !m.IsEventEditorVisible() {
			m.ToggleStateManagementMenu()
			return // Don't process other input this frame
		}
	}

	// If EdgeMenu, transit menu, tribute menu, or state menu handled input, don't process map input
	if edgeMenuHandledInput || transitMenuHandledInput || tributeMenuHandledInput || stateMenuHandledInput {
		return
	}

	// Process user input first - always accept wheel input (unless event editor is maximized)
	_, wheelY := WebSafeWheel()
	if wheelY != 0 {
		mx, my := ebiten.CursorPosition()

		// Block map zoom if event editor is visible but not in minimal mode OR if territory picker is active
		if (m.IsEventEditorVisible() && !m.IsEventEditorInMinimalMode()) || m.IsTerritoryPickerActive() {
			return
		}

		// Check if EdgeMenu is consuming wheel input first
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() && m.edgeMenu.IsMouseInside(mx, my) {
			// EdgeMenu will handle the wheel input, don't zoom the map
			return
		}

		// Check if mouse is in sidebar area
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			sidebarWidth := int(m.territoriesManager.GetSideMenuWidth())
			sidebarX := screenW - sidebarWidth

			if mx >= sidebarX {
				// Mouse is in sidebar area - skip map zoom when hovering over sidebar
				return // Don't process map zoom
			}
		}

		// Handle wheel input with smooth zooming for map
		m.handleSmoothZoom(wheelY, mx, my)
	}

	// Handle keyboard zoom with +/- keys (using smooth zoom) - block if event editor is maximized or territory picker is active
	if !tributeMenuHandledInput && (!m.IsEventEditorVisible() || (m.IsEventEditorVisible() && m.IsEventEditorInMinimalMode())) && !m.IsTerritoryPickerActive() {
		if inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadAdd) {
			m.handleSmoothZoom(3.0, screenW/2, screenH/2)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyMinus) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadSubtract) {
			m.handleSmoothZoom(-3.0, screenW/2, screenH/2)
		}
	}

	// Toggle territory display with T key
	if !tributeMenuHandledInput && inpututil.IsKeyJustPressed(ebiten.KeyT) {
		// Check if any text input is currently focused before toggling territories
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only toggle territories if no text input is focused
		if !textInputFocused {
			m.ToggleTerritories()
		}
	}

	// Toggle transit resource inspector with I key
	if !tributeMenuHandledInput && inpututil.IsKeyJustPressed(ebiten.KeyI) {
		// Check if any text input is currently focused before toggling transit resource menu
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only toggle transit resource menu if no text input is focused and event editor is not open
		if !textInputFocused && !m.IsEventEditorVisible() && m.transitResourceMenu != nil {
			if m.transitResourceMenu.IsVisible() {
				m.transitResourceMenu.Hide()
			} else {
				m.populateTransitResourceMenu()
				m.transitResourceMenu.Show()
			}
		}
	}

	// Toggle tribute menu with B key
	if !tributeMenuHandledInput && inpututil.IsKeyJustPressed(ebiten.KeyB) {
		// Check if any text input is currently focused before toggling tribute menu
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only toggle tribute menu if no text input is focused and event editor is not open
		if !textInputFocused && !m.IsEventEditorVisible() && m.tributeMenu != nil {
			if m.tributeMenu.IsVisible() {
				m.tributeMenu.Hide()
			} else {
				m.tributeMenu.Show()
			}
		}
	}

	// Handle spacebar (halt/resume or add ticks) - only when no text input is focused
	if !tributeMenuHandledInput {
		// Check if any text input is currently focused before processing spacebar
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
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
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Check if EdgeMenu is open and has text input focused
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() && m.edgeMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Check if state management menu is open and has text input focused
		if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() && m.stateManagementMenu.menu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Only process spacebar if:
		// 1. No text input is focused
		// 2. Event editor is not visible (or in minimal mode)
		// 3. Territory picker is not active
		if !textInputFocused &&
			(!m.IsEventEditorVisible() || (m.IsEventEditorVisible() && m.IsEventEditorInMinimalMode())) &&
			!m.IsTerritoryPickerActive() {

			// Check for Shift+Space (add ticks when halted)
			shift := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
			spacePressed := ebiten.IsKeyPressed(ebiten.KeySpace)

			if shift && spacePressed && eruntime.IsHalted() {
				// Shift+Space held: Add ticks continuously with timer
				m.shiftSpaceHoldTimer += deltaTime

				if m.shiftSpaceHoldTimer >= m.shiftSpaceInterval {
					// Reset timer
					m.shiftSpaceHoldTimer = 0.0

					// Get tick amount from state management menu
					ticksToAdd := 60 // Default value
					if m.stateManagementMenu != nil {
						ticksToAdd = m.stateManagementMenu.addTickValue
					}

					// Limit the number of ticks to prevent excessive processing
					if ticksToAdd > 12000 {
						ticksToAdd = 12000
					}

					// Run tick advancement in a separate goroutine to prevent UI blocking
					go func(ticks int) {
						for i := 0; i < ticks; i++ {
							eruntime.NextTick()
							// Add a small delay every 60 ticks to prevent overwhelming the system
							if i%60 == 0 {
								time.Sleep(1 * time.Millisecond)
							}
						}
					}(ticksToAdd)
				}
			} else {
				// Reset timer when not holding Shift+Space
				m.shiftSpaceHoldTimer = 0.0

				// Handle regular spacebar press (just pressed, not held)
				if inpututil.IsKeyJustPressed(ebiten.KeySpace) && !shift {
					// Regular Space: Toggle halt/resume state
					if eruntime.IsHalted() {
						eruntime.Resume()
					} else {
						eruntime.Halt()
					}
				}
			}
		}
	}

	// Handle claim editing keyboard shortcuts
	if m.isEditingClaims {
		// Handle area selection mode activation/deactivation
		ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

		if ctrlPressed && !m.isAreaSelecting {
			// Start area selection mode
			m.startAreaSelection()
		} else if !ctrlPressed && m.isAreaSelecting {
			// End area selection mode
			m.endAreaSelection()
		}

		// Update area selection if active
		if m.isAreaSelecting {
			m.updateAreaSelection()
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			m.CancelClaimEditing()
			// Set a flag to indicate we just handled an ESC key in claim editing mode
			// This will be checked by the State.handleKeyEvent method
			m.justHandledEscKey = true
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			m.StopClaimEditing()
			return
		}
	}

	// Handle loadout application mode keyboard shortcuts
	loadoutManager := GetLoadoutManager()
	if loadoutManager != nil && loadoutManager.IsApplyingLoadout() {
		// Handle area selection mode activation/deactivation
		ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

		if ctrlPressed && !m.isAreaSelecting {
			// Start area selection mode
			m.startAreaSelection()
		} else if !ctrlPressed && m.isAreaSelecting {
			// End area selection mode
			m.endAreaSelection()
		}

		// Update area selection if active
		if m.isAreaSelecting {
			m.updateAreaSelection()
		}

		// Note: ESC and Enter are handled by the loadout manager's Update method
	}

	// Handle escape key to close EdgeMenu or side menu
	if !tributeMenuHandledInput && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// First check if state management menu is open
		if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() {
			m.stateManagementMenu.menu.Hide()
			m.stateManagementMenuVisible = false
			return
		}

		// Then check if transit resource menu is open
		// if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
		// 	m.transitResourceMenu.Hide()
		// 	m.justHandledEscKey = true
		// 	return
		// }

		// Then check if EdgeMenu is open
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
			m.edgeMenu.Hide()
			// Deselect territory when EdgeMenu is closed
			if m.territoriesManager != nil {
				m.territoriesManager.DeselectTerritory()
			}
			return
		}

		// Then check side menu
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			// Check if there's an active text input first
			if m.territoriesManager.IsTextInputActive() {
				// Cancel text input editing
				m.territoriesManager.HandleKeyboardInput(ebiten.KeyEscape, true)
				return // Don't close menu, just cancel input
			}

			// Store the territory for recentering after menu closes
			selectedTerritory := m.territoriesManager.GetSelectedTerritory()

			// FIXED: First tell the territory manager we're about to close (but don't close yet)
			// This updates menuTargetOpen, which is used by CenterTerritory
			m.territoriesManager.PrepareToCloseMenu()

			// Now recenter the map first, so it starts moving before the menu closes
			if selectedTerritory != "" {
				m.CenterTerritory(selectedTerritory)
			}

			// Add a small delay then close the menu
			go func(tm *TerritoriesManager) {
				time.Sleep(50 * time.Millisecond)
				tm.CloseSideMenu()
			}(m.territoriesManager)
		}
	}

	// Handle keyboard input for territory text inputs
	if m.territoriesManager != nil && m.territoriesManager.IsTextInputActive() {
		// Handle all numeric keys and control keys for text input
		for _, key := range inpututil.AppendJustPressedKeys(nil) {
			if m.territoriesManager.HandleKeyboardInput(key, true) {
				break // Only handle one key at a time for text input
			}
		}
	}

	// Handle mouse back button (MouseButton3) to close state management menu first, then EdgeMenu, then side menu
	if !tributeMenuHandledInput && inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		// First check if state management menu is open
		if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() {
			m.stateManagementMenu.menu.Hide()
			m.stateManagementMenuVisible = false
			return
		}

		// Then check if transit resource menu is open
		// if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
		// 	m.transitResourceMenu.Hide()
		// 	m.justHandledEscKey = true
		// 	return
		// }

		// Then check if EdgeMenu is open
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
			m.edgeMenu.Hide()
			// Deselect territory when EdgeMenu is closed
			if m.territoriesManager != nil {
				m.territoriesManager.DeselectTerritory()
			}
			return
		}

		// Then check side menu
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			// Store the territory for recentering after menu closes
			selectedTerritory := m.territoriesManager.GetSelectedTerritory()

			// FIXED: First tell the territory manager we're about to close (but don't close yet)
			// This updates menuTargetOpen, which is used by CenterTerritory
			m.territoriesManager.PrepareToCloseMenu()

			// Now recenter the map first, so it starts moving before the menu closes
			if selectedTerritory != "" {
				m.CenterTerritory(selectedTerritory)
			}

			// Add a small delay then close the menu
			go func(tm *TerritoriesManager) {
				time.Sleep(50 * time.Millisecond)
				tm.CloseSideMenu()
			}(m.territoriesManager)

			return // Don't process further (prevents returning to main menu)
		}
	}

	// Update territory manager (including blink animation)
	if m.territoriesManager != nil {
		// Update territories manager including blinking animation
		m.territoriesManager.Update(deltaTime) // Pass real delta time
		m.territoriesManager.UpdateRealTimeData()
	}

	// Handle territory hover detection
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		mx, my := ebiten.CursorPosition()

		// Check if mouse is not in sidebar or EdgeMenu area
		isInSidebar := false
		if m.territoriesManager.IsSideMenuOpen() {
			sidebarWidth := int(m.territoriesManager.GetSideMenuWidth())
			sidebarX := screenW - sidebarWidth
			isInSidebar = mx >= sidebarX
		}

		isInEdgeMenu := false
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
			isInEdgeMenu = m.edgeMenu.IsMouseInside(mx, my)
		}

		// Update hovered territory if not in sidebar or EdgeMenu
		if !isInSidebar && !isInEdgeMenu {
			hoveredTerritory := m.territoriesManager.GetTerritoryAtPosition(mx, my, m.scale, m.offsetX, m.offsetY)
			m.hoveredTerritory = hoveredTerritory
		} else {
			m.hoveredTerritory = ""
		}
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		m.lastMouseX = mx
		m.lastMouseY = my

		// Block most map interactions if event editor is maximized OR if territory picker is active
		if (m.IsEventEditorVisible() && !m.IsEventEditorInMinimalMode()) || m.IsTerritoryPickerActive() {
			// Still allow map dragging but block other functionality
			m.dragging = true
			return
		}

		// Check if EdgeMenu is visible and mouse is inside EdgeMenu area
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
			// Only block input if mouse is actually inside the EdgeMenu bounds
			if m.edgeMenu.IsMouseInside(mx, my) {
				// EdgeMenu will handle its own input, don't process map clicks
				return
			}
		}

		// Check if we're in area selection mode first
		if m.isAreaSelecting {
			// Area selection will handle its own mouse input
			return
		}

		// Check if click is on side menu first
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			if m.territoriesManager.HandleSideMenuClick(mx, my, screenW) {
				// If a trading route or connected territory was clicked, center on that territory
				// (the side menu is already handled in HandleSideMenuClick)
				if route := m.territoriesManager.GetPendingRoute(); route != "" {
					m.CenterTerritory(route)
				}
				return // Click was handled by side menu
			}
		}

		// Only start dragging mode if EdgeMenu is not consuming the input
		// (EdgeMenu check was already done above, so if we reach here, it's safe to drag)
		m.dragging = true

	} else if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()

		// If event editor is maximized or territory picker is active, only allow dragging to end but block other interactions
		if (m.IsEventEditorVisible() && !m.IsEventEditorInMinimalMode()) || m.IsTerritoryPickerActive() {
			// Reset dragging state and return early
			m.dragging = false
			return
		}

		// Check if EdgeMenu is visible and mouse is inside EdgeMenu area
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
			// Only block input if mouse is actually inside the EdgeMenu bounds
			if m.edgeMenu.IsMouseInside(mx, my) {
				// EdgeMenu will handle its own input, don't process map clicks
				return
			}
		}

		// Check if we're in area selection mode first
		if m.isAreaSelecting {
			// Area selection will handle its own mouse input
			return
		}

		// Handle slider mouse release first
		if m.territoriesManager != nil {
			m.territoriesManager.handleMouseRelease()
		}

		// If released without dragging much, it's a click
		dragDistance := math.Sqrt(math.Pow(float64(mx-m.lastMouseX), 2) + math.Pow(float64(my-m.lastMouseY), 2))

		// If drag distance is small, consider it a click, not a drag
		if m.dragging && dragDistance < 5 {
			// Get current time for double-click detection
			currentTime := time.Now()
			timeSinceLastClick := currentTime.Sub(m.lastClickTime)

			// Check if click is within the claim editing banner area first (prevent any processing through banner)
			if m.isEditingClaims {
				bannerHeight := 140 // Same as overlayHeight in drawClaimEditingUI
				if my <= bannerHeight {
					// Check if click is on Save or Cancel buttons first
					buttonWidth := 100
					buttonHeight := 30
					buttonY := 15
					buttonSpacing := 15

					// Save button area
					saveX := m.screenW - (buttonWidth*2 + buttonSpacing + 20)
					if mx >= saveX && mx < saveX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
						// Let the Save button click be processed below
					} else {
						// Cancel button area
						cancelX := m.screenW - (buttonWidth + 20)
						if mx >= cancelX && mx < cancelX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
							// Let the Cancel button click be processed below
						} else {
							// Click is within the banner area but not on buttons, don't process any territory logic
							// Update last click time to prevent double-click issues
							m.lastClickTime = currentTime
							m.dragging = false
							return
						}
					}
				}
			}

			// Check for double-click (time threshold: 500ms)
			isDoubleClick := timeSinceLastClick < 500*time.Millisecond

			// Block double-click functionality if event editor is maximized
			if isDoubleClick && m.IsEventEditorVisible() && !m.IsEventEditorInMinimalMode() {
				// Update last click time but don't process double-click logic
				m.lastClickTime = currentTime
				m.dragging = false
				return
			}

			// Handle claim editing UI button clicks FIRST (before territory selection)
			if m.isEditingClaims {
				buttonWidth := 100
				buttonHeight := 30
				buttonY := 15
				buttonSpacing := 15

				// Save button
				saveX := m.screenW - (buttonWidth*2 + buttonSpacing + 20)
				if mx >= saveX && mx < saveX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
					m.StopClaimEditing()
					m.dragging = false
					return
				}

				// Cancel button
				cancelX := m.screenW - (buttonWidth + 20)
				if mx >= cancelX && mx < cancelX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
					m.CancelClaimEditing()
					m.dragging = false
					return
				}
			}

			// Handle territory selection ONLY if not in claim editing mode OR not clicking on buttons
			shouldHandleTerritorySelection := true
			if m.isEditingClaims {
				buttonWidth := 100
				buttonHeight := 30
				buttonY := 15
				buttonSpacing := 15

				// Check if clicking on Save button
				saveX := m.screenW - (buttonWidth*2 + buttonSpacing + 20)
				if mx >= saveX && mx < saveX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
					shouldHandleTerritorySelection = false
				}

				// Check if clicking on Cancel button
				cancelX := m.screenW - (buttonWidth + 20)
				if mx >= cancelX && mx < cancelX+buttonWidth && my >= buttonY && my < buttonY+buttonHeight {
					shouldHandleTerritorySelection = false
				}
			}

			if shouldHandleTerritorySelection {
				// Handle territory selection
				if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
					// Use the original offset for territory detection
					territoryClicked := m.territoriesManager.HandleMouseClick(mx, my, m.scale, m.offsetX, m.offsetY)

					// Handle claim editing mode - single click to toggle territory claims
					if m.isEditingClaims && territoryClicked != "" {
						m.ToggleTerritoryClaim(territoryClicked)
						// Update last click time but don't process double-click logic
						m.lastClickTime = currentTime
						m.dragging = false
						return
					}

					// Handle loadout application mode - single click to toggle territory selection
					loadoutManager := GetLoadoutManager()
					if loadoutManager != nil && loadoutManager.IsApplyingLoadout() && territoryClicked != "" {
						loadoutManager.ToggleTerritorySelection(territoryClicked)
						// Update last click time but don't process double-click logic
						m.lastClickTime = currentTime
						m.dragging = false
						return
					}

					// Handle double-click on empty space to close EdgeMenu or side menu
					if isDoubleClick && territoryClicked == "" {
						// First check if EdgeMenu is open
						if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
							m.edgeMenu.Hide()
							// Deselect territory when EdgeMenu is closed
							if m.territoriesManager != nil {
								m.territoriesManager.DeselectTerritory()
							}
						} else if m.territoriesManager.IsSideMenuOpen() {
							// Close side menu
							m.territoriesManager.PrepareToCloseMenu()
							// Close menu after a slight delay
							go func(tm *TerritoriesManager) {
								time.Sleep(50 * time.Millisecond)
								tm.CloseSideMenu()
							}(m.territoriesManager)
						}
					}

					// If territory was clicked and it's a double-click, select and center it
					// Don't do this in claim editing mode - we only want single-click territory toggling
					// Also don't show EdgeMenu if event editor is visible
					if isDoubleClick && territoryClicked != "" && !m.isEditingClaims && !m.IsEventEditorVisible() {
						// Center the territory first
						m.CenterTerritory(territoryClicked)

						// Check if this is the same territory that's already selected and blinking
						currentSelected := ""
						if m.territoriesManager != nil {
							currentSelected = m.territoriesManager.GetSelectedTerritoryName()
						}

						// Set selected territory for blinking effect only if it's a different territory
						// or if no territory is currently selected
						if m.territoriesManager != nil && currentSelected != territoryClicked {
							m.territoriesManager.SetSelectedTerritory(territoryClicked)
						}

						// Show EdgeMenu instead of old side menu
						if m.edgeMenu != nil {
							m.populateTerritoryMenu(territoryClicked)
							m.edgeMenu.Show()
						}
					}
				}

				// Update last click time for future double-click detection
				m.lastClickTime = currentTime
			}
		}

		// Reset dragging state (always reset, regardless of territory selection logic)
		m.dragging = false

	} else if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Check if we're in area selection mode first
		if m.isAreaSelecting {
			// Area selection will handle its own mouse input
			return
		}

		mx, my := ebiten.CursorPosition()

		// Always check if we're dragging a slider first, regardless of map dragging
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			if m.territoriesManager.handleSliderDrag(mx, my) {
				// Slider is being dragged, don't move the map
				return
			}
		}

		// Handle dragging when mouse is moved while button is held
		if m.dragging {
			// Check if EdgeMenu is consuming input
			isInEdgeMenu := false
			if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
				isInEdgeMenu = m.edgeMenu.IsMouseInside(mx, my)
			}

			// Only drag the map if not inside EdgeMenu
			if !isInEdgeMenu {
				// Calculate drag distance
				deltaX := float64(mx - m.lastMouseX)
				deltaY := float64(my - m.lastMouseY)

				// Move the map
				m.offsetX += deltaX
				m.offsetY += deltaY

				m.lastMouseX = mx
				m.lastMouseY = my
			}
		}
	}

	// Handle right-click for context menu FIRST (before updating existing menu)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		fmt.Printf("Right-click detected at (%d, %d)\n", mx, my)

		// Don't show context menu if we're in claim editing mode (right-click is used for area deselection)
		if m.isEditingClaims {
			// fmt.Println("Right-click ignored - in claim editing mode")
			return
		}

		// Don't show context menu if loadout is being applied
		if loadoutManager := GetLoadoutManager(); loadoutManager != nil && loadoutManager.IsApplyingLoadout() {
			// fmt.Println("Right-click ignored - in loadout application mode")
			return
		}

		// Check if EdgeMenu is consuming right-click input first
		if m.edgeMenu != nil && m.edgeMenu.IsVisible() && m.edgeMenu.IsMouseInside(mx, my) {
			// EdgeMenu area - don't show context menu
			// fmt.Println("Right-click in EdgeMenu area, ignoring")
			return
		}

		// Check if right-click is not in sidebar area
		if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
			sidebarWidth := int(m.territoriesManager.GetSideMenuWidth())
			sidebarX := screenW - sidebarWidth

			if mx >= sidebarX {
				// Right-click is in sidebar area - don't show context menu
				// fmt.Println("Right-click in sidebar area, ignoring")
				return
			}
		}

		// Setup and show context menu
		// fmt.Println("Setting up context menu...")
		m.setupContextMenu(mx, my)
		m.contextMenu.Show(mx, my, screenW, screenH)
		// fmt.Printf("Context menu shown at (%d, %d), visible: %v\n", mx, my, m.contextMenu.IsVisible())
		return // Don't update the context menu this frame to avoid hiding it immediately
	}

	// Update context menu - this might consume input including right-clicks to hide
	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		if m.contextMenu.Update() {
			// Context menu consumed the input, don't process further
			return
		}
	}
}

// Draw renders the map with current offset and scale
func (m *MapView) Draw(screen *ebiten.Image) {
	// Check if map manager is loaded
	if !m.mapManager.IsLoaded() {
		// Show loading message
		if m.isLoading {
			// Use a larger app font and center the text
			loadingFontSize := 40.0
			loadingFont := loadWynncraftFont(loadingFontSize)
			if loadingFont != nil {
				msg := "Loading map data..."
				bounds := text.BoundString(loadingFont, msg)
				textW := bounds.Dx()
				textH := bounds.Dy()
				spinnerRadius := 40.0
				arcThickness := 8.0
				spacing := 32.0 // space between spinner and text
				// total height of spinner + spacing + text
				totalHeight := spinnerRadius*2 + spacing + float64(textH)
				groupTop := float64(m.screenH)/2 - totalHeight/2
				// center X for both
				centerX := float64(m.screenW) / 2
				// spinner center
				spinnerCenterY := groupTop + spinnerRadius
				// text baseline
				textY := int(groupTop + spinnerRadius*2 + spacing + float64(textH))

				// youtube style arc
				arcLength := math.Pi * 1.2 // radians (about 216 degrees)
				// animate based on time
				angle := float64(time.Now().UnixNano()%2000000000) / 2000000000 * 2 * math.Pi
				segments := 60
				for i := 0; i < segments; i++ {
					segAngle := angle + arcLength*float64(i)/float64(segments)
					alpha := uint8(100 + 155*float64(i)/float64(segments)) // fade out
					col := color.RGBA{200, 200, 255, alpha}
					if i > segments-8 {
						// tail fade
						col.A = uint8(60 + 20*float64(i-segments+8))
					}
					x1 := centerX + spinnerRadius*math.Cos(segAngle)
					y1 := spinnerCenterY + spinnerRadius*math.Sin(segAngle)
					x2 := centerX + (spinnerRadius-arcThickness)*math.Cos(segAngle)
					y2 := spinnerCenterY + (spinnerRadius-arcThickness)*math.Sin(segAngle)
					vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), float32(arcThickness), col, false)
				}

				// draw loading text centered
				text.Draw(screen, msg, loadingFont, int(centerX)-textW/2, textY, color.White)
			} else {
				ebitenutil.DebugPrint(screen, "Loading map data...")
			}
		} else if err := m.mapManager.GetLoadError(); err != nil {
			ebitenutil.DebugPrint(screen, fmt.Sprintf("Error loading map: %v", err))
		}
		return
	}

	// Get the map image
	mapImage := m.mapManager.GetMapImage()
	if mapImage == nil {
		ebitenutil.DebugPrint(screen, "Map image not available")
		return
	}

	// Create draw options
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(m.scale, m.scale)

	// Just use the current offset without any adjustment - centering is handled in CenterTerritory
	op.GeoM.Translate(m.offsetX, m.offsetY)

	// Draw the map image
	screen.DrawImage(mapImage, op)

	// Draw overlay elements after drawing the map
	m.drawOverlayElements(screen)
}

// drawOverlayElements draws all elements that should appear on top of the map
func (m *MapView) drawOverlayElements(screen *ebiten.Image) {
	// This method is designed to be extended for drawing territories, markers, etc.
	// Everything drawn here will use the same coordinate system as the map

	// Draw territories if enabled
	if m.showTerritories && m.territoriesManager.IsLoaded() {
		// Use new GPU overlay system (old logic removed)
		m.territoriesManager.DrawTerritories(screen, m.scale, m.offsetX, m.offsetY, m.hoveredTerritory)
	}

	// Draw markers
	m.drawMarkers(screen, m.offsetX)

	// Draw selected region highlight if any
	m.drawSelectedRegion(screen, m.offsetX)

	// Draw territory hover text
	m.drawTerritoryHover(screen)

	// Draw EdgeMenu on top of everything else
	if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
		m.edgeMenu.Draw(screen)
	}

	// Draw transit resource menu
	if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
		m.transitResourceMenu.Draw(screen)
	}

	// Draw state management menu
	if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() {
		m.stateManagementMenu.menu.Draw(screen)
	}

	// Draw tribute menu
	if m.tributeMenu != nil && m.tributeMenu.IsVisible() {
		m.tributeMenu.Draw(screen)
	}

	// Draw context menu on top of everything
	if m.contextMenu != nil && m.contextMenu.IsVisible() {
		m.contextMenu.Draw(screen)
	}

	// Draw claim editing UI if active
	if m.isEditingClaims && m.editingUIVisible {
		m.drawClaimEditingUI(screen)
	}

	// Draw loadout application UI if active
	loadoutManager := GetLoadoutManager()
	if loadoutManager != nil && loadoutManager.IsApplyingLoadout() {
		loadoutManager.Draw(screen)
	}

	// Draw area selection rectangle if in area selection mode
	if m.isAreaSelecting {
		m.drawAreaSelection(screen)
	}

	// Draw event editor GUI if visible (should be on top of most elements)
	m.DrawEventEditor(screen)

	// Draw territory view switcher modal (should be on top of everything)
	if m.territoryViewSwitcher != nil {
		m.territoryViewSwitcher.Draw(screen)
	}
}

// drawMarkers draws all visible markers on the map
func (m *MapView) drawMarkers(screen *ebiten.Image, mapOffsetX float64) {
	if len(m.markers) == 0 || !m.mapManager.IsLoaded() {
		return
	}

	for _, marker := range m.markers {
		if !marker.IsVisible {
			continue
		}

		// Calculate screen position from map coordinates
		screenX := marker.X*m.scale + mapOffsetX
		screenY := marker.Y*m.scale + m.offsetY

		// Scale marker size with map scale
		size := marker.Size * m.scale

		// Draw marker circle
		radius := size / 2
		vector.DrawFilledCircle(screen, float32(screenX), float32(screenY), float32(radius), marker.Color, false)

		// Draw marker outline
		outlineColor := color.RGBA{255, 255, 255, 200}
		vector.DrawFilledCircle(screen, float32(screenX), float32(screenY), float32(radius+1), outlineColor, false)

		// Draw label if we're zoomed in enough
		if m.scale > 0.5 && marker.Label != "" {
			ebitenutil.DebugPrintAt(
				screen,
				marker.Label,
				int(screenX-float64(len(marker.Label)*3)),
				int(screenY-radius-10),
			)
		}
	}
}

// drawSelectedRegion highlights the currently selected region
func (m *MapView) drawSelectedRegion(screen *ebiten.Image, mapOffsetX float64) {
	if m.selectedRegionX < 0 || m.selectedRegionY < 0 || !m.mapManager.IsLoaded() {
		return // No region selected or map not loaded
	}

	// Draw a highlighted rectangle around the selected region
	// Using grid size of 100 pixels
	regionSize := 100.0 * m.scale

	// Calculate screen position from region coordinates
	screenX := float64(m.selectedRegionX)*regionSize*m.scale + mapOffsetX
	screenY := float64(m.selectedRegionY)*regionSize*m.scale + m.offsetY

	highlightColor := color.RGBA{255, 255, 0, 80}
	vector.DrawFilledRect(screen, float32(screenX), float32(screenY), float32(regionSize), float32(regionSize), highlightColor, false)

	// Draw outline
	outlineColor := color.RGBA{255, 255, 0, 200}
	vector.StrokeLine(screen, float32(screenX), float32(screenY), float32(screenX+regionSize), float32(screenY), 2, outlineColor, false)
	vector.StrokeLine(screen, float32(screenX+regionSize), float32(screenY), float32(screenX+regionSize), float32(screenY+regionSize), 2, outlineColor, false)
	vector.StrokeLine(screen, float32(screenX+regionSize), float32(screenY+regionSize), float32(screenX), float32(screenY+regionSize), 2, outlineColor, false)
	vector.StrokeLine(screen, float32(screenX), float32(screenY+regionSize), float32(screenX), float32(screenY), 2, outlineColor, false)

	// Draw region coordinates
	text := fmt.Sprintf("Region: %d,%d", m.selectedRegionX, m.selectedRegionY)
	ebitenutil.DebugPrintAt(screen, text, int(screenX+5), int(screenY+5))
}

// drawTerritoryHover displays comprehensive territory information in Wynncraft style using eruntime data
func (m *MapView) drawTerritoryHover(screen *ebiten.Image) {
	if m.hoveredTerritory == "" {
		return
	}

	// Get mouse position for hover text placement
	mx, my := ebiten.CursorPosition()

	// Load Wynncraft fonts - significantly larger for distance reading
	titleFontSize := 24.0
	fontSize := 18.0
	smallFontSize := 16.0

	titleFont := loadWynncraftFont(titleFontSize)
	hoverFont := loadWynncraftFont(fontSize)
	smallFont := loadWynncraftFont(smallFontSize)

	titleOffset := getFontVerticalOffset(titleFontSize)
	fontOffset := getFontVerticalOffset(fontSize)
	smallOffset := getFontVerticalOffset(smallFontSize)

	// Get territory data from eruntime
	territoryName := m.hoveredTerritory

	// Get territory stats from eruntime (this handles locking internally)
	territoryStats := eruntime.GetTerritoryStats(territoryName)
	if territoryStats == nil {
		return // Exit early if no data available
	}

	// Prepare structured data for rendering
	type ResourceInfo struct {
		Name       string
		Generation float64
		Stored     float64
		Capacity   float64
		Color      color.RGBA
	}

	resources := []ResourceInfo{
		{
			Name:       "Emerald",
			Generation: territoryStats.CurrentGeneration.Emeralds,
			Stored:     territoryStats.StoredResources.Emeralds,
			Capacity:   territoryStats.StorageCapacity.Emeralds,
			Color:      color.RGBA{144, 238, 144, 255}, // Light green
		},
		{
			Name:       "Ore",
			Generation: territoryStats.CurrentGeneration.Ores,
			Stored:     territoryStats.StoredResources.Ores,
			Capacity:   territoryStats.StorageCapacity.Ores,
			Color:      color.RGBA{220, 220, 220, 255}, // White
		},
		{
			Name:       "Crop",
			Generation: territoryStats.CurrentGeneration.Crops,
			Stored:     territoryStats.StoredResources.Crops,
			Capacity:   territoryStats.StorageCapacity.Crops,
			Color:      color.RGBA{255, 255, 0, 255}, // Yellow
		},
		{
			Name:       "Wood",
			Generation: territoryStats.CurrentGeneration.Wood,
			Stored:     territoryStats.StoredResources.Wood,
			Capacity:   territoryStats.StorageCapacity.Wood,
			Color:      color.RGBA{34, 139, 34, 255}, // Forest green (darker than emerald)
		},
		{
			Name:       "Fish",
			Generation: territoryStats.CurrentGeneration.Fish,
			Stored:     territoryStats.StoredResources.Fish,
			Capacity:   territoryStats.StorageCapacity.Fish,
			Color:      color.RGBA{127, 216, 230, 255}, // Light blue
		},
	}

	// Title: Territory Name [GUILD TAG]
	var guildTag string
	if territoryStats.Guild.Tag != "" && territoryStats.Guild.Tag != "NONE" {
		guildTag = territoryStats.Guild.Tag
	} else {
		guildTag = "NONE"
	}
	fullTitle := fmt.Sprintf("%s [%s]", territoryName, guildTag)

	// Calculate dimensions for background
	titleBounds := text.BoundString(titleFont, fullTitle)
	maxWidth := titleBounds.Dx()

	// Calculate maximum content width by checking actual content
	headerBounds := text.BoundString(hoverFont, "Resource Generation")
	if headerBounds.Dx() > maxWidth {
		maxWidth = headerBounds.Dx()
	}

	// Calculate minimum width needed for resource layout
	// Layout: "Resource" (120px) + "Generation" (120px) + "Storage" (120px) = 360px minimum
	minResourceWidth := 360
	if minResourceWidth > maxWidth {
		maxWidth = minResourceWidth
	}

	// Check defence section width
	defenceBounds := text.BoundString(hoverFont, "Territory Defences:")
	if defenceBounds.Dx() > maxWidth {
		maxWidth = defenceBounds.Dx()
	}

	// Check defence lines width (they are indented with sample longest line)
	sampleDefenceBounds := text.BoundString(smallFont, "  Defence: 11 (99.9%)")
	if sampleDefenceBounds.Dx() > maxWidth {
		maxWidth = sampleDefenceBounds.Dx()
	}

	// Calculate height
	titleHeight := titleBounds.Dy()
	lineHeight := text.BoundString(hoverFont, "Ag").Dy()
	smallLineHeight := text.BoundString(smallFont, "Ag").Dy()
	lineSpacing := 4
	sectionSpacing := 8

	// Calculate total height
	totalHeight := titleHeight + sectionSpacing*2              // Title and spacing
	totalHeight += lineHeight + lineSpacing                    // "Resource Generation" header
	totalHeight += len(resources) * (lineHeight + lineSpacing) // Resources (single line each)
	totalHeight += sectionSpacing                              // Space before defences
	totalHeight += lineHeight + lineSpacing                    // "Territory Defences:" header
	totalHeight += 4 * (smallLineHeight + lineSpacing)         // 4 defence lines
	totalHeight += sectionSpacing                              // Space before defence/treasury
	totalHeight += lineHeight + lineSpacing                    // Defence level line
	totalHeight += lineHeight + lineSpacing                    // Treasury line

	// Add padding
	padding := 15
	bgWidth := maxWidth + padding*2
	bgHeight := totalHeight + padding*2

	// Position hover tooltip
	hoverX := mx + 15
	hoverY := my - 35

	// Keep tooltip on screen
	if hoverX+bgWidth > m.screenW {
		hoverX = mx - bgWidth - 15
	}
	if hoverY < 0 {
		hoverY = my + 35
	}
	if hoverY+bgHeight > m.screenH {
		hoverY = m.screenH - bgHeight
	}

	// Draw background with Wynncraft-style decoration
	bgColor := color.RGBA{25, 25, 35, 240}
	vector.DrawFilledRect(screen, float32(hoverX), float32(hoverY), float32(bgWidth), float32(bgHeight), bgColor, false)

	// Draw main border
	borderColor := color.RGBA{120, 120, 160, 255}
	vector.StrokeRect(screen, float32(hoverX), float32(hoverY), float32(bgWidth), float32(bgHeight), 2, borderColor, false)

	// Draw title background
	titleBgColor := color.RGBA{45, 45, 65, 255}
	titleBgHeight := float32(titleHeight + sectionSpacing*2)
	vector.DrawFilledRect(screen, float32(hoverX), float32(hoverY), float32(bgWidth), titleBgHeight, titleBgColor, false)

	// Draw accent line under title
	accentColor := color.RGBA{255, 215, 0, 255} // Gold accent
	vector.DrawFilledRect(screen, float32(hoverX), float32(hoverY)+titleBgHeight-2, float32(bgWidth), 2, accentColor, false)

	// Draw territory name (title)
	textX := hoverX + padding
	textY := hoverY + padding + titleOffset
	titleColor := color.RGBA{255, 255, 255, 255}
	text.Draw(screen, fullTitle, titleFont, textX, textY, titleColor)

	// Draw content
	currentY := textY + titleHeight + sectionSpacing*2

	// Section header: Resource Generation
	headerColor := color.RGBA{255, 215, 0, 255} // Gold
	text.Draw(screen, "Resource Generation", hoverFont, textX, currentY+fontOffset, headerColor)
	currentY += lineHeight + lineSpacing

	// Draw resources (generation and storage on same line)
	for _, resource := range resources {
		// Determine color based on generation amount
		resourceColor := resource.Color
		if resource.Generation == 0 {
			resourceColor = color.RGBA{128, 128, 128, 255} // Grey for 0 resources
		}

		// Create formatted text
		resourceText := resource.Name
		genText := fmt.Sprintf("+%.0f/hr", resource.Generation)
		storageText := fmt.Sprintf("%.0f/%.0f", resource.Stored, resource.Capacity)

		// Draw resource name
		text.Draw(screen, resourceText, hoverFont, textX, currentY+fontOffset, resourceColor)

		// Draw generation amount (at calculated position)
		genX := textX + maxWidth/3 // Position at 1/3 of width
		text.Draw(screen, genText, hoverFont, genX, currentY+fontOffset, resourceColor)

		// Draw storage (at calculated position)
		storageColor := resourceColor
		if resource.Generation == 0 {
			storageColor = color.RGBA{100, 100, 100, 255} // Darker grey for storage when no generation
		}
		storageX := textX + (maxWidth*2)/3 // Position at 2/3 of width
		text.Draw(screen, storageText, smallFont, storageX, currentY+fontOffset, storageColor)

		currentY += lineHeight + lineSpacing
	}

	currentY += sectionSpacing

	// Territory Defences section
	territoryNameNoGuild := strings.Split(territoryName, " [")[0]
	terr := eruntime.GetTerritory(territoryNameNoGuild)
	if terr != nil {
		text.Draw(screen, "Territory Defences:", hoverFont, textX, currentY+fontOffset, headerColor)
		currentY += lineHeight + lineSpacing

		// Defence stats with proper alignment
		defenceColor := color.RGBA{200, 200, 220, 255}
		damageLevel := int(territoryStats.Upgrades.Damage)
		damageLow := terr.TowerStats.Damage.Low
		damageHigh := terr.TowerStats.Damage.High
		attackLevel := int(territoryStats.Upgrades.Attack)
		attack := terr.TowerStats.Attack
		healthLevel := int(territoryStats.Upgrades.Health)
		health := terr.TowerStats.Health
		defenceLevel := int(territoryStats.Upgrades.Defence)
		defence := terr.TowerStats.Defence * 100

		// Use same column positions as resources
		defenceGenX := 120
		defenceStorageX := 200

		// Define defence stats for alignment
		defenceStats := []struct {
			Name  string
			Level string
			Value string
		}{
			{"Damage", fmt.Sprintf("%d", damageLevel), fmt.Sprintf("(%s-%s)", eruntime.FormatValue(damageLow), eruntime.FormatValue(damageHigh))},
			{"Attack", fmt.Sprintf("%d", attackLevel), fmt.Sprintf("(%s/s)", eruntime.FormatValue(attack))},
			{"Health", fmt.Sprintf("%d", healthLevel), fmt.Sprintf("(%s)", eruntime.FormatValue(health))},
			{"Defence", fmt.Sprintf("%d", defenceLevel), fmt.Sprintf("(%s%%)", eruntime.FormatValue(defence))},
		}

		// Render defence stats with consistent alignment
		for _, stat := range defenceStats {
			// Render stat name
			text.Draw(screen, stat.Name, smallFont, textX+10, currentY+smallOffset, defenceColor)

			// Render level (middle column)
			text.Draw(screen, stat.Level, smallFont, textX+defenceGenX-20, currentY+smallOffset, defenceColor)

			// Render value (right column)
			text.Draw(screen, stat.Value, smallFont, textX+defenceStorageX-20, currentY+smallOffset, defenceColor)

			currentY += smallLineHeight + lineSpacing
		}
	}

	currentY += sectionSpacing

	// Defence level
	levelString, colour := getLevelString(terr.Level)
	text.Draw(screen, fmt.Sprintf("Defence: %s", levelString), hoverFont, textX, currentY+fontOffset, colour)
	currentY += lineHeight + lineSpacing

	// Treasury (placeholder)
	greyColor := color.RGBA{200, 200, 200, 255}
	treasuryBonus := territoryStats.GenerationBonus
	text.Draw(screen, fmt.Sprintf("Treasury: (%.2f%%)", treasuryBonus), hoverFont, textX, currentY+fontOffset, greyColor)
}

// handleSmoothZoom handles mouse wheel zoom events with smooth, arbitrary zoom amounts
func (m *MapView) handleSmoothZoom(wheelDelta float64, cursorX, cursorY int) {
	if !m.mapManager.IsLoaded() {
		return
	}

	// Increased sensitivity for smoother, more responsive zooming
	// Use exponential zoom for more natural feeling
	zoomSensitivity := 0.05
	zoomFactor := math.Pow(1.1, wheelDelta*zoomSensitivity*20)

	// Calculate new scale with constraints
	currentScale := m.scale
	if m.isAnimating {
		// If animating, use target scale as base for chaining zooms
		currentScale = m.targetScale
	}

	if currentScale < m.minScale {
		currentScale = m.minScale
	} else if currentScale > m.maxScale {
		currentScale = m.maxScale
	}

	newScale := currentScale * zoomFactor

	hitMinLimit := newScale < m.minScale
	hitMaxLimit := newScale > m.maxScale

	if newScale < m.minScale {
		newScale = m.minScale
	} else if newScale > m.maxScale {
		newScale = m.maxScale
	}

	// Only proceed if scale actually changed OR if we're not already at the limit
	// This prevents the animation system from breaking when hitting zoom limits
	scaleChanged := math.Abs(newScale-currentScale) >= 0.001
	alreadyAtLimit := (hitMinLimit && math.Abs(currentScale-m.minScale) < 0.001) ||
		(hitMaxLimit && math.Abs(currentScale-m.maxScale) < 0.001)

	if !scaleChanged && alreadyAtLimit {
		return
	}

	// Calculate world point under cursor before zoom
	worldX := (float64(cursorX) - m.offsetX) / m.scale
	worldY := (float64(cursorY) - m.offsetY) / m.scale

	// Calculate where that world point should be after zoom
	newScreenX := worldX*newScale + m.offsetX
	newScreenY := worldY*newScale + m.offsetY

	// Calculate target offsets to keep cursor over the same world point
	targetOffsetX := m.offsetX + float64(cursorX) - newScreenX
	targetOffsetY := m.offsetY + float64(cursorY) - newScreenY

	// Start animation to new scale and position
	m.animateToScale(newScale, targetOffsetX, targetOffsetY)
}

// GetScale returns the current map scale
func (m *MapView) GetScale() float64 {
	return m.scale
}

// GetOffset returns the current map offset
func (m *MapView) GetOffset() (float64, float64) {
	return m.offsetX, m.offsetY
}

// ResetView resets the map to default position and scale
func (m *MapView) ResetView() {
	if !m.mapManager.IsLoaded() {
		return
	}

	// Calculate centered position for the reset
	mapInfo := m.mapManager.GetMapInfo()
	if mapInfo == nil {
		return
	}

	// Use a scale that shows a good portion of the map
	resetScale := 0.3
	scaledMapWidth := float64(mapInfo.Width) * resetScale
	scaledMapHeight := float64(mapInfo.Height) * resetScale

	// Calculate centered offsets
	centeredOffsetX := (float64(m.screenW) - scaledMapWidth) / 2
	centeredOffsetY := (float64(m.screenH) - scaledMapHeight) / 2

	// Animate to the centered position and scale
	m.animateToScale(resetScale, centeredOffsetX, centeredOffsetY)

	// fmt.Println("Animating map view to centered position")
}

// ToggleCoordinates toggles the coordinate labels display
func (m *MapView) ToggleCoordinates() {
	m.showCoordinates = !m.showCoordinates
	fmt.Printf("Coordinate display: %v\n", m.showCoordinates)
}

// ToggleTerritories toggles the display of territory boundaries
func (m *MapView) ToggleTerritories() {
	m.showTerritories = !m.showTerritories
	fmt.Printf("Territory display: %v\n", m.showTerritories)
}

// AddMarker adds a new marker to the map
func (m *MapView) AddMarker(x, y float64, label string, markerColor color.RGBA, size float64) {
	marker := Marker{
		X:         x,
		Y:         y,
		Label:     label,
		Color:     markerColor,
		Size:      size,
		IsVisible: true,
	}
	m.markers = append(m.markers, marker)
	fmt.Printf("Added marker '%s' at (%.1f, %.1f)\n", label, x, y)
}

// ClearMarkers removes all markers from the map
func (m *MapView) ClearMarkers() {
	m.markers = make([]Marker, 0)
	// fmt.Println("All markers cleared")
}

// GetMapCoordinates converts screen coordinates to world coordinates
func (m *MapView) GetMapCoordinates(screenX, screenY int) (float64, float64) {
	// Convert screen coordinates to world coordinates
	worldX := (float64(screenX) - m.offsetX) / m.scale
	worldY := (float64(screenY) - m.offsetY) / m.scale

	return worldX, worldY
}

// centerMapView centers the map view in the screen
func (m *MapView) centerMapView() {
	if !m.mapManager.IsLoaded() {
		return
	}

	mapInfo := m.mapManager.GetMapInfo()
	if mapInfo == nil {
		return
	}

	// Calculate offsets to center the map
	scaledMapWidth := float64(mapInfo.Width) * m.scale
	scaledMapHeight := float64(mapInfo.Height) * m.scale

	// Center the map in the screen
	m.offsetX = (float64(m.screenW) - scaledMapWidth) / 2
	m.offsetY = (float64(m.screenH) - scaledMapHeight) / 2

	// Also update target and start values to match current position
	m.targetScale = m.scale
	m.targetOffsetX = m.offsetX
	m.targetOffsetY = m.offsetY
	m.startScale = m.scale
	m.startOffsetX = m.offsetX
	m.startOffsetY = m.offsetY

	fmt.Printf("Map centered: offset (%.1f, %.1f), scale %.2f\n", m.offsetX, m.offsetY, m.scale)
}

// centerOnTerritory centers the map view on the specified territory
func (m *MapView) centerOnTerritory(territoryName string) {
	// Check if we have territory borders data (used for click detection)
	if m.territoriesManager == nil {
		fmt.Printf("TerritoriesManager not available\n")
		return
	}

	// Try to get territory borders (this is the coordinate system that works for clicks)
	borders := m.territoriesManager.TerritoryBorders
	if borders == nil {
		fmt.Printf("Territory borders not available\n")
		return
	}

	border, exists := borders[territoryName]
	if !exists {
		fmt.Printf("Territory borders not found for: %s\n", territoryName)
		return
	}

	// Calculate center from borders [x1, y1, x2, y2]
	x1, y1, x2, y2 := border[0], border[1], border[2], border[3]
	centerX := (x1 + x2) / 2.0
	centerY := (y1 + y2) / 2.0

	fmt.Printf("Territory %s: borders [%.1f,%.1f] to [%.1f,%.1f], center (%.1f, %.1f)\n",
		territoryName, x1, y1, x2, y2, centerX, centerY)

	// Calculate target offsets to center this point on the screen
	// The transformation is: screenPos = worldPos * scale + offset
	// To center: screenCenter = territoryCenter * scale + targetOffset
	// So: targetOffset = screenCenter - territoryCenter * scale
	targetOffsetX := float64(m.screenW)/2.0 - centerX*m.scale
	targetOffsetY := float64(m.screenH)/2.0 - centerY*m.scale

	fmt.Printf("Current scale: %.3f, screen: %dx%d, target offset: (%.1f, %.1f)\n",
		m.scale, m.screenW, m.screenH, targetOffsetX, targetOffsetY)

	// Animate to the new position (keeping current scale)
	m.animateToScale(m.scale, targetOffsetX, targetOffsetY)

	fmt.Printf("Centering map on territory: %s\n", territoryName)
}

// updateAnimations handles smooth animation transitions
func (m *MapView) updateAnimations(deltaTime float64) {
	if !m.isAnimating {
		return
	}

	// Update animation progress
	m.animProgress += deltaTime * m.animSpeed

	if m.animProgress >= 1.0 {
		// Animation complete
		m.scale = m.targetScale
		m.offsetX = m.targetOffsetX
		m.offsetY = m.targetOffsetY
		m.isAnimating = false

		if m.originalAnimSpeed > 0 {
			m.animSpeed = m.originalAnimSpeed
			m.originalAnimSpeed = 0 // Reset for next use
		}

		// CRITICAL FIX: Update start values to match current values
		// This prevents animation state corruption when chaining animations
		m.startScale = m.scale
		m.startOffsetX = m.offsetX
		m.startOffsetY = m.offsetY
		return
	}

	// Apply easing function for smooth animation
	t := easeOutCubic(m.animProgress)

	// Interpolate between start and target values
	m.scale = m.startScale + (m.targetScale-m.startScale)*t
	m.offsetX = m.startOffsetX + (m.targetOffsetX-m.startOffsetX)*t
	m.offsetY = m.startOffsetY + (m.targetOffsetY-m.startOffsetY)*t
}

// easeOutCubic provides smooth easing for animations
func easeOutCubic(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return 1 - math.Pow(1-t, 3)
}

// animateToScale smoothly animates to a target scale and offset
func (m *MapView) animateToScale(targetScale, targetOffsetX, targetOffsetY float64) {
	// Store current values as start of animation
	m.startScale = m.scale
	m.startOffsetX = m.offsetX
	m.startOffsetY = m.offsetY

	// Set target values
	m.targetScale = targetScale
	m.targetOffsetX = targetOffsetX
	m.targetOffsetY = targetOffsetY

	scaleDifference := math.Abs(targetScale - m.scale)
	offsetXDifference := math.Abs(targetOffsetX - m.offsetX)
	offsetYDifference := math.Abs(targetOffsetY - m.offsetY)

	// Only skip animation if there's really no change at all
	if scaleDifference < 0.001 && offsetXDifference < 1.0 && offsetYDifference < 1.0 {
		// No significant change, just set values directly and ensure consistency
		m.scale = targetScale
		m.offsetX = targetOffsetX
		m.offsetY = targetOffsetY
		m.startScale = m.scale
		m.startOffsetX = m.offsetX
		m.startOffsetY = m.offsetY
		m.isAnimating = false
		return
	}

	// Calculate animation speed based on how big the change is
	speedAdjustment := 1.0
	if scaleDifference > 0.5 {
		// For bigger scale changes, make animation faster
		speedAdjustment = 1.5
	}

	// Start animation
	m.isAnimating = true
	m.animProgress = 0

	// Remember original speed to restore later if we modify it
	m.originalAnimSpeed = m.animSpeed
	if speedAdjustment != 1.0 {
		m.originalAnimSpeed = m.animSpeed
		m.animSpeed *= speedAdjustment
	}
}

// CenterTerritory centers the map view on a specific territory
// with compensation for the side menu width if it's open
func (m *MapView) CenterTerritory(territoryName string) {
	if m.territoriesManager == nil || !m.territoriesManager.IsLoaded() {
		return
	}

	// Get territory borders
	border, exists := m.territoriesManager.TerritoryBorders[territoryName]
	if !exists {
		return
	}

	// Calculate territory center
	centerX := (border[0] + border[2]) / 2
	centerY := (border[1] + border[3]) / 2

	// Always use current zoom level when centering
	targetScale := m.scale
	// Calculate current side menu width (animated) for smooth centering
	sideMenuWidth := m.territoriesManager.GetSideMenuWidth()
	// Calculate screen center with menu compensation
	screenCenterX := (float64(m.screenW) - sideMenuWidth) / 2
	screenCenterY := float64(m.screenH) / 2
	// Calculate new offsets to center the territory at current zoom
	newOffsetX := screenCenterX - centerX*targetScale
	newOffsetY := screenCenterY - centerY*targetScale

	// Animation speed adjustment for territory-to-territory transitions
	animSpeed := m.animSpeed
	if m.territoriesManager.IsSideMenuOpen() && m.territoriesManager.GetSelectedTerritory() != "" &&
		m.territoriesManager.GetSelectedTerritory() != territoryName {
		// Use faster animation for transitions between territories
		animSpeed = m.animSpeed * 1.5
	}

	// Smooth animation to the new position and scale
	m.startScale = m.scale
	m.startOffsetX = m.offsetX
	m.startOffsetY = m.offsetY
	m.targetScale = targetScale
	m.targetOffsetX = newOffsetX
	m.targetOffsetY = newOffsetY
	m.isAnimating = true
	m.animProgress = 0

	// Apply speed adjustment if needed
	oldAnimSpeed := m.animSpeed
	if animSpeed != m.animSpeed {
		m.animSpeed = animSpeed

		// Reset animation speed after a short delay
		go func(view *MapView, originalSpeed float64) {
			time.Sleep(500 * time.Millisecond)
			view.animSpeed = originalSpeed
		}(m, oldAnimSpeed)
	}
}

// Cleanup cleans up the MapView resources, should be called when the MapView is no longer needed
func (m *MapView) Cleanup() {
	if m.territoriesManager != nil {
		m.territoriesManager.Cleanup()
	}
}

// onTerritoryChanged handles territory change notifications and refreshes the menu if needed
func (m *MapView) onTerritoryChanged(territoryName string) {
	// Only refresh if this territory is currently being displayed in the menu
	if m.edgeMenu != nil && m.edgeMenu.IsVisible() && m.edgeMenu.GetCurrentTerritory() == territoryName {
		// Update tower stats and trading routes, don't rebuild the entire menu to avoid breaking sliders
		m.edgeMenu.UpdateTowerStats(territoryName)
		m.edgeMenu.UpdateTradingRoutes(territoryName)
		fmt.Printf("[DEBUG] Tower stats and trading routes updated for: %s\n", territoryName)
	}
}

// setupContextMenu configures the context menu items based on current context
func (m *MapView) setupContextMenu(mouseX, mouseY int) {
	// Clear any existing items
	m.contextMenu = NewSelectionAnywhere()

	// Check if we clicked on a territory using the same logic as HandleMouseClick
	clickedTerritory := ""
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		clickedTerritory = m.territoriesManager.HandleMouseClick(mouseX, mouseY, m.scale, m.offsetX, m.offsetY)
	}

	if clickedTerritory != "" {
		loadoutSubmenu := NewSelectionAnywhere()
		for _, l := range GetLoadoutManager().loadouts {
			loadout := l
			opts := loadout.TerritoryOptions
			loadoutSubmenu.Option(loadout.Name, "", true, func() {
				eruntime.Set(clickedTerritory, opts)
			})
		}

		// Context menu for territory
		m.contextMenu.
			Option("Open Territory Menu", "Double Click", true, func() {
				if m.territoriesManager != nil {
					m.territoriesManager.SetSelectedTerritory(clickedTerritory)
				}
				m.populateTerritoryMenu(clickedTerritory)
				if !m.IsEdgeMenuOpen() {
					m.edgeMenu.Show()
				}
			}).
			Option("Center on Territory", "C", true, func() {
				m.CenterTerritory(clickedTerritory)
			}).
			Divider().
			ContextMenu("Apply Loadout", "", true, loadoutSubmenu).
			Option("Set as HQ", "", !eruntime.GetTerritory(clickedTerritory).HQ, func() {
				// Set the clicked territory as HQ
				territory := eruntime.GetTerritory(clickedTerritory)
				if territory == nil {
					return
				}

				// Set HQ field as true
				eruntime.Set(clickedTerritory, typedef.TerritoryOptions{
					Upgrades:    territory.Options.Upgrade.Set,
					Bonuses:     territory.Options.Bonus.Set,
					Tax:         territory.Tax,
					RoutingMode: territory.RoutingMode,
					Border:      territory.Border,
					HQ:          true,
				})
			})
	} else {
		// Context menu for empty map area
		m.contextMenu.
			Option("Open Guild Manager", "G", !m.IsEventEditorVisible(), func() {
				m.territoriesManager.guildManager.Show()
			}).
			Option("Open State Management", "P", !m.IsEventEditorVisible(), func() {
				m.stateManagementMenu.Show()
			}).
			Option("Open Resource Inspector", "I", !m.IsEventEditorVisible(), func() {
				m.transitResourceMenu.Show()
			}).
			Option("Open Tribute Menu", "B", !m.IsEventEditorVisible(), func() {
				m.tributeMenu.Show()
			}).
			Option("Open Loadout Manager", "L", !m.IsEventEditorVisible(), func() {
				GetLoadoutManager().Show()
			})

		m.contextMenu.Divider()

		// Zoom options
		m.contextMenu.
			Option("Zoom In", "+", true, func() {
				m.handleSmoothZoom(3.0, mouseX, mouseY)
			}).
			Option("Zoom Out", "-", true, func() {
				m.handleSmoothZoom(-3.0, mouseX, mouseY)
			}).
			Option("Reset View", "R", true, func() {
				m.ResetView()
			}).
			Option("Toggle Fullscreen", "F11", true, func() {
				ebiten.SetFullscreen(!ebiten.IsFullscreen())
			})
	}
}

// populateTerritoryMenu populates the EdgeMenu with territory information
func (m *MapView) populateTerritoryMenu(territoryName string) {
	if m.edgeMenu == nil {
		return
	}

	// Get territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		m.edgeMenu.Text(fmt.Sprintf("Territory '%s' not found", territoryName), DefaultTextOptions())
		return
	}

	// Get territory stats for generation data
	territoryStats := eruntime.GetTerritoryStats(territoryName)
	if territoryStats == nil {
		return
	}

	// Save current collapsed states before clearing
	collapsedStates := m.edgeMenu.SaveCollapsedStates()

	// Clear existing elements
	m.edgeMenu.ClearElements()

	// Set the current territory being displayed
	m.edgeMenu.SetCurrentTerritory(territoryName)

	// Set title
	m.edgeMenu.SetTitle(fmt.Sprintf("Territory: %s", territoryName))

	// Add territory status information
	statusText := "Status: Normal"
	if territory.HQ {
		statusText = "Status: Headquarters"
	}
	m.edgeMenu.Text(statusText, DefaultTextOptions())

	// Owned by text display
	guildText := "Owned by: " + territory.Guild.Name
	if territory.Guild.Name == "" || territory.Guild.Name == "No Guild" {
		guildText = "Owned by: No Guild"
	}
	m.edgeMenu.Text(guildText, DefaultTextOptions())

	// Production (collapsible) - shows current production including all bonuses
	productionMenu := m.edgeMenu.CollapsibleMenu("Production", DefaultCollapsibleMenuOptions())

	// Store references to production text elements for real-time updates
	var emeraldProdText, oreProdText, woodProdText, fishProdText, cropProdText, noProdText, generationBonusText *MenuText

	hasProduction := false

	// Create text elements for all resources, but set visibility based on production
	emeraldProdOptions := DefaultTextOptions()
	emeraldProdOptions.Color = color.RGBA{0, 255, 0, 255} // Green for emeralds
	emeraldProdText = NewMenuText(fmt.Sprintf("%d Emerald per Hour", int(territoryStats.CurrentGeneration.Emeralds)), emeraldProdOptions)
	emeraldProdText.SetVisible(territoryStats.CurrentGeneration.Emeralds > 0)
	productionMenu.AddElement(emeraldProdText)
	if territoryStats.CurrentGeneration.Emeralds > 0 {
		hasProduction = true
	}

	oreProdOptions := DefaultTextOptions()
	oreProdOptions.Color = color.RGBA{180, 180, 180, 255} // Light grey for ores
	oreProdText = NewMenuText(fmt.Sprintf("%d Ore per Hour", int(territoryStats.CurrentGeneration.Ores)), oreProdOptions)
	oreProdText.SetVisible(territoryStats.CurrentGeneration.Ores > 0)
	productionMenu.AddElement(oreProdText)
	if territoryStats.CurrentGeneration.Ores > 0 {
		hasProduction = true
	}

	woodProdOptions := DefaultTextOptions()
	woodProdOptions.Color = color.RGBA{139, 69, 19, 255} // Brown for wood
	woodProdText = NewMenuText(fmt.Sprintf("%d Wood per Hour", int(territoryStats.CurrentGeneration.Wood)), woodProdOptions)
	woodProdText.SetVisible(territoryStats.CurrentGeneration.Wood > 0)
	productionMenu.AddElement(woodProdText)
	if territoryStats.CurrentGeneration.Wood > 0 {
		hasProduction = true
	}

	fishProdOptions := DefaultTextOptions()
	fishProdOptions.Color = color.RGBA{0, 150, 255, 255} // Blue for fish
	fishProdText = NewMenuText(fmt.Sprintf("%d Fish per Hour", int(territoryStats.CurrentGeneration.Fish)), fishProdOptions)
	fishProdText.SetVisible(territoryStats.CurrentGeneration.Fish > 0)
	productionMenu.AddElement(fishProdText)
	if territoryStats.CurrentGeneration.Fish > 0 {
		hasProduction = true
	}

	cropProdOptions := DefaultTextOptions()
	cropProdOptions.Color = color.RGBA{255, 255, 0, 255} // Yellow for crops
	cropProdText = NewMenuText(fmt.Sprintf("%d Crop per Hour", int(territoryStats.CurrentGeneration.Crops)), cropProdOptions)
	cropProdText.SetVisible(territoryStats.CurrentGeneration.Crops > 0)
	productionMenu.AddElement(cropProdText)
	if territoryStats.CurrentGeneration.Crops > 0 {
		hasProduction = true
	}

	// "No production" text
	noProdText = NewMenuText("No production", DefaultTextOptions())
	noProdText.SetVisible(!hasProduction)
	productionMenu.AddElement(noProdText)

	productionMenu.Spacer(DefaultSpacerOptions())
	generationBonusText = NewMenuText(fmt.Sprintf("Generation Bonuses: %.2f%%", territory.GenerationBonus), DefaultTextOptions())
	productionMenu.AddElement(generationBonusText)

	// Function to update production values instantly
	updateProductionValues := func() {
		// Recalculate territory stats with new treasury override
		updatedStats := eruntime.GetTerritoryStats(territory.Name)

		// Update emerald production
		emeraldProdText.SetText(fmt.Sprintf("%d Emerald per Hour", int(updatedStats.CurrentGeneration.Emeralds)))
		emeraldProdText.SetVisible(updatedStats.CurrentGeneration.Emeralds > 0)

		// Update ore production
		oreProdText.SetText(fmt.Sprintf("%d Ore per Hour", int(updatedStats.CurrentGeneration.Ores)))
		oreProdText.SetVisible(updatedStats.CurrentGeneration.Ores > 0)

		// Update wood production
		woodProdText.SetText(fmt.Sprintf("%d Wood per Hour", int(updatedStats.CurrentGeneration.Wood)))
		woodProdText.SetVisible(updatedStats.CurrentGeneration.Wood > 0)

		// Update fish production
		fishProdText.SetText(fmt.Sprintf("%d Fish per Hour", int(updatedStats.CurrentGeneration.Fish)))
		fishProdText.SetVisible(updatedStats.CurrentGeneration.Fish > 0)

		// Update crop production
		cropProdText.SetText(fmt.Sprintf("%d Crop per Hour", int(updatedStats.CurrentGeneration.Crops)))
		cropProdText.SetVisible(updatedStats.CurrentGeneration.Crops > 0)

		// Check if any production exists
		newHasProduction := updatedStats.CurrentGeneration.Emeralds > 0 ||
			updatedStats.CurrentGeneration.Ores > 0 ||
			updatedStats.CurrentGeneration.Wood > 0 ||
			updatedStats.CurrentGeneration.Fish > 0 ||
			updatedStats.CurrentGeneration.Crops > 0

		// Update "No production" text visibility
		noProdText.SetVisible(!newHasProduction)

		// Update generation bonus text
		// Get updated territory to get the new generation bonus
		updatedTerritory := eruntime.GetTerritory(territory.Name)
		if updatedTerritory != nil {
			generationBonusText.SetText(fmt.Sprintf("Generation Bonuses: %.2f%%", updatedTerritory.GenerationBonus))
		}
	}

	toggleSwitchOptions := DefaultToggleSwitchOptions()
	toggleSwitchOptions.Options = []string{"None", "VLow", "Low", "Medium", "High", "VHigh"}
	toggleSwitchOptions.Width = 1000  // Set a fixed width for the toggle switch
	toggleSwitchOptions.FontSize = 12 // Set a larger font size for better readability

	productionMenu.ToggleSwitch("Treasury Override", int(territory.TreasuryOverride), toggleSwitchOptions, func(index int, value string) {
		switch index {
		case 0: // AT
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideNone)
		case 1: // VL
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideVeryLow)
		case 2: // LO
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideLow)
		case 3: // MD
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideMedium)
		case 4: // HI
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideHigh)
		case 5: // VH
			eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverrideVeryHigh)
		}
		// Update production values instantly after treasury override change
		updateProductionValues()
	})

	// Connected Territories (collapsible)
	// connectedMenu := m.edgeMenu.CollapsibleMenu("Connected Territories", DefaultCollapsibleMenuOptions())
	// if len(territory.ConnectedTerritories) > 0 {
	// 	for _, connected := range territory.ConnectedTerritories {
	// 		connectedTerritoryName := connected // Capture the variable
	// 		connectedMenu.ClickableText(fmt.Sprintf("• %s", connected), DefaultTextOptions(), func() {
	// 			// When clicked, select this territory for blinking, open details, and center on it
	// 			if m.territoriesManager != nil {
	// 				m.territoriesManager.SetSelectedTerritory(connectedTerritoryName)
	// 			}
	// 			m.populateTerritoryMenu(connectedTerritoryName)
	// 			m.centerOnTerritory(connectedTerritoryName)
	// 		})
	// 	}
	// } else {
	// 	connectedMenu.Text("No connected territories", DefaultTextOptions())
	// }

	// Tower Stats (collapsible) - Shows tower stats with current upgrades
	towerStatsMenu := m.edgeMenu.CollapsibleMenu("Tower Stats", DefaultCollapsibleMenuOptions())
	// towerStatsMenu.Text(fmt.Sprintf("Damage: %.0f - %.0f", territory.TowerStats.Damage.Low, territory.TowerStats.Damage.High), DefaultTextOptions())
	// towerStatsMenu.Text(fmt.Sprintf("Wynn's Math Damage: %.0f - %.0f", territory.TowerStats.Damage.Low*2, territory.TowerStats.Damage.High*2), DefaultTextOptions())
	// towerStatsMenu.Text(fmt.Sprintf("Attack: %.1f/s", territory.TowerStats.Attack), DefaultTextOptions())
	// towerStatsMenu.Text(fmt.Sprintf("Health: %.0f", territory.TowerStats.Health), DefaultTextOptions())
	// towerStatsMenu.Text(fmt.Sprintf("Defence: %.1f%%", territory.TowerStats.Defence*100), DefaultTextOptions())
	// towerStatsMenu.Spacer(DefaultSpacerOptions())
	// // wynn's horrible math
	averageDPS := territory.TowerStats.Attack * ((float64(territory.TowerStats.Damage.High)) + (float64(territory.TowerStats.Damage.Low))) / 2
	// towerStatsMenu.Text(fmt.Sprintf("Average DPS: %.0f", averageDPS), DefaultTextOptions())
	averageDPS2 := territory.TowerStats.Attack * ((float64(territory.TowerStats.Damage.High * 2)) + (float64(territory.TowerStats.Damage.Low * 2))) / 2
	// towerStatsMenu.Text(fmt.Sprintf("Wynn's Math Average DPS: %.0f", averageDPS2), DefaultTextOptions())
	ehp := territory.TowerStats.Health / (1.0 - territory.TowerStats.Defence)
	// towerStatsMenu.Text(fmt.Sprintf("Effective HP: %.0f", ehp), DefaultTextOptions())

	// Helper function to calculate configured tower stats (using Set values)
	calculateConfiguredStats := func() typedef.TowerStats {
		// Get the user-configured upgrade levels (Set values)
		damageLevel := territory.Options.Upgrade.Set.Damage
		attackLevel := territory.Options.Upgrade.Set.Attack
		healthLevel := territory.Options.Upgrade.Set.Health
		defenceLevel := territory.Options.Upgrade.Set.Defence

		// Get upgrade multipliers from eruntime state
		costs := eruntime.GetCost()
		if costs == nil {
			return typedef.TowerStats{} // Return empty stats if costs not available
		}

		// Clamp upgrade levels to valid ranges
		if damageLevel < 0 {
			damageLevel = 0
		} else if damageLevel >= len(costs.UpgradeMultiplier.Damage) {
			damageLevel = len(costs.UpgradeMultiplier.Damage) - 1
		}

		if attackLevel < 0 {
			attackLevel = 0
		} else if attackLevel >= len(costs.UpgradeMultiplier.Attack) {
			attackLevel = len(costs.UpgradeMultiplier.Attack) - 1
		}

		if healthLevel < 0 {
			healthLevel = 0
		} else if healthLevel >= len(costs.UpgradeMultiplier.Health) {
			healthLevel = len(costs.UpgradeMultiplier.Health) - 1
		}

		if defenceLevel < 0 {
			defenceLevel = 0
		} else if defenceLevel >= len(costs.UpgradeMultiplier.Defence) {
			defenceLevel = len(costs.UpgradeMultiplier.Defence) - 1
		}

		// Base values
		baseDamageLow := 1000.0
		baseDamageHigh := 1500.0
		baseAttack := 0.5
		baseHealth := 300000.0
		baseDefence := 0.1

		// Apply upgrade multipliers using configured levels
		damageMultiplier := costs.UpgradeMultiplier.Damage[damageLevel]
		attackMultiplier := costs.UpgradeMultiplier.Attack[attackLevel]
		healthMultiplier := costs.UpgradeMultiplier.Health[healthLevel]
		defenceMultiplier := costs.UpgradeMultiplier.Defence[defenceLevel]

		newDamageLow := baseDamageLow * damageMultiplier
		newDamageHigh := baseDamageHigh * damageMultiplier
		newAttack := baseAttack * attackMultiplier
		newHealth := baseHealth * healthMultiplier
		newDefence := baseDefence * defenceMultiplier

		// Calculate link bonus and external connection bonus (same as simulation)
		linkBonus := 1.0
		if len(territory.Links.Direct) > 0 {
			linkBonus = 1.0 + (0.3 * float64(len(territory.Links.Direct)))
		}

		// HQ external bonus calculation - matches eruntime/calculate1.go
		externalBonus := 1.0
		if territory.HQ {
			if len(territory.Links.Externals) == 0 {
				externalBonus = 1.5 // HQ base bonus of 50% (1 + 0.5)
			} else {
				// Base HQ bonus (50%) + 25% per external territory
				externalBonus = 1.5 + (0.25 * float64(len(territory.Links.Externals)))
			}
		}

		return typedef.TowerStats{
			Damage: typedef.DamageRange{
				Low:  newDamageLow * linkBonus * externalBonus,
				High: newDamageHigh * linkBonus * externalBonus,
			},
			Attack:  newAttack,
			Health:  newHealth * linkBonus * externalBonus,
			Defence: newDefence,
		}
	}

	// Get configured stats
	configuredStats := calculateConfiguredStats()

	// Configured display what the tower would be like with user-configured levels
	configuredMenu := towerStatsMenu.CollapsibleMenu("Configured", DefaultCollapsibleMenuOptions())
	configuredMenu.Text(fmt.Sprintf("Damage: %.0f - %.0f", configuredStats.Damage.Low, configuredStats.Damage.High), DefaultTextOptions())
	configuredMenu.Text(fmt.Sprintf("Wynn's Math Damage: %.0f - %.0f", configuredStats.Damage.Low*2, configuredStats.Damage.High*2), DefaultTextOptions())
	configuredMenu.Text(fmt.Sprintf("Attack: %.1f/s", configuredStats.Attack), DefaultTextOptions())
	configuredMenu.Text(fmt.Sprintf("Health: %.0f", configuredStats.Health), DefaultTextOptions())
	configuredMenu.Text(fmt.Sprintf("Defence: %.1f%%", configuredStats.Defence*100), DefaultTextOptions())
	configuredMenu.Spacer(DefaultSpacerOptions())
	// Calculate averages for configured stats
	configuredAvgDPS := configuredStats.Attack * ((configuredStats.Damage.High + configuredStats.Damage.Low) / 2)
	configuredMenu.Text(fmt.Sprintf("Average DPS: %.0f", configuredAvgDPS), DefaultTextOptions())
	configuredAvgDPS2 := configuredStats.Attack * ((configuredStats.Damage.High*2 + configuredStats.Damage.Low*2) / 2)
	configuredMenu.Text(fmt.Sprintf("Wynn's Math Average DPS: %.0f", configuredAvgDPS2), DefaultTextOptions())
	configuredEHP := configuredStats.Health / (1.0 - configuredStats.Defence)
	configuredMenu.Text(fmt.Sprintf("Effective HP: %.0f", configuredEHP), DefaultTextOptions())

	// Add configured territory level (based on Set values)
	var configuredLevelString string
	var configuredLevelColor color.RGBA
	switch territory.SetLevel {
	case typedef.DefenceLevelVeryHigh:
		configuredLevelString, configuredLevelColor = "Very High", color.RGBA{255, 0, 0, 255}
	case typedef.DefenceLevelHigh:
		configuredLevelString, configuredLevelColor = "High", color.RGBA{255, 128, 0, 255}
	case typedef.DefenceLevelMedium:
		configuredLevelString, configuredLevelColor = "Medium", color.RGBA{255, 255, 0, 255}
	case typedef.DefenceLevelLow:
		configuredLevelString, configuredLevelColor = "Low", color.RGBA{128, 255, 0, 255}
	default: // typedef.DefenceLevelVeryLow
		configuredLevelString, configuredLevelColor = "Very Low", color.RGBA{0, 255, 0, 255}
	}
	configuredLevelTextOptions := DefaultTextOptions()
	configuredLevelTextOptions.Color = configuredLevelColor
	configuredMenu.Text(fmt.Sprintf("%s (%d)", configuredLevelString, territory.SetLevelInt), configuredLevelTextOptions)

	// Current, in contrast, display the actual stats based on affordability (from simulation)
	currentMenu := towerStatsMenu.CollapsibleMenu("Current", DefaultCollapsibleMenuOptions())
	currentMenu.Text(fmt.Sprintf("Damage: %.0f - %.0f", territory.TowerStats.Damage.Low, territory.TowerStats.Damage.High), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Wynn's Math Damage: %.0f - %.0f", territory.TowerStats.Damage.Low*2, territory.TowerStats.Damage.High*2), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Attack: %.1f/s", territory.TowerStats.Attack), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Health: %.0f", territory.TowerStats.Health), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Defence: %.1f%%", territory.TowerStats.Defence*100), DefaultTextOptions())
	currentMenu.Spacer(DefaultSpacerOptions())
	// Use the existing calculated averages for current stats
	currentMenu.Text(fmt.Sprintf("Average DPS: %.0f", averageDPS), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Wynn's Math Average DPS: %.0f", averageDPS2), DefaultTextOptions())
	currentMenu.Text(fmt.Sprintf("Effective HP: %.0f", ehp), DefaultTextOptions())

	// Add current territory level (based on At values)
	var currentLevelString string
	var currentLevelColor color.RGBA
	switch territory.Level {
	case typedef.DefenceLevelVeryHigh:
		currentLevelString, currentLevelColor = "Very High", color.RGBA{255, 0, 0, 255}
	case typedef.DefenceLevelHigh:
		currentLevelString, currentLevelColor = "High", color.RGBA{255, 128, 0, 255}
	case typedef.DefenceLevelMedium:
		currentLevelString, currentLevelColor = "Medium", color.RGBA{255, 255, 0, 255}
	case typedef.DefenceLevelLow:
		currentLevelString, currentLevelColor = "Low", color.RGBA{128, 255, 0, 255}
	default: // typedef.DefenceLevelVeryLow
		currentLevelString, currentLevelColor = "Very Low", color.RGBA{0, 255, 0, 255}
	}
	currentLevelTextOptions := DefaultTextOptions()
	currentLevelTextOptions.Color = currentLevelColor
	currentMenu.Text(fmt.Sprintf("%s (%d)", currentLevelString, territory.LevelInt), currentLevelTextOptions)

	// Links (collapsible) - Links to connections and externals
	totalBonus := 0
	linksMenu := m.edgeMenu.CollapsibleMenu("Links", DefaultCollapsibleMenuOptions())
	if len(territory.Links.Externals) == 0 {
		linksMenu.Text("No connections or externals", DefaultTextOptions())
	} else {
		collapsibleDirect := linksMenu.CollapsibleMenu("Direct Connections", DefaultCollapsibleMenuOptions())

		// Convert map to sorted slice to ensure consistent order
		var directConnections []string
		for conn := range territory.Links.Direct {
			directConnections = append(directConnections, conn)
		}
		sort.Strings(directConnections)

		for _, conn := range directConnections {
			totalBonus += 30 // Each direct connection gives +30% bonus
			// Capture the variable to avoid closure issues
			connName := conn
			collapsibleDirect.ClickableText(fmt.Sprintf("• %s (+30%%)", conn), DefaultTextOptions(), func() {
				if m.territoriesManager != nil {
					m.territoriesManager.SetSelectedTerritory(connName)
				}
				m.populateTerritoryMenu(connName)
				m.centerOnTerritory(connName)
			})
		}

		if territory.HQ {
			collapsibleExternals := linksMenu.CollapsibleMenu("Externals", DefaultCollapsibleMenuOptions())

			// Convert map to sorted slice to ensure consistent order
			var externals []string
			for external := range territory.Links.Externals {
				externals = append(externals, external)
			}
			sort.Strings(externals)

			for _, external := range externals {
				totalBonus += 25 // Each external gives +25% bonus
				// Capture the variable to avoid closure issues
				externalName := external
				collapsibleExternals.ClickableText(fmt.Sprintf("• %s (+25%%)", external), DefaultTextOptions(), func() {
					if m.territoriesManager != nil {
						m.territoriesManager.SetSelectedTerritory(externalName)
					}
					m.populateTerritoryMenu(externalName)
					m.centerOnTerritory(externalName)
				})
			}
		}
	}

	linksMenu.Text(fmt.Sprintf("Total Damage and Health Bonus: +%d%%", totalBonus), DefaultTextOptions())

	// Upgrades (collapsible) - Interactive controls with sliders and +/- buttons
	upgradesMenu := m.edgeMenu.CollapsibleMenu("Upgrades", DefaultCollapsibleMenuOptions())
	upgradesMenu.UpgradeControl("Damage", "damage", territoryName, territory.Options.Upgrade.Set.Damage)
	upgradesMenu.UpgradeControl("Attack", "attack", territoryName, territory.Options.Upgrade.Set.Attack)
	upgradesMenu.UpgradeControl("Health", "health", territoryName, territory.Options.Upgrade.Set.Health)
	upgradesMenu.UpgradeControl("Defence", "defence", territoryName, territory.Options.Upgrade.Set.Defence)

	// Bonuses (collapsible) - Interactive controls with sliders and +/- buttons
	bonusesMenu := m.edgeMenu.CollapsibleMenu("Bonuses", DefaultCollapsibleMenuOptions())

	bonuses1 := bonusesMenu.CollapsibleMenu("Tower Bonuses", DefaultCollapsibleMenuOptions())
	bonuses2 := bonusesMenu.CollapsibleMenu("Territorial Bonuses", DefaultCollapsibleMenuOptions())
	bonuses3 := bonusesMenu.CollapsibleMenu("Resource Bonuses", DefaultCollapsibleMenuOptions())

	bonuses1.BonusControl("Stronger Minions", "strongerMinions", territoryName, territory.Options.Bonus.Set.StrongerMinions)
	bonuses1.BonusControl("Tower Multi-Attack", "towerMultiAttack", territoryName, territory.Options.Bonus.Set.TowerMultiAttack)
	bonuses1.BonusControl("Tower Aura", "towerAura", territoryName, territory.Options.Bonus.Set.TowerAura)
	bonuses1.BonusControl("Tower Volley", "towerVolley", territoryName, territory.Options.Bonus.Set.TowerVolley)

	bonuses2.BonusControl("Gathering Experience", "gatheringExperience", territoryName, territory.Options.Bonus.Set.GatheringExperience)
	bonuses2.BonusControl("Mob Experience", "mobExperience", territoryName, territory.Options.Bonus.Set.MobExperience)
	bonuses2.BonusControl("Mob Damage", "mobDamage", territoryName, territory.Options.Bonus.Set.MobDamage)
	bonuses2.BonusControl("PvP Damage", "pvpDamage", territoryName, territory.Options.Bonus.Set.PvPDamage)
	bonuses2.BonusControl("XP Seeking", "xpSeeking", territoryName, territory.Options.Bonus.Set.XPSeeking)                // 8 territories limit
	bonuses2.BonusControl("Tome Seeking", "tomeSeeking", territoryName, territory.Options.Bonus.Set.TomeSeeking)          // 8 territories limit
	bonuses2.BonusControl("Emerald Seeking", "emeraldSeeking", territoryName, territory.Options.Bonus.Set.EmeraldSeeking) // 8 territories limit

	bonuses3.BonusControl("Larger Resource Storage", "largerResourceStorage", territoryName, territory.Options.Bonus.Set.LargerResourceStorage)
	bonuses3.BonusControl("Larger Emerald Storage", "largerEmeraldStorage", territoryName, territory.Options.Bonus.Set.LargerEmeraldStorage)
	bonuses3.BonusControl("Efficient Resource", "efficientResource", territoryName, territory.Options.Bonus.Set.EfficientResource)
	bonuses3.BonusControl("Efficient Emerald", "efficientEmerald", territoryName, territory.Options.Bonus.Set.EfficientEmerald)
	bonuses3.BonusControl("Resource Rate", "resourceRate", territoryName, territory.Options.Bonus.Set.ResourceRate)
	bonuses3.BonusControl("Emerald Rate", "emeraldRate", territoryName, territory.Options.Bonus.Set.EmeraldRate)

	// Total Costs (collapsible)
	costsMenu := m.edgeMenu.CollapsibleMenu("Total Costs", DefaultCollapsibleMenuOptions())

	emeraldCostOptions := DefaultTextOptions()
	emeraldCostOptions.Color = color.RGBA{0, 255, 0, 255} // Green for emeralds

	costsMenu.Text(fmt.Sprintf("%d Emerald per Hour", territory.Costs.Emeralds), emeraldCostOptions)
	oreCostOptions := DefaultTextOptions()
	oreCostOptions.Color = color.RGBA{180, 180, 180, 255} // Light grey for ores
	costsMenu.Text(fmt.Sprintf("%d Ore per Hour", territory.Costs.Ores), oreCostOptions)

	woodCostOptions := DefaultTextOptions()
	woodCostOptions.Color = color.RGBA{139, 69, 19, 255} // Brown for wood
	costsMenu.Text(fmt.Sprintf("%d Wood per Hour", territory.Costs.Wood), woodCostOptions)

	fishCostOptions := DefaultTextOptions()
	fishCostOptions.Color = color.RGBA{0, 150, 255, 255} // Blue for fish
	costsMenu.Text(fmt.Sprintf("%d Fish per Hour", territory.Costs.Fish), fishCostOptions)

	cropCostOptions := DefaultTextOptions()
	cropCostOptions.Color = color.RGBA{255, 255, 0, 255} // Yellow for crops
	costsMenu.Text(fmt.Sprintf("%d Crop per Hour", territory.Costs.Crops), cropCostOptions)

	// Resources (collapsible)
	resourcesMenu := m.edgeMenu.CollapsibleMenu("Resources", DefaultCollapsibleMenuOptions())

	// Calculate transit resources using the new decoupled transit system, with caching
	transitEmeralds := 0.0
	transitOres := 0.0
	transitWood := 0.0
	transitFish := 0.0
	transitCrops := 0.0

	// Use tick as cache key (assume eruntime.Tick() returns current tick)
	currentTick := eruntime.Tick()
	var transitResources []typedef.InTransitResources
	if m.transitCacheValid && m.cachedTransitTick == currentTick {
		if cached, ok := m.cachedTransitResources[territory.Name]; ok {
			transitResources = cached
		}
	}
	if transitResources == nil {
		transitResources = eruntime.GetTransitResourcesForTerritory(territory)
		if m.cachedTransitResources == nil {
			m.cachedTransitResources = make(map[string][]typedef.InTransitResources)
		}
		m.cachedTransitResources[territory.Name] = transitResources
		m.cachedTransitTick = currentTick
		m.transitCacheValid = true
	}
	for _, transit := range transitResources {
		transitEmeralds += transit.Emeralds
		transitOres += transit.Ores
		transitWood += transit.Wood
		transitFish += transit.Fish
		transitCrops += transit.Crops
	}

	// Create interactive resource storage controls
	resourcesMenu.ResourceStorageControl("Emerald", "emeralds", territoryName,
		int(territory.Storage.At.Emeralds), int(territory.Storage.Capacity.Emeralds)*10, int(transitEmeralds), int(territoryStats.CurrentGeneration.Emeralds),
		color.RGBA{0, 255, 0, 255}) // Green for emeralds

	resourcesMenu.ResourceStorageControl("Ore", "ores", territoryName,
		int(territory.Storage.At.Ores), int(territory.Storage.Capacity.Ores)*10, int(transitOres), int(territoryStats.CurrentGeneration.Ores),
		color.RGBA{180, 180, 180, 255}) // Light grey for ores

	resourcesMenu.ResourceStorageControl("Wood", "wood", territoryName,
		int(territory.Storage.At.Wood), int(territory.Storage.Capacity.Wood)*10, int(transitWood), int(territoryStats.CurrentGeneration.Wood),
		color.RGBA{139, 69, 19, 255}) // Brown for wood

	resourcesMenu.ResourceStorageControl("Fish", "fish", territoryName,
		int(territory.Storage.At.Fish), int(territory.Storage.Capacity.Fish)*10, int(transitFish), int(territoryStats.CurrentGeneration.Fish),
		color.RGBA{0, 150, 255, 255}) // Blue for fish

	resourcesMenu.ResourceStorageControl("Crop", "crops", territoryName,
		int(territory.Storage.At.Crops), int(territory.Storage.Capacity.Crops)*10, int(transitCrops), int(territoryStats.CurrentGeneration.Crops),
		color.RGBA{255, 255, 0, 255}) // Yellow for crops

	// Trading Routes (collapsible)
	routesMenu := m.edgeMenu.CollapsibleMenu("Trading Routes", DefaultCollapsibleMenuOptions())
	if len(territory.TradingRoutes) > 0 {
		// Get current guild information for route coloring
		currentGuild := territory.Guild
		allies := eruntime.GetAllies()

		for _, route := range territory.TradingRoutes {
			if len(route) == 0 {
				continue
			}

			// Get destination (last territory in route)
			destination := route[len(route)-1].Name

			// Format route title with route tax percentage
			routeTitle := ""
			if territory.RouteTax >= 0 {
				routeTaxPercentage := int(territory.RouteTax * 100)
				routeTitle = fmt.Sprintf("Route to %s (%d%%)", destination, routeTaxPercentage)
			} else {
				// If route tax is invalid (-1.0), don't show percentage
				routeTitle = fmt.Sprintf("Route to %s", destination)
			}

			// Create nested collapsible menu for this route (collapsed by default)
			routeOptions := DefaultCollapsibleMenuOptions()
			routeOptions.Collapsed = true
			routeSubmenu := routesMenu.CollapsibleMenu(routeTitle, routeOptions)

			for j, routeTerritory := range route {
				routeTerritoryName := routeTerritory.Name

				// Determine arrow color based on ownership
				var arrowColor color.RGBA
				var taxText string

				if routeTerritory.Guild.Name == currentGuild.Name {
					// We own this territory - bright blue
					arrowColor = color.RGBA{0, 150, 255, 255} // Bright blue
				} else {
					// Check if it's an ally
					isAlly := false
					if currentGuildAllies, exists := allies[&currentGuild]; exists {
						for _, ally := range currentGuildAllies {
							if ally.Name == routeTerritory.Guild.Name {
								isAlly = true
								break
							}
						}
					}

					if isAlly {
						// Allied territory - green
						arrowColor = color.RGBA{0, 255, 0, 255} // Green
					} else {
						// Not allied - red
						arrowColor = color.RGBA{255, 0, 0, 255} // Red
					}
				}

				// Add tax percentage only if not owned by us and has tax
				if routeTerritory.Guild.Name != currentGuild.Name {
					taxPercentage := int(routeTerritory.Tax.Tax * 100)
					if taxPercentage > 0 {
						taxText = fmt.Sprintf(" (%d%%)", taxPercentage)
					}
				}

				// Create the text with arrow
				var displayText string
				if j == 0 {
					displayText = fmt.Sprintf("→ %s%s", routeTerritoryName, taxText)
				} else {
					displayText = fmt.Sprintf("→ %s%s", routeTerritoryName, taxText)
				}

				// Create clickable text with the arrow color
				clickableOptions := DefaultTextOptions()
				clickableOptions.Color = arrowColor

				// Capture territory name for closure
				capturedTerritoryName := routeTerritoryName
				routeSubmenu.ClickableText(displayText, clickableOptions, func() {
					// Center map on clicked territory and open its menu
					m.CenterTerritory(capturedTerritoryName)
					m.populateTerritoryMenu(capturedTerritoryName)
				})
			}
		}
	} else {
		routesMenu.Text("No trading routes", DefaultTextOptions())
	}

	// In Transit Resource (collapsible)
	// transitMenu := m.edgeMenu.CollapsibleMenu("In Transit Resource", DefaultCollapsibleMenuOptions())
	// if len(territory.TransitResource) > 0 {
	// 	// Get current guild information for tax calculations
	// 	currentGuild := territory.Guild

	// 	for _, transitRes := range territory.TransitResource {
	// 		// Skip if origin or destination is nil
	// 		if transitRes.Origin == nil || transitRes.Destination == nil {
	// 			continue
	// 		}

	// 		// Create submenu title: "origin -> destination"
	// 		submenuTitle := fmt.Sprintf("%s -> %s", transitRes.Origin.Name, transitRes.Destination.Name)

	// 		// Create collapsible submenu for this transit route (collapsed by default)
	// 		transitSubmenuOptions := DefaultCollapsibleMenuOptions()
	// 		transitSubmenuOptions.Collapsed = true
	// 		transitSubmenu := transitMenu.CollapsibleMenu(submenuTitle, transitSubmenuOptions)

	// 		// Display resources (traversing)
	// 		headerOptions := DefaultTextOptions()
	// 		headerOptions.Color = color.RGBA{255, 215, 0, 255} // Gold header
	// 		transitSubmenu.Text("Resource (traversing)", headerOptions)

	// 		// Emerald - green
	// 		emeraldOptions := DefaultTextOptions()
	// 		emeraldOptions.Color = color.RGBA{0, 255, 0, 255}
	// 		transitSubmenu.Text(fmt.Sprintf("  %d Emerald", int(transitRes.Emeralds)), emeraldOptions)

	// 		// Ore - light grey
	// 		oreOptions := DefaultTextOptions()
	// 		oreOptions.Color = color.RGBA{180, 180, 180, 255}
	// 		transitSubmenu.Text(fmt.Sprintf("  %d Ore", int(transitRes.Ores)), oreOptions)

	// 		// Crop - yellow
	// 		cropOptions := DefaultTextOptions()
	// 		cropOptions.Color = color.RGBA{255, 255, 0, 255}
	// 		transitSubmenu.Text(fmt.Sprintf("  %d Crop", int(transitRes.Crops)), cropOptions)

	// 		// Wood - brown
	// 		woodOptions := DefaultTextOptions()
	// 		woodOptions.Color = color.RGBA{139, 69, 19, 255}
	// 		transitSubmenu.Text(fmt.Sprintf("  %d Wood", int(transitRes.Wood)), woodOptions)

	// 		// Fish - blue
	// 		fishOptions := DefaultTextOptions()
	// 		fishOptions.Color = color.RGBA{0, 150, 255, 255}
	// 		transitSubmenu.Text(fmt.Sprintf("  %d Fish", int(transitRes.Fish)), fishOptions)

	// 		// Calculate tax by this territory - 0 if origin has same guild as current territory
	// 		taxByThisTerritory := 0.0
	// 		if transitRes.Origin.Guild.Name != currentGuild.Name {
	// 			taxByThisTerritory = territory.Tax.Tax * 100 // Convert to percentage
	// 		}
	// 		// Tax color: green if 0%, red if > 0%
	// 		thisTerritoryTaxOptions := DefaultTextOptions()
	// 		if taxByThisTerritory > 0 {
	// 			thisTerritoryTaxOptions.Color = color.RGBA{255, 0, 0, 255} // Red for tax
	// 		} else {
	// 			thisTerritoryTaxOptions.Color = color.RGBA{0, 255, 0, 255} // Green for no tax
	// 		}
	// 		transitSubmenu.Text(fmt.Sprintf("%.2f%% Taxed by this territory", taxByThisTerritory), thisTerritoryTaxOptions)

	// 		// Route tax
	// 		routeTaxPercentage := 0.0
	// 		if transitRes.NextTax >= 0 {
	// 			routeTaxPercentage = transitRes.NextTax * 100 // Convert to percentage
	// 		}
	// 		// Route tax color: green if 0%, orange/red if > 0%
	// 		routeTaxOptions := DefaultTextOptions()
	// 		if routeTaxPercentage > 0 {
	// 			routeTaxOptions.Color = color.RGBA{255, 165, 0, 255} // Orange for route tax
	// 		} else {
	// 			routeTaxOptions.Color = color.RGBA{0, 255, 0, 255} // Green for no tax
	// 		}
	// 		transitSubmenu.Text(fmt.Sprintf("%.2f%% Route tax", routeTaxPercentage), routeTaxOptions)

	// 		// Routes (collapsible menu but not populated)
	// 		routesSubmenuOptions := DefaultCollapsibleMenuOptions()
	// 		routesSubmenuOptions.Collapsed = true
	// 		routesSubmenu := transitSubmenu.CollapsibleMenu("Routes", routesSubmenuOptions)
	// 		routesSubmenu.Text("Not populated yet", DefaultTextOptions())
	// 	}
	// } else {
	// 	greyOptions := DefaultTextOptions()
	// 	greyOptions.Color = color.RGBA{150, 150, 150, 255} // Grey for no resources
	// 	transitMenu.Text("No in transit resources", greyOptions)
	// }

	// Routing and Taxes (collapsible)
	taxesMenu := m.edgeMenu.CollapsibleMenu("Routing and Taxes", DefaultCollapsibleMenuOptions())

	// Routing Mode toggle switch
	routingModeIndex := 0
	if territory.RoutingMode == typedef.RoutingFastest {
		routingModeIndex = 1
	}
	routingToggleOptions := DefaultToggleSwitchOptions()
	routingToggleOptions.Options = []string{"Cheapest", "Fastest"}
	taxesMenu.ToggleSwitch("Routing Mode", routingModeIndex, routingToggleOptions, func(index int, value string) {
		fmt.Printf("Routing Mode changed to: %s\n", value)
		newRoutingMode := typedef.RoutingCheapest
		if index == 1 {
			newRoutingMode = typedef.RoutingFastest
		}

		// Delay the actual update to allow toggle animation to complete
		go func() {
			// Wait for toggle animation to complete (approximately 200ms)
			time.Sleep(200 * time.Millisecond)

			opts := typedef.TerritoryOptions{
				Upgrades:    territory.Options.Upgrade.Set,
				Bonuses:     territory.Options.Bonus.Set,
				Tax:         territory.Tax,
				RoutingMode: newRoutingMode,
				Border:      territory.Border,
				HQ:          territory.HQ,
			}
			eruntime.Set(territoryName, opts)
		}()
	})

	// Border toggle switch (checking if the logic should be reversed)
	borderIndex := 0
	if territory.Border == typedef.BorderClosed {
		borderIndex = 1
	}
	borderToggleOptions := DefaultToggleSwitchOptions()
	borderToggleOptions.Options = []string{"Opened", "Closed"}
	taxesMenu.ToggleSwitch("Border", borderIndex, borderToggleOptions, func(index int, value string) {
		fmt.Printf("Border changed to: %s (index: %d, current border: %d)\n", value, index, territory.Border)
		newBorder := typedef.BorderOpen
		if index == 1 {
			newBorder = typedef.BorderClosed
		}
		fmt.Printf("Setting new border to: %d\n", newBorder)

		// Delay the actual update to allow toggle animation to complete
		go func() {
			// Wait for toggle animation to complete (approximately 200ms)
			time.Sleep(200 * time.Millisecond)

			opts := typedef.TerritoryOptions{
				Upgrades:    territory.Options.Upgrade.Set,
				Bonuses:     territory.Options.Bonus.Set,
				Tax:         territory.Tax,
				RoutingMode: territory.RoutingMode,
				Border:      newBorder,
				HQ:          territory.HQ,
			}
			eruntime.Set(territoryName, opts)
		}()
	})

	// Add divider/spacer
	spacerOptions := DefaultSpacerOptions()
	spacerOptions.Height = 10
	taxesMenu.Spacer(spacerOptions)

	// Tax input (5-70%)
	currentTax := int(territory.Tax.Tax * 100) // Convert from decimal to percentage
	if currentTax < 5 {
		currentTax = 5
	}
	if currentTax > 70 {
		currentTax = 70
	}

	taxInputOptions := DefaultTextInputOptions()
	taxInputOptions.Width = 40 // Smaller width for 2-digit numbers
	taxInputOptions.MaxLength = 2
	taxInputOptions.Placeholder = "5-70"
	taxInputOptions.ValidateInput = func(newValue string) bool {
		// Allow empty string (for clearing)
		if newValue == "" {
			return true
		}
		// Only allow numeric characters
		for _, r := range newValue {
			if r < '0' || r > '9' {
				return false
			}
		}
		// Check if the numeric value is within range
		if val, err := strconv.Atoi(newValue); err == nil {
			return val >= 5 && val <= 70
		}
		return false
	}

	taxesMenu.TextInput("Tax %", fmt.Sprintf("%d", currentTax), taxInputOptions, func(value string) {
		if value == "" {
			return // Don't process empty values
		}
		if taxValue, err := strconv.Atoi(value); err == nil {
			// Clamp the value between 5 and 70
			if taxValue < 5 {
				taxValue = 5
			}
			if taxValue > 70 {
				taxValue = 70
			}

			fmt.Printf("Tax changed to: %d%%\n", taxValue)

			// Create new tax struct with updated Tax field
			newTax := territory.Tax
			newTax.Tax = float64(taxValue) / 100.0 // Convert back to decimal

			opts := typedef.TerritoryOptions{
				Upgrades:    territory.Options.Upgrade.Set,
				Bonuses:     territory.Options.Bonus.Set,
				Tax:         newTax,
				RoutingMode: territory.RoutingMode,
				Border:      territory.Border,
				HQ:          territory.HQ,
			}
			eruntime.Set(territoryName, opts)
		}
	})

	// Ally Tax input (5-70%)
	currentAllyTax := int(territory.Tax.Ally * 100) // Convert from decimal to percentage
	if currentAllyTax < 5 {
		currentAllyTax = 5
	}
	if currentAllyTax > 70 {
		currentAllyTax = 70
	}

	allyTaxInputOptions := DefaultTextInputOptions()
	allyTaxInputOptions.Width = 40 // Smaller width for 2-digit numbers
	allyTaxInputOptions.MaxLength = 2
	allyTaxInputOptions.Placeholder = "5-70"
	allyTaxInputOptions.ValidateInput = func(newValue string) bool {
		// Allow empty string (for clearing)
		if newValue == "" {
			return true
		}
		// Only allow numeric characters
		for _, r := range newValue {
			if r < '0' || r > '9' {
				return false
			}
		}
		// Check if the numeric value is within range
		if val, err := strconv.Atoi(newValue); err == nil {
			return val >= 5 && val <= 70
		}
		return false
	}

	taxesMenu.TextInput("Ally Tax %", fmt.Sprintf("%d", currentAllyTax), allyTaxInputOptions, func(value string) {
		if value == "" {
			return // Don't process empty values
		}
		if allyTaxValue, err := strconv.Atoi(value); err == nil {
			// Clamp the value between 5 and 70
			if allyTaxValue < 5 {
				allyTaxValue = 5
			}
			if allyTaxValue > 70 {
				allyTaxValue = 70
			}

			fmt.Printf("Ally Tax changed to: %d%%\n", allyTaxValue)

			// Create new tax struct with updated Ally field
			newTax := territory.Tax
			newTax.Ally = float64(allyTaxValue) / 100.0 // Convert back to decimal

			opts := typedef.TerritoryOptions{
				Upgrades:    territory.Options.Upgrade.Set,
				Bonuses:     territory.Options.Bonus.Set,
				Tax:         newTax,
				RoutingMode: territory.RoutingMode,
				Border:      territory.Border,
				HQ:          territory.HQ,
			}
			eruntime.Set(territoryName, opts)
		}
	})

	// Loadout button
	loadoutButtonOptions := DefaultButtonOptions()
	loadoutButtonOptions.TextColor = color.RGBA{255, 255, 255, 255}       // White text
	loadoutButtonOptions.BorderColor = color.RGBA{200, 200, 200, 255}     // Lighter grey border
	loadoutButtonOptions.BackgroundColor = color.RGBA{100, 100, 100, 255} // Grey background
	loadoutButtonOptions.HoverColor = color.RGBA{150, 150, 150, 255}      // Lighter grey on hover

	loadoutMenu := m.edgeMenu.CollapsibleMenu("Loadout (Click to apply)", DefaultCollapsibleMenuOptions())
	for _, item := range globalLoadoutManager.loadouts {
		// Capture the item in the loop scope to avoid closure issues
		capturedItem := item
		loadoutMenu.ClickableText(fmt.Sprintf("- %s", item.Name), DefaultTextOptions(), func() {
			territoryOpts := typedef.TerritoryOptions{
				Upgrades:    capturedItem.Upgrades,
				Bonuses:     capturedItem.Bonuses,
				Tax:         capturedItem.Tax,
				RoutingMode: capturedItem.RoutingMode,
				Border:      capturedItem.Border,
				HQ:          territory.HQ,
			}

			eruntime.Set(territoryName, territoryOpts)
		})
	}

	// Set as HQ button
	hqButtonOptions := DefaultButtonOptions()
	hqButtonText := "Set as HQ"
	if territory.HQ {
		hqButtonOptions.Enabled = false
		hqButtonOptions.BackgroundColor = color.RGBA{100, 100, 100, 255}
		hqButtonOptions.TextColor = color.RGBA{150, 150, 150, 255}
		hqButtonText = "Already HQ"
	}

	m.edgeMenu.Button(hqButtonText, hqButtonOptions, func() {
		if !territory.HQ {
			fmt.Printf("Setting %s as HQ\n", territoryName)
			// Call the Set function to set this territory as HQ
			opts := typedef.TerritoryOptions{
				Upgrades:    territory.Options.Upgrade.Set,
				Bonuses:     territory.Options.Bonus.Set,
				Tax:         territory.Tax,
				RoutingMode: territory.RoutingMode,
				Border:      territory.Border,
				HQ:          true,
			}
			eruntime.Set(territoryName, opts)
			// Refresh the menu to show updated state
			m.populateTerritoryMenu(territoryName)
		}
	})

	debugButton := DefaultButtonOptions()
	// Pink button
	debugButton.BackgroundColor = color.RGBA{255, 105, 180, 255} // Pink background
	debugButton.HoverColor = color.RGBA{255, 182, 193, 255}      // Lighter pink on hover
	debugButton.PressedColor = color.RGBA{255, 20, 147, 255}     // Darker pink when pressed
	debugButtonText := "Trigger Breakpoint"
	m.edgeMenu.Button(debugButtonText, debugButton, func() {
		t := eruntime.GetTerritory(territoryName)
		_ = t // For debugging
		runtime.Breakpoint()
	})

	// Restore collapsed states to maintain user's UI preferences
	m.edgeMenu.RestoreCollapsedStates(collapsedStates)
}

// IsEdgeMenuOpen returns whether the edge menu is currently open
func (m *MapView) IsEdgeMenuOpen() bool {
	return m.edgeMenu != nil && m.edgeMenu.IsVisible()
}

// IsTransitResourceMenuOpen returns whether the transit resource menu is currently open
func (m *MapView) IsTransitResourceMenuOpen() bool {
	// Transit Resource Menu is disabled
	return false
	// return m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible()
}

// IsStateManagementMenuOpen returns whether the state management menu is currently open
func (m *MapView) IsStateManagementMenuOpen() bool {
	return m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible()
}

// ToggleStateManagementMenu toggles the state management menu visibility
func (m *MapView) ToggleStateManagementMenu() {
	if m.stateManagementMenu == nil {
		return
	}

	// Don't toggle if event editor is open
	if m.IsEventEditorVisible() {
		return
	}

	// If opening state management menu, close EdgeMenu but keep side menu open
	if !m.stateManagementMenuVisible {
		// Do not close EdgeMenu or deselect territory when opening state management menu
		// This allows users to have both the territory EdgeMenu and state management menu open simultaneously
	}

	m.stateManagementMenuVisible = !m.stateManagementMenuVisible

	if m.stateManagementMenuVisible {
		m.stateManagementMenu.Show()
	} else {
		m.stateManagementMenu.menu.Hide()
	}
}

// GetTerritoriesManager returns the territories manager
func (m *MapView) GetTerritoriesManager() *TerritoriesManager {
	return m.territoriesManager
}

// StartClaimEditing starts the claim editing mode for a specific guild
func (m *MapView) StartClaimEditing(guildName, guildTag string) {
	fmt.Printf("[MAP] Starting claim editing for guild: %s [%s]\n", guildName, guildTag)

	m.isEditingClaims = true
	m.editingGuildName = guildName
	m.editingGuildTag = guildTag
	m.editingUIVisible = true

	// Load existing claims for this guild from storage
	claimManager := GetGuildClaimManager()
	if claimManager != nil {
		m.guildClaims = claimManager.GetClaimsForGuild(guildName, guildTag)
		fmt.Printf("[MAP] Loaded %d existing claims for guild %s [%s]\n", len(m.guildClaims), guildName, guildTag)
	} else {
		// Fallback to empty map if claim manager is not available
		m.guildClaims = make(map[string]bool)
		fmt.Printf("[MAP] Warning: Could not load existing claims, starting with empty claims\n")
	}

	// Close any open territory side menu
	if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
		m.territoriesManager.CloseSideMenu()
	}

	// Hide the EdgeMenu if it's open
	if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
		m.edgeMenu.Hide()
	}

	// Hide the state management menu if it's open
	if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() {
		m.stateManagementMenu.menu.Hide()
		m.stateManagementMenuVisible = false
	}

	// Update the territory renderer with guild info for coloring
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		if renderer := m.territoriesManager.GetRenderer(); renderer != nil {
			renderer.SetEditingGuild(guildName, guildTag, m.guildClaims)
		}
	}
}

// StopClaimEditing stops the claim editing mode and saves changes
func (m *MapView) StopClaimEditing() {
	if !m.isEditingClaims {
		return
	}

	fmt.Printf("[MAP] Stopping claim editing for guild: %s [%s]\n", m.editingGuildName, m.editingGuildTag)

	// Get the claim manager
	claimManager := GetGuildClaimManager()
	claimCount := 0

	// Batch process all claim changes without triggering individual redraws
	if claimManager != nil {
		claims := make([]GuildClaim, 0, len(m.guildClaims))
		// Temporarily disable automatic redraws during batch processing
		claimManager.suspendRedraws = true

		for territory, claimed := range m.guildClaims {
			if claimed {
				claimCount++
				fmt.Printf("  - Claimed territory: %s\n", territory)
				claims = append(claims, GuildClaim{
					TerritoryName: territory,
					GuildName:     m.editingGuildName,
					GuildTag:      m.editingGuildTag,
				})
			} else {
				// Only remove claims if this territory was originally claimed by the current guild
				if existingClaim, exists := claimManager.Claims[territory]; exists {
					if existingClaim.GuildName == m.editingGuildName && existingClaim.GuildTag == m.editingGuildTag {
						fmt.Printf("  - Removing claim for territory: %s (was claimed by current guild)\n", territory)
						claimManager.RemoveClaim(territory)
					} else {
						fmt.Printf("  - Skipping removal for territory: %s (belongs to different guild: %s [%s])\n",
							territory, existingClaim.GuildName, existingClaim.GuildTag)
					}
				}
			}
		}
		if len(claims) > 0 {
			claimManager.AddClaimsBatch(claims)
		}
		// Re-enable redraws
		claimManager.suspendRedraws = false

		// Print all claims for debugging
		claimManager.PrintClaims()
	}

	fmt.Printf("  Total territories claimed: %d\n", claimCount)

	// Reset editing state first
	m.isEditingClaims = false
	m.editingGuildName = ""
	m.editingGuildTag = ""
	m.guildClaims = nil
	m.editingUIVisible = false

	// Clear the editing guild from the renderer
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		if renderer := m.territoriesManager.GetRenderer(); renderer != nil {
			renderer.ClearEditingGuild()
		}
	}

	// Now trigger a single comprehensive redraw after all changes are complete
	if claimManager != nil {
		fmt.Printf("[MAP] Triggering final redraw after claim editing completion\n")
		claimManager.TriggerRedraw()
	}

	// Return to guild management interface after saving
	if m.territoriesManager != nil {
		// Hide transit resource menu when returning to guild management
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
			m.transitResourceMenu.Hide()
		}
		// Use a goroutine to open guild management after a brief delay
		// This ensures the editing state is fully cleared first
		go func() {
			time.Sleep(50 * time.Millisecond)
			m.territoriesManager.OpenGuildManagement()
		}()
	}
}

// CancelClaimEditing cancels the claim editing mode without saving
func (m *MapView) CancelClaimEditing() {
	if !m.isEditingClaims {
		return
	}

	fmt.Printf("[MAP] Cancelling claim editing for guild: %s [%s]\n", m.editingGuildName, m.editingGuildTag)

	// We need to restore the territory colors to their original state
	// This happens automatically because:
	// 1. When cancelling, we clear our local claim editing state
	// 2. The renderer will no longer see our temporary claims
	// 3. It will use the persistent claims from the GuildClaimManager
	// 4. We didn't permanently update the GuildClaimManager while editing

	// Reset editing state without saving
	m.isEditingClaims = false
	m.editingGuildName = ""
	m.editingGuildTag = ""
	m.guildClaims = nil
	m.editingUIVisible = false

	// Set the flag to indicate we handled an ESC key press
	m.justHandledEscKey = true

	// Return to guild management interface instead of main map view
	if m.territoriesManager != nil {
		// Hide transit resource menu when returning to guild management
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
			m.transitResourceMenu.Hide()
		}
		// Use a goroutine to open guild management after a brief delay
		// This ensures the editing state is fully cleared first
		go func() {
			time.Sleep(50 * time.Millisecond)
			m.territoriesManager.OpenGuildManagement()
		}()
	}

	// Store a local reference to the territories manager to avoid it being cleared
	// in case another part of the code modifies m.territoriesManager before the goroutine runs
	territoriesManager := m.territoriesManager

	// Use a goroutine with a slightly increased delay to reopen the guild management menu
	// This ensures that the ESC key event is fully processed first
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Hide transit resource menu when reopening guild management
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
			m.transitResourceMenu.Hide()
		}
		// Reopen the guild management menu after a slight delay
		if territoriesManager != nil {
			fmt.Printf("[MAP] Reopening guild management menu after claim edit cancellation\n")
			territoriesManager.OpenGuildManagement()
		} else {
			fmt.Printf("[MAP] Error: territories manager is nil, can't reopen guild management\n")
		}
	}()

	// Clear the editing guild from the renderer
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		if renderer := m.territoriesManager.GetRenderer(); renderer != nil {
			renderer.ClearEditingGuild()

			// Force a complete redraw of the territory cache
			if territoryCache := renderer.GetTerritoryCache(); territoryCache != nil {
				territoryCache.ForceRedraw()
			}
		}
	}
}

// IsEditingClaims returns whether claim editing mode is active
func (m *MapView) IsEditingClaims() bool {
	return m.isEditingClaims
}

// JustHandledEscKey returns whether we just handled an ESC key in claim editing mode
func (m *MapView) JustHandledEscKey() bool {
	return m.justHandledEscKey
}

// TributeMenuJustHandledEscKey returns whether tribute menu just handled an ESC key
func (m *MapView) TributeMenuJustHandledEscKey() bool {
	return m.tributeMenuHandledEscKey
}

// ToggleTerritoryClaim toggles the claim status of a territory
func (m *MapView) ToggleTerritoryClaim(territoryName string) {
	if !m.isEditingClaims {
		return
	}

	if m.guildClaims == nil {
		m.guildClaims = make(map[string]bool)
	}

	// Always allow toggling - the logic in StopClaimEditing will handle what gets persisted
	claimed := m.guildClaims[territoryName]
	m.guildClaims[territoryName] = !claimed

	// We don't update the persistent claim manager here
	// Instead, we only do that when finalizing with StopClaimEditing
	// This allows us to cancel edits without affecting persistent state

	// Update the territory renderer with the updated claims
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		if renderer := m.territoriesManager.GetRenderer(); renderer != nil {
			renderer.SetEditingGuild(m.editingGuildName, m.editingGuildTag, m.guildClaims)

			// Force a complete redraw of the territory cache
			if territoryCache := renderer.GetTerritoryCache(); territoryCache != nil {
				territoryCache.ForceRedraw()
			}
		}
	}

	if !claimed {
		fmt.Printf("[MAP] Claimed territory: %s for guild %s\n", territoryName, m.editingGuildName)
	} else {
		fmt.Printf("[MAP] Unclaimed territory: %s for guild %s\n", territoryName, m.editingGuildName)
	}
}

// drawClaimEditingUI draws the claim editing interface
func (m *MapView) drawClaimEditingUI(screen *ebiten.Image) {
	// Draw semi-transparent overlay at the top of screen
	overlayHeight := 140 // Increased height to accommodate additional text
	vector.DrawFilledRect(screen, 0, 0, float32(m.screenW), float32(overlayHeight), color.RGBA{0, 0, 0, 200}, false)

	// Load font
	font := loadWynncraftFont(20)
	if font == nil {
		return
	}
	fontOffset := getFontVerticalOffset(20)

	// Draw title - centered
	title := fmt.Sprintf("Editing Claims for Guild: %s [%s]", m.editingGuildName, m.editingGuildTag)
	titleBounds := text.BoundString(font, title)
	titleX := (m.screenW - titleBounds.Dx()) / 2
	titleY := 20 + fontOffset
	text.Draw(screen, title, font, titleX, titleY, color.RGBA{255, 215, 0, 255}) // Gold

	// Draw first instruction line - centered
	instruction1 := "Click territories to claim/unclaim them for this guild"
	instr1Bounds := text.BoundString(font, instruction1)
	instr1X := (m.screenW - instr1Bounds.Dx()) / 2
	instr1Y := titleY + 25
	text.Draw(screen, instruction1, font, instr1X, instr1Y, color.RGBA{200, 200, 200, 255})

	// Draw second instruction line - centered
	instruction2 := "Hold control for area selection"
	instr2Bounds := text.BoundString(font, instruction2)
	instr2X := (m.screenW - instr2Bounds.Dx()) / 2
	instr2Y := instr1Y + 25
	text.Draw(screen, instruction2, font, instr2X, instr2Y, color.RGBA{200, 200, 200, 255})

	// Draw claim count - centered
	claimCount := 0
	if m.guildClaims != nil {
		for _, claimed := range m.guildClaims {
			if claimed {
				claimCount++
			}
		}
	}
	countText := fmt.Sprintf("Territory Claimed: %d", claimCount)
	countBounds := text.BoundString(font, countText)
	countX := (m.screenW - countBounds.Dx()) / 2
	countY := instr2Y + 25
	text.Draw(screen, countText, font, countX, countY, color.RGBA{100, 255, 100, 255})

	// Draw control buttons
	buttonWidth := 100
	buttonHeight := 30
	buttonY := 15
	buttonSpacing := 15

	// Save button
	saveX := m.screenW - (buttonWidth*2 + buttonSpacing + 20)
	m.drawClaimEditButton(screen, saveX, buttonY, buttonWidth, buttonHeight, "Save", color.RGBA{60, 140, 60, 255}, color.RGBA{100, 200, 100, 255})

	// Cancel button
	cancelX := m.screenW - (buttonWidth + 20)
	m.drawClaimEditButton(screen, cancelX, buttonY, buttonWidth, buttonHeight, "Cancel", color.RGBA{140, 60, 60, 255}, color.RGBA{200, 100, 100, 255})

	// Draw claimed territories indicator on the map
	if m.guildClaims != nil && m.territoriesManager != nil && m.territoriesManager.IsLoaded() {
		for territoryName, claimed := range m.guildClaims {
			if claimed {
				m.drawClaimedTerritoryOverlay(screen, territoryName)
			}
		}
	}
}

// drawClaimEditButton draws a button for the claim editing UI
func (m *MapView) drawClaimEditButton(screen *ebiten.Image, x, y, width, height int, label string, bgColor, hoverColor color.RGBA) {
	// Check if mouse is hovering over the button
	mx, my := ebiten.CursorPosition()
	isHovered := mx >= x && mx < x+width && my >= y && my < y+height

	// Choose color based on hover state
	currentColor := bgColor
	if isHovered {
		currentColor = hoverColor
	}

	// Draw button background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), currentColor, false)

	// Draw button border
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 2, color.RGBA{255, 255, 255, 255}, false)

	// Draw button text
	font := loadWynncraftFont(16)
	if font != nil {
		textWidth := len(label) * 8 // Approximate
		textX := x + (width-textWidth)/2
		textY := y + height/2 + 6
		text.Draw(screen, label, font, textX, textY, color.RGBA{255, 255, 255, 255})
	}
}

// drawClaimedTerritoryOverlay draws an overlay on claimed territories
func (m *MapView) drawClaimedTerritoryOverlay(screen *ebiten.Image, territoryName string) {
	if m.territoriesManager == nil {
		return
	}

	// Get territory bounds and draw overlay
	territory, exists := m.territoriesManager.Territories[territoryName]
	if !exists || len(territory.Location.Start) < 2 || len(territory.Location.End) < 2 {
		return
	}

	// Calculate territory bounds in screen coordinates
	startX := territory.Location.Start[0]*m.scale + m.offsetX
	startY := territory.Location.Start[1]*m.scale + m.offsetY
	endX := territory.Location.End[0]*m.scale + m.offsetX
	endY := territory.Location.End[1]*m.scale + m.offsetY

	// Ensure proper ordering (start < end)
	if startX > endX {
		startX, endX = endX, startX
	}
	if startY > endY {
		startY, endY = endY, startY
	}

	width := endX - startX
	height := endY - startY

	// Draw overlay with guild claim color
	overlayColor := color.RGBA{0, 255, 0, 80} // Semi-transparent green
	vector.DrawFilledRect(screen, float32(startX), float32(startY), float32(width), float32(height), overlayColor, false)

	// Draw border
	borderColor := color.RGBA{0, 255, 0, 200} // Brighter green border
	vector.StrokeRect(screen, float32(startX), float32(startY), float32(width), float32(height), 2, borderColor, false)
}

// drawAreaSelection draws the rectangle for area selection
func (m *MapView) drawAreaSelection(screen *ebiten.Image) {
	if !m.isAreaSelecting || !m.areaSelectDragging {
		return
	}

	// Calculate current mouse position
	cx, cy := ebiten.CursorPosition()

	// Determine rectangle coordinates
	x0 := float32(m.areaSelectStartX)
	y0 := float32(m.areaSelectStartY)
	x1 := float32(cx)
	y1 := float32(cy)

	// Ensure proper ordering (x0, y0) is top-left, (x1, y1) is bottom-right
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}

	// Draw semi-transparent rectangle for selection area
	var selectionColor, borderColor color.RGBA
	if m.areaSelectIsDeselecting {
		// Red colors for deselection
		selectionColor = color.RGBA{255, 100, 100, 50} // Light red with transparency
		borderColor = color.RGBA{255, 100, 100, 200}   // Light red border
	} else {
		// Green colors for selection
		selectionColor = color.RGBA{100, 255, 100, 50} // Light green with transparency
		borderColor = color.RGBA{100, 255, 100, 200}   // Light green border
	}

	vector.DrawFilledRect(screen, x0, y0, x1-x0, y1-y0, selectionColor, false)

	// Draw border for the selection rectangle
	vector.StrokeRect(screen, x0, y0, x1-x0, y1-y0, 2, borderColor, false)
}

// startAreaSelection begins area selection mode
func (m *MapView) startAreaSelection() {
	// fmt.Println("[MAP] Starting area selection mode")
	m.isAreaSelecting = true
	m.areaSelectDragging = false
	m.areaSelectTempHighlight = make(map[string]bool)

	// Don't clear existing selections - allow cumulative area selection
	// This allows users to make multiple area selections while holding Ctrl
}

// endAreaSelection ends area selection mode without applying selections
// (selections are applied immediately when mouse is released)
func (m *MapView) endAreaSelection() {
	// fmt.Println("[MAP] Ending area selection mode")

	// Clear temporary state only - selections were already applied on mouse release
	m.isAreaSelecting = false
	m.areaSelectDragging = false
	m.areaSelectTempHighlight = nil
}

// updateAreaSelection handles mouse input for area selection
func (m *MapView) updateAreaSelection() {
	mx, my := ebiten.CursorPosition()

	// Check for left mouse press to start selection dragging
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !m.areaSelectDragging {
		m.areaSelectStartX = mx
		m.areaSelectStartY = my
		m.areaSelectEndX = mx
		m.areaSelectEndY = my
		m.areaSelectDragging = true
		m.areaSelectIsDeselecting = false // Left click is for selection
		fmt.Printf("[MAP] Started area selection drag at (%d, %d)\n", mx, my)
	}

	// Check for right mouse press to start deselection dragging
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && !m.areaSelectDragging {
		m.areaSelectStartX = mx
		m.areaSelectStartY = my
		m.areaSelectEndX = mx
		m.areaSelectEndY = my
		m.areaSelectDragging = true
		m.areaSelectIsDeselecting = true // Right click is for deselection
		fmt.Printf("[MAP] Started area deselection drag at (%d, %d)\n", mx, my)
	}

	// Update drag coordinates and apply real-time selection/deselection
	if m.areaSelectDragging {
		m.areaSelectEndX = mx
		m.areaSelectEndY = my

		// Apply real-time selection as the user drags
		m.applyRealtimeAreaSelection()
	}

	// Check for left mouse release to end selection dragging
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && m.areaSelectDragging && !m.areaSelectIsDeselecting {
		// Apply the current selection when mouse is released
		m.applyCurrentAreaSelection()
		m.areaSelectDragging = false
		fmt.Printf("[MAP] Ended area selection drag at (%d, %d)\n", mx, my)
	}

	// Check for right mouse release to end deselection dragging
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) && m.areaSelectDragging && m.areaSelectIsDeselecting {
		// Apply the current deselection when mouse is released
		m.applyCurrentAreaSelection()
		m.areaSelectDragging = false
		fmt.Printf("[MAP] Ended area deselection drag at (%d, %d)\n", mx, my)
	}
}

// applyAreaSelection applies territory selections based on current area selection rectangle
func (m *MapView) applyAreaSelection() {
	if !m.isAreaSelecting || !m.territoriesManager.IsLoaded() {
		return
	}

	// Get selection bounds in screen coordinates
	x1 := float64(m.areaSelectStartX)
	y1 := float64(m.areaSelectStartY)
	x2 := float64(m.areaSelectEndX)
	y2 := float64(m.areaSelectEndY)

	// Ensure proper ordering
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	fmt.Printf("[MAP] Applying area selection from (%.0f, %.0f) to (%.0f, %.0f)\n", x1, y1, x2, y2)

	selectedCount := 0
	// Check all territories to see if they intersect with the selection rectangle
	for territoryName, border := range m.territoriesManager.TerritoryBorders {
		// Convert territory bounds to screen coordinates
		territoryX1 := border[0]*m.scale + m.offsetX
		territoryY1 := border[1]*m.scale + m.offsetY
		territoryX2 := border[2]*m.scale + m.offsetX
		territoryY2 := border[3]*m.scale + m.offsetY

		// Ensure proper ordering for territory bounds
		if territoryX1 > territoryX2 {
			territoryX1, territoryX2 = territoryX2, territoryX1
		}
		if territoryY1 > territoryY2 {
			territoryY1, territoryY2 = territoryY2, territoryY1
		}

		// Check if more than half of the territory area is covered by the selection rectangle
		// Calculate the intersection rectangle
		intersectX1 := math.Max(territoryX1, x1)
		intersectY1 := math.Max(territoryY1, y1)
		intersectX2 := math.Min(territoryX2, x2)
		intersectY2 := math.Min(territoryY2, y2)

		// Check if there's any intersection at all
		if intersectX2 > intersectX1 && intersectY2 > intersectY1 {
			// Calculate areas
			territoryArea := (territoryX2 - territoryX1) * (territoryY2 - territoryY1)
			intersectionArea := (intersectX2 - intersectX1) * (intersectY2 - intersectY1)

			// Calculate coverage percentage
			coveragePercentage := intersectionArea / territoryArea

			// Only select if more than 50% of the territory is covered
			if coveragePercentage > 0.5 {
				m.ToggleTerritoryClaim(territoryName)
				selectedCount++
				fmt.Printf("[MAP] Territory %s selected (coverage: %.1f%%)\n", territoryName, coveragePercentage*100)
			}
		}
	}

	fmt.Printf("[MAP] Area selection applied to %d territories\n", selectedCount)
}

// applyRealtimeAreaSelection updates temporary territory highlights in real-time as the user drags
func (m *MapView) applyRealtimeAreaSelection() {
	if !m.isAreaSelecting || !m.areaSelectDragging || !m.territoriesManager.IsLoaded() || m.areaSelectTempHighlight == nil {
		return
	}

	// Get selection bounds in screen coordinates
	x1 := float64(m.areaSelectStartX)
	y1 := float64(m.areaSelectStartY)
	x2 := float64(m.areaSelectEndX)
	y2 := float64(m.areaSelectEndY)

	// Ensure proper ordering
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	// Clear previous temporary highlights
	for territoryName := range m.areaSelectTempHighlight {
		delete(m.areaSelectTempHighlight, territoryName)
	}

	// Check all territories to see if they should be temporarily highlighted
	for territoryName, border := range m.territoriesManager.TerritoryBorders {
		// Convert territory bounds to screen coordinates
		territoryX1 := border[0]*m.scale + m.offsetX
		territoryY1 := border[1]*m.scale + m.offsetY
		territoryX2 := border[2]*m.scale + m.offsetX
		territoryY2 := border[3]*m.scale + m.offsetY

		// Ensure proper ordering for territory bounds
		if territoryX1 > territoryX2 {
			territoryX1, territoryX2 = territoryX2, territoryX1
		}
		if territoryY1 > territoryY2 {
			territoryY1, territoryY2 = territoryY2, territoryY1
		}

		// Check if more than half of the territory area is covered by the selection rectangle
		// Calculate the intersection rectangle
		intersectX1 := math.Max(territoryX1, x1)
		intersectY1 := math.Max(territoryY1, y1)
		intersectX2 := math.Min(territoryX2, x2)
		intersectY2 := math.Min(territoryY2, y2)

		// Check if there's any intersection at all
		if intersectX2 > intersectX1 && intersectY2 > intersectY1 {
			// Calculate areas
			intersectionArea := (intersectX2 - intersectX1) * (intersectY2 - intersectY1)
			territoryArea := (territoryX2 - territoryX1) * (territoryY2 - territoryY1)
			// Only highlight if more than 50% of territory is covered
			if intersectionArea > territoryArea*0.5 {
				if m.areaSelectIsDeselecting {
					// For deselection, only highlight territories that are currently claimed
					if m.guildClaims != nil && m.guildClaims[territoryName] {

						m.areaSelectTempHighlight[territoryName] = true
					}
				} else {
					// For selection, highlight territories that are not currently claimed
					isCurrentlyClaimed := m.guildClaims != nil && m.guildClaims[territoryName]
					if !isCurrentlyClaimed {
						m.areaSelectTempHighlight[territoryName] = true
					}
				}
			}
		}
	}

	// Update the territory renderer to show temporary highlights
	if m.territoriesManager != nil && m.territoriesManager.IsLoaded() && m.territoriesManager.territoryRenderer != nil {
		// Create a combined map of permanent and temporary selections
		combinedClaims := make(map[string]bool)

		// Add existing permanent claims
		if m.guildClaims != nil {
			for territory, claimed := range m.guildClaims {
				combinedClaims[territory] = claimed
			}
		}

		// Handle temporary highlights based on selection/deselection mode
		for territory, highlighted := range m.areaSelectTempHighlight {
			if highlighted {
				if m.areaSelectIsDeselecting {
					// In deselection mode: temporarily remove the territory (show as unclaimed)
					combinedClaims[territory] = false
				} else {
					// In selection mode: temporarily add the territory (show as claimed)
					combinedClaims[territory] = true
				}
			}
		}

		// Update the renderer with combined claims for immediate visual feedback
		m.territoriesManager.territoryRenderer.SetEditingGuild(m.editingGuildName, m.editingGuildTag, combinedClaims)
	}
}

// applyCurrentAreaSelection applies the current area selection to permanent claims or loadout selections
func (m *MapView) applyCurrentAreaSelection() {
	// Apply final selection/deselection for all temporarily highlighted territories
	if m.areaSelectTempHighlight != nil {
		// Check if we're in loadout application mode
		loadoutManager := GetLoadoutManager()
		isLoadoutMode := loadoutManager != nil && loadoutManager.IsApplyingLoadout()

		for territoryName, highlighted := range m.areaSelectTempHighlight {
			if highlighted {
				if isLoadoutMode {
					// Handle loadout application mode area selection
					if m.areaSelectIsDeselecting {
						// Deselection mode: remove territory from loadout selection
						loadoutManager.RemoveTerritorySelection(territoryName)
					} else {
						// Selection mode: add territory to loadout selection
						loadoutManager.AddTerritorySelection(territoryName)
					}
				} else if m.isEditingClaims {
					// Handle claim editing mode area selection
					if m.areaSelectIsDeselecting {
						// Deselection mode: remove territory from claims
						if m.guildClaims != nil && m.guildClaims[territoryName] {
							m.guildClaims[territoryName] = false
							fmt.Printf("[MAP] Deselected territory: %s\n", territoryName)
						}
					} else {
						// Selection mode: add territory to claims (only if not already claimed)
						if m.guildClaims == nil {
							m.guildClaims = make(map[string]bool)
						}
						if !m.guildClaims[territoryName] {
							m.guildClaims[territoryName] = true
							fmt.Printf("[MAP] Selected territory: %s\n", territoryName)
						}
					}
				}
			}
		}

		// Update the territory renderer with the updated claims (only for claim editing mode)
		if m.isEditingClaims && m.territoriesManager != nil && m.territoriesManager.IsLoaded() && m.territoriesManager.territoryRenderer != nil {
			m.territoriesManager.territoryRenderer.SetEditingGuild(m.editingGuildName, m.editingGuildTag, m.guildClaims)
		}

		// Update the territory renderer with the updated loadout selection (only for loadout application mode)
		if isLoadoutMode && m.territoriesManager != nil && m.territoriesManager.IsLoaded() && m.territoriesManager.territoryRenderer != nil {
			if loadoutManager := GetLoadoutManager(); loadoutManager != nil && loadoutManager.IsApplyingLoadout() {
				if renderer := m.territoriesManager.GetRenderer(); renderer != nil {
					selectedTerritories := loadoutManager.GetSelectedTerritories()
					applyingLoadoutName := loadoutManager.GetApplyingLoadoutName()
					renderer.SetLoadoutApplicationMode(applyingLoadoutName, selectedTerritories)
					// Force a redraw to show the highlighting
					if cache := renderer.GetTerritoryCache(); cache != nil {
						cache.ForceRedraw()
					}
				}
			}
		}

		// Clear the temporary highlights after applying them
		m.areaSelectTempHighlight = make(map[string]bool)
	}
}
