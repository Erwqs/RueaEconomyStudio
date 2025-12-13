package app

import (
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

// UI Constants
const (
	UIModalBackgroundOpacity = 180
	UIBorderRadius           = 5
	UIButtonHeight           = 30
	UIInputHeight            = 25
	UIScrollbarWidth         = 12
	UIColorPickerSize        = 200
	UIAnimationSpeed         = 8.0
)

// UIColors defines the color palette for the UI system
var UIColors = struct {
	Background    color.RGBA
	Surface       color.RGBA
	Primary       color.RGBA
	Secondary     color.RGBA
	Accent        color.RGBA
	Text          color.RGBA
	TextSecondary color.RGBA
	Border        color.RGBA
	Success       color.RGBA
	Warning       color.RGBA
	Error         color.RGBA
	Hover         color.RGBA
}{
	Background:    color.RGBA{20, 20, 30, 255},
	Surface:       color.RGBA{40, 40, 50, 255},
	Primary:       color.RGBA{70, 130, 255, 255},
	Secondary:     color.RGBA{100, 100, 120, 255},
	Accent:        color.RGBA{255, 170, 50, 255},
	Text:          color.RGBA{255, 255, 255, 255},
	TextSecondary: color.RGBA{180, 180, 180, 255},
	Border:        color.RGBA{80, 80, 90, 255},
	Success:       color.RGBA{80, 200, 120, 255},
	Warning:       color.RGBA{255, 200, 80, 255},
	Error:         color.RGBA{255, 100, 100, 255},
	Hover:         color.RGBA{255, 255, 255, 30},
}

// UIModal represents a modal window
type UIModal struct {
	Title       string
	X, Y        int
	Width       int
	Height      int
	Visible     bool
	Closable    bool
	Content     UIComponent
	CloseButton UIButton
	animated    bool
	animPhase   float64
}

// UIComponent interface for all UI components
type UIComponent interface {
	Update(mx, my int) bool // Returns true if component handled input
	Draw(screen *ebiten.Image)
	GetBounds() image.Rectangle
}

// UIButton represents a clickable button
type UIButton struct {
	Text    string
	X, Y    int
	Width   int
	Height  int
	OnClick func()
	Enabled bool
	Style   ButtonStyle
	hovered bool
	pressed bool
	font    font.Face
}

// ButtonStyle defines button appearance
type ButtonStyle struct {
	BackgroundColor color.RGBA
	HoverColor      color.RGBA
	PressedColor    color.RGBA
	TextColor       color.RGBA
	BorderColor     color.RGBA
	BorderWidth     float32
}

// UITextInput represents a text input field
type UITextInput struct {
	Value       string
	Placeholder string
	X, Y        int
	Width       int
	Height      int
	MaxLength   int
	Focused     bool
	OnChange    func(value string)
	OnEnter     func(value string)
	font        font.Face
	cursorPos   int
	blinkTimer  time.Time
}

// UIColorPicker represents a color picker component
type UIColorPicker struct {
	SelectedColor color.RGBA
	X, Y          int
	Size          int
	OnColorChange func(color.RGBA)
	isDragging    bool
	saturation    float64
	brightness    float64
	hue           float64
}

// UIScrollableList represents a scrollable list of items
type UIScrollableList struct {
	Items          []UIListItem
	X, Y           int
	Width          int
	Height         int
	ScrollY        int
	ItemHeight     int
	OnItemClick    func(index int, item UIListItem)
	font           font.Face
	hoveredItem    int
	scrolling      bool
	touchScrolling bool
	scrollStartY   int
	scrollStartPos int
}

// UIListItem represents an item in a scrollable list
type UIListItem struct {
	Text     string
	Data     interface{}
	Color    color.RGBA
	Icon     *ebiten.Image
	Selected bool
}

// UIContainer groups multiple UI components
type UIContainer struct {
	Components []UIComponent
	X, Y       int
	Width      int
	Height     int
}

// NewUIModal creates a new modal window
func NewUIModal(title string, x, y, width, height int) *UIModal {
	modal := &UIModal{
		Title:     title,
		X:         x,
		Y:         y,
		Width:     width,
		Height:    height,
		Visible:   false,
		Closable:  true,
		animated:  true,
		animPhase: 0,
	}

	// Create close button
	modal.CloseButton = UIButton{
		Text:    "×",
		X:       x + width - 30,
		Y:       y + 5,
		Width:   25,
		Height:  25,
		Enabled: true,
		OnClick: func() { modal.Hide() },
		Style:   GetDefaultButtonStyle(),
		font:    loadWynncraftFont(16),
	}

	return modal
}

// Show displays the modal with animation
func (m *UIModal) Show() {
	m.Visible = true
	m.animPhase = 0
}

// Hide conceals the modal with animation
func (m *UIModal) Hide() {
	m.Visible = false
	m.animPhase = 0
}

// Update handles modal input and animations
func (m *UIModal) Update(mx, my int) bool {
	if !m.Visible {
		return false
	}

	px, py := primaryPointerPosition()

	// Handle animation
	if m.animated && m.animPhase < 1.0 {
		m.animPhase += UIAnimationSpeed / 60.0 // Assuming 60 FPS
		if m.animPhase > 1.0 {
			m.animPhase = 1.0
		}
	}

	// Handle close button
	if m.Closable && m.CloseButton.Update(px, py) {
		return true
	}

	// Handle content
	if m.Content != nil {
		// Adjust coordinates relative to modal content area
		contentX := px - (m.X + 10)
		contentY := py - (m.Y + 40)
		if m.Content.Update(contentX, contentY) {
			return true
		}
	}

	// Check if click is inside modal bounds
	bounds := m.GetBounds()
	if px >= bounds.Min.X && px <= bounds.Max.X && py >= bounds.Min.Y && py <= bounds.Max.Y {
		return true // Modal consumed the input
	}

	// Click outside modal - close if closable
	if m.Closable {
		if _, _, pressed := primaryJustPressed(); pressed {
			m.Hide()
			return true
		}
	}
	return false
}

// Draw renders the modal
func (m *UIModal) Draw(screen *ebiten.Image) {
	if !m.Visible {
		return
	}

	// Apply animation scaling
	scale := float32(1.0)
	if m.animated {
		// Ease-out animation
		t := m.animPhase
		scale = float32(0.8 + 0.2*t) // Scale from 0.8 to 1.0
	}

	// Draw semi-transparent background
	bgColor := color.RGBA{0, 0, 0, UIModalBackgroundOpacity}
	vector.DrawFilledRect(screen, 0, 0, float32(screen.Bounds().Dx()), float32(screen.Bounds().Dy()), bgColor, false)

	// Calculate scaled position and size
	centerX := float32(m.X + m.Width/2)
	centerY := float32(m.Y + m.Height/2)
	scaledWidth := float32(m.Width) * scale
	scaledHeight := float32(m.Height) * scale
	scaledX := centerX - scaledWidth/2
	scaledY := centerY - scaledHeight/2

	// Draw modal background
	vector.DrawFilledRect(screen, scaledX, scaledY, scaledWidth, scaledHeight, UIColors.Surface, false)

	// Draw border
	vector.StrokeRect(screen, scaledX, scaledY, scaledWidth, scaledHeight, 2, UIColors.Border, false)

	// Only draw content if animation is complete or nearly complete
	if m.animPhase > 0.8 {
		// Draw title bar
		titleBarHeight := float32(35)
		vector.DrawFilledRect(screen, scaledX, scaledY, scaledWidth, titleBarHeight, UIColors.Primary, false)

		// Draw title text
		if titleFont := loadWynncraftFont(16); titleFont != nil {
			titleX := int(scaledX) + 10
			titleY := int(scaledY) + 20
			text.Draw(screen, m.Title, titleFont, titleX, titleY, UIColors.Text)
		}

		// Draw close button if closable
		if m.Closable {
			m.CloseButton.X = int(scaledX + scaledWidth - 30)
			m.CloseButton.Y = int(scaledY + 5)
			m.CloseButton.Draw(screen)
		}

		// Draw content
		if m.Content != nil {
			// Set up clipping for content area
			contentX := int(scaledX + 10)
			contentY := int(scaledY + 40)
			contentW := int(scaledWidth - 20)
			contentH := int(scaledHeight - 50)

			// Use SubImage for clipping (if content exceeds bounds)
			if contentX >= 0 && contentY >= 0 && contentX+contentW <= screen.Bounds().Dx() && contentY+contentH <= screen.Bounds().Dy() {
				contentArea := screen.SubImage(image.Rect(contentX, contentY, contentX+contentW, contentY+contentH)).(*ebiten.Image)
				m.Content.Draw(contentArea)
			}
		}
	}
}

// GetBounds returns the modal's bounding rectangle
func (m *UIModal) GetBounds() image.Rectangle {
	return image.Rect(m.X, m.Y, m.X+m.Width, m.Y+m.Height)
}

// GetDefaultButtonStyle returns a default button style
func GetDefaultButtonStyle() ButtonStyle {
	return ButtonStyle{
		BackgroundColor: UIColors.Secondary,
		HoverColor:      UIColors.Primary,
		PressedColor:    UIColors.Accent,
		TextColor:       UIColors.Text,
		BorderColor:     UIColors.Border,
		BorderWidth:     1,
	}
}

// NewUIButton creates a new button
func NewUIButton(text string, x, y, width, height int, onClick func()) *UIButton {
	return &UIButton{
		Text:    text,
		X:       x,
		Y:       y,
		Width:   width,
		Height:  height,
		OnClick: onClick,
		Enabled: true,
		Style:   GetDefaultButtonStyle(),
		font:    loadWynncraftFont(14),
	}
}

// Update handles button input
func (b *UIButton) Update(mx, my int) bool {
	if !b.Enabled {
		return false
	}

	px, py := primaryPointerPosition()

	bounds := b.GetBounds()
	b.hovered = px >= bounds.Min.X && px <= bounds.Max.X && py >= bounds.Min.Y && py <= bounds.Max.Y

	if b.hovered {
		if _, _, pressed := primaryJustPressed(); pressed {
			b.pressed = true
			return true
		}
		if _, _, released := primaryJustReleased(); released && b.pressed {
			b.pressed = false
			if b.OnClick != nil {
				b.OnClick()
			}
			return true
		}
	}

	if _, _, released := primaryJustReleased(); released {
		b.pressed = false
	}

	return false
}

// Draw renders the button
func (b *UIButton) Draw(screen *ebiten.Image) {
	bounds := b.GetBounds()

	// Choose background color based on state
	bgColor := b.Style.BackgroundColor
	if !b.Enabled {
		bgColor = UIColors.Secondary
		bgColor.A = 128
	} else if b.pressed {
		bgColor = b.Style.PressedColor
	} else if b.hovered {
		bgColor = b.Style.HoverColor
	}

	// Draw button background
	vector.DrawFilledRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), bgColor, false)

	// Draw border
	if b.Style.BorderWidth > 0 {
		vector.StrokeRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
			float32(bounds.Dx()), float32(bounds.Dy()), b.Style.BorderWidth, b.Style.BorderColor, false)
	}

	// Draw text
	if b.font != nil && b.Text != "" {
		textColor := b.Style.TextColor
		if !b.Enabled {
			textColor = UIColors.TextSecondary
		}

		// Center text in button
		textBounds := text.BoundString(b.font, b.Text)
		textX := bounds.Min.X + (bounds.Dx()-textBounds.Dx())/2
		textY := bounds.Min.Y + (bounds.Dy()+textBounds.Dy())/2
		text.Draw(screen, b.Text, b.font, textX, textY, textColor)
	}
}

