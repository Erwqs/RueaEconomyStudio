package app

import (
	"image/color"
	"time"

	"etools/eruntime"
	"etools/typedef"

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
			{Name: "Production", Description: "Production efficiency view", HiddenGuild: "__VIEW_PRODUCTION__"},
			{Name: "Set Defence", Description: "Set defence level view", HiddenGuild: "__VIEW_SET_DEFENCE__"},
			{Name: "At Defence", Description: "At defence level view", HiddenGuild: "__VIEW_AT_DEFENCE__"},
			{Name: "Treasury", Description: "Treasury level view", HiddenGuild: "__VIEW_TREASURY__"},
			{Name: "Treasury Overrides", Description: "Treasury override view", HiddenGuild: "__VIEW_TREASURY_OVERRIDES__"},
			{Name: "Warning", Description: "Warning status view", HiddenGuild: "__VIEW_WARNING__"},
			{Name: "Tax", Description: "Territory taxation level", HiddenGuild: "__VIEW_TAX__"},
		},
		color: make(map[string]color.RGBA),
	}

	// Initialize hidden guild colors
	tvs.initializeHiddenGuilds()

	return tvs
}

// initializeHiddenGuilds sets up the color schemes for each view type
func (tvs *TerritoryViewSwitcher) initializeHiddenGuilds() {
	// Resource colors (normal production - 3600 base)
	tvs.color["__RESOURCE_WOOD__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}    // Lime green
	tvs.color["__RESOURCE_CROP__"] = color.RGBA{R: 255, G: 215, B: 0, A: 255}    // Gold/yellow
	tvs.color["__RESOURCE_FISH__"] = color.RGBA{R: 30, G: 144, B: 255, A: 255}   // Dodger blue (more blue)
	tvs.color["__RESOURCE_ORE__"] = color.RGBA{R: 255, G: 127, B: 193, A: 255}   // Light pink (less pink, more white)
	tvs.color["__RESOURCE_MULTI__"] = color.RGBA{R: 255, G: 255, B: 255, A: 255} // White for multiple resources

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
}

// Update handles input and modal visibility logic
func (tvs *TerritoryViewSwitcher) Update() {
	now := time.Now()

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
		}

		if shiftPressed {
			// Shift+Tab: go backwards
			tvs.selectedIndex--
			if tvs.selectedIndex < 0 {
				tvs.selectedIndex = len(tvs.views) - 1
			}
		} else {
			// Tab: go forwards
			tvs.selectedIndex++
			if tvs.selectedIndex >= len(tvs.views) {
				tvs.selectedIndex = 0
			}
		}

		// Apply the view change immediately
		if tvs.selectedIndex >= 0 && tvs.selectedIndex < len(tvs.views) {
			tvs.currentView = TerritoryViewType(tvs.selectedIndex)
		}
	}

	// Hide modal after delay if no Tab is pressed
	if tvs.modalVisible && !tabPressed && now.Sub(tvs.lastKeyCheck) > tvs.hideDelay {
		tvs.modalVisible = false
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

	case ViewProduction:
		return tvs.getProductionColor(territory), true

	default:
		return color.RGBA{}, false
	}
}

// GetTerritoryColorsForCurrentView returns colors for multiple territories at once (batched optimization)
func (tvs *TerritoryViewSwitcher) GetTerritoryColorsForCurrentView(territoryNames []string) map[string]color.RGBA {
	result := make(map[string]color.RGBA, len(territoryNames))

	// If current view is guild view, return empty map (let normal guild coloring handle it)
	if tvs.currentView == ViewGuild {
		return result
	}

	// Create a set of needed territory names for faster lookup
	neededNames := make(map[string]bool, len(territoryNames))
	for _, name := range territoryNames {
		neededNames[name] = true
	}

	// Get all territories and iterate once to find matches
	allTerritories := eruntime.GetAllTerritories()

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
			return tvs.color["__RESOURCE_MULTI_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_MULTI__"]
	}

	// Return color based on the single resource type and production level
	switch dominantResource {
	case "wood":
		if isDoubleProduction {
			return tvs.color["__RESOURCE_WOOD_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_WOOD__"]
	case "crops":
		if isDoubleProduction {
			return tvs.color["__RESOURCE_CROP_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_CROP__"]
	case "fish":
		if isDoubleProduction {
			return tvs.color["__RESOURCE_FISH_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_FISH__"]
	case "ore":
		if isDoubleProduction {
			return tvs.color["__RESOURCE_ORE_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_ORE__"]
	default:
		if isDoubleProduction {
			return tvs.color["__RESOURCE_MULTI_DOUBLE__"]
		}
		return tvs.color["__RESOURCE_MULTI__"]
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

	for i, view := range tvs.views {
		itemY := startY + i*itemHeight
		itemRect := [4]int{modalX + 10, itemY, modalWidth - 20, itemHeight - 5}

		// Highlight selected item
		if i == tvs.selectedIndex {
			vector.DrawFilledRect(screen, float32(itemRect[0]), float32(itemRect[1]), float32(itemRect[2]), float32(itemRect[3]), color.RGBA{R: 70, G: 130, B: 180, A: 200}, false)
		}

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
	default:
		return ""
	}
}

// getResourceHiddenGuild returns the hidden guild name for resource view
func (tvs *TerritoryViewSwitcher) getResourceHiddenGuild(territory *typedef.Territory) string {
	resources := territory.ResourceGeneration.Base

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
