package app

import (
	eventeditor "RueaES/event_editor"
	"RueaES/typedef"
	"fmt"
	"image/color"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
)

// TimelineEvent represents an event that can be placed on the timeline
type TimelineEvent struct {
	ID                  int                     // Unique identifier
	Name                string                  // Display name
	Tick                int                     // Position on timeline
	Duration            int                     // Duration in ticks (for future use)
	Description         string                  // Event description
	BackendEvent        *eventeditor.Event      // Reference to the backend event
	EventType           eventeditor.TriggerType // Type of trigger
	AffectedTerritories []*typedef.Territory    // Territories affected by this event
}

// EventEditorGUI represents the event editor interface
type EventEditorGUI struct {
	visible         bool
	x, y            int
	width, height   int
	timelineHeight  int
	playheadTick    int     // Current playhead position in ticks
	maxTicks        int     // Maximum number of ticks on timeline
	timelineScale   float64 // Zoom level of timel		panelHeight = e.height - EventEditorPadding*2 // Full height with top and bottom paddingne
	timelineOffset  int     // Horizontal scroll offset in ticks
	isDraggingHead  bool    // Is the playhead being dragged
	isDraggingBar   bool    // Is the timeline being dragged for scrolling
	font            font.Face
	titleFont       font.Face
	smallFont       font.Face // Smaller font for labels
	customTickInput string    // Input for custom tick amount

	// Selection functionality
	isSelecting          bool // Is the user currently selecting an area
	selectionStart       int  // Start tick of selection
	selectionEnd         int  // End tick of selection
	selectionStartX      int  // Mouse X position where selection started
	hasTimelineSelection bool // Whether there's an active timeline selection

	// Editor Area layout
	editorMinimal bool // Whether the editor area is in minimal view

	// Animation for show/hide
	animationPhase float64
	animationSpeed float64
	targetVisible  bool

	// Animation for layout transition
	layoutAnimationPhase float64 // 0.0 = maximized, 1.0 = minimal
	layoutAnimationSpeed float64 // Speed of layout transition
	targetMinimal        bool    // Target state for layout animation

	// Event list scrollable area
	events           []TimelineEvent // List of user-defined events with positions
	eventListScroll  int             // Scroll offset for event list
	selectedEvent    int             // Currently selected event index (-1 for none)
	maxVisibleEvents int             // Maximum number of events visible at once
	eventItemHeight  int             // Height of each event item
	nextEventID      int             // Counter for generating unique event IDs

	// Event list buttons
	buttonHeight int // Height of buttons at bottom of event list

	// Event editing
	editingEventIndex  int       // Index of event being edited (-1 for none)
	editingEventText   string    // Text being edited
	lastClickTime      time.Time // For double-click detection
	lastClickIndex     int       // Index of last clicked event
	cursorPosition     int       // Cursor position in text
	textSelectionStart int       // Start of text selection (-1 if no selection)
	textSelectionEnd   int       // End of text selection (-1 if no selection)
	hoveredButton      string    // Which button is currently hovered ("new", "delete", "import", "export", "")
	pressedButton      string    // Which button is currently pressed

	// Configuration area
	stateTickInput          string // Input for state tick value
	editingStateTick        bool   // Whether the state tick input is being edited
	stateTickCursorPos      int    // Cursor position in state tick input
	stateTickSelectionStart int    // Start of text selection in state tick (-1 if no selection)
	stateTickSelectionEnd   int    // End of text selection in state tick (-1 if no selection)
	selectedEventType       int    // Currently selected event type (0=Guild Change, 1=Territory Update, 2=Resource Edit)
	typeDropdownOpen        bool   // Whether the type dropdown is open
	hoveredTypeOption       int    // Which type option is hovered (-1 for none)

	// Territory selection for events
	territorySelectionOpen bool                          // Whether territory selection is open
	availableTerritories   []*typedef.Territory          // Available territories for selection
	selectedTerritories    map[string]bool               // Selected territory IDs
	territoryScrollOffset  int                           // Scroll offset for territory list
	territorySelector      *EventEditorTerritorySelector // Territory selector modal
	territoryPicker        *EventEditorTerritoryPicker   // Territory picker for map-based selection
}

const (
	TimelineMinHeight     = 80
	TimelineMaxHeight     = 200
	PlayheadWidth         = 3
	PlayheadHandleSize    = 8
	TimelineMarkersHeight = 20
	EventEditorPadding    = 20
)

// NewEventEditorGUI creates a new event editor GUI
func NewEventEditorGUI(mapView *MapView) *EventEditorGUI {
	editor := &EventEditorGUI{
		visible:              false,
		timelineHeight:       120,
		playheadTick:         0,
		maxTicks:             600,
		timelineScale:        1.0,
		timelineOffset:       0,
		isDraggingHead:       false,
		isDraggingBar:        false,
		animationPhase:       0.0,
		animationSpeed:       8.0,
		targetVisible:        false,
		font:                 loadWynncraftFont(20), // Much bigger font
		titleFont:            loadWynncraftFont(32), // Much bigger title font
		smallFont:            loadWynncraftFont(18), // Smaller font for labels
		customTickInput:      "",
		isSelecting:          false,
		selectionStart:       0,
		selectionEnd:         0,
		selectionStartX:      0,
		hasTimelineSelection: false,
		editorMinimal:        false, // Start in maximized view
		layoutAnimationPhase: 0.0,   // Start in maximized state
		layoutAnimationSpeed: 6.0,   // Smooth but quick transition
		targetMinimal:        false, // Start targeting maximized state

		// Initialize event list - start empty
		events:           []TimelineEvent{}, // Start with empty list
		eventListScroll:  0,
		selectedEvent:    -1,
		maxVisibleEvents: 10,
		eventItemHeight:  60, // Increased from 30 to 60 for much bigger font
		nextEventID:      1,  // Start event IDs from 1
		buttonHeight:     35, // Height for buttons at bottom of list
		hoveredButton:    "", // No button hovered initially
		pressedButton:    "", // No button pressed initially

		// Initialize editing state
		editingEventIndex:  -1, // No event being edited initially
		editingEventText:   "",
		lastClickTime:      time.Time{},
		lastClickIndex:     -1,
		cursorPosition:     0,
		textSelectionStart: -1,
		textSelectionEnd:   -1,

		// Initialize configuration area
		stateTickInput:    "0",
		editingStateTick:  false,
		selectedEventType: 0, // Default to Guild Change
		typeDropdownOpen:  false,
		hoveredTypeOption: -1,

		// Initialize territory selection
		territorySelectionOpen: false,
		selectedTerritories:    make(map[string]bool),
		territoryScrollOffset:  0,
	}

	// Initialize territory selector
	editor.territorySelector = NewEventEditorTerritorySelector(editor)

	// Initialize territory picker
	if mapView != nil {
		editor.territoryPicker = NewEventEditorTerritoryPicker(editor, mapView)
	}

	return editor
}

// Show displays the event editor
func (e *EventEditorGUI) Show() {
	e.targetVisible = true
	if !e.visible {
		e.visible = true
		e.animationPhase = 0.0
	}
}

// Hide conceals the event editor
func (e *EventEditorGUI) Hide() {
	e.targetVisible = false
}

// IsVisible returns whether the event editor is currently visible
func (e *EventEditorGUI) IsVisible() bool {
	return e.visible
}