// GetBounds returns the button's bounding rectangle
func (b *UIButton) GetBounds() image.Rectangle {
	return image.Rect(b.X, b.Y, b.X+b.Width, b.Y+b.Height)
}

// NewUITextInput creates a new text input field
func NewUITextInput(placeholder string, x, y, width int, maxLength int) *UITextInput {
	return &UITextInput{
		Placeholder: placeholder,
		X:           x,
		Y:           y,
		Width:       width,
		Height:      UIInputHeight,
		MaxLength:   maxLength,
		font:        loadWynncraftFont(14),
		blinkTimer:  time.Now(),
	}
}

// Update handles text input
func (t *UITextInput) Update(mx, my int) bool {
	px, py := primaryPointerPosition()
	bounds := t.GetBounds()
	wasClicked := px >= bounds.Min.X && px <= bounds.Max.X && py >= bounds.Min.Y && py <= bounds.Max.Y

	if _, _, pressed := primaryJustPressed(); pressed {
		t.Focused = wasClicked
		if wasClicked {
			// Position cursor based on click position
			// This is a simplified version - could be improved with proper text metrics
			clickOffset := px - bounds.Min.X - 5
			if clickOffset <= 0 {
				t.cursorPos = 0
			} else if t.font != nil {
				// Estimate cursor position
				for i := 0; i <= len(t.Value); i++ {
					textWidth := text.BoundString(t.font, t.Value[:i]).Dx()
					if textWidth >= clickOffset {
						t.cursorPos = i
						break
					}
				}
				if t.cursorPos > len(t.Value) {
					t.cursorPos = len(t.Value)
				}
			}
			return true
		}
	}

	if !t.Focused {
		return false
	}

	// Handle keyboard input
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if t.OnEnter != nil {
			t.OnEnter(t.Value)
		}
		t.Focused = false
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.Focused = false
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && t.cursorPos > 0 {
		t.Value = t.Value[:t.cursorPos-1] + t.Value[t.cursorPos:]
		t.cursorPos--
		if t.OnChange != nil {
			t.OnChange(t.Value)
		}
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) && t.cursorPos < len(t.Value) {
		t.Value = t.Value[:t.cursorPos] + t.Value[t.cursorPos+1:]
		if t.OnChange != nil {
			t.OnChange(t.Value)
		}
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) && t.cursorPos > 0 {
		t.cursorPos--
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) && t.cursorPos < len(t.Value) {
		t.cursorPos++
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		t.cursorPos = 0
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
		t.cursorPos = len(t.Value)
		return true
	}

	// Handle character input - simplified approach using key presses
	// This handles basic ASCII character input
	for key := ebiten.KeyA; key <= ebiten.KeyZ; key++ {
		if inpututil.IsKeyJustPressed(key) {
			var char rune
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				char = rune('A' + int(key-ebiten.KeyA))
			} else {
				char = rune('a' + int(key-ebiten.KeyA))
			}
			if len(t.Value) < t.MaxLength {
				t.Value = t.Value[:t.cursorPos] + string(char) + t.Value[t.cursorPos:]
				t.cursorPos++
				if t.OnChange != nil {
					t.OnChange(t.Value)
				}
			}
		}
	}

	// Handle digit input
	for key := ebiten.Key0; key <= ebiten.Key9; key++ {
		if inpututil.IsKeyJustPressed(key) {
			char := rune('0' + int(key-ebiten.Key0))
			if len(t.Value) < t.MaxLength {
				t.Value = t.Value[:t.cursorPos] + string(char) + t.Value[t.cursorPos:]
				t.cursorPos++
				if t.OnChange != nil {
					t.OnChange(t.Value)
				}
			}
		}
	}

	// Handle some common special characters
	specialKeys := map[ebiten.Key]rune{
		ebiten.KeySpace:     ' ',
		ebiten.KeyMinus:     '-',
		ebiten.KeyEqual:     '=',
		ebiten.KeyPeriod:    '.',
		ebiten.KeyComma:     ',',
		ebiten.KeySemicolon: ';',
		ebiten.KeyQuote:     '\'',
	}

	for key, char := range specialKeys {
		if inpututil.IsKeyJustPressed(key) && len(t.Value) < t.MaxLength {
			t.Value = t.Value[:t.cursorPos] + string(char) + t.Value[t.cursorPos:]
			t.cursorPos++
			if t.OnChange != nil {
				t.OnChange(t.Value)
			}
		}
	}

	return wasClicked
}

