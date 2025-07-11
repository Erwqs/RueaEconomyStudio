package app

import (
	"etools/eruntime"
	"etools/numbers"
	"etools/typedef"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

// TributeMenuState represents the current state of the tribute menu
type TributeMenuState int

const (
	TributeMenuList TributeMenuState = iota
	TributeMenuEdit
)

// TributeMenu manages the tribute system UI
type TributeMenu struct {
	isVisible bool
	state     TributeMenuState

	// List screen data
	selectedTributeIndex int
	scrollOffset         int
	activeTributes       []*typedef.ActiveTribute

	// Edit screen data
	editingTribute  *typedef.ActiveTribute // nil for new tribute
	fromGuildIndex  int
	toGuildIndex    int
	availableGuilds []GuildOption
	resourceAmounts ResourceAmounts

	// Dropdown components
	fromGuildDropdown *FilterableDropdown
	toGuildDropdown   *FilterableDropdown

	// Resource input fields
	emeraldInput *EnhancedTextInput
	oreInput     *EnhancedTextInput
	woodInput    *EnhancedTextInput
	fishInput    *EnhancedTextInput
	cropInput    *EnhancedTextInput

	// UI constants
	menuWidth   int
	menuHeight  int
	lineHeight  int
	scrollSpeed int

	// Button hover states
	closeButtonHovered   bool
	newButtonHovered     bool
	editButtonHovered    map[int]bool
	deleteButtonHovered  map[int]bool
	disableButtonHovered map[int]bool
	backButtonHovered    bool
	saveButtonHovered    bool

	// Input handling flags
	justHandledEscKey bool // Whether we just handled an ESC key
}

// GuildOption represents a guild option in the dropdown
type GuildOption struct {
	Name  string
	Tag   string
	IsNil bool // true for "Nil Guild" option
}

// ResourceAmounts holds the resource amounts for tribute creation/editing
type ResourceAmounts struct {
	Emeralds numbers.FixedPoint128
	Ores     numbers.FixedPoint128
	Wood     numbers.FixedPoint128
	Fish     numbers.FixedPoint128
	Crops    numbers.FixedPoint128
}

// NewTributeMenu creates a new tribute menu
func NewTributeMenu() *TributeMenu {
	tm := &TributeMenu{
		isVisible:            false,
		state:                TributeMenuList,
		selectedTributeIndex: 0,
		scrollOffset:         0,
		menuWidth:            800,
		menuHeight:           600,
		lineHeight:           25,
		scrollSpeed:          3,
		editButtonHovered:    make(map[int]bool),
		deleteButtonHovered:  make(map[int]bool),
		disableButtonHovered: make(map[int]bool),
	}

	// Initialize dropdowns and inputs (will be updated with actual guild data when shown)
	tm.initializeInputs()

	return tm
}

// Show displays the tribute menu
func (tm *TributeMenu) Show() {
	tm.isVisible = true
	tm.state = TributeMenuList
	tm.refreshData()
}

// Hide hides the tribute menu
func (tm *TributeMenu) Hide() {
	tm.isVisible = false
	tm.resetEditState()
}

// IsVisible returns whether the menu is currently visible
func (tm *TributeMenu) IsVisible() bool {
	return tm.isVisible
}

// refreshData refreshes the tribute and guild data
func (tm *TributeMenu) refreshData() {
	// Get active tributes
	tm.activeTributes = eruntime.GetAllActiveTributes()

	// Get available guilds (only those that have territories)
	tm.availableGuilds = tm.getAvailableGuilds()

	// Update dropdown options
	tm.updateDropdownOptions()
}

// getAvailableGuilds returns guilds that have at least one territory
func (tm *TributeMenu) getAvailableGuilds() []GuildOption {
	guilds := []GuildOption{}

	// Add "Nil Sink/Source" option first
	guilds = append(guilds, GuildOption{
		Name:  "Nil Sink/Source",
		Tag:   "_NIL",
		IsNil: true,
	})

	// Get all territories and find guilds with territories
	territories := eruntime.GetTerritories()
	guildSet := make(map[string]*typedef.Guild)

	for _, territory := range territories {
		if territory != nil && territory.Guild.Name != "" && territory.Guild.Name != "No Guild" {
			guildSet[territory.Guild.Tag] = &territory.Guild
		}
	}

	// Convert to sorted slice
	var guildList []*typedef.Guild
	for _, guild := range guildSet {
		guildList = append(guildList, guild)
	}

	// Sort by guild name
	sort.Slice(guildList, func(i, j int) bool {
		return guildList[i].Name < guildList[j].Name
	})

	// Add to options
	for _, guild := range guildList {
		guilds = append(guilds, GuildOption{
			Name:  guild.Name,
			Tag:   guild.Tag,
			IsNil: false,
		})
	}

	return guilds
}

// resetEditState resets the edit screen state
func (tm *TributeMenu) resetEditState() {
	tm.editingTribute = nil
	tm.fromGuildIndex = 0
	tm.toGuildIndex = 0
	tm.resourceAmounts = ResourceAmounts{}
}

// Update handles input and updates the menu state
// Returns true if input was handled and should not be processed by other systems
func (tm *TributeMenu) Update() bool {
	if !tm.isVisible {
		return false
	}

	// Update dropdowns and text inputs for edit screen FIRST
	// This allows them to handle clicks before we consume them
	dropdownHandledInput := false
	if tm.state == TributeMenuEdit {
		mx, my := ebiten.CursorPosition()

		// Update dropdowns first so they can handle clicks
		if tm.fromGuildDropdown != nil {
			if tm.fromGuildDropdown.Update(mx, my) {
				dropdownHandledInput = true
			}
		}
		if tm.toGuildDropdown != nil {
			if tm.toGuildDropdown.Update(mx, my) {
				dropdownHandledInput = true
			}
		}

		// Update resource input fields only if they're not handling special keys
		shouldSkipTextInput := false

		// Check if any dropdown is focused - skip text input to avoid conflicts
		if (tm.fromGuildDropdown != nil && tm.fromGuildDropdown.InputFocused) ||
			(tm.toGuildDropdown != nil && tm.toGuildDropdown.InputFocused) {
			shouldSkipTextInput = true
		}

		if tm.emeraldInput != nil {
			tm.emeraldInput.UpdateWithSkipInput(shouldSkipTextInput)
		}
		if tm.oreInput != nil {
			tm.oreInput.UpdateWithSkipInput(shouldSkipTextInput)
		}
		if tm.woodInput != nil {
			tm.woodInput.UpdateWithSkipInput(shouldSkipTextInput)
		}
		if tm.fishInput != nil {
			tm.fishInput.UpdateWithSkipInput(shouldSkipTextInput)
		}
		if tm.cropInput != nil {
			tm.cropInput.UpdateWithSkipInput(shouldSkipTextInput)
		}
	}

	// Handle mouse clicks after dropdowns have had a chance to handle them
	if !dropdownHandledInput && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		tm.handleClick(mx, my)
		return true // Consume click input
	}

	// Handle right-click to prevent context menus
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return true // Consume right-click
	}

	// Handle middle-click to prevent any middle-click actions
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		return true // Consume middle-click
	}

	// Handle escape key - go back or close (always allow this)
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		tm.handleEscape()
		return true // Consume escape key
	}

	// Handle back mouse button (always allow this)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		tm.handleEscape()
		return true // Consume back button
	}

	// Consume all other keyboard input to prevent hotkeys from triggering
	// This includes T, R, P, L, G and any other keys
	keys := inpututil.AppendJustPressedKeys(nil)
	if len(keys) > 0 {
		return true // Consume any keyboard input
	}

	// Consume mouse wheel to prevent scroll-through
	scrollX, scrollY := ebiten.Wheel()
	if scrollX != 0 || scrollY != 0 {
		return true // Consume wheel input
	}

	// Update hover states
	mx, my := ebiten.CursorPosition()
	tm.updateHoverStates(mx, my)

	// Return true if dropdowns handled input, or always true when visible to prevent input leak
	return dropdownHandledInput || true
}