// Update handles input and updates the event editor
func (e *EventEditorGUI) Update(screenW, screenH int) bool {
	// Update dimensions
	e.width = screenW
	e.height = screenH
	e.x = 0
	e.y = 0

	// Update animation
	deltaTime := 1.0 / 60.0 // Assume 60 FPS for smooth animation
	if e.targetVisible && e.animationPhase < 1.0 {
		e.animationPhase += e.animationSpeed * deltaTime
		if e.animationPhase > 1.0 {
			e.animationPhase = 1.0
		}
	} else if !e.targetVisible && e.animationPhase > 0.0 {
		e.animationPhase -= e.animationSpeed * deltaTime
		if e.animationPhase < 0.0 {
			e.animationPhase = 0.0
			e.visible = false
		}
	}

	// Update layout animation
	if e.targetMinimal && e.layoutAnimationPhase < 1.0 {
		e.layoutAnimationPhase += e.layoutAnimationSpeed * deltaTime
		if e.layoutAnimationPhase > 1.0 {
			e.layoutAnimationPhase = 1.0
			e.editorMinimal = true
		}
	} else if !e.targetMinimal && e.layoutAnimationPhase > 0.0 {
		e.layoutAnimationPhase -= e.layoutAnimationSpeed * deltaTime
		if e.layoutAnimationPhase < 0.0 {
			e.layoutAnimationPhase = 0.0
			e.editorMinimal = false
		}
	}

	if !e.visible {
		return false
	}

	// Handle text input for event editing
	e.handleTextEditing()

	// Handle ESC key to close
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && e.editingEventIndex < 0 {
		e.Hide()
		return true
	}

	// Handle mouse back button to close
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		e.Hide()
		return true
	}

	// Handle keyboard input for timeline navigation (only if not editing text)
	// Arrow keys for playhead movement
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		e.playheadTick -= 5 // Move 5 ticks left
		if e.playheadTick < 0 {
			e.playheadTick = 0
		}
	}
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		e.playheadTick += 5 // Move 5 ticks right
		if e.playheadTick > e.maxTicks {
			e.playheadTick = e.maxTicks
		}
	}

	// Home/End keys for quick navigation (only if not editing text)
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		e.playheadTick = 0
	}
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		e.playheadTick = e.maxTicks
	}

	// Page Up/Page Down for timeline scrolling (only if not editing text)
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		scrollAmount := int(50.0 / e.timelineScale) // Scroll by 50 ticks (adjusted for zoom)
		e.timelineOffset -= scrollAmount
		if e.timelineOffset < 0 {
			e.timelineOffset = 0
		}
	}
	if !e.IsTextInputFocused() && inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		scrollAmount := int(50.0 / e.timelineScale) // Scroll by 50 ticks (adjusted for zoom)
		visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
		maxOffset := e.maxTicks - visibleTicks
		if maxOffset < 0 {
			maxOffset = 0
		}
		e.timelineOffset += scrollAmount
		if e.timelineOffset > maxOffset {
			e.timelineOffset = maxOffset
		}
	}

	// Get mouse position
	mouseX, mouseY := ebiten.CursorPosition()

	// Calculate timeline bounds - halve the height as requested
	timelineHeight := e.timelineHeight / 2
	var timelineY int
	if e.editorMinimal {
		// In minimal mode, timeline is at the bottom of the screen
		timelineY = e.height - timelineHeight - EventEditorPadding
	} else {
		// In maximized mode, timeline is below the editor area
		timelineY = e.height - timelineHeight - EventEditorPadding
	}
	timelineX := EventEditorPadding
	// Reserve space for the + button (50px) on the right side
	timelineWidth := e.width - (EventEditorPadding * 2) - 50

	// Handle mouse input for timeline
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton0) {
		// Check if clicking on timeline area
		if mouseY >= timelineY && mouseY <= timelineY+timelineHeight &&
			mouseX >= timelineX && mouseX <= timelineX+timelineWidth {

			// Check if Ctrl is held for area selection
			ctrlHeld := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

			if ctrlHeld {
				// Start area selection
				e.isSelecting = true
				e.selectionStartX = mouseX
				e.hasTimelineSelection = false

				// Calculate current visible range in ticks
				visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
				visibleStart := e.timelineOffset
				visibleEnd := e.timelineOffset + visibleTicks
				if visibleEnd > e.maxTicks {
					visibleEnd = e.maxTicks
				}

				// Calculate start tick for selection
				clickRatio := float64(mouseX-timelineX) / float64(timelineWidth)
				startTick := visibleStart + int(clickRatio*float64(visibleEnd-visibleStart))
				if startTick < 0 {
					startTick = 0
				} else if startTick > e.maxTicks {
					startTick = e.maxTicks
				}
				e.selectionStart = startTick
				e.selectionEnd = startTick
			} else {
				// Normal playhead positioning
				// Clear any existing selection
				e.hasTimelineSelection = false
				e.isSelecting = false

				// Calculate current visible range in ticks
				visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
				visibleStart := e.timelineOffset
				visibleEnd := e.timelineOffset + visibleTicks
				if visibleEnd > e.maxTicks {
					visibleEnd = e.maxTicks
				}

				// Calculate where the click should move the playhead
				clickRatio := float64(mouseX-timelineX) / float64(timelineWidth)
				newTick := visibleStart + int(clickRatio*float64(visibleEnd-visibleStart))
				if newTick < 0 {
					newTick = 0
				} else if newTick > e.maxTicks {
					newTick = e.maxTicks
				}

				// Move playhead to clicked position and start dragging
				e.playheadTick = newTick
				e.isDraggingHead = true
			}
		}

		// Check if clicking on "+" button to extend timeline
		plusButtonSize := 30
		plusButtonX := timelineX + timelineWidth + 10
		plusButtonY := timelineY + timelineHeight/2 - 15
		if mouseX >= plusButtonX && mouseX <= plusButtonX+plusButtonSize &&
			mouseY >= plusButtonY && mouseY <= plusButtonY+plusButtonSize {
			e.maxTicks += 600 // Add 600 ticks by default
		}

		// Check if clicking on toggle button for editor area layout
		// Calculate panel bounds to get toggle button position
		maxPanelX := EventEditorPadding
		maxPanelY := EventEditorPadding
		maxPanelWidth := e.width - (EventEditorPadding * 2)

		minPanelX := EventEditorPadding
		minPanelY := EventEditorPadding
		minPanelWidth := e.width/5 - EventEditorPadding

		// Interpolate between layouts
		t := e.easeInOut(e.layoutAnimationPhase)
		panelX := int(float64(maxPanelX) + t*float64(minPanelX-maxPanelX))
		panelY := int(float64(maxPanelY) + t*float64(minPanelY-maxPanelY))
		panelWidth := int(float64(maxPanelWidth) + t*float64(minPanelWidth-maxPanelWidth))

		toggleButtonSize := 30
		toggleButtonX := panelX + panelWidth - toggleButtonSize - 10
		toggleButtonY := panelY + 10
		if mouseX >= toggleButtonX && mouseX <= toggleButtonX+toggleButtonSize &&
			mouseY >= toggleButtonY && mouseY <= toggleButtonY+toggleButtonSize {
			e.targetMinimal = !e.targetMinimal
		}

		// Check if clicking on event list area
		e.handleEventListClick(mouseX, mouseY)

		// Check if clicking on configuration area
		e.handleConfigurationAreaClick(mouseX, mouseY)
	}

	// Handle right-click on "+" button for custom tick amount
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton1) {
		plusButtonSize := 30
		plusButtonX := timelineX + timelineWidth + 10
		plusButtonY := timelineY + timelineHeight/2 - 15
		if mouseX >= plusButtonX && mouseX <= plusButtonX+plusButtonSize &&
			mouseY >= plusButtonY && mouseY <= plusButtonY+plusButtonSize {
			// TODO: Implement custom tick input dialog
			// For now, just add 600 ticks
			e.maxTicks += 600
		}
	}

	// Handle mouse dragging - process this ALWAYS when mouse is pressed and we started dragging
	if ebiten.IsMouseButtonPressed(ebiten.MouseButton0) && (e.isDraggingHead || e.isSelecting) {
		// Calculate current visible range in ticks
		visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
		visibleStart := e.timelineOffset
		visibleEnd := e.timelineOffset + visibleTicks
		if visibleEnd > e.maxTicks {
			visibleEnd = e.maxTicks
		}

		if e.isSelecting {
			// Handle area selection dragging
			// Calculate current tick for selection end
			clampedMouseX := mouseX
			if clampedMouseX < timelineX {
				clampedMouseX = timelineX
			} else if clampedMouseX > timelineX+timelineWidth {
				clampedMouseX = timelineX + timelineWidth
			}

			mouseRatio := float64(clampedMouseX-timelineX) / float64(timelineWidth)
			endTick := visibleStart + int(mouseRatio*float64(visibleEnd-visibleStart))
			if endTick < 0 {
				endTick = 0
			} else if endTick > e.maxTicks {
				endTick = e.maxTicks
			}

			e.selectionEnd = endTick
			// Don't swap during dragging - let the drawing code handle the direction
			return true
		} else if e.isDraggingHead {
			// Handle playhead dragging with auto-scroll
			// Auto-scroll when dragging near edges (only when zoomed in)
			if e.timelineScale > 1.0 {
				scrollZone := timelineWidth / 20          // 5% of timeline width on each side
				baseScrollSpeed := 10.0 / e.timelineScale // Base speed adjusted for zoom

				// Check if mouse is in left scroll zone and we can scroll left
				if mouseX < timelineX+scrollZone && e.timelineOffset > 0 {
					// Calculate distance from edge (0.0 = at edge, 1.0 = at scroll zone boundary)
					distanceFromEdge := float64(mouseX-timelineX) / float64(scrollZone)
					// Invert so closer to edge = higher speed (0.0 = max speed, 1.0 = min speed)
					speedMultiplier := 1.0 - distanceFromEdge
					// Apply exponential curve for more dramatic effect near edge
					speedMultiplier = speedMultiplier * speedMultiplier
					// Scale from 0.2x to 3.0x speed based on proximity
					finalSpeed := baseScrollSpeed * (0.2 + speedMultiplier*2.8)

					scrollSpeed := int(finalSpeed)
					if scrollSpeed < 1 {
						scrollSpeed = 1
					}

					e.timelineOffset -= scrollSpeed
					if e.timelineOffset < 0 {
						e.timelineOffset = 0
					}
					// Recalculate visible range after scrolling
					visibleStart = e.timelineOffset
					visibleEnd = e.timelineOffset + visibleTicks
					if visibleEnd > e.maxTicks {
						visibleEnd = e.maxTicks
					}
				}

				// Check if mouse is in right scroll zone and we can scroll right
				if mouseX > timelineX+timelineWidth-scrollZone {
					maxOffset := e.maxTicks - visibleTicks
					if maxOffset > 0 && e.timelineOffset < maxOffset {
						// Calculate distance from right edge
						distanceFromRightEdge := float64(timelineX+timelineWidth-mouseX) / float64(scrollZone)
						// Invert so closer to edge = higher speed
						speedMultiplier := 1.0 - distanceFromRightEdge
						// Apply exponential curve for more dramatic effect near edge
						speedMultiplier = speedMultiplier * speedMultiplier
						// Scale from 0.2x to 3.0x speed based on proximity
						finalSpeed := baseScrollSpeed * (0.2 + speedMultiplier*2.8)

						scrollSpeed := int(finalSpeed)
						if scrollSpeed < 1 {
							scrollSpeed = 1
						}

						e.timelineOffset += scrollSpeed
						if e.timelineOffset > maxOffset {
							e.timelineOffset = maxOffset
						}
						// Recalculate visible range after scrolling
						visibleStart = e.timelineOffset
						visibleEnd = e.timelineOffset + visibleTicks
						if visibleEnd > e.maxTicks {
							visibleEnd = e.maxTicks
						}
					}
				}
			}

			// Allow dragging even outside timeline bounds for better UX
			// Clamp mouse position to timeline bounds for position calculation
			clampedMouseX := mouseX
			if clampedMouseX < timelineX {
				clampedMouseX = timelineX
			} else if clampedMouseX > timelineX+timelineWidth {
				clampedMouseX = timelineX + timelineWidth
			}

			// Convert mouse position to timeline position directly
			mouseRatio := float64(clampedMouseX-timelineX) / float64(timelineWidth)

			// Map mouse position to timeline position
			newTick := visibleStart + int(mouseRatio*float64(visibleEnd-visibleStart))
			if newTick < 0 {
				newTick = 0
			} else if newTick > e.maxTicks {
				newTick = e.maxTicks
			}
			e.playheadTick = newTick
			return true
		}
	}

	// Release dragging when mouse button is released
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButton0) {
		if e.isSelecting {
			// Finalize selection if there's a meaningful range
			selectionRange := e.selectionEnd - e.selectionStart
			if selectionRange < 0 {
				selectionRange = -selectionRange
			}
			if selectionRange > 5 { // Minimum 5 ticks to avoid accidental selections
				e.hasTimelineSelection = true
				// Now that selection is finalized, ensure start <= end for consistency
				if e.selectionStart > e.selectionEnd {
					e.selectionStart, e.selectionEnd = e.selectionEnd, e.selectionStart
				}
			} else {
				e.hasTimelineSelection = false
			}
			e.isSelecting = false
		}
		e.isDraggingHead = false
		e.isDraggingBar = false
	}

	// Handle mouse wheel for timeline scrolling and zooming
	if mouseY >= timelineY && mouseY <= timelineY+timelineHeight &&
		mouseX >= timelineX && mouseX <= timelineX+timelineWidth {
		_, wheelY := ebiten.Wheel()
		if wheelY != 0 {
			// Check if Ctrl is held for zooming
			ctrlHeld := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

			if ctrlHeld {
				// Zoom timeline
				oldScale := e.timelineScale
				e.timelineScale += wheelY * 0.1
				if e.timelineScale < 0.5 {
					e.timelineScale = 0.5
				} else if e.timelineScale > 5.0 {
					e.timelineScale = 5.0
				}

				// Adjust offset to zoom around mouse cursor
				if oldScale != e.timelineScale {
					mouseRatio := float64(mouseX-timelineX) / float64(timelineWidth)
					oldVisibleTicks := int(float64(e.maxTicks) / oldScale)
					newVisibleTicks := int(float64(e.maxTicks) / e.timelineScale)
					tickChange := oldVisibleTicks - newVisibleTicks
					e.timelineOffset += int(mouseRatio * float64(tickChange))

					if e.timelineOffset < 0 {
						e.timelineOffset = 0
					}
					maxOffset := e.maxTicks - newVisibleTicks
					if maxOffset < 0 {
						maxOffset = 0
					}
					if e.timelineOffset > maxOffset {
						e.timelineOffset = maxOffset
					}
				}
			} else {
				// Horizontal scrolling
				scrollSpeed := int(20.0 / e.timelineScale) // Slower scrolling when zoomed in
				if scrollSpeed < 1 {
					scrollSpeed = 1
				}
				e.timelineOffset -= int(wheelY) * scrollSpeed

				// Clamp offset to valid range
				visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
				maxOffset := e.maxTicks - visibleTicks
				if maxOffset < 0 {
					maxOffset = 0
				}
				if e.timelineOffset < 0 {
					e.timelineOffset = 0
				} else if e.timelineOffset > maxOffset {
					e.timelineOffset = maxOffset
				}
			}
			return true
		}
	}

	// Handle mouse wheel for event list scrolling
	e.handleEventListScroll(mouseX, mouseY)

	// Update button hover states
	e.updateButtonHover(mouseX, mouseY)

	// Update configuration area hover states
	e.updateConfigurationHover(mouseX, mouseY)

	// Update territory selector
	if e.territorySelector != nil && e.territorySelector.IsVisible() {
		if e.territorySelector.Update() {
			return true // Territory selector consumed input
		}
	}

	// Update territory picker
	if e.territoryPicker != nil && e.territoryPicker.IsActive() {
		if e.territoryPicker.Update() {
			return true // Territory picker consumed input
		}
	}

	// Only consume input if we're interacting with editor components
	// In maximized mode, always consume input to disable map interaction
	// In minimal mode, only consume input when mouse is over editor areas

	if e.layoutAnimationPhase < 0.5 {
		// In maximized mode (or transitioning towards it), consume all input
		return true
	}

	// In minimal mode (or transitioning towards it), only consume input over editor areas
	// Define editor areas that should consume input
	var editorAreas []struct{ x, y, w, h int }

	// Add main panel area
	var panelX, panelY, panelWidth, panelHeight int

	// Calculate interpolated panel dimensions for input detection
	// Calculate maximized layout values
	maxPanelX := EventEditorPadding
	maxPanelY := EventEditorPadding
	maxPanelWidth := e.width - (EventEditorPadding * 2)
	timelineStartY := timelineY - EventEditorPadding
	maxPanelHeight := timelineStartY - EventEditorPadding

	// Calculate minimal layout values
	minPanelX := EventEditorPadding
	minPanelY := EventEditorPadding
	minPanelWidth := e.width/5 - EventEditorPadding
	minPanelHeight := e.height - EventEditorPadding*2

	// Interpolate between layouts
	t := e.easeInOut(e.layoutAnimationPhase)
	panelX = int(float64(maxPanelX) + t*float64(minPanelX-maxPanelX))
	panelY = int(float64(maxPanelY) + t*float64(minPanelY-maxPanelY))
	panelWidth = int(float64(maxPanelWidth) + t*float64(minPanelWidth-maxPanelWidth))
	panelHeight = int(float64(maxPanelHeight) + t*float64(minPanelHeight-maxPanelHeight))
	editorAreas = append(editorAreas, struct{ x, y, w, h int }{panelX, panelY, panelWidth, panelHeight})

	// Add timeline area
	editorAreas = append(editorAreas, struct{ x, y, w, h int }{timelineX, timelineY, timelineWidth, timelineHeight})

	// Add plus button area
	plusButtonSize := 30
	plusButtonX := timelineX + timelineWidth + 10
	plusButtonY := timelineY + timelineHeight/2 - 15
	editorAreas = append(editorAreas, struct{ x, y, w, h int }{plusButtonX, plusButtonY, plusButtonSize, plusButtonSize})

	// Check if mouse is over any editor area
	mouseOverEditor := false
	for _, area := range editorAreas {
		if mouseX >= area.x && mouseX <= area.x+area.w &&
			mouseY >= area.y && mouseY <= area.y+area.h {
			mouseOverEditor = true
			break
		}
	}

	// Only consume input if mouse is over editor areas or we're actively dragging
	return mouseOverEditor || e.isDraggingHead || e.isSelecting
}