// Draw renders the text input
func (t *UITextInput) Draw(screen *ebiten.Image) {
	bounds := t.GetBounds()

	// Background
	bgColor := UIColors.Surface
	if t.Focused {
		bgColor = color.RGBA{50, 50, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), bgColor, false)

	// Border
	borderColor := UIColors.Border
	if t.Focused {
		borderColor = UIColors.Primary
	}
	vector.StrokeRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), 2, borderColor, false)

	// Text content
	displayText := t.Value
	textColor := UIColors.Text
	if displayText == "" && !t.Focused {
		displayText = t.Placeholder
		textColor = UIColors.TextSecondary
	}

	if t.font != nil && displayText != "" {
		textX := bounds.Min.X + 5
		textY := bounds.Min.Y + (bounds.Dy()+text.BoundString(t.font, displayText).Dy())/2
		text.Draw(screen, displayText, t.font, textX, textY, textColor)
	}

	// Draw cursor if focused
	if t.Focused && time.Since(t.blinkTimer).Milliseconds()%1000 < 500 {
		if t.font != nil {
			cursorText := t.Value[:t.cursorPos]
			cursorX := bounds.Min.X + 5 + text.BoundString(t.font, cursorText).Dx()
			cursorY1 := bounds.Min.Y + 3
			cursorY2 := bounds.Min.Y + bounds.Dy() - 3
			vector.StrokeLine(screen, float32(cursorX), float32(cursorY1),
				float32(cursorX), float32(cursorY2), 1, UIColors.Text, false)
		}
	}
}

