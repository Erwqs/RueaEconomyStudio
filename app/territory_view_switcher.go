package app

import (
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"RueaES/eruntime"
	"RueaES/pluginhost"
	"RueaES/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// TerritoryViewType represents different territory view modes
type TerritoryViewType int

const (
	ViewGuild TerritoryViewType = iota
	ViewResource
	ViewProduction
	ViewSetDefence
	ViewAtDefence
	ViewTreasury
	ViewTreasuryOverrides
	ViewWarning
	ViewTax
	ViewFilter
	ViewThroughput
	ViewAnalysis
	ViewPluginsExtension
)

// TerritoryViewInfo holds information about each view type
type TerritoryViewInfo struct {
	Name        string
	Description string
	HiddenGuild string // The "hidden guild" name used for coloring
}

// TerritoryViewSwitcher manages the territory view switching functionality
type TerritoryViewSwitcher struct {
	currentView   TerritoryViewType
	hideDelay     time.Duration
	keysPressed   map[ebiten.Key]bool
	lastKeyCheck  time.Time
	modalVisible  bool
	selectedIndex int
	views         []TerritoryViewInfo
	color         map[string]color.RGBA // Map of hidden guild names to their colors

	highlightPos         float64
	highlightTarget      float64
	highlightLastUpdate  time.Time
	highlightInitialized bool
	lastMouseX           int
	lastMouseY           int
	lastMouseMove        time.Time
	highlightWrapDir     int
	highlightWrapTo      int

	throughputCache         map[string]float64 // Territory name -> summed in-guild transit value
	throughputGuildPeak     map[string]float64 // Guild tag -> max throughput for normalization
	throughputCacheTick     uint64             // Tick the throughput cache was built on
	throughputCacheWallTime time.Time          // Wall-clock time the throughput cache was built

	// Analysis (chokepoint) visualization
	analysisScoresCardinal map[string]float64 // normalized 0..1 by max
	analysisScoresOrdinal  map[string]float64 // percentile-based 0..1 for ordinal mode
	analysisGuild          string

	filteredTerritories map[string]float64
	filterActive        bool
}

// NewTerritoryViewSwitcher creates a new territory view switcher
func NewTerritoryViewSwitcher() *TerritoryViewSwitcher {
	tvs := &TerritoryViewSwitcher{
		currentView:   ViewGuild,
		hideDelay:     500 * time.Millisecond, // Hide after 500ms of no key press
		keysPressed:   make(map[ebiten.Key]bool),
		lastKeyCheck:  time.Now(),
		modalVisible:  false,
		selectedIndex: 0,
		views: []TerritoryViewInfo{
			{Name: "Guild", Description: "Guild territory view", HiddenGuild: "__VIEW_GUILD__"},
			{Name: "Resource", Description: "Resource type view", HiddenGuild: "__VIEW_RESOURCE__"},
			{Name: "Production", Description: "Production level, type and issues view", HiddenGuild: "__VIEW_PRODUCTION__"},
			{Name: "Set Defence", Description: "Set defence level view", HiddenGuild: "__VIEW_SET_DEFENCE__"},
			{Name: "At Defence", Description: "At defence level view", HiddenGuild: "__VIEW_AT_DEFENCE__"},
			{Name: "Treasury", Description: "Treasury level view", HiddenGuild: "__VIEW_TREASURY__"},
			{Name: "Treasury Overrides", Description: "Treasury override view", HiddenGuild: "__VIEW_TREASURY_OVERRIDES__"},
			{Name: "Warning", Description: "Warning status view", HiddenGuild: "__VIEW_WARNING__"},
			{Name: "Tax", Description: "Territory taxation level", HiddenGuild: "__VIEW_TAX__"},
			{Name: "Filter", Description: "Filtered territories view", HiddenGuild: "__VIEW_FILTER__"},
			{Name: "Throughput", Description: "In-guild transit load heatmap", HiddenGuild: "__VIEW_THROUGHPUT__"},
			{Name: "Analysis", Description: "Analysis results view", HiddenGuild: "__VIEW_ANALYSIS__"},
			{Name: "Extension", Description: "Extension provided overlays", HiddenGuild: "__VIEW_PLUGINS_EXTENSION__"},
		},
		color:                make(map[string]color.RGBA),
		filteredTerritories:  make(map[string]float64),
		filterActive:         false,
		highlightPos:         0,
		highlightTarget:      0,
		highlightLastUpdate:  time.Now(),
		highlightInitialized: false,
		lastMouseX:           0,
		lastMouseY:           0,
		lastMouseMove:        time.Now(),
		highlightWrapDir:     0,
		highlightWrapTo:      0,
	}

	// Initialize hidden guild colors
	tvs.initializeHiddenGuilds()

	return tvs
}

// SetCurrentView sets the active view programmatically (e.g., when analysis finishes).
func (tvs *TerritoryViewSwitcher) SetCurrentView(view TerritoryViewType) {
	if view < ViewGuild || int(view) >= len(tvs.views) {
		return
	}
	tvs.currentView = view
	tvs.selectedIndex = int(view)
	tvs.highlightTarget = float64(tvs.selectedIndex)
	if !tvs.modalVisible {
		tvs.highlightPos = tvs.highlightTarget
		tvs.highlightInitialized = true
	}
	tvs.highlightWrapDir = 0
}

func smoothstep(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func scaledColor(c color.RGBA, factor float64) color.RGBA {
	scale := func(v uint8) uint8 {
		val := float64(v) * factor
		if val < 0 {
			val = 0
		}
		if val > 255 {
			val = 255
		}
		return uint8(val)
	}

	return color.RGBA{R: scale(c.R), G: scale(c.G), B: scale(c.B), A: c.A}
}

func blendColors(a, b color.RGBA, t float64) color.RGBA {
	t = math.Min(1, math.Max(0, t))
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: mix(a.A, b.A)}
}

// initializeHiddenGuilds sets up the color schemes for each view type
func (tvs *TerritoryViewSwitcher) initializeHiddenGuilds() {
	// Resource colors (normal production - 3600 base)
	tvs.color["__RESOURCE_WOOD__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}    // Lime green
	tvs.color["__RESOURCE_CROP__"] = color.RGBA{R: 255, G: 215, B: 0, A: 255}    // Gold/yellow
	tvs.color["__RESOURCE_FISH__"] = color.RGBA{R: 30, G: 144, B: 255, A: 255}   // Dodger blue (more blue)
	tvs.color["__RESOURCE_ORE__"] = color.RGBA{R: 255, G: 127, B: 193, A: 255}   // Light pink (less pink, more white)
	tvs.color["__RESOURCE_MULTI__"] = color.RGBA{R: 255, G: 255, B: 255, A: 255} // White for multiple resources
	defaultEmerald := typedef.DefaultResourceColors().Emerald.ToRGBA()
	tvs.color["__RESOURCE_EMERALD__"] = defaultEmerald
	tvs.color["__RESOURCE_EMERALD_DOUBLE__"] = scaledColor(defaultEmerald, 0.8)

	// Resource colors (double production - 7200+ base) - lighter variants
	tvs.color["__RESOURCE_WOOD_DOUBLE__"] = color.RGBA{R: 34, G: 139, B: 34, A: 255}  // Forest green (darker)
	tvs.color["__RESOURCE_CROP_DOUBLE__"] = color.RGBA{R: 218, G: 165, B: 32, A: 255} // Goldenrod (darker)
	tvs.color["__RESOURCE_FISH_DOUBLE__"] = color.RGBA{R: 70, G: 130, B: 180, A: 255} // Steel blue (darker)
	tvs.color["__RESOURCE_ORE_DOUBLE__"] = color.RGBA{R: 199, G: 21, B: 133, A: 255}  // Medium violet red (darker)

	// Defence level colors (very low to very high)
	tvs.color["__DEFENCE_VERY_LOW__"] = color.RGBA{R: 0, G: 128, B: 0, A: 255}  // Dark green
	tvs.color["__DEFENCE_LOW__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}     // Lime green
	tvs.color["__DEFENCE_MEDIUM__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}  // Yellow
	tvs.color["__DEFENCE_HIGH__"] = color.RGBA{R: 255, G: 165, B: 0, A: 255}    // Orange
	tvs.color["__DEFENCE_VERY_HIGH__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red

	// Treasury level colors (same as defence)
	tvs.color["__TREASURY_VERY_LOW__"] = color.RGBA{R: 0, G: 128, B: 0, A: 255}  // Dark green
	tvs.color["__TREASURY_LOW__"] = color.RGBA{R: 50, G: 255, B: 50, A: 255}     // Lime green
	tvs.color["__TREASURY_MEDIUM__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}  // Yellow
	tvs.color["__TREASURY_HIGH__"] = color.RGBA{R: 255, G: 96, B: 0, A: 255}     // Orange
	tvs.color["__TREASURY_VERY_HIGH__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red

	// Treasury override colors (similar to treasury but with different saturation)
	tvs.color["__TREASURY_OVERRIDE_NONE__"] = color.RGBA{R: 128, G: 128, B: 128, A: 255}   // Gray for no override
	tvs.color["__TREASURY_OVERRIDE_VERY_LOW__"] = color.RGBA{R: 34, G: 139, B: 34, A: 255} // Darker green
	tvs.color["__TREASURY_OVERRIDE_LOW__"] = color.RGBA{R: 50, G: 255, B: 50, A: 255}      // Forest green
	tvs.color["__TREASURY_OVERRIDE_MEDIUM__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}   // Goldenrod
	tvs.color["__TREASURY_OVERRIDE_HIGH__"] = color.RGBA{R: 255, G: 96, B: 0, A: 255}      // Dark orange
	tvs.color["__TREASURY_OVERRIDE_VERY_HIGH__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255}  // Crimson

	// Warning colors
	tvs.color["__WARNING_ACTIVE__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255} // Yellow for warnings
	tvs.color["__WARNING_NONE__"] = color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray for no warnings

	// Tax level colors - based on taxation burden from other guilds
	tvs.color["__TAX_NONE__"] = color.RGBA{R: 144, G: 238, B: 144, A: 255}    // Light green for no tax
	tvs.color["__TAX_VERY_LOW__"] = color.RGBA{R: 173, G: 255, B: 47, A: 255} // Green-yellow for <10% tax
	tvs.color["__TAX_LOW__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}       // Yellow for moderate tax
	tvs.color["__TAX_MEDIUM__"] = color.RGBA{R: 255, G: 215, B: 0, A: 255}    // Gold for higher tax
	tvs.color["__TAX_HIGH__"] = color.RGBA{R: 255, G: 165, B: 0, A: 255}      // Orange for high tax
	tvs.color["__TAX_VERY_HIGH__"] = color.RGBA{R: 255, G: 69, B: 0, A: 255}  // Orange-red for very high tax
	tvs.color["__TAX_EXTREME__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255}     // Red for extreme tax
	tvs.color["__TAX_CUT_OFF__"] = color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray for cut off territories

	// Production colors - these are base colors that will be mixed and scaled
	// Green for emerald production
	tvs.color["__PRODUCTION_EMERALD__"] = color.RGBA{R: 0, G: 255, B: 0, A: 255} // Pure green
	// Blue for resource production
	tvs.color["__PRODUCTION_RESOURCE__"] = color.RGBA{R: 0, G: 0, B: 255, A: 255} // Pure blue
	// Red for production errors (cannot afford upgrades)
	tvs.color["__PRODUCTION_ERROR__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Pure red
	// Default dark grey for no production
	tvs.color["__PRODUCTION_NONE__"] = color.RGBA{R: 64, G: 64, B: 64, A: 255} // Dark grey

	// Filter view default color (pink highlight)
	tvs.color["__VIEW_FILTER__"] = color.RGBA{R: 247, G: 119, B: 169, A: 255}
}

// SetFilteredTerritories updates the set of territories that should be highlighted in filter view.
func (tvs *TerritoryViewSwitcher) SetFilteredTerritories(matches map[string]float64, active bool) {
	tvs.filterActive = active
	if matches == nil {
		tvs.filteredTerritories = make(map[string]float64)
		return
	}
	if tvs.filteredTerritories == nil {
		tvs.filteredTerritories = make(map[string]float64, len(matches))
	} else {
		for k := range tvs.filteredTerritories {
			delete(tvs.filteredTerritories, k)
		}
	}
	for k, v := range matches {
		if v > 0 {
			tvs.filteredTerritories[k] = v
		}
	}
}

// Update handles input and modal visibility logic
func (tvs *TerritoryViewSwitcher) Update() {
	now := time.Now()
	mx, my := ebiten.CursorPosition()
	if mx != tvs.lastMouseX || my != tvs.lastMouseY {
		tvs.lastMouseMove = now
		tvs.lastMouseX = mx
		tvs.lastMouseY = my
	}

	tabPressed := ebiten.IsKeyPressed(ebiten.KeyTab)
	shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	// Track key states for edge detection
	prevTabPressed := tvs.keysPressed[ebiten.KeyTab]
	tvs.keysPressed[ebiten.KeyTab] = tabPressed
	tvs.keysPressed[ebiten.KeyShiftLeft] = shiftPressed
	tvs.keysPressed[ebiten.KeyShiftRight] = shiftPressed

	// Detect Tab key press (edge detection)
	if tabPressed && !prevTabPressed {
		tvs.lastKeyCheck = now

		if !tvs.modalVisible {
			tvs.modalVisible = true
			tvs.selectedIndex = int(tvs.currentView)
			tvs.highlightTarget = float64(tvs.selectedIndex)
			tvs.highlightPos = tvs.highlightTarget
			tvs.highlightInitialized = true
			tvs.highlightWrapDir = 0
		}

		// If mouse is over the list and moving, jump to hovered entry.
		// Otherwise, advance by Tab/Shift+Tab.
		mouseMovedRecently := now.Sub(tvs.lastMouseMove) < 150*time.Millisecond
		if tvs.modalVisible {
			screenW, screenH := ebiten.WindowSize()
			modalWidth := 400
			modalHeight := 60 + len(tvs.views)*40 + 20
			modalX := (screenW - modalWidth) / 2
			modalY := (screenH - modalHeight) / 2
			startY := modalY + 50
			itemHeight := 40
			itemX := modalX + 10
			itemW := modalWidth - 20
			itemH := itemHeight - 5
			listTop := startY
			listBottom := startY + len(tvs.views)*itemHeight - 5
			mouseInList := mx >= itemX && mx <= itemX+itemW && my >= listTop && my <= listBottom

			if mouseInList && mouseMovedRecently {
				index := int((my - startY) / itemHeight)
				if index < 0 {
					index = 0
				}
				if index >= len(tvs.views) {
					index = len(tvs.views) - 1
				}
				itemY := startY + index*itemHeight
				if my <= itemY+itemH {
					tvs.selectedIndex = index
					tvs.currentView = TerritoryViewType(tvs.selectedIndex)
					tvs.highlightWrapDir = 0
				} else {
					if shiftPressed {
						// Shift+Tab: go backwards
						prevIndex := tvs.selectedIndex
						tvs.selectedIndex--
						if tvs.selectedIndex < 0 {
							tvs.selectedIndex = len(tvs.views) - 1
							tvs.highlightWrapDir = -1
							tvs.highlightWrapTo = tvs.selectedIndex
							tvs.highlightTarget = -1
							_ = prevIndex
						}
					} else {
						// Tab: go forwards
						prevIndex := tvs.selectedIndex
						tvs.selectedIndex++
						if tvs.selectedIndex >= len(tvs.views) {
							tvs.selectedIndex = 0
							tvs.highlightWrapDir = 1
							tvs.highlightWrapTo = tvs.selectedIndex
							tvs.highlightTarget = float64(len(tvs.views))
							_ = prevIndex
						}
					}
				}
			} else {
				if shiftPressed {
					// Shift+Tab: go backwards
					prevIndex := tvs.selectedIndex
					tvs.selectedIndex--
					if tvs.selectedIndex < 0 {
						tvs.selectedIndex = len(tvs.views) - 1
						tvs.highlightWrapDir = -1
						tvs.highlightWrapTo = tvs.selectedIndex
						tvs.highlightTarget = -1
						_ = prevIndex
					}
				} else {
					// Tab: go forwards
					prevIndex := tvs.selectedIndex
					tvs.selectedIndex++
					if tvs.selectedIndex >= len(tvs.views) {
						tvs.selectedIndex = 0
						tvs.highlightWrapDir = 1
						tvs.highlightWrapTo = tvs.selectedIndex
						tvs.highlightTarget = float64(len(tvs.views))
						_ = prevIndex
					}
				}
			}
		} else {
			if shiftPressed {
				// Shift+Tab: go backwards
				prevIndex := tvs.selectedIndex
				tvs.selectedIndex--
				if tvs.selectedIndex < 0 {
					tvs.selectedIndex = len(tvs.views) - 1
					tvs.highlightWrapDir = -1
					tvs.highlightWrapTo = tvs.selectedIndex
					tvs.highlightTarget = -1
					_ = prevIndex
				}
			} else {
				// Tab: go forwards
				prevIndex := tvs.selectedIndex
				tvs.selectedIndex++
				if tvs.selectedIndex >= len(tvs.views) {
					tvs.selectedIndex = 0
					tvs.highlightWrapDir = 1
					tvs.highlightWrapTo = tvs.selectedIndex
					tvs.highlightTarget = float64(len(tvs.views))
					_ = prevIndex
				}
			}
		}

		// Apply the view change immediately
		if tvs.selectedIndex >= 0 && tvs.selectedIndex < len(tvs.views) {
			tvs.currentView = TerritoryViewType(tvs.selectedIndex)
			if tvs.highlightWrapDir == 0 {
				tvs.highlightTarget = float64(tvs.selectedIndex)
			}
		}
	}

	// Hide modal after delay if no Tab is pressed
	if tvs.modalVisible && !tabPressed && now.Sub(tvs.lastKeyCheck) > tvs.hideDelay {
		tvs.modalVisible = false
	}

	// While holding Tab, hovering an entry switches to that view (only while mouse is moving)
	if tabPressed && tvs.modalVisible {
		// reuse mx,my from movement tracking
		// Modal dimensions (match Draw)
		screenW, screenH := ebiten.WindowSize()
		modalWidth := 400
		modalHeight := 60 + len(tvs.views)*40 + 20
		modalX := (screenW - modalWidth) / 2
		modalY := (screenH - modalHeight) / 2
		startY := modalY + 50
		itemHeight := 40
		itemX := modalX + 10
		itemW := modalWidth - 20
		itemH := itemHeight - 5
		listTop := startY
		listBottom := startY + len(tvs.views)*itemHeight - 5

		mouseInList := mx >= itemX && mx <= itemX+itemW && my >= listTop && my <= listBottom
		mouseMovedRecently := now.Sub(tvs.lastMouseMove) < 150*time.Millisecond
		if mouseInList && mouseMovedRecently {
			index := int((my - startY) / itemHeight)
			if index < 0 {
				index = 0
			}
			if index >= len(tvs.views) {
				index = len(tvs.views) - 1
			}
			itemY := startY + index*itemHeight
			if my <= itemY+itemH {
				desiredIndex := (float64(my) - float64(startY)) / float64(itemHeight)
				if desiredIndex < 0 {
					desiredIndex = 0
				}
				maxIndex := float64(len(tvs.views) - 1)
				if desiredIndex > maxIndex {
					desiredIndex = maxIndex
				}

				nearest := index
				stickiness := 0.75
				tvs.highlightTarget = desiredIndex*(1.0-stickiness) + float64(nearest)*stickiness

				if tvs.selectedIndex != nearest {
					tvs.selectedIndex = nearest
					tvs.currentView = TerritoryViewType(tvs.selectedIndex)
					tvs.highlightWrapDir = 0
				}
			} else {
				if tvs.highlightWrapDir == 0 {
					tvs.highlightTarget = float64(tvs.selectedIndex)
				}
			}
		} else {
			if tvs.highlightWrapDir == 0 {
				tvs.highlightTarget = float64(tvs.selectedIndex)
			}
		}
	}

	if !tvs.highlightInitialized {
		tvs.highlightPos = float64(tvs.selectedIndex)
		tvs.highlightTarget = tvs.highlightPos
		tvs.highlightInitialized = true
		tvs.highlightLastUpdate = now
	}

	dt := now.Sub(tvs.highlightLastUpdate).Seconds()
	if dt < 0 {
		dt = 0
	}
	if dt > 0.1 {
		dt = 0.1
	}
	tvs.highlightLastUpdate = now

	moveSpeed := 50.0
	alpha := 1 - math.Exp(-moveSpeed*dt)
	ease := smoothstep(alpha)
	tvs.highlightPos = tvs.highlightPos + (tvs.highlightTarget-tvs.highlightPos)*ease

	if tvs.highlightWrapDir != 0 {
		maxIndex := float64(len(tvs.views) - 1)
		if tvs.highlightWrapDir < 0 && tvs.highlightPos <= -0.5 {
			tvs.highlightPos = maxIndex + 1
			tvs.highlightTarget = float64(tvs.highlightWrapTo)
			tvs.highlightWrapDir = 0
		} else if tvs.highlightWrapDir > 0 && tvs.highlightPos >= maxIndex+0.5 {
			tvs.highlightPos = -1
			tvs.highlightTarget = float64(tvs.highlightWrapTo)
			tvs.highlightWrapDir = 0
		}
	}
}

// GetCurrentView returns the current territory view type
func (tvs *TerritoryViewSwitcher) GetCurrentView() TerritoryViewType {
	return tvs.currentView
}

// IsModalVisible returns whether the switcher modal is currently visible
func (tvs *TerritoryViewSwitcher) IsModalVisible() bool {
	return tvs.modalVisible
}

// GetTerritoryColorForCurrentView returns the appropriate color for a territory based on the current view
func (tvs *TerritoryViewSwitcher) GetTerritoryColorForCurrentView(territoryName string) (color.RGBA, bool) {
	// Get territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return color.RGBA{}, false
	}

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	switch tvs.currentView {
	case ViewGuild:
		// Return no color - let the normal guild coloring system handle it
		return color.RGBA{}, false

	case ViewPluginsExtension:
		if col, ok := pluginhost.GetOverlayColor(territory.Name); ok {
			return col, true
		}
		return color.RGBA{}, false

	case ViewAnalysis:
		return tvs.getAnalysisColor(territory.Name)

	case ViewResource:
		return tvs.getResourceColor(territory), true

	case ViewSetDefence:
		return tvs.getDefenceColor(territory.SetLevel), true

	case ViewAtDefence:
		return tvs.getDefenceColor(territory.Level), true

	case ViewTreasury:
		return tvs.getTreasuryColor(territory.Treasury), true

	case ViewTreasuryOverrides:
		return tvs.getTreasuryOverrideColor(territory.TreasuryOverride), true

	case ViewWarning:
		return tvs.getWarningColor(territory.Warning), true

	case ViewTax:
		return tvs.getTaxColor(territory), true

	case ViewFilter:
		return tvs.getFilterColor(), true

	case ViewProduction:
		return tvs.getProductionColor(territory), true

	case ViewThroughput:
		// Build cache once per tick to avoid repeated transit scans per frame
		tvs.ensureThroughputCache()
		return tvs.getThroughputColor(territory), true

	default:
		return color.RGBA{}, false
	}
}

func (tvs *TerritoryViewSwitcher) getFilterColor() color.RGBA {
	if col, ok := tvs.color["__VIEW_FILTER__"]; ok {
		return col
	}
	return color.RGBA{R: 110, G: 110, B: 110, A: 255}
}

// GetTerritoryColorsForCurrentView returns colors for multiple territories at once (batched optimization)
func (tvs *TerritoryViewSwitcher) GetTerritoryColorsForCurrentView(territoryNames []string) map[string]color.RGBA {
	result := make(map[string]color.RGBA, len(territoryNames))

	// Guild view uses normal coloring; Analysis view is handled separately below
	if tvs.currentView == ViewGuild {
		return result
	}

	// Filter view uses a uniform neutral color for all territories
	if tvs.currentView == ViewFilter {
		col := tvs.getFilterColor()
		base := color.RGBA{R: 40, G: 40, B: 45, A: 255}
		if !tvs.filterActive {
			for _, name := range territoryNames {
				result[name] = base
			}
			return result
		}
		for _, name := range territoryNames {
			weight, ok := tvs.filteredTerritories[name]
			if !ok || weight <= 0 {
				result[name] = base
				continue
			}
			result[name] = blendColors(base, col, weight)
		}
		return result
	}

	// Analysis view: compute colors directly without scanning all territories
	if tvs.currentView == ViewAnalysis {
		for _, name := range territoryNames {
			if col, ok := tvs.getAnalysisColor(name); ok {
				result[name] = col
			}
		}
		return result
	}

	// Build throughput cache once if needed before the per-territory loop
	if tvs.currentView == ViewThroughput {
		tvs.ensureThroughputCache()
	}

	// Create a set of needed territory names for faster lookup
	neededNames := make(map[string]bool, len(territoryNames))
	for _, name := range territoryNames {
		neededNames[name] = true
	}

	// Get all territories and iterate once to find matches
	allTerritories := eruntime.GetAllTerritories()

	// Fast-path plugin overlays: use cached host map and filter to needed names.
	if tvs.currentView == ViewPluginsExtension {
		overlays := pluginhost.CopyOverlayCache()
		for name, col := range overlays {
			if neededNames[name] {
				result[name] = col
			}
		}
		return result
	}

	for _, territory := range allTerritories {
		if territory == nil || !neededNames[territory.Name] {
			continue
		}

		territory.Mu.RLock()

		var territoryColor color.RGBA
		var hasColor bool

		switch tvs.currentView {
		case ViewResource:
			territoryColor = tvs.getResourceColor(territory)
			hasColor = true
		case ViewSetDefence:
			territoryColor = tvs.getDefenceColor(territory.SetLevel)
			hasColor = true
		case ViewAtDefence:
			territoryColor = tvs.getDefenceColor(territory.Level)
			hasColor = true
		case ViewTreasury:
			territoryColor = tvs.getTreasuryColor(territory.Treasury)
			hasColor = true
		case ViewTreasuryOverrides:
			territoryColor = tvs.getTreasuryOverrideColor(territory.TreasuryOverride)
			hasColor = true
		case ViewWarning:
			territoryColor = tvs.getWarningColor(territory.Warning)
			hasColor = true
		case ViewTax:
			territoryColor = tvs.getTaxColor(territory)
			hasColor = true
		case ViewProduction:
			territoryColor = tvs.getProductionColor(territory)
			hasColor = true
		case ViewThroughput:
			territoryColor = tvs.getThroughputColor(territory)
			hasColor = true
		}

		territory.Mu.RUnlock()

		if hasColor {
			result[territory.Name] = territoryColor
		}
	}

	return result
}

// getResourceColor determines the color based on resource generation
func (tvs *TerritoryViewSwitcher) getResourceColor(territory *typedef.Territory) color.RGBA {
	resources := territory.ResourceGeneration.Base
	options := eruntime.GetRuntimeOptions()

	woodColor := options.ResourceColors.Wood.ToRGBA()
	cropColor := options.ResourceColors.Crop.ToRGBA()
	fishColor := options.ResourceColors.Fish.ToRGBA()
	oreColor := options.ResourceColors.Ore.ToRGBA()
	multiColor := options.ResourceColors.Multi.ToRGBA()

	doubleFactor := 0.85

	// Optionally highlight emerald-generating territories using the emerald palette.
	if options.ShowEmeraldGenerators && resources.Emeralds >= 18000 {
		emeraldColor := options.ResourceColors.Emerald.ToRGBA()
		// City emerald territories generate 18000/h; darken to indicate the higher tier.
		if resources.Emeralds >= 18000 {
			emeraldColor = scaledColor(emeraldColor, 0.85)
		}
		return emeraldColor
	}

	// Count non-zero resources (excluding emeralds)
	resourceCount := 0
	dominantResource := ""
	isDoubleProduction := false

	if resources.Wood > 0 {
		resourceCount++
		dominantResource = "wood"
		// Check if this is double production (7200 or higher)
		if resources.Wood >= 7200 {
			isDoubleProduction = true
		}
	}
	if resources.Crops > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "crops"
		}
		// Check if this is double production
		if resources.Crops >= 7200 {
			isDoubleProduction = true
		}
	}
	if resources.Fish > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "fish"
		}
		// Check if this is double production
		if resources.Fish >= 7200 {
			isDoubleProduction = true
		}
	}
	if resources.Ores > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "ore"
		}
		// Check if this is double production
		if resources.Ores >= 7200 {
			isDoubleProduction = true
		}
	}

	// If multiple resources or none, return appropriate color
	if resourceCount != 1 {
		if isDoubleProduction {
			return scaledColor(multiColor, doubleFactor)
		}
		return multiColor
	}

	// Return color based on the single resource type and production level
	switch dominantResource {
	case "wood":
		if isDoubleProduction {
			return scaledColor(woodColor, doubleFactor)
		}
		return woodColor
	case "crops":
		if isDoubleProduction {
			return scaledColor(cropColor, doubleFactor)
		}
		return cropColor
	case "fish":
		if isDoubleProduction {
			return scaledColor(fishColor, doubleFactor)
		}
		return fishColor
	case "ore":
		if isDoubleProduction {
			return scaledColor(oreColor, doubleFactor)
		}
		return oreColor
	default:
		if isDoubleProduction {
			return scaledColor(multiColor, doubleFactor)
		}
		return multiColor
	}
}

// getDefenceColor returns color based on defence level
func (tvs *TerritoryViewSwitcher) getDefenceColor(level typedef.DefenceLevel) color.RGBA {
	switch level {
	case typedef.DefenceLevelVeryLow:
		return tvs.color["__DEFENCE_VERY_LOW__"]
	case typedef.DefenceLevelLow:
		return tvs.color["__DEFENCE_LOW__"]
	case typedef.DefenceLevelMedium:
		return tvs.color["__DEFENCE_MEDIUM__"]
	case typedef.DefenceLevelHigh:
		return tvs.color["__DEFENCE_HIGH__"]
	case typedef.DefenceLevelVeryHigh:
		return tvs.color["__DEFENCE_VERY_HIGH__"]
	default:
		return tvs.color["__DEFENCE_VERY_LOW__"]
	}
}

// getTreasuryColor returns color based on treasury level
func (tvs *TerritoryViewSwitcher) getTreasuryColor(level typedef.TreasuryLevel) color.RGBA {
	switch level {
	case typedef.TreasuryLevelVeryLow:
		return tvs.color["__TREASURY_VERY_LOW__"]
	case typedef.TreasuryLevelLow:
		return tvs.color["__TREASURY_LOW__"]
	case typedef.TreasuryLevelMedium:
		return tvs.color["__TREASURY_MEDIUM__"]
	case typedef.TreasuryLevelHigh:
		return tvs.color["__TREASURY_HIGH__"]
	case typedef.TreasuryLevelVeryHigh:
		return tvs.color["__TREASURY_VERY_HIGH__"]
	default:
		return tvs.color["__TREASURY_VERY_LOW__"]
	}
}

// getTreasuryOverrideColor returns color based on treasury override level
func (tvs *TerritoryViewSwitcher) getTreasuryOverrideColor(level typedef.TreasuryOverride) color.RGBA {
	switch level {
	case typedef.TreasuryOverrideNone:
		return tvs.color["__TREASURY_OVERRIDE_NONE__"]
	case typedef.TreasuryOverrideVeryLow:
		return tvs.color["__TREASURY_OVERRIDE_VERY_LOW__"]
	case typedef.TreasuryOverrideLow:
		return tvs.color["__TREASURY_OVERRIDE_LOW__"]
	case typedef.TreasuryOverrideMedium:
		return tvs.color["__TREASURY_OVERRIDE_MEDIUM__"]
	case typedef.TreasuryOverrideHigh:
		return tvs.color["__TREASURY_OVERRIDE_HIGH__"]
	case typedef.TreasuryOverrideVeryHigh:
		return tvs.color["__TREASURY_OVERRIDE_VERY_HIGH__"]
	default:
		return tvs.color["__TREASURY_OVERRIDE_NONE__"]
	}
}

// getWarningColor returns color based on warning status
func (tvs *TerritoryViewSwitcher) getWarningColor(warning typedef.Warning) color.RGBA {
	if warning != 0 {
		return tvs.color["__WARNING_ACTIVE__"]
	}
	return tvs.color["__WARNING_NONE__"]
}

// getTaxColor returns color based on estimated tax burden on this territory
func (tvs *TerritoryViewSwitcher) getTaxColor(territory *typedef.Territory) color.RGBA {
	taxLevel := tvs.calculateTaxLevel(territory)
	return tvs.color[tvs.getTaxHiddenGuildFromLevel(taxLevel)]
}

// getTaxHiddenGuild returns the hidden guild name for tax view
func (tvs *TerritoryViewSwitcher) getTaxHiddenGuild(territory *typedef.Territory) string {
	taxLevel := tvs.calculateTaxLevel(territory)
	return tvs.getTaxHiddenGuildFromLevel(taxLevel)
}

// getTaxHiddenGuildFromLevel converts tax level to hidden guild name
func (tvs *TerritoryViewSwitcher) getTaxHiddenGuildFromLevel(level int) string {
	switch level {
	case 0:
		return "__TAX_NONE__"
	case 1:
		return "__TAX_VERY_LOW__"
	case 2:
		return "__TAX_LOW__"
	case 3:
		return "__TAX_MEDIUM__"
	case 4:
		return "__TAX_HIGH__"
	case 5:
		return "__TAX_VERY_HIGH__"
	case 6:
		return "__TAX_EXTREME__"
	case -1:
		return "__TAX_CUT_OFF__"
	default:
		return "__TAX_NONE__"
	}
}

// calculateTaxLevel estimates the tax burden on a territory based on multiple factors
// Returns: -1 for cut off (no route to HQ), 0 for none, 1-6 for increasing tax levels
func (tvs *TerritoryViewSwitcher) calculateTaxLevel(territory *typedef.Territory) int {
	// Check if territory has no trading routes (cut off from HQ)
	if len(territory.TradingRoutes) == 0 || territory.RouteTax < 0 {
		return -1 // Cut off - gray
	}

	// If it's an HQ territory, it doesn't get taxed by others
	if territory.HQ {
		return 0 // No tax
	}

	// Calculate tax based on route tax percentage
	routeTax := territory.RouteTax * 100 // Convert to percentage

	// Determine tax level based on route tax
	if routeTax <= 0 {
		return 0 // No tax - light green
	} else if routeTax < 15 {
		return 1 // Very low tax (<5%) - lime-yellow
	} else if routeTax < 35 {
		return 2 // Low tax (5-15%) - yellow
	} else if routeTax < 55 {
		return 3 // Medium tax (15-30%) - gold
	} else if routeTax < 75 {
		return 4 // High tax (30-50%) - orange
	} else if routeTax < 95 {
		return 5 // Very high tax (50-70%) - orange-red
	} else {
		return 6 // Extreme tax (70%+) - red
	}
}

// getProductionColor calculates the production color for a territory
func (tvs *TerritoryViewSwitcher) getProductionColor(territory *typedef.Territory) color.RGBA {
	// Get production data
	emeraldProduction := territory.Options.Bonus.At.EfficientEmerald + territory.Options.Bonus.At.EmeraldRate
	resourceProduction := territory.Options.Bonus.At.EfficientResource + territory.Options.Bonus.At.ResourceRate

	// Get max possible production (assuming max level is 6 for each)
	maxEmeraldProduction := 12  // EfficientEmerald(6) + EmeraldRate(6)
	maxResourceProduction := 12 // EfficientResource(6) + ResourceRate(6)

	// Check for production errors (cannot afford upgrades)
	emeraldError := (territory.Options.Bonus.Set.EfficientEmerald > territory.Options.Bonus.At.EfficientEmerald) ||
		(territory.Options.Bonus.Set.EmeraldRate > territory.Options.Bonus.At.EmeraldRate)
	resourceError := (territory.Options.Bonus.Set.EfficientResource > territory.Options.Bonus.At.EfficientResource) ||
		(territory.Options.Bonus.Set.ResourceRate > territory.Options.Bonus.At.ResourceRate)

	// If no production at all, return dark grey
	if emeraldProduction == 0 && resourceProduction == 0 {
		return tvs.color["__PRODUCTION_NONE__"]
	}

	// Calculate color intensity based on production levels (0.0 to 1.0)
	emeraldIntensity := float64(emeraldProduction) / float64(maxEmeraldProduction)
	resourceIntensity := float64(resourceProduction) / float64(maxResourceProduction)

	// Start with black (no color)
	r, g, b := 0.0, 0.0, 0.0

	// Use more vibrant base colors (start at 128 for minimum intensity, scale up to 255)
	minIntensity := 128.0
	maxIntensity := 255.0

	// Add emerald production (green) if no emerald error
	if emeraldProduction > 0 && !emeraldError {
		g = minIntensity + (emeraldIntensity * (maxIntensity - minIntensity))
	}

	// Add resource production (blue) if no resource error
	if resourceProduction > 0 && !resourceError {
		b = minIntensity + (resourceIntensity * (maxIntensity - minIntensity))
	}

	// Add red component for errors
	errorIntensity := 0.0
	if emeraldError && emeraldProduction > 0 {
		errorIntensity += emeraldIntensity
	}
	if resourceError && resourceProduction > 0 {
		errorIntensity += resourceIntensity
	}
	if errorIntensity > 0 {
		r = minIntensity + ((errorIntensity / 2) * (maxIntensity - minIntensity))
	}

	// When mixing colors, ensure they don't exceed 255 by normalizing
	totalComponents := 0
	if r > 0 {
		totalComponents++
	}
	if g > 0 {
		totalComponents++
	}
	if b > 0 {
		totalComponents++
	}

	// If we have multiple color components, normalize to prevent oversaturation
	if totalComponents > 1 {
		maxComponent := r
		if g > maxComponent {
			maxComponent = g
		}
		if b > maxComponent {
			maxComponent = b
		}

		// If the brightest component would cause oversaturation when mixed, scale down
		if maxComponent > 200 { // Leave room for mixing
			scale := 200.0 / maxComponent
			r *= scale
			g *= scale
			b *= scale
		}
	}

	// Final clamp to ensure we never exceed 255
	if r > 255 {
		r = 255
	}
	if g > 255 {
		g = 255
	}
	if b > 255 {
		b = 255
	}

	// If we have no color components (all errors but no actual production), return dark grey
	if r == 0 && g == 0 && b == 0 {
		return tvs.color["__PRODUCTION_NONE__"]
	}

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

// ensureThroughputCache builds a per-territory throughput table once per tick
// The cache only counts transits that belong to the same guild as the territory they are on.
func (tvs *TerritoryViewSwitcher) ensureThroughputCache() {
	currentTick := eruntime.Tick()
	actualTPS, _, _ := eruntime.GetTickProcessingPerformance()
	buildStart := time.Now()

	// Build at most once per interval: 120 ticks at <=120 TPS, else cap to once per real second
	const tickInterval = uint64(120)
	if actualTPS > 120 {
		if !tvs.throughputCacheWallTime.IsZero() && time.Since(tvs.throughputCacheWallTime) < time.Second {
			return
		}
	} else {
		if tvs.throughputCache != nil && tvs.throughputGuildPeak != nil && currentTick-tvs.throughputCacheTick < tickInterval {
			return
		}
	}

	// Reset caches without reallocating on every tick
	if tvs.throughputCache == nil {
		tvs.throughputCache = make(map[string]float64)
	} else {
		for k := range tvs.throughputCache {
			delete(tvs.throughputCache, k)
		}
	}

	if tvs.throughputGuildPeak == nil {
		tvs.throughputGuildPeak = make(map[string]float64)
	} else {
		for k := range tvs.throughputGuildPeak {
			delete(tvs.throughputGuildPeak, k)
		}
	}

	// Take state lock (via GetAllTerritories) before transit lock to avoid st.mu -> tm.mu inversion
	territories := eruntime.GetAllTerritories()
	territorySnapshotDuration := time.Since(buildStart)
	throughputTotals := eruntime.GetInGuildTransitTotals()
	for _, territory := range territories {
		if territory == nil {
			continue
		}

		territory.Mu.RLock()
		territoryName := territory.Name
		guildTag := territory.Guild.Tag
		territory.Mu.RUnlock()

		totalValue := throughputTotals[territoryName]
		tvs.throughputCache[territoryName] = totalValue

		if guildTag != "" && totalValue > tvs.throughputGuildPeak[guildTag] {
			tvs.throughputGuildPeak[guildTag] = totalValue
		}
	}

	tvs.throughputCacheTick = currentTick
	tvs.throughputCacheWallTime = time.Now()

	_ = territorySnapshotDuration
}

// getThroughputBuildInterval returns how many ticks to wait between cache rebuilds.
// We rebuild every 120 ticks when TPS <= 120. At higher TPS, the caller caps rebuilds to 1s wall-clock.
func (tvs *TerritoryViewSwitcher) getThroughputBuildInterval() uint64 {
	return 120
}

// getThroughputColor returns a red-weighted gradient based on how much in-guild transit flows through a territory
func (tvs *TerritoryViewSwitcher) getThroughputColor(territory *typedef.Territory) color.RGBA {
	if territory == nil {
		return color.RGBA{}
	}

	territory.Mu.RLock()
	territoryName := territory.Name
	guildTag := territory.Guild.Tag
	territory.Mu.RUnlock()

	// Neutral color for territories without a guild
	if guildTag == "" {
		return color.RGBA{R: 80, G: 80, B: 80, A: 255}
	}

	throughput := tvs.throughputCache[territoryName]
	peak := tvs.throughputGuildPeak[guildTag]
	if peak <= 0 {
		return color.RGBA{R: 60, G: 60, B: 60, A: 255}
	}

	ratio := throughput / peak
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	// Apply adjustable curve: gamma <1 brightens faster, >1 brightens slower
	curve := eruntime.GetRuntimeOptions().ThroughputCurve
	gamma := math.Pow(2, -curve) // curve>0 => gamma<1 (faster brighten); curve<0 => gamma>1 (slower)
	if gamma <= 0 {
		gamma = 1
	}
	ratio = math.Pow(ratio, gamma)

	// Gradient from dark steel-blue to vivid hot red for stronger contrast
	baseR, baseG, baseB := 50.0, 70.0, 110.0
	peakR, peakG, peakB := 255.0, 40.0, 30.0

	r := uint8(math.Round(baseR + (peakR-baseR)*ratio))
	g := uint8(math.Round(baseG + (peakG-baseG)*ratio))
	b := uint8(math.Round(baseB + (peakB-baseB)*ratio))

	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// Draw renders the modal switcher UI
func (tvs *TerritoryViewSwitcher) Draw(screen *ebiten.Image) {
	if !tvs.modalVisible {
		return
	}

	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Modal dimensions
	modalWidth := 400
	modalHeight := 60 + len(tvs.views)*40 + 20 // Header + items + padding
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	// Draw modal background
	vector.DrawFilledRect(screen, float32(modalX), float32(modalY), float32(modalWidth), float32(modalHeight), color.RGBA{R: 40, G: 40, B: 40, A: 240}, false)

	// Draw modal border
	vector.StrokeRect(screen, float32(modalX), float32(modalY), float32(modalWidth), float32(modalHeight), 2, color.RGBA{R: 100, G: 100, B: 100, A: 255}, false)

	// Draw title
	font := loadWynncraftFont(18)
	titleText := "Territory View"
	titleBounds := text.BoundString(font, titleText)
	titleX := modalX + (modalWidth-titleBounds.Dx())/2
	titleY := modalY + 25
	text.Draw(screen, titleText, font, titleX, titleY, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	// Draw view options
	startY := modalY + 50
	itemHeight := 40
	itemX := modalX + 10
	itemW := modalWidth - 20
	itemH := itemHeight - 5
	listHeight := len(tvs.views)*itemHeight - 5

	// Animated highlight
	highlightPos := tvs.highlightPos
	if !tvs.highlightInitialized {
		highlightPos = float64(tvs.selectedIndex)
	}
	highlightY := float64(startY) + highlightPos*float64(itemHeight)
	clipTop := float64(startY)
	clipBottom := float64(startY + listHeight)
	drawTop := math.Max(highlightY, clipTop)
	drawBottom := math.Min(highlightY+float64(itemH), clipBottom)
	if drawBottom > drawTop {
		drawY := float32(drawTop)
		drawH := float32(drawBottom - drawTop)
		vector.DrawFilledRect(screen, float32(itemX), drawY, float32(itemW), drawH, color.RGBA{R: 70, G: 130, B: 180, A: 200}, false)
	}

	for i, view := range tvs.views {
		itemY := startY + i*itemHeight
		itemRect := [4]int{itemX, itemY, itemW, itemH}

		// Draw view name
		viewFont := loadWynncraftFont(16)
		viewText := view.Name
		viewX := itemRect[0] + 10
		viewY := itemRect[1] + 20
		text.Draw(screen, viewText, viewFont, viewX, viewY, color.RGBA{R: 255, G: 255, B: 255, A: 255})

		// Draw description
		descFont := loadWynncraftFont(12)
		descText := view.Description
		descX := itemRect[0] + 10
		descY := itemRect[1] + 32
		text.Draw(screen, descText, descFont, descX, descY, color.RGBA{R: 180, G: 180, B: 180, A: 255})
	}

	// Draw instructions at the bottom
	instrFont := loadWynncraftFont(12)
	instrText := "Tab to cycle views, shift to go backwards"
	instrBounds := text.BoundString(instrFont, instrText)
	instrX := modalX + (modalWidth-instrBounds.Dx())/2
	instrY := modalY + modalHeight - 15
	text.Draw(screen, instrText, instrFont, instrX, instrY, color.RGBA{R: 150, G: 150, B: 150, A: 255})
}

// GetHiddenGuildNameForTerritory returns the appropriate "hidden guild" name for a territory
// This is used by the guild coloring system to apply the correct color
func (tvs *TerritoryViewSwitcher) GetHiddenGuildNameForTerritory(territoryName string) string {
	if tvs.currentView == ViewGuild {
		return "" // Use normal guild coloring
	}

	// Get territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return ""
	}

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	switch tvs.currentView {
	case ViewResource:
		return tvs.getResourceHiddenGuild(territory)
	case ViewSetDefence:
		return tvs.getDefenceHiddenGuild(territory.SetLevel)
	case ViewAtDefence:
		return tvs.getDefenceHiddenGuild(territory.Level)
	case ViewTreasury:
		return tvs.getTreasuryHiddenGuild(territory.Treasury)
	case ViewTreasuryOverrides:
		return tvs.getTreasuryOverrideHiddenGuild(territory.TreasuryOverride)
	case ViewWarning:
		return tvs.getWarningHiddenGuild(territory.Warning)
	case ViewTax:
		return tvs.getTaxHiddenGuild(territory)
	case ViewFilter:
		return ""
	case ViewAnalysis:
		return ""
	default:
		return ""
	}
}

// getResourceHiddenGuild returns the hidden guild name for resource view
func (tvs *TerritoryViewSwitcher) getResourceHiddenGuild(territory *typedef.Territory) string {
	resources := territory.ResourceGeneration.Base
	opts := eruntime.GetRuntimeOptions()

	// Emerald generators can be optionally highlighted as their own color.
	if opts.ShowEmeraldGenerators && resources.Emeralds > 0 {
		if resources.Emeralds >= 18000 {
			return "__RESOURCE_EMERALD_DOUBLE__"
		}
		return "__RESOURCE_EMERALD__"
	}

	// Count non-zero resources (excluding emeralds)
	resourceCount := 0
	dominantResource := ""

	if resources.Wood > 0 {
		resourceCount++
		dominantResource = "wood"
	}
	if resources.Crops > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "crops"
		}
	}
	if resources.Fish > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "fish"
		}
	}
	if resources.Ores > 0 {
		resourceCount++
		if dominantResource == "" {
			dominantResource = "ore"
		}
	}

	// If multiple resources or none, return multi-resource guild
	if resourceCount != 1 {
		return "__RESOURCE_MULTI__"
	}

	// Return guild name based on the single resource type
	switch dominantResource {
	case "wood":
		return "__RESOURCE_WOOD__"
	case "crops":
		return "__RESOURCE_CROP__"
	case "fish":
		return "__RESOURCE_FISH__"
	case "ore":
		return "__RESOURCE_ORE__"
	default:
		return "__RESOURCE_MULTI__"
	}
}

// getDefenceHiddenGuild returns the hidden guild name for defence view
func (tvs *TerritoryViewSwitcher) getDefenceHiddenGuild(level typedef.DefenceLevel) string {
	switch level {
	case typedef.DefenceLevelVeryLow:
		return "__DEFENCE_VERY_LOW__"
	case typedef.DefenceLevelLow:
		return "__DEFENCE_LOW__"
	case typedef.DefenceLevelMedium:
		return "__DEFENCE_MEDIUM__"
	case typedef.DefenceLevelHigh:
		return "__DEFENCE_HIGH__"
	case typedef.DefenceLevelVeryHigh:
		return "__DEFENCE_VERY_HIGH__"
	default:
		return "__DEFENCE_VERY_LOW__"
	}
}

// getTreasuryHiddenGuild returns the hidden guild name for treasury view
func (tvs *TerritoryViewSwitcher) getTreasuryHiddenGuild(level typedef.TreasuryLevel) string {
	switch level {
	case typedef.TreasuryLevelVeryLow:
		return "__TREASURY_VERY_LOW__"
	case typedef.TreasuryLevelLow:
		return "__TREASURY_LOW__"
	case typedef.TreasuryLevelMedium:
		return "__TREASURY_MEDIUM__"
	case typedef.TreasuryLevelHigh:
		return "__TREASURY_HIGH__"
	case typedef.TreasuryLevelVeryHigh:
		return "__TREASURY_VERY_HIGH__"
	default:
		return "__TREASURY_VERY_LOW__"
	}
}

// getTreasuryOverrideHiddenGuild returns the hidden guild name for treasury override view
func (tvs *TerritoryViewSwitcher) getTreasuryOverrideHiddenGuild(level typedef.TreasuryOverride) string {
	switch level {
	case typedef.TreasuryOverrideNone:
		return "__TREASURY_OVERRIDE_NONE__"
	case typedef.TreasuryOverrideVeryLow:
		return "__TREASURY_OVERRIDE_VERY_LOW__"
	case typedef.TreasuryOverrideLow:
		return "__TREASURY_OVERRIDE_LOW__"
	case typedef.TreasuryOverrideMedium:
		return "__TREASURY_OVERRIDE_MEDIUM__"
	case typedef.TreasuryOverrideHigh:
		return "__TREASURY_OVERRIDE_HIGH__"
	case typedef.TreasuryOverrideVeryHigh:
		return "__TREASURY_OVERRIDE_VERY_HIGH__"
	default:
		return "__TREASURY_OVERRIDE_NONE__"
	}
}

// getWarningHiddenGuild returns the hidden guild name for warning view
func (tvs *TerritoryViewSwitcher) getWarningHiddenGuild(warning typedef.Warning) string {
	if warning != 0 {
		return "__WARNING_ACTIVE__"
	}
	return "__WARNING_NONE__"
}

// GetHiddenGuildColor returns the color for a hidden guild name
func (tvs *TerritoryViewSwitcher) GetHiddenGuildColor(hiddenGuildName string) (color.RGBA, bool) {
	col, exists := tvs.color[hiddenGuildName]
	return col, exists
}

// SetAnalysisResults stores the latest chokepoint scores (normalized 0..1) for visualization.
// scores should be keyed by territory name.
func (tvs *TerritoryViewSwitcher) SetAnalysisResults(guildTag string, scores map[string]float64) {
	if scores == nil {
		tvs.analysisScoresCardinal = nil
		tvs.analysisScoresOrdinal = nil
		tvs.analysisGuild = ""
		return
	}

	// Normalize by max importance to keep values within [0,1].
	maxVal := 0.0
	for _, v := range scores {
		if v > maxVal {
			maxVal = v
		}
	}

	cardinal := make(map[string]float64, len(scores))
	for name, v := range scores {
		if maxVal > 0 {
			cardinal[name] = v / maxVal
		} else {
			cardinal[name] = 0
		}
	}

	// Build ordinal percentiles for optional ordinal mode coloring.
	ordinal := make(map[string]float64, len(scores))
	if len(scores) > 1 {
		type pair struct {
			name  string
			value float64
		}
		list := make([]pair, 0, len(scores))
		for n, v := range cardinal {
			list = append(list, pair{name: n, value: v})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].value < list[j].value
		})

		denom := float64(len(list) - 1)
		if denom < 1 {
			denom = 1
		}
		for idx, item := range list {
			ordinal[item.name] = float64(idx) / denom
		}
	} else {
		for n := range scores {
			ordinal[n] = cardinal[n]
		}
	}

	tvs.analysisScoresCardinal = cardinal
	tvs.analysisScoresOrdinal = ordinal
	tvs.analysisGuild = guildTag
}

// getAnalysisColor maps importance to a yellow-scale heatmap. Territories without scores are grey.
func (tvs *TerritoryViewSwitcher) getAnalysisColor(territoryName string) (color.RGBA, bool) {
	if tvs.analysisScoresCardinal == nil {
		return color.RGBA{}, false
	}

	value := tvs.getAnalysisValue(territoryName)
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	// Apply user-configured curve
	value = tvs.applyChokepointCurve(value)

	// Base grey to bright yellow ramp
	base := color.RGBA{R: 70, G: 70, B: 70, A: 255}
	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}

	mix := func(a, b uint8, t float64) uint8 {
		return uint8(math.Round(float64(a)*(1-t) + float64(b)*t))
	}

	col := color.RGBA{
		R: mix(base.R, yellow.R, value),
		G: mix(base.G, yellow.G, value),
		B: mix(base.B, yellow.B, value),
		A: 255,
	}

	return col, true
}

// getAnalysisValue chooses cardinal or ordinal score based on runtime settings.
func (tvs *TerritoryViewSwitcher) getAnalysisValue(territoryName string) float64 {
	opts := eruntime.GetRuntimeOptions()
	useOrdinal := strings.EqualFold(opts.ChokepointMode, "Ordinal")

	if useOrdinal {
		if v, ok := tvs.analysisScoresOrdinal[territoryName]; ok {
			return v
		}
	} else {
		if v, ok := tvs.analysisScoresCardinal[territoryName]; ok {
			return v
		}
	}
	return 0
}

// applyChokepointCurve applies a gamma/log-style curve where 0 = linear, >0 brightens faster, <0 darkens.
func (tvs *TerritoryViewSwitcher) applyChokepointCurve(val float64) float64 {
	if val <= 0 {
		return 0
	}
	if val >= 1 {
		return 1
	}
	curve := eruntime.GetRuntimeOptions().ChokepointCurve
	if curve == 0 {
		return val
	}
	if curve > 0 {
		exponent := 1.0 / (1.0 + curve) // <1 -> log-ish brightening
		return math.Pow(val, exponent)
	}
	// curve < 0: inverse log (darken). Increase exponent >1
	exponent := 1.0 - curve
	if exponent < 0.1 {
		exponent = 0.1
	}
	if exponent > 10 {
		exponent = 10
	}
	return math.Pow(val, exponent)
}
