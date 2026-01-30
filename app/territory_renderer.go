package app

import (
	"image"
	"image/color"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// TradingRouteID represents a unique identifier for a trading route line
type TradingRouteID struct {
	From string
	To   string
}

// TradingRouteLine represents a trading route line with its properties
type TradingRouteLine struct {
	ID         TradingRouteID
	Color      color.RGBA
	Thickness  float32
	IsSelected bool
	IsVisible  bool
	CreatedAt  time.Time
}

// TerritoryRenderer handles the rendering of territories with memory-efficient caching
type TerritoryRenderer struct {
	manager *TerritoriesManager

	// Trading route line management
	tradingRouteLines map[TradingRouteID]*TradingRouteLine
	routeLinesMutex   sync.RWMutex
	selectedRouteID   *TradingRouteID

	// Memory-efficient territory cache
	territoryCache *TerritoryCache

	// Guild claim editing information
	editingGuildName string
	editingGuildTag  string
	editingClaims    map[string]bool

	// Loadout application mode information
	isLoadoutApplicationMode bool
	loadoutName              string
	selectedForLoadout       map[string]bool

	// White pixel image for drawing
	whitePixel *ebiten.Image
}

// TerritoryCache is a placeholder for the territory cache system.
type TerritoryCache struct {
	manager *TerritoriesManager
}

// NewTerritoryCache creates a new TerritoryCache instance.
func NewTerritoryCache() *TerritoryCache {
	return &TerritoryCache{}
}

// AttachManager allows the cache to know about its manager for invalidation.
func (tc *TerritoryCache) AttachManager(manager *TerritoriesManager) {
	tc.manager = manager
}

// ForceRedraw marks the buffer as needing update, forcing a redraw next frame.
func (tc *TerritoryCache) ForceRedraw() {
	if tc.manager != nil {
		tc.manager.bufferMutex.Lock()
		tc.manager.bufferNeedsUpdate = true
		tc.manager.bufferMutex.Unlock()
	}
}

// InvalidateCache is an alias for ForceRedraw.
func (tc *TerritoryCache) InvalidateCache() {
	tc.ForceRedraw()
}

// NewTerritoryRenderer creates a new territory renderer
func NewTerritoryRenderer(manager *TerritoriesManager) *TerritoryRenderer {
	cache := NewTerritoryCache()
	cache.AttachManager(manager) // Attach the manager to the cache

	return &TerritoryRenderer{
		manager:           manager,
		tradingRouteLines: make(map[TradingRouteID]*TradingRouteLine),
		selectedRouteID:   nil,
		territoryCache:    cache,
		editingClaims:     make(map[string]bool), // Initialize the editing claims map
	}
}

// CreateOrUpdateTradingRoute creates or updates a trading route line
func (tr *TerritoryRenderer) CreateOrUpdateTradingRoute(from, to string, customColor *color.RGBA) {
	tr.routeLinesMutex.Lock()
	defer tr.routeLinesMutex.Unlock()

	routeID := TradingRouteID{From: from, To: to}

	// Check if route already exists
	if existing, exists := tr.tradingRouteLines[routeID]; exists {
		// Update existing route
		if customColor != nil {
			existing.Color = *customColor
		}
		existing.IsVisible = true
		return
	}

	// Create new route with default or custom color
	routeColor := tr.manager.RouteColor
	if customColor != nil {
		routeColor = *customColor
	} else {
		// Increase visibility by making routes more opaque
		routeColor.A = 220 // Increase alpha from 160 to 220 for better visibility
	}

	tr.tradingRouteLines[routeID] = &TradingRouteLine{
		ID:         routeID,
		Color:      routeColor,
		Thickness:  float32(tr.manager.RouteThickness),
		IsSelected: false,
		IsVisible:  true,
		CreatedAt:  time.Now(),
	}
}

// GetTradingRoute gets a trading route line by ID
func (tr *TerritoryRenderer) GetTradingRoute(from, to string) *TradingRouteLine {
	tr.routeLinesMutex.RLock()
	defer tr.routeLinesMutex.RUnlock()

	routeID := TradingRouteID{From: from, To: to}
	return tr.tradingRouteLines[routeID]
}

// SelectTradingRoute selects a trading route for highlighting
func (tr *TerritoryRenderer) SelectTradingRoute(from, to string) {
	tr.routeLinesMutex.Lock()
	defer tr.routeLinesMutex.Unlock()

	// Deselect previous route
	if tr.selectedRouteID != nil {
		if prevRoute, exists := tr.tradingRouteLines[*tr.selectedRouteID]; exists {
			prevRoute.IsSelected = false
		}
	}

	// Select new route
	routeID := TradingRouteID{From: from, To: to}
	if route, exists := tr.tradingRouteLines[routeID]; exists {
		route.IsSelected = true
		tr.selectedRouteID = &routeID
	} else {
		tr.selectedRouteID = nil
	}
}

// ClearRouteSelection clears the current route selection
func (tr *TerritoryRenderer) ClearRouteSelection() {
	tr.routeLinesMutex.Lock()
	defer tr.routeLinesMutex.Unlock()

	if tr.selectedRouteID != nil {
		if route, exists := tr.tradingRouteLines[*tr.selectedRouteID]; exists {
			route.IsSelected = false
		}
		tr.selectedRouteID = nil
	}
}

// SetRouteColor sets a custom color for a specific route
func (tr *TerritoryRenderer) SetRouteColor(from, to string, color color.RGBA) {
	tr.routeLinesMutex.Lock()
	defer tr.routeLinesMutex.Unlock()

	routeID := TradingRouteID{From: from, To: to}
	if route, exists := tr.tradingRouteLines[routeID]; exists {
		route.Color = color
	}
}

// GetSelectedRoute returns the currently selected route ID
func (tr *TerritoryRenderer) GetSelectedRoute() *TradingRouteID {
	tr.routeLinesMutex.RLock()
	defer tr.routeLinesMutex.RUnlock()

	return tr.selectedRouteID
}

// GetTerritoryCache returns the territory cache
func (tr *TerritoryRenderer) GetTerritoryCache() *TerritoryCache {
	return tr.territoryCache
}

// SetEditingGuild sets the guild being edited for claim highlighting
func (tr *TerritoryRenderer) SetEditingGuild(name, tag string, claims map[string]bool) {
	// fmt.Printf("[RENDERER] SetEditingGuild called: name=%s, tag=%s, claims count=%d\n", name, tag, len(claims))
	tr.editingGuildName = name
	tr.editingGuildTag = tag
	tr.editingClaims = claims

	// Force a redraw whenever editing guild information changes
	if tr.territoryCache != nil {
		// fmt.Printf("[RENDERER] Forcing cache redraw\n")
		tr.territoryCache.ForceRedraw()
	}
}

// ClearEditingGuild clears the editing guild information
func (tr *TerritoryRenderer) ClearEditingGuild() {
	tr.editingGuildName = ""
	tr.editingGuildTag = ""
	tr.editingClaims = nil
}

// GetEditingGuildName returns the name of the currently editing guild
func (tr *TerritoryRenderer) GetEditingGuildName() string {
	return tr.editingGuildName
}

// IsTerritoryClaimed checks if a territory is claimed by the editing guild
func (tr *TerritoryRenderer) IsTerritoryClaimed(territoryName string) bool {
	if tr.editingClaims == nil {
		// fmt.Printf("[RENDERER] IsTerritoryClaimed(%s): editingClaims is nil\n", territoryName)
		return false
	}
	claimed, exists := tr.editingClaims[territoryName]
	result := exists && claimed
	// fmt.Printf("[RENDERER] IsTerritoryClaimed(%s): exists=%v, claimed=%v, result=%v\n", territoryName, exists, claimed, result)
	return result
}

// SetLoadoutApplicationMode sets the loadout application mode and selected territories
func (tr *TerritoryRenderer) SetLoadoutApplicationMode(loadoutName string, selectedTerritories map[string]bool) {
	tr.isLoadoutApplicationMode = true
	tr.loadoutName = loadoutName
	tr.selectedForLoadout = selectedTerritories
	// fmt.Printf("[RENDERER] Set loadout application mode: %s, selected territories: %v\n", loadoutName, selectedTerritories)
}

// ClearLoadoutApplicationMode clears the loadout application mode
func (tr *TerritoryRenderer) ClearLoadoutApplicationMode() {
	tr.isLoadoutApplicationMode = false
	tr.loadoutName = ""
	tr.selectedForLoadout = nil
	// fmt.Printf("[RENDERER] Cleared loadout application mode\n")
}

// IsLoadoutApplicationMode returns whether the renderer is in loadout application mode
func (tr *TerritoryRenderer) IsLoadoutApplicationMode() bool {
	return tr.isLoadoutApplicationMode
}

// IsTerritorySelectedForLoadout checks if a territory is selected for loadout application
func (tr *TerritoryRenderer) IsTerritorySelectedForLoadout(territoryName string) bool {
	if tr.selectedForLoadout == nil {
		// fmt.Printf("[RENDERER] IsTerritorySelectedForLoadout(%s): selectedForLoadout is nil\n", territoryName)
		return false
	}
	selected, exists := tr.selectedForLoadout[territoryName]
	result := exists && selected
	// fmt.Printf("[RENDERER] IsTerritorySelectedForLoadout(%s): exists=%v, selected=%v, result=%v\n", territoryName, exists, selected, result)
	return result
}

// RenderToBuffer renders territories and routes to the specified buffer with memory-efficient caching
func (tr *TerritoryRenderer) RenderToBuffer(buffer *ebiten.Image, scale, viewX, viewY float64, hoveredTerritory string) {
	if !tr.manager.isLoaded {
		return
	}

	// Temporarily re-enable old rectangle rendering during claim editing to debug green stripes
	// Skip old rectangle-based rendering when in claim editing mode to avoid conflicts with new GPU system
	// if mapView := GetMapView(); mapView != nil && mapView.IsEditingClaims() {
	//	return
	// }

	// Use the memory-efficient cache system for direct rendering
	// This approach eliminates temporary buffer creation and reduces memory usage
	tr.territoryCache.renderDirectly(buffer, scale, viewX, viewY, tr.manager, tr, hoveredTerritory)
}

// renderDirectly implements the territory cache rendering logic by calling the actual territory drawing function.
func (tc *TerritoryCache) renderDirectly(buffer *ebiten.Image, scale, viewX, viewY float64, manager *TerritoriesManager, renderer *TerritoryRenderer, hoveredTerritory string) {
	// fmt.Printf("[TERRITORY_CACHE] renderDirectly called with scale=%.2f, renderer editing guild='%s'\n", scale, renderer.editingGuildName)

	// Get buffer dimensions for culling
	bounds := buffer.Bounds()
	screenWidth := float64(bounds.Dx())
	screenHeight := float64(bounds.Dy())

	// Get visible territories
	manager.territoryMutex.RLock()
	visible := make(map[string]struct{})
	for name := range manager.TerritoryBorders {
		visible[name] = struct{}{}
	}
	manager.territoryMutex.RUnlock()

	// Call the actual territory drawing function that handles guild colors and editing
	renderer.drawTerritoriesToOverlayWithHover(buffer, visible, scale, viewX, viewY, screenWidth, screenHeight, hoveredTerritory)
}

// drawRoutesToOverlay draws trading routes to the overlay buffer
func (tr *TerritoryRenderer) drawRoutesToOverlay(overlay *ebiten.Image, visible map[string]struct{}, scale, viewX, viewY, screenWidth, screenHeight float64) {
	tm := tr.manager
	tm.territoryMutex.RLock()
	defer tm.territoryMutex.RUnlock()

	for name := range visible {
		territory := tm.Territories[name]
		border1, exists := tm.TerritoryBorders[name]
		if !exists {
			continue
		}

		centerX1_no_transform := (border1[0] + border1[2]) / 2
		centerY1_no_transform := (border1[1] + border1[3]) / 2
		screenCenterX1 := centerX1_no_transform*scale + viewX
		screenCenterY1 := centerY1_no_transform*scale + viewY

		for _, routeName := range territory.TradingRoutes {
			border2, exists := tm.TerritoryBorders[routeName]
			if !exists {
				continue
			}

			centerX2_no_transform := (border2[0] + border2[2]) / 2
			centerY2_no_transform := (border2[1] + border2[3]) / 2
			screenCenterX2 := centerX2_no_transform*scale + viewX
			screenCenterY2 := centerY2_no_transform*scale + viewY

			// Cull lines that are completely outside the screen
			lineMinX := math.Min(screenCenterX1, screenCenterX2)
			lineMaxX := math.Max(screenCenterX1, screenCenterX2)
			lineMinY := math.Min(screenCenterY1, screenCenterY2)
			lineMaxY := math.Max(screenCenterY1, screenCenterY2)

			if lineMaxX <= 0 || lineMinX >= screenWidth || lineMaxY <= 0 || lineMinY >= screenHeight {
				continue
			}

			// Create or update the trading route line
			tr.CreateOrUpdateTradingRoute(name, routeName, nil)

			// Get the route line properties
			routeID := TradingRouteID{From: name, To: routeName}
			tr.routeLinesMutex.RLock()
			routeLine := tr.tradingRouteLines[routeID]
			tr.routeLinesMutex.RUnlock()

			if routeLine == nil || !routeLine.IsVisible {
				continue
			}

			// Determine route color and thickness
			routeColor := routeLine.Color
			routeThickness := routeLine.Thickness

			// Highlight selected route
			if routeLine.IsSelected {
				// Make selected routes brighter and thicker
				routeColor.R = uint8(math.Min(255, float64(routeColor.R)*1.3))
				routeColor.G = uint8(math.Min(255, float64(routeColor.G)*1.3))
				routeColor.B = uint8(math.Min(255, float64(routeColor.B)*1.3))
				routeThickness *= 1.5
			}

			// Draw the route line with improved visibility
			vector.StrokeLine(overlay,
				float32(screenCenterX1), float32(screenCenterY1),
				float32(screenCenterX2), float32(screenCenterY2),
				float32(routeThickness), routeColor, true)
		}
	}
}

// drawTerritoriesToOverlayWithHover draws territory rectangles to the overlay buffer with hover information
func (tr *TerritoryRenderer) drawTerritoriesToOverlayWithHover(overlay *ebiten.Image, visible map[string]struct{}, scale, viewX, viewY, screenWidth, screenHeight float64, hoveredTerritory string) {
	tm := tr.manager

	// Create a 1x1 white pixel if we don't have one
	if tr.whitePixel == nil {
		tr.whitePixel = ebiten.NewImage(1, 1)
		tr.whitePixel.Fill(color.RGBA{255, 255, 255, 255})
	}

	// Batch all territories into a single DrawTriangles call for performance
	vertices := make([]ebiten.Vertex, 0, len(visible)*4) // 4 vertices per territory
	indices := make([]uint16, 0, len(visible)*6)         // 6 indices per territory (2 triangles)
	vertexIndex := uint16(0)

	for name := range visible {
		border := tm.TerritoryBorders[name]

		// Validate border data to prevent rendering artifacts
		if len(border) < 4 {
			// fmt.Printf("[RENDERER] Warning: Invalid border data for territory %s (length %d)\n", name, len(border))
			continue
		}

		// Check for NaN or infinite values in border coordinates
		if !(border[0] == border[0]) || !(border[1] == border[1]) || !(border[2] == border[2]) || !(border[3] == border[3]) {
			// fmt.Printf("[RENDERER] Warning: NaN/Inf values in border data for territory %s\n", name)
			continue
		}

		// Apply view transformation (scale and offset)
		tx1 := border[0]*scale + viewX
		ty1 := border[1]*scale + viewY
		tx2 := border[2]*scale + viewX
		ty2 := border[3]*scale + viewY

		// Culling: Check if the box is outside the screen
		if tx2 <= 0 || tx1 >= screenWidth || ty2 <= 0 || ty1 >= screenHeight {
			continue
		}

		width := tx2 - tx1
		height := ty2 - ty1

		// Skip if territory would be too small to render or has invalid dimensions
		if width <= 0 || height <= 0 {
			continue
		} // Set fill and border colors
		fillColor := tm.FillColor
		borderColor := tm.BorderColor

		// First check if this territory has a persistent guild claim
		if claimManager := GetGuildClaimManager(); claimManager != nil {
			if claim, exists := claimManager.GetClaim(name); exists {
				// Get guild color from the guild manager
				if guildManager := GetEnhancedGuildManager(); guildManager != nil {
					if guildColor, found := guildManager.GetGuildColor(claim.GuildName, claim.GuildTag); found {
						// Use guild color for the territory
						fillColor = guildColor
						borderColor = guildColor
					}
				}
			}
		}

		// Check if this territory is claimed by the currently editing guild (override persistent claims)
		if tr.editingGuildName != "" && tr.IsTerritoryClaimed(name) {
			// fmt.Printf("[RENDERER] Territory %s is claimed by editing guild %s [%s]\n", name, tr.editingGuildName, tr.editingGuildTag)
			// Get guild color from the guild manager
			if guildManager := GetEnhancedGuildManager(); guildManager != nil {
				if guildColor, found := guildManager.GetGuildColor(tr.editingGuildName, tr.editingGuildTag); found {
					// fmt.Printf("[RENDERER] Using guild color for territory %s: R=%d G=%d B=%d\n", name, guildColor.R, guildColor.G, guildColor.B)
					// Use guild color for the territory
					fillColor = guildColor
					borderColor = guildColor
				} else {
					// fmt.Printf("[RENDERER] Guild color not found for %s [%s]\n", tr.editingGuildName, tr.editingGuildTag)
				}
			}
		}

		// Check if this territory is selected for loadout application (override other colors)
		isSelectedForLoadout := tr.isLoadoutApplicationMode && tr.IsTerritorySelectedForLoadout(name)
		if isSelectedForLoadout {
			// fmt.Printf("[RENDERER] Territory %s is selected for loadout application - applying yellow color\n", name)
			// Use bright yellow for selected territories
			fillColor = color.RGBA{255, 255, 0, 120}   // Bright yellow with moderate opacity
			borderColor = color.RGBA{255, 200, 0, 255} // Slightly orange-yellow for border with full opacity
			// fmt.Printf("[RENDERER] Set colors - Fill: R=%d G=%d B=%d A=%d, Border: R=%d G=%d B=%d A=%d\n",
			// fillColor.R, fillColor.G, fillColor.B, fillColor.A,
			// borderColor.R, borderColor.G, borderColor.B, borderColor.A)
		} else if tr.isLoadoutApplicationMode {
			// fmt.Printf("[RENDERER] Territory %s is NOT selected for loadout (loadout mode active)\n", name)
		}

		// For the enhanced shader approach, we need:
		// 1. Draw fills with VERY LOW opacity so the shader can further reduce it
		// 2. Draw borders with HIGH contrast to make them detectable by the shader

		// Only apply these modifications if NOT selected for loadout (to preserve yellow color)
		if !isSelectedForLoadout {
			// Opacity for fill areas - increased for better visibility
			fillColor.A = 70 // Higher starting alpha to improve visibility, especially over blue water

			// High contrast for borders to make them easily detectable by the luminance threshold
			borderColor.A = 255
			// Make border much brighter for better detection by the shader
			borderColor.R = uint8(math.Min(255, float64(borderColor.R)*1.5))
			borderColor.G = uint8(math.Min(255, float64(borderColor.G)*1.5))
			borderColor.B = uint8(math.Min(255, float64(borderColor.B)*1.5))
		}

		// Handle blinking effect for selected territory with 330ms cycles
		if tm.selectedTerritory == name && tm.isBlinking {
			blinkCycle := math.Mod(tm.blinkTimer*1000, 660) // 660ms total cycle (330ms + 330ms)
			if blinkCycle < 330 {                           // First 330ms - lighten
				// During blink, adjust colors for detection by shader
				fillColor.A = uint8(float64(fillColor.A) * 0.8)
				borderColor.A = uint8(float64(borderColor.A) * 0.8)
			}
			// After 330ms - darken (default colors)

			// Encode selected state with a special marker in the green channel
			// We want to preserve the original territory color instead of forcing red
			// So set green to max to identify this as a selected territory
			originalR := fillColor.R

			// Store original colors but adjust to encode selection
			fillColor.G = 255 // Use green channel to encode selection
			borderColor.G = 255

			// Keep red channel to preserve some of the original color information
			// Use original red value but slightly enhanced
			fillColor.R = uint8(math.Min(255, float64(originalR)*1.2))
			borderColor.R = uint8(math.Min(255, float64(originalR)*1.2))
		} // Encode hover state for the shader to detect
		// But only if it's not selected (to avoid conflicts)
		isHovered := (hoveredTerritory == name)
		if isHovered && !(tm.selectedTerritory == name && tm.isBlinking) {
			// Set alpha channel to maximum to encode hover state (will be used by shader)
			fillColor.A = 255
			borderColor.A = 255

			// FLIPPED HOVER EFFECT: We'll keep the same colors, since the shader will handle the darkening
			// Just set an identifier (max alpha) for the shader to detect which territories are hovered
			// No color modification here as the shader will now make hovered territories darker
		}

		// Validate color values to prevent shader artifacts
		if fillColor.R > 255 {
			fillColor.R = 255
		}
		if fillColor.G > 255 {
			fillColor.G = 255
		}
		if fillColor.B > 255 {
			fillColor.B = 255
		}
		if fillColor.A > 255 {
			fillColor.A = 255
		}

		// Draw filled rectangle
		rect := image.Rect(int(tx1), int(ty1), int(tx2), int(ty2))

		// Additional validation to prevent invalid rectangles that could cause rendering artifacts
		// Allow territories to extend beyond screen, but prevent extreme negative values or NaN/Inf
		if rect.Dx() > 0 && rect.Dy() > 0 &&
			tx1 == tx1 && ty1 == ty1 && tx2 == tx2 && ty2 == ty2 && // Check for NaN
			tx1 > -10000 && ty1 > -10000 && tx2 < 20000 && ty2 < 20000 { // Reasonable bounds

			// Use DrawTriangles instead of Fill to avoid potential SubImage issues
			// Add vertices to batch instead of drawing individually
			colorR := float32(fillColor.R) / 255
			colorG := float32(fillColor.G) / 255
			colorB := float32(fillColor.B) / 255
			colorA := float32(fillColor.A) / 255

			// Debug: Log vertex colors for selected territories
			if tr.isLoadoutApplicationMode && tr.IsTerritorySelectedForLoadout(name) {
				// fmt.Printf("[RENDERER] Creating vertices for selected territory %s with colors: R=%.3f G=%.3f B=%.3f A=%.3f\n",
				// name, colorR, colorG, colorB, colorA)
			}

			// Add 4 vertices for this territory rectangle
			vertices = append(vertices,
				ebiten.Vertex{DstX: float32(tx1), DstY: float32(ty1), SrcX: 0, SrcY: 0, ColorR: colorR, ColorG: colorG, ColorB: colorB, ColorA: colorA},
				ebiten.Vertex{DstX: float32(tx2), DstY: float32(ty1), SrcX: 0, SrcY: 0, ColorR: colorR, ColorG: colorG, ColorB: colorB, ColorA: colorA},
				ebiten.Vertex{DstX: float32(tx1), DstY: float32(ty2), SrcX: 0, SrcY: 0, ColorR: colorR, ColorG: colorG, ColorB: colorB, ColorA: colorA},
				ebiten.Vertex{DstX: float32(tx2), DstY: float32(ty2), SrcX: 0, SrcY: 0, ColorR: colorR, ColorG: colorG, ColorB: colorB, ColorA: colorA},
			)

			// Add 6 indices for 2 triangles that form the rectangle
			indices = append(indices, vertexIndex, vertexIndex+1, vertexIndex+2, vertexIndex+1, vertexIndex+2, vertexIndex+3)
			vertexIndex += 4
		}

		// Territory borders removed for performance optimization since this shit takes 90% of the CPU time of the drawing routine
		// vector.StrokeRect(overlay, float32(tx1), float32(ty1), float32(width), float32(height), float32(tm.BorderThickness), borderColor, true)
	}

	// Draw all territories in a single batched DrawTriangles call
	if len(vertices) > 0 {
		opts := &ebiten.DrawTrianglesOptions{}
		opts.CompositeMode = ebiten.CompositeModeSourceOver
		overlay.DrawTriangles(vertices, indices, tr.whitePixel, opts)
	}
}
