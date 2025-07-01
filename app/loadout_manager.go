package app

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"etools/eruntime"
	"etools/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
)

// LoadoutData represents a territory loadout configuration
type LoadoutData struct {
	Name                     string `json:"name"`
	typedef.TerritoryOptions `json:"options"`
}

// LoadoutImportExport represents the file format for import/export
type LoadoutImportExport struct {
	Type     string        `json:"type"`    // "loadouts"
	Version  string        `json:"version"` // "1.0"
	Loadouts []LoadoutData `json:"loadouts"`
}

// LoadoutManager manages territory loadouts similar to guild management
type LoadoutManager struct {
	visible             bool
	loadouts            []LoadoutData
	selectedIndex       int
	nameInput           *EnhancedTextInput
	scrollOffset        int
	editingIndex        int // -1 when not editing, index when editing
	showColorPicker     bool
	editSideMenuVisible bool
	editSideMenu        *EdgeMenu
	editingLoadout      *LoadoutData // Copy of loadout being edited

	// Loadout application mode
	isApplyingLoadout    bool
	applyingLoadoutIndex int             // Index of loadout being applied
	applyingLoadoutName  string          // Name of loadout being applied
	selectedTerritories  map[string]bool // Selected territories for loadout application
	applyUIVisible       bool            // Whether apply mode UI is visible

	// UI state
	addButtonHovered    bool
	editButtonHovered   map[int]bool
	deleteButtonHovered map[int]bool
	applyButtonHovered  map[int]bool
	importButtonHovered bool
	exportButtonHovered bool
	closeButtonHovered  bool
}

// NewLoadoutManager creates a new loadout manager
func NewLoadoutManager() *LoadoutManager {
	lm := &LoadoutManager{
		visible:             false,
		loadouts:            make([]LoadoutData, 0),
		selectedIndex:       -1,
		editingIndex:        -1,
		showColorPicker:     false,
		editSideMenuVisible: false,
		editButtonHovered:   make(map[int]bool),
		deleteButtonHovered: make(map[int]bool),
		applyButtonHovered:  make(map[int]bool),
		isApplyingLoadout:   false,
		selectedTerritories: make(map[string]bool),
		applyUIVisible:      false,
	}

	// Initialize text input
	lm.nameInput = NewEnhancedTextInput("Enter loadout name", 120, 150, 200, 25, 50)

	// Load existing loadouts from file
	lm.loadFromFile()

	return lm
}

// IsVisible returns whether the loadout manager is currently visible or in application mode
func (lm *LoadoutManager) IsVisible() bool {
	return lm.visible || lm.isApplyingLoadout
}

// Show displays the loadout manager
func (lm *LoadoutManager) Show() {
	// Close guild manager if it's open (mutually exclusive)
	guildManager := GetEnhancedGuildManager()
	if guildManager != nil && guildManager.IsVisible() {
		guildManager.Hide()
	}

	lm.visible = true
}

// Hide hides the loadout manager
func (lm *LoadoutManager) Hide() {
	lm.visible = false
	lm.editSideMenuVisible = false
	lm.nameInput.Focused = false
}

// Toggle toggles the visibility of the loadout manager
func (lm *LoadoutManager) Toggle() {
	if lm.visible {
		lm.Hide()
	} else {
		lm.Show()
	}
}

// Update handles input and updates the loadout manager
func (lm *LoadoutManager) Update() bool {
	// Handle loadout application mode (even when main UI is not visible)
	if lm.isApplyingLoadout {
		// Handle escape key to cancel loadout application
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			fmt.Printf("[LOADOUT] ESC pressed in apply mode - canceling\n")
			lm.CancelLoadoutApplication()
			return true
		}

		// Handle MouseButton3 (back button) to cancel loadout application
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
			fmt.Printf("[LOADOUT] Back button pressed in apply mode - canceling\n")
			lm.CancelLoadoutApplication()
			return true
		}

		// Handle enter key to apply loadout to selected territories
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			fmt.Printf("[LOADOUT] Enter pressed in apply mode - applying\n")
			lm.StopLoadoutApplication()
			return true
		}

		// Handle clicks when in apply mode
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			fmt.Printf("[LOADOUT] Click in apply mode at (%d, %d)\n", mx, my)

			// First check if click is on UI buttons - this must come first!
			if lm.applyUIVisible && lm.handleApplyModeClick(mx, my) {
				fmt.Printf("[LOADOUT] Click handled by apply mode button\n")
				return true
			}

			// Then check if click is within the banner area to prevent clicking through
			if lm.applyUIVisible && my <= 140 { // Banner height is 140
				fmt.Printf("[LOADOUT] Click blocked by banner area\n")
				return true // Consume the click to prevent it from going to the map
			}
		}

		// Let the map handle area selection and territory clicking
		return false
	}

	if !lm.visible {
		return false
	}

	// Handle edit side menu updates
	if lm.editSideMenuVisible && lm.editSideMenu != nil {
		screenW, screenH := ebiten.WindowSize()
		if lm.editSideMenu.Update(screenW, screenH, 1.0/60.0) {
			// Side menu consumed input
			return true
		}
	}

	// Handle escape key to close
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if lm.editSideMenuVisible {
			lm.hideEditSideMenu()
		} else {
			lm.Hide()
		}
		return true
	}

	// Handle MouseButton3 (back button) to close, just like ESC
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		if lm.editSideMenuVisible {
			lm.hideEditSideMenu()
		} else {
			lm.Hide()
		}
		return true
	}

	// Handle scrolling (like guild manager)
	_, dy := ebiten.Wheel()
	if dy != 0 {
		// Calculate max visible items and scroll bounds like guild manager
		itemHeight := 60
		listHeight := 280 // Height of the list area (panelHeight-280 from main draw)
		maxVisibleItems := listHeight / itemHeight
		maxScrollOffset := len(lm.loadouts) - maxVisibleItems
		if maxScrollOffset < 0 {
			maxScrollOffset = 0
		}

		// Update scroll offset with proper bounds checking
		lm.scrollOffset -= int(dy * 3) // Use same scroll speed as guild manager
		if lm.scrollOffset < 0 {
			lm.scrollOffset = 0
		}
		if lm.scrollOffset > maxScrollOffset {
			lm.scrollOffset = maxScrollOffset
		}
		return true // Consumed scroll input
	}

	// Get mouse position
	mx, my := ebiten.CursorPosition()

	// Update text input
	lm.nameInput.Update()

	// Handle clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		lm.handleClick(mx, my)
		return true // Consumed click input
	}

	// Update hover states
	lm.updateHoverStates(mx, my)

	// Return true to indicate input was handled (when manager is visible)
	return true
}

