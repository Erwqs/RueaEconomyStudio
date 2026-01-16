package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// TextInputOptions configures text input appearance and behavior
type TextInputOptions struct {
	Width            int
	Height           int
	MaxLength        int
	Placeholder      string
	BackgroundColor  color.RGBA
	BorderColor      color.RGBA
	FocusColor       color.RGBA
	TextColor        color.RGBA
	PlaceholderColor color.RGBA
	FontSize         int
	Multiline        bool
	ValidateInput    func(newValue string) bool // Function to validate input in real-time
}

func DefaultTextInputOptions() TextInputOptions {
	return TextInputOptions{
		Width:            200,
		Height:           25,
		MaxLength:        100,
		Placeholder:      "",
		BackgroundColor:  color.RGBA{40, 40, 50, 255},
		BorderColor:      color.RGBA{100, 100, 100, 255},
		FocusColor:       color.RGBA{100, 150, 255, 255},
		TextColor:        color.RGBA{255, 255, 255, 255},
		PlaceholderColor: color.RGBA{120, 120, 120, 255},
		FontSize:         16,
		Multiline:        false,
	}
}

// MenuTextInput represents a text input field element
type MenuTextInput struct {
	BaseMenuElement
	label          string
	value          string
	options        TextInputOptions
	callback       func(value string)
	focused        bool
	cursorPos      int
	selectionStart int // Start of text selection (-1 if no selection)
	blinkTimer     time.Time
	rect           image.Rectangle
}

func NewMenuTextInput(label string, initialValue string, options TextInputOptions, callback func(string)) *MenuTextInput {
	return &MenuTextInput{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		value:           initialValue,
		options:         options,
		callback:        callback,
		cursorPos:       len(initialValue),
		selectionStart:  -1, // No selection initially
		blinkTimer:      time.Now(),
	}
}

