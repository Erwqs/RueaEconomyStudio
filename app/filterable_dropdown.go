package app

import (
	"image"
	"image/color"
	"runtime"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.design/x/clipboard"
)

// FilterableDropdownOption represents an option in the filterable dropdown
type FilterableDropdownOption struct {
	Display string
	Value   string
	Data    interface{}
}

// FilterableDropdown represents a dropdown with text input filtering
type FilterableDropdown struct {
	X, Y          int
	Width, Height int

	// Options and filtering
	allOptions      []FilterableDropdownOption
	filteredOptions []FilterableDropdownOption
	selectedIndex   int

	// Text input state
	inputText    string
	InputFocused bool
	cursorPos    int
	cursorBlink  int
	selStart     int // Selection start position (-1 if no selection)
	selEnd       int // Selection end position (-1 if no selection)

	// Dropdown state
	IsOpen          bool
	hoveredIndex    int
	maxVisibleItems int
	scrollOffset    int
	placeholder     string

	// Visual properties
	backgroundColor color.RGBA
	borderColor     color.RGBA
	textColor       color.RGBA
	hoverColor      color.RGBA
	selectedColor   color.RGBA

	// Callbacks
	onSelected func(option FilterableDropdownOption)

	// Boundary constraints
	containerBounds image.Rectangle
	openUpward      bool
}

var dropdownOverlayQueue []*FilterableDropdown

// NewFilterableDropdown creates a new filterable dropdown component
func NewFilterableDropdown(x, y, width, height int, options []FilterableDropdownOption, onSelected func(FilterableDropdownOption)) *FilterableDropdown {
	fd := &FilterableDropdown{
		X:               x,
		Y:               y,
		Width:           width,
		Height:          height,
		allOptions:      options,
		filteredOptions: make([]FilterableDropdownOption, len(options)),
		selectedIndex:   -1,
		inputText:       "",
		InputFocused:    false,
		cursorPos:       0,
		cursorBlink:     0,
		selStart:        -1,
		selEnd:          -1,
		IsOpen:          false,
		hoveredIndex:    -1,
		maxVisibleItems: 8,
		scrollOffset:    0,
		backgroundColor: color.RGBA{40, 40, 40, 255},
		borderColor:     color.RGBA{100, 100, 100, 255},
		textColor:       color.RGBA{200, 200, 200, 255},
		hoverColor:      color.RGBA{80, 80, 80, 255},
		selectedColor:   color.RGBA{100, 150, 255, 100},
		onSelected:      onSelected,
		openUpward:      false,
		placeholder:     "Select or type...",
	}

	// Copy all options to filtered initially
	copy(fd.filteredOptions, options)

	return fd
}

// SetContainerBounds sets the boundary constraints for the dropdown
func (fd *FilterableDropdown) SetContainerBounds(bounds image.Rectangle) {
	fd.containerBounds = bounds
	fd.calculateOpenDirection()
}

// calculateOpenDirection determines whether to open upward or downward
func (fd *FilterableDropdown) calculateOpenDirection() {
	if fd.containerBounds.Empty() {
		fd.openUpward = false
		return
	}

	// Calculate space available below and above
	spaceBelow := fd.containerBounds.Max.Y - (fd.Y + fd.Height)
	spaceAbove := fd.Y - fd.containerBounds.Min.Y

	// Calculate required space for dropdown
	visibleItems := len(fd.filteredOptions)
	if visibleItems > fd.maxVisibleItems {
		visibleItems = fd.maxVisibleItems
	}
	requiredSpace := visibleItems * fd.Height

	// Decide direction based on available space
	if spaceBelow >= requiredSpace {
		fd.openUpward = false
	} else if spaceAbove >= requiredSpace {
		fd.openUpward = true
	} else {
		// Neither direction has enough space, use the direction with more space
		if spaceBelow >= spaceAbove {
			fd.openUpward = false
		} else {
			fd.openUpward = true
		}
	}
}