// handleEscape handles escape key or back button
func (tm *TributeMenu) handleEscape() {
	tm.justHandledEscKey = true // Set flag to indicate we handled ESC
	switch tm.state {
	case TributeMenuList:
		tm.Hide() // Close the menu
	case TributeMenuEdit:
		tm.state = TributeMenuList // Go back to list
		tm.resetEditState()
		tm.refreshData()
	}
}

// editTribute starts editing an existing tribute
func (tm *TributeMenu) editTribute(tribute *typedef.ActiveTribute) {
	tm.state = TributeMenuEdit
	tm.editingTribute = tribute

	// Set guild indices
	tm.fromGuildIndex = tm.findGuildIndex(tribute.FromGuildName)
	tm.toGuildIndex = tm.findGuildIndex(tribute.ToGuildName)

	// Set resource amounts (use AmountPerHour for editing)
	tm.resourceAmounts = ResourceAmounts{
		Emeralds: tribute.AmountPerHour.Emeralds,
		Ores:     tribute.AmountPerHour.Ores,
		Wood:     tribute.AmountPerHour.Wood,
		Fish:     tribute.AmountPerHour.Fish,
		Crops:    tribute.AmountPerHour.Crops,
	}

	// Initialize dropdown selections
	tm.initializeDropdownSelections()
}

// createNewTribute starts creating a new tribute
func (tm *TributeMenu) createNewTribute() {
	tm.state = TributeMenuEdit
	tm.resetEditState()

	// Initialize dropdown selections
	tm.initializeDropdownSelections()
}

// findGuildIndex finds the index of a guild in the available guilds list
func (tm *TributeMenu) findGuildIndex(guildName string) int {
	if guildName == "" {
		return 0 // Nil Guild
	}

	for i, guild := range tm.availableGuilds {
		if guild.Name == guildName {
			return i
		}
	}

	return 0 // Default to Nil Guild if not found
}

// deleteTribute deletes a tribute
func (tm *TributeMenu) deleteTribute(tribute *typedef.ActiveTribute) {
	err := eruntime.DeleteTribute(tribute.ID)
	if err != nil {
		fmt.Printf("Error deleting tribute: %v\n", err)
	} else {
		tm.refreshData()
		if tm.selectedTributeIndex >= len(tm.activeTributes) && tm.selectedTributeIndex > 0 {
			tm.selectedTributeIndex--
		}
	}
}

// toggleTribute toggles a tribute's active state
func (tm *TributeMenu) toggleTribute(tribute *typedef.ActiveTribute) {
	var err error
	if tribute.IsActive {
		err = eruntime.DisableTributeByID(tribute.ID)
	} else {
		err = eruntime.EnableTributeByID(tribute.ID)
	}

	if err != nil {
		fmt.Printf("Error toggling tribute: %v\n", err)
	} else {
		tm.refreshData()
	}
}