func (t *MenuTextInput) Update(mx, my int, deltaTime float64) bool {
	if !t.visible {
		return false
	}

	t.updateAnimation(deltaTime)

	// Handle focus
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		oldFocused := t.focused
		t.focused = mx >= t.rect.Min.X && mx < t.rect.Max.X && my >= t.rect.Min.Y && my < t.rect.Max.Y
		if t.focused != oldFocused {
			if t.focused {
				t.blinkTimer = time.Now()
			}
			return true
		}
	}

	if !t.focused {
		return false
	}

	// Handle text input
	oldValue := t.value
	inputRunes := ebiten.AppendInputChars(nil)
	for _, r := range inputRunes {
		if len(t.value) < t.options.MaxLength && r >= 32 && r != 127 { // Printable characters
			// If there's a selection, delete it first
			if t.hasSelection() {
				t.deleteSelection()
			}

			// Create the new value that would result from this input
			newValue := t.value[:t.cursorPos] + string(r) + t.value[t.cursorPos:]

			// Check if validation function allows this new value
			if t.options.ValidateInput == nil || t.options.ValidateInput(newValue) {
				t.value = newValue
				t.cursorPos++
			}
		}
	}

	// Handle special keys
	repeatingKeyPressed := inpututil.IsKeyJustPressed(ebiten.KeyBackspace) ||
		(ebiten.IsKeyPressed(ebiten.KeyBackspace) && inpututil.KeyPressDuration(ebiten.KeyBackspace) >= 30 && inpututil.KeyPressDuration(ebiten.KeyBackspace)%6 == 0)

	if repeatingKeyPressed {
		if t.hasSelection() {
			t.deleteSelection()
		} else if t.cursorPos > 0 {
			t.value = t.value[:t.cursorPos-1] + t.value[t.cursorPos:]
			t.cursorPos--
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if t.hasSelection() {
			t.deleteSelection()
		} else if t.cursorPos < len(t.value) {
			t.value = t.value[:t.cursorPos] + t.value[t.cursorPos+1:]
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		t.selectionStart = -1 // Clear selection on home/end
		t.cursorPos = 0
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		t.selectionStart = -1 // Clear selection on home/end
		t.cursorPos = len(t.value)
	}

	// Handle clipboard operations
	ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)

	if ctrlPressed {
		// Ctrl+A - Select All
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			t.selectionStart = 0
			t.cursorPos = len(t.value)
			return true
		}

		// Ctrl+C - Copy
		if inpututil.IsKeyJustPressed(ebiten.KeyC) && t.hasSelection() {
			selectedText := t.getSelectedText()
			if selectedText != "" {
				// Try to copy to clipboard (this may not work in all environments)
				// Note: Ebiten doesn't have built-in clipboard support, so we'll store it internally
				setClipboard(selectedText)
				return true
			}
		}

		// Ctrl+X - Cut
		if inpututil.IsKeyJustPressed(ebiten.KeyX) && t.hasSelection() {
			selectedText := t.getSelectedText()
			if selectedText != "" {
				setClipboard(selectedText)
				t.deleteSelection()
				return true
			}
		}

		// Ctrl+V - Paste
		if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			clipboardText := getClipboard()
			if clipboardText != "" {
				if t.hasSelection() {
					t.deleteSelection()
				}
				// Insert clipboard text at cursor position
				if len(t.value)+len(clipboardText) <= t.options.MaxLength {
					newValue := t.value[:t.cursorPos] + clipboardText + t.value[t.cursorPos:]
					if t.options.ValidateInput == nil || t.options.ValidateInput(newValue) {
						t.value = newValue
						t.cursorPos += len(clipboardText)
					}
				}
				return true
			}
		}
	}

	// Handle arrow keys with shift for selection
	shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		if shiftPressed {
			if t.selectionStart == -1 {
				t.selectionStart = t.cursorPos
			}
		} else {
			t.clearSelection()
		}
		if t.cursorPos > 0 {
			t.cursorPos--
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		if shiftPressed {
			if t.selectionStart == -1 {
				t.selectionStart = t.cursorPos
			}
		} else {
			t.clearSelection()
		}
		if t.cursorPos < len(t.value) {
			t.cursorPos++
		}
	}

	sanitizedChanged, flagged := t.applyTextSanitization()

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && t.callback != nil {
		if flagged {
			showAdvertisingToast()
		}
		t.callback(t.value)
		t.focused = false
		return true
	}

	if flagged {
		showAdvertisingToast()
	}

	valueChanged := oldValue != t.value || sanitizedChanged
	if valueChanged && t.callback != nil {
		t.callback(t.value)
	}

	return valueChanged
}

func (t *MenuTextInput) applyTextSanitization() (bool, bool) {
	sanitized, changed, flagged := sanitizeTextValue(t.value)
	if changed {
		t.value = sanitized
		if t.cursorPos > len(t.value) {
			t.cursorPos = len(t.value)
		}
	}
	return changed, flagged
}