// filterOptions filters the options based on the input text
func (fd *FilterableDropdown) filterOptions() {
	if fd.inputText == "" {
		fd.filteredOptions = make([]FilterableDropdownOption, len(fd.allOptions))
		copy(fd.filteredOptions, fd.allOptions)
	} else {
		fd.filteredOptions = fd.filteredOptions[:0] // Clear slice
		filterText := strings.ToLower(fd.inputText)

		for _, option := range fd.allOptions {
			if strings.Contains(strings.ToLower(option.Display), filterText) ||
				strings.Contains(strings.ToLower(option.Value), filterText) {
				fd.filteredOptions = append(fd.filteredOptions, option)
			}
		}
	}

	// Reset hover and scroll when filtering
	fd.hoveredIndex = -1
	fd.scrollOffset = 0
}

// Update handles input for the filterable dropdown
func (fd *FilterableDropdown) Update(mx, my int) bool {
	// Update cursor blink
	fd.cursorBlink++

	// Check if mouse is over dropdown header
	headerBounds := image.Rect(fd.X, fd.Y, fd.X+fd.Width, fd.Y+fd.Height)
	mouseOverHeader := mx >= headerBounds.Min.X && mx <= headerBounds.Max.X &&
		my >= headerBounds.Min.Y && my <= headerBounds.Max.Y

	// Handle click on dropdown header
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mouseOverHeader {
			fd.InputFocused = true
			// Select all text when clicked (for easy replacement)
			if len(fd.inputText) > 0 {
				fd.selStart = 0
				fd.selEnd = len(fd.inputText)
				fd.cursorPos = fd.selEnd
			}
			fd.calculateOpenDirection()
			fd.IsOpen = !fd.IsOpen
			if fd.IsOpen {
				fd.filterOptions()
			}
			return true // Handled input
		}

		// Check if clicking on dropdown items when open
		if fd.IsOpen && len(fd.filteredOptions) > 0 {
			visibleItems := len(fd.filteredOptions) - fd.scrollOffset
			if visibleItems > fd.maxVisibleItems {
				visibleItems = fd.maxVisibleItems
			}

			for i := 0; i < visibleItems; i++ {
				actualIndex := i + fd.scrollOffset
				if actualIndex >= len(fd.filteredOptions) {
					break
				}

				var itemY int
				if fd.openUpward {
					itemY = fd.Y - (visibleItems-i)*fd.Height
				} else {
					itemY = fd.Y + fd.Height + i*fd.Height
				}

				itemBounds := image.Rect(fd.X, itemY, fd.X+fd.Width, itemY+fd.Height)

				if mx >= itemBounds.Min.X && mx <= itemBounds.Max.X &&
					my >= itemBounds.Min.Y && my <= itemBounds.Max.Y {
					// Item clicked
					fd.selectedIndex = actualIndex
					fd.inputText = fd.filteredOptions[actualIndex].Display
					fd.cursorPos = len(fd.inputText)
					fd.IsOpen = false
					fd.InputFocused = false
					if fd.onSelected != nil {
						fd.onSelected(fd.filteredOptions[actualIndex])
					}
					return true // Handled input
				}
			}
		}

		// Click outside dropdown, close it
		if !mouseOverHeader {
			fd.IsOpen = false
			fd.InputFocused = false
		}
	}

	// Handle text input if focused
	if fd.InputFocused {
		// Handle character input
		chars := ebiten.AppendInputChars(nil)
		for _, r := range chars {
			// Allow alphanumeric, space, and common punctuation
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
				r == ' ' || r == '-' || r == '_' || r == '[' || r == ']' || r == '(' || r == ')' {
				// If there's a selection, delete it first
				if fd.hasSelection() {
					fd.deleteSelection()
				}
				// Insert character at cursor position
				fd.inputText = fd.inputText[:fd.cursorPos] + string(r) + fd.inputText[fd.cursorPos:]
				fd.cursorPos++
				fd.filterOptions()
			}
		}

		// Handle control key combinations
		if ebiten.IsKeyPressed(ebiten.KeyControl) {
			// Select all (Ctrl+A)
			if inpututil.IsKeyJustPressed(ebiten.KeyA) {
				fd.selStart = 0
				fd.selEnd = len(fd.inputText)
				fd.cursorPos = fd.selEnd
			}

			// Copy (Ctrl+C)
			if inpututil.IsKeyJustPressed(ebiten.KeyC) && fd.hasSelection() && runtime.GOOS != "js" {
				start, end := fd.getOrderedSelection()
				if start >= 0 && end <= len(fd.inputText) && start <= end {
					selectedText := fd.inputText[start:end]
					clipboard.Write(clipboard.FmtText, []byte(selectedText))
				}
			}

			// Paste (Ctrl+V)
			if inpututil.IsKeyJustPressed(ebiten.KeyV) && runtime.GOOS != "js" {
				if clipData := clipboard.Read(clipboard.FmtText); clipData != nil {
					clipText := string(clipData)
					// If there's a selection, delete it first
					if fd.hasSelection() {
						fd.deleteSelection()
					}
					// Insert clipboard text at cursor position
					fd.inputText = fd.inputText[:fd.cursorPos] + clipText + fd.inputText[fd.cursorPos:]
					fd.cursorPos += len(clipText)
					fd.filterOptions()
				}
			}

			// Cut (Ctrl+X)
			if inpututil.IsKeyJustPressed(ebiten.KeyX) && fd.hasSelection() && runtime.GOOS != "js" {
				start, end := fd.getOrderedSelection()
				if start >= 0 && end <= len(fd.inputText) && start <= end {
					selectedText := fd.inputText[start:end]
					clipboard.Write(clipboard.FmtText, []byte(selectedText))
					fd.deleteSelection()
				}
			}

			// Word navigation (Ctrl+Left/Right)
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
				fd.moveCursorByWord(-1, ebiten.IsKeyPressed(ebiten.KeyShift))
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
				fd.moveCursorByWord(1, ebiten.IsKeyPressed(ebiten.KeyShift))
			}
		} else {
			// Handle backspace
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
				if fd.hasSelection() {
					fd.deleteSelection()
					fd.filterOptions()
				} else if fd.cursorPos > 0 {
					fd.inputText = fd.inputText[:fd.cursorPos-1] + fd.inputText[fd.cursorPos:]
					fd.cursorPos--
					fd.filterOptions()
				}
			}

			// Handle delete
			if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
				if fd.hasSelection() {
					fd.deleteSelection()
					fd.filterOptions()
				} else if fd.cursorPos < len(fd.inputText) {
					fd.inputText = fd.inputText[:fd.cursorPos] + fd.inputText[fd.cursorPos+1:]
					fd.filterOptions()
				}
			}

			// Handle arrow keys for text navigation
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
				if fd.cursorPos > 0 {
					if !ebiten.IsKeyPressed(ebiten.KeyShift) {
						fd.clearSelection()
					} else {
						// Start selection if not already selecting
						if fd.selStart == -1 {
							fd.selStart = fd.cursorPos
						}
					}
					fd.cursorPos--
					if ebiten.IsKeyPressed(ebiten.KeyShift) {
						fd.selEnd = fd.cursorPos
					}
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
				if fd.cursorPos < len(fd.inputText) {
					if !ebiten.IsKeyPressed(ebiten.KeyShift) {
						fd.clearSelection()
					} else {
						// Start selection if not already selecting
						if fd.selStart == -1 {
							fd.selStart = fd.cursorPos
						}
					}
					fd.cursorPos++
					if ebiten.IsKeyPressed(ebiten.KeyShift) {
						fd.selEnd = fd.cursorPos
					}
				}
			}

			// Handle home/end keys
			if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
				if !ebiten.IsKeyPressed(ebiten.KeyShift) {
					fd.clearSelection()
				} else {
					if fd.selStart == -1 {
						fd.selStart = fd.cursorPos
					}
				}
				fd.cursorPos = 0
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					fd.selEnd = fd.cursorPos
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
				if !ebiten.IsKeyPressed(ebiten.KeyShift) {
					fd.clearSelection()
				} else {
					if fd.selStart == -1 {
						fd.selStart = fd.cursorPos
					}
				}
				fd.cursorPos = len(fd.inputText)
				if ebiten.IsKeyPressed(ebiten.KeyShift) {
					fd.selEnd = fd.cursorPos
				}
			}
		}

		// Handle arrow keys for dropdown navigation when open
		if fd.IsOpen && len(fd.filteredOptions) > 0 {
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
				if fd.hoveredIndex < len(fd.filteredOptions)-1 {
					fd.hoveredIndex++
					// Adjust scroll if needed
					if fd.hoveredIndex >= fd.scrollOffset+fd.maxVisibleItems {
						fd.scrollOffset++
					}
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
				if fd.hoveredIndex > 0 {
					fd.hoveredIndex--
					// Adjust scroll if needed
					if fd.hoveredIndex < fd.scrollOffset {
						fd.scrollOffset--
					}
				} else if fd.hoveredIndex == -1 && len(fd.filteredOptions) > 0 {
					fd.hoveredIndex = 0
				}
			}

			// Handle enter to select
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && fd.hoveredIndex >= 0 && fd.hoveredIndex < len(fd.filteredOptions) {
				fd.selectedIndex = fd.hoveredIndex
				fd.inputText = fd.filteredOptions[fd.hoveredIndex].Display
				fd.cursorPos = len(fd.inputText)
				fd.IsOpen = false
				fd.InputFocused = false
				if fd.onSelected != nil {
					fd.onSelected(fd.filteredOptions[fd.hoveredIndex])
				}
			}
		}

		// Handle escape to close
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			fd.IsOpen = false
			fd.InputFocused = false
		}
	}

	// Handle hover over dropdown items when open
	fd.hoveredIndex = -1
	if fd.IsOpen && len(fd.filteredOptions) > 0 {
		visibleItems := len(fd.filteredOptions) - fd.scrollOffset
		if visibleItems > fd.maxVisibleItems {
			visibleItems = fd.maxVisibleItems
		}

		for i := 0; i < visibleItems; i++ {
			actualIndex := i + fd.scrollOffset
			if actualIndex >= len(fd.filteredOptions) {
				break
			}

			var itemY int
			if fd.openUpward {
				itemY = fd.Y - (visibleItems-i)*fd.Height
			} else {
				itemY = fd.Y + fd.Height + i*fd.Height
			}

			itemBounds := image.Rect(fd.X, itemY, fd.X+fd.Width, itemY+fd.Height)

			if mx >= itemBounds.Min.X && mx <= itemBounds.Max.X &&
				my >= itemBounds.Min.Y && my <= itemBounds.Max.Y {
				fd.hoveredIndex = actualIndex
				break
			}
		}
	}

	// Handle scroll wheel when open
	if fd.IsOpen && len(fd.filteredOptions) > fd.maxVisibleItems {
		_, scrollY := ebiten.Wheel()
		if scrollY > 0 && fd.scrollOffset > 0 {
			fd.scrollOffset--
		} else if scrollY < 0 && fd.scrollOffset < len(fd.filteredOptions)-fd.maxVisibleItems {
			fd.scrollOffset++
		}
	}

	// Return true if we have input focus (consuming input), false otherwise
	return fd.InputFocused || fd.IsOpen
}