// saveTribute saves the current tribute being edited
func (tm *TributeMenu) saveTribute() {
	fromGuildName := ""
	toGuildName := ""

	// Get selected guilds from dropdowns
	if tm.fromGuildDropdown != nil {
		if selected, ok := tm.fromGuildDropdown.GetSelected(); ok {
			if data, ok := selected.Data.(GuildOption); ok && !data.IsNil {
				fromGuildName = data.Name
			}
		} else {
			// If no selection but there's input text, try to find matching guild
			inputText := tm.fromGuildDropdown.GetInputText()
			for _, guild := range tm.availableGuilds {
				displayName := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
				if inputText == displayName && !guild.IsNil {
					fromGuildName = guild.Name
					break
				}
			}
		}
	}

	if tm.toGuildDropdown != nil {
		if selected, ok := tm.toGuildDropdown.GetSelected(); ok {
			if data, ok := selected.Data.(GuildOption); ok && !data.IsNil {
				toGuildName = data.Name
			}
		} else {
			// If no selection but there's input text, try to find matching guild
			inputText := tm.toGuildDropdown.GetInputText()
			for _, guild := range tm.availableGuilds {
				displayName := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
				if inputText == displayName && !guild.IsNil {
					toGuildName = guild.Name
					break
				}
			}
		}
	}

	// Validate input
	if fromGuildName == "" && toGuildName == "" {
		fmt.Println("Error: At least one guild must be specified")
		return
	}

	// Get resource amounts from text inputs
	emeraldAmount := numbers.FixedPoint128{Whole: 0, Fraction: 0}
	oreAmount := numbers.FixedPoint128{Whole: 0, Fraction: 0}
	woodAmount := numbers.FixedPoint128{Whole: 0, Fraction: 0}
	fishAmount := numbers.FixedPoint128{Whole: 0, Fraction: 0}
	cropAmount := numbers.FixedPoint128{Whole: 0, Fraction: 0}

	if tm.emeraldInput != nil {
		if val, err := strconv.ParseFloat(tm.emeraldInput.Value, 64); err == nil {
			emeraldAmount = numbers.NewFixedPointFromFloat(val)
		}
	}
	if tm.oreInput != nil {
		if val, err := strconv.ParseFloat(tm.oreInput.Value, 64); err == nil {
			oreAmount = numbers.NewFixedPointFromFloat(val)
		}
	}
	if tm.woodInput != nil {
		if val, err := strconv.ParseFloat(tm.woodInput.Value, 64); err == nil {
			woodAmount = numbers.NewFixedPointFromFloat(val)
		}
	}
	if tm.fishInput != nil {
		if val, err := strconv.ParseFloat(tm.fishInput.Value, 64); err == nil {
			fishAmount = numbers.NewFixedPointFromFloat(val)
		}
	}
	if tm.cropInput != nil {
		if val, err := strconv.ParseFloat(tm.cropInput.Value, 64); err == nil {
			cropAmount = numbers.NewFixedPointFromFloat(val)
		}
	}

	amount := typedef.BasicResources{
		Emeralds: emeraldAmount,
		Ores:     oreAmount,
		Wood:     woodAmount,
		Fish:     fishAmount,
		Crops:    cropAmount,
	}

	var err error
	if tm.editingTribute != nil {
		// Update existing tribute (delete and recreate for simplicity)
		err = eruntime.DeleteTribute(tm.editingTribute.ID)
		if err != nil {
			fmt.Printf("Error deleting old tribute: %v\n", err)
			return
		}
	}

	// Create new tribute (1 minute interval)
	_, err = eruntime.CreateNewTribute(fromGuildName, toGuildName, amount, 1)
	if err != nil {
		fmt.Printf("Error creating tribute: %v\n", err)
		return
	}

	// Go back to list screen
	tm.state = TributeMenuList
	tm.resetEditState()
	tm.refreshData()
}

// Draw renders the tribute menu
func (tm *TributeMenu) Draw(screen *ebiten.Image) {
	if !tm.isVisible {
		return
	}

	// Draw background
	tm.drawBackground(screen)

	switch tm.state {
	case TributeMenuList:
		tm.drawListScreen(screen)
	case TributeMenuEdit:
		tm.drawEditScreen(screen)
	}
}

// drawBackground draws the menu background
func (tm *TributeMenu) drawBackground(screen *ebiten.Image) {
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()

	// Semi-transparent overlay
	overlayImg := ebiten.NewImage(screenW, screenH)
	overlayImg.Fill(color.RGBA{0, 0, 0, 150})
	screen.DrawImage(overlayImg, nil)

	// Menu background
	menuX := (screenW - tm.menuWidth) / 2
	menuY := (screenH - tm.menuHeight) / 2

	menuImg := ebiten.NewImage(tm.menuWidth, tm.menuHeight)
	menuImg.Fill(color.RGBA{40, 40, 50, 255})

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(menuX), float64(menuY))
	screen.DrawImage(menuImg, op)

	// Draw border
	borderColor := color.RGBA{100, 100, 120, 255}
	tm.drawRect(screen, float32(menuX), float32(menuY), float32(tm.menuWidth), float32(tm.menuHeight), borderColor, true)

	// Draw close button
	closeX := menuX + tm.menuWidth - 35
	closeY := menuY + 10
	closeColor := color.RGBA{200, 50, 50, 255}
	if tm.closeButtonHovered {
		closeColor = color.RGBA{255, 80, 80, 255}
	}
	tm.drawRect(screen, float32(closeX), float32(closeY), 25, 25, closeColor, false)
	tm.drawRect(screen, float32(closeX), float32(closeY), 25, 25, color.RGBA{100, 100, 120, 255}, true)

	closeFont := loadWynncraftFont(18)
	text.Draw(screen, "X", closeFont, closeX+8, closeY+17, color.RGBA{255, 255, 255, 255})
}

// drawRect draws a filled or outlined rectangle
func (tm *TributeMenu) drawRect(screen *ebiten.Image, x, y, width, height float32, col color.RGBA, outline bool) {
	if outline {
		// Draw border (simplified - just draw thin rectangles)
		borderImg := ebiten.NewImage(int(width), 2)
		borderImg.Fill(col)

		// Top border
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(borderImg, op)

		// Bottom border
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y+height-2))
		screen.DrawImage(borderImg, op)

		// Left border
		borderImg = ebiten.NewImage(2, int(height))
		borderImg.Fill(col)
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(borderImg, op)

		// Right border
		op = &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+width-2), float64(y))
		screen.DrawImage(borderImg, op)
	} else {
		// Draw filled rectangle
		rectImg := ebiten.NewImage(int(width), int(height))
		rectImg.Fill(col)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(rectImg, op)
	}
}

