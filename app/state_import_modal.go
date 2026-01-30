package app

import (
	"RueaES/eruntime"
	"fmt"
	"image/color"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// StateImportOption represents an import option with dependencies
type StateImportOption struct {
	ID           string
	Label        string
	Description  string
	Dependencies []string // IDs of options that must be selected if this is selected
	Dependents   []string // IDs of options that depend on this one
}

// StateImportModal handles the selective state import dialog
type StateImportModal struct {
	visible        bool
	modalX, modalY int
	modalW, modalH int
	filePath       string
	fileVersion    string

	// UI elements
	checkboxes      map[string]*Checkbox
	options         []StateImportOption
	titleFont       font.Face
	contentFont     font.Face
	descriptionFont font.Face

	// Button states
	importButton     Rect
	cancelButton     Rect
	everythingButton Rect
	hoveredButton    string

	// Callbacks
	onImport func(selectedOptions map[string]bool, filePath string)
	onCancel func()
}

// StateImportOptions defines all available import options with their dependencies
var StateImportOptions = []StateImportOption{
	{
		ID:           "core",
		Label:        "Core",
		Description:  "State ticks, runtime options and key components",
		Dependencies: []string{},
		Dependents:   []string{},
	},
	{
		ID:           "guilds",
		Label:        "Guilds",
		Description:  "Guild list name, colors and tag",
		Dependencies: []string{},
		Dependents:   []string{"territories", "territory_config", "territory_data", "in_transit"},
	},
	{
		ID:           "territories",
		Label:        "Territories",
		Description:  "Guilds' territories",
		Dependencies: []string{"guilds"},
		Dependents:   []string{"territory_config", "territory_data", "in_transit"},
	},
	{
		ID:           "territory_config",
		Label:        "Territory Configurations",
		Description:  "Upgrades, bonuses, taxes, routing modes, borders",
		Dependencies: []string{"guilds", "territories"},
		Dependents:   []string{"territory_data", "in_transit"},
	},
	{
		ID:           "territory_data",
		Label:        "Territory Data",
		Description:  "Stateful data like resources and treasury",
		Dependencies: []string{"guilds", "territories"},
		Dependents:   []string{"in_transit"},
	},
	{
		ID:           "in_transit",
		Label:        "In Transit Resources",
		Description:  "In transit resources (those resources outside any terr)",
		Dependencies: []string{"guilds", "territories", "territory_config", "territory_data"},
		Dependents:   []string{},
	},
	{
		ID:           "tributes",
		Label:        "Tributes",
		Description:  "Tribute data",
		Dependencies: []string{"guilds", "territories", "territory_data"},
		Dependents:   []string{},
	},
	{
		ID:           "loadouts",
		Label:        "Loadouts",
		Description:  "Merge loadouts from this state file into local loadouts",
		Dependencies: []string{},
		Dependents:   []string{},
	},
}

// NewStateImportModal creates a new state import modal
func NewStateImportModal(titleFont, contentFont, descriptionFont font.Face) *StateImportModal {
	modal := &StateImportModal{
		visible:         false,
		titleFont:       titleFont,
		contentFont:     contentFont,
		descriptionFont: descriptionFont,
		checkboxes:      make(map[string]*Checkbox),
		options:         StateImportOptions,
	}

	return modal
}

// Show displays the modal with the specified file path
func (sim *StateImportModal) Show(filePath string) {
	sim.filePath = filePath
	sim.visible = true

	// Get file version
	version, err := eruntime.GetStateFileInfo(filePath)
	if err != nil {
		sim.fileVersion = "Unknown"
		// fmt.Printf("[STATE] Failed to get file version: %v\n", err)
	} else {
		sim.fileVersion = version
	}

	// Calculate modal dimensions and position
	screenW, screenH := ebiten.WindowSize()
	sim.modalW = 600
	sim.modalH = 500
	sim.modalX = (screenW - sim.modalW) / 2
	sim.modalY = (screenH - sim.modalH) / 2

	// Initialize checkboxes
	sim.initializeCheckboxes()

	// Calculate button positions (now with 3 buttons)
	buttonWidth := 85
	buttonHeight := 35
	buttonSpacing := 10
	buttonY := sim.modalY + sim.modalH - 60

	// Everything button (leftmost)
	sim.everythingButton = Rect{
		X:      sim.modalX + sim.modalW - (buttonWidth*3 + buttonSpacing*2 + 20),
		Y:      buttonY,
		Width:  buttonWidth,
		Height: buttonHeight,
	}

	// Import button (middle)
	sim.importButton = Rect{
		X:      sim.modalX + sim.modalW - (buttonWidth*2 + buttonSpacing + 20),
		Y:      buttonY,
		Width:  buttonWidth,
		Height: buttonHeight,
	}

	// Cancel button (rightmost)
	sim.cancelButton = Rect{
		X:      sim.modalX + sim.modalW - (buttonWidth + 20),
		Y:      buttonY,
		Width:  buttonWidth,
		Height: buttonHeight,
	}
}

// Hide closes the modal
func (sim *StateImportModal) Hide() {
	sim.visible = false
	sim.hoveredButton = ""
}

// IsVisible returns whether the modal is currently visible
func (sim *StateImportModal) IsVisible() bool {
	return sim.visible
}

// SetCallbacks sets the callback functions
func (sim *StateImportModal) SetCallbacks(onImport func(selectedOptions map[string]bool, filePath string), onCancel func()) {
	sim.onImport = onImport
	sim.onCancel = onCancel
}

// initializeCheckboxes sets up the checkboxes for all options
func (sim *StateImportModal) initializeCheckboxes() {
	checkboxSize := 20
	startY := sim.modalY + 80
	lineHeight := 45

	for i, option := range sim.options {
		checkbox := NewCheckbox(
			sim.modalX+30,
			startY+i*lineHeight,
			checkboxSize,
			option.Label,
			sim.contentFont,
		)

		// Set up click handler for dependency management
		optionID := option.ID
		checkbox.SetOnClick(func(checked bool) {
			sim.handleCheckboxClick(optionID, checked)
		})

		sim.checkboxes[option.ID] = checkbox
	}
}

// handleCheckboxClick manages dependencies when a checkbox is clicked
func (sim *StateImportModal) handleCheckboxClick(optionID string, checked bool) {
	option := sim.getOption(optionID)
	if option == nil {
		return
	}

	if checked {
		// When checking an option, also check its dependencies
		for _, depID := range option.Dependencies {
			if checkbox, exists := sim.checkboxes[depID]; exists {
				checkbox.SetChecked(true)
			}
		}
	} else {
		// When unchecking an option, also uncheck its dependents
		for _, depID := range option.Dependents {
			if checkbox, exists := sim.checkboxes[depID]; exists {
				checkbox.SetChecked(false)
			}
		}
	}
}

// getOption returns the option with the specified ID
func (sim *StateImportModal) getOption(id string) *StateImportOption {
	for i := range sim.options {
		if sim.options[i].ID == id {
			return &sim.options[i]
		}
	}
	return nil
}

// Update handles input for the modal
func (sim *StateImportModal) Update() {
	if !sim.visible {
		return
	}

	mouseX, mouseY := ebiten.CursorPosition()
	mousePressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	// Update checkboxes
	for _, checkbox := range sim.checkboxes {
		checkbox.Update(mouseX, mouseY, mousePressed)
	}

	// Update button hover states
	sim.hoveredButton = ""
	if rectContains(mouseX, mouseY, sim.everythingButton) {
		sim.hoveredButton = "import_all"
	} else if rectContains(mouseX, mouseY, sim.importButton) {
		sim.hoveredButton = "import"
	} else if rectContains(mouseX, mouseY, sim.cancelButton) {
		sim.hoveredButton = "cancel"
	}

	// Handle button clicks
	if mousePressed {
		if sim.hoveredButton == "import_all" {
			sim.handleEverything()
		} else if sim.hoveredButton == "import" {
			sim.handleImport()
		} else if sim.hoveredButton == "cancel" {
			sim.handleCancel()
		}
	}

	// Handle Escape key
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		sim.handleCancel()
	}
}