// Draw renders the filterable dropdown
func (fd *FilterableDropdown) Draw(screen *ebiten.Image) {
	// Draw dropdown header (text input)
	fd.drawHeader(screen)

	// Queue dropdown list to be drawn in the global overlay pass so it sits above all UI.
	if fd.IsOpen {
		queueDropdownOverlay(fd)
	}
}

// DrawHeaderOnly renders just the input/header without any open list.
func (fd *FilterableDropdown) DrawHeaderOnly(screen *ebiten.Image) {
	fd.drawHeader(screen)
}

// SetPlaceholder overrides the hint text shown when no value is selected.
func (fd *FilterableDropdown) SetPlaceholder(text string) {
	fd.placeholder = text
}

// DrawOverlay redraws only the open menu portion so it can be layered above other UI.
func (fd *FilterableDropdown) DrawOverlay(screen *ebiten.Image) {
	if !fd.IsOpen {
		return
	}

	fd.drawItems(screen)
}

// queueDropdownOverlay records an open dropdown so its list can be drawn after all other UI.
func queueDropdownOverlay(fd *FilterableDropdown) {
	if fd == nil {
		return
	}
	dropdownOverlayQueue = append(dropdownOverlayQueue, fd)
}

// FlushDropdownOverlays draws all queued dropdown lists above the rest of the UI and clears the queue.
func FlushDropdownOverlays(screen *ebiten.Image) {
	for _, fd := range dropdownOverlayQueue {
		if fd != nil && fd.IsOpen {
			fd.drawItems(screen)
		}
	}
	dropdownOverlayQueue = dropdownOverlayQueue[:0]
}