// GetBounds returns the text input's bounding rectangle
func (t *UITextInput) GetBounds() image.Rectangle {
	return image.Rect(t.X, t.Y, t.X+t.Width, t.Y+t.Height)
}

// SetText sets the text input's value
func (t *UITextInput) SetText(text string) {
	t.Value = text
	t.cursorPos = len(text)
}

// GetText returns the text input's current value
func (t *UITextInput) GetText() string {
	return t.Value
}

// isPrintableChar checks if a character is printable
func isPrintableChar(r rune) bool {
	return r >= 32 && r <= 126
}

// NewUIColorPicker creates a new color picker
func NewUIColorPicker(x, y, size int, initialColor color.RGBA) *UIColorPicker {
	h, s, v := uiRgbToHSV(initialColor.R, initialColor.G, initialColor.B)
	return &UIColorPicker{
		SelectedColor: initialColor,
		X:             x,
		Y:             y,
		Size:          size,
		hue:           h,
		saturation:    s,
		brightness:    v,
	}
}

// Update handles color picker input
func (cp *UIColorPicker) Update(mx, my int) bool {
	px, py := primaryPointerPosition()
	bounds := cp.GetBounds()
	if px < bounds.Min.X || px > bounds.Max.X || py < bounds.Min.Y || py > bounds.Max.Y {
		if _, _, released := primaryJustReleased(); released {
			cp.isDragging = false
		}
		return false
	}

	if _, _, pressed := primaryJustPressed(); pressed {
		cp.isDragging = true
	}

	if cp.isDragging && primaryPressed() {
		// Convert mouse position to color
		relX := float64(px-bounds.Min.X) / float64(bounds.Dx())
		relY := float64(py-bounds.Min.Y) / float64(bounds.Dy())

		relX = math.Max(0, math.Min(1, relX))
		relY = math.Max(0, math.Min(1, relY))

		// Simple color picker implementation
		// Left side: hue selection, right side: saturation/brightness
		if relX < 0.2 { // Hue strip
			cp.hue = relY * 360
		} else { // Saturation/brightness area
			cp.saturation = (relX - 0.2) / 0.8
			cp.brightness = 1.0 - relY
		}

		// Update selected color
		r, g, b := uiHsvToRGB(cp.hue, cp.saturation, cp.brightness)
		cp.SelectedColor = color.RGBA{r, g, b, 255}

		if cp.OnColorChange != nil {
			cp.OnColorChange(cp.SelectedColor)
		}
		return true
	}

	if _, _, released := primaryJustReleased(); released {
		cp.isDragging = false
	}

	return false
}

