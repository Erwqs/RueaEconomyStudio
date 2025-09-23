package app

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// Enhanced UI Colors matching the territories side menu style
var EnhancedUIColors = struct {
	Background      color.RGBA
	ModalBackground color.RGBA
	Surface         color.RGBA
	Primary         color.RGBA
	Text            color.RGBA
	TextSecondary   color.RGBA
	TextPlaceholder color.RGBA
	Border          color.RGBA
	BorderActive    color.RGBA
	BorderGreen     color.RGBA // Green border for add button
	Button          color.RGBA
	ButtonGreen     color.RGBA // Green button for Done action
	ButtonRed       color.RGBA // Red button for Cancel action
	ButtonHover     color.RGBA
	ButtonActive    color.RGBA
	ItemBackground  color.RGBA
	ItemHover       color.RGBA
	ItemSelected    color.RGBA
	RemoveButton    color.RGBA
	RemoveHover     color.RGBA
	Selection       color.RGBA
}{
	Background:      color.RGBA{0, 0, 0, 150},       // Dark overlay
	ModalBackground: color.RGBA{30, 30, 45, 255},    // Same as territories menu
	Surface:         color.RGBA{40, 40, 50, 255},    // Slightly lighter for inputs
	Primary:         color.RGBA{100, 100, 255, 180}, // Blue accent
	Text:            color.RGBA{255, 255, 255, 255}, // White text
	TextSecondary:   color.RGBA{200, 200, 200, 255}, // Light gray
	TextPlaceholder: color.RGBA{120, 120, 120, 255}, // Darker gray for placeholders
	Border:          color.RGBA{80, 80, 90, 255},    // Default border
	BorderActive:    color.RGBA{100, 150, 255, 255}, // Blue when active
	BorderGreen:     color.RGBA{60, 180, 60, 255},   // Green border for add button
	Button:          color.RGBA{60, 120, 60, 255},   // Green button
	ButtonGreen:     color.RGBA{60, 150, 60, 255},   // Green button for Done action
	ButtonRed:       color.RGBA{180, 60, 60, 255},   // Red button for Cancel action
	ButtonHover:     color.RGBA{80, 160, 80, 255},   // Lighter green on hover
	ButtonActive:    color.RGBA{50, 100, 50, 255},   // Darker green when pressed
	ItemBackground:  color.RGBA{40, 40, 40, 255},    // Guild item background
	ItemHover:       color.RGBA{60, 60, 60, 255},    // Hovered item
	ItemSelected:    color.RGBA{80, 80, 120, 255},   // Selected item
	RemoveButton:    color.RGBA{200, 100, 100, 255}, // Red remove button
	RemoveHover:     color.RGBA{255, 120, 120, 255}, // Lighter red on hover
	Selection:       color.RGBA{65, 105, 225, 128},  // Semi-transparent Royal Blue
}

// EnhancedModal represents a modal with dark overlay background
type EnhancedModal struct {
	Title         string
	X, Y          int
	Width, Height int
	visible       bool
	font          font.Face
	animPhase     float64
	animSpeed     float64
	animating     bool
}

// NewEnhancedModal creates a new enhanced modal
func NewEnhancedModal(title string, width, height int) *EnhancedModal {
	// Center the modal on screen
	screenW, screenH := WebSafeWindowSize()
	x := (screenW - width) / 2
	y := (screenH - height) / 2

	return &EnhancedModal{
		Title:     title,
		X:         x,
		Y:         y,
		Width:     width,
		Height:    height,
		visible:   false,
		font:      loadWynncraftFont(18),
		animPhase: 0,
		animSpeed: 8.0,
		animating: false,
	}
}

// Show displays the modal with animation
func (m *EnhancedModal) Show() {
	m.visible = true
	m.animating = true
	m.animPhase = 0

	// Recenter modal when showing (in case window was resized)
	screenW, screenH := WebSafeWindowSize()
	m.X = (screenW - m.Width) / 2
	m.Y = (screenH - m.Height) / 2
}