// Draw renders the modal
func (sim *StateImportModal) Draw(screen *ebiten.Image) {
	if !sim.visible {
		return
	}

	// Draw background overlay
	screenW, screenH := ebiten.WindowSize()
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH),
		color.RGBA{0, 0, 0, 128}, false)

	// Draw modal background
	modalColor := color.RGBA{45, 45, 50, 255}
	vector.DrawFilledRect(screen, float32(sim.modalX), float32(sim.modalY),
		float32(sim.modalW), float32(sim.modalH), modalColor, false)

	// Draw modal border for consistency with other modals
	borderColor := color.RGBA{100, 150, 255, 255}
	vector.StrokeRect(screen, float32(sim.modalX), float32(sim.modalY),
		float32(sim.modalW), float32(sim.modalH), 3, borderColor, false)

	// Draw title bar
	titleBarColor := color.RGBA{60, 120, 180, 255}
	vector.DrawFilledRect(screen, float32(sim.modalX), float32(sim.modalY),
		float32(sim.modalW), 40, titleBarColor, false)

	// Draw title text
	titleText := "Import State Options"
	if sim.filePath != "" {
		titleText = fmt.Sprintf("Import State: %s", filepath.Base(sim.filePath))
	}
	text.Draw(screen, titleText, sim.titleFont, sim.modalX+15, sim.modalY+25, color.White)

	// Draw instruction text
	instructionText := "Select which components to import from the state file:"
	text.Draw(screen, instructionText, sim.contentFont, sim.modalX+20, sim.modalY+60, color.White)

	// Draw checkboxes and descriptions
	for _, option := range sim.options {
		checkbox := sim.checkboxes[option.ID]
		if checkbox != nil {
			checkbox.Draw(screen)

			// Draw description text - aligned with checkbox label and slightly below
			descX := checkbox.X + checkbox.Size + 10    // Align with label
			descY := checkbox.Y + checkbox.Size + 5     // Just below the checkbox/label line
			descColor := color.RGBA{160, 160, 160, 255} // Slightly darker gray
			text.Draw(screen, option.Description, sim.descriptionFont,
				descX, descY, descColor)
		}
	}

	// Draw file version at bottom left
	versionText := fmt.Sprintf("File Version: %s", sim.fileVersion)
	versionColor := color.RGBA{160, 160, 160, 255}
	versionY := sim.modalY + sim.modalH - 25
	text.Draw(screen, versionText, sim.descriptionFont, sim.modalX+20, versionY, versionColor)

	// Draw buttons
	sim.drawButton(screen, sim.everythingButton, "Import All", sim.hoveredButton == "import_all")
	sim.drawButton(screen, sim.importButton, "Import", sim.hoveredButton == "import")
	sim.drawButton(screen, sim.cancelButton, "Cancel", sim.hoveredButton == "cancel")
}

