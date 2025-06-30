package app

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
)

// EnhancedGuildData represents a guild entry
type EnhancedGuildData struct {
	Name  string `json:"name"`
	Tag   string `json:"tag"`
	Color string `json:"color"` // Hex color representation (e.g., "#FF0000")
}

// EnhancedTextInput represents a text input field with placeholder support
type EnhancedTextInput struct {
	Value          string
	Placeholder    string
	X, Y           int
	Width          int
	Height         int
	MaxLength      int
	Focused        bool
	cursorBlink    time.Time
	textOffset     int
	cursorPos      int       // Current cursor position
	selStart       int       // Selection start position
	selEnd         int       // Selection end position
	backspaceTimer time.Time // Timer for continuous backspace
	deleteTimer    time.Time // Timer for continuous delete
}

// NewEnhancedTextInput creates a new enhanced text input
func NewEnhancedTextInput(placeholder string, x, y, width, height, maxLength int) *EnhancedTextInput {
	return &EnhancedTextInput{
		Value:          "",
		Placeholder:    placeholder,
		X:              x,
		Y:              y,
		Width:          width,
		Height:         height,
		MaxLength:      maxLength,
		Focused:        false,
		cursorBlink:    time.Now(),
		textOffset:     0,
		cursorPos:      0,
		selStart:       -1,
		selEnd:         -1,
		backspaceTimer: time.Time{},
		deleteTimer:    time.Time{},
	}
}

// Update updates the text input
func (t *EnhancedTextInput) Update() bool {
	return t.UpdateWithSkipInput(false)
}

// UpdateWithSkipInput updates the text input with option to skip character input
func (t *EnhancedTextInput) UpdateWithSkipInput(skipInput bool) bool {
	changed := false

	// Note: Focus management is handled by the parent (guild manager)
	// Don't handle clicks here to avoid conflicts

	// Handle text input if focused
	if t.Focused {
		// Handle character input only if not skipping
		if !skipInput {
			chars := ebiten.AppendInputChars(nil)
			if len(chars) > 0 {
				// If there's a selection, delete it first
				if t.hasSelection() {
					t.deleteSelection()
					changed = true
				}

				for _, r := range chars {
					// Allow A-Z, a-z characters, whitespace, numbers, and common punctuation
					if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
						r == ' ' || r == '-' || r == '_' || r == '.' || r == ',' || r == '!' || r == '?' {
						if t.MaxLength == 0 || len(t.Value) < t.MaxLength {
							// Ensure cursor position is within bounds
							if t.cursorPos < 0 {
								t.cursorPos = 0
							}
							if t.cursorPos > len(t.Value) {
								t.cursorPos = len(t.Value)
							}
							// Insert at cursor position
							t.Value = t.Value[:t.cursorPos] + string(r) + t.Value[t.cursorPos:]
							t.cursorPos++
							changed = true
						}
					}
				}
			}
		}

		// Handle control key combinations
		if ebiten.IsKeyPressed(ebiten.KeyControl) {
			// Select all (Ctrl+A)
			if inpututil.IsKeyJustPressed(ebiten.KeyA) {
				t.selStart = 0
				t.selEnd = len(t.Value)
				t.cursorPos = t.selEnd
				return true
			}

			// Copy (Ctrl+C)
			if inpututil.IsKeyJustPressed(ebiten.KeyC) && t.hasSelection() {
				start, end := t.getOrderedSelection()
				// Ensure indices are within bounds
				if start >= 0 && end <= len(t.Value) && start <= end {
					selectedText := t.Value[start:end]
					clipboard.Write(clipboard.FmtText, []byte(selectedText))
				}
				return true
			}

			// Paste (Ctrl+V)
			if inpututil.IsKeyJustPressed(ebiten.KeyV) {
				if clipData := clipboard.Read(clipboard.FmtText); clipData != nil {
					clipText := string(clipData)
					// If there's a selection, delete it first
					if t.hasSelection() {
						t.deleteSelection()
						changed = true
					}

					// Insert clipboard text at cursor position
					if t.MaxLength == 0 || len(t.Value)+len(clipText) <= t.MaxLength {
						// Ensure cursor position is within bounds
						if t.cursorPos < 0 {
							t.cursorPos = 0
						}
						if t.cursorPos > len(t.Value) {
							t.cursorPos = len(t.Value)
						}
						t.Value = t.Value[:t.cursorPos] + clipText + t.Value[t.cursorPos:]
						t.cursorPos += len(clipText)
						changed = true
					}
				}
				return true
			}

			// Cut (Ctrl+X)
			if inpututil.IsKeyJustPressed(ebiten.KeyX) && t.hasSelection() {
				start, end := t.getOrderedSelection()
				// Ensure indices are within bounds
				if start >= 0 && end <= len(t.Value) && start <= end {
					selectedText := t.Value[start:end]
					clipboard.Write(clipboard.FmtText, []byte(selectedText))
					// Delete the selection
					t.deleteSelection()
					changed = true
				}
				return true
			}

			// Word navigation (Ctrl+Left/Right)
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
				t.moveCursorByWord(-1, ebiten.IsKeyPressed(ebiten.KeyShift))
				return true
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
				t.moveCursorByWord(1, ebiten.IsKeyPressed(ebiten.KeyShift))
				return true
			}
		}

		// Handle backspace with continuous deletion
		backspacePressed := ebiten.IsKeyPressed(ebiten.KeyBackspace)
		if backspacePressed {
			shouldDelete := false
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
				// First press
				shouldDelete = true
				t.backspaceTimer = time.Now()
			} else if time.Since(t.backspaceTimer) > 500*time.Millisecond {
				// After initial delay, delete continuously
				if time.Since(t.backspaceTimer).Milliseconds()%50 < 16 { // ~20fps for continuous deletion
					shouldDelete = true
				}
			}

			if shouldDelete {
				if t.hasSelection() {
					t.deleteSelection()
					changed = true
				} else if t.cursorPos > 0 {
					// Ensure cursor position is within bounds
					if t.cursorPos > len(t.Value) {
						t.cursorPos = len(t.Value)
					}
					if t.cursorPos > 0 {
						t.Value = t.Value[:t.cursorPos-1] + t.Value[t.cursorPos:]
						t.cursorPos--
						changed = true
					}
				}
			}
		}

		// Handle delete with continuous deletion
		deletePressed := ebiten.IsKeyPressed(ebiten.KeyDelete)
		if deletePressed {
			shouldDelete := false
			if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
				// First press
				shouldDelete = true
				t.deleteTimer = time.Now()
			} else if time.Since(t.deleteTimer) > 500*time.Millisecond {
				// After initial delay, delete continuously
				if time.Since(t.deleteTimer).Milliseconds()%50 < 16 { // ~60fps
					shouldDelete = true
				}
			}

			if shouldDelete {
				if t.hasSelection() {
					t.deleteSelection()
					changed = true
				} else if t.cursorPos < len(t.Value) {
					// Ensure cursor position is within bounds
					if t.cursorPos < 0 {
						t.cursorPos = 0
					}
					if t.cursorPos < len(t.Value) {
						t.Value = t.Value[:t.cursorPos] + t.Value[t.cursorPos+1:]
						changed = true
					}
				}
			}
		}

		// Handle arrow keys
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if t.cursorPos > 0 {
				if !ebiten.IsKeyPressed(ebiten.KeyShift) {
					t.selStart = -1
					t.selEnd = -1
				} else {
					// Start selection if not already selecting
					if t.selStart == -1 {
						t.selStart = t.cursorPos
					}
				}
				t.cursorPos--
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					t.selEnd = t.cursorPos
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if t.cursorPos < len(t.Value) {
				if !ebiten.IsKeyPressed(ebiten.KeyShift) {
					t.selStart = -1
					t.selEnd = -1
				} else {
					// Start selection if not already selecting
					if t.selStart == -1 {
						t.selStart = t.cursorPos
					}
				}
				t.cursorPos++
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					t.selEnd = t.cursorPos
				}
			}
		}

		// Handle home/end keys
		if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
			if !ebiten.IsKeyPressed(ebiten.KeyShift) {
				t.selStart = -1
				t.selEnd = -1
			} else {
				// Start selection if not already selecting
				if t.selStart == -1 {
					t.selStart = t.cursorPos
				}
			}
			t.cursorPos = 0
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				t.selEnd = t.cursorPos
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
			if !ebiten.IsKeyPressed(ebiten.KeyShift) {
				t.selStart = -1
				t.selEnd = -1
			} else {
				// Start selection if not already selecting
				if t.selStart == -1 {
					t.selStart = t.cursorPos
				}
			}
			t.cursorPos = len(t.Value)
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				t.selEnd = t.cursorPos
			}
		}
	}

	return changed
}