// drawListScreen draws the tribute list screen with buttons
func (tm *TributeMenu) drawListScreen(screen *ebiten.Image) {
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	menuX := (screenW - tm.menuWidth) / 2
	menuY := (screenH - tm.menuHeight) / 2

	// Title
	title := "Active Tributes"
	text.Draw(screen, title, loadWynncraftFont(24), menuX+20, menuY+40, color.RGBA{255, 255, 255, 255})

	// Instructions
	instructions := "Create and manage resource tributes between guilds"
	text.Draw(screen, instructions, loadWynncraftFont(16), menuX+20, menuY+65, color.RGBA{200, 200, 200, 255})

	// New button
	newButtonX := menuX + 20
	newButtonY := menuY + 80
	newButtonColor := color.RGBA{50, 150, 50, 255}
	if tm.newButtonHovered {
		newButtonColor = color.RGBA{70, 200, 70, 255}
	}
	tm.drawRect(screen, float32(newButtonX), float32(newButtonY), 80, 30, newButtonColor, false)
	tm.drawRect(screen, float32(newButtonX), float32(newButtonY), 80, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, "New", loadWynncraftFont(16), newButtonX+25, newButtonY+20, color.RGBA{255, 255, 255, 255})

	// Tribute list
	listY := menuY + 130
	itemHeight := 50
	maxVisibleItems := (tm.menuHeight - 150) / itemHeight

	if len(tm.activeTributes) == 0 {
		// No tributes message
		noTributesText := "No active tributes. Click 'New' to create one."
		text.Draw(screen, noTributesText, loadWynncraftFont(18), menuX+50, menuY+200, color.RGBA{150, 150, 150, 255})
		return
	}

	// Draw tribute items
	for i := 0; i < maxVisibleItems && i < len(tm.activeTributes); i++ {
		itemIndex := i + tm.scrollOffset
		if itemIndex >= len(tm.activeTributes) {
			break
		}

		tribute := tm.activeTributes[itemIndex]
		itemY := listY + i*itemHeight

		tm.drawTributeItem(screen, menuX+20, itemY, tm.menuWidth-60, tribute, itemIndex)
	}
}

// drawTributeItem draws a single tribute item with buttons
func (tm *TributeMenu) drawTributeItem(screen *ebiten.Image, x, y, width int, tribute *typedef.ActiveTribute, index int) {
	// Item background
	itemBgColor := color.RGBA{50, 50, 60, 255}
	if index%2 == 1 {
		itemBgColor = color.RGBA{45, 45, 55, 255}
	}
	tm.drawRect(screen, float32(x), float32(y), float32(width), 40, itemBgColor, false)

	// Status color
	statusColor := color.RGBA{0, 255, 0, 255}
	statusText := "Active"
	if !tribute.IsActive {
		statusColor = color.RGBA{255, 100, 100, 255}
		statusText = "Disabled"
	}

	// From/To names
	fromName := tribute.FromGuildName
	if fromName == "" {
		fromName = "[Spawn]"
	}
	toName := tribute.ToGuildName
	if toName == "" {
		toName = "[Sink]"
	}

	// Main tribute info
	tributeText := fmt.Sprintf("%s -> %s | %s", fromName, toName, statusText)
	text.Draw(screen, tributeText, loadWynncraftFont(16), x+10, y+15, statusColor)

	// Resource amounts (show per-minute amounts directly from AmountPerMinute)
	resourceText := fmt.Sprintf("E:%.2f O:%.2f W:%.2f F:%.2f C:%.2f per min",
		tribute.AmountPerMinute.Emeralds, tribute.AmountPerMinute.Ores, tribute.AmountPerMinute.Wood,
		tribute.AmountPerMinute.Fish, tribute.AmountPerMinute.Crops)
	text.Draw(screen, resourceText, loadWynncraftFont(14), x+10, y+30, color.RGBA{180, 180, 180, 255})

	// Edit button
	editButtonX := x + width - 140
	editButtonColor := color.RGBA{70, 100, 150, 255}
	if tm.editButtonHovered[index] {
		editButtonColor = color.RGBA{90, 130, 200, 255}
	}
	tm.drawRect(screen, float32(editButtonX), float32(y+5), 40, 30, editButtonColor, false)
	tm.drawRect(screen, float32(editButtonX), float32(y+5), 40, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, "Edit", loadWynncraftFont(14), editButtonX+10, y+20, color.RGBA{255, 255, 255, 255})

	// Delete button
	deleteButtonX := x + width - 90
	deleteButtonColor := color.RGBA{150, 70, 70, 255}
	if tm.deleteButtonHovered[index] {
		deleteButtonColor = color.RGBA{200, 90, 90, 255}
	}
	tm.drawRect(screen, float32(deleteButtonX), float32(y+5), 40, 30, deleteButtonColor, false)
	tm.drawRect(screen, float32(deleteButtonX), float32(y+5), 40, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, "Del", loadWynncraftFont(14), deleteButtonX+10, y+20, color.RGBA{255, 255, 255, 255})

	// Toggle button
	toggleButtonX := x + width - 40
	toggleButtonColor := color.RGBA{100, 100, 100, 255}
	toggleText := "On"
	if !tribute.IsActive {
		toggleButtonColor = color.RGBA{120, 120, 80, 255}
		toggleText = "Off"
	}
	if tm.disableButtonHovered[index] {
		if tribute.IsActive {
			toggleButtonColor = color.RGBA{120, 120, 120, 255}
		} else {
			toggleButtonColor = color.RGBA{140, 140, 100, 255}
		}
	}
	tm.drawRect(screen, float32(toggleButtonX), float32(y+5), 30, 30, toggleButtonColor, false)
	tm.drawRect(screen, float32(toggleButtonX), float32(y+5), 30, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, toggleText, loadWynncraftFont(12), toggleButtonX+7, y+18, color.RGBA{255, 255, 255, 255})
}