// handleClick handles mouse clicks on various UI elements
func (lm *LoadoutManager) handleClick(mx, my int) {
	screenW, screenH := ebiten.WindowSize()
	panelWidth := 800
	panelHeight := 600
	x := (screenW - panelWidth) / 2
	y := (screenH - panelHeight) / 2

	// Check if clicked on name input
	nameInputX := x + 20
	nameInputY := y + 140
	nameClicked := mx >= nameInputX && mx < nameInputX+lm.nameInput.Width &&
		my >= nameInputY && my < nameInputY+lm.nameInput.Height

	// Update focus based on clicks
	if nameClicked {
		lm.nameInput.Focused = true
		lm.nameInput.cursorBlink = time.Now()
		// Set cursor position based on click position (simplified to end for now)
		lm.nameInput.cursorPos = len(lm.nameInput.Value)
		lm.nameInput.selStart = -1
		lm.nameInput.selEnd = -1
		return // Don't process other clicks when focusing text input
	} else {
		// Clicked outside text input, remove focus
		lm.nameInput.Focused = false
	}

	// Close button
	closeX := x + panelWidth - 35
	closeY := y + 10
	if mx >= closeX && mx <= closeX+25 && my >= closeY && my <= closeY+25 {
		lm.Hide()
		return
	}

	// Add button
	addButtonY := y + 140 // Match the draw position
	if mx >= x+350 && mx <= x+400 && my >= addButtonY && my <= addButtonY+30 {
		lm.addLoadout()
		return
	}

	// Import button
	importButtonY := y + 180 // Match the draw position
	if mx >= x+20 && mx <= x+90 && my >= importButtonY && my <= importButtonY+30 {
		lm.importLoadouts()
		return
	}

	// Export button
	exportButtonY := y + 180 // Match the draw position
	if mx >= x+100 && mx <= x+170 && my >= exportButtonY && my <= exportButtonY+30 {
		lm.exportLoadouts()
		return
	}

	// Loadout list items (use same logic as drawing)
	listY := y + 250                // Match the drawLoadoutsList call: y+250
	listHeight := panelHeight - 280 // Same as in main draw call
	itemHeight := 60
	maxVisibleItems := listHeight / itemHeight

	for i := 0; i < maxVisibleItems && i < len(lm.loadouts); i++ {
		itemIndex := i + lm.scrollOffset
		if itemIndex >= len(lm.loadouts) {
			break
		}

		itemY := listY + i*itemHeight // Use i (visible index) not itemIndex (absolute index)
		if itemY < y+250 || itemY > y+panelHeight-60 {
			continue // Skip items outside visible area
		}

		// Calculate item coordinates (match the exact drawLoadoutItem parameters)
		itemX := x + 20              // Match the drawLoadoutItem call: x+20
		itemWidth := panelWidth - 60 // Match the drawLoadoutItem call: width-60 = 740

		// Edit button - coordinates relative to item position
		editButtonX := itemX + itemWidth - 180
		if mx >= editButtonX && mx <= editButtonX+50 && my >= itemY+10 && my <= itemY+40 {
			lm.editLoadout(itemIndex)
			return
		}

		// Delete button - coordinates relative to item position
		deleteButtonX := itemX + itemWidth - 120
		if mx >= deleteButtonX && mx <= deleteButtonX+50 && my >= itemY+10 && my <= itemY+40 {
			lm.deleteLoadout(itemIndex)
			return
		}

		// Apply button - coordinates relative to item position
		applyButtonX := itemX + itemWidth - 60
		if mx >= applyButtonX && mx <= applyButtonX+50 && my >= itemY+10 && my <= itemY+40 {
			lm.applyLoadout(itemIndex)
			return
		}
	}
}

// updateHoverStates updates hover states for UI elements
func (lm *LoadoutManager) updateHoverStates(mx, my int) {
	screenW, screenH := ebiten.WindowSize()
	panelWidth := 800
	panelHeight := 600
	x := (screenW - panelWidth) / 2
	y := (screenH - panelHeight) / 2

	// Close button hover
	closeX := x + panelWidth - 35
	closeY := y + 10
	lm.closeButtonHovered = mx >= closeX && mx <= closeX+25 && my >= closeY && my <= closeY+25

	// Add button hover
	addButtonY := y + 140 // Match the draw position
	lm.addButtonHovered = mx >= x+350 && mx <= x+400 && my >= addButtonY && my <= addButtonY+30

	// Import/Export button hover
	importButtonY := y + 180
	exportButtonY := y + 180
	lm.importButtonHovered = mx >= x+20 && mx <= x+90 && my >= importButtonY && my <= importButtonY+30
	lm.exportButtonHovered = mx >= x+100 && mx <= x+170 && my >= exportButtonY && my <= exportButtonY+30

	// Reset all button hover states
	for i := range lm.editButtonHovered {
		lm.editButtonHovered[i] = false
		lm.deleteButtonHovered[i] = false
		lm.applyButtonHovered[i] = false
	}

	// Loadout list item hovers (use same logic as drawing and click detection)
	listY := y + 250                // Match the drawLoadoutsList call: y+250
	listHeight := panelHeight - 280 // Same as in main draw call
	itemHeight := 60
	maxVisibleItems := listHeight / itemHeight

	for i := 0; i < maxVisibleItems && i < len(lm.loadouts); i++ {
		itemIndex := i + lm.scrollOffset
		if itemIndex >= len(lm.loadouts) {
			break
		}

		itemY := listY + i*itemHeight // Use i (visible index) not itemIndex (absolute index)
		if itemY < y+250 || itemY > y+panelHeight-60 {
			continue
		}

		// Calculate item coordinates (match the exact drawLoadoutItem parameters)
		itemX := x + 20              // Match the drawLoadoutItem call: x+20
		itemWidth := panelWidth - 60 // Match the drawLoadoutItem call: width-60 = 740

		// Edit button hover - coordinates relative to item position (aligned with drawn button)
		editButtonX := itemX + itemWidth - 180
		lm.editButtonHovered[itemIndex] = mx >= editButtonX && mx <= editButtonX+50 && my >= itemY+10 && my <= itemY+40

		// Delete button hover - coordinates relative to item position (aligned with drawn button)
		deleteButtonX := itemX + itemWidth - 120
		lm.deleteButtonHovered[itemIndex] = mx >= deleteButtonX && mx <= deleteButtonX+50 && my >= itemY+10 && my <= itemY+40

		// Apply button hover - coordinates relative to item position (aligned with drawn button)
		applyButtonX := itemX + itemWidth - 60
		lm.applyButtonHovered[itemIndex] = mx >= applyButtonX && mx <= applyButtonX+50 && my >= itemY+10 && my <= itemY+40
	}
}