// hasSelection returns true if there is text selected
func (t *EnhancedTextInput) hasSelection() bool {
	return t.selStart != -1 && t.selEnd != -1 && t.selStart != t.selEnd
}

// deleteSelection removes the selected text
func (t *EnhancedTextInput) deleteSelection() {
	if !t.hasSelection() {
		return
	}
	start, end := t.getOrderedSelection()
	// Ensure indices are within bounds
	if start < 0 {
		start = 0
	}
	if end > len(t.Value) {
		end = len(t.Value)
	}
	if start > len(t.Value) {
		start = len(t.Value)
	}
	if end < 0 {
		end = 0
	}

	if start <= end && start >= 0 && end <= len(t.Value) {
		t.Value = t.Value[:start] + t.Value[end:]
		t.cursorPos = start
		// Ensure cursor position is within bounds
		if t.cursorPos < 0 {
			t.cursorPos = 0
		}
		if t.cursorPos > len(t.Value) {
			t.cursorPos = len(t.Value)
		}
	}
	t.selStart = -1
	t.selEnd = -1
}

// clearSelection clears text selection without deleting the text
func (t *EnhancedTextInput) clearSelection() {
	t.selStart = -1
	t.selEnd = -1
}

// getOrderedSelection returns the selection start/end in correct order
func (t *EnhancedTextInput) getOrderedSelection() (int, int) {
	if t.selStart <= t.selEnd {
		return t.selStart, t.selEnd
	}
	return t.selEnd, t.selStart
}

// moveCursorByWord moves the cursor to the next/previous word boundary
func (t *EnhancedTextInput) moveCursorByWord(direction int, selecting bool) {
	if selecting && t.selStart == -1 {
		t.selStart = t.cursorPos
	}

	if direction < 0 && t.cursorPos > 0 {
		// Move left by word
		pos := t.cursorPos - 1
		for pos > 0 && t.Value[pos-1] == ' ' {
			pos--
		}
		for pos > 0 && t.Value[pos-1] != ' ' {
			pos--
		}
		t.cursorPos = pos
	} else if direction > 0 && t.cursorPos < len(t.Value) {
		// Move right by word
		pos := t.cursorPos
		for pos < len(t.Value) && t.Value[pos] == ' ' {
			pos++
		}
		for pos < len(t.Value) && t.Value[pos] != ' ' {
			pos++
		}
		t.cursorPos = pos
	}

	if selecting {
		t.selEnd = t.cursorPos
	} else {
		t.selStart = -1
		t.selEnd = -1
	}
}

// EnhancedGuildManager handles the guild management UI with enhanced styling
type EnhancedGuildManager struct {
	visible            bool
	nameInput          *EnhancedTextInput
	tagInput           *EnhancedTextInput
	guilds             []EnhancedGuildData
	filteredGuilds     []EnhancedGuildData
	scrollOffset       int
	hoveredIndex       int
	selectedIndex      int
	guildFilePath      string
	modalX             int
	modalY             int
	modalWidth         int
	modalHeight        int
	justOpened         bool                             // Flag to prevent initial character input
	framesSinceOpen    int                              // Count frames since opening
	colorPicker        *ColorPicker                     // Color picker for guild colors
	keyEventCh         <-chan KeyEvent                  // Channel for receiving key events
	onEditClaim        func(guildName, guildTag string) // Callback for edit claim button
	onGuildDataChanged func()                           // Callback for when guild data changes
	// Status message manager
	statusMessageManager StatusMessageManager
	// Scrollbar interaction state
	scrollbarDragging bool
	dragStartY        int
	dragStartOffset   int
	// Performance optimization: guild lookup caches
	guildByNameTag map[string]*EnhancedGuildData // name+tag -> guild
	guildByName    map[string]*EnhancedGuildData // name -> guild
	guildByTag     map[string]*EnhancedGuildData // tag -> guild
	colorCache     map[string]color.RGBA         // color string -> parsed RGBA
	cachesDirty    bool                          // whether caches need rebuilding
	// API Import state
	apiImportInProgress bool // whether API import is currently running
}

// NewEnhancedGuildManager creates a new enhanced guild manager
func NewEnhancedGuildManager() *EnhancedGuildManager {
	screenW, screenH := ebiten.WindowSize()
	modalWidth := 600
	modalHeight := 500
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	// Tag input has max length 4 for tag validation (3-4 chars)
	nameInput := NewEnhancedTextInput("Enter guild name...", modalX+30, modalY+70, 200, 35, 50)
	tagInput := NewEnhancedTextInput("Tag...", modalX+240, modalY+70, 50, 35, 4)

	gm := &EnhancedGuildManager{
		visible:         false,
		nameInput:       nameInput,
		tagInput:        tagInput,
		guilds:          []EnhancedGuildData{},
		filteredGuilds:  []EnhancedGuildData{},
		scrollOffset:    0,
		hoveredIndex:    -1,
		selectedIndex:   -1,
		guildFilePath:   "guilds.json",
		modalX:          modalX,
		modalY:          modalY,
		modalWidth:      modalWidth,
		modalHeight:     modalHeight,
		justOpened:      false,
		framesSinceOpen: 0,
		statusMessageManager: StatusMessageManager{
			Messages: []StatusMessage{},
		},
		// Initialize performance caches
		guildByNameTag: make(map[string]*EnhancedGuildData),
		guildByName:    make(map[string]*EnhancedGuildData),
		guildByTag:     make(map[string]*EnhancedGuildData),
		colorCache:     make(map[string]color.RGBA),
		cachesDirty:    true,
		// Initialize API import state
		apiImportInProgress: false,
	}

	// Initialize color picker
	colorPickerWidth := 400
	colorPickerHeight := 350
	colorPickerX := (screenW - colorPickerWidth) / 2
	colorPickerY := (screenH - colorPickerHeight) / 2
	gm.colorPicker = NewColorPicker(colorPickerX, colorPickerY, colorPickerWidth, colorPickerHeight)

	// Load guilds from file
	gm.loadGuildsFromFile()

	// Set the singleton instance
	enhancedGuildManagerInstance = gm

	return gm
}

// Show makes the guild manager visible
func (gm *EnhancedGuildManager) Show() {
	fmt.Printf("[GUILD_MANAGER] Showing guild manager\n")
	gm.visible = true
	gm.nameInput.Focused = true
	gm.justOpened = true   // Set flag to prevent initial character input
	gm.framesSinceOpen = 0 // Reset frame counter

	// Clear any existing text in the name input to prevent 'g' character
	gm.nameInput.Value = ""
	gm.nameInput.cursorPos = 0
	gm.nameInput.selStart = -1
	gm.nameInput.selEnd = -1

	// Also clear tag input
	gm.tagInput.Value = ""
	gm.tagInput.cursorPos = 0
	gm.tagInput.selStart = -1
	gm.tagInput.selEnd = -1

	gm.filterGuilds()

	// Center the modal on screen
	screenW, screenH := ebiten.WindowSize()
	gm.modalX = (screenW - gm.modalWidth) / 2
	gm.modalY = (screenH - gm.modalHeight) / 2

	// Update input positions
	gm.nameInput.X = gm.modalX + 30
	gm.nameInput.Y = gm.modalY + 80
	gm.tagInput.X = gm.modalX + 240
	gm.tagInput.Y = gm.modalY + 80
}

// Hide makes the guild manager invisible
func (gm *EnhancedGuildManager) Hide() {
	fmt.Printf("[GUILD_MANAGER] Hiding guild manager\n")
	gm.visible = false
	gm.nameInput.Focused = false
	gm.tagInput.Focused = false
}