// Draw renders the color picker
func (cp *UIColorPicker) Draw(screen *ebiten.Image) {
	bounds := cp.GetBounds()

	// Draw hue strip on the left
	hueStripWidth := bounds.Dx() / 5
	for y := 0; y < bounds.Dy(); y++ {
		hue := float64(y) / float64(bounds.Dy()) * 360
		r, g, b := uiHsvToRGB(hue, 1.0, 1.0)
		hueColor := color.RGBA{r, g, b, 255}
		vector.DrawFilledRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y+y),
			float32(hueStripWidth), 1, hueColor, false)
	}

	// Draw saturation/brightness area
	sbX := bounds.Min.X + hueStripWidth
	sbWidth := bounds.Dx() - hueStripWidth
	for x := 0; x < sbWidth; x++ {
		for y := 0; y < bounds.Dy(); y++ {
			sat := float64(x) / float64(sbWidth)
			bright := 1.0 - float64(y)/float64(bounds.Dy())
			r, g, b := uiHsvToRGB(cp.hue, sat, bright)
			sbColor := color.RGBA{r, g, b, 255}
			vector.DrawFilledRect(screen, float32(sbX+x), float32(bounds.Min.Y+y),
				1, 1, sbColor, false)
		}
	}

	// Draw border
	vector.StrokeRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), 2, UIColors.Border, false)

	// Draw current color indicator
	selectedY := int(cp.hue / 360.0 * float64(bounds.Dy()))
	vector.DrawFilledRect(screen, float32(bounds.Min.X-2), float32(bounds.Min.Y+selectedY-2),
		float32(hueStripWidth+4), 4, UIColors.Text, false)

	// Draw saturation/brightness indicator
	selX := sbX + int(cp.saturation*float64(sbWidth))
	selY := bounds.Min.Y + int((1.0-cp.brightness)*float64(bounds.Dy()))
	vector.DrawFilledCircle(screen, float32(selX), float32(selY), 4, UIColors.Text, false)
}