func (t *MenuTextInput) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !t.visible || t.animProgress <= 0.01 {
		return 0
	}

	height := t.options.Height + 35 // Increased space for better spacing
	alpha := float32(t.animProgress)

	// Draw label
	labelColor := color.RGBA{255, 255, 255, uint8(float32(255) * alpha)}
	text.Draw(screen, t.label, font, x, y+18, labelColor)

	// Input field with more spacing and aligned with label
	inputY := y + 28 // Increased from 20 to 28 for more space
	inputWidth := t.options.Width
	if inputWidth > width-10 {
		inputWidth = width - 10
	}

	t.rect = image.Rect(x, inputY, x+inputWidth, inputY+t.options.Height) // Removed +10 indentation

	// Background
	bgColor := t.options.BackgroundColor
	bgColor.A = uint8(float32(bgColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(t.rect.Min.X), float32(t.rect.Min.Y), float32(t.rect.Dx()), float32(t.rect.Dy()), bgColor, false)

	// Border
	borderColor := t.options.BorderColor
	if t.focused {
		borderColor = t.options.FocusColor
	}
	borderColor.A = uint8(float32(borderColor.A) * alpha)
	vector.StrokeRect(screen, float32(t.rect.Min.X), float32(t.rect.Min.Y), float32(t.rect.Dx()), float32(t.rect.Dy()), 2, borderColor, false)

	// Text content
	displayText := t.value
	if displayText == "" && !t.focused {
		displayText = t.options.Placeholder
	}

	textColor := t.options.TextColor
	if displayText == t.options.Placeholder {
		textColor = t.options.PlaceholderColor
	}
	textColor.A = uint8(float32(textColor.A) * alpha)

	// Draw text with selection highlighting
	if displayText != "" {
		textX := t.rect.Min.X + 5
		textY := t.rect.Min.Y + t.rect.Dy()/2 + 6

		if t.hasSelection() && t.focused {
			// Draw text in three parts: before selection, selection (highlighted), after selection
			start := t.selectionStart
			end := t.cursorPos
			if start > end {
				start, end = end, start
			}

			// Ensure indices are within bounds
			if start < 0 {
				start = 0
			}
			if end > len(displayText) {
				end = len(displayText)
			}

			currentX := textX

			// Draw text before selection
			if start > 0 {
				beforeText := displayText[:start]
				text.Draw(screen, beforeText, font, currentX, textY, textColor)
				currentX += text.BoundString(font, beforeText).Dx()
			}

			// Draw selected text with highlight background
			if end > start {
				selectedText := displayText[start:end]
				selectedWidth := text.BoundString(font, selectedText).Dx()

				// Draw selection highlight background
				selectionColor := color.RGBA{100, 150, 255, 180} // Semi-transparent blue
				selectionColor.A = uint8(float32(selectionColor.A) * alpha)
				vector.DrawFilledRect(screen, float32(currentX), float32(t.rect.Min.Y+2), float32(selectedWidth), float32(t.rect.Dy()-4), selectionColor, false)

				// Draw selected text (inverted colors)
				selectedTextColor := color.RGBA{255, 255, 255, 255}
				selectedTextColor.A = uint8(float32(selectedTextColor.A) * alpha)
				text.Draw(screen, selectedText, font, currentX, textY, selectedTextColor)
				currentX += selectedWidth
			}

			// Draw text after selection
			if end < len(displayText) {
				afterText := displayText[end:]
				text.Draw(screen, afterText, font, currentX, textY, textColor)
			}
		} else {
			// No selection, draw normally
			text.Draw(screen, displayText, font, textX, textY, textColor)
		}
	}

	// Draw cursor if focused (but not when there's a selection)
	if t.focused && !t.hasSelection() && time.Since(t.blinkTimer).Milliseconds()%1000 < 500 {
		cursorX := t.rect.Min.X + 5
		if t.cursorPos > 0 && t.cursorPos <= len(t.value) {
			textWidth := text.BoundString(font, t.value[:t.cursorPos]).Dx()
			cursorX += textWidth
		}
		cursorColor := t.options.TextColor
		cursorColor.A = uint8(float32(cursorColor.A) * alpha)
		vector.DrawFilledRect(screen, float32(cursorX), float32(t.rect.Min.Y+3), 1, float32(t.rect.Dy()-6), cursorColor, false)
	}

	return height
}

func (t *MenuTextInput) GetMinHeight() int {
	return t.options.Height + 35 // Updated to match the new height calculation
}

func (t *MenuTextInput) SetValue(value string) {
	t.value = value
	t.cursorPos = len(value)
}

func (t *MenuTextInput) GetValue() string {
	return t.value
}

// ToggleSwitchOptions configures toggle switch appearance and behavior
type ToggleSwitchOptions struct {
	Width           int
	Height          int
	BackgroundColor color.RGBA
	ActiveColor     color.RGBA
	HandleColor     color.RGBA
	TextColor       color.RGBA
	FontSize        int
	Options         []string // e.g., ["Open", "Close"], ["On", "Off"]
}