// IsVisible returns true if the guild manager is visible
func (gm *EnhancedGuildManager) IsVisible() bool {
	return gm.visible
}

// Update handles input and updates the guild manager state
func (gm *EnhancedGuildManager) Update() bool {
	if !gm.visible {
		return false
	}

	// Update status messages
	gm.statusMessageManager.Update()

	// Update color picker first (highest priority)
	if gm.colorPicker.IsVisible() {
		return gm.colorPicker.Update()
	}

	// Get mouse position
	mx, my := ebiten.CursorPosition()

	// Check if mouse is in modal area
	inModal := mx >= gm.modalX && mx <= gm.modalX+gm.modalWidth &&
		my >= gm.modalY && my <= gm.modalY+gm.modalHeight

	// Always block mouse wheel events and clicks when visible
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		// Only scroll if mouse is in modal and not dragging scrollbar
		if inModal && !gm.scrollbarDragging {
			// Check if scrollbar is needed
			maxVisibleItems := (gm.modalHeight - 180) / 35
			if len(gm.filteredGuilds) > maxVisibleItems {
				// Handle scrollbar interaction first
				scrollbarX := gm.modalX + gm.modalWidth - 15
				scrollbarWidth := 8

				// If mouse is over scrollbar area, don't do wheel scrolling
				if mx >= scrollbarX && mx <= scrollbarX+scrollbarWidth {
					// Don't scroll with wheel when hovering over scrollbar
				} else {
					// Normal wheel scrolling
					gm.scrollOffset -= int(wheelY * 3)
					if gm.scrollOffset < 0 {
						gm.scrollOffset = 0
					}
					maxOffset := len(gm.filteredGuilds) - maxVisibleItems
					if maxOffset < 0 {
						maxOffset = 0
					}
					if gm.scrollOffset > maxOffset {
						gm.scrollOffset = maxOffset
					}
				}
			}
		}
		return true // Block wheel event regardless of modal position
	}

	// Handle scrollbar dragging
	if gm.scrollbarDragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		maxVisibleItems := (gm.modalHeight - 180) / 35
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarHeight := gm.modalHeight - 160
			deltaY := my - gm.dragStartY

			// Calculate scroll change based on drag distance
			if scrollbarHeight > 0 {
				maxOffset := len(gm.filteredGuilds) - maxVisibleItems
				scrollDelta := int(float64(deltaY) / float64(scrollbarHeight) * float64(maxOffset))
				newScrollOffset := gm.dragStartOffset + scrollDelta

				if newScrollOffset < 0 {
					newScrollOffset = 0
				} else if newScrollOffset > maxOffset {
					newScrollOffset = maxOffset
				}
				gm.scrollOffset = newScrollOffset
			}
		}
		return true
	}

	// Stop dragging when mouse is released
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		gm.scrollbarDragging = false
	}

	// Handle mouse clicks when visible
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !inModal {
			gm.Hide()
			return true
		}

		// Check for X button click first (high priority)
		closeButtonSize := 24
		closeButtonX := gm.modalX + gm.modalWidth - closeButtonSize - 15
		closeButtonY := gm.modalY + 15
		mx, my := ebiten.CursorPosition()

		if mx >= closeButtonX && mx < closeButtonX+closeButtonSize &&
			my >= closeButtonY && my < closeButtonY+closeButtonSize {
			gm.Hide()
			return true
		}

		// Handle scrollbar clicks (high priority)
		maxVisibleItems := (gm.modalHeight - 180) / 35
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarX := gm.modalX + gm.modalWidth - 15
			scrollbarWidth := 8
			scrollbarHeight := gm.modalHeight - 160
			scrollbarY := gm.modalY + 140

			if mx >= scrollbarX && mx <= scrollbarX+scrollbarWidth &&
				my >= scrollbarY && my <= scrollbarY+scrollbarHeight {

				// Calculate thumb position and size
				thumbHeight := scrollbarHeight * maxVisibleItems / len(gm.filteredGuilds)
				if thumbHeight < 20 {
					thumbHeight = 20
				}

				maxOffset := len(gm.filteredGuilds) - maxVisibleItems
				thumbY := scrollbarY + (scrollbarHeight-thumbHeight)*gm.scrollOffset/maxOffset

				// Check if click is on thumb
				if my >= thumbY && my <= thumbY+thumbHeight {
					// Start dragging thumb
					gm.scrollbarDragging = true
					gm.dragStartY = my
					gm.dragStartOffset = gm.scrollOffset
				} else {
					// Click on track - jump to position
					relativeY := my - scrollbarY
					newScrollOffset := int(float64(relativeY) / float64(scrollbarHeight) * float64(maxOffset))
					if newScrollOffset < 0 {
						newScrollOffset = 0
					} else if newScrollOffset > maxOffset {
						newScrollOffset = maxOffset
					}
					gm.scrollOffset = newScrollOffset
				}
				return true
			}
		}

		// If inside modal, continue processing below (don't return true here)
	}

	// Process key events from input manager
	if gm.keyEventCh != nil {
		for {
			select {
			case event := <-gm.keyEventCh:
				if event.Pressed && event.Key == ebiten.KeyEscape {
					gm.Hide()
					return true
				}
			default:
				// No more events to process
				goto keyEventsProcessed
			}
		}
	}