// Draw renders the event editor
func (e *EventEditorGUI) Draw(screen *ebiten.Image) {
	if !e.visible || e.animationPhase <= 0.0 {
		return
	}

	// Apply animation scaling
	alpha := uint8(255 * e.animationPhase)

	// Draw semi-transparent background overlay - lighter in minimal mode
	var overlayAlpha uint8
	// Interpolate overlay alpha based on layout animation
	maxOverlayAlpha := 180.0
	minOverlayAlpha := 0.0
	currentOverlayAlpha := maxOverlayAlpha - (e.layoutAnimationPhase * (maxOverlayAlpha - minOverlayAlpha))
	overlayAlpha = uint8(currentOverlayAlpha * e.animationPhase)

	overlayColor := color.RGBA{0, 0, 0, overlayAlpha}
	vector.DrawFilledRect(screen, 0, 0, float32(e.width), float32(e.height), overlayColor, false)

	// Calculate timeline bounds - halve the height as requested
	timelineHeight := e.timelineHeight / 2
	var timelineY int
	if e.editorMinimal {
		// In minimal mode, timeline is at the bottom of the screen
		timelineY = e.height - timelineHeight - EventEditorPadding
	} else {
		// In maximized mode, timeline is below the editor area
		timelineY = e.height - timelineHeight - EventEditorPadding
	}
	timelineX := EventEditorPadding
	// Reserve space for the + button (50px) on the right side
	timelineWidth := e.width - (EventEditorPadding * 2) - 50

	// Draw main panel background - animate between maximized and minimal views
	var panelX, panelY, panelWidth, panelHeight int

	// Calculate maximized layout values
	maxPanelX := EventEditorPadding
	maxPanelY := EventEditorPadding
	maxPanelWidth := e.width - (EventEditorPadding * 2)
	timelineStartY := timelineY - EventEditorPadding
	maxPanelHeight := timelineStartY - EventEditorPadding

	// Calculate minimal layout values
	minPanelX := EventEditorPadding
	minPanelY := EventEditorPadding
	minPanelWidth := e.width/5 - EventEditorPadding
	minPanelHeight := (e.height - EventEditorPadding*2) - 80

	// Interpolate between layouts using easeInOut function for smooth animation
	t := e.easeInOut(e.layoutAnimationPhase)

	panelX = int(float64(maxPanelX) + t*float64(minPanelX-maxPanelX))
	panelY = int(float64(maxPanelY) + t*float64(minPanelY-maxPanelY))
	panelWidth = int(float64(maxPanelWidth) + t*float64(minPanelWidth-maxPanelWidth))
	panelHeight = int(float64(maxPanelHeight) + t*float64(minPanelHeight-maxPanelHeight))

	panelColor := color.RGBA{UIColors.Surface.R, UIColors.Surface.G, UIColors.Surface.B, alpha}
	vector.DrawFilledRect(screen, float32(panelX), float32(panelY), float32(panelWidth), float32(panelHeight), panelColor, false)

	// Draw panel border
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}
	vector.StrokeRect(screen, float32(panelX), float32(panelY), float32(panelWidth), float32(panelHeight), 2, borderColor, false)

	// Draw title
	titleColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
	titleText := "Event Editor"
	titleWidth := font.MeasureString(e.titleFont, titleText).Round()
	titleX := panelX + (panelWidth-titleWidth)/2
	titleY := panelY + e.titleFont.Metrics().Ascent.Round() + 10
	text.Draw(screen, titleText, e.titleFont, titleX, titleY, titleColor)

	// Draw event list in the editor panel
	e.drawEventList(screen, panelX, panelY, panelWidth, panelHeight, alpha)

	// Draw configuration area when maximized
	e.drawConfigurationArea(screen, panelX, panelY, panelWidth, panelHeight, alpha)

	// Draw toggle button for editor area layout
	e.drawToggleButton(screen, alpha)

	// Draw timeline background
	timelineColor := color.RGBA{UIColors.Background.R, UIColors.Background.G, UIColors.Background.B, alpha}
	vector.DrawFilledRect(screen, float32(timelineX), float32(timelineY), float32(timelineWidth), float32(timelineHeight), timelineColor, false)

	// Draw timeline border
	vector.StrokeRect(screen, float32(timelineX), float32(timelineY), float32(timelineWidth), float32(timelineHeight), 2, borderColor, false)

	// Draw timeline markers
	e.drawTimelineMarkers(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw selection area
	e.drawSelection(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw event markers on timeline
	e.drawEventMarkers(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw playhead
	e.drawPlayhead(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw timeline labels
	e.drawTimelineLabels(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw "+" button to extend timeline
	e.drawPlusButton(screen, timelineX, timelineY, timelineWidth, timelineHeight, alpha)

	// Draw configuration area when maximized
	e.drawConfigurationArea(screen, panelX, panelY, panelWidth, panelHeight, alpha)

	// Draw territory selector if visible
	if e.territorySelector != nil {
		e.territorySelector.Draw(screen)
	}
}

// drawTimelineMarkers draws the time markers on the timeline
func (e *EventEditorGUI) drawTimelineMarkers(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	markerColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}

	// Calculate visible range in ticks
	visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
	visibleStart := e.timelineOffset
	visibleEnd := e.timelineOffset + visibleTicks
	if visibleEnd > e.maxTicks {
		visibleEnd = e.maxTicks
	}

	// Calculate marker step based on zoom level
	markerStep := int(float64(visibleTicks) / 10.0) // Aim for ~10 major markers
	if markerStep < 10 {
		markerStep = 10
	} else if markerStep < 50 {
		markerStep = 50
	} else if markerStep < 100 {
		markerStep = 100
	}

	// Draw major markers
	for tick := (visibleStart / markerStep) * markerStep; tick <= visibleEnd; tick += markerStep {
		if tick < 0 || tick > e.maxTicks {
			continue
		}

		screenPos := float64(tick-visibleStart) / float64(visibleEnd-visibleStart)
		x := float32(timelineX + int(screenPos*float64(timelineWidth)))

		// Draw major marker
		vector.StrokeLine(screen, x, float32(timelineY), x, float32(timelineY+TimelineMarkersHeight), 1, markerColor, false)

		// Draw minor markers between major ones
		minorStep := markerStep / 5
		for minorTick := tick + minorStep; minorTick < tick+markerStep && minorTick <= visibleEnd; minorTick += minorStep {
			if minorTick < 0 || minorTick > e.maxTicks {
				continue
			}

			minorScreenPos := float64(minorTick-visibleStart) / float64(visibleEnd-visibleStart)
			minorX := float32(timelineX + int(minorScreenPos*float64(timelineWidth)))
			vector.StrokeLine(screen, minorX, float32(timelineY), minorX, float32(timelineY+TimelineMarkersHeight/2), 1, markerColor, false)
		}
	}
}

// drawPlayhead draws the playhead indicator on the timeline
func (e *EventEditorGUI) drawPlayhead(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	// Calculate visible range in ticks
	visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
	visibleStart := e.timelineOffset
	visibleEnd := e.timelineOffset + visibleTicks
	if visibleEnd > e.maxTicks {
		visibleEnd = e.maxTicks
	}

	// Only draw if playhead is in visible range
	if e.playheadTick >= visibleStart && e.playheadTick <= visibleEnd {
		screenPos := float64(e.playheadTick-visibleStart) / float64(visibleEnd-visibleStart)
		x := float32(timelineX + int(screenPos*float64(timelineWidth)))

		// Choose colors based on dragging state
		var playheadColor, handleColor, handleBorderColor color.RGBA
		if e.isDraggingHead {
			// Brighter colors when dragging
			playheadColor = color.RGBA{UIColors.Accent.R, UIColors.Accent.G, UIColors.Accent.B, alpha}
			handleColor = color.RGBA{255, 200, 50, alpha}
			handleBorderColor = color.RGBA{255, 255, 255, alpha}
		} else {
			// Normal colors
			playheadColor = color.RGBA{UIColors.Accent.R, UIColors.Accent.G, UIColors.Accent.B, alpha}
			handleColor = color.RGBA{UIColors.Primary.R, UIColors.Primary.G, UIColors.Primary.B, alpha}
			handleBorderColor = color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
		}

		// Draw playhead line
		lineWidth := PlayheadWidth
		if e.isDraggingHead {
			lineWidth = PlayheadWidth + 1 // Slightly thicker when dragging
		}
		vector.StrokeLine(screen, x, float32(timelineY), x, float32(timelineY+timelineHeight), float32(lineWidth), playheadColor, false)

		// Draw playhead handle
		handleY := float32(timelineY + TimelineMarkersHeight)
		handleSize := float32(PlayheadHandleSize)
		if e.isDraggingHead {
			handleSize += 2 // Slightly larger when dragging
		}
		vector.DrawFilledCircle(screen, x, handleY+handleSize/2, handleSize/2, handleColor, false)

		// Draw handle border
		vector.StrokeCircle(screen, x, handleY+handleSize/2, handleSize/2, 1, handleBorderColor, false)
	}
}

// drawTimelineLabels draws time labels on the timeline
func (e *EventEditorGUI) drawTimelineLabels(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	labelColor := color.RGBA{UIColors.TextSecondary.R, UIColors.TextSecondary.G, UIColors.TextSecondary.B, alpha}

	// Draw current playhead time in ticks
	timeText := fmt.Sprintf("Time: +%d ticks", e.playheadTick)
	textX := timelineX + 10
	textY := timelineY + timelineHeight - 10
	text.Draw(screen, timeText, e.font, textX, textY, labelColor)

	// Draw zoom level and visible range
	visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
	visibleStart := e.timelineOffset
	visibleEnd := e.timelineOffset + visibleTicks
	if visibleEnd > e.maxTicks {
		visibleEnd = e.maxTicks
	}

	zoomText := fmt.Sprintf("Zoom: %.1fx | View: %d-%d ticks | Max: %d", e.timelineScale, visibleStart, visibleEnd, e.maxTicks)
	zoomWidth := font.MeasureString(e.font, zoomText).Round()
	zoomX := timelineX + timelineWidth - zoomWidth - 10
	zoomY := timelineY + timelineHeight - 10
	text.Draw(screen, zoomText, e.font, zoomX, zoomY, labelColor)
}

// drawPlusButton draws the "+" button to extend the timeline
func (e *EventEditorGUI) drawPlusButton(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	// Position the plus button outside the timeline area
	plusButtonSize := 30
	plusButtonX := timelineX + timelineWidth + 10
	plusButtonY := timelineY + timelineHeight/2 - 15

	// Draw button background
	buttonColor := color.RGBA{UIColors.Primary.R, UIColors.Primary.G, UIColors.Primary.B, alpha}
	vector.DrawFilledRect(screen, float32(plusButtonX), float32(plusButtonY), float32(plusButtonSize), float32(plusButtonSize), buttonColor, false)

	// Draw button border
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}
	vector.StrokeRect(screen, float32(plusButtonX), float32(plusButtonY), float32(plusButtonSize), float32(plusButtonSize), 2, borderColor, false)

	// Draw plus sign
	plusColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
	centerX := float32(plusButtonX + plusButtonSize/2)
	centerY := float32(plusButtonY + plusButtonSize/2)
	plusSize := float32(10)

	// Horizontal line
	vector.StrokeLine(screen, centerX-plusSize/2, centerY, centerX+plusSize/2, centerY, 2, plusColor, false)
	// Vertical line
	vector.StrokeLine(screen, centerX, centerY-plusSize/2, centerX, centerY+plusSize/2, 2, plusColor, false)

	// Draw tooltip
	tooltipText := "+600"
	tooltipColor := color.RGBA{UIColors.TextSecondary.R, UIColors.TextSecondary.G, UIColors.TextSecondary.B, alpha}
	tooltipX := plusButtonX + plusButtonSize/2 - font.MeasureString(e.font, tooltipText).Round()/2
	tooltipY := plusButtonY - 5
	text.Draw(screen, tooltipText, e.font, tooltipX, tooltipY, tooltipColor)
}

// drawToggleButton draws the toggle button for switching editor area layout
func (e *EventEditorGUI) drawToggleButton(screen *ebiten.Image, alpha uint8) {
	// Calculate panel bounds to position toggle button in top-right corner

	// Calculate panel bounds
	maxPanelX := EventEditorPadding
	maxPanelY := EventEditorPadding
	maxPanelWidth := e.width - (EventEditorPadding * 2)

	minPanelX := EventEditorPadding
	minPanelY := EventEditorPadding
	minPanelWidth := e.width/5 - EventEditorPadding

	// Interpolate between layouts
	t := e.easeInOut(e.layoutAnimationPhase)
	panelX := int(float64(maxPanelX) + t*float64(minPanelX-maxPanelX))
	panelY := int(float64(maxPanelY) + t*float64(minPanelY-maxPanelY))
	panelWidth := int(float64(maxPanelWidth) + t*float64(minPanelWidth-maxPanelWidth))

	// Position toggle button in top-right corner of panel
	toggleButtonSize := 30
	toggleButtonX := panelX + panelWidth - toggleButtonSize - 10
	toggleButtonY := panelY + 10

	// Draw button background
	buttonColor := color.RGBA{UIColors.Secondary.R, UIColors.Secondary.G, UIColors.Secondary.B, alpha}
	vector.DrawFilledRect(screen, float32(toggleButtonX), float32(toggleButtonY), float32(toggleButtonSize), float32(toggleButtonSize), buttonColor, false)

	// Draw button border
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}
	vector.StrokeRect(screen, float32(toggleButtonX), float32(toggleButtonY), float32(toggleButtonSize), float32(toggleButtonSize), 2, borderColor, false)

	// Draw icon based on current state
	iconColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
	centerX := float32(toggleButtonX + toggleButtonSize/2)
	centerY := float32(toggleButtonY + toggleButtonSize/2)
	iconSize := float32(8)

	if e.editorMinimal {
		// Draw + icon (maximize)
		// Horizontal line
		vector.StrokeLine(screen, centerX-iconSize/2, centerY, centerX+iconSize/2, centerY, 2, iconColor, false)
		// Vertical line
		vector.StrokeLine(screen, centerX, centerY-iconSize/2, centerX, centerY+iconSize/2, 2, iconColor, false)
	} else {
		// Draw - icon (minimize)
		// Horizontal line only
		vector.StrokeLine(screen, centerX-iconSize/2, centerY, centerX+iconSize/2, centerY, 2, iconColor, false)
	}
}

// drawSelection draws the selection area on the timeline
func (e *EventEditorGUI) drawSelection(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	if !e.hasTimelineSelection && !e.isSelecting {
		return
	}

	// Calculate visible range in ticks
	visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
	visibleStart := e.timelineOffset
	visibleEnd := e.timelineOffset + visibleTicks
	if visibleEnd > e.maxTicks {
		visibleEnd = e.maxTicks
	}

	// Get selection range
	selStart := e.selectionStart
	selEnd := e.selectionEnd

	// Handle direction during active selection vs finalized selection
	if e.isSelecting {
		// During active selection, don't swap - use actual start and end values
		// This allows proper visualization of both left-to-right and right-to-left drags
	} else if e.hasTimelineSelection {
		// For finalized selection, ensure start <= end
		if selStart > selEnd {
			selStart, selEnd = selEnd, selStart
		}
	}

	// For drawing purposes, always ensure left <= right
	drawLeft := selStart
	drawRight := selEnd
	if drawLeft > drawRight {
		drawLeft, drawRight = drawRight, drawLeft
	}

	// Check if selection is visible in current viewport
	if drawRight < visibleStart || drawLeft > visibleEnd {
		return // Selection is not visible
	}

	// Clamp selection to visible range
	drawStart := drawLeft
	drawEnd := drawRight
	if drawStart < visibleStart {
		drawStart = visibleStart
	}
	if drawEnd > visibleEnd {
		drawEnd = visibleEnd
	}

	// Convert to screen coordinates
	startScreenPos := float64(drawStart-visibleStart) / float64(visibleEnd-visibleStart)
	endScreenPos := float64(drawEnd-visibleStart) / float64(visibleEnd-visibleStart)

	startX := float32(timelineX + int(startScreenPos*float64(timelineWidth)))
	endX := float32(timelineX + int(endScreenPos*float64(timelineWidth)))

	// Draw selection background
	selectionColor := color.RGBA{100, 150, 255, uint8(60 * float64(alpha) / 255)} // Semi-transparent blue
	if e.isSelecting {
		selectionColor = color.RGBA{100, 150, 255, uint8(80 * float64(alpha) / 255)} // Slightly more opaque during selection
	}

	selectionWidth := endX - startX
	if selectionWidth > 0 {
		vector.DrawFilledRect(screen, startX, float32(timelineY), selectionWidth, float32(timelineHeight), selectionColor, false)

		// Draw selection borders
		borderColor := color.RGBA{100, 150, 255, alpha}
		vector.StrokeLine(screen, startX, float32(timelineY), startX, float32(timelineY+timelineHeight), 2, borderColor, false)
		vector.StrokeLine(screen, endX, float32(timelineY), endX, float32(timelineY+timelineHeight), 2, borderColor, false)
	}
}

// handleEventListClick handles mouse clicks on the event list
func (e *EventEditorGUI) handleEventListClick(mouseX, mouseY int) {
	listX, listY, listWidth, listHeight := e.getEventListBounds()

	// Check if clicking within event list bounds (but not on buttons)
	buttonAreaHeight := e.buttonHeight*2 + 10
	listContentHeight := listHeight - buttonAreaHeight

	if mouseX >= listX && mouseX <= listX+listWidth &&
		mouseY >= listY && mouseY <= listY+listContentHeight {

		// Calculate which event was clicked
		relativeY := mouseY - listY
		clickedIndex := (relativeY / e.eventItemHeight) + e.eventListScroll

		// Check if the click is valid
		if clickedIndex >= 0 && clickedIndex < len(e.events) {
			currentTime := time.Now()

			// Check for double-click (within 500ms and same item)
			if clickedIndex == e.lastClickIndex &&
				currentTime.Sub(e.lastClickTime) < 500*time.Millisecond {
				// Double-click: start editing
				e.editingEventIndex = clickedIndex
				e.editingEventText = e.events[clickedIndex].Name
				e.cursorPosition = len(e.editingEventText) // Position cursor at end
				e.clearSelection()                         // Clear any text selection
			} else {
				// Single click: just select
				if e.editingEventIndex == clickedIndex {
					// If we're editing this event, don't change selection
					return
				}
				e.selectedEvent = clickedIndex
				// Stop editing if we click on a different event
				if e.editingEventIndex != clickedIndex {
					e.finishEditing()
				}
			}

			e.lastClickTime = currentTime
			e.lastClickIndex = clickedIndex
		} else {
			// Clicked outside any event - finish editing and clear selection
			e.finishEditing()
			e.selectedEvent = -1
		}
	} else {
		// Clicked outside event list content area - finish editing
		e.finishEditing()
	}

	// Check if clicking on buttons at bottom of event list
	e.handleEventListButtonClick(mouseX, mouseY)
}

// handleEventListScroll handles mouse wheel scrolling over the event list
func (e *EventEditorGUI) handleEventListScroll(mouseX, mouseY int) {
	listX, listY, listWidth, listHeight := e.getEventListBounds()

	// Check if mouse is over event list
	if mouseX >= listX && mouseX <= listX+listWidth &&
		mouseY >= listY && mouseY <= listY+listHeight {

		_, wheelY := ebiten.Wheel()
		if wheelY != 0 {
			e.eventListScroll -= int(wheelY * 3)
			e.clampEventListScroll()
		}
	}
}

// getEventListBounds calculates the bounds of the event list area
func (e *EventEditorGUI) getEventListBounds() (int, int, int, int) {
	// Calculate panel bounds first
	timelineHeight := e.timelineHeight / 2
	var timelineY int
	if e.editorMinimal {
		timelineY = e.height - timelineHeight - EventEditorPadding
	} else {
		timelineY = e.height - timelineHeight - EventEditorPadding
	}

	// Calculate maximized and minimal layout values
	maxPanelX := EventEditorPadding
	maxPanelY := EventEditorPadding
	maxPanelWidth := e.width - (EventEditorPadding * 2)
	timelineStartY := timelineY - EventEditorPadding
	maxPanelHeight := timelineStartY - EventEditorPadding

	minPanelX := EventEditorPadding
	minPanelY := EventEditorPadding
	minPanelWidth := e.width/5 - EventEditorPadding
	minPanelHeight := (e.height - EventEditorPadding*2) - 80

	// Interpolate between layouts
	t := e.easeInOut(e.layoutAnimationPhase)
	panelX := int(float64(maxPanelX) + t*float64(minPanelX-maxPanelX))
	panelY := int(float64(maxPanelY) + t*float64(minPanelY-maxPanelY))
	panelWidth := int(float64(maxPanelWidth) + t*float64(minPanelWidth-maxPanelWidth))
	panelHeight := int(float64(maxPanelHeight) + t*float64(minPanelHeight-maxPanelHeight))

	// Calculate event list area - takes 1/3 width when maximized, full width when minimal
	listPadding := 15                             // Increased padding for better spacing
	configAreaPadding := 15                       // Padding between scrollable area and configuration area
	titleHeight := 70                             // Increased height for much bigger title font and better spacing
	buttonAreaHeight := (e.buttonHeight * 2) + 15 // Space for 2 rows of buttons plus padding

	var listX, listY, listWidth, listHeight int

	// Calculate list width: 1/3 when maximized (phase=0), full when minimal (phase=1)
	// When maximized, reserve space for padding between list and config area
	maxListWidth := (panelWidth - listPadding*2 - configAreaPadding) / 3 // 1/3 width when maximized with padding
	minListWidth := panelWidth - listPadding*2                           // Full width when minimal

	// Interpolate between max and min widths
	tWidth := e.easeInOut(e.layoutAnimationPhase)
	listWidth = int(float64(maxListWidth) + tWidth*float64(minListWidth-maxListWidth))

	listX = panelX + listPadding
	listY = panelY + titleHeight
	listHeight = panelHeight - titleHeight - listPadding - buttonAreaHeight

	return listX, listY, listWidth, listHeight
}

// drawEventList draws the scrollable event list
func (e *EventEditorGUI) drawEventList(screen *ebiten.Image, panelX, panelY, panelWidth, panelHeight int, alpha uint8) {
	listX, listY, listWidth, listHeight := e.getEventListBounds()

	// Draw list background
	listColor := color.RGBA{UIColors.Background.R, UIColors.Background.G, UIColors.Background.B, alpha}
	vector.DrawFilledRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight), listColor, false)

	// Draw list border
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}
	vector.StrokeRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight), 1, borderColor, false)

	// Calculate visible items
	e.maxVisibleEvents = listHeight / e.eventItemHeight
	startIndex := e.eventListScroll
	endIndex := startIndex + e.maxVisibleEvents
	if endIndex > len(e.events) {
		endIndex = len(e.events)
	}

	// Draw visible events
	for i := startIndex; i < endIndex; i++ {
		itemY := listY + (i-startIndex)*e.eventItemHeight

		// Draw selection highlight
		if i == e.selectedEvent {
			selectionColor := color.RGBA{UIColors.Accent.R, UIColors.Accent.G, UIColors.Accent.B, uint8(100 * float64(alpha) / 255)}
			vector.DrawFilledRect(screen, float32(listX), float32(itemY), float32(listWidth), float32(e.eventItemHeight), selectionColor, false)
		}

		// Draw event text or text input if editing
		textColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
		textX := listX + 8
		textY := itemY + e.eventItemHeight/2 + e.font.Metrics().Ascent.Round()/2

		if i == e.editingEventIndex {
			// Draw text input background
			inputBgColor := color.RGBA{UIColors.Surface.R, UIColors.Surface.G, UIColors.Surface.B, alpha}
			vector.DrawFilledRect(screen, float32(textX-2), float32(itemY+2), float32(listWidth-16), float32(e.eventItemHeight-4), inputBgColor, false)

			// Draw text input border
			borderColor := color.RGBA{UIColors.Primary.R, UIColors.Primary.G, UIColors.Primary.B, alpha}
			vector.StrokeRect(screen, float32(textX-2), float32(itemY+2), float32(listWidth-16), float32(e.eventItemHeight-4), 2, borderColor, false)

			// Draw editing text
			eventText := e.editingEventText

			// Draw text selection highlight if any
			if e.hasSelection() {
				start := e.textSelectionStart
				end := e.textSelectionEnd
				if start > end {
					start, end = end, start
				}

				// Measure text before selection start
				beforeSelection := ""
				if start > 0 && start <= len(eventText) {
					beforeSelection = eventText[:start]
				}
				beforeWidth := font.MeasureString(e.font, beforeSelection).Round()

				// Measure selected text
				selectedText := ""
				if start >= 0 && end <= len(eventText) && start < end {
					selectedText = eventText[start:end]
				}
				selectedWidth := font.MeasureString(e.font, selectedText).Round()

				// Draw selection background
				if selectedWidth > 0 {
					selectionColor := color.RGBA{UIColors.Primary.R, UIColors.Primary.G, UIColors.Primary.B, uint8(128 * float64(alpha) / 255)}
					selectionX := textX + beforeWidth
					vector.DrawFilledRect(screen, float32(selectionX), float32(itemY+4), float32(selectedWidth), float32(e.eventItemHeight-8), selectionColor, false)
				}
			}

			// Draw the text
			text.Draw(screen, eventText, e.font, textX, textY, textColor)

			// Draw cursor (blinking) at correct position
			cursorTime := time.Now().UnixMilli() % 1000
			if cursorTime < 500 { // Blink every 500ms
				// Measure text before cursor position to get correct X position
				beforeCursor := ""
				if e.cursorPosition > 0 && e.cursorPosition <= len(eventText) {
					beforeCursor = eventText[:e.cursorPosition]
				}
				cursorXOffset := font.MeasureString(e.font, beforeCursor).Round()
				cursorX := textX + cursorXOffset
				cursorY1 := itemY + 5
				cursorY2 := itemY + e.eventItemHeight - 5
				vector.StrokeLine(screen, float32(cursorX), float32(cursorY1), float32(cursorX), float32(cursorY2), 1, textColor, false)
			}
		} else {
			// Normal text display
			eventText := e.events[i].Name // Get the name from the TimelineEvent
			text.Draw(screen, eventText, e.font, textX, textY, textColor)
		}
	}

	// Draw scrollbar if needed
	if len(e.events) > e.maxVisibleEvents {
		e.drawEventListScrollbar(screen, listX+listWidth-8, listY, 8, listHeight, alpha)
	}

	// Draw buttons at the bottom of the list
	e.drawEventListButtons(screen, listX, listY, listWidth, listHeight, alpha)

	// Draw list title
	titleColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}
	titleText := "Events"
	titleX := listX + 8
	titleY := listY - 15 // More space above the list
	text.Draw(screen, titleText, e.font, titleX, titleY, titleColor)
}