// drawHeader draws the main dropdown header (text input field)
func (fd *FilterableDropdown) drawHeader(screen *ebiten.Image) {
	// Draw background
	ebitenutil.DrawRect(screen, float64(fd.X), float64(fd.Y), float64(fd.Width), float64(fd.Height), fd.backgroundColor)

	// Draw border (thicker if focused)
	borderColor := fd.borderColor
	if fd.InputFocused {
		borderColor = color.RGBA{150, 150, 255, 255}
	}
	ebitenutil.DrawRect(screen, float64(fd.X-1), float64(fd.Y-1), float64(fd.Width+2), float64(fd.Height+2), borderColor)
	ebitenutil.DrawRect(screen, float64(fd.X), float64(fd.Y), float64(fd.Width), float64(fd.Height), fd.backgroundColor)

	// Draw text
	font := loadWynncraftFont(16)
	textToShow := fd.inputText
	if textToShow == "" && !fd.InputFocused {
		textToShow = fd.placeholder
		text.Draw(screen, textToShow, font, fd.X+8, fd.Y+fd.Height/2+4, color.RGBA{120, 120, 120, 255})
	} else {
		// Draw selection highlight first if there's a selection
		if fd.InputFocused && fd.hasSelection() {
			start, end := fd.getOrderedSelection()
			if start >= 0 && end <= len(fd.inputText) && start < end {
				// Calculate selection bounds
				var startX, endX int
				startX = fd.X + 8
				if start > 0 {
					startText := fd.inputText[:start]
					startBounds := text.BoundString(font, startText)
					startX += startBounds.Dx()
				}

				endX = fd.X + 8
				if end > 0 {
					endText := fd.inputText[:end]
					endBounds := text.BoundString(font, endText)
					endX += endBounds.Dx()
				}

				// Draw selection background
				selectionColor := color.RGBA{100, 150, 255, 128}
				ebitenutil.DrawRect(screen, float64(startX), float64(fd.Y+3), float64(endX-startX), float64(fd.Height-6), selectionColor)
			}
		}

		text.Draw(screen, textToShow, font, fd.X+8, fd.Y+fd.Height/2+4, fd.textColor)
	}

	// Draw cursor if focused
	if fd.InputFocused && (fd.cursorBlink/30)%2 == 0 { // Blink every 30 frames
		cursorX := fd.X + 8
		if fd.cursorPos > 0 && fd.cursorPos <= len(fd.inputText) {
			cursorText := fd.inputText[:fd.cursorPos]
			bounds := text.BoundString(font, cursorText)
			cursorX += bounds.Dx()
		}
		ebitenutil.DrawRect(screen, float64(cursorX), float64(fd.Y+5), 1, float64(fd.Height-10), fd.textColor)
	}

	// Draw dropdown arrow
	arrowX := fd.X + fd.Width - 20
	arrowY := fd.Y + fd.Height/2
	arrow := "v"
	if fd.IsOpen {
		arrow = "^"
	}
	font12 := loadWynncraftFont(14)
	text.Draw(screen, arrow, font12, arrowX, arrowY+3, fd.textColor)
}

