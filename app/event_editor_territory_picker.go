package app

import (
	"etools/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// EventEditorTerritoryPicker handles territory selection for events using the map interface
// Similar to guild claim manager and loadout application territory selection
type EventEditorTerritoryPicker struct {
	active              bool                          // Whether territory picking is active
	eventEditor         *EventEditorGUI               // Reference back to the event editor
	mapView             *MapView                      // Reference to the map view
	selectedTerritories map[string]*typedef.Territory // Selected territories by ID
	isSelecting         bool                          // Whether user is currently area selecting
	selectionStartX     int                           // Start X of area selection
	selectionStartY     int                           // Start Y of area selection
	framesAfterStart    int                           // Frames since territory picking started
}

// NewEventEditorTerritoryPicker creates a new territory picker
func NewEventEditorTerritoryPicker(eventEditor *EventEditorGUI, mapView *MapView) *EventEditorTerritoryPicker {
	return &EventEditorTerritoryPicker{
		active:              false,
		eventEditor:         eventEditor,
		mapView:             mapView,
		selectedTerritories: make(map[string]*typedef.Territory),
		isSelecting:         false,
		framesAfterStart:    0,
	}
}

// StartTerritoryPicking activates territory picking mode
func (tp *EventEditorTerritoryPicker) StartTerritoryPicking() {
	tp.active = true
	tp.framesAfterStart = 0
	tp.selectedTerritories = make(map[string]*typedef.Territory)

	// Copy existing selections from event editor
	if tp.eventEditor != nil {
		selectedIDs := tp.eventEditor.selectedTerritories

		// Find territories by ID from available list
		for _, territory := range tp.eventEditor.availableTerritories {
			if selectedIDs[territory.ID] {
				tp.selectedTerritories[territory.ID] = territory
			}
		}
	}

	// Make event editor go to minimal state
	if tp.eventEditor != nil {
		tp.eventEditor.targetMinimal = true
	}

	// fmt.Printf("[TERRITORY PICKER] Started territory picking with %d pre-selected territories\n", len(tp.selectedTerritories))
}

// StopTerritoryPicking deactivates territory picking mode
func (tp *EventEditorTerritoryPicker) StopTerritoryPicking() {
	tp.active = false
	tp.isSelecting = false

	// Copy selections back to event editor
	if tp.eventEditor != nil {
		selectedIDs := make(map[string]bool)
		for id := range tp.selectedTerritories {
			selectedIDs[id] = true
		}
		tp.eventEditor.selectedTerritories = selectedIDs

		// Restore event editor to normal state
		tp.eventEditor.targetMinimal = false
	}

	// fmt.Printf("[TERRITORY PICKER] Stopped territory picking with %d selected territories\n", len(tp.selectedTerritories))
}

// IsActive returns whether territory picking is currently active
func (tp *EventEditorTerritoryPicker) IsActive() bool {
	return tp.active
}

// GetSelectedTerritories returns the currently selected territories
func (tp *EventEditorTerritoryPicker) GetSelectedTerritories() map[string]*typedef.Territory {
	return tp.selectedTerritories
}

// Update handles input for territory picking
func (tp *EventEditorTerritoryPicker) Update() bool {
	if !tp.active {
		return false
	}

	tp.framesAfterStart++

	// Handle ESC to cancel territory picking (but not immediately after starting)
	if tp.framesAfterStart > 5 && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		tp.StopTerritoryPicking()
		return true
	}

	// Handle Enter to confirm selection
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		tp.StopTerritoryPicking()
		return true
	}

	// Handle mouse input
	mouseX, mouseY := ebiten.CursorPosition()

	// Handle left click for territory selection
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check if Ctrl is held for area selection
		ctrlHeld := ebiten.IsKeyPressed(ebiten.KeyControl) ||
			ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyControlRight)

		if ctrlHeld {
			// Start area selection
			tp.isSelecting = true
			tp.selectionStartX = mouseX
			tp.selectionStartY = mouseY
		} else {
			// Single territory click
			if tp.mapView != nil && tp.mapView.territoriesManager != nil {
				// Use dummy values for now until we can get the proper view parameters
				scale := 1.0
				viewX := 0.0
				viewY := 0.0

				territoryID := tp.mapView.territoriesManager.GetTerritoryAtPosition(mouseX, mouseY, scale, viewX, viewY)
				if territoryID != "" {
					// Find the territory object by ID
					territory := tp.findTerritoryByID(territoryID)
					if territory != nil {
						tp.ToggleTerritorySelection(territory)
					}
				}
			}
		}
		return true
	}

	// Handle area selection dragging
	if tp.isSelecting && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Area selection is active - let the map view handle the visual feedback
		// We'll process the selection when the mouse is released
		return true
	}

	// Handle area selection completion
	if tp.isSelecting && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		tp.isSelecting = false

		// For now, just clear the selection flag
		// TODO: Implement area selection with proper territory detection
		// fmt.Printf("[TERRITORY PICKER] Area selection completed (not yet implemented)\n")
		return true
	}

	return true // Always consume input when territory picking is active
}

// ToggleTerritorySelection toggles the selection state of a territory
func (tp *EventEditorTerritoryPicker) ToggleTerritorySelection(territory *typedef.Territory) {
	if territory == nil {
		return
	}

	if tp.selectedTerritories[territory.ID] != nil {
		delete(tp.selectedTerritories, territory.ID)
		// fmt.Printf("[TERRITORY PICKER] Deselected territory: %s\n", territory.Name)
	} else {
		tp.selectedTerritories[territory.ID] = territory
		// fmt.Printf("[TERRITORY PICKER] Selected territory: %s\n", territory.Name)
	}
}

// AddTerritorySelection adds a territory to the selection
func (tp *EventEditorTerritoryPicker) AddTerritorySelection(territory *typedef.Territory) {
	if territory == nil {
		return
	}

	tp.selectedTerritories[territory.ID] = territory
}

// RemoveTerritorySelection removes a territory from the selection
func (tp *EventEditorTerritoryPicker) RemoveTerritorySelection(territory *typedef.Territory) {
	if territory == nil {
		return
	}

	delete(tp.selectedTerritories, territory.ID)
}

// Draw renders any UI elements for territory picking mode
func (tp *EventEditorTerritoryPicker) Draw(screen *ebiten.Image) {
	if !tp.active {
		return
	}

	// Draw instructions in minimal event editor or as overlay
	// This will be handled by the map view or territory renderer
	// to show selected territories with special highlighting
}

// findTerritoryByID finds a territory by its ID from the available territories
func (tp *EventEditorTerritoryPicker) findTerritoryByID(territoryID string) *typedef.Territory {
	if tp.eventEditor == nil {
		return nil
	}

	for _, territory := range tp.eventEditor.availableTerritories {
		if territory.ID == territoryID {
			return territory
		}
	}
	return nil
}
