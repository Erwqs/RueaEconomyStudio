package app

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image/color"
	"runtime"
	"sort"
	"strings"
	"time"

	"RueaES/storage"

	"RueaES/eruntime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
)

var guildColorCRC32Table = crc32.MakeTable(crc32.IEEE)

// EnhancedGuildData represents a guild entry
type EnhancedGuildData struct {
	Name  string `json:"name"`
	Tag   string `json:"tag"`
	Color string `json:"color"` // Hex color representation (e.g., "#FF0000")
	Show  bool   `json:"show"`
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
	contextMenu    *SelectionAnywhere
}

// TextInput creates a new enhanced text input
func TextInput(placeholder string, x, y, width, height, maxLength int) *EnhancedTextInput {
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

	if t.contextMenu != nil && t.contextMenu.IsVisible() {
		if t.contextMenu.Update() {
			return true
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		mx, my := ebiten.CursorPosition()
		if mx >= t.X && mx < t.X+t.Width && my >= t.Y && my < t.Y+t.Height {
			if !t.Focused {
				t.Focused = true
			}
			t.showContextMenu(mx, my)
			return true
		}
	}

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
						r == ' ' || r == '-' || r == '_' || r == '.' || r == ',' || r == '!' || r == '?' || r == '/' {
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
			if inpututil.IsKeyJustPressed(ebiten.KeyC) && t.hasSelection() && runtime.GOOS != "js" {
				start, end := t.getOrderedSelection()
				// Ensure indices are within bounds
				if start >= 0 && end <= len(t.Value) && start <= end {
					selectedText := t.Value[start:end]
					clipboard.Write(clipboard.FmtText, []byte(selectedText))
				}
				return true
			}

			// Paste (Ctrl+V)
			if inpututil.IsKeyJustPressed(ebiten.KeyV) && runtime.GOOS != "js" {
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
			if inpututil.IsKeyJustPressed(ebiten.KeyX) && t.hasSelection() && runtime.GOOS != "js" {
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

	sanitizedChanged, flagged := t.applyTextSanitization()
	if sanitizedChanged {
		changed = true
	}
	if flagged {
		showAdvertisingToast()
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

func (t *EnhancedTextInput) applyTextSanitization() (bool, bool) {
	sanitized, changed, flagged := sanitizeTextValue(t.Value)
	if changed {
		t.Value = sanitized
		if t.cursorPos > len(t.Value) {
			t.cursorPos = len(t.Value)
		}
	}
	return changed, flagged
}

// getOrderedSelection returns the selection start/end in correct order
func (t *EnhancedTextInput) getOrderedSelection() (int, int) {
	if t.selStart <= t.selEnd {
		return t.selStart, t.selEnd
	}
	return t.selEnd, t.selStart
}

func (t *EnhancedTextInput) showContextMenu(mx, my int) {
	menu := NewSelectionAnywhere()
	hasSelection := t.hasSelection()
	clipboardAvailable := runtime.GOOS != "js"
	canPaste := false
	if clipboardAvailable {
		if clipData := clipboard.Read(clipboard.FmtText); clipData != nil {
			canPaste = len(clipData) > 0
		}
	}

	menu.Option("Copy", "", hasSelection && clipboardAvailable, func() {
		if !clipboardAvailable || !t.hasSelection() {
			return
		}
		start, end := t.getOrderedSelection()
		if start >= 0 && end <= len(t.Value) && start <= end {
			selectedText := t.Value[start:end]
			clipboard.Write(clipboard.FmtText, []byte(selectedText))
		}
	})

	menu.Option("Cut", "", hasSelection && clipboardAvailable, func() {
		if !clipboardAvailable || !t.hasSelection() {
			return
		}
		oldValue := t.Value
		start, end := t.getOrderedSelection()
		if start >= 0 && end <= len(t.Value) && start <= end {
			selectedText := t.Value[start:end]
			clipboard.Write(clipboard.FmtText, []byte(selectedText))
			t.deleteSelection()
			t.applyContextValueChange(oldValue)
		}
	})

	menu.Option("Paste", "", canPaste && clipboardAvailable, func() {
		if !clipboardAvailable {
			return
		}
		clipData := clipboard.Read(clipboard.FmtText)
		if clipData == nil {
			return
		}
		clipText := string(clipData)
		if clipText == "" {
			return
		}
		oldValue := t.Value
		if t.hasSelection() {
			t.deleteSelection()
		}
		if t.MaxLength == 0 || len(t.Value)+len(clipText) <= t.MaxLength {
			if t.cursorPos < 0 {
				t.cursorPos = 0
			}
			if t.cursorPos > len(t.Value) {
				t.cursorPos = len(t.Value)
			}
			t.Value = t.Value[:t.cursorPos] + clipText + t.Value[t.cursorPos:]
			t.cursorPos += len(clipText)
			t.clearSelection()
			t.applyContextValueChange(oldValue)
		}
	})

	screenW, screenH := ebiten.WindowSize()
	menu.Show(mx, my, screenW, screenH)
	t.contextMenu = menu
	SetActiveContextMenu(menu)
}

func (t *EnhancedTextInput) applyContextValueChange(oldValue string) {
	sanitizedChanged, flagged := t.applyTextSanitization()
	if sanitizedChanged || oldValue != t.Value {
		// No callback here, but keep state consistent
	}
	if flagged {
		showAdvertisingToast()
	}
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
	noneGuildVisible   bool
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
	framesToIgnoreESC  int                              // Frames to ignore ESC after opening
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

	// Double-click tracking for guild items
	lastClickTime  time.Time
	lastClickIndex int

	// Territory listing modal
	territoryModalVisible bool
	territoryModalGuild   *EnhancedGuildData

	// Guild filtering options
	showOnMapGuildsOnly        bool // Toggle to show only guilds that have territories
	territoryList              []string
	territoryScrollOffset      int
	territoryHoveredIndex      int
	territorySelectedIndex     int
	territoryModalX            int
	territoryModalY            int
	territoryModalWidth        int
	territoryModalHeight       int
	territoryScrollbarDragging bool
	territoryDragStartY        int
	territoryDragStartOffset   int
	territoryLastClickTime     time.Time
	territoryLastClickIndex    int

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
	nameInput := TextInput("Enter guild name...", modalX+30, modalY+70, 200, 35, 50)
	tagInput := TextInput("Tag...", modalX+240, modalY+70, 50, 35, 4)

	gm := &EnhancedGuildManager{
		visible:          false,
		nameInput:        nameInput,
		tagInput:         tagInput,
		guilds:           []EnhancedGuildData{},
		filteredGuilds:   []EnhancedGuildData{},
		noneGuildVisible: true,
		scrollOffset:     0,
		hoveredIndex:     -1,
		selectedIndex:    -1,
		guildFilePath:    storage.DataFile("guilds.json"),
		modalX:           modalX,
		modalY:           modalY,
		modalWidth:       modalWidth,
		modalHeight:      modalHeight,
		justOpened:       false,
		framesSinceOpen:  0,
		statusMessageManager: StatusMessageManager{
			Messages: []StatusMessage{},
		},
		// Initialize double-click tracking
		lastClickTime:  time.Time{},
		lastClickIndex: -1,
		// Initialize territory modal
		territoryModalVisible:  false,
		territoryModalGuild:    nil,
		territoryList:          []string{},
		territoryScrollOffset:  0,
		territoryHoveredIndex:  -1,
		territorySelectedIndex: -1,
		territoryModalWidth:    500,
		territoryModalHeight:   400,

		// Initialize guild filtering
		showOnMapGuildsOnly:        false, // Start with showing all guilds
		territoryScrollbarDragging: false,
		territoryDragStartY:        0,
		territoryDragStartOffset:   0,
		territoryLastClickTime:     time.Time{},
		territoryLastClickIndex:    -1,
		// Initialize performance caches
		guildByNameTag: make(map[string]*EnhancedGuildData),
		guildByName:    make(map[string]*EnhancedGuildData),
		guildByTag:     make(map[string]*EnhancedGuildData),
		colorCache:     make(map[string]color.RGBA),
		cachesDirty:    true,
		// Initialize API import state
		apiImportInProgress: false,
	}

	// Initialize territory modal positioning
	gm.territoryModalX = (screenW - gm.territoryModalWidth) / 2
	gm.territoryModalY = (screenH - gm.territoryModalHeight) / 2

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

	// Register for guild change notifications from eruntime (but only reload, don't let eruntime overwrite our file)
	eruntime.SetGuildChangeCallback(func() {
		// fmt.Printf("[GUILD_MANAGER] Received guild change notification - checking for new guilds to add\n")
		gm.loadGuildsFromFile() // This will reload and merge any new guilds while preserving existing colors
	})

	// Register for specific guild change notifications (more efficient for HQ changes)
	eruntime.SetGuildSpecificChangeCallback(func(guildName string) {
		// fmt.Printf("[GUILD_MANAGER] Received specific guild change notification for guild: %s\n", guildName)
		// For specific guild changes (like HQ updates), we only need to refresh the visual state
		// No need to reload the entire guilds file since this is just a visual update
		// The actual data is already updated in eruntime
	})

	return gm
}

// Show makes the guild manager visible
func (gm *EnhancedGuildManager) Show() {
	// fmt.Printf("[GUILD_MANAGER] Showing guild manager\n")
	gm.visible = true
	gm.nameInput.Focused = true
	gm.justOpened = true      // Set flag to prevent initial character input
	gm.framesSinceOpen = 0    // Reset frame counter
	gm.framesToIgnoreESC = 10 // Ignore ESC for first 10 frames after opening

	// Clear any existing text in the name input to prevent 'g' character
	gm.nameInput.Value = ""
	gm.nameInput.cursorPos = 0
	gm.nameInput.selStart = -1
	gm.nameInput.selEnd = -1

	// Also clear tag input
	gm.tagInput.Value = ""
	gm.tagInput.cursorPos = 0
	gm.tagInput.selStart = -1

	// Clear/drain any pending key events from the channel to prevent accumulated ESC events
	if gm.keyEventCh != nil {
		for {
			select {
			case <-gm.keyEventCh:
				// Drain the channel
			default:
				// No more events to drain
				goto channelDrained
			}
		}
	}
channelDrained:
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
	// fmt.Printf("[GUILD_MANAGER] Hiding guild manager\n")
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

	// Update territory modal first (highest priority after color picker)
	if gm.territoryModalVisible {
		return gm.updateTerritoryModal()
	}

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
			maxVisibleItems := (gm.modalHeight - 215) / 35
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
		maxVisibleItems := (gm.modalHeight - 215) / 35
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarHeight := gm.modalHeight - 195
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
		maxVisibleItems := (gm.modalHeight - 215) / 35
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarX := gm.modalX + gm.modalWidth - 15
			scrollbarWidth := 8
			scrollbarHeight := gm.modalHeight - 195
			scrollbarY := gm.modalY + 175

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
					// Don't handle ESC here if territory modal is visible - it's already handled in updateTerritoryModal
					if !gm.territoryModalVisible {
						// Only allow ESC to close if we've waited enough frames
						if gm.framesToIgnoreESC <= 0 {
							gm.Hide()
							return true
						}
						// Otherwise ignore the ESC key
					}
					// If territory modal is visible, just consume the event without action
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

	// Decrement ESC ignore counter
	if gm.framesToIgnoreESC > 0 {
		gm.framesToIgnoreESC--
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
			listStartY := gm.modalY + 175

			// Calculate which items are visible based on scroll offset
			maxVisibleItems := (gm.modalHeight - 215) / itemHeight

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

				// Show/hide toggle area
				showToggleRect := Rect{
					X:      gm.modalX + gm.modalWidth - 190,
					Y:      itemY + 5,
					Width:  60,
					Height: 25,
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

				// Check show/hide toggle
				if mx >= showToggleRect.X && mx < showToggleRect.X+showToggleRect.Width &&
					my >= showToggleRect.Y && my < showToggleRect.Y+showToggleRect.Height {

					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						gm.setGuildVisibilityByIndex(itemIndex, !gm.filteredGuilds[itemIndex].Show)
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

					// Handle click on item with double-click detection
					if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
						currentTime := time.Now()

						// Check for double-click (within 500ms and same item)
						if itemIndex == gm.lastClickIndex && currentTime.Sub(gm.lastClickTime) < 500*time.Millisecond {
							// Double-click: show territory modal
							gm.showTerritoryModal(&gm.filteredGuilds[itemIndex])
						} else {
							// Single click: just select
							gm.selectedIndex = itemIndex
						}

						gm.lastClickIndex = itemIndex
						gm.lastClickTime = currentTime
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

		// Handle toggle button click
		toggleButtonRect := Rect{
			X:      gm.modalX + 30,
			Y:      gm.modalY + 125,
			Width:  120,
			Height: 25,
		}

		if mx >= toggleButtonRect.X && mx < toggleButtonRect.X+toggleButtonRect.Width &&
			my >= toggleButtonRect.Y && my < toggleButtonRect.Y+toggleButtonRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				// Toggle the filter
				gm.showOnMapGuildsOnly = !gm.showOnMapGuildsOnly
				// Re-filter the guilds with the new setting
				gm.filterGuilds()
				return true
			}
		}

		// Handle global show/hide button click
		globalToggleRect := Rect{
			X:      gm.modalX + 170,
			Y:      gm.modalY + 125,
			Width:  95,
			Height: 25,
		}

		if mx >= globalToggleRect.X && mx < globalToggleRect.X+globalToggleRect.Width &&
			my >= globalToggleRect.Y && my < globalToggleRect.Y+globalToggleRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				gm.setAllGuildVisibility(!gm.areAllGuildsVisible())
				return true
			}
		}

		// Handle invert visibility button click
		invertRect := Rect{
			X:      gm.modalX + 275,
			Y:      gm.modalY + 125,
			Width:  85,
			Height: 25,
		}

		if mx >= invertRect.X && mx < invertRect.X+invertRect.Width &&
			my >= invertRect.Y && my < invertRect.Y+invertRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				gm.invertGuildVisibility()
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

	// Draw On Map Only toggle button
	toggleButtonRect := Rect{
		X:      gm.modalX + 30,
		Y:      gm.modalY + 125,
		Width:  120,
		Height: 25,
	}

	toggleButtonColor := color.RGBA{70, 70, 80, 255} // Default grey
	toggleTextColor := EnhancedUIColors.Text
	toggleText := "Showing All Guilds"

	if gm.showOnMapGuildsOnly {
		toggleButtonColor = color.RGBA{80, 192, 80, 255} // Green when active
		toggleText = "Zinnig Mode On"
	}

	if mx >= toggleButtonRect.X && mx < toggleButtonRect.X+toggleButtonRect.Width &&
		my >= toggleButtonRect.Y && my < toggleButtonRect.Y+toggleButtonRect.Height {
		if gm.showOnMapGuildsOnly {
			toggleButtonColor = color.RGBA{100, 227, 100, 255} // Lighter green on hover
		} else {
			toggleButtonColor = color.RGBA{90, 90, 100, 255} // Lighter grey on hover
		}
	}

	vector.DrawFilledRect(screen, float32(toggleButtonRect.X), float32(toggleButtonRect.Y),
		float32(toggleButtonRect.Width), float32(toggleButtonRect.Height), toggleButtonColor, false)

	vector.StrokeRect(screen, float32(toggleButtonRect.X), float32(toggleButtonRect.Y),
		float32(toggleButtonRect.Width), float32(toggleButtonRect.Height), 2, EnhancedUIColors.Border, false)

	toggleTextBounds := text.BoundString(contentFont, toggleText)
	text.Draw(screen, toggleText, contentFont,
		toggleButtonRect.X+(toggleButtonRect.Width-toggleTextBounds.Dx())/2,
		toggleButtonRect.Y+(toggleButtonRect.Height+toggleTextBounds.Dy())/2-2,
		toggleTextColor)

	// Draw global visibility toggle
	allVisible := gm.areAllGuildsVisible()
	globalToggleText := "Hide All"
	if !allVisible {
		globalToggleText = "Show All"
	}

	globalToggleRect := Rect{
		X:      gm.modalX + 170,
		Y:      gm.modalY + 125,
		Width:  95,
		Height: 25,
	}

	globalToggleColor := color.RGBA{70, 70, 80, 255}
	if mx >= globalToggleRect.X && mx < globalToggleRect.X+globalToggleRect.Width &&
		my >= globalToggleRect.Y && my < globalToggleRect.Y+globalToggleRect.Height {
		globalToggleColor = color.RGBA{90, 90, 100, 255}
	}

	vector.DrawFilledRect(screen, float32(globalToggleRect.X), float32(globalToggleRect.Y),
		float32(globalToggleRect.Width), float32(globalToggleRect.Height), globalToggleColor, false)

	vector.StrokeRect(screen, float32(globalToggleRect.X), float32(globalToggleRect.Y),
		float32(globalToggleRect.Width), float32(globalToggleRect.Height), 2, EnhancedUIColors.Border, false)

	globalToggleBounds := text.BoundString(contentFont, globalToggleText)
	text.Draw(screen, globalToggleText, contentFont,
		globalToggleRect.X+(globalToggleRect.Width-globalToggleBounds.Dx())/2,
		globalToggleRect.Y+(globalToggleRect.Height+globalToggleBounds.Dy())/2-2,
		EnhancedUIColors.Text)

	// Draw invert visibility button
	invertRect := Rect{
		X:      gm.modalX + 275,
		Y:      gm.modalY + 125,
		Width:  85,
		Height: 25,
	}

	invertColor := color.RGBA{70, 70, 80, 255}
	if mx >= invertRect.X && mx < invertRect.X+invertRect.Width &&
		my >= invertRect.Y && my < invertRect.Y+invertRect.Height {
		invertColor = color.RGBA{90, 90, 100, 255}
	}

	vector.DrawFilledRect(screen, float32(invertRect.X), float32(invertRect.Y),
		float32(invertRect.Width), float32(invertRect.Height), invertColor, false)

	vector.StrokeRect(screen, float32(invertRect.X), float32(invertRect.Y),
		float32(invertRect.Width), float32(invertRect.Height), 2, EnhancedUIColors.Border, false)

	invertBounds := text.BoundString(contentFont, "Invert")
	text.Draw(screen, "Invert", contentFont,
		invertRect.X+(invertRect.Width-invertBounds.Dx())/2,
		invertRect.Y+(invertRect.Height+invertBounds.Dy())/2-2,
		EnhancedUIColors.Text)

	// Draw guild list heading
	listHeading := fmt.Sprintf("Guilds (%d)", len(gm.filteredGuilds))
	text.Draw(screen, listHeading, contentFont, gm.modalX+30, gm.modalY+165+contentOffset, EnhancedUIColors.Text)

	// Draw guild list
	if len(gm.filteredGuilds) > 0 {
		itemHeight := 35
		listStartY := gm.modalY + 175

		// Calculate which items are visible based on scroll offset
		maxVisibleItems := (gm.modalHeight - 215) / itemHeight
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

			// Draw show/hide toggle
			showToggleRect := Rect{
				X:      gm.modalX + gm.modalWidth - 190,
				Y:      itemY + 5,
				Width:  60,
				Height: 25,
			}

			showToggleColor := color.RGBA{70, 70, 80, 255}
			showToggleText := "Hidden"
			if guild.Show {
				showToggleColor = color.RGBA{80, 192, 80, 255}
				showToggleText = "Shown"
			}

			if mx >= showToggleRect.X && mx < showToggleRect.X+showToggleRect.Width &&
				my >= showToggleRect.Y && my < showToggleRect.Y+showToggleRect.Height {
				if guild.Show {
					showToggleColor = color.RGBA{100, 227, 100, 255}
				} else {
					showToggleColor = color.RGBA{90, 90, 100, 255}
				}
			}

			vector.DrawFilledRect(screen,
				float32(showToggleRect.X),
				float32(showToggleRect.Y),
				float32(showToggleRect.Width),
				float32(showToggleRect.Height),
				showToggleColor, false)

			vector.StrokeRect(screen,
				float32(showToggleRect.X),
				float32(showToggleRect.Y),
				float32(showToggleRect.Width),
				float32(showToggleRect.Height), 1, EnhancedUIColors.Border, false)

			showTextBounds := text.BoundString(contentFont, showToggleText)
			text.Draw(screen, showToggleText, contentFont,
				showToggleRect.X+(showToggleRect.Width-showTextBounds.Dx())/2,
				showToggleRect.Y+(showToggleRect.Height+showTextBounds.Dy())/2-2,
				EnhancedUIColors.Text)

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
			scrollbarHeight := gm.modalHeight - 195
			thumbHeight := scrollbarHeight * maxVisibleItems / len(gm.filteredGuilds)
			thumbY := gm.modalY + 175 + (scrollbarHeight-thumbHeight)*gm.scrollOffset/(len(gm.filteredGuilds)-maxVisibleItems)

			// Draw scrollbar track
			vector.DrawFilledRect(screen,
				float32(gm.modalX+gm.modalWidth-15),
				float32(gm.modalY+175),
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

	// Draw territory modal if visible (should be drawn last to be on top)
	gm.drawTerritoryModal(screen)
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

// guildHasTerritories checks if a guild has any territories on the map
func (gm *EnhancedGuildManager) guildHasTerritories(guildName, guildTag string) bool {
	territories := eruntime.GetTerritories()
	for _, territory := range territories {
		if territory != nil &&
			(strings.EqualFold(territory.Guild.Name, guildName) ||
				strings.EqualFold(territory.Guild.Tag, guildTag)) {
			return true
		}
	}
	return false
}

// filterGuilds filters the guild list based on search criteria
func (gm *EnhancedGuildManager) filterGuilds() {
	gm.filteredGuilds = []EnhancedGuildData{}
	nameFilter := strings.ToLower(gm.nameInput.Value)
	tagFilter := strings.ToLower(gm.tagInput.Value)

	// fmt.Printf("[GUILD_FILTER] Filtering %d guilds with nameFilter='%s', tagFilter='%s'\n",
	// len(gm.guilds), nameFilter, tagFilter)

	for _, guild := range gm.guilds {
		nameMatch := nameFilter == "" || strings.Contains(strings.ToLower(guild.Name), nameFilter)
		tagMatch := tagFilter == "" || strings.Contains(strings.ToLower(guild.Tag), tagFilter)
		onMapMatch := !gm.showOnMapGuildsOnly || gm.guildHasTerritories(guild.Name, guild.Tag)

		if nameMatch && tagMatch && onMapMatch {
			gm.filteredGuilds = append(gm.filteredGuilds, guild)
		}
	}

	// fmt.Printf("[GUILD_FILTER] Found %d matches\n", len(gm.filteredGuilds))

	// Reset scroll offset when filtering to ensure results are visible
	gm.scrollOffset = 0
}

func colourFor(guildName string) string {
	checksum := crc32.Checksum([]byte(guildName), guildColorCRC32Table)
	return fmt.Sprintf("%08x", checksum)[2:]
}

func (gm *EnhancedGuildManager) addGuild(name, tag string) {
	name, _, nameFlagged := sanitizeTextValue(name)
	tag, _, tagFlagged := sanitizeTextValue(tag)
	if nameFlagged || tagFlagged {
		showAdvertisingToast()
	}

	// Validate input
	if name == "" {
		return
	}

	// Validate tag (3-4 characters)
	if len(tag) < 3 || len(tag) > 4 {
		return
	}
	if strings.EqualFold(tag, "NONE") {
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
		Color: "#" + colourFor(name),
		Show:  true,
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
	NewToast().AutoClose(3*time.Second).Text("Guild added successfully!", ToastOption{}).Show()
}

// removeGuild removes a guild from the list
func (gm *EnhancedGuildManager) removeGuild(index int) {
	if index < 0 || index >= len(gm.filteredGuilds) {
		return
	}

	// Never remove the synthetic None guild entry
	if strings.EqualFold(gm.filteredGuilds[index].Tag, "NONE") {
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
	NewToast().AutoClose(3*time.Second).Text("Guild removed successfully!", ToastOption{}).Show()
}

// loadGuildsFromFile loads guilds from JSON file and merges with guilds from eruntime
func (gm *EnhancedGuildManager) loadGuildsFromFile() {
	data, err := storage.ReadDataFile("guilds.json")
	if err != nil {
		// File doesn't exist, start with empty list
		gm.guilds = []EnhancedGuildData{}
		gm.filteredGuilds = []EnhancedGuildData{}
	} else {
		type guildFileEntry struct {
			Name  string `json:"name"`
			Tag   string `json:"tag"`
			Color string `json:"color"`
			Show  *bool  `json:"show"`
		}

		var guilds []guildFileEntry
		if err := json.Unmarshal(data, &guilds); err != nil {
			// Invalid JSON, start with empty list
			gm.guilds = []EnhancedGuildData{}
			gm.filteredGuilds = []EnhancedGuildData{}
		} else {
			converted := make([]EnhancedGuildData, 0, len(guilds))
			for _, entry := range guilds {
				show := true
				if entry.Show != nil {
					show = *entry.Show
				}
				color := entry.Color
				if color == "" {
					color = "#FFAA00" // Default yellow
				}

				converted = append(converted, EnhancedGuildData{
					Name:  entry.Name,
					Tag:   entry.Tag,
					Color: color,
					Show:  show,
				})
			}
			gm.guilds = converted
		}
	}

	gm.ensureNoneGuildEntry()

	// Create maps of existing guilds for fast lookup (preserves local colors)
	existingGuildsByTag := make(map[string]*EnhancedGuildData)
	existingGuildsByName := make(map[string]*EnhancedGuildData)
	for i := range gm.guilds {
		existingGuildsByTag[gm.guilds[i].Tag] = &gm.guilds[i]
		existingGuildsByName[gm.guilds[i].Name] = &gm.guilds[i]
	}

	// Get all guilds from eruntime and merge any new ones
	allGuildNames := eruntime.GetAllGuilds()
	newGuildsAdded := false

	for _, guildDisplay := range allGuildNames {
		// Skip "No Guild [NONE]" as it's not a real guild
		if guildDisplay == "No Guild [NONE]" {
			continue
		}

		// Parse guild name to extract tag (assumes format "Name [TAG]")
		var guildName, guildTag string
		if strings.Contains(guildDisplay, " [") && strings.HasSuffix(guildDisplay, "]") {
			// Extract name and tag from "Name [TAG]" format
			lastBracket := strings.LastIndex(guildDisplay, " [")
			guildName = guildDisplay[:lastBracket]
			guildTag = guildDisplay[lastBracket+2 : len(guildDisplay)-1] // Remove " [" and "]"
		} else {
			guildName = guildDisplay
			guildTag = ""
		}

		// If a local guild exists with the same name, skip adding (local takes precedence)
		if _, exists := existingGuildsByName[guildName]; exists {
			continue
		}
		// If a local guild exists with the same tag (and tag is not empty), skip adding
		if guildTag != "" {
			if _, exists := existingGuildsByTag[guildTag]; exists {
				continue
			}
		}

		// New guild from eruntime - add it with a default color
		newGuild := EnhancedGuildData{
			Name:  guildName,
			Tag:   guildTag,
			Color: "#FFAA00", // Default yellow color for new guilds
			Show:  true,
		}
		gm.guilds = append(gm.guilds, newGuild)
		newGuildsAdded = true
		// fmt.Printf("[GUILD_MANAGER] Added new guild from eruntime: %s [%s]\n", guildName, guildTag)
	}

	// If we added new guilds, save the updated list
	if newGuildsAdded {
		gm.saveGuildsToFile()
		// fmt.Printf("[GUILD_MANAGER] Saved updated guild list with new guilds from state file\n")
	}

	gm.cachesDirty = true // Invalidate caches when loading new data
	gm.filteredGuilds = make([]EnhancedGuildData, len(gm.guilds))
	copy(gm.filteredGuilds, gm.guilds)
}

// ensureNoneGuildEntry keeps the synthetic "No Guild" entry in sync and defaults to visible when missing
func (gm *EnhancedGuildManager) ensureNoneGuildEntry() {
	// found := false

	for i := range gm.guilds {
		if strings.EqualFold(gm.guilds[i].Tag, "NONE") {
			// found = true
			if gm.guilds[i].Name == "" {
				gm.guilds[i].Name = "No Guild"
			}
			if gm.guilds[i].Color == "" {
				gm.guilds[i].Color = "#888888"
			}
			gm.noneGuildVisible = gm.guilds[i].Show
			return
		}
	}

	// If not present, append a default entry and mirror current visibility flag
	gm.guilds = append(gm.guilds, EnhancedGuildData{
		Name:  "No Guild",
		Tag:   "NONE",
		Color: "#888888",
		Show:  gm.noneGuildVisible,
	})
}

// saveGuildsToFile saves guilds to JSON file
func (gm *EnhancedGuildManager) saveGuildsToFile() {
	gm.ensureNoneGuildEntry()

	data, err := json.MarshalIndent(gm.guilds, "", "  ")
	if err != nil {
		return
	}

	storage.WriteDataFile("guilds.json", data, 0644)

	// Notify territory system that guild data has changed
	if gm.onGuildDataChanged != nil {
		// fmt.Printf("[GUILD_MANAGER] Guild data changed, calling callback to invalidate territory cache\n")
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

// showTerritoryModal opens the territory listing modal for a specific guild
func (gm *EnhancedGuildManager) showTerritoryModal(guild *EnhancedGuildData) {
	gm.territoryModalVisible = true
	gm.territoryModalGuild = guild
	gm.territoryScrollOffset = 0
	gm.territoryHoveredIndex = -1
	gm.territorySelectedIndex = -1

	// Get territory claims for this guild
	claimManager := GetGuildClaimManager()
	if claimManager != nil {
		claims := claimManager.GetClaimsForGuild(guild.Name, guild.Tag)
		gm.territoryList = make([]string, 0, len(claims))
		for territoryName := range claims {
			gm.territoryList = append(gm.territoryList, territoryName)
		}
	} else {
		gm.territoryList = []string{}
	}

	// Sort the territory list for consistent display
	sort.Strings(gm.territoryList)

	// Center the modal on screen
	screenW, screenH := ebiten.WindowSize()
	gm.territoryModalX = (screenW - gm.territoryModalWidth) / 2
	gm.territoryModalY = (screenH - gm.territoryModalHeight) / 2
}

// hideTerritoryModal closes the territory listing modal
func (gm *EnhancedGuildManager) hideTerritoryModal() {
	gm.territoryModalVisible = false
	gm.territoryModalGuild = nil
	gm.territoryList = []string{}
	gm.territoryScrollOffset = 0
	gm.territoryHoveredIndex = -1
	gm.territorySelectedIndex = -1
}

// updateTerritoryModal handles input and updates for the territory modal
func (gm *EnhancedGuildManager) updateTerritoryModal() bool {
	// Handle ESC key to close territory modal - also drain key event channel to prevent double handling
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// Drain any ESC events from the key event channel to prevent main guild manager from also processing
		if gm.keyEventCh != nil {
			for {
				select {
				case event := <-gm.keyEventCh:
					// Consume ESC events without processing them
					if event.Pressed && event.Key == ebiten.KeyEscape {
						// Just consume, don't process
					}
				default:
					// No more events to drain
					goto drained
				}
			}
		}
	drained:
		gm.hideTerritoryModal()
		return true
	}

	mx, my := ebiten.CursorPosition()

	// Check if mouse is in territory modal area
	inTerritoryModal := mx >= gm.territoryModalX && mx <= gm.territoryModalX+gm.territoryModalWidth &&
		my >= gm.territoryModalY && my <= gm.territoryModalY+gm.territoryModalHeight

	// Handle mouse wheel scrolling
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 && inTerritoryModal {
		maxVisibleItems := (gm.territoryModalHeight - 120) / 30
		if len(gm.territoryList) > maxVisibleItems {
			gm.territoryScrollOffset -= int(wheelY * 3)
			if gm.territoryScrollOffset < 0 {
				gm.territoryScrollOffset = 0
			}
			maxOffset := len(gm.territoryList) - maxVisibleItems
			if maxOffset < 0 {
				maxOffset = 0
			}
			if gm.territoryScrollOffset > maxOffset {
				gm.territoryScrollOffset = maxOffset
			}
		}
	}

	// Handle scrollbar dragging for territory modal
	if gm.territoryScrollbarDragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		maxVisibleItems := (gm.territoryModalHeight - 120) / 30
		if len(gm.territoryList) > maxVisibleItems {
			scrollbarHeight := gm.territoryModalHeight - 120 // Match with drawing coordinates
			deltaY := my - gm.territoryDragStartY

			if scrollbarHeight > 0 {
				maxOffset := len(gm.territoryList) - maxVisibleItems

				// Calculate thumb constraints
				thumbHeight := (maxVisibleItems * scrollbarHeight) / len(gm.territoryList)
				if thumbHeight < 20 {
					thumbHeight = 20
				}
				maxThumbY := scrollbarHeight - thumbHeight

				scrollDelta := int(float64(deltaY) / float64(maxThumbY) * float64(maxOffset))
				newScrollOffset := gm.territoryDragStartOffset + scrollDelta

				if newScrollOffset < 0 {
					newScrollOffset = 0
				} else if newScrollOffset > maxOffset {
					newScrollOffset = maxOffset
				}
				gm.territoryScrollOffset = newScrollOffset
			}
		}
		return true
	}

	// Stop dragging when mouse is released
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		gm.territoryScrollbarDragging = false
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !inTerritoryModal {
			// Clicked outside territory modal, close it
			gm.hideTerritoryModal()
			return true
		}

		// Check for close button click
		closeButtonSize := 24
		closeButtonX := gm.territoryModalX + gm.territoryModalWidth - closeButtonSize - 15
		closeButtonY := gm.territoryModalY + 15

		if mx >= closeButtonX && mx < closeButtonX+closeButtonSize &&
			my >= closeButtonY && my < closeButtonY+closeButtonSize {
			gm.hideTerritoryModal()
			return true
		}

		// Handle scrollbar clicks
		maxVisibleItems := (gm.territoryModalHeight - 120) / 30
		if len(gm.territoryList) > maxVisibleItems {
			// Match coordinates with drawing code: listX+listWidth-20
			listX := gm.territoryModalX + 20
			listWidth := gm.territoryModalWidth - 40
			scrollbarX := listX + listWidth - 20
			scrollbarWidth := 16
			scrollbarY := gm.territoryModalY + 80
			scrollbarHeight := gm.territoryModalHeight - 120

			if mx >= scrollbarX && mx <= scrollbarX+scrollbarWidth &&
				my >= scrollbarY && my <= scrollbarY+scrollbarHeight {

				// Calculate thumb position and size
				thumbHeight := (maxVisibleItems * scrollbarHeight) / len(gm.territoryList)
				if thumbHeight < 20 {
					thumbHeight = 20
				}

				maxOffset := len(gm.territoryList) - maxVisibleItems
				maxThumbY := scrollbarHeight - thumbHeight
				thumbY := scrollbarY + (gm.territoryScrollOffset*maxThumbY)/(maxOffset)

				// Check if click is on thumb
				if my >= thumbY && my <= thumbY+thumbHeight {
					// Start dragging thumb
					gm.territoryScrollbarDragging = true
					gm.territoryDragStartY = my
					gm.territoryDragStartOffset = gm.territoryScrollOffset
				} else {
					// Click on track - jump to position
					relativeY := my - scrollbarY
					newScrollOffset := int(float64(relativeY) / float64(scrollbarHeight) * float64(maxOffset))
					if newScrollOffset < 0 {
						newScrollOffset = 0
					} else if newScrollOffset > maxOffset {
						newScrollOffset = maxOffset
					}
					gm.territoryScrollOffset = newScrollOffset
				}
				return true
			}
		}

		// Handle territory item clicks
		gm.territoryHoveredIndex = -1
		if len(gm.territoryList) > 0 {
			itemHeight := 30
			listStartY := gm.territoryModalY + 80

			maxVisibleItems := (gm.territoryModalHeight - 120) / itemHeight
			for i := 0; i < len(gm.territoryList) && i < maxVisibleItems; i++ {
				itemIndex := i + gm.territoryScrollOffset
				if itemIndex >= len(gm.territoryList) {
					break
				}

				itemY := listStartY + (i * itemHeight)

				// Territory item area
				itemRect := Rect{
					X:      gm.territoryModalX + 20,
					Y:      itemY,
					Width:  gm.territoryModalWidth - 60,
					Height: itemHeight,
				}

				if mx >= itemRect.X && mx < itemRect.X+itemRect.Width &&
					my >= itemRect.Y && my < itemRect.Y+itemRect.Height {

					currentTime := time.Now()

					// Check for double-click (within 500ms and same item)
					if itemIndex == gm.territoryLastClickIndex && currentTime.Sub(gm.territoryLastClickTime) < 500*time.Millisecond {
						// Double-click: show EdgeMenu for territory
						gm.showEdgeMenuForTerritory(gm.territoryList[itemIndex])
					} else {
						// Single click: just select
						gm.territorySelectedIndex = itemIndex
					}

					gm.territoryLastClickIndex = itemIndex
					gm.territoryLastClickTime = currentTime
					return true
				}
			}
		}
	}

	// Update hover state for territory items
	gm.territoryHoveredIndex = -1
	if inTerritoryModal && len(gm.territoryList) > 0 {
		itemHeight := 30
		listStartY := gm.territoryModalY + 80

		maxVisibleItems := (gm.territoryModalHeight - 120) / itemHeight
		for i := 0; i < len(gm.territoryList) && i < maxVisibleItems; i++ {
			itemIndex := i + gm.territoryScrollOffset
			if itemIndex >= len(gm.territoryList) {
				break
			}

			itemY := listStartY + (i * itemHeight)

			// Territory item area
			itemRect := Rect{
				X:      gm.territoryModalX + 20,
				Y:      itemY,
				Width:  gm.territoryModalWidth - 60,
				Height: itemHeight,
			}

			if mx >= itemRect.X && mx < itemRect.X+itemRect.Width &&
				my >= itemRect.Y && my < itemRect.Y+itemRect.Height {
				gm.territoryHoveredIndex = itemIndex
				break
			}
		}
	}

	// Block all input while territory modal is visible
	return true
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
	value, _, flagged := sanitizeTextValue(t.Value)
	if flagged {
		showAdvertisingToast()
	}
	t.Value = value
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

// GetAllGuildColors returns all guild colors in a format suitable for state saving
func (gm *EnhancedGuildManager) GetAllGuildColors() map[string]map[string]string {
	gm.ensureCachesValid()
	result := make(map[string]map[string]string)

	for _, guild := range gm.guilds {
		result[guild.Name] = map[string]string{
			"tag":   guild.Tag,
			"color": guild.Color,
		}
	}

	return result
}

// SetAllGuildColors sets guild colors from a state load operation
func (gm *EnhancedGuildManager) SetAllGuildColors(guildColors map[string]map[string]string) {
	// Clear existing guilds
	gm.guilds = []EnhancedGuildData{}

	// Add guilds from color data
	for name, data := range guildColors {
		tag := data["tag"]
		color := data["color"]

		gm.guilds = append(gm.guilds, EnhancedGuildData{
			Name:  name,
			Tag:   tag,
			Color: color,
			Show:  true,
		})
	}

	gm.ensureNoneGuildEntry()
	gm.cachesDirty = true
	gm.filterGuilds()
	gm.saveGuildsToFile()
}

// MergeGuildColors merges guild colors from a state load operation
func (gm *EnhancedGuildManager) MergeGuildColors(guildColors map[string]map[string]string) {
	gm.ensureCachesValid()

	for name, data := range guildColors {
		tag := data["tag"]
		color := data["color"]

		// Check if guild already exists
		existing := false
		for i, guild := range gm.guilds {
			if guild.Name == name || guild.Tag == tag {
				// Update existing guild
				gm.guilds[i].Name = name
				gm.guilds[i].Tag = tag
				gm.guilds[i].Color = color
				existing = true
				break
			}
		}

		// Add new guild if it doesn't exist
		if !existing {
			gm.guilds = append(gm.guilds, EnhancedGuildData{
				Name:  name,
				Tag:   tag,
				Color: color,
				Show:  true,
			})
		}
	}

	gm.ensureNoneGuildEntry()
	gm.cachesDirty = true
	gm.filterGuilds()
	gm.saveGuildsToFile()
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

// IsGuildVisible reports whether a guild (or the synthetic NONE guild) should be shown on the map
func (gm *EnhancedGuildManager) IsGuildVisible(name, tag string) bool {
	if gm == nil {
		return true
	}

	// Treat empty tag/name and NONE as the synthetic no-guild entry
	if (name == "" && tag == "") || strings.EqualFold(tag, "NONE") {
		return gm.noneGuildVisible
	}

	gm.ensureCachesValid()

	if name != "" && tag != "" {
		if guild, found := gm.guildByNameTag[name+"|"+tag]; found {
			return guild.Show
		}
	}

	if name != "" {
		if guild, found := gm.guildByName[name]; found {
			return guild.Show
		}
	}

	if tag != "" {
		if guild, found := gm.guildByTag[tag]; found {
			return guild.Show
		}
	}

	return true
}

// setGuildVisibilityByIndex updates the visibility flag for a guild in both filtered and main lists
func (gm *EnhancedGuildManager) setGuildVisibilityByIndex(guildIndex int, visible bool) {
	if guildIndex < 0 || guildIndex >= len(gm.filteredGuilds) {
		return
	}

	target := gm.filteredGuilds[guildIndex]

	for i := range gm.guilds {
		if gm.guilds[i].Name == target.Name && gm.guilds[i].Tag == target.Tag {
			gm.guilds[i].Show = visible
			if strings.EqualFold(gm.guilds[i].Tag, "NONE") {
				gm.noneGuildVisible = visible
			}
			break
		}
	}

	gm.filteredGuilds[guildIndex].Show = visible
	gm.cachesDirty = true
	gm.saveGuildsToFile()
}

// setAllGuildVisibility sets the same visibility across all guilds (including NONE)
func (gm *EnhancedGuildManager) setAllGuildVisibility(visible bool) {
	gm.ensureNoneGuildEntry()

	for i := range gm.guilds {
		gm.guilds[i].Show = visible
		if strings.EqualFold(gm.guilds[i].Tag, "NONE") {
			gm.noneGuildVisible = visible
		}
	}

	gm.cachesDirty = true
	gm.filterGuilds()
	gm.saveGuildsToFile()
}

// invertGuildVisibility flips the visibility flag for every guild
func (gm *EnhancedGuildManager) invertGuildVisibility() {
	gm.ensureNoneGuildEntry()

	for i := range gm.guilds {
		gm.guilds[i].Show = !gm.guilds[i].Show
		if strings.EqualFold(gm.guilds[i].Tag, "NONE") {
			gm.noneGuildVisible = gm.guilds[i].Show
		}
	}

	gm.cachesDirty = true
	gm.filterGuilds()
	gm.saveGuildsToFile()
}

// areAllGuildsVisible returns true if every guild (including NONE) is visible
func (gm *EnhancedGuildManager) areAllGuildsVisible() bool {
	gm.ensureNoneGuildEntry()

	for i := range gm.guilds {
		if !gm.guilds[i].Show {
			return false
		}
	}
	return true
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
		// fmt.Printf("[GUILD_MANAGER] %s\n", errMsg)

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
	// fmt.Printf("[GUILD_MANAGER] %s\n", guildMsg)

	NewToast().
		Text("Guild Import Complete", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		Text(guildMsg, ToastOption{Colour: color.RGBA{200, 255, 200, 255}}).
		AutoClose(time.Second * 5).
		Show()

	// Import territories next
	importedTerritories, skippedTerritories, err := gm.ImportTerritoriesFromAPI()
	if err != nil {
		errMsg := fmt.Sprintf("Error importing territories: %v", err)
		// fmt.Printf("[GUILD_MANAGER] %s\n", errMsg)

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
	// fmt.Printf("[GUILD_MANAGER] %s\n", territoryMsg)

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
	gm.noneGuildVisible = true
	gm.ensureNoneGuildEntry()
	gm.cachesDirty = true // Invalidate caches when clearing
	gm.filteredGuilds = []EnhancedGuildData{}
	gm.selectedIndex = -1
	gm.hoveredIndex = -1
	gm.scrollOffset = 0

	// Rebuild filtered list to include the synthetic None entry
	gm.filterGuilds()

	// Save the empty list to file
	gm.saveGuildsToFile()

	// Show success message
	// fmt.Println("[GUILD_MANAGER] All guilds cleared successfully")
	NewToast().Text("All guilds cleared!", ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
		AutoClose(3 * time.Second).
		Show()
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

// showEdgeMenuForTerritory shows the EdgeMenu for a specific territory
func (gm *EnhancedGuildManager) showEdgeMenuForTerritory(territoryName string) {
	// Get the global MapView instance
	mapView := GetMapView()
	if mapView == nil {
		// fmt.Printf("[GUILD_MANAGER] MapView not available\n")
		return
	}

	// Center the map on the territory
	mapView.CenterTerritory(territoryName)

	// Set selected territory for blinking effect
	if mapView.territoriesManager != nil {
		mapView.territoriesManager.SetSelectedTerritory(territoryName)
	}

	// Show EdgeMenu with territory information
	if mapView.edgeMenu != nil {
		mapView.OpenTerritoryMenu(territoryName)
		// fmt.Printf("[GUILD_MANAGER] Opened EdgeMenu for territory: %s\n", territoryName)

		// Hide the entire guild manager so EdgeMenu is visible
		gm.Hide()
	} else {
		// fmt.Printf("[GUILD_MANAGER] EdgeMenu not available\n")
	}
}

// drawTerritoryModal renders the territory modal on screen
func (gm *EnhancedGuildManager) drawTerritoryModal(screen *ebiten.Image) {
	if !gm.territoryModalVisible {
		return
	}

	// Get screen dimensions
	screenW, screenH := ebiten.WindowSize()

	// Draw dimming background
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH), color.RGBA{0, 0, 0, 128}, false)

	// Draw modal background
	vector.DrawFilledRect(screen, float32(gm.territoryModalX), float32(gm.territoryModalY), float32(gm.territoryModalWidth), float32(gm.territoryModalHeight), color.RGBA{30, 30, 30, 255}, false)

	// Draw modal border
	vector.StrokeRect(screen, float32(gm.territoryModalX), float32(gm.territoryModalY), float32(gm.territoryModalWidth), float32(gm.territoryModalHeight), 2, color.RGBA{100, 100, 100, 255}, false)

	// Get font
	font := loadWynncraftFont(16)

	// Draw title
	if gm.territoryModalGuild != nil && font != nil {
		title := fmt.Sprintf("Territories owned by %s [%s]", gm.territoryModalGuild.Name, gm.territoryModalGuild.Tag)
		text.Draw(screen, title, font, gm.territoryModalX+20, gm.territoryModalY+30, color.RGBA{255, 255, 255, 255})
	}

	// Draw territory count
	if len(gm.territoryList) > 0 && font != nil {
		countText := fmt.Sprintf("Total: %d territories", len(gm.territoryList))
		text.Draw(screen, countText, font, gm.territoryModalX+20, gm.territoryModalY+55, color.RGBA{200, 200, 200, 255})
	}

	// Draw territory list area background
	listX := gm.territoryModalX + 20
	listY := gm.territoryModalY + 80
	listWidth := gm.territoryModalWidth - 40
	listHeight := gm.territoryModalHeight - 120

	vector.DrawFilledRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight), color.RGBA{25, 25, 25, 255}, false)
	vector.StrokeRect(screen, float32(listX), float32(listY), float32(listWidth), float32(listHeight), 1, color.RGBA{100, 100, 100, 255}, false)

	// Draw territory items
	if len(gm.territoryList) > 0 && font != nil {
		itemHeight := 30
		maxVisibleItems := listHeight / itemHeight

		for i := 0; i < maxVisibleItems && (i+gm.territoryScrollOffset) < len(gm.territoryList); i++ {
			territoryIndex := i + gm.territoryScrollOffset
			territoryName := gm.territoryList[territoryIndex]

			itemY := listY + (i * itemHeight)

			// Determine item colors
			var bgColor color.RGBA
			if territoryIndex == gm.territorySelectedIndex {
				bgColor = color.RGBA{70, 120, 200, 255} // Selected - blue
			} else if territoryIndex == gm.territoryHoveredIndex {
				bgColor = color.RGBA{60, 60, 60, 255} // Hovered - dark grey
			} else {
				bgColor = color.RGBA{35, 35, 35, 255} // Normal - darker grey
			}

			// Draw item background
			vector.DrawFilledRect(screen, float32(listX+2), float32(itemY), float32(listWidth-4), float32(itemHeight), bgColor, false)

			// Draw territory name
			text.Draw(screen, territoryName, font, listX+10, itemY+20, color.RGBA{255, 255, 255, 255})
		}
	} else if font != nil {
		// Draw "No territories" message
		text.Draw(screen, "No territories owned", font, listX+20, listY+30, color.RGBA{150, 150, 150, 255})
	}

	// Draw scrollbar if needed
	if len(gm.territoryList) > 0 {
		maxVisibleItems := listHeight / 30
		if len(gm.territoryList) > maxVisibleItems {
			gm.drawTerritoryScrollbar(screen, listX+listWidth-20, listY, 16, listHeight, maxVisibleItems)
		}
	}

	// Draw close button (X in top-right corner)
	closeX := gm.territoryModalX + gm.territoryModalWidth - 30
	closeY := gm.territoryModalY + 10
	vector.DrawFilledRect(screen, float32(closeX), float32(closeY), 20, 20, color.RGBA{150, 50, 50, 255}, false)

	// Draw X
	if font != nil {
		text.Draw(screen, "×", font, closeX+6, closeY+15, color.RGBA{255, 255, 255, 255})
	}
}

// drawTerritoryScrollbar draws a vertical scrollbar for the territory list
func (gm *EnhancedGuildManager) drawTerritoryScrollbar(screen *ebiten.Image, x, y, width, height, maxVisibleItems int) {
	if len(gm.territoryList) <= maxVisibleItems {
		return
	}

	// Draw scrollbar track
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), color.RGBA{40, 40, 40, 255}, false)

	// Calculate thumb size and position
	thumbHeight := (maxVisibleItems * height) / len(gm.territoryList)
	if thumbHeight < 20 {
		thumbHeight = 20
	}

	maxThumbY := height - thumbHeight
	thumbY := (gm.territoryScrollOffset * maxThumbY) / (len(gm.territoryList) - maxVisibleItems)

	// Draw scrollbar thumb
	vector.DrawFilledRect(screen, float32(x+2), float32(y+thumbY), float32(width-4), float32(thumbHeight), color.RGBA{100, 100, 100, 255}, false)
}
