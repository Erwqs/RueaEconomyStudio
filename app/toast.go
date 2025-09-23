package app

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// ToastOption represents styling and behavior options for toast text segments
type ToastOption struct {
	Colour    color.RGBA
	Underline bool
	OnClick   func()
}

// ToastTextSegment represents a text segment in a toast
type ToastTextSegment struct {
	Text    string
	Options ToastOption
	Bounds  struct {
		X, Y, Width, Height int
	}
}

// ToastButton represents a clickable button in a toast
type ToastButton struct {
	Text    string
	OnClick func()
	X, Y    int
	Width   int
	Height  int
	Options ToastOption
}

// Toast represents a single toast notification
type Toast struct {
	ID           string
	TextSegments []ToastTextSegment
	Buttons      []ToastButton
	AutoCloseAt  *time.Time
	CreatedAt    time.Time
	X, Y         int
	Width        int
	Height       int
	Background   color.RGBA
	Border       color.RGBA
	visible      bool
}

// ToastBuilder provides a fluent interface for building toasts
type ToastBuilder struct {
	toast *Toast
}

// ToastManager manages all active toasts
type ToastManager struct {
	toasts      []*Toast
	nextID      int
	maxToasts   int
	screenW     int
	screenH     int
	toastWidth  int
	toastHeight int
	margin      int
}

var globalToastManager *ToastManager

// NewToast creates a new toast builder with the given text and options
func NewToast() *ToastBuilder {
	toast := &Toast{
		ID:           generateToastID(),
		TextSegments: []ToastTextSegment{},
		Buttons:      []ToastButton{},
		CreatedAt:    time.Now(),
		Background:   color.RGBA{40, 40, 50, 240},
		Border:       color.RGBA{70, 130, 255, 255},
		visible:      true,
	}

	return &ToastBuilder{toast: toast}
}

// Text adds a text segment to the toast
func (tb *ToastBuilder) Text(text string, options ToastOption) *ToastBuilder {
	segment := ToastTextSegment{
		Text:    text,
		Options: options,
	}
	tb.toast.TextSegments = append(tb.toast.TextSegments, segment)
	return tb
}

// Button adds a button to the toast
func (tb *ToastBuilder) Button(text string, onClick func(), x, y int, options ToastOption) *ToastBuilder {
	button := ToastButton{
		Text:    text,
		OnClick: onClick,
		X:       x,
		Y:       y,
		Width:   80, // Default width
		Height:  25, // Default height
		Options: options,
	}
	tb.toast.Buttons = append(tb.toast.Buttons, button)
	return tb
}

// AutoClose sets the toast to automatically close after the specified duration
func (tb *ToastBuilder) AutoClose(duration time.Duration) *ToastBuilder {
	closeTime := time.Now().Add(duration)
	tb.toast.AutoCloseAt = &closeTime
	return tb
}

// Background sets the background color of the toast
func (tb *ToastBuilder) Background(bg color.RGBA) *ToastBuilder {
	tb.toast.Background = bg
	return tb
}

// Border sets the border color of the toast
func (tb *ToastBuilder) Border(border color.RGBA) *ToastBuilder {
	tb.toast.Border = border
	return tb
}

// Show displays the toast by adding it to the global toast manager
func (tb *ToastBuilder) Show() {
	if globalToastManager == nil {
		InitToastManager()
	}

	// Add close button by default
	tb.addCloseButton()

	// Calculate toast dimensions and layout
	tb.calculateLayout()
	globalToastManager.AddToast(tb.toast)
}

// addCloseButton adds a close button to the toast
func (tb *ToastBuilder) addCloseButton() {
	closeButton := ToastButton{
		Text: "x",
		OnClick: func() {
			// Remove this toast when close button is clicked
			GetToastManager().RemoveToast(tb.toast.ID)
		},
		X:      0, // Will be positioned in calculateLayout
		Y:      0,
		Width:  20,
		Height: 20,
		Options: ToastOption{
			Colour: color.RGBA{180, 60, 60, 255}, // Red close button
		},
	}
	tb.toast.Buttons = append(tb.toast.Buttons, closeButton)
}