// GetBounds returns the color picker's bounding rectangle
func (cp *UIColorPicker) GetBounds() image.Rectangle {
	return image.Rect(cp.X, cp.Y, cp.X+cp.Size, cp.Y+cp.Size)
}

// Color conversion utilities
func uiRgbToHSV(r, g, b uint8) (h, s, v float64) {
	rf, gf, bf := float64(r)/255.0, float64(g)/255.0, float64(b)/255.0
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	delta := max - min

	v = max

	if max == 0 {
		s = 0
	} else {
		s = delta / max
	}

	if delta == 0 {
		h = 0
	} else {
		switch max {
		case rf:
			h = 60 * (math.Mod((gf-bf)/delta, 6))
		case gf:
			h = 60 * ((bf-rf)/delta + 2)
		case bf:
			h = 60 * ((rf-gf)/delta + 4)
		}
	}

	if h < 0 {
		h += 360
	}

	return h, s, v
}

func uiHsvToRGB(h, s, v float64) (r, g, b uint8) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}

	r = uint8((rf + m) * 255)
	g = uint8((gf + m) * 255)
	b = uint8((bf + m) * 255)
	return
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// NewUIScrollableList creates a new scrollable list
func NewUIScrollableList(x, y, width, height, itemHeight int) *UIScrollableList {
	return &UIScrollableList{
		X:           x,
		Y:           y,
		Width:       width,
		Height:      height,
		ItemHeight:  itemHeight,
		hoveredItem: -1,
		font:        loadWynncraftFont(14),
	}
}

// Update handles scrollable list input
func (sl *UIScrollableList) Update(mx, my int) bool {
	px, py := primaryPointerPosition()
	bounds := sl.GetBounds()
	inside := px >= bounds.Min.X && px <= bounds.Max.X && py >= bounds.Min.Y && py <= bounds.Max.Y

	if !inside && !sl.touchScrolling {
		sl.hoveredItem = -1
		return false
	}

	// Handle mouse wheel scrolling
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		sl.ScrollY -= int(wheelY * 30)
	}

	if inside {
		relY := py - bounds.Min.Y + sl.ScrollY
		itemIndex := relY / sl.ItemHeight
		if itemIndex >= 0 && itemIndex < len(sl.Items) {
			sl.hoveredItem = itemIndex
		} else {
			sl.hoveredItem = -1
		}
	}

	// Start touch/mouse drag scrolling
	if _, pressY, pressed := primaryJustPressed(); pressed && inside {
		sl.touchScrolling = true
		sl.scrollStartY = pressY
		sl.scrollStartPos = sl.ScrollY
		return true
	}

	maxScroll := len(sl.Items)*sl.ItemHeight - sl.Height
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Continue drag scrolling
	if sl.touchScrolling && primaryPressed() {
		delta := py - sl.scrollStartY
		newScroll := sl.scrollStartPos - delta
		if newScroll < 0 {
			newScroll = 0
		} else if newScroll > maxScroll {
			newScroll = maxScroll
		}
		sl.ScrollY = newScroll
		return true
	}

	// Release: stop scrolling and treat as tap if there was minimal movement
	if rx, ry, released := primaryJustReleased(); released {
		if sl.touchScrolling {
			movement := absInt(ry - sl.scrollStartY)
			sl.touchScrolling = false
			if movement < sl.ItemHeight/4 {
				relY := ry - bounds.Min.Y + sl.ScrollY
				itemIndex := relY / sl.ItemHeight
				insideRelease := rx >= bounds.Min.X && rx <= bounds.Max.X && ry >= bounds.Min.Y && ry <= bounds.Max.Y
				if itemIndex >= 0 && itemIndex < len(sl.Items) && insideRelease {
					sl.hoveredItem = itemIndex
					if sl.OnItemClick != nil {
						sl.OnItemClick(itemIndex, sl.Items[itemIndex])
					}
				}
			}
			return true
		}
	}

	// Clamp scroll position after wheel input
	if sl.ScrollY < 0 {
		sl.ScrollY = 0
	} else if sl.ScrollY > maxScroll {
		sl.ScrollY = maxScroll
	}

	return inside
}

