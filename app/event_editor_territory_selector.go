package app

import (
	"etools/typedef"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// EventEditorTerritorySelector handles territory selection for events
type EventEditorTerritorySelector struct {
	visible              bool
	modal                *UIModalExtended
	selectedTerritories  map[string]bool // Selected territories by name
	availableTerritories []*typedef.Territory
	filteredTerritories  []*typedef.Territory
	scrollOffset         int
	hoveredIndex         int
	searchInput          *UITextInputExtended
	framesAfterOpen      int
	eventEditor          *EventEditorGUI // Reference back to the event editor
}

// NewEventEditorTerritorySelector creates a new territory selector
func NewEventEditorTerritorySelector(eventEditor *EventEditorGUI) *EventEditorTerritorySelector {
	screenW, screenH := WebSafeWindowSize()
	modalWidth := 600
	modalHeight := 500
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	modal := NewUIModalExtended("Select Territories for Event", modalX, modalY, modalWidth, modalHeight)
	searchInput := NewUITextInputExtended("Search territories...", modalX+20, modalY+60, modalWidth-40, 40)

	return &EventEditorTerritorySelector{
		visible:             false,
		modal:               modal,
		selectedTerritories: make(map[string]bool),
		scrollOffset:        0,
		hoveredIndex:        -1,
		searchInput:         searchInput,
		framesAfterOpen:     0,
		eventEditor:         eventEditor,
	}
}

// Show displays the territory selector
func (ts *EventEditorTerritorySelector) Show() {
	ts.visible = true
	ts.framesAfterOpen = 0
	ts.updateFilteredTerritories()

	// Copy current selection from event editor
	if ts.eventEditor != nil {
		ts.selectedTerritories = make(map[string]bool)
		for id, selected := range ts.eventEditor.selectedTerritories {
			ts.selectedTerritories[id] = selected
		}
	}
}

// Hide conceals the territory selector
func (ts *EventEditorTerritorySelector) Hide() {
	ts.visible = false
	ts.framesAfterOpen = 0

	// Copy selection back to event editor
	if ts.eventEditor != nil {
		ts.eventEditor.selectedTerritories = make(map[string]bool)
		for id, selected := range ts.selectedTerritories {
			ts.eventEditor.selectedTerritories[id] = selected
		}
	}
}

// IsVisible returns whether the territory selector is currently visible
func (ts *EventEditorTerritorySelector) IsVisible() bool {
	return ts.visible
}

// SetAvailableTerritories sets the list of available territories
func (ts *EventEditorTerritorySelector) SetAvailableTerritories(territories []*typedef.Territory) {
	ts.availableTerritories = territories
	ts.updateFilteredTerritories()
}

// Update handles input and updates the territory selector
func (ts *EventEditorTerritorySelector) Update() bool {
	if !ts.visible {
		return false
	}

	ts.framesAfterOpen++

	// Handle ESC to close (but not immediately after opening)
	if ts.framesAfterOpen > 5 && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ts.Hide()
		return true
	}

	// Update search input
	if ts.searchInput.Update() {
		ts.updateFilteredTerritories()
	}

	// Handle mouse input
	mouseX, mouseY := ebiten.CursorPosition()

	// Check if clicking outside modal
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		modalBounds := ts.modal.GetBounds()
		if mouseX < modalBounds.Min.X || mouseX > modalBounds.Max.X ||
			mouseY < modalBounds.Min.Y || mouseY > modalBounds.Max.Y {
			ts.Hide()
			return true
		}
	}

	// Handle territory list interactions
	ts.handleTerritoryListInput(mouseX, mouseY)

	// Handle modal buttons
	if ts.handleModalButtons(mouseX, mouseY) {
		return true
	}

	// Handle scroll
	_, dy := ebiten.Wheel()
	if dy != 0 {
		ts.scrollOffset -= int(dy * 3)
		if ts.scrollOffset < 0 {
			ts.scrollOffset = 0
		}
		maxScroll := len(ts.filteredTerritories) - 10 // Show 10 territories at once
		if maxScroll < 0 {
			maxScroll = 0
		}
		if ts.scrollOffset > maxScroll {
			ts.scrollOffset = maxScroll
		}
	}

	return true // Input was handled
}

// Draw renders the territory selector
func (ts *EventEditorTerritorySelector) Draw(screen *ebiten.Image) {
	if !ts.visible {
		return
	}

	// Draw modal background
	ts.modal.Draw(screen)

	modalBounds := ts.modal.GetBounds()

	// Draw search input
	ts.searchInput.Draw(screen)

	// Draw territory list
	ts.drawTerritoryList(screen, modalBounds)

	// Draw selection summary
	ts.drawSelectionSummary(screen, modalBounds)

	// Draw buttons
	ts.drawButtons(screen, modalBounds)
}