// drawItems draws the dropdown items when open
func (fd *FilterableDropdown) drawItems(screen *ebiten.Image) {
	if len(fd.filteredOptions) == 0 {
		return
	}

	visibleItems := len(fd.filteredOptions) - fd.scrollOffset
	if visibleItems > fd.maxVisibleItems {
		visibleItems = fd.maxVisibleItems
	}

	if visibleItems <= 0 {
		return
	}

	// Calculate the starting position for items
	var itemsStartY int
	if fd.openUpward {
		itemsStartY = fd.Y - visibleItems*fd.Height
	} else {
		itemsStartY = fd.Y + fd.Height
	}

	// Draw background for all items
	totalHeight := visibleItems * fd.Height
	ebitenutil.DrawRect(screen, float64(fd.X), float64(itemsStartY), float64(fd.Width), float64(totalHeight), fd.backgroundColor)

	// Draw border around items
	ebitenutil.DrawRect(screen, float64(fd.X-1), float64(itemsStartY-1), float64(fd.Width+2), float64(totalHeight+2), fd.borderColor)
	ebitenutil.DrawRect(screen, float64(fd.X), float64(itemsStartY), float64(fd.Width), float64(totalHeight), fd.backgroundColor)

	font := loadWynncraftFont(16)

	// Draw each visible item
	for i := 0; i < visibleItems; i++ {
		actualIndex := i + fd.scrollOffset
		if actualIndex >= len(fd.filteredOptions) {
			break
		}

		itemY := itemsStartY + i*fd.Height

		// Draw hover highlight
		if actualIndex == fd.hoveredIndex {
			ebitenutil.DrawRect(screen, float64(fd.X), float64(itemY), float64(fd.Width), float64(fd.Height), fd.hoverColor)
		}

		// Draw selection highlight
		if actualIndex == fd.selectedIndex {
			ebitenutil.DrawRect(screen, float64(fd.X), float64(itemY), float64(fd.Width), float64(fd.Height), fd.selectedColor)
		}

		// Draw item text
		option := fd.filteredOptions[actualIndex]
		text.Draw(screen, option.Display, font, fd.X+8, itemY+fd.Height/2+4, fd.textColor)
	}

	// Draw scroll indicator if needed
	if len(fd.filteredOptions) > fd.maxVisibleItems {
		scrollBarX := fd.X + fd.Width - 10
		scrollBarY := itemsStartY
		scrollBarHeight := totalHeight

		// Draw scroll track
		ebitenutil.DrawRect(screen, float64(scrollBarX), float64(scrollBarY), 8, float64(scrollBarHeight), color.RGBA{60, 60, 60, 255})

		// Calculate scroll thumb position and size
		thumbHeight := float64(scrollBarHeight) * float64(fd.maxVisibleItems) / float64(len(fd.filteredOptions))
		thumbY := float64(scrollBarY) + float64(scrollBarHeight)*float64(fd.scrollOffset)/float64(len(fd.filteredOptions))

		// Draw scroll thumb
		ebitenutil.DrawRect(screen, float64(scrollBarX)+1, thumbY, 6, thumbHeight, color.RGBA{120, 120, 120, 255})
	}
}