func DefaultToggleSwitchOptions() ToggleSwitchOptions {
	return ToggleSwitchOptions{
		Width:           140,
		Height:          25,
		BackgroundColor: color.RGBA{60, 60, 70, 255},
		ActiveColor:     color.RGBA{100, 150, 255, 255},
		HandleColor:     color.RGBA{255, 255, 255, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		FontSize:        14,
		Options:         []string{"Off", "On"},
	}
}

// MenuToggleSwitch represents a toggle switch element
type MenuToggleSwitch struct {
	BaseMenuElement
	label         string
	selectedIndex int
	options       ToggleSwitchOptions
	callback      func(index int, value string)
	animPosition  float64
	rect          image.Rectangle
}

func NewMenuToggleSwitch(label string, initialIndex int, options ToggleSwitchOptions, callback func(int, string)) *MenuToggleSwitch {
	if initialIndex < 0 || initialIndex >= len(options.Options) {
		initialIndex = 0
	}

	return &MenuToggleSwitch{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		selectedIndex:   initialIndex,
		options:         options,
		callback:        callback,
		animPosition:    float64(initialIndex),
	}
}

func (s *MenuToggleSwitch) Update(mx, my int, deltaTime float64) bool {
	if !s.visible {
		return false
	}

	s.updateAnimation(deltaTime)

	// Animate position
	target := float64(s.selectedIndex)
	if math.Abs(s.animPosition-target) > 0.01 {
		diff := target - s.animPosition
		s.animPosition += diff * 8.0 * deltaTime
	}

	// Handle click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= s.rect.Min.X && mx < s.rect.Max.X && my >= s.rect.Min.Y && my < s.rect.Max.Y {
			// Calculate which option was clicked
			optionWidth := s.rect.Dx() / len(s.options.Options)
			clickedIndex := (mx - s.rect.Min.X) / optionWidth

			// Ensure the index is within bounds
			if clickedIndex >= 0 && clickedIndex < len(s.options.Options) {
				// Only change if user clicked a different option
				if clickedIndex != s.selectedIndex {
					s.selectedIndex = clickedIndex
					if s.callback != nil {
						s.callback(s.selectedIndex, s.options.Options[s.selectedIndex])
					}
				}
				return true
			}
		}
	}

	return false
}