// Draw renders the scrollable list
func (sl *UIScrollableList) Draw(screen *ebiten.Image) {
	bounds := sl.GetBounds()

	// Draw background
	vector.DrawFilledRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), UIColors.Surface, false)

	// Draw border
	vector.StrokeRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y),
		float32(bounds.Dx()), float32(bounds.Dy()), 1, UIColors.Border, false)

	// Create clipping area for items
	if bounds.Min.X >= 0 && bounds.Min.Y >= 0 && bounds.Max.X <= screen.Bounds().Dx() && bounds.Max.Y <= screen.Bounds().Dy() {
		listArea := screen.SubImage(bounds).(*ebiten.Image)

		// Draw items
		for i, item := range sl.Items {
			itemY := i*sl.ItemHeight - sl.ScrollY
			if itemY+sl.ItemHeight < 0 || itemY > bounds.Dy() {
				continue // Skip items outside visible area
			}

			itemBounds := image.Rect(0, itemY, bounds.Dx(), itemY+sl.ItemHeight)

			// Draw item background
			itemBgColor := UIColors.Surface
			if i == sl.hoveredItem {
				itemBgColor = UIColors.Hover
			} else if item.Selected {
				itemBgColor = UIColors.Primary
			}

			if itemY >= 0 && itemY+sl.ItemHeight <= bounds.Dy() {
				vector.DrawFilledRect(listArea, float32(itemBounds.Min.X), float32(itemBounds.Min.Y),
					float32(itemBounds.Dx()), float32(itemBounds.Dy()), itemBgColor, false)
			}

			// Draw item text
			if sl.font != nil && item.Text != "" {
				textColor := item.Color
				if textColor.A == 0 {
					textColor = UIColors.Text
				}

				textX := 5
				textY := itemY + (sl.ItemHeight+text.BoundString(sl.font, item.Text).Dy())/2
				if textY > 0 && textY < bounds.Dy() {
					text.Draw(listArea, item.Text, sl.font, textX, textY, textColor)
				}
			}
		}

		// Draw scrollbar if needed
		maxScroll := len(sl.Items)*sl.ItemHeight - sl.Height
		if maxScroll > 0 {
			scrollbarHeight := (sl.Height * sl.Height) / (len(sl.Items) * sl.ItemHeight)
			scrollbarY := (sl.ScrollY * (sl.Height - scrollbarHeight)) / maxScroll

			vector.DrawFilledRect(listArea,
				float32(bounds.Dx()-UIScrollbarWidth), float32(scrollbarY),
				UIScrollbarWidth, float32(scrollbarHeight),
				UIColors.Secondary, false)
		}
	}
}

// GetBounds returns the scrollable list's bounding rectangle
func (sl *UIScrollableList) GetBounds() image.Rectangle {
	return image.Rect(sl.X, sl.Y, sl.X+sl.Width, sl.Y+sl.Height)
}

// AddItem adds an item to the scrollable list
func (sl *UIScrollableList) AddItem(item UIListItem) {
	sl.Items = append(sl.Items, item)
}

// RemoveItem removes an item at the specified index
func (sl *UIScrollableList) RemoveItem(index int) {
	if index >= 0 && index < len(sl.Items) {
		sl.Items = append(sl.Items[:index], sl.Items[index+1:]...)
	}
}

// ClearItems removes all items from the list
func (sl *UIScrollableList) ClearItems() {
	sl.Items = nil
}

// Update handles container input
func (c *UIContainer) Update(mx, my int) bool {
	// Process components in reverse order (top to bottom)
	for i := len(c.Components) - 1; i >= 0; i-- {
		if c.Components[i].Update(mx-c.X, my-c.Y) {
			return true
		}
	}
	return false
}

// Draw renders all components in the container
func (c *UIContainer) Draw(screen *ebiten.Image) {
	for _, component := range c.Components {
		component.Draw(screen)
	}
}

// GetBounds returns the container's bounding rectangle
func (c *UIContainer) GetBounds() image.Rectangle {
	return image.Rect(c.X, c.Y, c.X+c.Width, c.Y+c.Height)
}

// AddComponent adds a component to the container
func (c *UIContainer) AddComponent(component UIComponent) {
	c.Components = append(c.Components, component)
}