// drawEventListScrollbar draws the scrollbar for the event list
func (e *EventEditorGUI) drawEventListScrollbar(screen *ebiten.Image, x, y, width, height int, alpha uint8) {
	// Draw scrollbar background
	bgColor := color.RGBA{50, 50, 50, alpha}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), bgColor, false)

	// Calculate thumb size and position
	thumbHeight := (height * e.maxVisibleEvents) / len(e.events)
	if thumbHeight < 40 { // Increased minimum from 20 to 40 for better visibility with bigger fonts
		thumbHeight = 40
	}

	maxScroll := len(e.events) - e.maxVisibleEvents
	if maxScroll <= 0 {
		return
	}

	thumbY := y + (height-thumbHeight)*e.eventListScroll/maxScroll

	// Draw scrollbar thumb
	thumbColor := color.RGBA{150, 150, 150, alpha}
	vector.DrawFilledRect(screen, float32(x+1), float32(thumbY), float32(width-2), float32(thumbHeight), thumbColor, false)
}

// clampEventListScroll ensures scroll offset is within valid bounds
func (e *EventEditorGUI) clampEventListScroll() {
	maxScroll := len(e.events) - e.maxVisibleEvents
	if maxScroll < 0 {
		maxScroll = 0
	}

	if e.eventListScroll < 0 {
		e.eventListScroll = 0
	} else if e.eventListScroll > maxScroll {
		e.eventListScroll = maxScroll
	}
}