// Hide hides the modal
func (m *EnhancedModal) Hide() {
	m.visible = false
	m.animating = false
	m.animPhase = 0
}

// IsVisible returns whether the modal is visible
func (m *EnhancedModal) IsVisible() bool {
	return m.visible
}

// GetBounds returns the modal bounds as a rectangle
func (m *EnhancedModal) GetBounds() image.Rectangle {
	return image.Rect(m.X, m.Y, m.X+m.Width, m.Y+m.Height)
}

// Contains checks if a point is inside the modal
func (m *EnhancedModal) Contains(x, y int) bool {
	if !m.visible {
		return false
	}
	return x >= m.X && x < m.X+m.Width && y >= m.Y && y < m.Y+m.Height
}

// Update updates the modal animation
func (m *EnhancedModal) Update() {
	if m.animating && m.visible {
		m.animPhase += m.animSpeed / 60.0 // Assuming 60 FPS
		if m.animPhase >= 1.0 {
			m.animPhase = 1.0
			m.animating = false
		}
	}
}

// Draw renders the modal
func (m *EnhancedModal) Draw(screen *ebiten.Image) {
	if !m.visible {
		return
	}

	m.Update()

	// Draw dark overlay background
	screenBounds := screen.Bounds()
	vector.DrawFilledRect(screen, 0, 0, float32(screenBounds.Dx()), float32(screenBounds.Dy()),
		EnhancedUIColors.Background, false)

	// Apply animation scaling
	scale := float32(0.8 + 0.2*m.animPhase) // Scale from 0.8 to 1.0
	alpha := uint8(255 * m.animPhase)       // Fade in

	// Calculate scaled position and size
	centerX := float32(m.X + m.Width/2)
	centerY := float32(m.Y + m.Height/2)
	scaledWidth := float32(m.Width) * scale
	scaledHeight := float32(m.Height) * scale
	scaledX := centerX - scaledWidth/2
	scaledY := centerY - scaledHeight/2

	// Draw modal background with animated alpha
	modalColor := EnhancedUIColors.ModalBackground
	modalColor.A = alpha
	vector.DrawFilledRect(screen, scaledX, scaledY, scaledWidth, scaledHeight, modalColor, false)

	// Draw border
	borderColor := EnhancedUIColors.Primary
	borderColor.A = alpha
	vector.StrokeRect(screen, scaledX, scaledY, scaledWidth, scaledHeight, 2, borderColor, false)

	// Only draw content if animation is mostly complete
	if m.animPhase > 0.8 {
		// Draw title bar
		titleBarHeight := float32(35)
		titleBarColor := EnhancedUIColors.Primary
		titleBarColor.A = alpha
		vector.DrawFilledRect(screen, scaledX, scaledY, scaledWidth, titleBarHeight, titleBarColor, false)

		// Draw title text
		if m.font != nil {
			titleX := int(scaledX) + 15
			titleY := int(scaledY) + 23
			titleColor := EnhancedUIColors.Text
			titleColor.A = alpha
			text.Draw(screen, m.Title, m.font, titleX, titleY, titleColor)
		}
	}
}

// GetContentArea returns the content area coordinates
func (m *EnhancedModal) GetContentArea() (int, int, int, int) {
	if !m.visible {
		return 0, 0, 0, 0
	}

	scale := float32(0.8 + 0.2*m.animPhase)
	centerX := float32(m.X + m.Width/2)
	centerY := float32(m.Y + m.Height/2)
	scaledWidth := float32(m.Width) * scale
	scaledHeight := float32(m.Height) * scale
	scaledX := centerX - scaledWidth/2
	scaledY := centerY - scaledHeight/2

	contentX := int(scaledX) + 10
	contentY := int(scaledY) + 45 // Below title bar
	contentW := int(scaledWidth) - 20
	contentH := int(scaledHeight) - 55

	return contentX, contentY, contentW, contentH
}