// drawEditScreen draws the tribute edit/create screen with buttons
func (tm *TributeMenu) drawEditScreen(screen *ebiten.Image) {
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	menuX := (screenW - tm.menuWidth) / 2
	menuY := (screenH - tm.menuHeight) / 2

	// Title
	title := "Edit Tribute"
	if tm.editingTribute == nil {
		title = "Create New Tribute"
	}
	text.Draw(screen, title, loadWynncraftFont(24), menuX+20, menuY+40, color.RGBA{255, 255, 255, 255})

	// Instructions
	instructions := "Type or select guild names from dropdown. Set hourly amounts (sent as 1/60th per minute)."
	text.Draw(screen, instructions, loadWynncraftFont(14), menuX+20, menuY+65, color.RGBA{200, 200, 200, 255})

	// Back button
	backButtonX := menuX + 20
	backButtonY := menuY + 80
	backButtonColor := color.RGBA{100, 100, 100, 255}
	if tm.backButtonHovered {
		backButtonColor = color.RGBA{130, 130, 130, 255}
	}
	tm.drawRect(screen, float32(backButtonX), float32(backButtonY), 60, 30, backButtonColor, false)
	tm.drawRect(screen, float32(backButtonX), float32(backButtonY), 60, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, "Back", loadWynncraftFont(16), backButtonX+15, backButtonY+20, color.RGBA{255, 255, 255, 255})

	// Save button
	saveButtonX := menuX + tm.menuWidth - 80
	saveButtonY := menuY + 80
	saveButtonColor := color.RGBA{50, 150, 50, 255}
	if tm.saveButtonHovered {
		saveButtonColor = color.RGBA{70, 200, 70, 255}
	}
	tm.drawRect(screen, float32(saveButtonX), float32(saveButtonY), 60, 30, saveButtonColor, false)
	tm.drawRect(screen, float32(saveButtonX), float32(saveButtonY), 60, 30, color.RGBA{100, 100, 120, 255}, true)
	text.Draw(screen, "Save", loadWynncraftFont(16), saveButtonX+15, saveButtonY+20, color.RGBA{255, 255, 255, 255})

	y := menuY + 130

	// From Guild selection dropdown (draw label, position dropdown, but don't draw it yet)
	text.Draw(screen, "From Guild:", loadWynncraftFont(18), menuX+30, y+10, color.RGBA{255, 255, 255, 255})
	if tm.fromGuildDropdown != nil {
		// Position the dropdown but don't draw it yet
		tm.fromGuildDropdown.X = menuX + 130
		tm.fromGuildDropdown.Y = y - 5
		tm.fromGuildDropdown.SetContainerBounds(image.Rect(menuX, menuY, menuX+tm.menuWidth, menuY+tm.menuHeight))
	}

	y += 50

	// To Guild selection dropdown (draw label, position dropdown, but don't draw it yet)
	text.Draw(screen, "To Guild:", loadWynncraftFont(18), menuX+30, y+10, color.RGBA{255, 255, 255, 255})
	if tm.toGuildDropdown != nil {
		// Position the dropdown but don't draw it yet
		tm.toGuildDropdown.X = menuX + 130
		tm.toGuildDropdown.Y = y - 5
		tm.toGuildDropdown.SetContainerBounds(image.Rect(menuX, menuY, menuX+tm.menuWidth, menuY+tm.menuHeight))
	}

	y += 50

	// Resource amounts section
	text.Draw(screen, "Resource Amounts (per hour):", loadWynncraftFont(18), menuX+30, y, color.RGBA{255, 255, 255, 255})
	y += 30

	// Emerald input
	emeraldLabelY := y
	emeraldInputY := y - 18 // Position input to align with the label
	text.Draw(screen, "Emeralds:", loadWynncraftFont(16), menuX+50, emeraldLabelY, color.RGBA{0, 255, 0, 255})
	if tm.emeraldInput != nil {
		tm.emeraldInput.X = menuX + 130
		tm.emeraldInput.Y = emeraldInputY
		tm.emeraldInput.Draw(screen)

		// Show per-minute amount
		emeraldValue := 0.0
		if val, err := strconv.ParseFloat(tm.emeraldInput.Value, 64); err == nil {
			emeraldValue = val
			perMinuteText := fmt.Sprintf("(%.2f/min)", emeraldValue/60)
			text.Draw(screen, perMinuteText, loadWynncraftFont(14), menuX+240, emeraldLabelY, color.RGBA{150, 150, 150, 255})
		}
	}
	y += 30

	// Ore input
	oreLabelY := y
	oreInputY := y - 18 // Position input to align with the label
	text.Draw(screen, "Ores:", loadWynncraftFont(16), menuX+50, oreLabelY, color.RGBA{180, 180, 180, 255})
	if tm.oreInput != nil {
		tm.oreInput.X = menuX + 130
		tm.oreInput.Y = oreInputY
		tm.oreInput.Draw(screen)

		// Show per-minute amount
		oreValue := 0.0
		if val, err := strconv.ParseFloat(tm.oreInput.Value, 64); err == nil {
			oreValue = val
			perMinuteText := fmt.Sprintf("(%.2f/min)", oreValue/60)
			text.Draw(screen, perMinuteText, loadWynncraftFont(14), menuX+240, oreLabelY, color.RGBA{150, 150, 150, 255})
		}
	}
	y += 30

	// Wood input
	woodLabelY := y
	woodInputY := y - 18 // Position input to align with the label
	text.Draw(screen, "Wood:", loadWynncraftFont(16), menuX+50, woodLabelY, color.RGBA{139, 69, 19, 255})
	if tm.woodInput != nil {
		tm.woodInput.X = menuX + 130
		tm.woodInput.Y = woodInputY
		tm.woodInput.Draw(screen)

		// Show per-minute amount
		woodValue := 0.0
		if val, err := strconv.ParseFloat(tm.woodInput.Value, 64); err == nil {
			woodValue = val
			perMinuteText := fmt.Sprintf("(%.2f/min)", woodValue/60)
			text.Draw(screen, perMinuteText, loadWynncraftFont(14), menuX+240, woodLabelY, color.RGBA{150, 150, 150, 255})
		}
	}
	y += 30

	// Fish input
	fishLabelY := y
	fishInputY := y - 18 // Position input to align with the label
	text.Draw(screen, "Fish:", loadWynncraftFont(16), menuX+50, fishLabelY, color.RGBA{0, 150, 255, 255})
	if tm.fishInput != nil {
		tm.fishInput.X = menuX + 130
		tm.fishInput.Y = fishInputY
		tm.fishInput.Draw(screen)

		// Show per-minute amount
		fishValue := 0.0
		if val, err := strconv.ParseFloat(tm.fishInput.Value, 64); err == nil {
			fishValue = val
			perMinuteText := fmt.Sprintf("(%.2f/min)", fishValue/60)
			text.Draw(screen, perMinuteText, loadWynncraftFont(14), menuX+240, fishLabelY, color.RGBA{150, 150, 150, 255})
		}
	}
	y += 30

	// Crop input
	cropLabelY := y
	cropInputY := y - 18 // Position input to align with the label
	text.Draw(screen, "Crops:", loadWynncraftFont(16), menuX+50, cropLabelY, color.RGBA{255, 255, 0, 255})
	if tm.cropInput != nil {
		tm.cropInput.X = menuX + 130
		tm.cropInput.Y = cropInputY
		tm.cropInput.Draw(screen)

		// Show per-minute amount
		cropValue := 0.0
		if val, err := strconv.ParseFloat(tm.cropInput.Value, 64); err == nil {
			cropValue = val
			perMinuteText := fmt.Sprintf("(%.2f/min)", cropValue/60)
			text.Draw(screen, perMinuteText, loadWynncraftFont(14), menuX+240, cropLabelY, color.RGBA{150, 150, 150, 255})
		}
	}
	y += 40

	// Information about how tributes work
	infoText := "Use Nil Sink/Source to spawn in/sink resources."
	text.Draw(screen, infoText, loadWynncraftFont(14), menuX+30, y, color.RGBA{180, 180, 180, 255})

	// NOW Draw dropdowns LAST so they appear on top of everything else
	// Draw the non-focused dropdown first, then the focused one on top
	if tm.fromGuildDropdown != nil && tm.toGuildDropdown != nil {
		// Determine which dropdown should be drawn on top (the focused/open one)
		if tm.toGuildDropdown.InputFocused || tm.toGuildDropdown.IsOpen {
			// Draw "From" dropdown first, then "To" dropdown on top
			tm.fromGuildDropdown.Draw(screen)
			tm.toGuildDropdown.Draw(screen)
		} else {
			// Draw "To" dropdown first, then "From" dropdown on top
			tm.toGuildDropdown.Draw(screen)
			tm.fromGuildDropdown.Draw(screen)
		}
	} else {
		// Fallback: draw any existing dropdowns
		if tm.fromGuildDropdown != nil {
			tm.fromGuildDropdown.Draw(screen)
		}
		if tm.toGuildDropdown != nil {
			tm.toGuildDropdown.Draw(screen)
		}
	}
}