// updateButtonHover updates which button is currently being hovered over
func (e *EventEditorGUI) updateButtonHover(mouseX, mouseY int) {
	listX, listY, listWidth, listHeight := e.getEventListBounds()

	// Calculate button area
	buttonAreaY := listY + listHeight + 5 // Position buttons below the list with small gap
	buttonWidth := (listWidth - 15) / 2   // Two buttons per row with padding
	buttonPadding := 5

	// Clear hover state
	e.hoveredButton = ""

	// Check New button (top left)
	newButtonX := listX + buttonPadding
	newButtonY := buttonAreaY + buttonPadding
	if mouseX >= newButtonX && mouseX <= newButtonX+buttonWidth &&
		mouseY >= newButtonY && mouseY <= newButtonY+e.buttonHeight {
		e.hoveredButton = "new"
		return
	}

	// Check Delete button (top right)
	deleteButtonX := listX + buttonWidth + buttonPadding*2
	deleteButtonY := buttonAreaY + buttonPadding
	if mouseX >= deleteButtonX && mouseX <= deleteButtonX+buttonWidth &&
		mouseY >= deleteButtonY && mouseY <= deleteButtonY+e.buttonHeight {
		e.hoveredButton = "delete"
		return
	}

	// Check Import button (bottom left)
	importButtonX := listX + buttonPadding
	importButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2
	if mouseX >= importButtonX && mouseX <= importButtonX+buttonWidth &&
		mouseY >= importButtonY && mouseY <= importButtonY+e.buttonHeight {
		e.hoveredButton = "import"
		return
	}

	// Check Export button (bottom right)
	exportButtonX := listX + buttonWidth + buttonPadding*2
	exportButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2
	if mouseX >= exportButtonX && mouseX <= exportButtonX+buttonWidth &&
		mouseY >= exportButtonY && mouseY <= exportButtonY+e.buttonHeight {
		e.hoveredButton = "export"
		return
	}
}

// updateConfigurationHover updates which configuration element is currently being hovered over
func (e *EventEditorGUI) updateConfigurationHover(mouseX, mouseY int) {
	// Only update hover when maximized and dropdown is open
	if e.layoutAnimationPhase > 0.5 || !e.typeDropdownOpen {
		e.hoveredTypeOption = -1
		return
	}

	// Calculate configuration area bounds
	listX, listY, listWidth, listHeight := e.getEventListBounds()
	configAreaPadding := 15

	configX := listX + listWidth + configAreaPadding
	configY := listY
	configWidth := e.width - (EventEditorPadding * 2) - configX - 10

	// Only handle if there's enough space and mouse is in area
	if configWidth < 200 || mouseX < configX || mouseX > configX+configWidth ||
		mouseY < configY || mouseY > configY+listHeight {
		e.hoveredTypeOption = -1
		return
	}

	// Calculate column positions
	columnWidth := (configWidth - 45) / 2
	rightColumnX := configX + 15 + columnWidth + 15

	// Calculate content area
	titleY := configY + 15 + e.font.Metrics().Ascent.Round()
	dividerY := titleY + 8
	contentY := dividerY + 10
	rightCurrentY := contentY + 12 // After "Type:" label (reduced to match drawing code)
	dropdownHeight := 30

	// Clear hover state
	e.hoveredTypeOption = -1

	// Check dropdown options if open
	if e.typeDropdownOpen {
		eventTypes := []string{"Guild Change", "Territory Update", "Resource Edit"}
		optionHeight := 25
		for i, _ := range eventTypes {
			if i == e.selectedEventType {
				continue // Skip the currently selected option
			}

			optionY := rightCurrentY + dropdownHeight + (i * optionHeight)
			if i > e.selectedEventType {
				optionY -= optionHeight // Adjust for skipped selected option
			}

			if mouseX >= rightColumnX && mouseX <= rightColumnX+columnWidth &&
				mouseY >= optionY && mouseY <= optionY+optionHeight {
				e.hoveredTypeOption = i
				return
			}
		}
	}
}

// handleEventListButtonClick handles clicks on the buttons at the bottom of the event list
func (e *EventEditorGUI) handleEventListButtonClick(mouseX, mouseY int) {
	listX, listY, listWidth, listHeight := e.getEventListBounds()

	// Calculate button area - position buttons below the list
	buttonAreaY := listY + listHeight + 5 // Position buttons below the list with small gap

	buttonWidth := (listWidth - 15) / 2 // Two buttons per row with padding
	buttonPadding := 5

	// Check New button (top left)
	newButtonX := listX + buttonPadding
	newButtonY := buttonAreaY + buttonPadding
	if mouseX >= newButtonX && mouseX <= newButtonX+buttonWidth &&
		mouseY >= newButtonY && mouseY <= newButtonY+e.buttonHeight {
		e.addNewEvent()
		return
	}

	// Check Delete button (top right)
	deleteButtonX := listX + buttonWidth + buttonPadding*2
	deleteButtonY := buttonAreaY + buttonPadding
	if mouseX >= deleteButtonX && mouseX <= deleteButtonX+buttonWidth &&
		mouseY >= deleteButtonY && mouseY <= deleteButtonY+e.buttonHeight {
		if e.selectedEvent >= 0 && e.selectedEvent < len(e.events) {
			e.deleteSelectedEvent()
		}
		return
	}

	// Check Import button (bottom left)
	importButtonX := listX + buttonPadding
	importButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2
	if mouseX >= importButtonX && mouseX <= importButtonX+buttonWidth &&
		mouseY >= importButtonY && mouseY <= importButtonY+e.buttonHeight {
		e.importEvents()
		return
	}

	// Check Export button (bottom right)
	exportButtonX := listX + buttonWidth + buttonPadding*2
	exportButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2
	if mouseX >= exportButtonX && mouseX <= exportButtonX+buttonWidth &&
		mouseY >= exportButtonY && mouseY <= exportButtonY+e.buttonHeight {
		e.exportEvents()
		return
	}
}

// addNewEvent creates a new event and adds it to the timeline
func (e *EventEditorGUI) addNewEvent() {
	// Create a new event with default values
	newEvent := TimelineEvent{
		ID:          e.nextEventID,
		Name:        fmt.Sprintf("Event %d", e.nextEventID),
		Tick:        e.playheadTick, // Place at current playhead position
		Duration:    1,              // Default duration
		Description: "",             // No description
		EventType:   eventeditor.TriggerStateTick,
	}

	e.events = append(e.events, newEvent)
	e.nextEventID++
	e.selectedEvent = len(e.events) - 1 // Select the new event

	// Ensure the new event is visible
	e.ensureEventVisible(e.selectedEvent)
}

// deleteSelectedEvent deletes the currently selected event
func (e *EventEditorGUI) deleteSelectedEvent() {
	if e.selectedEvent < 0 || e.selectedEvent >= len(e.events) {
		return
	}

	// Remove the event from the slice
	e.events = append(e.events[:e.selectedEvent], e.events[e.selectedEvent+1:]...)

	// Adjust selected event index
	if e.selectedEvent >= len(e.events) {
		e.selectedEvent = len(e.events) - 1
	}

	// Clamp scroll to valid range
	e.clampEventListScroll()
}

// SetAvailableTerritories sets the list of available territories for event configuration
func (e *EventEditorGUI) SetAvailableTerritories(territories []*typedef.Territory) {
	e.availableTerritories = territories
	if e.territorySelector != nil {
		e.territorySelector.SetAvailableTerritories(territories)
	}
}

// importEvents placeholder for importing events
func (e *EventEditorGUI) importEvents() {
	// TODO: Implement import functionality
	fmt.Println("Import events functionality - to be implemented")
}

// exportEvents placeholder for exporting events
func (e *EventEditorGUI) exportEvents() {
	// TODO: Implement export functionality
	fmt.Println("Export events functionality - to be implemented")
}

// ensureEventVisible ensures the specified event index is visible in the scroll area
func (e *EventEditorGUI) ensureEventVisible(eventIndex int) {
	if eventIndex < 0 || eventIndex >= len(e.events) {
		return
	}

	if eventIndex < e.eventListScroll {
		e.eventListScroll = eventIndex
	} else if eventIndex >= e.eventListScroll+e.maxVisibleEvents {
		e.eventListScroll = eventIndex - e.maxVisibleEvents + 1
	}

	e.clampEventListScroll()
}

// handleConfigurationAreaClick handles mouse clicks on the configuration area
func (e *EventEditorGUI) handleConfigurationAreaClick(mouseX, mouseY int) {
	// Only handle clicks when maximized
	if e.layoutAnimationPhase > 0.5 {
		return
	}

	// Calculate configuration area bounds
	listX, listY, listWidth, listHeight := e.getEventListBounds()
	configAreaPadding := 15

	configX := listX + listWidth + configAreaPadding
	configY := listY
	configWidth := e.width - (EventEditorPadding * 2) - configX - 10

	// Only handle if there's enough space and click is in area
	if configWidth < 200 || mouseX < configX || mouseX > configX+configWidth ||
		mouseY < configY || mouseY > configY+listHeight {
		return
	}

	// Calculate column positions
	columnWidth := (configWidth - 45) / 2
	leftColumnX := configX + 15
	rightColumnX := leftColumnX + columnWidth + 15

	// Calculate content area
	titleY := configY + 15 + e.font.Metrics().Ascent.Round()
	dividerY := titleY + 8
	contentY := dividerY + 10
	currentY := contentY + 12 // After "State Tick:" label (reduced from 20 to 12)
	dropdownHeight := 30

	// Check State Tick input box (left column)
	inputHeight := 30
	if mouseX >= leftColumnX && mouseX <= leftColumnX+columnWidth &&
		mouseY >= currentY && mouseY <= currentY+inputHeight {
		e.editingStateTick = true
		e.stateTickCursorPos = len(e.stateTickInput) // Position cursor at end
		e.clearStateTickSelection()                  // Clear any selection
		e.typeDropdownOpen = false                   // Close dropdown if open
		return
	}

	// Check Type dropdown (right column)
	rightCurrentY := contentY + 12 // After "Type:" label (reduced from 20 to 12)
	if mouseX >= rightColumnX && mouseX <= rightColumnX+columnWidth &&
		mouseY >= rightCurrentY && mouseY <= rightCurrentY+dropdownHeight {
		e.typeDropdownOpen = !e.typeDropdownOpen
		e.editingStateTick = false // Close state tick editing if open
		return
	}

	// Check dropdown options if open
	if e.typeDropdownOpen {
		eventTypes := []string{"Guild Change", "Territory Update", "Resource Edit"}
		optionHeight := 25
		for i, _ := range eventTypes {
			if i == e.selectedEventType {
				continue // Skip the currently selected option
			}

			optionY := rightCurrentY + dropdownHeight + (i * optionHeight)
			if i > e.selectedEventType {
				optionY -= optionHeight // Adjust for skipped selected option
			}

			if mouseX >= rightColumnX && mouseX <= rightColumnX+columnWidth &&
				mouseY >= optionY && mouseY <= optionY+optionHeight {
				e.selectedEventType = i
				e.typeDropdownOpen = false
				return
			}
		}
	}

	// Calculate positions for new fields
	newFieldsCurrentY := contentY + 12 + 30 + 15 + 12 + 30 + 50 // After type dropdown

	// Check Territory selection button
	territoryButtonY := newFieldsCurrentY + 25 // Button is now 25px below the label
	territoryButtonHeight := 30
	territoryButtonWidth := 100
	if mouseX >= leftColumnX && mouseX <= leftColumnX+territoryButtonWidth &&
		mouseY >= territoryButtonY && mouseY <= territoryButtonY+territoryButtonHeight {
		// Start territory picking mode
		if e.territoryPicker != nil {
			e.territoryPicker.StartTerritoryPicking()
		}
		e.editingStateTick = false
		e.typeDropdownOpen = false
		return
	}

	// Click outside closes any open controls
	e.editingStateTick = false
	e.typeDropdownOpen = false
	e.territorySelectionOpen = false
}