func (s *MenuToggleSwitch) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !s.visible || s.animProgress <= 0.01 {
		return 0
	}

	height := s.options.Height + 35 // Increased space for better spacing
	alpha := float32(s.animProgress)

	// Draw label
	labelColor := s.options.TextColor
	labelColor.A = uint8(float32(labelColor.A) * alpha)
	text.Draw(screen, s.label, font, x, y+18, labelColor)

	// Toggle switch with more spacing and aligned with label
	switchY := y + 28 // Increased from 20 to 28 for more space
	switchWidth := s.options.Width
	if switchWidth > width-10 {
		switchWidth = width - 10
	}

	s.rect = image.Rect(x, switchY, x+switchWidth, switchY+s.options.Height) // Removed +10 indentation

	// Background
	bgColor := s.options.BackgroundColor
	bgColor.A = uint8(float32(bgColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(s.rect.Min.X), float32(s.rect.Min.Y), float32(s.rect.Dx()), float32(s.rect.Dy()), bgColor, false)

	// Handle
	handleWidth := s.rect.Dx() / len(s.options.Options)
	handleX := s.rect.Min.X + int(s.animPosition*float64(handleWidth))

	handleColor := s.options.ActiveColor
	handleColor.A = uint8(float32(handleColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(handleX), float32(s.rect.Min.Y), float32(handleWidth), float32(s.rect.Dy()), handleColor, false)

	// Draw option labels
	for i, option := range s.options.Options {
		optionX := s.rect.Min.X + i*handleWidth
		textBounds := text.BoundString(font, option)
		textX := optionX + (handleWidth-textBounds.Dx())/2
		textY := s.rect.Min.Y + s.rect.Dy()/2 + 5

		// Always use white text for better visibility
		optionColor := color.RGBA{255, 255, 255, 255} // White text for all options
		optionColor.A = uint8(float32(optionColor.A) * alpha)
		text.Draw(screen, option, font, textX, textY, optionColor)
	}

	return height
}

func (s *MenuToggleSwitch) GetMinHeight() int {
	return s.options.Height + 35 // Updated to match the new height calculation
}

func (s *MenuToggleSwitch) SetSelectedIndex(index int) {
	if index >= 0 && index < len(s.options.Options) {
		s.selectedIndex = index
	}
}

// SetSelectedValue sets the selected option by value string
func (s *MenuToggleSwitch) SetSelectedValue(value string) {
	for i, option := range s.options.Options {
		if option == value {
			s.selectedIndex = i
			return
		}
	}
}

// SetDefault sets the default selected option by index
func (s *MenuToggleSwitch) SetDefault(index int) {
	s.SetSelectedIndex(index)
}

// SetDefaultValue sets the default selected option by value string
func (s *MenuToggleSwitch) SetDefaultValue(value string) {
	s.SetSelectedValue(value)
}

func (s *MenuToggleSwitch) GetSelectedIndex() int {
	return s.selectedIndex
}

// SetIndex sets the toggle switch to the specified index with animation and callback
func (s *MenuToggleSwitch) SetIndex(index int) {
	if index >= 0 && index < len(s.options.Options) && index != s.selectedIndex {
		s.selectedIndex = index
		// Don't change animPosition here - let the animation naturally move to the new position
		if s.callback != nil {
			s.callback(s.selectedIndex, s.options.Options[s.selectedIndex])
		}
	}
}

// SetIndexImmediate sets the toggle switch to the specified index without animation
func (s *MenuToggleSwitch) SetIndexImmediate(index int) {
	if index >= 0 && index < len(s.options.Options) {
		s.selectedIndex = index
		s.animPosition = float64(index) // Set animation position immediately
		if s.callback != nil {
			s.callback(s.selectedIndex, s.options.Options[s.selectedIndex])
		}
	}
}

// SpacerOptions configures spacer appearance
type SpacerOptions struct {
	Height int
	Color  color.RGBA
}

func DefaultSpacerOptions() SpacerOptions {
	return SpacerOptions{
		Height: 20,
		Color:  color.RGBA{0, 0, 0, 0}, // Transparent
	}
}

// MenuSpacer represents a spacing element
type MenuSpacer struct {
	BaseMenuElement
	options SpacerOptions
}

func NewMenuSpacer(options SpacerOptions) *MenuSpacer {
	return &MenuSpacer{
		BaseMenuElement: NewBaseMenuElement(),
		options:         options,
	}
}

func (s *MenuSpacer) Update(mx, my int, deltaTime float64) bool {
	s.updateAnimation(deltaTime)
	return false
}

func (s *MenuSpacer) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !s.visible || s.animProgress <= 0.01 {
		return 0
	}

	height := int(float64(s.options.Height) * s.animProgress)

	// Draw colored background if specified
	if s.options.Color.A > 0 {
		bgColor := s.options.Color
		bgColor.A = uint8(float32(bgColor.A) * float32(s.animProgress))
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), bgColor, false)
	}

	return height
}

func (s *MenuSpacer) GetMinHeight() int {
	return s.options.Height
}

// ProgressBarOptions configures progress bar appearance
type ProgressBarOptions struct {
	Width           int
	Height          int
	BackgroundColor color.RGBA
	FillColor       color.RGBA
	BorderColor     color.RGBA
	TextColor       color.RGBA
	ShowPercentage  bool
	ShowValue       bool
	ValueFormat     string
}