keyEventsProcessed:

	// Clear the just opened flag and manage frame counter to prevent 'g' character input
	skipCharInput := false
	if gm.justOpened || gm.framesSinceOpen < 2 {
		skipCharInput = true
		gm.framesSinceOpen++
		// Consume any pending character input to prevent 'g' from appearing
		ebiten.AppendInputChars(nil)
		if gm.framesSinceOpen >= 2 {
			gm.justOpened = false
		}
	}

	// Update input fields - skip character input if just opened
	nameChanged := gm.nameInput.UpdateWithSkipInput(skipCharInput)
	tagChanged := gm.tagInput.UpdateWithSkipInput(skipCharInput)

	// Handle focus management for text inputs
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()

		// Check if clicked on name input
		nameClicked := mx >= gm.nameInput.X && mx < gm.nameInput.X+gm.nameInput.Width &&
			my >= gm.nameInput.Y && my < gm.nameInput.Y+gm.nameInput.Height

		// Check if clicked on tag input
		tagClicked := mx >= gm.tagInput.X && mx < gm.tagInput.X+gm.tagInput.Width &&
			my >= gm.tagInput.Y && my < gm.tagInput.Y+gm.tagInput.Height

		// Update focus based on clicks
		if nameClicked {
			gm.tagInput.clearSelection() // Clear selection from tag input when switching
			gm.nameInput.Focused = true
			gm.tagInput.Focused = false
			gm.nameInput.cursorBlink = time.Now()
			// TODO: Set cursor position based on click position
			gm.nameInput.cursorPos = len(gm.nameInput.Value)
			gm.nameInput.selStart = -1
			gm.nameInput.selEnd = -1
		} else if tagClicked {
			gm.nameInput.clearSelection() // Clear selection from name input when switching
			gm.nameInput.Focused = false
			gm.tagInput.Focused = true
			gm.tagInput.cursorBlink = time.Now()
			// TODO: Set cursor position based on click position
			gm.tagInput.cursorPos = len(gm.tagInput.Value)
			gm.tagInput.selStart = -1
			gm.tagInput.selEnd = -1
		}
		// If clicked elsewhere (handled by main modal logic), don't change focus here
	}

	// Ensure only one input is focused at a time (backup check)
	if gm.nameInput.Focused && gm.tagInput.Focused {
		// If both somehow got focused, clear the tag input focus
		gm.tagInput.Focused = false
	}

	// If any of the search fields changed, update filtered guilds
	if nameChanged || tagChanged {
		gm.filterGuilds()
	}

	// Handle Tab key for switching between inputs
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if gm.nameInput.Focused {
			gm.nameInput.clearSelection() // Clear selection when losing focus
			gm.nameInput.Focused = false
			gm.tagInput.Focused = true
		} else {
			gm.tagInput.clearSelection() // Clear selection when losing focus
			gm.tagInput.Focused = false
			gm.nameInput.Focused = true
		}
		return true
	}

	// Handle Enter key to add guild
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if gm.nameInput.Focused || gm.tagInput.Focused {
			gm.addGuild(gm.nameInput.Value, gm.tagInput.Value)
			return true
		}
	}

	// Reset hovered index
	gm.hoveredIndex = -1

	// Handle item interactions only if mouse is in modal
	if inModal {
		// Check for hovering/clicking on guild items
		if len(gm.filteredGuilds) > 0 {
			itemHeight := 35
			listStartY := gm.modalY + 140

			// Calculate which items are visible based on scroll offset
			maxVisibleItems := (gm.modalHeight - 180) / itemHeight

			for i := 0; i < len(gm.filteredGuilds) && i < maxVisibleItems; i++ {
				itemIndex := i + gm.scrollOffset
				if itemIndex >= len(gm.filteredGuilds) {
					break
				}

				itemY := listStartY + (i * itemHeight)

				// Guild item area
				itemRect := Rect{
					X:      gm.modalX + 20,
					Y:      itemY,
					Width:  gm.modalWidth - 80,
					Height: itemHeight,
				}

				// Remove button area
				removeRect := Rect{
					X:      gm.modalX + gm.modalWidth - 50,
					Y:      itemY + 5,
					Width:  25,
					Height: 25,
				}

				// Color button area
				colorRect := Rect{
					X:      gm.modalX + gm.modalWidth - 120,
					Y:      itemY + 7,
					Width:  20,
					Height: 20,
				}

				// Edit claim button area
				editClaimRect := Rect{
					X:      gm.modalX + gm.modalWidth - 90,
					Y:      itemY + 5,
					Width:  35,
					Height: 25,
				}

				// Check if hovering over color button first (higher priority)
				if mx >= colorRect.X && mx < colorRect.X+colorRect.Width &&
					my >= colorRect.Y && my < colorRect.Y+colorRect.Height {

					// Handle click on color button
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						gm.openColorPickerForGuild(itemIndex)
						return true
					}
				}

				// Check if hovering over edit claim button
				if mx >= editClaimRect.X && mx < editClaimRect.X+editClaimRect.Width &&
					my >= editClaimRect.Y && my < editClaimRect.Y+editClaimRect.Height {

					// Handle click on edit claim button
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						if gm.onEditClaim != nil {
							gm.onEditClaim(gm.filteredGuilds[itemIndex].Name, gm.filteredGuilds[itemIndex].Tag)
						}
						return true
					}
				}

				// Check if hovering over remove button
				if mx >= removeRect.X && mx < removeRect.X+removeRect.Width &&
					my >= removeRect.Y && my < removeRect.Y+removeRect.Height {

					// Handle click on remove button
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						gm.removeGuild(itemIndex)
						return true
					}
				}

				// Check if hovering over item (lower priority)
				if mx >= itemRect.X && mx < itemRect.X+itemRect.Width &&
					my >= itemRect.Y && my < itemRect.Y+itemRect.Height {
					gm.hoveredIndex = itemIndex

					// Handle click on item
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						gm.selectedIndex = itemIndex
						return true
					}
				}
			}
		}

		// Handle Clear button click
		clearButtonRect := Rect{
			X:      gm.modalX + gm.modalWidth - 270,
			Y:      gm.modalY + 80,
			Width:  80,
			Height: 35,
		}

		if mx >= clearButtonRect.X && mx < clearButtonRect.X+clearButtonRect.Width &&
			my >= clearButtonRect.Y && my < clearButtonRect.Y+clearButtonRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !gm.apiImportInProgress {
				// Clear all guilds only if not importing
				gm.clearAllGuilds()
				return true
			}
		}

		// Handle API Import button click
		apiImportButtonRect := Rect{
			X:      gm.modalX + gm.modalWidth - 180,
			Y:      gm.modalY + 80,
			Width:  90,
			Height: 35,
		}

		if mx >= apiImportButtonRect.X && mx < apiImportButtonRect.X+apiImportButtonRect.Width &&
			my >= apiImportButtonRect.Y && my < apiImportButtonRect.Y+apiImportButtonRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !gm.apiImportInProgress {
				// Run the API import only if not already importing
				go gm.runAPIImport()
				return true
			}
		}

		// Handle add button click
		addButtonRect := Rect{
			X:      gm.modalX + gm.modalWidth - 80,
			Y:      gm.modalY + 80,
			Width:  50,
			Height: 35,
		}

		if mx >= addButtonRect.X && mx < addButtonRect.X+addButtonRect.Width &&
			my >= addButtonRect.Y && my < addButtonRect.Y+addButtonRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !gm.apiImportInProgress {
				// Add guild only if not importing
				gm.addGuild(gm.nameInput.Value, gm.tagInput.Value)
				return true
			}
		}
	}

	// Block all input while visible
	return true
}