// SetOptions updates the dropdown options
func (fd *FilterableDropdown) SetOptions(options []FilterableDropdownOption) {
	fd.allOptions = options
	fd.filterOptions()
	fd.selectedIndex = -1
	fd.IsOpen = false
}

// GetSelected returns the currently selected option
func (fd *FilterableDropdown) GetSelected() (FilterableDropdownOption, bool) {
	if fd.selectedIndex >= 0 && fd.selectedIndex < len(fd.filteredOptions) {
		return fd.filteredOptions[fd.selectedIndex], true
	}
	return FilterableDropdownOption{}, false
}

// SetSelected sets the selected option by finding a matching option
func (fd *FilterableDropdown) SetSelected(value string) {
	for i, option := range fd.allOptions {
		if option.Value == value || option.Display == value {
			fd.selectedIndex = i
			fd.inputText = option.Display
			fd.cursorPos = len(fd.inputText)
			break
		}
	}
}

// GetInputText returns the current input text
func (fd *FilterableDropdown) GetInputText() string {
	return fd.inputText
}

// SetInputText sets the input text and filters options
func (fd *FilterableDropdown) SetInputText(text string) {
	fd.inputText = text
	fd.cursorPos = len(text)
	fd.filterOptions()
}

// ClearSelection clears the current selection
func (fd *FilterableDropdown) ClearSelection() {
	fd.selectedIndex = -1
	fd.inputText = ""
	fd.cursorPos = 0
	fd.filterOptions()
}