// updateFilteredTerritories filters territories based on search input
func (ts *EventEditorTerritorySelector) updateFilteredTerritories() {
	searchText := ts.searchInput.GetText()
	ts.filteredTerritories = []*typedef.Territory{}

	for _, territory := range ts.availableTerritories {
		if searchText == "" || containsString(territory.Name, searchText) {
			ts.filteredTerritories = append(ts.filteredTerritories, territory)
		}
	}

	// Reset scroll when filtering changes
	ts.scrollOffset = 0
}

// handleTerritoryListInput handles mouse input for the territory list
func (ts *EventEditorTerritorySelector) handleTerritoryListInput(mouseX, mouseY int) {
	modalBounds := ts.modal.GetBounds()
	listX := modalBounds.Min.X + 20
	listY := modalBounds.Min.Y + 120
	listWidth := modalBounds.Max.X - modalBounds.Min.X - 40
	itemHeight := 30
	maxVisible := 10

	ts.hoveredIndex = -1

	for i := 0; i < maxVisible && (i+ts.scrollOffset) < len(ts.filteredTerritories); i++ {
		itemY := listY + i*itemHeight
		if mouseX >= listX && mouseX <= listX+listWidth &&
			mouseY >= itemY && mouseY <= itemY+itemHeight {
			ts.hoveredIndex = i + ts.scrollOffset

			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				territory := ts.filteredTerritories[ts.hoveredIndex]
				// Toggle selection
				if ts.selectedTerritories[territory.ID] {
					delete(ts.selectedTerritories, territory.ID)
				} else {
					ts.selectedTerritories[territory.ID] = true
				}
			}
			break
		}
	}
}

// handleModalButtons handles modal button clicks
func (ts *EventEditorTerritorySelector) handleModalButtons(mouseX, mouseY int) bool {
	modalBounds := ts.modal.GetBounds()
	buttonWidth := 80
	buttonHeight := 30
	buttonY := modalBounds.Max.Y - 50

	// OK button
	okButtonX := modalBounds.Max.X - 180
	if mouseX >= okButtonX && mouseX <= okButtonX+buttonWidth &&
		mouseY >= buttonY && mouseY <= buttonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			ts.Hide() // This copies selection back to event editor
			return true
		}
	}

	// Cancel button
	cancelButtonX := modalBounds.Max.X - 90
	if mouseX >= cancelButtonX && mouseX <= cancelButtonX+buttonWidth &&
		mouseY >= buttonY && mouseY <= buttonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Don't copy selection back
			ts.visible = false
			return true
		}
	}

	// Select All button
	selectAllButtonX := modalBounds.Min.X + 20
	if mouseX >= selectAllButtonX && mouseX <= selectAllButtonX+buttonWidth &&
		mouseY >= buttonY && mouseY <= buttonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			for _, territory := range ts.filteredTerritories {
				ts.selectedTerritories[territory.ID] = true
			}
			return true
		}
	}

	// Clear All button
	clearAllButtonX := modalBounds.Min.X + 110
	if mouseX >= clearAllButtonX && mouseX <= clearAllButtonX+buttonWidth &&
		mouseY >= buttonY && mouseY <= buttonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			ts.selectedTerritories = make(map[string]bool)
			return true
		}
	}

	return false
}

// drawTerritoryList draws the scrollable territory list
func (ts *EventEditorTerritorySelector) drawTerritoryList(screen *ebiten.Image, modalBounds image.Rectangle) {
	listX := modalBounds.Min.X + 20
	listY := modalBounds.Min.Y + 120
	listWidth := modalBounds.Max.X - modalBounds.Min.X - 40
	itemHeight := 30
	maxVisible := 10

	// Draw list background
	listHeight := itemHeight * maxVisible
	vector.DrawFilledRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight),
		color.RGBA{30, 30, 30, 255}, false)
	vector.StrokeRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight),
		1, color.RGBA{100, 100, 100, 255}, false)

	// Draw territories
	for i := 0; i < maxVisible && (i+ts.scrollOffset) < len(ts.filteredTerritories); i++ {
		territory := ts.filteredTerritories[i+ts.scrollOffset]
		itemY := listY + i*itemHeight

		// Determine colors
		var bgColor color.RGBA
		if ts.selectedTerritories[territory.ID] {
			bgColor = color.RGBA{50, 100, 200, 255} // Selected
		} else if i+ts.scrollOffset == ts.hoveredIndex {
			bgColor = color.RGBA{60, 60, 60, 255} // Hovered
		} else {
			bgColor = color.RGBA{40, 40, 40, 255} // Normal
		}

		// Draw item background
		vector.DrawFilledRect(screen, float32(listX), float32(itemY), float32(listWidth), float32(itemHeight),
			bgColor, false)

		// Draw territory name
		textColor := color.RGBA{255, 255, 255, 255}
		text.Draw(screen, territory.Name, loadWynncraftFont(16), listX+10, itemY+20, textColor)

		// Draw selection checkbox
		checkboxSize := 16
		checkboxX := listX + listWidth - 30
		checkboxY := itemY + (itemHeight-checkboxSize)/2

		checkboxColor := color.RGBA{100, 100, 100, 255}
		if ts.selectedTerritories[territory.ID] {
			checkboxColor = color.RGBA{50, 200, 50, 255}
		}

		vector.DrawFilledRect(screen, float32(checkboxX), float32(checkboxY), float32(checkboxSize), float32(checkboxSize),
			checkboxColor, false)
		vector.StrokeRect(screen, float32(checkboxX), float32(checkboxY), float32(checkboxSize), float32(checkboxSize),
			1, color.RGBA{200, 200, 200, 255}, false)
	}

	// Draw scrollbar if needed
	if len(ts.filteredTerritories) > maxVisible {
		ts.drawScrollbar(screen, listX+listWidth-10, listY, 10, listHeight, maxVisible)
	}
}