// handleTextEditing handles keyboard input during text editing with full shortcuts support
func (e *EventEditorGUI) handleTextEditing() {
	// Handle state tick input editing
	if e.editingStateTick {
		e.handleStateTickInput()
		return
	}

	// Handle event name editing
	if e.editingEventIndex < 0 {
		return
	}

	// Debug: Print that we're in text editing mode
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		fmt.Printf("Text editing: processing arrow key (editingEventIndex: %d)\n", e.editingEventIndex)
	}

	// Handle keyboard shortcuts and text manipulation
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	// Handle cursor movement and selection
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if ctrl {
			// Ctrl+Left: Jump to previous word
			e.moveCursorToWordBoundary(-1, shift)
		} else {
			// Left: Move cursor left
			e.moveCursor(-1, shift)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if ctrl {
			// Ctrl+Right: Jump to next word
			e.moveCursorToWordBoundary(1, shift)
		} else {
			// Right: Move cursor right
			e.moveCursor(1, shift)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		// Home: Move to beginning
		e.setCursorPosition(0, shift)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		// End: Move to end
		e.setCursorPosition(len(e.editingEventText), shift)
	}

	// Handle text selection shortcuts
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		// Ctrl+A: Select all
		e.selectAll()
	}

	// Handle clipboard operations (placeholder for now)
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		// Ctrl+C: Copy selected text
		e.copySelectedText()
	}

	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyX) {
		// Ctrl+X: Cut selected text
		e.cutSelectedText()
	}

	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyV) {
		// Ctrl+V: Paste text (placeholder)
		e.pasteText()
	}

	// Handle text deletion
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if ctrl {
			// Ctrl+Backspace: Delete word backward
			e.deleteWordBackward()
		} else {
			// Backspace: Delete character backward
			e.deleteCharacterBackward()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if ctrl {
			// Ctrl+Delete: Delete word forward
			e.deleteWordForward()
		} else {
			// Delete: Delete character forward
			e.deleteCharacterForward()
		}
	}

	// Handle Enter to finish editing
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		e.finishEditing()
		return
	}

	// Handle Escape to cancel editing
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.cancelEditing()
		return
	}

	// Handle regular character input
	e.handleCharacterInput()
}

// Text editing helper methods

// moveCursor moves the cursor by the specified offset and optionally extends selection
func (e *EventEditorGUI) moveCursor(offset int, extend bool) {
	newPos := e.cursorPosition + offset
	e.setCursorPosition(newPos, extend)
}

// setCursorPosition sets the cursor to a specific position and optionally extends selection
func (e *EventEditorGUI) setCursorPosition(position int, extend bool) {
	// Clamp position to valid range
	if position < 0 {
		position = 0
	} else if position > len(e.editingEventText) {
		position = len(e.editingEventText)
	}

	if extend {
		// Extend selection
		if e.textSelectionStart == -1 {
			// Start new selection from current cursor position
			e.textSelectionStart = e.cursorPosition
		}
		e.textSelectionEnd = position
	} else {
		// Clear selection
		e.clearSelection()
	}

	e.cursorPosition = position
}

// moveCursorToWordBoundary moves cursor to word boundary in the specified direction
func (e *EventEditorGUI) moveCursorToWordBoundary(direction int, extend bool) {
	text := e.editingEventText
	pos := e.cursorPosition

	if direction < 0 {
		// Move backward to previous word start
		for pos > 0 && (text[pos-1] == ' ' || text[pos-1] == '\t') {
			pos-- // Skip whitespace
		}
		for pos > 0 && text[pos-1] != ' ' && text[pos-1] != '\t' {
			pos-- // Move to word start
		}
	} else {
		// Move forward to next word start
		for pos < len(text) && text[pos] != ' ' && text[pos] != '\t' {
			pos++ // Skip current word
		}
		for pos < len(text) && (text[pos] == ' ' || text[pos] == '\t') {
			pos++ // Skip whitespace
		}
	}

	e.setCursorPosition(pos, extend)
}

// selectAll selects all text
func (e *EventEditorGUI) selectAll() {
	e.textSelectionStart = 0
	e.textSelectionEnd = len(e.editingEventText)
	e.cursorPosition = len(e.editingEventText)
}

// clearSelection clears the current text selection
func (e *EventEditorGUI) clearSelection() {
	e.textSelectionStart = -1
	e.textSelectionEnd = -1
}

// hasSelection returns true if there's an active text selection
func (e *EventEditorGUI) hasSelection() bool {
	return e.textSelectionStart != -1 && e.textSelectionEnd != -1 && e.textSelectionStart != e.textSelectionEnd
}

// getSelectedText returns the currently selected text
func (e *EventEditorGUI) getSelectedText() string {
	if !e.hasSelection() {
		return ""
	}

	start := e.textSelectionStart
	end := e.textSelectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(e.editingEventText) {
		return ""
	}

	return e.editingEventText[start:end]
}

// deleteSelection deletes the currently selected text
func (e *EventEditorGUI) deleteSelection() {
	if !e.hasSelection() {
		return
	}

	start := e.textSelectionStart
	end := e.textSelectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(e.editingEventText) {
		return
	}

	e.editingEventText = e.editingEventText[:start] + e.editingEventText[end:]
	e.cursorPosition = start
	e.clearSelection()
}

// insertText inserts text at the current cursor position (replacing selection if any)
func (e *EventEditorGUI) insertText(text string) {
	if e.hasSelection() {
		e.deleteSelection()
	}

	// Limit the total length after insertion
	if len(e.editingEventText)+len(text) <= 50 {
		e.editingEventText += text
	}
}

// Clipboard operations with proper clipboard support
func (e *EventEditorGUI) copySelectedText() {
	if text := e.getSelectedText(); text != "" {
		// Only attempt clipboard operations on supported platforms
		if runtime.GOOS != "js" {
			clipboard.Write(clipboard.FmtText, []byte(text))
		}
	}
}

func (e *EventEditorGUI) cutSelectedText() {
	if text := e.getSelectedText(); text != "" {
		// Only attempt clipboard operations on supported platforms
		if runtime.GOOS != "js" {
			clipboard.Write(clipboard.FmtText, []byte(text))
		}
		e.deleteSelection()
	}
}

func (e *EventEditorGUI) pasteText() {
	// Only attempt clipboard operations on supported platforms
	if runtime.GOOS != "js" {
		if data := clipboard.Read(clipboard.FmtText); data != nil {
			e.insertText(string(data))
		}
	}
}

// Character deletion methods
func (e *EventEditorGUI) deleteCharacterBackward() {
	if e.hasSelection() {
		e.deleteSelection()
		return
	}

	if e.cursorPosition > 0 {
		before := e.editingEventText[:e.cursorPosition-1]
		after := e.editingEventText[e.cursorPosition:]
		e.editingEventText = before + after
		e.cursorPosition--
	}
}

func (e *EventEditorGUI) deleteCharacterForward() {
	if e.hasSelection() {
		e.deleteSelection()
		return
	}

	if e.cursorPosition < len(e.editingEventText) {
		before := e.editingEventText[:e.cursorPosition]
		after := e.editingEventText[e.cursorPosition+1:]
		e.editingEventText = before + after
	}
}

func (e *EventEditorGUI) deleteWordBackward() {
	if e.hasSelection() {
		e.deleteSelection()
		return
	}

	oldPos := e.cursorPosition
	e.moveCursorToWordBoundary(-1, false)
	newPos := e.cursorPosition

	before := e.editingEventText[:newPos]
	after := e.editingEventText[oldPos:]
	e.editingEventText = before + after
}

func (e *EventEditorGUI) deleteWordForward() {
	if e.hasSelection() {
		e.deleteSelection()
		return
	}

	oldPos := e.cursorPosition
	e.moveCursorToWordBoundary(1, false)
	newPos := e.cursorPosition

	before := e.editingEventText[:oldPos]
	after := e.editingEventText[newPos:]
	e.editingEventText = before + after
	e.cursorPosition = oldPos
}

// cancelEditing cancels editing without saving changes
func (e *EventEditorGUI) cancelEditing() {
	e.editingEventIndex = -1
	e.editingEventText = ""
	e.clearSelection()
	e.cursorPosition = 0
}

// handleCharacterInput handles regular character input
func (e *EventEditorGUI) handleCharacterInput() {
	chars := ebiten.AppendInputChars(nil)
	if len(chars) > 0 {
		text := string(chars)
		// Limit the total length after insertion
		if len(e.editingEventText)+len(text) <= 50 {
			e.insertText(text)
		}
	}
}

// finishEditing saves the current editing text and stops editing mode
func (e *EventEditorGUI) finishEditing() {
	if e.editingEventIndex >= 0 && e.editingEventIndex < len(e.events) {
		// Save the edited text
		e.events[e.editingEventIndex].Name = e.editingEventText
	}

	// Clear editing state
	e.editingEventIndex = -1
	e.editingEventText = ""
	e.clearSelection()
	e.cursorPosition = 0
}

// handleStateTickInput handles keyboard input for the state tick text box
func (e *EventEditorGUI) handleStateTickInput() {
	// Handle keyboard shortcuts and text manipulation
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	// Handle cursor movement and selection
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if ctrl {
			// Ctrl+Left: Move to previous word boundary
			e.moveStateTickCursorToWordBoundary(-1, shift)
		} else {
			// Left: Move cursor left
			e.moveStateTickCursor(-1, shift)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if ctrl {
			// Ctrl+Right: Move to next word boundary
			e.moveStateTickCursorToWordBoundary(1, shift)
		} else {
			// Right: Move cursor right
			e.moveStateTickCursor(1, shift)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		// Home: Move to beginning
		e.setStateTickCursorPosition(0, shift)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		// End: Move to end
		e.setStateTickCursorPosition(len(e.stateTickInput), shift)
	}

	// Handle text selection shortcuts
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		// Ctrl+A: Select all
		e.selectAllStateTick()
	}

	// Handle clipboard operations
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		// Ctrl+C: Copy selected text
		e.copySelectedStateTickText()
	}

	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyX) {
		// Ctrl+X: Cut selected text
		e.cutSelectedStateTickText()
	}

	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyV) {
		// Ctrl+V: Paste text
		e.pasteStateTickText()
	}

	// Handle text deletion
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if ctrl {
			// Ctrl+Backspace: Delete word backward
			e.deleteStateTickWordBackward()
		} else {
			// Backspace: Delete character backward
			e.deleteStateTickCharacterBackward()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if ctrl {
			// Ctrl+Delete: Delete word forward
			e.deleteStateTickWordForward()
		} else {
			// Delete: Delete character forward
			e.deleteStateTickCharacterForward()
		}
	}

	// Handle Enter to finish editing
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		e.editingStateTick = false
		return
	}

	// Handle Escape to cancel editing
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.editingStateTick = false
		return
	}

	// Handle regular character input - only allow digits
	e.handleStateTickCharacterInput()
}

// State tick text editing helper methods

// moveStateTickCursor moves the cursor by the specified offset and optionally extends selection
func (e *EventEditorGUI) moveStateTickCursor(offset int, extend bool) {
	newPos := e.stateTickCursorPos + offset
	e.setStateTickCursorPosition(newPos, extend)
}

// setStateTickCursorPosition sets the cursor to a specific position and optionally extends selection
func (e *EventEditorGUI) setStateTickCursorPosition(position int, extend bool) {
	// Clamp position to valid range
	if position < 0 {
		position = 0
	} else if position > len(e.stateTickInput) {
		position = len(e.stateTickInput)
	}

	if extend {
		// Extend selection
		if e.stateTickSelectionStart == -1 {
			// Start new selection from current cursor position
			e.stateTickSelectionStart = e.stateTickCursorPos
		}
		e.stateTickSelectionEnd = position
	} else {
		// Clear selection
		e.clearStateTickSelection()
	}

	e.stateTickCursorPos = position
}

// moveStateTickCursorToWordBoundary moves cursor to word boundary in the specified direction
func (e *EventEditorGUI) moveStateTickCursorToWordBoundary(direction int, extend bool) {
	text := e.stateTickInput
	pos := e.stateTickCursorPos

	if direction < 0 {
		// Move backward to previous word start (for numbers, this is just the beginning)
		pos = 0
	} else {
		// Move forward to next word start (for numbers, this is just the end)
		pos = len(text)
	}

	e.setStateTickCursorPosition(pos, extend)
}

