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
	ViewSetDefence
	ViewAtDefence
	ViewTreasury
	ViewWarning
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
	hiddenGuilds  map[string]color.RGBA // Map of hidden guild names to their colors
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
			{Name: "Set Defence", Description: "Set defence level view", HiddenGuild: "__VIEW_SET_DEFENCE__"},
			{Name: "At Defence", Description: "At defence level view", HiddenGuild: "__VIEW_AT_DEFENCE__"},
			{Name: "Treasury", Description: "Treasury level view", HiddenGuild: "__VIEW_TREASURY__"},
			{Name: "Warning", Description: "Warning status view", HiddenGuild: "__VIEW_WARNING__"},
		},
		hiddenGuilds: make(map[string]color.RGBA),
	}

	// Initialize hidden guild colors
	tvs.initializeHiddenGuilds()

	return tvs
}

// initializeHiddenGuilds sets up the color schemes for each view type
func (tvs *TerritoryViewSwitcher) initializeHiddenGuilds() {
	// Resource colors
	tvs.hiddenGuilds["__RESOURCE_WOOD__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}    // Lime green
	tvs.hiddenGuilds["__RESOURCE_CROP__"] = color.RGBA{R: 255, G: 215, B: 0, A: 255}    // Gold/yellow
	tvs.hiddenGuilds["__RESOURCE_FISH__"] = color.RGBA{R: 30, G: 144, B: 255, A: 255}   // Dodger blue (more blue)
	tvs.hiddenGuilds["__RESOURCE_ORE__"] = color.RGBA{R: 255, G: 127, B: 193, A: 255}   // Light pink (less pink, more white)
	tvs.hiddenGuilds["__RESOURCE_MULTI__"] = color.RGBA{R: 255, G: 255, B: 255, A: 255} // White for multiple resources

	// Defence level colors (very low to very high)
	tvs.hiddenGuilds["__DEFENCE_VERY_LOW__"] = color.RGBA{R: 0, G: 128, B: 0, A: 255}  // Dark green
	tvs.hiddenGuilds["__DEFENCE_LOW__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}     // Lime green
	tvs.hiddenGuilds["__DEFENCE_MEDIUM__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}  // Yellow
	tvs.hiddenGuilds["__DEFENCE_HIGH__"] = color.RGBA{R: 255, G: 165, B: 0, A: 255}    // Orange
	tvs.hiddenGuilds["__DEFENCE_VERY_HIGH__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red

	// Treasury level colors (same as defence)
	tvs.hiddenGuilds["__TREASURY_VERY_LOW__"] = color.RGBA{R: 0, G: 128, B: 0, A: 255}  // Dark green
	tvs.hiddenGuilds["__TREASURY_LOW__"] = color.RGBA{R: 50, G: 205, B: 50, A: 255}     // Lime green
	tvs.hiddenGuilds["__TREASURY_MEDIUM__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255}  // Yellow
	tvs.hiddenGuilds["__TREASURY_HIGH__"] = color.RGBA{R: 255, G: 165, B: 0, A: 255}    // Orange
	tvs.hiddenGuilds["__TREASURY_VERY_HIGH__"] = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Red

	// Warning colors
	tvs.hiddenGuilds["__WARNING_ACTIVE__"] = color.RGBA{R: 255, G: 255, B: 0, A: 255} // Yellow for warnings
	tvs.hiddenGuilds["__WARNING_NONE__"] = color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray for no warnings
}

// Update handles input and modal visibility logic
func (tvs *TerritoryViewSwitcher) Update() {
	now := time.Now()

	// Check for Ctrl+Tab or Ctrl+Shift+Tab
	ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	tabPressed := ebiten.IsKeyPressed(ebiten.KeyTab)
	shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	// Track key states for edge detection
	prevTabPressed := tvs.keysPressed[ebiten.KeyTab]

	tvs.keysPressed[ebiten.KeyControlLeft] = ebiten.IsKeyPressed(ebiten.KeyControlLeft)
	tvs.keysPressed[ebiten.KeyControlRight] = ebiten.IsKeyPressed(ebiten.KeyControlRight)
	tvs.keysPressed[ebiten.KeyTab] = tabPressed
	tvs.keysPressed[ebiten.KeyShiftLeft] = shiftPressed

	// Detect Tab key press while Ctrl is held
	if ctrlPressed && tabPressed && !prevTabPressed {
		// Tab was just pressed while Ctrl is held
		tvs.lastKeyCheck = now

		if !tvs.modalVisible {
			// First time pressing Ctrl+Tab, show modal
			tvs.modalVisible = true
			tvs.selectedIndex = int(tvs.currentView)
		}

		// Switch view
		if shiftPressed {
			// Ctrl+Shift+Tab: go backwards
			tvs.selectedIndex--
			if tvs.selectedIndex < 0 {
				tvs.selectedIndex = len(tvs.views) - 1
			}
		} else {
			// Ctrl+Tab: go forwards
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

	// Hide modal when Ctrl is released
	if tvs.modalVisible && !ctrlPressed {
		tvs.modalVisible = false
		// View has already been applied above, no need to apply again
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

	case ViewWarning:
		return tvs.getWarningColor(territory.Warning), true

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
		case ViewWarning:
			territoryColor = tvs.getWarningColor(territory.Warning)
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

	// If multiple resources or none, return white
	if resourceCount != 1 {
		return tvs.hiddenGuilds["__RESOURCE_MULTI__"]
	}

	// Return color based on the single resource type
	switch dominantResource {
	case "wood":
		return tvs.hiddenGuilds["__RESOURCE_WOOD__"]
	case "crops":
		return tvs.hiddenGuilds["__RESOURCE_CROP__"]
	case "fish":
		return tvs.hiddenGuilds["__RESOURCE_FISH__"]
	case "ore":
		return tvs.hiddenGuilds["__RESOURCE_ORE__"]
	default:
		return tvs.hiddenGuilds["__RESOURCE_MULTI__"]
	}
}

// getDefenceColor returns color based on defence level
func (tvs *TerritoryViewSwitcher) getDefenceColor(level typedef.DefenceLevel) color.RGBA {
	switch level {
	case typedef.DefenceLevelVeryLow:
		return tvs.hiddenGuilds["__DEFENCE_VERY_LOW__"]
	case typedef.DefenceLevelLow:
		return tvs.hiddenGuilds["__DEFENCE_LOW__"]
	case typedef.DefenceLevelMedium:
		return tvs.hiddenGuilds["__DEFENCE_MEDIUM__"]
	case typedef.DefenceLevelHigh:
		return tvs.hiddenGuilds["__DEFENCE_HIGH__"]
	case typedef.DefenceLevelVeryHigh:
		return tvs.hiddenGuilds["__DEFENCE_VERY_HIGH__"]
	default:
		return tvs.hiddenGuilds["__DEFENCE_VERY_LOW__"]
	}
}

// getTreasuryColor returns color based on treasury level
func (tvs *TerritoryViewSwitcher) getTreasuryColor(level typedef.TreasuryLevel) color.RGBA {
	switch level {
	case typedef.TreasuryLevelVeryLow:
		return tvs.hiddenGuilds["__TREASURY_VERY_LOW__"]
	case typedef.TreasuryLevelLow:
		return tvs.hiddenGuilds["__TREASURY_LOW__"]
	case typedef.TreasuryLevelMedium:
		return tvs.hiddenGuilds["__TREASURY_MEDIUM__"]
	case typedef.TreasuryLevelHigh:
		return tvs.hiddenGuilds["__TREASURY_HIGH__"]
	case typedef.TreasuryLevelVeryHigh:
		return tvs.hiddenGuilds["__TREASURY_VERY_HIGH__"]
	default:
		return tvs.hiddenGuilds["__TREASURY_VERY_LOW__"]
	}
}

// getWarningColor returns color based on warning status
func (tvs *TerritoryViewSwitcher) getWarningColor(warning typedef.Warning) color.RGBA {
	if warning != 0 {
		return tvs.hiddenGuilds["__WARNING_ACTIVE__"]
	}
	return tvs.hiddenGuilds["__WARNING_NONE__"]
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
	instrText := "Hold Ctrl + Tab to cycle views, shift to go backwards"
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
	case ViewWarning:
		return tvs.getWarningHiddenGuild(territory.Warning)
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

// getWarningHiddenGuild returns the hidden guild name for warning view
func (tvs *TerritoryViewSwitcher) getWarningHiddenGuild(warning typedef.Warning) string {
	if warning != 0 {
		return "__WARNING_ACTIVE__"
	}
	return "__WARNING_NONE__"
}

// GetHiddenGuildColor returns the color for a hidden guild name
func (tvs *TerritoryViewSwitcher) GetHiddenGuildColor(hiddenGuildName string) (color.RGBA, bool) {
	col, exists := tvs.hiddenGuilds[hiddenGuildName]
	return col, exists
}