func DefaultProgressBarOptions() ProgressBarOptions {
	return ProgressBarOptions{
		Width:           200,
		Height:          20,
		BackgroundColor: color.RGBA{40, 40, 50, 255},
		FillColor:       color.RGBA{100, 200, 100, 255},
		BorderColor:     color.RGBA{100, 100, 100, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		ShowPercentage:  true,
		ShowValue:       false,
		ValueFormat:     "%.1f",
	}
}

// MenuProgressBar represents a progress bar element
type MenuProgressBar struct {
	BaseMenuElement
	label    string
	value    float64
	minValue float64
	maxValue float64
	options  ProgressBarOptions
}

func NewMenuProgressBar(label string, value, minValue, maxValue float64, options ProgressBarOptions) *MenuProgressBar {
	return &MenuProgressBar{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		value:           value,
		minValue:        minValue,
		maxValue:        maxValue,
		options:         options,
	}
}

func (p *MenuProgressBar) Update(mx, my int, deltaTime float64) bool {
	p.updateAnimation(deltaTime)
	return false
}

func (p *MenuProgressBar) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !p.visible || p.animProgress <= 0.01 {
		return 0
	}

	height := p.options.Height + 25
	alpha := float32(p.animProgress)

	// Draw label
	labelColor := p.options.TextColor
	labelColor.A = uint8(float32(labelColor.A) * alpha)
	text.Draw(screen, p.label, font, x, y+18, labelColor)

	// Progress bar
	barY := y + 20
	barWidth := p.options.Width
	if barWidth > width-20 {
		barWidth = width - 20
	}

	barRect := image.Rect(x+10, barY, x+10+barWidth, barY+p.options.Height)

	// Background
	bgColor := p.options.BackgroundColor
	bgColor.A = uint8(float32(bgColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(barRect.Min.X), float32(barRect.Min.Y), float32(barRect.Dx()), float32(barRect.Dy()), bgColor, false)

	// Fill
	progress := (p.value - p.minValue) / (p.maxValue - p.minValue)
	progress = math.Max(0, math.Min(1, progress))
	fillWidth := float32(barRect.Dx()) * float32(progress)

	fillColor := p.options.FillColor
	fillColor.A = uint8(float32(fillColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(barRect.Min.X), float32(barRect.Min.Y), fillWidth, float32(barRect.Dy()), fillColor, false)

	// Border
	borderColor := p.options.BorderColor
	borderColor.A = uint8(float32(borderColor.A) * alpha)
	vector.StrokeRect(screen, float32(barRect.Min.X), float32(barRect.Min.Y), float32(barRect.Dx()), float32(barRect.Dy()), 1, borderColor, false)

	// Text overlay
	var overlayText string
	if p.options.ShowPercentage {
		overlayText = fmt.Sprintf("%.0f%%", progress*100)
	} else if p.options.ShowValue {
		overlayText = fmt.Sprintf(p.options.ValueFormat, p.value)
	}

	if overlayText != "" {
		textBounds := text.BoundString(font, overlayText)
		textX := barRect.Min.X + (barRect.Dx()-textBounds.Dx())/2
		textY := barRect.Min.Y + barRect.Dy()/2 + 5
		text.Draw(screen, overlayText, font, textX, textY, labelColor)
	}

	return height
}

func (p *MenuProgressBar) GetMinHeight() int {
	return p.options.Height + 25
}

func (p *MenuProgressBar) SetValue(value float64) {
	p.value = math.Max(p.minValue, math.Min(p.maxValue, value))
}

func (p *MenuProgressBar) GetValue() float64 {
	return p.value
}

// Extension methods for EdgeMenu to support new components

// TextInput adds a text input to the menu
func (m *EdgeMenu) TextInput(label string, initialValue string, options TextInputOptions, callback func(string)) *EdgeMenu {
	textInput := NewMenuTextInput(label, initialValue, options, callback)
	m.elements = append(m.elements, textInput)
	return m
}

// ToggleSwitch adds a toggle switch to the menu and returns the toggle switch for further configuration
func (m *EdgeMenu) ToggleSwitch(label string, initialIndex int, options ToggleSwitchOptions, callback func(int, string)) *MenuToggleSwitch {
	toggle := NewMenuToggleSwitch(label, initialIndex, options, callback)
	m.elements = append(m.elements, toggle)
	return toggle
}

// Spacer adds a spacer to the menu
func (m *EdgeMenu) Spacer(options SpacerOptions) *EdgeMenu {
	spacer := NewMenuSpacer(options)
	m.elements = append(m.elements, spacer)
	return m
}

// ProgressBar adds a progress bar to the menu
func (m *EdgeMenu) ProgressBar(label string, value, minValue, maxValue float64, options ProgressBarOptions) *EdgeMenu {
	progressBar := NewMenuProgressBar(label, value, minValue, maxValue, options)
	m.elements = append(m.elements, progressBar)
	return m
}

// SetFocusOnLastTextInput focuses the most recently added text input
func (m *EdgeMenu) SetFocusOnLastTextInput() *EdgeMenu {
	// Find the last text input element and focus it
	for i := len(m.elements) - 1; i >= 0; i-- {
		if textInput, ok := m.elements[i].(*MenuTextInput); ok {
			textInput.focused = true
			textInput.blinkTimer = time.Now()
			break
		}
	}
	return m
}

// Extension methods for CollapsibleMenu to support new components

// TextInput adds a text input to the collapsible menu
func (c *CollapsibleMenu) TextInput(label string, initialValue string, options TextInputOptions, callback func(string)) *CollapsibleMenu {
	textInput := NewMenuTextInput(label, initialValue, options, callback)
	c.elements = append(c.elements, textInput)
	return c
}

// ToggleSwitch adds a toggle switch to the collapsible menu
func (c *CollapsibleMenu) ToggleSwitch(label string, initialIndex int, options ToggleSwitchOptions, callback func(int, string)) *MenuToggleSwitch {
	toggle := NewMenuToggleSwitch(label, initialIndex, options, callback)
	c.elements = append(c.elements, toggle)
	return toggle
}

// Spacer adds a spacer to the collapsible menu
func (c *CollapsibleMenu) Spacer(options SpacerOptions) *CollapsibleMenu {
	spacer := NewMenuSpacer(options)
	c.elements = append(c.elements, spacer)
	return c
}

// ProgressBar adds a progress bar to the collapsible menu
func (c *CollapsibleMenu) ProgressBar(label string, value, minValue, maxValue float64, options ProgressBarOptions) *CollapsibleMenu {
	progressBar := NewMenuProgressBar(label, value, minValue, maxValue, options)
	c.elements = append(c.elements, progressBar)
	return c
}

// Simple clipboard implementation (global variable for simplicity)
var internalClipboard string

func setClipboard(text string) {
	internalClipboard = text
}

func getClipboard() string {
	return internalClipboard
}

// hasSelection returns true if there is text selected
func (t *MenuTextInput) hasSelection() bool {
	return t.selectionStart != -1 && t.selectionStart != t.cursorPos
}

// getSelectedText returns the currently selected text
func (t *MenuTextInput) getSelectedText() string {
	if !t.hasSelection() {
		return ""
	}
	start := t.selectionStart
	end := t.cursorPos
	if start > end {
		start, end = end, start
	}
	return t.value[start:end]
}

// deleteSelection deletes the selected text and clears the selection
func (t *MenuTextInput) deleteSelection() {
	if !t.hasSelection() {
		return
	}
	start := t.selectionStart
	end := t.cursorPos
	if start > end {
		start, end = end, start
	}
	t.value = t.value[:start] + t.value[end:]
	t.cursorPos = start
	t.selectionStart = -1
}

// clearSelection clears the current selection
func (t *MenuTextInput) clearSelection() {
	t.selectionStart = -1
}

// SetFocused sets the focused state for MenuTextInput
func (t *MenuTextInput) SetFocused(focused bool) {
	t.focused = focused
	if focused {
		t.blinkTimer = time.Now()
	}
}