// selectAllStateTick selects all text in state tick input
func (e *EventEditorGUI) selectAllStateTick() {
	e.stateTickSelectionStart = 0
	e.stateTickSelectionEnd = len(e.stateTickInput)
	e.stateTickCursorPos = len(e.stateTickInput)
}

// clearStateTickSelection clears the current text selection
func (e *EventEditorGUI) clearStateTickSelection() {
	e.stateTickSelectionStart = -1
	e.stateTickSelectionEnd = -1
}

// hasStateTickSelection returns true if there's an active text selection
func (e *EventEditorGUI) hasStateTickSelection() bool {
	return e.stateTickSelectionStart != -1 && e.stateTickSelectionEnd != -1 && e.stateTickSelectionStart != e.stateTickSelectionEnd
}

// getSelectedStateTickText returns the currently selected text
func (e *EventEditorGUI) getSelectedStateTickText() string {
	if !e.hasStateTickSelection() {
		return ""
	}

	start := e.stateTickSelectionStart
	end := e.stateTickSelectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(e.stateTickInput) {
		return ""
	}

	return e.stateTickInput[start:end]
}

// deleteStateTickSelection deletes the currently selected text
func (e *EventEditorGUI) deleteStateTickSelection() {
	if !e.hasStateTickSelection() {
		return
	}

	start := e.stateTickSelectionStart
	end := e.stateTickSelectionEnd
	if start > end {
		start, end = end, start
	}

	if start < 0 || end > len(e.stateTickInput) {
		return
	}

	e.stateTickInput = e.stateTickInput[:start] + e.stateTickInput[end:]
	e.stateTickCursorPos = start
	e.clearStateTickSelection()
}

// insertStateTickText inserts text at the current cursor position (replacing selection if any)
func (e *EventEditorGUI) insertStateTickText(text string) {
	// Only allow digits in state tick input
	filteredText := ""
	for _, char := range text {
		if char >= '0' && char <= '9' {
			filteredText += string(char)
		}
	}

	if filteredText == "" {
		return
	}

	if e.hasStateTickSelection() {
		e.deleteStateTickSelection()
	}

	// Limit total length
	if len(e.stateTickInput)+len(filteredText) > 10 {
		return
	}

	before := e.stateTickInput[:e.stateTickCursorPos]
	after := e.stateTickInput[e.stateTickCursorPos:]
	e.stateTickInput = before + filteredText + after
	e.stateTickCursorPos += len(filteredText)
}

// Clipboard operations for state tick
func (e *EventEditorGUI) copySelectedStateTickText() {
	if text := e.getSelectedStateTickText(); text != "" {
		// Only attempt clipboard operations on supported platforms
		if runtime.GOOS != "js" {
			err := clipboard.Init()
			if err == nil {
				clipboard.Write(clipboard.FmtText, []byte(text))
			}
		}
	}
}

func (e *EventEditorGUI) cutSelectedStateTickText() {
	if text := e.getSelectedStateTickText(); text != "" {
		// Only attempt clipboard operations on supported platforms
		if runtime.GOOS != "js" {
			err := clipboard.Init()
			if err == nil {
				clipboard.Write(clipboard.FmtText, []byte(text))
			}
		}
		e.deleteStateTickSelection()
	}
}

func (e *EventEditorGUI) pasteStateTickText() {
	// Only attempt clipboard operations on supported platforms
	if runtime.GOOS != "js" {
		err := clipboard.Init()
		if err == nil {
			data := clipboard.Read(clipboard.FmtText)
			if data != nil {
				text := string(data)
				e.insertStateTickText(text)
			}
		}
	}
}

// Character deletion methods for state tick
func (e *EventEditorGUI) deleteStateTickCharacterBackward() {
	if e.hasStateTickSelection() {
		e.deleteStateTickSelection()
		return
	}

	if e.stateTickCursorPos > 0 {
		before := e.stateTickInput[:e.stateTickCursorPos-1]
		after := e.stateTickInput[e.stateTickCursorPos:]
		e.stateTickInput = before + after
		e.stateTickCursorPos--
	}
}

func (e *EventEditorGUI) deleteStateTickCharacterForward() {
	if e.hasStateTickSelection() {
		e.deleteStateTickSelection()
		return
	}

	if e.stateTickCursorPos < len(e.stateTickInput) {
		before := e.stateTickInput[:e.stateTickCursorPos]
		after := e.stateTickInput[e.stateTickCursorPos+1:]
		e.stateTickInput = before + after
	}
}

func (e *EventEditorGUI) deleteStateTickWordBackward() {
	if e.hasStateTickSelection() {
		e.deleteStateTickSelection()
		return
	}

	// For numeric input, delete to beginning
	e.stateTickInput = e.stateTickInput[e.stateTickCursorPos:]
	e.stateTickCursorPos = 0
}

func (e *EventEditorGUI) deleteStateTickWordForward() {
	if e.hasStateTickSelection() {
		e.deleteStateTickSelection()
		return
	}

	// For numeric input, delete to end
	e.stateTickInput = e.stateTickInput[:e.stateTickCursorPos]
}

// handleStateTickCharacterInput handles regular character input for state tick
func (e *EventEditorGUI) handleStateTickCharacterInput() {
	chars := ebiten.AppendInputChars(nil)
	if len(chars) > 0 {
		text := string(chars)
		e.insertStateTickText(text)
	}
}

// IsTextInputFocused returns whether the event editor has text input focused
func (e *EventEditorGUI) IsTextInputFocused() bool {
	// Check if territory selector has text input focused
	if e.territorySelector != nil && e.territorySelector.HasTextInputFocused() {
		return true
	}
	// Territory picker doesn't have text inputs, so no need to check
	return e.editingEventIndex >= 0 || e.editingStateTick
}

// easeInOut applies smooth easing to animation values
// Input t should be in range [0.0, 1.0]
// Returns smoothed value in range [0.0, 1.0]
func (e *EventEditorGUI) easeInOut(t float64) float64 {
	if t < 0.5 {
		return 2.0 * t * t
	}
	return 1.0 - 2.0*(1.0-t)*(1.0-t)
}

// GetLayoutAnimationPhase returns the current layout animation phase (0.0 = maximized, 1.0 = minimal)
func (e *EventEditorGUI) GetLayoutAnimationPhase() float64 {
	return e.layoutAnimationPhase
}