// hasSelection returns whether there is currently a text selection
func (fd *FilterableDropdown) hasSelection() bool {
	return fd.selStart != -1 && fd.selEnd != -1 && fd.selStart != fd.selEnd
}

// getOrderedSelection returns the selection bounds in order (start <= end)
func (fd *FilterableDropdown) getOrderedSelection() (int, int) {
	if fd.selStart == -1 || fd.selEnd == -1 {
		return -1, -1
	}
	if fd.selStart <= fd.selEnd {
		return fd.selStart, fd.selEnd
	}
	return fd.selEnd, fd.selStart
}

// deleteSelection deletes the currently selected text
func (fd *FilterableDropdown) deleteSelection() {
	if !fd.hasSelection() {
		return
	}

	start, end := fd.getOrderedSelection()
	fd.inputText = fd.inputText[:start] + fd.inputText[end:]
	fd.cursorPos = start
	fd.clearSelection()
}

// clearSelection clears the current text selection
func (fd *FilterableDropdown) clearSelection() {
	fd.selStart = -1
	fd.selEnd = -1
}

// moveCursorByWord moves the cursor by word boundaries
func (fd *FilterableDropdown) moveCursorByWord(direction int, selecting bool) {
	if direction < 0 {
		// Move left by word
		if fd.cursorPos <= 0 {
			return
		}

		if !selecting {
			fd.clearSelection()
		} else if fd.selStart == -1 {
			fd.selStart = fd.cursorPos
		}

		pos := fd.cursorPos - 1
		// Skip whitespace
		for pos > 0 && (fd.inputText[pos] == ' ' || fd.inputText[pos] == '\t') {
			pos--
		}
		// Skip word characters
		for pos > 0 && fd.inputText[pos] != ' ' && fd.inputText[pos] != '\t' {
			pos--
		}
		if pos > 0 {
			pos++
		}

		fd.cursorPos = pos

		if selecting {
			fd.selEnd = fd.cursorPos
		}
	} else {
		// Move right by word
		if fd.cursorPos >= len(fd.inputText) {
			return
		}

		if !selecting {
			fd.clearSelection()
		} else if fd.selStart == -1 {
			fd.selStart = fd.cursorPos
		}

		pos := fd.cursorPos
		// Skip word characters
		for pos < len(fd.inputText) && fd.inputText[pos] != ' ' && fd.inputText[pos] != '\t' {
			pos++
		}
		// Skip whitespace
		for pos < len(fd.inputText) && (fd.inputText[pos] == ' ' || fd.inputText[pos] == '\t') {
			pos++
		}

		fd.cursorPos = pos

		if selecting {
			fd.selEnd = fd.cursorPos
		}
	}
}