// handleClick handles mouse clicks on various UI elements
func (tm *TributeMenu) handleClick(mx, my int) {
	screenW, screenH := ebiten.WindowSize()
	menuX := (screenW - tm.menuWidth) / 2
	menuY := (screenH - tm.menuHeight) / 2

	// Close button
	closeX := menuX + tm.menuWidth - 35
	closeY := menuY + 10
	if mx >= closeX && mx <= closeX+25 && my >= closeY && my <= closeY+25 {
		tm.Hide()
		return
	}

	switch tm.state {
	case TributeMenuList:
		tm.handleListClick(mx, my, menuX, menuY)
	case TributeMenuEdit:
		tm.handleEditClick(mx, my, menuX, menuY)
	}
}

// handleListClick handles clicks on the tribute list screen
func (tm *TributeMenu) handleListClick(mx, my, menuX, menuY int) {
	// New button
	newButtonX := menuX + 20
	newButtonY := menuY + 80
	if mx >= newButtonX && mx <= newButtonX+80 && my >= newButtonY && my <= newButtonY+30 {
		tm.createNewTribute()
		return
	}

	// Tribute list items
	listY := menuY + 130
	itemHeight := 50 // Double line height for each tribute
	maxVisibleItems := (tm.menuHeight - 150) / itemHeight

	for i := 0; i < maxVisibleItems && i < len(tm.activeTributes); i++ {
		itemIndex := i + tm.scrollOffset
		if itemIndex >= len(tm.activeTributes) {
			break
		}

		itemY := listY + i*itemHeight
		itemX := menuX + 20 // Match the drawing x coordinate
		itemWidth := tm.menuWidth - 60

		// Edit button - match exact drawing coordinates
		editButtonX := itemX + itemWidth - 140
		if mx >= editButtonX && mx <= editButtonX+40 && my >= itemY+5 && my <= itemY+35 {
			tm.editTribute(tm.activeTributes[itemIndex])
			return
		}

		// Delete button - match exact drawing coordinates
		deleteButtonX := itemX + itemWidth - 90
		if mx >= deleteButtonX && mx <= deleteButtonX+40 && my >= itemY+5 && my <= itemY+35 {
			tm.deleteTribute(tm.activeTributes[itemIndex])
			return
		}

		// Toggle button - match exact drawing coordinates
		toggleButtonX := itemX + itemWidth - 40
		if mx >= toggleButtonX && mx <= toggleButtonX+30 && my >= itemY+5 && my <= itemY+35 {
			tm.toggleTribute(tm.activeTributes[itemIndex])
			return
		}
	}
}

// handleEditClick handles clicks on the tribute edit screen
func (tm *TributeMenu) handleEditClick(mx, my, menuX, menuY int) {
	// Handle text input focus changes first (but don't return early)
	tm.handleTextInputClicks(mx, my, menuX, menuY)

	// Back button
	backButtonX := menuX + 20
	backButtonY := menuY + 80
	if mx >= backButtonX && mx <= backButtonX+60 && my >= backButtonY && my <= backButtonY+30 {
		tm.state = TributeMenuList
		tm.resetEditState()
		tm.refreshData()
		return
	}

	// Save button
	saveButtonX := menuX + tm.menuWidth - 80
	saveButtonY := menuY + 80
	if mx >= saveButtonX && mx <= saveButtonX+60 && my >= saveButtonY && my <= saveButtonY+30 {
		tm.saveTribute()
		return
	}
}