// drawScrollbar draws a scrollbar for the territory list
func (ts *EventEditorTerritorySelector) drawScrollbar(screen *ebiten.Image, x, y, width, height, maxVisible int) {
	// Scrollbar background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height),
		color.RGBA{20, 20, 20, 255}, false)

	// Scrollbar thumb
	totalItems := len(ts.filteredTerritories)
	thumbHeight := (height * maxVisible) / totalItems
	if thumbHeight < 20 {
		thumbHeight = 20
	}

	thumbY := y + (height-thumbHeight)*ts.scrollOffset/(totalItems-maxVisible)

	vector.DrawFilledRect(screen, float32(x), float32(thumbY), float32(width), float32(thumbHeight),
		color.RGBA{100, 100, 100, 255}, false)
}

// drawSelectionSummary draws the selection summary
func (ts *EventEditorTerritorySelector) drawSelectionSummary(screen *ebiten.Image, modalBounds image.Rectangle) {
	summaryY := modalBounds.Max.Y - 80
	selectedCount := len(ts.selectedTerritories)
	totalCount := len(ts.filteredTerritories)

	summaryText := fmt.Sprintf("Selected: %d / %d territories", selectedCount, totalCount)
	textColor := color.RGBA{200, 200, 200, 255}
	text.Draw(screen, summaryText, loadWynncraftFont(16), modalBounds.Min.X+20, summaryY, textColor)
}

// drawButtons draws the modal buttons
func (ts *EventEditorTerritorySelector) drawButtons(screen *ebiten.Image, modalBounds image.Rectangle) {
	buttonWidth := 80
	buttonHeight := 30
	buttonY := modalBounds.Max.Y - 50

	// Button style
	buttonColor := color.RGBA{70, 70, 70, 255}
	buttonBorder := color.RGBA{150, 150, 150, 255}
	textColor := color.RGBA{255, 255, 255, 255}

	// Select All button
	selectAllButtonX := modalBounds.Min.X + 20
	vector.DrawFilledRect(screen, float32(selectAllButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		buttonColor, false)
	vector.StrokeRect(screen, float32(selectAllButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		1, buttonBorder, false)
	text.Draw(screen, "Select All", loadWynncraftFont(14), selectAllButtonX+10, buttonY+20, textColor)

	// Clear All button
	clearAllButtonX := modalBounds.Min.X + 110
	vector.DrawFilledRect(screen, float32(clearAllButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		buttonColor, false)
	vector.StrokeRect(screen, float32(clearAllButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		1, buttonBorder, false)
	text.Draw(screen, "Clear All", loadWynncraftFont(14), clearAllButtonX+15, buttonY+20, textColor)

	// OK button
	okButtonX := modalBounds.Max.X - 180
	okButtonColor := color.RGBA{50, 150, 50, 255} // Green
	vector.DrawFilledRect(screen, float32(okButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		okButtonColor, false)
	vector.StrokeRect(screen, float32(okButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		1, buttonBorder, false)
	text.Draw(screen, "OK", loadWynncraftFont(16), okButtonX+30, buttonY+20, textColor)

	// Cancel button
	cancelButtonX := modalBounds.Max.X - 90
	cancelButtonColor := color.RGBA{150, 50, 50, 255} // Red
	vector.DrawFilledRect(screen, float32(cancelButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		cancelButtonColor, false)
	vector.StrokeRect(screen, float32(cancelButtonX), float32(buttonY), float32(buttonWidth), float32(buttonHeight),
		1, buttonBorder, false)
	text.Draw(screen, "Cancel", loadWynncraftFont(14), cancelButtonX+20, buttonY+20, textColor)
}

// HasTextInputFocused returns whether any text input is focused
func (ts *EventEditorTerritorySelector) HasTextInputFocused() bool {
	return ts.visible && ts.searchInput.IsFocused()
}

// Helper function for case-insensitive contains check
func containsString(str, substr string) bool {
	str = strings.ToLower(str)
	substr = strings.ToLower(substr)
	return strings.Contains(str, substr)
}