// Draw renders the enhanced guild manager with color picker support
func (gm *EnhancedGuildManager) Draw(screen *ebiten.Image) {
	if !gm.visible {
		return
	}

	// Draw background overlay
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH), EnhancedUIColors.Background, false)

	// Draw modal background with same style as territories side menu
	modalColor := EnhancedUIColors.ModalBackground
	vector.DrawFilledRect(screen, float32(gm.modalX), float32(gm.modalY), float32(gm.modalWidth), float32(gm.modalHeight), modalColor, false)

	// Draw border
	borderColor := EnhancedUIColors.Primary
	vector.StrokeRect(screen, float32(gm.modalX), float32(gm.modalY), float32(gm.modalWidth), float32(gm.modalHeight), 3, borderColor, false)

	// Draw top accent bar
	accentColor := EnhancedUIColors.Primary
	vector.DrawFilledRect(screen, float32(gm.modalX), float32(gm.modalY), float32(gm.modalWidth), 3, accentColor, false)

	// Load fonts
	titleFont := loadWynncraftFont(22)
	contentFont := loadWynncraftFont(16)

	// Get font offsets
	titleOffset := getFontVerticalOffset(22)
	contentOffset := getFontVerticalOffset(16)

	// Draw title
	titleY := gm.modalY + 35
	titleText := "Guild Management"
	text.Draw(screen, titleText, titleFont, gm.modalX+30, titleY+titleOffset, EnhancedUIColors.Text)

	// Draw close button (X)
	closeButtonSize := 24
	closeButtonX := gm.modalX + gm.modalWidth - closeButtonSize - 15
	closeButtonY := gm.modalY + 15
	mx, my := ebiten.CursorPosition()

	closeButtonColor := EnhancedUIColors.RemoveButton
	if mx >= closeButtonX && mx < closeButtonX+closeButtonSize &&
		my >= closeButtonY && my < closeButtonY+closeButtonSize {
		closeButtonColor = EnhancedUIColors.RemoveHover
	}

	vector.DrawFilledRect(screen, float32(closeButtonX), float32(closeButtonY), float32(closeButtonSize), float32(closeButtonSize), closeButtonColor, false)
	text.Draw(screen, "x", contentFont, closeButtonX+8, closeButtonY+contentOffset+12, EnhancedUIColors.Text)

	// Draw text inputs
	gm.nameInput.Draw(screen)
	gm.tagInput.Draw(screen)

	// Draw Clear button
	clearButtonRect := Rect{
		X:      gm.modalX + gm.modalWidth - 270,
		Y:      gm.modalY + 80,
		Width:  80,
		Height: 35,
	}

	clearButtonColor := color.RGBA{120, 60, 60, 255} // Dark red button color
	clearTextColor := EnhancedUIColors.Text

	if gm.apiImportInProgress {
		// Disabled state during import
		clearButtonColor = color.RGBA{60, 60, 60, 255}  // Dark grey when disabled
		clearTextColor = color.RGBA{100, 100, 100, 255} // Greyed out text
	} else if mx >= clearButtonRect.X && mx < clearButtonRect.X+clearButtonRect.Width &&
		my >= clearButtonRect.Y && my < clearButtonRect.Y+clearButtonRect.Height {
		clearButtonColor = color.RGBA{150, 80, 80, 255} // Lighter red on hover
	}

	vector.DrawFilledRect(screen, float32(clearButtonRect.X), float32(clearButtonRect.Y),
		float32(clearButtonRect.Width), float32(clearButtonRect.Height), clearButtonColor, false)

	vector.StrokeRect(screen, float32(clearButtonRect.X), float32(clearButtonRect.Y),
		float32(clearButtonRect.Width), float32(clearButtonRect.Height), 2, EnhancedUIColors.Border, false)

	clearText := "Clear"
	clearTextBounds := text.BoundString(contentFont, clearText)
	text.Draw(screen, clearText, contentFont,
		clearButtonRect.X+(clearButtonRect.Width-clearTextBounds.Dx())/2,
		clearButtonRect.Y+(clearButtonRect.Height+clearTextBounds.Dy())/2-2,
		clearTextColor)

	// Draw API Import button
	apiImportButtonRect := Rect{
		X:      gm.modalX + gm.modalWidth - 180,
		Y:      gm.modalY + 80,
		Width:  90,
		Height: 35,
	}

	apiImportButtonColor := color.RGBA{80, 80, 90, 255} // Secondary/Grey button color
	apiImportTextColor := EnhancedUIColors.Text
	apiImportText := "API Import"

	if gm.apiImportInProgress {
		// Disabled state during import
		apiImportButtonColor = color.RGBA{60, 60, 60, 255}  // Dark grey when disabled
		apiImportTextColor = color.RGBA{100, 100, 100, 255} // Greyed out text
		apiImportText = "Importing..."                      // Change text to show progress
	} else if mx >= apiImportButtonRect.X && mx < apiImportButtonRect.X+apiImportButtonRect.Width &&
		my >= apiImportButtonRect.Y && my < apiImportButtonRect.Y+apiImportButtonRect.Height {
		apiImportButtonColor = color.RGBA{100, 100, 110, 255} // Slightly lighter on hover
	}

	vector.DrawFilledRect(screen, float32(apiImportButtonRect.X), float32(apiImportButtonRect.Y),
		float32(apiImportButtonRect.Width), float32(apiImportButtonRect.Height), apiImportButtonColor, false)

	vector.StrokeRect(screen, float32(apiImportButtonRect.X), float32(apiImportButtonRect.Y),
		float32(apiImportButtonRect.Width), float32(apiImportButtonRect.Height), 2, EnhancedUIColors.Border, false)

	apiImportTextBounds := text.BoundString(contentFont, apiImportText)
	text.Draw(screen, apiImportText, contentFont,
		apiImportButtonRect.X+(apiImportButtonRect.Width-apiImportTextBounds.Dx())/2,
		apiImportButtonRect.Y+(apiImportButtonRect.Height+apiImportTextBounds.Dy())/2-2,
		apiImportTextColor)

	// Draw add button
	addButtonRect := Rect{
		X:      gm.modalX + gm.modalWidth - 80,
		Y:      gm.modalY + 80,
		Width:  50,
		Height: 35,
	}

	addButtonColor := EnhancedUIColors.Button
	addTextColor := EnhancedUIColors.Text
	addBorderColor := EnhancedUIColors.BorderGreen

	if gm.apiImportInProgress {
		// Disabled state during import
		addButtonColor = color.RGBA{60, 60, 60, 255}  // Dark grey when disabled
		addTextColor = color.RGBA{100, 100, 100, 255} // Greyed out text
		addBorderColor = color.RGBA{80, 80, 80, 255}  // Grey border when disabled
	} else if mx >= addButtonRect.X && mx < addButtonRect.X+addButtonRect.Width &&
		my >= addButtonRect.Y && my < addButtonRect.Y+addButtonRect.Height {
		addButtonColor = EnhancedUIColors.ButtonHover
	}

	vector.DrawFilledRect(screen, float32(addButtonRect.X), float32(addButtonRect.Y),
		float32(addButtonRect.Width), float32(addButtonRect.Height), addButtonColor, false)

	vector.StrokeRect(screen, float32(addButtonRect.X), float32(addButtonRect.Y),
		float32(addButtonRect.Width), float32(addButtonRect.Height), 2, addBorderColor, false)

	addText := "Add"
	addTextBounds := text.BoundString(contentFont, addText)
	text.Draw(screen, addText, contentFont,
		addButtonRect.X+(addButtonRect.Width-addTextBounds.Dx())/2,
		addButtonRect.Y+(addButtonRect.Height+addTextBounds.Dy())/2-2,
		addTextColor)

	// Draw guild list heading
	listHeading := fmt.Sprintf("Guilds (%d)", len(gm.filteredGuilds))
	text.Draw(screen, listHeading, contentFont, gm.modalX+30, gm.modalY+130+contentOffset, EnhancedUIColors.Text)

	// Draw guild list
	if len(gm.filteredGuilds) > 0 {
		itemHeight := 35
		listStartY := gm.modalY + 140

		// Calculate which items are visible based on scroll offset
		maxVisibleItems := (gm.modalHeight - 180) / itemHeight
		//fmt.Printf("[GUILD_RENDER] Rendering guild list: %d filtered guilds, offset=%d, maxVisible=%d\n",
		//	len(gm.filteredGuilds), gm.scrollOffset, maxVisibleItems)

		for i := 0; i < len(gm.filteredGuilds) && i < maxVisibleItems; i++ {
			itemIndex := i + gm.scrollOffset
			if itemIndex >= len(gm.filteredGuilds) {
				break
			}

			guild := gm.filteredGuilds[itemIndex]
			itemY := listStartY + (i * itemHeight)

			// Determine item background color
			bgColor := EnhancedUIColors.ItemBackground
			if itemIndex == gm.hoveredIndex {
				bgColor = EnhancedUIColors.ItemHover
			}
			if itemIndex == gm.selectedIndex {
				bgColor = EnhancedUIColors.ItemSelected
			}

			// Draw item background
			vector.DrawFilledRect(screen,
				float32(gm.modalX+20),
				float32(itemY),
				float32(gm.modalWidth-80),
				float32(itemHeight),
				bgColor, false)

			// Parse guild color
			r, g, b, ok := parseHexColor(guild.Color)
			var guildColor color.RGBA
			if ok {
				guildColor = color.RGBA{r, g, b, 255}
			} else {
				guildColor = EnhancedUIColors.Text // Default to white if invalid color
			}

			// Draw guild info with color
			guildText := fmt.Sprintf("%s [%s]", guild.Name, guild.Tag)
			text.Draw(screen, guildText, contentFont, gm.modalX+35, itemY+contentOffset+16, guildColor)

			// Draw color button
			colorButtonSize := 20
			colorButtonX := gm.modalX + gm.modalWidth - 120
			colorButtonY := itemY + 7

			colorButtonColor := guildColor
			vector.DrawFilledRect(screen,
				float32(colorButtonX),
				float32(colorButtonY),
				float32(colorButtonSize),
				float32(colorButtonSize),
				colorButtonColor, false)

			// Draw border around color button
			vector.StrokeRect(screen,
				float32(colorButtonX),
				float32(colorButtonY),
				float32(colorButtonSize),
				float32(colorButtonSize), 2, EnhancedUIColors.Border, false)

			// Draw edit claim button
			editClaimButtonRect := Rect{
				X:      gm.modalX + gm.modalWidth - 90,
				Y:      itemY + 5,
				Width:  35,
				Height: 25,
			}

			editClaimButtonColor := color.RGBA{60, 100, 180, 255} // Blue-ish button
			if mx >= editClaimButtonRect.X && mx < editClaimButtonRect.X+editClaimButtonRect.Width &&
				my >= editClaimButtonRect.Y && my < editClaimButtonRect.Y+editClaimButtonRect.Height {
				editClaimButtonColor = color.RGBA{80, 120, 200, 255} // Lighter blue on hover
			}

			vector.DrawFilledRect(screen,
				float32(editClaimButtonRect.X),
				float32(editClaimButtonRect.Y),
				float32(editClaimButtonRect.Width),
				float32(editClaimButtonRect.Height),
				editClaimButtonColor, false)

			vector.StrokeRect(screen,
				float32(editClaimButtonRect.X),
				float32(editClaimButtonRect.Y),
				float32(editClaimButtonRect.Width),
				float32(editClaimButtonRect.Height), 1, EnhancedUIColors.Border, false)

			// Draw "Edit" text in edit claim button
			text.Draw(screen, "Edit", contentFont, editClaimButtonRect.X+6, editClaimButtonRect.Y+contentOffset+12, EnhancedUIColors.Text)

			// Draw remove button
			removeRect := Rect{
				X:      gm.modalX + gm.modalWidth - 50,
				Y:      itemY + 5,
				Width:  25,
				Height: 25,
			}

			removeColor := EnhancedUIColors.RemoveButton
			if mx >= removeRect.X && mx < removeRect.X+removeRect.Width &&
				my >= removeRect.Y && my < removeRect.Y+removeRect.Height {
				removeColor = EnhancedUIColors.RemoveHover
			}

			vector.DrawFilledRect(screen,
				float32(removeRect.X),
				float32(removeRect.Y),
				float32(removeRect.Width),
				float32(removeRect.Height),
				removeColor, false)

			// Draw X in remove button
			text.Draw(screen, "x", contentFont, removeRect.X+8, removeRect.Y+contentOffset+12, EnhancedUIColors.Text)
		}

		// Draw scrollbar if needed
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarHeight := gm.modalHeight - 160
			thumbHeight := scrollbarHeight * maxVisibleItems / len(gm.filteredGuilds)
			thumbY := gm.modalY + 140 + (scrollbarHeight-thumbHeight)*gm.scrollOffset/(len(gm.filteredGuilds)-maxVisibleItems)

			// Draw scrollbar track
			vector.DrawFilledRect(screen,
				float32(gm.modalX+gm.modalWidth-15),
				float32(gm.modalY+140),
				float32(8),
				float32(scrollbarHeight),
				EnhancedUIColors.Border, false)

			// Draw scrollbar thumb
			vector.DrawFilledRect(screen,
				float32(gm.modalX+gm.modalWidth-15),
				float32(thumbY),
				float32(8),
				float32(thumbHeight),
				EnhancedUIColors.Primary, false)
		}
	} else {
		// Draw empty message
		emptyText := "No guilds found. Add one using the fields above."
		emptyBounds := text.BoundString(contentFont, emptyText)
		text.Draw(screen, emptyText, contentFont,
			gm.modalX+(gm.modalWidth-emptyBounds.Dx())/2,
			gm.modalY+200+contentOffset+12,
			EnhancedUIColors.TextSecondary)
	}

	// Draw color picker if visible
	if gm.colorPicker.IsVisible() {
		gm.colorPicker.Draw(screen)
	}

	// Draw status messages
	gm.statusMessageManager.Draw(screen, gm.modalX+30, gm.modalY+150, contentFont, EnhancedUIColors.Text)
}