// drawButton draws a button with the specified state
func (sim *StateImportModal) drawButton(screen *ebiten.Image, rect Rect, label string, hovered bool) {
	buttonColor := color.RGBA{70, 130, 180, 255}
	textColor := color.White

	if hovered {
		buttonColor = color.RGBA{90, 150, 200, 255}
	}

	// Draw button background
	vector.DrawFilledRect(screen, float32(rect.X), float32(rect.Y),
		float32(rect.Width), float32(rect.Height), buttonColor, false)

	// Draw button border for consistency with other modals
	borderColor := color.RGBA{80, 80, 80, 255}
	vector.StrokeRect(screen, float32(rect.X), float32(rect.Y),
		float32(rect.Width), float32(rect.Height), 2, borderColor, false)

	// Draw button text (centered)
	textBounds := text.BoundString(sim.contentFont, label)
	textX := rect.X + (rect.Width-textBounds.Dx())/2
	textY := rect.Y + (rect.Height+textBounds.Dy())/2
	text.Draw(screen, label, sim.contentFont, textX, textY, textColor)
}

// handleImport processes the import request
func (sim *StateImportModal) handleImport() {
	selectedOptions := make(map[string]bool)

	for id, checkbox := range sim.checkboxes {
		selectedOptions[id] = checkbox.IsChecked()
	}

	if sim.onImport != nil {
		sim.onImport(selectedOptions, sim.filePath)
	}

	sim.Hide()
}

// handleEverything selects all options and imports everything
func (sim *StateImportModal) handleEverything() {
	// Select all checkboxes
	for _, checkbox := range sim.checkboxes {
		checkbox.SetChecked(true)
	}

	// Create selectedOptions map with all options set to true
	selectedOptions := make(map[string]bool)
	for _, option := range sim.options {
		selectedOptions[option.ID] = true
	}

	if sim.onImport != nil {
		sim.onImport(selectedOptions, sim.filePath)
	}

	sim.Hide()
}

// handleCancel processes the cancel request
func (sim *StateImportModal) handleCancel() {
	if sim.onCancel != nil {
		sim.onCancel()
	}

	sim.Hide()
}

// rectContains checks if a point is within a Rect value.
func rectContains(x, y int, rect Rect) bool {
	return x >= rect.X && x < rect.X+rect.Width &&
		y >= rect.Y && y < rect.Y+rect.Height
}