// EnhancedButton represents a styled button
type EnhancedButton struct {
	X, Y    int
	Width   int
	Height  int
	Text    string
	OnClick func()
	enabled bool
	hovered bool
	pressed bool
	font    font.Face
	// Styling fields
	backgroundColor *color.RGBA // Custom background color (nil = use default)
	hoverColor      *color.RGBA // Custom hover color (nil = use default)
	pressedColor    *color.RGBA // Custom pressed color (nil = use default)
	textColor       *color.RGBA // Custom text color (nil = use default)
	borderColor     *color.RGBA // Custom border color (nil = use default)
	disabledColor   *color.RGBA // Custom disabled color (nil = use default)
}

// NewEnhancedButton creates a new enhanced button
func NewEnhancedButton(text string, x, y, width, height int, onClick func()) *EnhancedButton {
	return &EnhancedButton{
		X:       x,
		Y:       y,
		Width:   width,
		Height:  height,
		Text:    text,
		OnClick: onClick,
		enabled: true,
		font:    loadWynncraftFont(16),
	}
}

// SetPosition updates the button position
func (b *EnhancedButton) SetPosition(x, y int) {
	b.X = x
	b.Y = y
}

// GetBounds returns the button bounds
func (b *EnhancedButton) GetBounds() image.Rectangle {
	return image.Rect(b.X, b.Y, b.X+b.Width, b.Y+b.Height)
}

// SetBackgroundColor sets the button's background color
func (b *EnhancedButton) SetBackgroundColor(c color.RGBA) {
	b.backgroundColor = &c
}

// SetHoverColor sets the button's hover color
func (b *EnhancedButton) SetHoverColor(c color.RGBA) {
	b.hoverColor = &c
}

// SetPressedColor sets the button's pressed color
func (b *EnhancedButton) SetPressedColor(c color.RGBA) {
	b.pressedColor = &c
}

// SetTextColor sets the button's text color
func (b *EnhancedButton) SetTextColor(c color.RGBA) {
	b.textColor = &c
}

// SetBorderColor sets the button's border color
func (b *EnhancedButton) SetBorderColor(c color.RGBA) {
	b.borderColor = &c
}

// SetDisabledColor sets the button's disabled color
func (b *EnhancedButton) SetDisabledColor(c color.RGBA) {
	b.disabledColor = &c
}

// ClearCustomStyling removes all custom styling, reverting to defaults
func (b *EnhancedButton) ClearCustomStyling() {
	b.backgroundColor = nil
	b.hoverColor = nil
	b.pressedColor = nil
	b.textColor = nil
	b.borderColor = nil
	b.disabledColor = nil
}

// SetButtonStyle sets a complete button style theme
func (b *EnhancedButton) SetButtonStyle(bg, hover, pressed color.RGBA) {
	b.backgroundColor = &bg
	b.hoverColor = &hover
	b.pressedColor = &pressed
}

// SetRedButtonStyle applies a red button theme (for dangerous actions)
func (b *EnhancedButton) SetRedButtonStyle() {
	b.SetButtonStyle(
		color.RGBA{220, 53, 69, 255}, // Red background
		color.RGBA{240, 73, 89, 255}, // Lighter red on hover
		color.RGBA{200, 33, 49, 255}, // Darker red when pressed
	)
	// Ensure white text for good contrast on red background
	b.SetTextColor(color.RGBA{255, 255, 255, 255})
}

// SetYellowButtonStyle applies a yellow button theme (for warning actions)
func (b *EnhancedButton) SetYellowButtonStyle() {
	b.SetButtonStyle(
		color.RGBA{255, 193, 7, 255},  // Yellow background
		color.RGBA{255, 213, 77, 255}, // Lighter yellow on hover
		color.RGBA{235, 173, 0, 255},  // Darker yellow when pressed
	)
	// Set dark text for better contrast on yellow background
	b.SetTextColor(color.RGBA{0, 0, 0, 255})
}