// initializeInputs creates and configures the dropdown and text input components
func (tm *TributeMenu) initializeInputs() {
	// From Guild dropdown
	tm.fromGuildDropdown = NewFilterableDropdown(0, 0, 300, 30, []FilterableDropdownOption{}, func(option FilterableDropdownOption) {
		// Find the guild index in availableGuilds
		for i, guild := range tm.availableGuilds {
			if guild.Tag == option.Value {
				tm.fromGuildIndex = i
				break
			}
		}
	})

	// To Guild dropdown
	tm.toGuildDropdown = NewFilterableDropdown(0, 0, 300, 30, []FilterableDropdownOption{}, func(option FilterableDropdownOption) {
		// Find the guild index in availableGuilds
		for i, guild := range tm.availableGuilds {
			if guild.Tag == option.Value {
				tm.toGuildIndex = i
				break
			}
		}
	})

	// Resource input fields
	tm.emeraldInput = NewEnhancedTextInput("0", 0, 0, 100, 25, 10)
	tm.oreInput = NewEnhancedTextInput("0", 0, 0, 100, 25, 10)
	tm.woodInput = NewEnhancedTextInput("0", 0, 0, 100, 25, 10)
	tm.fishInput = NewEnhancedTextInput("0", 0, 0, 100, 25, 10)
	tm.cropInput = NewEnhancedTextInput("0", 0, 0, 100, 25, 10)
}

// updateDropdownOptions updates the dropdown options with current guild data
func (tm *TributeMenu) updateDropdownOptions() {
	// Convert guild options to dropdown options
	options := make([]FilterableDropdownOption, len(tm.availableGuilds))
	for i, guild := range tm.availableGuilds {
		displayName := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
		options[i] = FilterableDropdownOption{
			Display: displayName,
			Value:   guild.Tag,
			Data:    guild,
		}
	}

	// Update both dropdowns
	if tm.fromGuildDropdown != nil {
		tm.fromGuildDropdown.SetOptions(options)
	}
	if tm.toGuildDropdown != nil {
		tm.toGuildDropdown.SetOptions(options)
	}
}

// initializeDropdownSelections sets the dropdown selections based on current guild indices
func (tm *TributeMenu) initializeDropdownSelections() {
	if tm.fromGuildDropdown != nil && tm.fromGuildIndex >= 0 && tm.fromGuildIndex < len(tm.availableGuilds) {
		guild := tm.availableGuilds[tm.fromGuildIndex]
		displayName := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
		tm.fromGuildDropdown.SetInputText(displayName)
	}

	if tm.toGuildDropdown != nil && tm.toGuildIndex >= 0 && tm.toGuildIndex < len(tm.availableGuilds) {
		guild := tm.availableGuilds[tm.toGuildIndex]
		displayName := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
		tm.toGuildDropdown.SetInputText(displayName)
	}

	// Initialize text input values with current resource amounts
	if tm.emeraldInput != nil {
		tm.emeraldInput.Value = fmt.Sprintf("%.0f", tm.resourceAmounts.Emeralds)
	}
	if tm.oreInput != nil {
		tm.oreInput.Value = fmt.Sprintf("%.0f", tm.resourceAmounts.Ores)
	}
	if tm.woodInput != nil {
		tm.woodInput.Value = fmt.Sprintf("%.0f", tm.resourceAmounts.Wood)
	}
	if tm.fishInput != nil {
		tm.fishInput.Value = fmt.Sprintf("%.0f", tm.resourceAmounts.Fish)
	}
	if tm.cropInput != nil {
		tm.cropInput.Value = fmt.Sprintf("%.0f", tm.resourceAmounts.Crops)
	}
}

// updateHoverStates updates hover states for UI elements
func (tm *TributeMenu) updateHoverStates(mx, my int) {
	screenW, screenH := ebiten.WindowSize()
	menuX := (screenW - tm.menuWidth) / 2
	menuY := (screenH - tm.menuHeight) / 2

	// Close button hover
	closeX := menuX + tm.menuWidth - 35
	closeY := menuY + 10
	tm.closeButtonHovered = mx >= closeX && mx <= closeX+25 && my >= closeY && my <= closeY+25

	switch tm.state {
	case TributeMenuList:
		tm.updateListHoverStates(mx, my, menuX, menuY)
	case TributeMenuEdit:
		tm.updateEditHoverStates(mx, my, menuX, menuY)
	}
}

// updateListHoverStates updates hover states for the list screen
func (tm *TributeMenu) updateListHoverStates(mx, my, menuX, menuY int) {
	// New button hover
	newButtonX := menuX + 20
	newButtonY := menuY + 80
	tm.newButtonHovered = mx >= newButtonX && mx <= newButtonX+80 && my >= newButtonY && my <= newButtonY+30

	// Reset all button hover states
	for i := range tm.editButtonHovered {
		tm.editButtonHovered[i] = false
		tm.deleteButtonHovered[i] = false
		tm.disableButtonHovered[i] = false
	}

	// Tribute list item hovers
	listY := menuY + 130
	itemHeight := 50
	maxVisibleItems := (tm.menuHeight - 150) / itemHeight

	for i := 0; i < maxVisibleItems && i < len(tm.activeTributes); i++ {
		itemIndex := i + tm.scrollOffset
		if itemIndex >= len(tm.activeTributes) {
			break
		}

		itemY := listY + i*itemHeight
		itemX := menuX + 20 // Match the drawing x coordinate
		itemWidth := tm.menuWidth - 60

		// Edit button hover - match exact drawing coordinates
		editButtonX := itemX + itemWidth - 140
		tm.editButtonHovered[itemIndex] = mx >= editButtonX && mx <= editButtonX+40 && my >= itemY+5 && my <= itemY+35

		// Delete button hover - match exact drawing coordinates
		deleteButtonX := itemX + itemWidth - 90
		tm.deleteButtonHovered[itemIndex] = mx >= deleteButtonX && mx <= deleteButtonX+40 && my >= itemY+5 && my <= itemY+35

		// Toggle button hover - match exact drawing coordinates
		toggleButtonX := itemX + itemWidth - 40
		tm.disableButtonHovered[itemIndex] = mx >= toggleButtonX && mx <= toggleButtonX+30 && my >= itemY+5 && my <= itemY+35
	}
}