// calculateLayout calculates the layout of text segments and buttons
func (tb *ToastBuilder) calculateLayout() {
	font := loadWynncraftFont(16)
	if font == nil {
		return
	}

	padding := 15
	lineHeight := 20
	buttonSpacing := 10
	closeButtonSize := 20

	// Get screen width for text wrapping (1/3 of screen width)
	screenW, _ := WebSafeWindowSize()
	maxTextWidth := screenW / 3
	if maxTextWidth < 250 {
		maxTextWidth = 250 // Minimum width
	}
	if maxTextWidth > 500 {
		maxTextWidth = 500 // Maximum width to prevent overly wide toasts
	}

	// Calculate text layout with word wrapping
	currentY := padding + lineHeight
	toastWidth := 0

	for i := range tb.toast.TextSegments {
		segment := &tb.toast.TextSegments[i]

		// Handle multi-line text with word wrapping
		lines := tb.wrapText(segment.Text, font, maxTextWidth-padding*2-closeButtonSize-10)

		segment.Bounds.X = padding
		segment.Bounds.Y = currentY
		segment.Bounds.Width = 0
		segment.Bounds.Height = len(lines) * lineHeight

		// Calculate the width of the longest line
		for _, line := range lines {
			bounds := text.BoundString(font, line)
			if bounds.Dx() > segment.Bounds.Width {
				segment.Bounds.Width = bounds.Dx()
			}
		}

		// Store the wrapped lines in the segment
		segment.Text = strings.Join(lines, "\n")

		// Update width requirement
		segmentWidth := segment.Bounds.Width + padding*2 + closeButtonSize + 10
		if segmentWidth > toastWidth {
			toastWidth = segmentWidth
		}

		// Move Y position down for next segment
		currentY += segment.Bounds.Height + 5 // Small spacing between segments
	}

	// Ensure minimum width
	if toastWidth < 250 {
		toastWidth = 250
	}

	// Position regular buttons below text (excluding close button)
	regularButtons := tb.toast.Buttons[:len(tb.toast.Buttons)-1] // All except the last (close) button
	if len(regularButtons) > 0 {
		currentY += buttonSpacing
		buttonX := padding

		for i := range regularButtons {
			button := &regularButtons[i]
			if button.X == 0 && button.Y == 0 { // Use default positioning
				button.X = buttonX
				button.Y = currentY
				buttonX += button.Width + buttonSpacing
			}
		}
		currentY += 25 + buttonSpacing // Button height + spacing
	}

	// Set toast dimensions
	tb.toast.Width = toastWidth
	tb.toast.Height = currentY + padding

	// Position close button in top-right corner
	if len(tb.toast.Buttons) > 0 {
		closeButton := &tb.toast.Buttons[len(tb.toast.Buttons)-1] // Last button is close button
		closeButton.X = tb.toast.Width - closeButtonSize - 5
		closeButton.Y = 5
	}
}

// generateToastID generates a unique ID for a toast
func generateToastID() string {
	if globalToastManager == nil {
		return "toast_0"
	}
	id := globalToastManager.nextID
	globalToastManager.nextID++
	return fmt.Sprintf("toast_%d", id)
}

// InitToastManager initializes the global toast manager
func InitToastManager() {
	if globalToastManager != nil {
		return
	}

	screenW, screenH := WebSafeWindowSize()
	globalToastManager = &ToastManager{
		toasts:      []*Toast{},
		nextID:      0,
		maxToasts:   5,
		screenW:     screenW,
		screenH:     screenH,
		toastWidth:  300,
		toastHeight: 80,
		margin:      15,
	}
}

// GetToastManager returns the global toast manager
func GetToastManager() *ToastManager {
	if globalToastManager == nil {
		InitToastManager()
	}
	return globalToastManager
}