// StatusMessage represents a temporary status message
type StatusMessage struct {
	Text      string
	StartTime time.Time
	Duration  time.Duration
}

// StatusMessageManager for the EnhancedGuildManager
type StatusMessageManager struct {
	Messages []StatusMessage
}

// AddMessage adds a new status message
func (sm *StatusMessageManager) AddMessage(text string, duration time.Duration) {
	sm.Messages = append(sm.Messages, StatusMessage{
		Text:      text,
		StartTime: time.Now(),
		Duration:  duration,
	})
}

// Update updates the status messages, removing expired ones
func (sm *StatusMessageManager) Update() {
	now := time.Now()
	var activeMessages []StatusMessage

	for _, msg := range sm.Messages {
		if now.Sub(msg.StartTime) < msg.Duration {
			activeMessages = append(activeMessages, msg)
		}
	}

	sm.Messages = activeMessages
}

// Draw draws the status messages
func (sm *StatusMessageManager) Draw(screen *ebiten.Image, x, y int, font font.Face, color color.Color) {
	lineHeight := 20
	for i, msg := range sm.Messages {
		messageY := y + (i * lineHeight)
		text.Draw(screen, msg.Text, font, x, messageY, color)
	}
}

// filterGuilds filters the guild list based on search criteria
func (gm *EnhancedGuildManager) filterGuilds() {
	gm.filteredGuilds = []EnhancedGuildData{}
	nameFilter := strings.ToLower(gm.nameInput.Value)
	tagFilter := strings.ToLower(gm.tagInput.Value)

	fmt.Printf("[GUILD_FILTER] Filtering %d guilds with nameFilter='%s', tagFilter='%s'\n",
		len(gm.guilds), nameFilter, tagFilter)

	for _, guild := range gm.guilds {
		nameMatch := nameFilter == "" || strings.Contains(strings.ToLower(guild.Name), nameFilter)
		tagMatch := tagFilter == "" || strings.Contains(strings.ToLower(guild.Tag), tagFilter)

		if nameMatch && tagMatch {
			gm.filteredGuilds = append(gm.filteredGuilds, guild)
		}
	}

	fmt.Printf("[GUILD_FILTER] Found %d matches\n", len(gm.filteredGuilds))

	// Reset scroll offset when filtering to ensure results are visible
	gm.scrollOffset = 0
}

// addGuild adds a new guild to the list
func (gm *EnhancedGuildManager) addGuild(name, tag string) {
	// Trim whitespace
	name = strings.TrimSpace(name)
	tag = strings.TrimSpace(tag)

	// Validate input
	if name == "" {
		return
	}

	// Validate tag (3-4 characters)
	if len(tag) < 3 || len(tag) > 4 {
		return
	}

	// Check for duplicates
	for _, guild := range gm.guilds {
		if strings.EqualFold(guild.Name, name) || strings.EqualFold(guild.Tag, tag) {
			return
		}
	}

	// Add new guild with default color
	newGuild := EnhancedGuildData{
		Name:  name,
		Tag:   tag,
		Color: "#FFAA00", // Default yellow color
	}

	gm.guilds = append(gm.guilds, newGuild)
	gm.cachesDirty = true // Invalidate caches

	// Clear input fields
	gm.nameInput.Value = ""
	gm.nameInput.cursorPos = 0
	gm.nameInput.selStart = -1
	gm.nameInput.selEnd = -1
	gm.tagInput.Value = ""
	gm.tagInput.cursorPos = 0
	gm.tagInput.selStart = -1
	gm.tagInput.selEnd = -1

	// Update filtered list
	gm.filterGuilds()

	// Save to file
	gm.saveGuildsToFile()

	// Show success message
	gm.statusMessageManager.AddMessage("Guild added successfully!", 3*time.Second)
	NewToast().AutoClose(3 * time.Second).Text("Guild added successfully!", ToastOption{}).Show()
}

// removeGuild removes a guild from the list
func (gm *EnhancedGuildManager) removeGuild(index int) {
	if index < 0 || index >= len(gm.filteredGuilds) {
		return
	}

	// Find the guild in the main list
	guildToRemove := gm.filteredGuilds[index]
	for i, guild := range gm.guilds {
		if guild.Name == guildToRemove.Name && guild.Tag == guildToRemove.Tag {
			// Remove from main list
			gm.guilds = append(gm.guilds[:i], gm.guilds[i+1:]...)
			gm.cachesDirty = true // Invalidate caches
			break
		}
	}

	// Update filtered list
	gm.filterGuilds()

	// Adjust selection if needed
	if gm.selectedIndex >= len(gm.filteredGuilds) {
		gm.selectedIndex = -1
	}

	// Save to file
	gm.saveGuildsToFile()

	// Show success message
	NewToast().AutoClose(3 * time.Second).Text("Guild removed successfully!", ToastOption{}).Show()
}