// updateEditHoverStates updates hover states for the edit screen
func (tm *TributeMenu) updateEditHoverStates(mx, my, menuX, menuY int) {
	// Back button hover
	backButtonX := menuX + 20
	backButtonY := menuY + 80
	tm.backButtonHovered = mx >= backButtonX && mx <= backButtonX+60 && my >= backButtonY && my <= backButtonY+30

	// Save button hover
	saveButtonX := menuX + tm.menuWidth - 80
	saveButtonY := menuY + 80
	tm.saveButtonHovered = mx >= saveButtonX && mx <= saveButtonX+60 && my >= saveButtonY && my <= saveButtonY+30
}

// handleTextInputClicks handles focus changes for text input fields
func (tm *TributeMenu) handleTextInputClicks(mx, my, menuX, menuY int) {
	// Check which text input was clicked (based on positions that will be set in drawEditScreen)
	inputWidth := 100
	inputHeight := 25

	// Calculate Y positions to match drawEditScreen exactly
	// Starting Y position: menuY + 130 (title area) + 50 (from guild) + 50 (to guild) + 30 (resource section header)
	baseY := menuY + 130 + 50 + 50 + 30 // menuY + 260

	// These Y positions will match what's set in drawEditScreen (18 pixels above labels)
	emeraldY := baseY - 18    // First resource at menuY + 242
	oreY := baseY + 30 - 18   // Second resource at menuY + 272
	woodY := baseY + 60 - 18  // Third resource at menuY + 302
	fishY := baseY + 90 - 18  // Fourth resource at menuY + 332
	cropY := baseY + 120 - 18 // Fifth resource at menuY + 362
	inputX := menuX + 130

	// Track if any input was clicked
	anyInputClicked := false

	// Check emerald input
	if mx >= inputX && mx <= inputX+inputWidth && my >= emeraldY && my <= emeraldY+inputHeight {
		tm.focusTextInput(tm.emeraldInput)
		anyInputClicked = true
		return
	}

	// Check ore input
	if mx >= inputX && mx <= inputX+inputWidth && my >= oreY && my <= oreY+inputHeight {
		tm.focusTextInput(tm.oreInput)
		anyInputClicked = true
		return
	}

	// Check wood input
	if mx >= inputX && mx <= inputX+inputWidth && my >= woodY && my <= woodY+inputHeight {
		tm.focusTextInput(tm.woodInput)
		anyInputClicked = true
		return
	}

	// Check fish input
	if mx >= inputX && mx <= inputX+inputWidth && my >= fishY && my <= fishY+inputHeight {
		tm.focusTextInput(tm.fishInput)
		anyInputClicked = true
		return
	}

	// Check crop input
	if mx >= inputX && mx <= inputX+inputWidth && my >= cropY && my <= cropY+inputHeight {
		tm.focusTextInput(tm.cropInput)
		anyInputClicked = true
		return
	}

	// If no text input was clicked, unfocus all text inputs
	if !anyInputClicked {
		tm.unfocusAllTextInputs()
	}
}

// focusTextInput focuses a specific text input and unfocuses all others
func (tm *TributeMenu) focusTextInput(targetInput *EnhancedTextInput) {
	// Unfocus all first
	tm.unfocusAllTextInputs()

	// Focus the target and select all text
	if targetInput != nil {
		targetInput.Focused = true
		// Select all text when focusing (ready to be replaced by user)
		if len(targetInput.Value) > 0 {
			targetInput.selStart = 0
			targetInput.selEnd = len(targetInput.Value)
			targetInput.cursorPos = len(targetInput.Value)
		}
	}
}

// unfocusAllTextInputs unfocuses all text input fields
func (tm *TributeMenu) unfocusAllTextInputs() {
	if tm.emeraldInput != nil {
		tm.emeraldInput.Focused = false
	}
	if tm.oreInput != nil {
		tm.oreInput.Focused = false
	}
	if tm.woodInput != nil {
		tm.woodInput.Focused = false
	}
	if tm.fishInput != nil {
		tm.fishInput.Focused = false
	}
	if tm.cropInput != nil {
		tm.cropInput.Focused = false
	}
}

// HasTextInputFocused returns true if any text input is currently focused
func (tm *TributeMenu) HasTextInputFocused() bool {
	if !tm.isVisible || tm.state != TributeMenuEdit {
		return false
	}

	// Check if any of the resource input fields are focused
	if tm.emeraldInput != nil && tm.emeraldInput.Focused {
		return true
	}
	if tm.oreInput != nil && tm.oreInput.Focused {
		return true
	}
	if tm.woodInput != nil && tm.woodInput.Focused {
		return true
	}
	if tm.fishInput != nil && tm.fishInput.Focused {
		return true
	}
	if tm.cropInput != nil && tm.cropInput.Focused {
		return true
	}

	// Check if any dropdown has input focused
	if tm.fromGuildDropdown != nil && tm.fromGuildDropdown.InputFocused {
		return true
	}
	if tm.toGuildDropdown != nil && tm.toGuildDropdown.InputFocused {
		return true
	}

	return false
}

// JustHandledEscKey returns whether the tribute menu just handled an ESC key
func (tm *TributeMenu) JustHandledEscKey() bool {
	return tm.justHandledEscKey
}

// ClearEscKeyFlag clears the ESC key handled flag
func (tm *TributeMenu) ClearEscKeyFlag() {
	tm.justHandledEscKey = false
}