// AddToast adds a toast to the manager
func (tm *ToastManager) AddToast(toast *Toast) {
	// Remove oldest toast if we're at the limit
	if len(tm.toasts) >= tm.maxToasts {
		tm.toasts = tm.toasts[1:]
	}

	// Position the toast
	tm.positionToast(toast)
	tm.toasts = append(tm.toasts, toast)
}

// positionToast calculates the position for a new toast
func (tm *ToastManager) positionToast(toast *Toast) {
	screenW, screenH := WebSafeWindowSize()
	tm.screenW = screenW
	tm.screenH = screenH

	// Position toasts in the top-right corner
	x := screenW - toast.Width - tm.margin
	y := tm.margin

	// Stack toasts vertically
	for _, existingToast := range tm.toasts {
		if existingToast.visible {
			y += existingToast.Height + tm.margin
		}
	}

	toast.X = x
	toast.Y = y
}

// Update updates all toasts and handles input
func (tm *ToastManager) Update() {
	now := time.Now()
	mx, my := ebiten.CursorPosition()

	// Remove expired toasts
	var activeToasts []*Toast
	for _, toast := range tm.toasts {
		if toast.AutoCloseAt != nil && now.After(*toast.AutoCloseAt) {
			continue // Skip expired toast
		}
		activeToasts = append(activeToasts, toast)
	}
	tm.toasts = activeToasts

	// Reposition toasts if needed
	tm.repositionToasts()

	// Handle input for each toast
	for _, toast := range tm.toasts {
		if !toast.visible {
			continue
		}

		// Check button clicks
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			for _, button := range toast.Buttons {
				if mx >= toast.X+button.X && mx <= toast.X+button.X+button.Width &&
					my >= toast.Y+button.Y && my <= toast.Y+button.Y+button.Height {
					if button.OnClick != nil {
						button.OnClick()
					}
					return
				}
			}

			// Check text segment clicks
			for _, segment := range toast.TextSegments {
				if segment.Options.OnClick != nil {
					// For multi-line segments, check if click is within the entire segment bounds
					if mx >= toast.X+segment.Bounds.X && mx <= toast.X+segment.Bounds.X+segment.Bounds.Width &&
						my >= toast.Y+segment.Bounds.Y-segment.Bounds.Height && my <= toast.Y+segment.Bounds.Y+segment.Bounds.Height {
						segment.Options.OnClick()
						return
					}
				}
			}
		}
	}
}

// repositionToasts repositions all toasts after one is removed
func (tm *ToastManager) repositionToasts() {
	screenW, screenH := WebSafeWindowSize()
	tm.screenW = screenW
	tm.screenH = screenH

	y := tm.margin
	for _, toast := range tm.toasts {
		if !toast.visible {
			continue
		}

		toast.X = screenW - toast.Width - tm.margin
		toast.Y = y
		y += toast.Height + tm.margin
	}
}

// Draw renders all active toasts
func (tm *ToastManager) Draw(screen *ebiten.Image) {
	font := loadWynncraftFont(16)
	if font == nil {
		return
	}

	for _, toast := range tm.toasts {
		if !toast.visible {
			continue
		}

		tm.drawToast(screen, toast, font)
	}
}