// addLoadout adds a new loadout
func (lm *LoadoutManager) addLoadout() {
	name := strings.TrimSpace(lm.nameInput.Value)
	if name == "" {
		NewToast().
			Text("Please enter a loadout name", ToastOption{Colour: color.RGBA{255, 200, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	// Check for duplicate names
	for _, loadout := range lm.loadouts {
		if loadout.Name == name {
			NewToast().
				Text("Loadout name already exists", ToastOption{Colour: color.RGBA{255, 150, 100, 255}}).
				AutoClose(time.Second * 3).
				Show()
			return
		}
	}

	// Create new loadout with default values
	newLoadout := LoadoutData{
		Name: name,
		TerritoryOptions: typedef.TerritoryOptions{
			Upgrades: typedef.Upgrade{
				Damage:  0,
				Attack:  0,
				Health:  0,
				Defence: 0,
			},
			Bonuses: typedef.Bonus{
				StrongerMinions:       0,
				TowerMultiAttack:      0,
				TowerAura:             0,
				TowerVolley:           0,
				XpSeeking:             0,
				TomeSeeking:           0,
				EmeraldSeeking:        0,
				LargerResourceStorage: 0,
				LargerEmeraldStorage:  0,
				EfficientResource:     0,
				EfficientEmerald:      0,
				ResourceRate:          0,
				EmeraldRate:           0,
			},
			Tax: typedef.TerritoryTax{
				Tax:  0.05, // 5% default
				Ally: 0.05, // 5% default
			},
			RoutingMode: typedef.RoutingCheapest,
			Border:      typedef.BorderOpen,
			HQ:          false,
		},
	}

	lm.loadouts = append(lm.loadouts, newLoadout)
	lm.nameInput.Value = ""
	lm.nameInput.cursorPos = 0
	lm.saveToFile()
	NewToast().
		Text("Loadout added successfully", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// editLoadout opens the edit side menu for a loadout
func (lm *LoadoutManager) editLoadout(index int) {
	if index < 0 || index >= len(lm.loadouts) {
		return
	}

	// If edit menu is already visible and we're editing the same loadout, don't replay animation
	if lm.editSideMenuVisible && lm.editingIndex == index {
		return
	}

	lm.editingIndex = index
	lm.editingLoadout = &LoadoutData{}

	fmt.Printf("[LOADOUT] About to copy loadout[%d]\n", index)
	fmt.Printf("[LOADOUT] Source loadout name: %s\n", lm.loadouts[index].Name)
	fmt.Printf("[LOADOUT] Source loadout upgrades: %+v\n", lm.loadouts[index].Upgrades)
	fmt.Printf("[LOADOUT] Source loadout bonuses: %+v\n", lm.loadouts[index].Bonuses)

	// Create a deep copy of the loadout to avoid modifying the original
	lm.editingLoadout.Name = lm.loadouts[index].Name
	lm.editingLoadout.TerritoryOptions = lm.loadouts[index].TerritoryOptions

	fmt.Printf("[LOADOUT] After copy - editingLoadout name: %s\n", lm.editingLoadout.Name)
	fmt.Printf("[LOADOUT] After copy - editingLoadout upgrades: %+v\n", lm.editingLoadout.Upgrades)
	fmt.Printf("[LOADOUT] After copy - editingLoadout bonuses: %+v\n", lm.editingLoadout.Bonuses)

	// Initialize the fake "loadout" territory with the current loadout values
	// This allows the UpgradeControl and BonusControl to work with the existing system
	opts := typedef.TerritoryOptions{
		Upgrades:    lm.editingLoadout.Upgrades,
		Bonuses:     lm.editingLoadout.Bonuses,
		Tax:         lm.editingLoadout.Tax,
		RoutingMode: lm.editingLoadout.RoutingMode,
		Border:      lm.editingLoadout.Border,
		HQ:          false,
	}
	result := eruntime.Set("loadout", opts)
	if result != nil {
		fmt.Printf("[LOADOUT] Successfully created fake territory with upgrades: %+v\n", result.Options.Upgrade.Set)
	} else {
		fmt.Printf("[LOADOUT] Failed to create fake territory\n")
	}

	lm.showEditSideMenu()
}

// deleteLoadout removes a loadout
func (lm *LoadoutManager) deleteLoadout(index int) {
	if index < 0 || index >= len(lm.loadouts) {
		return
	}

	// Remove loadout
	lm.loadouts = append(lm.loadouts[:index], lm.loadouts[index+1:]...)
	lm.saveToFile()
	NewToast().
		Text("Loadout deleted successfully", ToastOption{Colour: color.RGBA{255, 150, 150, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// applyLoadout starts the loadout application mode for territory selection
func (lm *LoadoutManager) applyLoadout(index int) {
	if index < 0 || index >= len(lm.loadouts) {
		return
	}

	fmt.Printf("[LOADOUT] Starting loadout application mode for: %s\n", lm.loadouts[index].Name)

	lm.isApplyingLoadout = true
	lm.applyingLoadoutIndex = index
	lm.applyingLoadoutName = lm.loadouts[index].Name
	lm.selectedTerritories = make(map[string]bool)
	lm.applyUIVisible = true

	// Close the loadout manager main window when entering apply mode
	lm.visible = false

	// Close any open side menus
	if lm.editSideMenuVisible {
		lm.hideEditSideMenu()
	}

	// Close any open territory menus
	mapView := GetMapView()
	if mapView != nil {
		// Close EdgeMenu if open
		if mapView.edgeMenu != nil && mapView.edgeMenu.IsVisible() {
			mapView.edgeMenu.Hide()
		}

		// Close territory side menu if open
		if mapView.territoriesManager != nil && mapView.territoriesManager.IsSideMenuOpen() {
			mapView.territoriesManager.CloseSideMenu()
		}

		// Deselect any currently selected territory
		if mapView.territoriesManager != nil {
			mapView.territoriesManager.DeselectTerritory()
		}

		// Update the territory renderer to show loadout application mode
		if mapView.territoriesManager != nil && mapView.territoriesManager.IsLoaded() {
			if renderer := mapView.territoriesManager.GetRenderer(); renderer != nil {
				renderer.SetLoadoutApplicationMode(lm.applyingLoadoutName, lm.selectedTerritories)
				// Force a redraw to show the highlighting
				if cache := renderer.GetTerritoryCache(); cache != nil {
					cache.ForceRedraw()
				}
			}
		}
	}
}

// StopLoadoutApplication stops the loadout application mode and applies the loadout to selected territories
func (lm *LoadoutManager) StopLoadoutApplication() {
	if !lm.isApplyingLoadout {
		return
	}

	fmt.Printf("[LOADOUT] Stopping loadout application for: %s\n", lm.applyingLoadoutName)
	fmt.Printf("[LOADOUT] Selected territories: %v\n", lm.selectedTerritories)

	if lm.applyingLoadoutIndex < 0 || lm.applyingLoadoutIndex >= len(lm.loadouts) {
		lm.CancelLoadoutApplication()
		return
	}

	loadout := lm.loadouts[lm.applyingLoadoutIndex]
	fmt.Printf("[LOADOUT] Loadout to apply: %+v\n", loadout)

	// Apply loadout to selected territories
	appliedCount := 0
	for territoryName := range lm.selectedTerritories {
		if lm.selectedTerritories[territoryName] {
			// Don't change HQ status when applying loadout
			opts := loadout.TerritoryOptions
			opts.HQ = false

			fmt.Printf("[LOADOUT] Applying loadout to territory: %s\n", territoryName)
			fmt.Printf("[LOADOUT] Loadout upgrades: Damage=%d, Attack=%d, Health=%d, Defence=%d\n",
				loadout.Upgrades.Damage, loadout.Upgrades.Attack, loadout.Upgrades.Health, loadout.Upgrades.Defence)

			result := eruntime.Set(territoryName, opts)
			if result != nil {
				appliedCount++
				fmt.Printf("[LOADOUT] Successfully applied loadout to territory: %s\n", territoryName)
			} else {
				fmt.Printf("[LOADOUT] Failed to apply loadout to territory: %s (territory not found)\n", territoryName)
			}
		}
	}

	// Clear application mode state
	lm.isApplyingLoadout = false
	lm.applyingLoadoutIndex = -1
	lm.applyingLoadoutName = ""
	lm.selectedTerritories = make(map[string]bool)
	lm.applyUIVisible = false

	// Clear the territory renderer from loadout application mode
	mapView := GetMapView()
	if mapView != nil && mapView.territoriesManager != nil && mapView.territoriesManager.IsLoaded() {
		if renderer := mapView.territoriesManager.GetRenderer(); renderer != nil {
			renderer.ClearLoadoutApplicationMode()
			// Force a redraw to clear the highlighting
			if cache := renderer.GetTerritoryCache(); cache != nil {
				cache.ForceRedraw()
			}
		}
	}

	NewToast().
		Text("Loadout Applied", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text(fmt.Sprintf("Applied loadout '%s' to %d territories", loadout.Name, appliedCount), ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		AutoClose(time.Second * 5).
		Show()
}

// CancelLoadoutApplication cancels the loadout application mode without applying changes
func (lm *LoadoutManager) CancelLoadoutApplication() {
	if !lm.isApplyingLoadout {
		return
	}

	fmt.Printf("[LOADOUT] Canceling loadout application for: %s\n", lm.applyingLoadoutName)

	// Clear application mode state
	lm.isApplyingLoadout = false
	lm.applyingLoadoutIndex = -1
	lm.applyingLoadoutName = ""
	lm.selectedTerritories = make(map[string]bool)
	lm.applyUIVisible = false

	// Clear the territory renderer from loadout application mode
	mapView := GetMapView()
	if mapView != nil && mapView.territoriesManager != nil && mapView.territoriesManager.IsLoaded() {
		if renderer := mapView.territoriesManager.GetRenderer(); renderer != nil {
			renderer.ClearLoadoutApplicationMode()
			// Force a redraw to clear the highlighting
			if cache := renderer.GetTerritoryCache(); cache != nil {
				cache.ForceRedraw()
			}
		}
	}

	NewToast().
		Text("Loadout Application Canceled", ToastOption{Colour: color.RGBA{255, 200, 100, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// handleApplyModeClick handles mouse clicks during loadout application mode
func (lm *LoadoutManager) handleApplyModeClick(mx, my int) bool {
	screenW, _ := ebiten.WindowSize()

	// Handle clicks on the apply mode UI buttons
	if lm.applyUIVisible {
		// Calculate UI banner position (same as claim editing UI)
		overlayHeight := 140
		buttonWidth := 70
		buttonHeight := 30
		buttonSpacing := 10

		// Apply button - positioned on the right side like guild claim editing
		applyButtonX := screenW - (buttonWidth*2 + buttonSpacing + 20)
		applyButtonY := overlayHeight - 50
		fmt.Printf("[LOADOUT] Checking apply button: mouse(%d,%d) vs button(%d,%d,%d,%d)\n",
			mx, my, applyButtonX, applyButtonY, applyButtonX+buttonWidth, applyButtonY+buttonHeight)
		if mx >= applyButtonX && mx <= applyButtonX+buttonWidth && my >= applyButtonY && my <= applyButtonY+buttonHeight {
			fmt.Printf("[LOADOUT] Apply button clicked!\n")
			lm.StopLoadoutApplication()
			return true
		}

		// Cancel button - positioned next to Apply button on the right
		cancelButtonX := screenW - (buttonWidth + 20)
		cancelButtonY := overlayHeight - 50
		fmt.Printf("[LOADOUT] Checking cancel button: mouse(%d,%d) vs button(%d,%d,%d,%d)\n",
			mx, my, cancelButtonX, cancelButtonY, cancelButtonX+buttonWidth, cancelButtonY+buttonHeight)
		if mx >= cancelButtonX && mx <= cancelButtonX+buttonWidth && my >= cancelButtonY && my <= cancelButtonY+buttonHeight {
			fmt.Printf("[LOADOUT] Cancel button clicked!\n")
			lm.CancelLoadoutApplication()
			return true
		}
	}

	return false
}

// IsApplyingLoadout returns whether the loadout manager is in application mode
func (lm *LoadoutManager) IsApplyingLoadout() bool {
	return lm.isApplyingLoadout
}

// GetApplyingLoadoutName returns the name of the loadout being applied
func (lm *LoadoutManager) GetApplyingLoadoutName() string {
	return lm.applyingLoadoutName
}

// GetSelectedTerritories returns the currently selected territories for loadout application
func (lm *LoadoutManager) GetSelectedTerritories() map[string]bool {
	return lm.selectedTerritories
}

// AddTerritorySelection adds a territory to the selection for loadout application
func (lm *LoadoutManager) AddTerritorySelection(territoryName string) {
	if lm.isApplyingLoadout {
		lm.selectedTerritories[territoryName] = true
		fmt.Printf("[LOADOUT] Added territory to selection: %s\n", territoryName)

		// Update the territory renderer with the new selection
		lm.updateTerritoryRenderer()
	}
}

// RemoveTerritorySelection removes a territory from the selection for loadout application
func (lm *LoadoutManager) RemoveTerritorySelection(territoryName string) {
	if lm.isApplyingLoadout {
		delete(lm.selectedTerritories, territoryName)
		fmt.Printf("[LOADOUT] Removed territory from selection: %s\n", territoryName)

		// Update the territory renderer with the new selection
		lm.updateTerritoryRenderer()
	}
}

// ToggleTerritorySelection toggles a territory's selection state for loadout application
func (lm *LoadoutManager) ToggleTerritorySelection(territoryName string) {
	if lm.isApplyingLoadout {
		if lm.selectedTerritories[territoryName] {
			lm.RemoveTerritorySelection(territoryName)
		} else {
			lm.AddTerritorySelection(territoryName)
		}
	}
}

// Draw renders the loadout manager
func (lm *LoadoutManager) Draw(screen *ebiten.Image) {
	if !lm.visible && !lm.isApplyingLoadout {
		return
	}

	// If in loadout application mode, draw apply mode UI instead
	if lm.isApplyingLoadout && lm.applyUIVisible {
		lm.drawApplyModeUI(screen)
		return
	}

	// Regular loadout manager UI
	if !lm.visible {
		return
	}

	screenW, screenH := ebiten.WindowSize()

	// Draw overlay background
	overlayColor := color.RGBA{0, 0, 0, 150}
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH), overlayColor, false)

	// Panel dimensions and position
	panelWidth := 800
	panelHeight := 600
	x := float32((screenW - panelWidth) / 2)
	y := float32((screenH - panelHeight) / 2)

	// Draw main panel background
	panelColor := color.RGBA{40, 40, 50, 255}
	vector.DrawFilledRect(screen, x, y, float32(panelWidth), float32(panelHeight), panelColor, false)

	// Draw border
	borderColor := color.RGBA{100, 100, 120, 255}
	vector.StrokeRect(screen, x, y, float32(panelWidth), float32(panelHeight), 2, borderColor, false)

	// Title
	titleFont := loadWynncraftFont(24)
	titleText := "Loadout Manager"
	titleBounds := text.BoundString(titleFont, titleText)
	titleX := int(x) + (panelWidth-titleBounds.Dx())/2
	titleY := int(y) + 40
	text.Draw(screen, titleText, titleFont, titleX, titleY, color.RGBA{255, 255, 255, 255})

	// Close button
	closeX := int(x) + panelWidth - 35
	closeY := int(y) + 10
	closeColor := color.RGBA{200, 50, 50, 255}
	if lm.closeButtonHovered {
		closeColor = color.RGBA{255, 80, 80, 255}
	}
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), 25, 25, closeColor, false)
	vector.StrokeRect(screen, float32(closeX), float32(closeY), 25, 25, 2, color.RGBA{100, 100, 120, 255}, false)

	closeFont := loadWynncraftFont(16)
	text.Draw(screen, "X", closeFont, closeX+8, closeY+17, color.RGBA{255, 255, 255, 255})

	// Instructions
	instructionFont := loadWynncraftFont(16) // Increased from 14 to 16
	instructionText := "Create loadouts to quickly apply territory configurations. Enter a name and click Add."
	text.Draw(screen, instructionText, instructionFont, int(x)+20, int(y)+80, color.RGBA{200, 200, 200, 255})

	// Name input label
	labelFont := loadWynncraftFont(18) // Increased from 16 to 18
	text.Draw(screen, "Name:", labelFont, int(x)+20, int(y)+130, color.RGBA{255, 255, 255, 255})

	// Name input
	lm.nameInput.X = int(x) + 20
	lm.nameInput.Y = int(y) + 140
	lm.nameInput.Draw(screen)

	// Add button
	addButtonX := int(x) + 350
	addButtonY := int(y) + 140
	addButtonColor := color.RGBA{50, 150, 50, 255}
	if lm.addButtonHovered {
		addButtonColor = color.RGBA{70, 200, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(addButtonX), float32(addButtonY), 50, 30, addButtonColor, false)
	vector.StrokeRect(screen, float32(addButtonX), float32(addButtonY), 50, 30, 2, color.RGBA{100, 100, 120, 255}, false)

	addFont := loadWynncraftFont(16) // Increased from 14 to 16
	text.Draw(screen, "Add", addFont, addButtonX+15, addButtonY+20, color.RGBA{255, 255, 255, 255})

	// Import/Export buttons
	importButtonX := int(x) + 20
	importButtonY := int(y) + 180
	importButtonColor := color.RGBA{50, 100, 150, 255}
	if lm.importButtonHovered {
		importButtonColor = color.RGBA{70, 130, 200, 255}
	}
	vector.DrawFilledRect(screen, float32(importButtonX), float32(importButtonY), 70, 30, importButtonColor, false)
	vector.StrokeRect(screen, float32(importButtonX), float32(importButtonY), 70, 30, 2, color.RGBA{100, 100, 120, 255}, false)
	text.Draw(screen, "Import", addFont, importButtonX+15, importButtonY+20, color.RGBA{255, 255, 255, 255})

	exportButtonX := int(x) + 100
	exportButtonY := int(y) + 180
	exportButtonColor := color.RGBA{150, 100, 50, 255}
	if lm.exportButtonHovered {
		exportButtonColor = color.RGBA{200, 130, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(exportButtonX), float32(exportButtonY), 70, 30, exportButtonColor, false)
	vector.StrokeRect(screen, float32(exportButtonX), float32(exportButtonY), 70, 30, 2, color.RGBA{100, 100, 120, 255}, false)
	text.Draw(screen, "Export", addFont, exportButtonX+15, exportButtonY+20, color.RGBA{255, 255, 255, 255})

	// Loadouts list header
	headerY := int(y) + 230
	text.Draw(screen, "Loadouts:", labelFont, int(x)+20, headerY, color.RGBA{255, 255, 255, 255})

	// Loadouts list
	lm.drawLoadoutsList(screen, int(x), int(y)+250, panelWidth, panelHeight-280)

	// Draw edit side menu if visible
	if lm.editSideMenuVisible && lm.editSideMenu != nil {
		lm.editSideMenu.Draw(screen)
	}
}

// drawLoadoutsList draws the list of loadouts with proper scrollbar and bounds like guild manager
func (lm *LoadoutManager) drawLoadoutsList(screen *ebiten.Image, x, y, width, height int) {
	listFont := loadWynncraftFont(16) // Increased from 14 to 16

	// Draw scrollable list background
	listBgColor := color.RGBA{30, 30, 40, 255}
	vector.DrawFilledRect(screen, float32(x+10), float32(y), float32(width-20), float32(height), listBgColor, false)

	// Draw border
	listBorderColor := color.RGBA{80, 80, 100, 255}
	vector.StrokeRect(screen, float32(x+10), float32(y), float32(width-20), float32(height), 1, listBorderColor, false)

	if len(lm.loadouts) == 0 {
		// No loadouts message
		emptyText := "No loadouts created yet. Create one using the form above."
		emptyColor := color.RGBA{150, 150, 150, 255}
		text.Draw(screen, emptyText, listFont, x+30, y+30, emptyColor)
		return
	}

	// Calculate visible items like guild manager
	itemHeight := 60
	maxVisibleItems := height / itemHeight

	// Clamp scroll offset to valid range
	maxScrollOffset := len(lm.loadouts) - maxVisibleItems
	if maxScrollOffset < 0 {
		maxScrollOffset = 0
	}
	if lm.scrollOffset > maxScrollOffset {
		lm.scrollOffset = maxScrollOffset
	}
	if lm.scrollOffset < 0 {
		lm.scrollOffset = 0
	}

	// Draw visible loadout items only (like guild manager)
	for i := 0; i < maxVisibleItems && i < len(lm.loadouts); i++ {
		itemIndex := i + lm.scrollOffset
		if itemIndex >= len(lm.loadouts) {
			break
		}

		loadout := lm.loadouts[itemIndex]
		itemY := y + (i * itemHeight)

		// Only draw if item is within visible bounds
		if itemY >= y && itemY <= y+height-itemHeight {
			lm.drawLoadoutItem(screen, x+20, itemY, width-60, loadout, itemIndex) // width-60 to leave space for scrollbar
		}
	}

	// Draw scrollbar if needed (like guild manager)
	if len(lm.loadouts) > maxVisibleItems {
		scrollbarX := x + width - 25
		scrollbarY := y
		scrollbarWidth := 8
		scrollbarHeight := height

		// Draw scrollbar track
		trackColor := color.RGBA{80, 80, 100, 255}
		vector.DrawFilledRect(screen, float32(scrollbarX), float32(scrollbarY), float32(scrollbarWidth), float32(scrollbarHeight), trackColor, false)

		// Calculate thumb size and position
		thumbHeight := scrollbarHeight * maxVisibleItems / len(lm.loadouts)
		if thumbHeight < 20 {
			thumbHeight = 20 // Minimum thumb size
		}

		thumbY := scrollbarY
		if maxScrollOffset > 0 {
			thumbY = scrollbarY + (scrollbarHeight-thumbHeight)*lm.scrollOffset/maxScrollOffset
		}

		// Draw scrollbar thumb
		thumbColor := color.RGBA{120, 120, 150, 255}
		vector.DrawFilledRect(screen, float32(scrollbarX), float32(thumbY), float32(scrollbarWidth), float32(thumbHeight), thumbColor, false)
	}
}

// drawLoadoutItem draws a single loadout item
func (lm *LoadoutManager) drawLoadoutItem(screen *ebiten.Image, x, y, width int, loadout LoadoutData, index int) {
	itemFont := loadWynncraftFont(16)  // Increased from 14 to 16
	smallFont := loadWynncraftFont(14) // Increased from 12 to 14

	// Item background
	itemBgColor := color.RGBA{50, 50, 60, 255}
	if index%2 == 1 {
		itemBgColor = color.RGBA{45, 45, 55, 255}
	}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), 50, itemBgColor, false)

	// Loadout name
	nameColor := color.RGBA{255, 255, 255, 255}
	text.Draw(screen, loadout.Name, itemFont, x+10, y+20, nameColor)

	// Loadout summary
	summaryText := fmt.Sprintf("Tax: %d%% | Routing: %s | Border: %s | Upgrades: %d levels",
		int(loadout.Tax.Tax*100),
		getRoutingModeString(loadout.RoutingMode),
		getBorderString(loadout.Border),
		getTotalUpgradeLevels(loadout.Upgrades))
	summaryColor := color.RGBA{180, 180, 180, 255}
	text.Draw(screen, summaryText, smallFont, x+10, y+35, summaryColor)

	// Edit button
	editButtonX := x + width - 180
	editButtonColor := color.RGBA{70, 100, 150, 255}
	if lm.editButtonHovered[index] {
		editButtonColor = color.RGBA{90, 130, 200, 255}
	}
	vector.DrawFilledRect(screen, float32(editButtonX), float32(y+10), 50, 30, editButtonColor, false)
	vector.StrokeRect(screen, float32(editButtonX), float32(y+10), 50, 30, 2, color.RGBA{100, 100, 120, 255}, false)
	text.Draw(screen, "Edit", smallFont, editButtonX+15, y+25, color.RGBA{255, 255, 255, 255})

	// Delete button
	deleteButtonX := x + width - 120
	deleteButtonColor := color.RGBA{150, 70, 70, 255}
	if lm.deleteButtonHovered[index] {
		deleteButtonColor = color.RGBA{200, 90, 90, 255}
	}
	vector.DrawFilledRect(screen, float32(deleteButtonX), float32(y+10), 50, 30, deleteButtonColor, false)
	vector.StrokeRect(screen, float32(deleteButtonX), float32(y+10), 50, 30, 2, color.RGBA{100, 100, 120, 255}, false)
	text.Draw(screen, "Delete", smallFont, deleteButtonX+10, y+25, color.RGBA{255, 255, 255, 255})

	// Apply button
	applyButtonX := x + width - 60
	applyButtonColor := color.RGBA{70, 150, 70, 255}
	if lm.applyButtonHovered[index] {
		applyButtonColor = color.RGBA{90, 200, 90, 255}
	}
	vector.DrawFilledRect(screen, float32(applyButtonX), float32(y+10), 50, 30, applyButtonColor, false)
	vector.StrokeRect(screen, float32(applyButtonX), float32(y+10), 50, 30, 2, color.RGBA{100, 100, 120, 255}, false)
	text.Draw(screen, "Apply", smallFont, applyButtonX+10, y+25, color.RGBA{255, 255, 255, 255})
}

// Helper functions for displaying loadout information
func getRoutingModeString(mode typedef.Routing) string {
	if mode == typedef.RoutingFastest {
		return "Fastest"
	}
	return "Cheapest"
}

func getBorderString(border typedef.Border) string {
	if border == typedef.BorderClosed {
		return "Closed"
	}
	return "Open"
}

func getTotalUpgradeLevels(upgrades typedef.Upgrade) int {
	return upgrades.Damage + upgrades.Attack + upgrades.Health + upgrades.Defence
}

// File operations
func (lm *LoadoutManager) saveToFile() {
	data := LoadoutImportExport{
		Type:     "loadouts",
		Version:  "1.0",
		Loadouts: lm.loadouts,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling loadouts: %v\n", err)
		return
	}

	// Create loadouts directory if it doesn't exist
	loadoutsDir := "loadouts"
	if _, err := os.Stat(loadoutsDir); os.IsNotExist(err) {
		os.MkdirAll(loadoutsDir, 0755)
	}

	// Save to file
	filename := filepath.Join(loadoutsDir, "loadouts.json")
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error saving loadouts: %v\n", err)
	}
}

func (lm *LoadoutManager) loadFromFile() {
	filename := filepath.Join("loadouts", "loadouts.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		// File doesn't exist or can't be read, start with empty loadouts
		lm.loadouts = make([]LoadoutData, 0)
		return
	}

	var importData LoadoutImportExport
	err = json.Unmarshal(data, &importData)
	if err != nil {
		fmt.Printf("Error parsing loadouts file: %v\n", err)
		lm.loadouts = make([]LoadoutData, 0)
		return
	}

	// Validate file type
	if importData.Type != "loadouts" {
		fmt.Printf("Invalid file type: %s (expected: loadouts)\n", importData.Type)
		lm.loadouts = make([]LoadoutData, 0)
		return
	}

	lm.loadouts = importData.Loadouts
}

func (lm *LoadoutManager) importLoadouts() {
	// For now, we'll implement a simple clipboard-based import
	// In a full implementation, you'd want a file dialog

	clipboardData := clipboard.Read(clipboard.FmtText)
	if len(clipboardData) == 0 {
		NewToast().
			Text("No data in clipboard to import", ToastOption{Colour: color.RGBA{255, 200, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	var importData LoadoutImportExport
	err := json.Unmarshal(clipboardData, &importData)
	if err != nil {
		NewToast().
			Text("Invalid JSON data in clipboard", ToastOption{Colour: color.RGBA{255, 150, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	if importData.Type != "loadouts" {
		NewToast().
			Text("Invalid file type (expected: loadouts)", ToastOption{Colour: color.RGBA{255, 150, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	// Add imported loadouts (avoiding duplicates)
	importedCount := 0
	for _, importedLoadout := range importData.Loadouts {
		// Check for duplicate names
		exists := false
		for _, existingLoadout := range lm.loadouts {
			if existingLoadout.Name == importedLoadout.Name {
				exists = true
				break
			}
		}

		if !exists {
			lm.loadouts = append(lm.loadouts, importedLoadout)
			importedCount++
		}
	}

	lm.saveToFile()
	NewToast().
		Text("Import Complete", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text(fmt.Sprintf("Imported %d loadouts", importedCount), ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		AutoClose(time.Second * 4).
		Show()
}

func (lm *LoadoutManager) exportLoadouts() {
	if len(lm.loadouts) == 0 {
		NewToast().
			Text("No loadouts to export", ToastOption{Colour: color.RGBA{255, 200, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	data := LoadoutImportExport{
		Type:     "loadouts",
		Version:  "1.0",
		Loadouts: lm.loadouts,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		NewToast().
			Text("Error creating export data", ToastOption{Colour: color.RGBA{255, 150, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return
	}

	// Copy to clipboard
	clipboard.Write(clipboard.FmtText, jsonData)
	NewToast().
		Text("Export Complete", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text("Loadouts copied to clipboard", ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		AutoClose(time.Second * 4).
		Show()
}

// Global loadout manager instance
var globalLoadoutManager *LoadoutManager

// GetLoadoutManager returns the global loadout manager instance
func GetLoadoutManager() *LoadoutManager {
	if globalLoadoutManager == nil {
		globalLoadoutManager = NewLoadoutManager()
	}
	return globalLoadoutManager
}

// showEditSideMenu creates and shows the edit side menu for a loadout
func (lm *LoadoutManager) showEditSideMenu() {
	if lm.editingLoadout == nil {
		return
	}

	// If menu is already visible, just rebuild content without replaying animation
	if lm.editSideMenuVisible && lm.editSideMenu != nil {
		lm.buildEditSideMenuContent()
		return
	}

	lm.editSideMenuVisible = true

	// Create a new EdgeMenu for editing - positioned on the RIGHT like territory menu
	options := DefaultEdgeMenuOptions()
	options.Width = 400                              // Same as territory menu
	options.Position = EdgeMenuRight                 // RIGHT side like territory menu
	options.Background = color.RGBA{30, 30, 45, 240} // Same as territory menu
	options.BorderColor = color.RGBA{80, 80, 255, 150}
	options.Scrollable = true // Enable scrolling for content overflow
	options.Height = 0        // Full screen height

	lm.editSideMenu = NewEdgeMenu(fmt.Sprintf("Loadout: %s", lm.editingLoadout.Name), options)

	// Add loadout configuration elements
	lm.buildEditSideMenuContent()

	lm.editSideMenu.Show()
}

// hideEditSideMenu hides the edit side menu
func (lm *LoadoutManager) hideEditSideMenu() {
	if lm.editSideMenu != nil {
		lm.editSideMenu.Hide()
	}
	lm.editSideMenuVisible = false
	lm.editingIndex = -1
	lm.editingLoadout = nil

	// Clean up the fake "loadout" territory
	// Note: eruntime.Remove() or similar might not exist, so we could just leave it
	// The fake territory will be overwritten next time anyway
}

// buildEditSideMenuContent builds the content for the edit side menu
func (lm *LoadoutManager) buildEditSideMenuContent() {
	if lm.editSideMenu == nil || lm.editingLoadout == nil {
		return
	}

	// Update the menu title to reflect the current loadout being edited
	lm.editSideMenu.SetTitle(fmt.Sprintf("Loadout: %s", lm.editingLoadout.Name))

	// Clear existing elements
	lm.editSideMenu.ClearElements()

	// Add instructions
	instructionOptions := DefaultTextOptions()
	instructionOptions.Color = color.RGBA{200, 200, 200, 255}
	lm.editSideMenu.Text("Configure the settings for this loadout:", instructionOptions)

	// Upgrades (collapsible) - Custom controls that directly update loadout
	upgradesMenu := lm.editSideMenu.CollapsibleMenu("Upgrades", DefaultCollapsibleMenuOptions())

	// Custom upgrade controls that directly modify lm.editingLoadout.Upgrades
	lm.addUpgradeSlider(upgradesMenu, "Damage", &lm.editingLoadout.Upgrades.Damage)
	lm.addUpgradeSlider(upgradesMenu, "Attack", &lm.editingLoadout.Upgrades.Attack)
	lm.addUpgradeSlider(upgradesMenu, "Health", &lm.editingLoadout.Upgrades.Health)
	lm.addUpgradeSlider(upgradesMenu, "Defence", &lm.editingLoadout.Upgrades.Defence)

	// Bonuses (collapsible) - Custom controls that directly update loadout
	bonusesMenu := lm.editSideMenu.CollapsibleMenu("Bonuses", DefaultCollapsibleMenuOptions())

	// Custom bonus controls that directly modify lm.editingLoadout.Bonuses
	lm.addBonusSlider(bonusesMenu, "Stronger Minions", "strongerMinions", &lm.editingLoadout.Bonuses.StrongerMinions)
	lm.addBonusSlider(bonusesMenu, "Tower Multi-Attack", "towerMultiAttack", &lm.editingLoadout.Bonuses.TowerMultiAttack)
	lm.addBonusSlider(bonusesMenu, "Tower Aura", "towerAura", &lm.editingLoadout.Bonuses.TowerAura)
	lm.addBonusSlider(bonusesMenu, "Tower Volley", "towerVolley", &lm.editingLoadout.Bonuses.TowerVolley)
	// lm.addBonusSlider(bonusesMenu, "XP Seeking", "xpSeeking", &lm.editingLoadout.Bonuses.XpSeeking)
	// lm.addBonusSlider(bonusesMenu, "Tome Seeking", "tomeSeeking", &lm.editingLoadout.Bonuses.TomeSeeking)
	// lm.addBonusSlider(bonusesMenu, "Emerald Seeking", "emeraldSeeking", &lm.editingLoadout.Bonuses.EmeraldSeeking)
	lm.addBonusSlider(bonusesMenu, "Larger Resource Storage", "largerResourceStorage", &lm.editingLoadout.Bonuses.LargerResourceStorage)
	lm.addBonusSlider(bonusesMenu, "Larger Emerald Storage", "largerEmeraldStorage", &lm.editingLoadout.Bonuses.LargerEmeraldStorage)
	lm.addBonusSlider(bonusesMenu, "Efficient Resource", "efficientResource", &lm.editingLoadout.Bonuses.EfficientResource)
	lm.addBonusSlider(bonusesMenu, "Efficient Emerald", "efficientEmerald", &lm.editingLoadout.Bonuses.EfficientEmerald)
	lm.addBonusSlider(bonusesMenu, "Resource Rate", "resourceRate", &lm.editingLoadout.Bonuses.ResourceRate)
	lm.addBonusSlider(bonusesMenu, "Emerald Rate", "emeraldRate", &lm.editingLoadout.Bonuses.EmeraldRate)

	// Routing and Taxes (collapsible)
	taxesMenu := lm.editSideMenu.CollapsibleMenu("Routing and Taxes", DefaultCollapsibleMenuOptions())

	// Routing Mode toggle switch
	routingModeIndex := 0
	if lm.editingLoadout.RoutingMode == typedef.RoutingFastest {
		routingModeIndex = 1
	}
	routingToggleOptions := DefaultToggleSwitchOptions()
	routingToggleOptions.Options = []string{"Cheapest", "Fastest"}
	taxesMenu.ToggleSwitch("Routing Mode", routingModeIndex, routingToggleOptions, func(index int, value string) {
		if index == 1 {
			lm.editingLoadout.RoutingMode = typedef.RoutingFastest
		} else {
			lm.editingLoadout.RoutingMode = typedef.RoutingCheapest
		}
	})

	// Border toggle switch
	borderIndex := 0
	if lm.editingLoadout.Border == typedef.BorderClosed {
		borderIndex = 1
	}
	borderToggleOptions := DefaultToggleSwitchOptions()
	borderToggleOptions.Options = []string{"Opened", "Closed"}
	taxesMenu.ToggleSwitch("Border", borderIndex, borderToggleOptions, func(index int, value string) {
		if index == 1 {
			lm.editingLoadout.Border = typedef.BorderClosed
		} else {
			lm.editingLoadout.Border = typedef.BorderOpen
		}
	})

	// Add divider/spacer
	spacerOptions := DefaultSpacerOptions()
	spacerOptions.Height = 10
	taxesMenu.Spacer(spacerOptions)

	// Tax input (5-70%)
	currentTax := int(lm.editingLoadout.Tax.Tax * 100)
	if currentTax < 5 {
		currentTax = 5
	}
	if currentTax > 70 {
		currentTax = 70
	}

	taxInputOptions := DefaultTextInputOptions()
	taxInputOptions.Width = 40
	taxInputOptions.MaxLength = 2
	taxInputOptions.Placeholder = "5-70"
	taxInputOptions.ValidateInput = func(newValue string) bool {
		if newValue == "" {
			return true
		}
		for _, r := range newValue {
			if r < '0' || r > '9' {
				return false
			}
		}
		if val, err := strconv.Atoi(newValue); err == nil {
			return val >= 5 && val <= 70
		}
		return false
	}

	taxesMenu.TextInput("Tax %", fmt.Sprintf("%d", currentTax), taxInputOptions, func(value string) {
		if value == "" {
			return
		}
		if taxValue, err := strconv.Atoi(value); err == nil {
			if taxValue < 5 {
				taxValue = 5
			}
			if taxValue > 70 {
				taxValue = 70
			}
			lm.editingLoadout.Tax.Tax = float64(taxValue) / 100.0
		}
	})

	// Ally Tax input (5-70%)
	currentAllyTax := int(lm.editingLoadout.Tax.Ally * 100)
	if currentAllyTax < 5 {
		currentAllyTax = 5
	}
	if currentAllyTax > 70 {
		currentAllyTax = 70
	}

	allyTaxInputOptions := DefaultTextInputOptions()
	allyTaxInputOptions.Width = 40
	allyTaxInputOptions.MaxLength = 2
	allyTaxInputOptions.Placeholder = "5-70"
	allyTaxInputOptions.ValidateInput = func(newValue string) bool {
		if newValue == "" {
			return true
		}
		for _, r := range newValue {
			if r < '0' || r > '9' {
				return false
			}
		}
		if val, err := strconv.Atoi(newValue); err == nil {
			return val >= 5 && val <= 70
		}
		return false
	}

	taxesMenu.TextInput("Ally Tax %", fmt.Sprintf("%d", currentAllyTax), allyTaxInputOptions, func(value string) {
		if value == "" {
			return
		}
		if allyTaxValue, err := strconv.Atoi(value); err == nil {
			if allyTaxValue < 5 {
				allyTaxValue = 5
			}
			if allyTaxValue > 70 {
				allyTaxValue = 70
			}
			lm.editingLoadout.Tax.Ally = float64(allyTaxValue) / 100.0
		}
	})

	// Action buttons
	saveButtonOptions := DefaultButtonOptions()
	saveButtonOptions.Height = 35
	saveButtonOptions.BackgroundColor = color.RGBA{50, 150, 50, 255}
	saveButtonOptions.HoverColor = color.RGBA{70, 200, 70, 255}
	lm.editSideMenu.Button("Save Changes", saveButtonOptions, func() {
		lm.saveEditChanges()
		lm.hideEditSideMenu()
	})

	cancelButtonOptions := DefaultButtonOptions()
	cancelButtonOptions.Height = 35
	cancelButtonOptions.BackgroundColor = color.RGBA{150, 50, 50, 255}
	cancelButtonOptions.HoverColor = color.RGBA{200, 70, 70, 255}
	lm.editSideMenu.Button("Cancel", cancelButtonOptions, func() {
		lm.hideEditSideMenu()
	})
}

// saveEditChanges saves the changes from the edit side menu
func (lm *LoadoutManager) saveEditChanges() {
	if lm.editingIndex < 0 || lm.editingIndex >= len(lm.loadouts) || lm.editingLoadout == nil {
		fmt.Printf("[LOADOUT] Cannot save: editingIndex=%d, loadouts count=%d, editingLoadout nil=%v\n",
			lm.editingIndex, len(lm.loadouts), lm.editingLoadout == nil)
		return
	}

	fmt.Printf("[LOADOUT] Saving loadout with upgrades: %+v\n", lm.editingLoadout.Upgrades)
	fmt.Printf("[LOADOUT] Original loadout upgrades: %+v\n", lm.loadouts[lm.editingIndex].Upgrades)

	// Save the edited loadout back to the list (editingLoadout was modified directly by the sliders)
	lm.loadouts[lm.editingIndex] = *lm.editingLoadout
	lm.saveToFile()

	fmt.Printf("[LOADOUT] Saved loadout upgrades: %+v\n", lm.loadouts[lm.editingIndex].Upgrades)

	NewToast().
		Text("Loadout saved successfully", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// HasTextInputFocused returns true if the name input is currently focused
func (lm *LoadoutManager) HasTextInputFocused() bool {
	return lm.nameInput.Focused
}

// drawApplyModeUI draws the UI for loadout application mode
func (lm *LoadoutManager) drawApplyModeUI(screen *ebiten.Image) {
	screenW, _ := ebiten.WindowSize()

	// Draw overlay banner (similar to claim editing UI)
	overlayHeight := 140
	overlayColor := color.RGBA{40, 40, 60, 200}
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(overlayHeight), overlayColor, false)

	// Draw border at bottom of overlay
	borderColor := color.RGBA{100, 150, 255, 255}
	vector.DrawFilledRect(screen, 0, float32(overlayHeight-3), float32(screenW), 3, borderColor, false)

	// Title
	titleFont := loadWynncraftFont(24)
	titleText := "Loadout Application"
	titleBounds := text.BoundString(titleFont, titleText)
	titleX := (screenW - titleBounds.Dx()) / 2
	text.Draw(screen, titleText, titleFont, titleX, 35, color.RGBA{255, 255, 255, 255})

	// Subtitle with loadout name
	subtitleFont := loadWynncraftFont(18)
	subtitleText := fmt.Sprintf("Applying: %s", lm.applyingLoadoutName)
	subtitleBounds := text.BoundString(subtitleFont, subtitleText)
	subtitleX := (screenW - subtitleBounds.Dx()) / 2
	text.Draw(screen, subtitleText, subtitleFont, subtitleX, 60, color.RGBA{200, 220, 255, 255})

	// Instructions
	instructionFont := loadWynncraftFont(14)
	instructionText := "Click territories to select/deselect. Use Ctrl+drag for area selection. Press Enter to apply or Escape to cancel."
	instructionBounds := text.BoundString(instructionFont, instructionText)
	instructionX := (screenW - instructionBounds.Dx()) / 2
	text.Draw(screen, instructionText, instructionFont, instructionX, 85, color.RGBA{180, 200, 240, 255})

	// Selection count
	selectedCount := 0
	for _, selected := range lm.selectedTerritories {
		if selected {
			selectedCount++
		}
	}
	countText := fmt.Sprintf("Selected: %d territories", selectedCount)
	countBounds := text.BoundString(instructionFont, countText)
	countX := (screenW - countBounds.Dx()) / 2
	text.Draw(screen, countText, instructionFont, countX, 105, color.RGBA{150, 255, 150, 255})

	// Apply button - positioned on the right side like guild claim editing
	buttonWidth := 70
	buttonHeight := 30
	buttonSpacing := 10
	applyButtonX := screenW - (buttonWidth*2 + buttonSpacing + 20) // Same pattern as guild claim editing
	applyButtonY := overlayHeight - 50
	applyButtonColor := color.RGBA{50, 150, 50, 255}
	mx, my := ebiten.CursorPosition()
	if mx >= applyButtonX && mx <= applyButtonX+buttonWidth && my >= applyButtonY && my <= applyButtonY+buttonHeight {
		applyButtonColor = color.RGBA{70, 200, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(applyButtonX), float32(applyButtonY), float32(buttonWidth), float32(buttonHeight), applyButtonColor, false)
	vector.StrokeRect(screen, float32(applyButtonX), float32(applyButtonY), float32(buttonWidth), float32(buttonHeight), 2, color.RGBA{100, 100, 120, 255}, false)

	buttonFont := loadWynncraftFont(14)
	applyText := "Apply"
	applyBounds := text.BoundString(buttonFont, applyText)
	text.Draw(screen, applyText, buttonFont,
		applyButtonX+(buttonWidth-applyBounds.Dx())/2,
		applyButtonY+(buttonHeight+applyBounds.Dy())/2-2,
		color.RGBA{255, 255, 255, 255})

	// Cancel button - positioned next to Apply button on the right
	cancelButtonX := screenW - (buttonWidth + 20) // Same pattern as guild claim editing
	cancelButtonY := overlayHeight - 50
	cancelButtonColor := color.RGBA{150, 50, 50, 255}
	if mx >= cancelButtonX && mx <= cancelButtonX+buttonWidth && my >= cancelButtonY && my <= cancelButtonY+buttonHeight {
		cancelButtonColor = color.RGBA{200, 70, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(cancelButtonX), float32(cancelButtonY), float32(buttonWidth), float32(buttonHeight), cancelButtonColor, false)
	vector.StrokeRect(screen, float32(cancelButtonX), float32(cancelButtonY), float32(buttonWidth), float32(buttonHeight), 2, color.RGBA{100, 100, 120, 255}, false)

	cancelText := "Cancel"
	cancelBounds := text.BoundString(buttonFont, cancelText)
	text.Draw(screen, cancelText, buttonFont,
		cancelButtonX+(buttonWidth-cancelBounds.Dx())/2,
		cancelButtonY+(buttonHeight+cancelBounds.Dy())/2-2,
		color.RGBA{255, 255, 255, 255})
	text.Draw(screen, cancelText, buttonFont,
		cancelButtonX+(70-cancelBounds.Dx())/2,
		cancelButtonY+(30+cancelBounds.Dy())/2-2,
		color.RGBA{255, 255, 255, 255})
}

// updateTerritoryRenderer updates the territory renderer with the current selection
func (lm *LoadoutManager) updateTerritoryRenderer() {
	if !lm.isApplyingLoadout {
		fmt.Printf("[LOADOUT] updateTerritoryRenderer called but not in apply mode\n")
		return
	}

	fmt.Printf("[LOADOUT] updateTerritoryRenderer called with %d selected territories\n", len(lm.selectedTerritories))

	mapView := GetMapView()
	if mapView != nil && mapView.territoriesManager != nil && mapView.territoriesManager.IsLoaded() {
		if renderer := mapView.territoriesManager.GetRenderer(); renderer != nil {
			fmt.Printf("[LOADOUT] Calling SetLoadoutApplicationMode with loadout: %s\n", lm.applyingLoadoutName)
			renderer.SetLoadoutApplicationMode(lm.applyingLoadoutName, lm.selectedTerritories)
			// Force a redraw to show the updated highlighting
			if cache := renderer.GetTerritoryCache(); cache != nil {
				fmt.Printf("[LOADOUT] Forcing territory cache redraw\n")
				cache.ForceRedraw()
			} else {
				fmt.Printf("[LOADOUT] Territory cache is nil\n")
			}
		} else {
			fmt.Printf("[LOADOUT] Territory renderer is nil\n")
		}
	} else {
		fmt.Printf("[LOADOUT] Map view or territories manager not available\n")
	}
}

// addUpgradeSlider adds a custom upgrade slider that directly modifies the loadout value
func (lm *LoadoutManager) addUpgradeSlider(menu *CollapsibleMenu, name string, valuePtr *int) {
	// Upgrades always have max level 11 (like in the original UpgradeControl)
	minLevel := 0
	maxLevel := 11

	sliderOptions := DefaultSliderOptions()
	sliderOptions.MinValue = float64(minLevel)
	sliderOptions.MaxValue = float64(maxLevel)
	sliderOptions.Step = 1
	sliderOptions.ShowValue = true
	sliderOptions.ValueFormat = "%.0f"

	menu.Slider(name, float64(*valuePtr), sliderOptions, func(value float64) {
		*valuePtr = int(value)
		fmt.Printf("[LOADOUT] Updated %s to %d\n", name, *valuePtr)
	})
}

// addBonusSlider adds a custom bonus slider that directly modifies the loadout value
func (lm *LoadoutManager) addBonusSlider(menu *CollapsibleMenu, name, bonusType string, valuePtr *int) {
	// Get max level from costs (like in the original BonusControl)
	costs := eruntime.GetCost()
	maxLevel := getBonusMaxLevel(costs, bonusType)

	sliderOptions := DefaultSliderOptions()
	sliderOptions.MinValue = 0
	sliderOptions.MaxValue = float64(maxLevel)
	sliderOptions.Step = 1
	sliderOptions.ShowValue = true
	sliderOptions.ValueFormat = "%.0f"

	menu.Slider(name, float64(*valuePtr), sliderOptions, func(value float64) {
		*valuePtr = int(value)
		fmt.Printf("[LOADOUT] Updated %s to %d\n", name, *valuePtr)
	})
}