// SetGreenButtonStyle applies a green button theme (for success actions)
func (b *EnhancedButton) SetGreenButtonStyle() {
	b.SetButtonStyle(
		color.RGBA{40, 167, 69, 255}, // Green background
		color.RGBA{60, 187, 89, 255}, // Lighter green on hover
		color.RGBA{20, 147, 49, 255}, // Darker green when pressed
	)
}

// SetBlueButtonStyle applies a blue button theme (for primary actions)
func (b *EnhancedButton) SetBlueButtonStyle() {
	b.SetButtonStyle(
		color.RGBA{70, 130, 255, 255}, // Blue background
		color.RGBA{90, 150, 255, 255}, // Lighter blue on hover
		color.RGBA{50, 110, 235, 255}, // Darker blue when pressed
	)
}

// SetGrayButtonStyle applies a gray button theme (for secondary actions)
func (b *EnhancedButton) SetGrayButtonStyle() {
	b.SetButtonStyle(
		color.RGBA{108, 117, 125, 255}, // Gray background
		color.RGBA{128, 137, 145, 255}, // Lighter gray on hover
		color.RGBA{88, 97, 105, 255},   // Darker gray when pressed
	)
	// Ensure white text for good contrast on gray background
	b.SetTextColor(color.RGBA{255, 255, 255, 255})
}

// Update handles button input
func (b *EnhancedButton) Update(mx, my int) bool {
	if !b.enabled {
		b.hovered = false
		b.pressed = false
		return false
	}

	bounds := b.GetBounds()
	b.hovered = mx >= bounds.Min.X && mx < bounds.Max.X && my >= bounds.Min.Y && my < bounds.Max.Y

	if b.hovered && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		b.pressed = true
		if b.OnClick != nil {
			b.OnClick()
		}
		return true
	}

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		b.pressed = false
	}

	return false
}

// Draw renders the button
func (b *EnhancedButton) Draw(screen *ebiten.Image) {
	// Button background - use custom colors if set
	var bgColor color.RGBA
	if !b.enabled {
		if b.disabledColor != nil {
			bgColor = *b.disabledColor
		} else {
			bgColor = color.RGBA{40, 40, 40, 255}
		}
	} else if b.pressed {
		if b.pressedColor != nil {
			bgColor = *b.pressedColor
		} else {
			bgColor = EnhancedUIColors.ButtonActive
		}
	} else if b.hovered {
		if b.hoverColor != nil {
			bgColor = *b.hoverColor
		} else {
			bgColor = EnhancedUIColors.ButtonHover
		}
	} else {
		if b.backgroundColor != nil {
			bgColor = *b.backgroundColor
		} else {
			bgColor = EnhancedUIColors.Button
		}
	}

	vector.DrawFilledRect(screen, float32(b.X), float32(b.Y), float32(b.Width), float32(b.Height), bgColor, false)

	// Button border - use custom border color if set
	var borderColor color.RGBA
	if b.borderColor != nil {
		borderColor = *b.borderColor
	} else {
		borderColor = EnhancedUIColors.Border
		if b.enabled && (b.hovered || b.pressed) {
			borderColor = EnhancedUIColors.BorderActive
		}
	}
	vector.StrokeRect(screen, float32(b.X), float32(b.Y), float32(b.Width), float32(b.Height), 2, borderColor, false)

	// Button text - use custom text color if set
	if b.font != nil && b.Text != "" {
		var textColor color.RGBA
		if b.textColor != nil {
			textColor = *b.textColor
		} else {
			textColor = EnhancedUIColors.Text
			if !b.enabled {
				textColor = EnhancedUIColors.TextSecondary
			}
		}

		bounds := text.BoundString(b.font, b.Text)
		textX := b.X + (b.Width-bounds.Dx())/2
		textY := b.Y + (b.Height+bounds.Dy())/2 - 2
		text.Draw(screen, b.Text, b.font, textX, textY, textColor)
	}
}