// drawToast renders a single toast
func (tm *ToastManager) drawToast(screen *ebiten.Image, toast *Toast, font font.Face) {
	// Draw background
	vector.DrawFilledRect(screen,
		float32(toast.X), float32(toast.Y),
		float32(toast.Width), float32(toast.Height),
		toast.Background, false)

	// Draw border
	vector.StrokeRect(screen,
		float32(toast.X), float32(toast.Y),
		float32(toast.Width), float32(toast.Height),
		2, toast.Border, false)

	// Draw text segments
	for _, segment := range toast.TextSegments {
		textColor := segment.Options.Colour
		if textColor.A == 0 { // Default color if not specified
			textColor = color.RGBA{255, 255, 255, 255}
		}

		// Handle multi-line text
		lines := strings.Split(segment.Text, "\n")
		lineHeight := 20

		for i, line := range lines {
			lineY := toast.Y + segment.Bounds.Y + (i * lineHeight)
			text.Draw(screen, line, font,
				toast.X+segment.Bounds.X,
				lineY,
				textColor)

			// Draw underline if specified
			if segment.Options.Underline {
				lineBounds := text.BoundString(font, line)
				underlineY := lineY + 2
				vector.DrawFilledRect(screen,
					float32(toast.X+segment.Bounds.X), float32(underlineY),
					float32(lineBounds.Dx()), 1,
					textColor, false)
			}
		}
	}

	// Draw buttons
	mx, my := ebiten.CursorPosition()
	for _, button := range toast.Buttons {
		buttonX := toast.X + button.X
		buttonY := toast.Y + button.Y

		// Button background color
		buttonColor := button.Options.Colour
		if buttonColor.A == 0 { // Default button color
			buttonColor = color.RGBA{70, 130, 255, 255}
		}

		// Hover effect
		isHovered := mx >= buttonX && mx <= buttonX+button.Width &&
			my >= buttonY && my <= buttonY+button.Height
		if isHovered {
			buttonColor.R = min(255, buttonColor.R+30)
			buttonColor.G = min(255, buttonColor.G+30)
			buttonColor.B = min(255, buttonColor.B+30)
		}

		// Draw button background
		vector.DrawFilledRect(screen,
			float32(buttonX), float32(buttonY),
			float32(button.Width), float32(button.Height),
			buttonColor, false)

		// Draw button border
		vector.StrokeRect(screen,
			float32(buttonX), float32(buttonY),
			float32(button.Width), float32(button.Height),
			1, color.RGBA{255, 255, 255, 100}, false)

		// Draw button text
		textBounds := text.BoundString(font, button.Text)
		textX := buttonX + (button.Width-textBounds.Dx())/2
		textY := buttonY + (button.Height-textBounds.Dy())/2 + textBounds.Dy()

		text.Draw(screen, button.Text, font, textX, textY, color.RGBA{255, 255, 255, 255})

		// Draw underline if specified
		if button.Options.Underline {
			underlineY := textY + 2
			vector.DrawFilledRect(screen,
				float32(textX), float32(underlineY),
				float32(textBounds.Dx()), 1,
				color.RGBA{255, 255, 255, 255}, false)
		}
	}
}

// RemoveToast removes a toast by ID
func (tm *ToastManager) RemoveToast(id string) {
	for i, toast := range tm.toasts {
		if toast.ID == id {
			tm.toasts = append(tm.toasts[:i], tm.toasts[i+1:]...)
			tm.repositionToasts()
			return
		}
	}
}

// Clear removes all toasts
func (tm *ToastManager) Clear() {
	tm.toasts = []*Toast{}
}

// min helper function for color calculation
func min(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

// wrapText wraps text to fit within maxWidth, respecting word boundaries and \n characters
func (tb *ToastBuilder) wrapText(textStr string, font font.Face, maxWidth int) []string {
	if font == nil {
		return []string{textStr}
	}

	var lines []string

	// First split by explicit newlines
	paragraphs := strings.Split(textStr, "\n")

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		// Check if the entire paragraph fits
		bounds := text.BoundString(font, paragraph)
		if bounds.Dx() <= maxWidth {
			lines = append(lines, paragraph)
			continue
		}

		// Word wrap the paragraph
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		currentLine := words[0]

		for i := 1; i < len(words); i++ {
			testLine := currentLine + " " + words[i]
			testBounds := text.BoundString(font, testLine)

			if testBounds.Dx() <= maxWidth {
				currentLine = testLine
			} else {
				// Current line is full, start a new one
				lines = append(lines, currentLine)
				currentLine = words[i]
			}
		}

		// Add the last line
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}