// loadGuildsFromFile loads guilds from JSON file
func (gm *EnhancedGuildManager) loadGuildsFromFile() {
	data, err := os.ReadFile(gm.guildFilePath)
	if err != nil {
		// File doesn't exist, start with empty list
		gm.guilds = []EnhancedGuildData{}
		gm.filteredGuilds = []EnhancedGuildData{}
		return
	}

	var guilds []EnhancedGuildData
	if err := json.Unmarshal(data, &guilds); err != nil {
		// Invalid JSON, start with empty list
		gm.guilds = []EnhancedGuildData{}
		gm.filteredGuilds = []EnhancedGuildData{}
		return
	}

	// Ensure all guilds have a color (for backward compatibility)
	for i := range guilds {
		if guilds[i].Color == "" {
			guilds[i].Color = "#FFAA00" // Default yellow
		}
	}

	gm.guilds = guilds
	gm.cachesDirty = true // Invalidate caches when loading new data
	gm.filteredGuilds = make([]EnhancedGuildData, len(guilds))
	copy(gm.filteredGuilds, guilds)
}

// saveGuildsToFile saves guilds to JSON file
func (gm *EnhancedGuildManager) saveGuildsToFile() {
	data, err := json.MarshalIndent(gm.guilds, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(gm.guildFilePath, data, 0644)

	// Notify territory system that guild data has changed
	if gm.onGuildDataChanged != nil {
		fmt.Printf("[GUILD_MANAGER] Guild data changed, calling callback to invalidate territory cache\n")
		gm.onGuildDataChanged()
	}
}

// openColorPickerForGuild opens the color picker for a specific guild
func (gm *EnhancedGuildManager) openColorPickerForGuild(guildIndex int) {
	if guildIndex < 0 || guildIndex >= len(gm.filteredGuilds) {
		return
	}

	guild := gm.filteredGuilds[guildIndex]

	// Set up callbacks
	gm.colorPicker.onConfirm = func(selectedColor color.RGBA) {
		gm.updateGuildColor(guildIndex, selectedColor.R, selectedColor.G, selectedColor.B)
	}

	gm.colorPicker.onCancel = func() {
		// Just close the color picker, no changes needed
	}

	// Show the color picker
	gm.colorPicker.Show(guildIndex, guild.Color)
}

// updateGuildColor updates a guild's color and saves to file
func (gm *EnhancedGuildManager) updateGuildColor(guildIndex int, r, g, b uint8) {
	if guildIndex < 0 || guildIndex >= len(gm.filteredGuilds) {
		return
	}

	// Find the guild in the filtered list
	guildToUpdate := gm.filteredGuilds[guildIndex]

	// Find and update the guild in the main list
	for i := range gm.guilds {
		if gm.guilds[i].Name == guildToUpdate.Name && gm.guilds[i].Tag == guildToUpdate.Tag {
			// Update color
			gm.guilds[i].Color = fmt.Sprintf("#%02X%02X%02X", r, g, b)
			gm.cachesDirty = true // Invalidate caches when color changes
			break
		}
	}

	// Update the filtered list as well
	gm.filteredGuilds[guildIndex].Color = fmt.Sprintf("#%02X%02X%02X", r, g, b)

	// Save to file
	gm.saveGuildsToFile()
}

// Draw method for EnhancedTextInput
func (t *EnhancedTextInput) Draw(screen *ebiten.Image) {
	// Draw input background
	bgColor := EnhancedUIColors.Surface
	if t.Focused {
		bgColor = EnhancedUIColors.Surface
	}

	vector.DrawFilledRect(screen, float32(t.X), float32(t.Y), float32(t.Width), float32(t.Height), bgColor, false)

	// Draw border
	borderColor := EnhancedUIColors.Border
	if t.Focused {
		borderColor = EnhancedUIColors.Primary
	}
	vector.StrokeRect(screen, float32(t.X), float32(t.Y), float32(t.Width), float32(t.Height), 2, borderColor, false)

	// Load font
	font := loadWynncraftFont(16)

	// Draw text or placeholder
	textX := t.X + 8
	textY := t.Y + (t.Height+16)/2 // Simplified positioning for better alignment

	if t.Value != "" {
		// Draw selection background if any
		if t.hasSelection() && t.Focused {
			start, end := t.getOrderedSelection()
			if start >= 0 && end <= len(t.Value) {
				// Calculate selection bounds (simplified)
				selectionColor := EnhancedUIColors.Selection
				vector.DrawFilledRect(screen,
					float32(textX+start*8), // Approximate character width
					float32(t.Y+4),
					float32((end-start)*8),
					float32(t.Height-8),
					selectionColor, false)
			}
		}

		// Draw actual text
		text.Draw(screen, t.Value, font, textX, textY, EnhancedUIColors.Text)

		// Draw cursor if focused
		if t.Focused && time.Since(t.cursorBlink).Milliseconds()%1000 < 500 {
			// Calculate precise cursor position using actual text width
			// Ensure cursor position is within bounds
			cursorPos := t.cursorPos
			if cursorPos > len(t.Value) {
				cursorPos = len(t.Value)
			}
			if cursorPos < 0 {
				cursorPos = 0
			}

			cursorText := t.Value[:cursorPos]
			cursorX := textX
			if len(cursorText) > 0 && font != nil {
				bounds := text.BoundString(font, cursorText)
				cursorX = textX + bounds.Dx()
			}
			// Draw blinking underscore cursor
			vector.DrawFilledRect(screen, float32(cursorX), float32(t.Y+t.Height-6), 8, 2, EnhancedUIColors.Text, false)
		}
	} else if !t.Focused {
		// Draw placeholder text
		text.Draw(screen, t.Placeholder, font, textX, textY, EnhancedUIColors.TextPlaceholder)
	} else {
		// Draw cursor at beginning when focused but empty
		if time.Since(t.cursorBlink).Milliseconds()%1000 < 500 {
			// Draw blinking underscore cursor at start position
			vector.DrawFilledRect(screen, float32(textX), float32(t.Y+t.Height-6), 8, 2, EnhancedUIColors.Text, false)
		}
	}
}

// GetEnhancedGuildManager returns the singleton instance of the Enhanced Guild Manager
var enhancedGuildManagerInstance *EnhancedGuildManager

func GetEnhancedGuildManager() *EnhancedGuildManager {
	return enhancedGuildManagerInstance
}

// SetEnhancedGuildManagerInstance sets the singleton instance of the Enhanced Guild Manager
func SetEnhancedGuildManagerInstance(instance *EnhancedGuildManager) {
	enhancedGuildManagerInstance = instance
}

// SetInputManager sets the input manager for the guild manager
func (gm *EnhancedGuildManager) SetInputManager(inputManager *InputManager) {
	if inputManager != nil {
		gm.keyEventCh = inputManager.Subscribe()
	}
}

// SetEditClaimCallback sets the callback function for edit claim button clicks
func (gm *EnhancedGuildManager) SetEditClaimCallback(callback func(guildName, guildTag string)) {
	gm.onEditClaim = callback
}

// SetGuildDataChangedCallback sets the callback function for when guild data changes
func (gm *EnhancedGuildManager) SetGuildDataChangedCallback(callback func()) {
	gm.onGuildDataChanged = callback
}

// SetText sets the text input's value and adjusts cursor position
func (t *EnhancedTextInput) SetText(text string) {
	t.Value = text
	// Ensure cursor position is within bounds
	if t.cursorPos > len(t.Value) {
		t.cursorPos = len(t.Value)
	}
	// Clear any selection when setting new text
	t.selStart = -1
	t.selEnd = -1
}

// GetText returns the text input's current value
func (t *EnhancedTextInput) GetText() string {
	return t.Value
}

// GetGuildByName returns guild data for a given guild name
func (gm *EnhancedGuildManager) GetGuildByName(name string) (*EnhancedGuildData, bool) {
	for _, guild := range gm.guilds {
		if guild.Name == name {
			return &guild, true
		}
	}
	return nil, false
}

// GetGuildByTag returns guild data for a given guild tag
func (gm *EnhancedGuildManager) GetGuildByTag(tag string) (*EnhancedGuildData, bool) {
	for _, guild := range gm.guilds {
		if guild.Tag == tag {
			return &guild, true
		}
	}
	return nil, false
}

// GetGuildColor returns the color for a guild by name and tag
func (gm *EnhancedGuildManager) GetGuildColor(name, tag string) (color.RGBA, bool) {
	// Ensure caches are up to date
	gm.ensureCachesValid()

	// Try to find by both name and tag (highest priority)
	if name != "" && tag != "" {
		key := name + "|" + tag
		if guild, found := gm.guildByNameTag[key]; found {
			if cachedColor, cached := gm.colorCache[guild.Color]; cached {
				return cachedColor, true
			}
			// Fallback to parsing if not cached
			if r, g, b, ok := parseHexColor(guild.Color); ok {
				color := color.RGBA{r, g, b, 255}
				gm.colorCache[guild.Color] = color // Cache for next time
				return color, true
			}
			return EnhancedUIColors.Text, false
		}
	}

	// If not found with both, try just by name
	if name != "" {
		if guild, found := gm.guildByName[name]; found {
			if cachedColor, cached := gm.colorCache[guild.Color]; cached {
				return cachedColor, true
			}
			// Fallback to parsing if not cached
			if r, g, b, ok := parseHexColor(guild.Color); ok {
				color := color.RGBA{r, g, b, 255}
				gm.colorCache[guild.Color] = color // Cache for next time
				return color, true
			}
			return EnhancedUIColors.Text, false
		}
	}

	// If still not found, try just by tag
	if tag != "" {
		if guild, found := gm.guildByTag[tag]; found {
			if cachedColor, cached := gm.colorCache[guild.Color]; cached {
				return cachedColor, true
			}
			// Fallback to parsing if not cached
			if r, g, b, ok := parseHexColor(guild.Color); ok {
				color := color.RGBA{r, g, b, 255}
				gm.colorCache[guild.Color] = color // Cache for next time
				return color, true
			}
			return EnhancedUIColors.Text, false
		}
	}

	return EnhancedUIColors.Text, false
}

// runAPIImport imports guilds and territories from the API
func (gm *EnhancedGuildManager) runAPIImport() {
	// Set import in progress flag
	gm.apiImportInProgress = true
	defer func() {
		// Always clear the flag when done
		gm.apiImportInProgress = false
	}()

	// Show a toast notification instead of status message
	NewToast().
		Text("Starting API import...", ToastOption{Colour: color.RGBA{100, 200, 255, 255}}).
		AutoClose(time.Second * 3).
		Show()

	// Import guilds first
	importedGuilds, skippedGuilds, err := gm.ImportGuildsFromAPI()
	if err != nil {
		errMsg := fmt.Sprintf("Error importing guilds: %v", err)
		fmt.Printf("[GUILD_MANAGER] %s\n", errMsg)

		// Show error toast with dismiss button
		NewToast().
			Text("Import Failed", ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			Text(errMsg, ToastOption{Colour: color.RGBA{255, 200, 200, 255}}).
			Button("Dismiss", func() {}, 0, 0, ToastOption{Colour: color.RGBA{255, 120, 120, 255}}).
			AutoClose(time.Second * 10).
			Show()
		return
	}

	// Show result message for guilds with success styling
	guildMsg := fmt.Sprintf("Imported %d guilds, skipped %d existing", importedGuilds, skippedGuilds)
	fmt.Printf("[GUILD_MANAGER] %s\n", guildMsg)

	NewToast().
		Text("Guild Import Complete", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text(guildMsg, ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		AutoClose(time.Second * 5).
		Show()

	// Import territories next
	importedTerritories, skippedTerritories, err := gm.ImportTerritoriesFromAPI()
	if err != nil {
		errMsg := fmt.Sprintf("Error importing territories: %v", err)
		fmt.Printf("[GUILD_MANAGER] %s\n", errMsg)

		// Show error toast with retry option
		NewToast().
			Text("Territory Import Failed", ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			Text(errMsg, ToastOption{Colour: color.RGBA{255, 200, 200, 255}}).
			Button("Retry", func() { go gm.runAPIImport() }, 0, 0, ToastOption{Colour: color.RGBA{255, 150, 100, 255}}).
			Button("Dismiss", func() {}, 0, 0, ToastOption{Colour: color.RGBA{255, 120, 120, 255}}).
			AutoClose(time.Second * 15).
			Show()
		return
	}

	// Show final success toast with statistics
	territoryMsg := fmt.Sprintf("Updated %d territory claims, skipped %d", importedTerritories, skippedTerritories)
	fmt.Printf("[GUILD_MANAGER] %s\n", territoryMsg)

	totalMsg := fmt.Sprintf("Import complete! %d guilds, %d territories", importedGuilds, importedTerritories)

	NewToast().
		Text("API Import Complete!", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text(totalMsg, ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		Text("Click to view details", ToastOption{
			Colour:    color.RGBA{150, 200, 255, 255},
			Underline: true,
			OnClick: func() {
				// Show detailed statistics in another toast
				NewToast().
					Text("Import Statistics", ToastOption{Colour: color.RGBA{255, 255, 100, 255}}).
					Text(guildMsg, ToastOption{Colour: color.RGBA{200, 200, 200, 255}}).
					Text(territoryMsg, ToastOption{Colour: color.RGBA{200, 200, 200, 255}}).
					AutoClose(time.Second * 10).
					Show()
			},
		}).
		AutoClose(time.Second * 8).
		Show()
}

// clearAllGuilds removes all guilds from the list and saves the empty list
func (gm *EnhancedGuildManager) clearAllGuilds() {
	// Clear the guilds slice
	gm.guilds = []EnhancedGuildData{}
	gm.cachesDirty = true // Invalidate caches when clearing
	gm.filteredGuilds = []EnhancedGuildData{}
	gm.selectedIndex = -1
	gm.hoveredIndex = -1
	gm.scrollOffset = 0

	// Save the empty list to file
	gm.saveGuildsToFile()

	// Show success message
	fmt.Println("[GUILD_MANAGER] All guilds cleared successfully")
	gm.statusMessageManager.AddMessage("All guilds cleared", 3*time.Second)
}

// rebuildCaches rebuilds the performance lookup caches
func (gm *EnhancedGuildManager) rebuildCaches() {
	// Clear existing caches
	gm.guildByNameTag = make(map[string]*EnhancedGuildData)
	gm.guildByName = make(map[string]*EnhancedGuildData)
	gm.guildByTag = make(map[string]*EnhancedGuildData)
	gm.colorCache = make(map[string]color.RGBA)

	// Rebuild caches from current guild data
	for i := range gm.guilds {
		guild := &gm.guilds[i]

		// Name+Tag combination (highest priority)
		if guild.Name != "" && guild.Tag != "" {
			key := guild.Name + "|" + guild.Tag
			gm.guildByNameTag[key] = guild
		}

		// Name only (fallback)
		if guild.Name != "" {
			// Only store if we don't already have a guild with this name
			// This preserves the first occurrence priority
			if _, exists := gm.guildByName[guild.Name]; !exists {
				gm.guildByName[guild.Name] = guild
			}
		}

		// Tag only (fallback)
		if guild.Tag != "" {
			// Only store if we don't already have a guild with this tag
			// This preserves the first occurrence priority
			if _, exists := gm.guildByTag[guild.Tag]; !exists {
				gm.guildByTag[guild.Tag] = guild
			}
		}

		// Pre-parse and cache colors
		if guild.Color != "" {
			if r, g, b, ok := parseHexColor(guild.Color); ok {
				gm.colorCache[guild.Color] = color.RGBA{r, g, b, 255}
			}
		}
	}

	gm.cachesDirty = false
}

// ensureCachesValid ensures the lookup caches are up to date
func (gm *EnhancedGuildManager) ensureCachesValid() {
	if gm.cachesDirty {
		gm.rebuildCaches()
	}
}

// HasTextInputFocused returns true if any text input in the guild manager is currently focused
func (gm *EnhancedGuildManager) HasTextInputFocused() bool {
	return gm.nameInput.Focused || gm.tagInput.Focused
}

// IsNameInputFocused returns true if the name input is currently focused
func (gm *EnhancedGuildManager) IsNameInputFocused() bool {
	return gm.nameInput.Focused
}

// IsTagInputFocused returns true if the tag input is currently focused
func (gm *EnhancedGuildManager) IsTagInputFocused() bool {
	return gm.tagInput.Focused
}