// drawConfigurationArea draws the configuration area when the event editor is maximized
func (e *EventEditorGUI) drawConfigurationArea(screen *ebiten.Image, panelX, panelY, panelWidth, panelHeight int, alpha uint8) {
	// Only draw when maximized (or transitioning to maximized)
	if e.layoutAnimationPhase > 0.5 {
		return // Don't draw config area when mostly minimal
	}

	// Calculate configuration area bounds
	listX, listY, listWidth, listHeight := e.getEventListBounds()
	configAreaPadding := 15

	configX := listX + listWidth + configAreaPadding
	configY := listY
	configWidth := panelX + panelWidth - configX - 10 // Leave some padding from panel edge
	configHeight := listHeight

	// Only draw if there's enough space
	if configWidth < 200 {
		return
	}

	// Draw configuration area background
	configColor := color.RGBA{UIColors.Surface.R, UIColors.Surface.G, UIColors.Surface.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	vector.DrawFilledRect(screen, float32(configX), float32(configY), float32(configWidth), float32(configHeight), configColor, false)

	// Draw configuration area border
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	vector.StrokeRect(screen, float32(configX), float32(configY), float32(configWidth), float32(configHeight), 1, borderColor, false)

	// Draw title (using regular font instead of title font for smaller size)
	titleText := "Event Configuration"
	titleColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	titleX := configX + 15
	titleY := configY + 15 + e.font.Metrics().Ascent.Round()
	text.Draw(screen, titleText, e.font, titleX, titleY, titleColor)

	// Draw horizontal divider line below title
	dividerY := titleY + 8
	dividerStartX := configX + 15
	dividerEndX := configX + configWidth - 15
	vector.StrokeLine(screen, float32(dividerStartX), float32(dividerY), float32(dividerEndX), float32(dividerY), 1, borderColor, false)

	// Calculate content area (below title and divider)
	contentY := dividerY + 20             // Increased padding from 10 to 20 for better visual separation
	columnWidth := (configWidth - 45) / 2 // Two columns with padding
	leftColumnX := configX + 15
	rightColumnX := leftColumnX + columnWidth + 15

	// Colors for UI elements
	textColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	inputBgColor := color.RGBA{UIColors.Background.R, UIColors.Background.G, UIColors.Background.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	inputBorderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}
	dropdownBgColor := color.RGBA{UIColors.Secondary.R, UIColors.Secondary.G, UIColors.Secondary.B, uint8(float64(alpha) * (1.0 - e.layoutAnimationPhase))}

	currentY := contentY

	// Left Column - State Tick Input
	// Label (using smaller font)
	stateTickLabel := "State Tick:"
	text.Draw(screen, stateTickLabel, e.smallFont, leftColumnX, currentY, textColor)
	currentY += 12 // Much reduced spacing from 20 to 12

	// Input box
	inputHeight := 30
	inputBoxColor := inputBgColor
	if e.editingStateTick {
		inputBoxColor = color.RGBA{inputBgColor.R + 20, inputBgColor.G + 20, inputBgColor.B + 20, inputBgColor.A}
	}
	vector.DrawFilledRect(screen, float32(leftColumnX), float32(currentY), float32(columnWidth), float32(inputHeight), inputBoxColor, false)
	vector.StrokeRect(screen, float32(leftColumnX), float32(currentY), float32(columnWidth), float32(inputHeight), 1, inputBorderColor, false)

	// Input text
	inputText := e.stateTickInput
	if inputText == "" {
		inputText = "0"
	}
	// Center the text horizontally within the input box
	textWidth := font.MeasureString(e.smallFont, inputText).Round()
	textX := leftColumnX + (columnWidth-textWidth)/2
	textY := currentY + inputHeight/2 + e.smallFont.Metrics().Ascent.Round()/2
	text.Draw(screen, inputText, e.smallFont, textX, textY, textColor)

	// Draw cursor if editing
	if e.editingStateTick {
		cursorX := textX + font.MeasureString(e.smallFont, inputText[:e.stateTickCursorPos]).Round()
		vector.DrawFilledRect(screen, float32(cursorX), float32(currentY+5), 2, float32(inputHeight-10), textColor, false)

		// Draw selection if any
		if e.hasStateTickSelection() {
			start := e.stateTickSelectionStart
			end := e.stateTickSelectionEnd
			if start > end {
				start, end = end, start
			}
			selStartX := textX + font.MeasureString(e.smallFont, inputText[:start]).Round()
			selEndX := textX + font.MeasureString(e.smallFont, inputText[:end]).Round()
			selectionColor := color.RGBA{100, 150, 255, 100}
			vector.DrawFilledRect(screen, float32(selStartX), float32(currentY+2), float32(selEndX-selStartX), float32(inputHeight-4), selectionColor, false)
		}
	}

	currentY += inputHeight + 15 // Reduced spacing from 20 to 15

	// Right Column - Type Dropdown
	rightCurrentY := contentY

	// Label (using smaller font)
	typeLabel := "Type:"
	text.Draw(screen, typeLabel, e.smallFont, rightColumnX, rightCurrentY, textColor)
	rightCurrentY += 12 // Much reduced spacing from 20 to 12

	// Dropdown button
	dropdownHeight := 30
	eventTypes := []string{"Guild Change", "Territory Update", "Resource Edit"}
	selectedTypeText := eventTypes[e.selectedEventType]

	// Dropdown background
	dropdownColor := dropdownBgColor
	if e.typeDropdownOpen {
		dropdownColor = color.RGBA{dropdownBgColor.R + 30, dropdownBgColor.G + 30, dropdownBgColor.B + 30, dropdownBgColor.A}
	}
	vector.DrawFilledRect(screen, float32(rightColumnX), float32(rightCurrentY), float32(columnWidth), float32(dropdownHeight), dropdownColor, false)
	vector.StrokeRect(screen, float32(rightColumnX), float32(rightCurrentY), float32(columnWidth), float32(dropdownHeight), 1, inputBorderColor, false)

	// Dropdown text
	// Center the text horizontally within the dropdown button (accounting for arrow space)
	arrowSpace := 24 // Space reserved for arrow on the right
	availableTextWidth := columnWidth - arrowSpace
	dropdownTextWidth := font.MeasureString(e.smallFont, selectedTypeText).Round()
	dropdownTextX := rightColumnX + (availableTextWidth-dropdownTextWidth)/2
	dropdownTextY := rightCurrentY + dropdownHeight/2 + e.smallFont.Metrics().Ascent.Round()/2
	text.Draw(screen, selectedTypeText, e.smallFont, dropdownTextX, dropdownTextY, textColor)

	// Dropdown arrow
	arrowSize := 8
	arrowX := rightColumnX + columnWidth - arrowSize - 8
	arrowY := rightCurrentY + dropdownHeight/2
	if e.typeDropdownOpen {
		// Up arrow
		vector.DrawFilledRect(screen, float32(arrowX), float32(arrowY-2), float32(arrowSize), 2, textColor, false)
		vector.DrawFilledRect(screen, float32(arrowX+2), float32(arrowY-4), float32(arrowSize-4), 2, textColor, false)
		vector.DrawFilledRect(screen, float32(arrowX+4), float32(arrowY-6), float32(arrowSize-8), 2, textColor, false)
	} else {
		// Down arrow
		vector.DrawFilledRect(screen, float32(arrowX), float32(arrowY), float32(arrowSize), 2, textColor, false)
		vector.DrawFilledRect(screen, float32(arrowX+2), float32(arrowY+2), float32(arrowSize-4), 2, textColor, false)
		vector.DrawFilledRect(screen, float32(arrowX+4), float32(arrowY+4), float32(arrowSize-8), 2, textColor, false)
	}

	// Draw dropdown options if open
	if e.typeDropdownOpen {
		optionHeight := 25
		for i, option := range eventTypes {
			if i == e.selectedEventType {
				continue // Skip the currently selected option
			}

			optionY := rightCurrentY + dropdownHeight + (i * optionHeight)
			if i > e.selectedEventType {
				optionY -= optionHeight // Adjust for skipped selected option
			}

			// Option background
			optionColor := dropdownBgColor
			if e.hoveredTypeOption == i {
				optionColor = color.RGBA{dropdownBgColor.R + 40, dropdownBgColor.G + 40, dropdownBgColor.B + 40, dropdownBgColor.A}
			}
			vector.DrawFilledRect(screen, float32(rightColumnX), float32(optionY), float32(columnWidth), float32(optionHeight), optionColor, false)
			vector.StrokeRect(screen, float32(rightColumnX), float32(optionY), float32(columnWidth), float32(optionHeight), 1, inputBorderColor, false)

			// Option text
			optionTextWidth := font.MeasureString(e.smallFont, option).Round()
			optionTextX := rightColumnX + (columnWidth-optionTextWidth)/2
			optionTextY := optionY + optionHeight/2 + e.smallFont.Metrics().Ascent.Round()/2
			text.Draw(screen, option, e.smallFont, optionTextX, optionTextY, textColor)
		}
	}

	// Add territory selection below the type dropdown
	currentY += 50 // Space below type dropdown

	// Territory selection label and count
	territoryLabel := "Affected Territories:"
	text.Draw(screen, territoryLabel, e.smallFont, leftColumnX, currentY, textColor)

	// Show selected territories count next to label
	selectedCount := len(e.selectedTerritories)
	countText := fmt.Sprintf("(%d selected)", selectedCount)
	labelWidth := font.MeasureString(e.smallFont, territoryLabel).Round()
	text.Draw(screen, countText, e.smallFont, leftColumnX+labelWidth+10, currentY, color.RGBA{150, 150, 150, alpha})

	currentY += 25 // Space for the button below the label

	// Territory selection button with hover/click effects
	territoryButtonHeight := 30
	territoryButtonWidth := 100

	// Get mouse position for hover detection
	mx, my := ebiten.CursorPosition()
	isHovering := mx >= leftColumnX && mx <= leftColumnX+territoryButtonWidth &&
		my >= currentY && my <= currentY+territoryButtonHeight
	isPressed := isHovering && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	// Button color based on state
	var territoryButtonColor color.RGBA
	if e.territoryPicker != nil && e.territoryPicker.IsActive() {
		territoryButtonColor = color.RGBA{90, 140, 220, alpha} // Lighter when active
	} else if isPressed {
		territoryButtonColor = color.RGBA{50, 100, 180, alpha} // Darker when pressed
	} else if isHovering {
		territoryButtonColor = color.RGBA{80, 130, 210, alpha} // Lighter when hovered
	} else {
		territoryButtonColor = color.RGBA{70, 120, 200, alpha} // Default blue
	}

	vector.DrawFilledRect(screen, float32(leftColumnX), float32(currentY), float32(territoryButtonWidth), float32(territoryButtonHeight), territoryButtonColor, false)
	vector.StrokeRect(screen, float32(leftColumnX), float32(currentY), float32(territoryButtonWidth), float32(territoryButtonHeight), 1, color.RGBA{100, 150, 230, alpha}, false)

	// Button text
	buttonText := "Select"
	if e.territoryPicker != nil && e.territoryPicker.IsActive() {
		buttonText = "Selecting..."
	}
	buttonTextWidth := font.MeasureString(e.smallFont, buttonText).Round()
	buttonTextX := leftColumnX + (territoryButtonWidth-buttonTextWidth)/2
	buttonTextY := currentY + territoryButtonHeight/2 + e.smallFont.Metrics().Ascent.Round()/2
	text.Draw(screen, buttonText, e.smallFont, buttonTextX, buttonTextY, color.RGBA{255, 255, 255, alpha})
}

// drawEventMarkers draws event markers on the timeline
func (e *EventEditorGUI) drawEventMarkers(screen *ebiten.Image, timelineX, timelineY, timelineWidth, timelineHeight int, alpha uint8) {
	if len(e.events) == 0 {
		return
	}

	// Calculate visible range in ticks
	visibleTicks := int(float64(e.maxTicks) / e.timelineScale)
	visibleStart := e.timelineOffset
	visibleEnd := e.timelineOffset + visibleTicks
	if visibleEnd > e.maxTicks {
		visibleEnd = e.maxTicks
	}

	// Draw each event that's visible in the timeline
	for i, event := range e.events {
		if event.Tick < visibleStart || event.Tick > visibleEnd {
			continue // Event is not in visible range
		}

		// Calculate screen position
		screenPos := float64(event.Tick-visibleStart) / float64(visibleEnd-visibleStart)
		x := float32(timelineX + int(screenPos*float64(timelineWidth)))

		// Choose color based on selection
		var markerColor color.RGBA
		if i == e.selectedEvent {
			markerColor = color.RGBA{255, 200, 50, alpha} // Bright yellow for selected
		} else {
			markerColor = color.RGBA{50, 200, 255, alpha} // Bright blue for unselected
		}

		// Draw event marker line
		vector.StrokeLine(screen, x, float32(timelineY), x, float32(timelineY+timelineHeight), 3, markerColor, false)

		// Draw event marker diamond at the top
		diamondSize := float32(6)
		diamondY := float32(timelineY + 5)

		// Draw diamond shape
		vector.DrawFilledRect(screen, x-diamondSize/2, diamondY-diamondSize/2, diamondSize, diamondSize, markerColor, false)

		// Draw event name if there's space (only for selected events to avoid clutter)
		if i == e.selectedEvent {
			eventName := event.Name
			nameWidth := font.MeasureString(e.font, eventName).Round()
			nameX := int(x) - nameWidth/2
			nameY := timelineY - 5

			// Clamp text position to timeline bounds
			if nameX < timelineX {
				nameX = timelineX
			} else if nameX+nameWidth > timelineX+timelineWidth {
				nameX = timelineX + timelineWidth - nameWidth
			}

			// Draw text background for better readability
			textBgColor := color.RGBA{0, 0, 0, uint8(180 * float64(alpha) / 255)}
			vector.DrawFilledRect(screen, float32(nameX-2), float32(nameY-e.font.Metrics().Ascent.Round()-2),
				float32(nameWidth+4), float32(e.font.Metrics().Height.Round()+4), textBgColor, false)

			// Draw event name
			textColor := color.RGBA{255, 255, 255, alpha}
			text.Draw(screen, eventName, e.font, nameX, nameY, textColor)
		}
	}
}

// drawEventListButtons draws the buttons at the bottom of the event list
func (e *EventEditorGUI) drawEventListButtons(screen *ebiten.Image, listX, listY, listWidth, listHeight int, alpha uint8) {
	buttonAreaY := listY + listHeight + 5 // Position buttons below the list with small gap

	buttonWidth := (listWidth - 15) / 2 // Two buttons per row with padding
	buttonPadding := 5

	// Button colors with hover and press effects (same style as toast buttons)
	baseButtonColor := color.RGBA{UIColors.Secondary.R, UIColors.Secondary.G, UIColors.Secondary.B, alpha}
	borderColor := color.RGBA{UIColors.Border.R, UIColors.Border.G, UIColors.Border.B, alpha}
	textColor := color.RGBA{UIColors.Text.R, UIColors.Text.G, UIColors.Text.B, alpha}

	// Helper function to get button color with effects
	getButtonColor := func(baseColor color.RGBA, isHovered, isPressed bool) color.RGBA {
		result := baseColor
		if isPressed {
			// Darker when pressed
			result.R = uint8(float64(result.R) * 0.8)
			result.G = uint8(float64(result.G) * 0.8)
			result.B = uint8(float64(result.B) * 0.8)
		} else if isHovered {
			// Lighter when hovered (same as toast buttons)
			result.R = uint8(float64(result.R) * 1.2)
			result.G = uint8(float64(result.G) * 1.2)
			result.B = uint8(float64(result.B) * 1.2)
			if result.R > 255 {
				result.R = 255
			}
			if result.G > 255 {
				result.G = 255
			}
			if result.B > 255 {
				result.B = 255
			}
		}
		return result
	}

	// Draw New button (top left)
	newButtonX := listX + buttonPadding
	newButtonY := buttonAreaY + buttonPadding

	newButtonColor := getButtonColor(baseButtonColor, e.hoveredButton == "new", e.pressedButton == "new")

	vector.DrawFilledRect(screen, float32(newButtonX), float32(newButtonY), float32(buttonWidth), float32(e.buttonHeight), newButtonColor, false)
	vector.StrokeRect(screen, float32(newButtonX), float32(newButtonY), float32(buttonWidth), float32(e.buttonHeight), 1, borderColor, false)

	newText := "New"
	newTextWidth := font.MeasureString(e.font, newText).Round()
	newTextX := newButtonX + (buttonWidth-newTextWidth)/2
	newTextY := newButtonY + e.buttonHeight/2 + e.font.Metrics().Ascent.Round()/2
	text.Draw(screen, newText, e.font, newTextX, newTextY, textColor)

	// Draw Delete button (top right)
	deleteButtonX := listX + buttonWidth + buttonPadding*2
	deleteButtonY := buttonAreaY + buttonPadding

	// Choose color based on hover/press state and availability
	var deleteButtonColor color.RGBA
	deleteTextColor := textColor
	if e.selectedEvent < 0 || e.selectedEvent >= len(e.events) {
		deleteButtonColor = color.RGBA{60, 60, 60, alpha} // Grayed out
		deleteTextColor = color.RGBA{120, 120, 120, alpha}
	} else {
		deleteButtonColor = getButtonColor(baseButtonColor, e.hoveredButton == "delete", e.pressedButton == "delete")
	}

	vector.DrawFilledRect(screen, float32(deleteButtonX), float32(deleteButtonY), float32(buttonWidth), float32(e.buttonHeight), deleteButtonColor, false)
	vector.StrokeRect(screen, float32(deleteButtonX), float32(deleteButtonY), float32(buttonWidth), float32(e.buttonHeight), 1, borderColor, false)

	deleteText := "Delete"
	deleteTextWidth := font.MeasureString(e.font, deleteText).Round()
	deleteTextX := deleteButtonX + (buttonWidth-deleteTextWidth)/2
	deleteTextY := deleteButtonY + e.buttonHeight/2 + e.font.Metrics().Ascent.Round()/2
	text.Draw(screen, deleteText, e.font, deleteTextX, deleteTextY, deleteTextColor)

	// Draw Import button (bottom left)
	importButtonX := listX + buttonPadding
	importButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2

	importButtonColor := getButtonColor(baseButtonColor, e.hoveredButton == "import", e.pressedButton == "import")

	vector.DrawFilledRect(screen, float32(importButtonX), float32(importButtonY), float32(buttonWidth), float32(e.buttonHeight), importButtonColor, false)
	vector.StrokeRect(screen, float32(importButtonX), float32(importButtonY), float32(buttonWidth), float32(e.buttonHeight), 1, borderColor, false)

	importText := "Import"
	importTextWidth := font.MeasureString(e.font, importText).Round()
	importTextX := importButtonX + (buttonWidth-importTextWidth)/2
	importTextY := importButtonY + e.buttonHeight/2 + e.font.Metrics().Ascent.Round()/2
	text.Draw(screen, importText, e.font, importTextX, importTextY, textColor)

	// Draw Export button (bottom right)
	exportButtonX := listX + buttonWidth + buttonPadding*2
	exportButtonY := buttonAreaY + e.buttonHeight + buttonPadding*2

	exportButtonColor := getButtonColor(baseButtonColor, e.hoveredButton == "export", e.pressedButton == "export")

	vector.DrawFilledRect(screen, float32(exportButtonX), float32(exportButtonY), float32(buttonWidth), float32(e.buttonHeight), exportButtonColor, false)
	vector.StrokeRect(screen, float32(exportButtonX), float32(exportButtonY), float32(buttonWidth), float32(e.buttonHeight), 1, borderColor, false)

	exportText := "Export"
	exportTextWidth := font.MeasureString(e.font, exportText).Round()
	exportTextX := exportButtonX + (buttonWidth-exportTextWidth)/2
	exportTextY := exportButtonY + e.buttonHeight/2 + e.font.Metrics().Ascent.Round()/2
	text.Draw(screen, exportText, e.font, exportTextX, exportTextY, textColor)
}
