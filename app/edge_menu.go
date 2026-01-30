package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"strconv"
	"strings"
	"time"

	"RueaES/eruntime"
	"RueaES/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
)

// EdgeMenuPosition defines where the menu is positioned
type EdgeMenuPosition int

const (
	EdgeMenuRight EdgeMenuPosition = iota
	EdgeMenuLeft
	EdgeMenuTop
	EdgeMenuBottom
)

// EdgeMenuOptions configures the edge menu
type EdgeMenuOptions struct {
	Width            int
	Height           int
	Position         EdgeMenuPosition
	Background       color.RGBA
	BorderColor      color.RGBA
	Scrollable       bool
	HorizontalScroll bool // Enable horizontal scrolling instead of vertical
	Closable         bool
	Animated         bool
}

// DefaultEdgeMenuOptions returns default options for edge menu
func DefaultEdgeMenuOptions() EdgeMenuOptions {
	return EdgeMenuOptions{
		Width:            400,
		Height:           0, // 0 means full screen height/width
		Position:         EdgeMenuRight,
		Background:       color.RGBA{30, 30, 45, 240},
		BorderColor:      color.RGBA{80, 80, 255, 150},
		Scrollable:       true,
		HorizontalScroll: false, // Default to vertical scrolling
		Closable:         true,
		Animated:         true,
	}
}

// EdgeMenuElement represents a base interface for all menu elements
type EdgeMenuElement interface {
	Update(mx, my int, deltaTime float64) bool                      // Returns true if handled input
	Draw(screen *ebiten.Image, x, y, width int, font font.Face) int // Returns height used
	GetMinHeight() int
	IsVisible() bool
	SetVisible(visible bool)
}

// BaseMenuElement provides common functionality for menu elements
type BaseMenuElement struct {
	visible        bool
	animProgress   float64
	animTarget     float64
	animSpeed      float64
	revealProgress float64
}

func NewBaseMenuElement() BaseMenuElement {
	return BaseMenuElement{
		visible:        true,
		animProgress:   1.0,
		animTarget:     1.0,
		animSpeed:      24.0,
		revealProgress: 1.0,
	}
}

func (b *BaseMenuElement) IsVisible() bool {
	return b.visible
}

func (b *BaseMenuElement) SetVisible(visible bool) {
	b.visible = visible
	b.animTarget = 0.0
	if visible {
		b.animTarget = 1.0
	}
}

func (b *BaseMenuElement) SetRevealProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	b.revealProgress = progress
}

func (b *BaseMenuElement) displayAlpha() float64 {
	return b.animProgress * b.revealProgress
}

func (b *BaseMenuElement) updateAnimation(deltaTime float64) {
	if math.Abs(b.animProgress-b.animTarget) > 0.01 {
		diff := b.animTarget - b.animProgress
		b.animProgress += diff * b.animSpeed * deltaTime
	} else {
		b.animProgress = b.animTarget
	}
}

// ButtonOptions configures button appearance and behavior
type ButtonOptions struct {
	BackgroundColor color.RGBA
	HoverColor      color.RGBA
	PressedColor    color.RGBA
	TextColor       color.RGBA
	BorderColor     color.RGBA
	BorderWidth     float32
	Height          int
	FontSize        int
	Enabled         bool
}

func DefaultButtonOptions() ButtonOptions {
	return ButtonOptions{
		BackgroundColor: color.RGBA{60, 120, 60, 255},
		HoverColor:      color.RGBA{80, 160, 80, 255},
		PressedColor:    color.RGBA{50, 100, 50, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		BorderColor:     color.RGBA{100, 200, 100, 255},
		BorderWidth:     2.0,
		Height:          35,
		FontSize:        16,
		Enabled:         true,
	}
}

// MenuButton represents a clickable button element
type MenuButton struct {
	BaseMenuElement
	text     string
	options  ButtonOptions
	callback func()
	hovered  bool
	pressed  bool
	rect     image.Rectangle
}

func NewMenuButton(text string, options ButtonOptions, callback func()) *MenuButton {
	return &MenuButton{
		BaseMenuElement: NewBaseMenuElement(),
		text:            text,
		options:         options,
		callback:        callback,
	}
}

// SetEnabled toggles the button's enabled state.
func (b *MenuButton) SetEnabled(enabled bool) {
	b.options.Enabled = enabled
}

// SetText updates the button label.
func (b *MenuButton) SetText(text string) {
	b.text = text
}

// MenuColorButton shows a color swatch with a clickable row.
type MenuColorButton struct {
	BaseMenuElement
	text     string
	swatch   color.RGBA
	options  ButtonOptions
	callback func()
	hovered  bool
	pressed  bool
	rect     image.Rectangle
}

func NewMenuColorButton(text string, swatch color.RGBA, options ButtonOptions, callback func()) *MenuColorButton {
	return &MenuColorButton{
		BaseMenuElement: NewBaseMenuElement(),
		text:            text,
		swatch:          swatch,
		options:         options,
		callback:        callback,
	}
}

func (b *MenuColorButton) SetColor(swatch color.RGBA) {
	b.swatch = swatch
}

func (b *MenuButton) Update(mx, my int, deltaTime float64) bool {
	if !b.visible || !b.options.Enabled {
		return false
	}

	b.updateAnimation(deltaTime)

	oldHovered := b.hovered
	b.hovered = mx >= b.rect.Min.X && mx < b.rect.Max.X && my >= b.rect.Min.Y && my < b.rect.Max.Y

	if mx != -1 && my != -1 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, b.rect) {
			b.pressed = true
			return true
		}

		if px, py, released := primaryJustReleased(); released {
			wasPressed := b.pressed
			b.pressed = false
			if wasPressed && pointInRect(px, py, b.rect) && b.callback != nil {
				b.callback()
			}
			if wasPressed {
				return true
			}
		}
	}

	return b.hovered != oldHovered
}

func (b *MenuColorButton) Update(mx, my int, deltaTime float64) bool {
	if !b.visible || !b.options.Enabled {
		return false
	}

	b.updateAnimation(deltaTime)

	oldHovered := b.hovered
	b.hovered = mx >= b.rect.Min.X && mx < b.rect.Max.X && my >= b.rect.Min.Y && my < b.rect.Max.Y

	if mx != -1 && my != -1 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, b.rect) {
			b.pressed = true
			return true
		}

		if px, py, released := primaryJustReleased(); released {
			wasPressed := b.pressed
			b.pressed = false
			if wasPressed && pointInRect(px, py, b.rect) && b.callback != nil {
				b.callback()
			}
			if wasPressed {
				return true
			}
		}
	}

	return b.hovered != oldHovered
}

func (b *MenuButton) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !b.visible || b.displayAlpha() <= 0.01 {
		return 0
	}

	height := b.options.Height
	alpha := float32(b.displayAlpha())

	// Store rect for click detection
	b.rect = image.Rect(x, y, x+width, y+height)

	// Choose color based on state
	bgColor := b.options.BackgroundColor
	if !b.options.Enabled {
		bgColor = color.RGBA{40, 40, 60, 255}
	} else if b.pressed {
		bgColor = b.options.PressedColor
	} else if b.hovered {
		bgColor = b.options.HoverColor
	}

	// Apply animation alpha
	bgColor.A = uint8(float32(bgColor.A) * alpha)

	// Draw background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), bgColor, false)

	// Draw border
	borderColor := b.options.BorderColor
	borderColor.A = uint8(float32(borderColor.A) * alpha)
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), b.options.BorderWidth, borderColor, false)

	// Draw text
	textColor := b.options.TextColor
	if !b.options.Enabled {
		textColor = color.RGBA{150, 150, 150, 255}
	}
	textColor.A = uint8(float32(textColor.A) * alpha)

	textWidth := text.BoundString(font, b.text).Dx()
	textX := x + (width-textWidth)/2
	textY := y + height/2 + 6 // Approximate vertical center
	text.Draw(screen, b.text, font, textX, textY, textColor)

	return height
}

func (b *MenuColorButton) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !b.visible || b.displayAlpha() <= 0.01 {
		return 0
	}

	height := b.options.Height
	alpha := float32(b.displayAlpha())

	// Store rect for click detection
	b.rect = image.Rect(x, y, x+width, y+height)

	bgColor := b.options.BackgroundColor
	if !b.options.Enabled {
		bgColor = color.RGBA{40, 40, 60, 255}
	} else if b.pressed {
		bgColor = b.options.PressedColor
	} else if b.hovered {
		bgColor = b.options.HoverColor
	}
	bgColor.A = uint8(float32(bgColor.A) * alpha)

	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), bgColor, false)
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), b.options.BorderWidth, b.options.BorderColor, false)

	textColor := b.options.TextColor
	textColor.A = uint8(float32(textColor.A) * alpha)
	textX := x + 15
	textY := y + height/2 + b.options.FontSize/2 - 4
	text.Draw(screen, b.text, font, textX, textY, textColor)

	swatchSize := height - 10
	swatchX := x + width - swatchSize - 10
	swatchY := y + (height-swatchSize)/2
	swatchColor := b.swatch
	swatchColor.A = uint8(float32(swatchColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(swatchX), float32(swatchY), float32(swatchSize), float32(swatchSize), swatchColor, false)
	vector.StrokeRect(screen, float32(swatchX), float32(swatchY), float32(swatchSize), float32(swatchSize), 2, color.RGBA{255, 255, 255, 180}, false)

	return height
}

func (b *MenuColorButton) GetMinHeight() int {
	return b.options.Height
}

func (b *MenuColorButton) IsVisible() bool { return b.visible }

func (b *MenuColorButton) SetVisible(visible bool) { b.BaseMenuElement.SetVisible(visible) }

func (b *MenuButton) GetMinHeight() int {
	return b.options.Height
}

// TextOptions configures text appearance
type TextOptions struct {
	Color    color.RGBA
	FontSize int
	Bold     bool
	Italic   bool
	Centered bool
	Height   int
}

func DefaultTextOptions() TextOptions {
	return TextOptions{
		Color:    color.RGBA{255, 255, 255, 255},
		FontSize: 16,
		Bold:     false,
		Italic:   false,
		Centered: false,
		Height:   25,
	}
}

// MenuText represents a text display element
type MenuText struct {
	BaseMenuElement
	text    string
	options TextOptions
}

func NewMenuText(text string, options TextOptions) *MenuText {
	return &MenuText{
		BaseMenuElement: NewBaseMenuElement(),
		text:            text,
		options:         options,
	}
}

func (t *MenuText) Update(mx, my int, deltaTime float64) bool {
	t.updateAnimation(deltaTime)
	return false
}

func (t *MenuText) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !t.visible || t.displayAlpha() <= 0.01 {
		return 0
	}

	height := t.options.Height
	alpha := float32(t.displayAlpha())

	textColor := t.options.Color
	textColor.A = uint8(float32(textColor.A) * alpha)

	textX := x
	if t.options.Centered {
		textWidth := text.BoundString(font, t.text).Dx()
		textX = x + (width-textWidth)/2
	}

	textY := y + height/2 + 6 // Approximate vertical center
	text.Draw(screen, t.text, font, textX, textY, textColor)

	return height
}

func (t *MenuText) GetMinHeight() int {
	return t.options.Height
}

// SetText updates the text content without rebuilding the element
func (t *MenuText) SetText(newText string) {
	t.text = newText
}

// GetText returns the current text content
func (t *MenuText) GetText() string {
	return t.text
}

// MenuClickableText represents a clickable text display element
type MenuClickableText struct {
	BaseMenuElement
	text     string
	options  TextOptions
	callback func()
	hovered  bool
	rect     image.Rectangle
}

func NewMenuClickableText(text string, options TextOptions, callback func()) *MenuClickableText {
	return &MenuClickableText{
		BaseMenuElement: NewBaseMenuElement(),
		text:            text,
		options:         options,
		callback:        callback,
	}
}

func (t *MenuClickableText) Update(mx, my int, deltaTime float64) bool {
	t.updateAnimation(deltaTime)

	// Check if mouse is over the text
	t.hovered = mx >= t.rect.Min.X && mx < t.rect.Max.X && my >= t.rect.Min.Y && my < t.rect.Max.Y

	if mx != -1 && my != -1 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, t.rect) {
			if t.callback != nil {
				t.callback()
			}
			return true
		}
	}

	return false
}

func (t *MenuClickableText) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !t.visible || t.displayAlpha() <= 0.01 {
		return 0
	}

	height := t.options.Height
	alpha := float32(t.displayAlpha())

	// Store rect for click detection
	t.rect = image.Rect(x, y, x+width, y+height)

	textColor := t.options.Color

	// Draw hover background box if hovered (like collapsible menu headers)
	if t.hovered {
		hoverColor := color.RGBA{80, 80, 120, uint8(float32(180) * alpha)} // Similar to collapsible menu header
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), hoverColor, false)
	}

	textColor.A = uint8(float32(textColor.A) * alpha)

	textX := x
	if t.options.Centered {
		textWidth := text.BoundString(font, t.text).Dx()
		textX = x + (width-textWidth)/2
	}

	textY := y + height/2 + 6 // Approximate vertical center
	text.Draw(screen, t.text, font, textX, textY, textColor)

	return height
}

func (t *MenuClickableText) GetMinHeight() int {
	return t.options.Height
}

// CheckboxOptions configures checkbox appearance and behavior
type CheckboxOptions struct {
	BoxSize       int
	Height        int
	LabelColor    color.RGBA
	BoxColor      color.RGBA
	CheckColor    color.RGBA
	BorderColor   color.RGBA
	DisabledColor color.RGBA
	Enabled       bool
}

func DefaultCheckboxOptions() CheckboxOptions {
	return CheckboxOptions{
		BoxSize:       18,
		Height:        28,
		LabelColor:    color.RGBA{255, 255, 255, 255},
		BoxColor:      color.RGBA{70, 70, 80, 255},
		CheckColor:    color.RGBA{120, 200, 120, 255},
		BorderColor:   color.RGBA{120, 120, 140, 255},
		DisabledColor: color.RGBA{90, 90, 100, 180},
		Enabled:       true,
	}
}

// MenuCheckbox represents a checkbox with label
type MenuCheckbox struct {
	BaseMenuElement
	label    string
	checked  bool
	options  CheckboxOptions
	callback func(bool)
	hovered  bool
	rect     image.Rectangle
}

func NewMenuCheckbox(label string, checked bool, options CheckboxOptions, callback func(bool)) *MenuCheckbox {
	if options.BoxSize == 0 {
		options.BoxSize = 18
	}
	if options.Height == 0 {
		options.Height = 28
	}
	return &MenuCheckbox{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		checked:         checked,
		options:         options,
		callback:        callback,
	}
}

func (c *MenuCheckbox) Update(mx, my int, deltaTime float64) bool {
	if !c.visible {
		return false
	}

	c.updateAnimation(deltaTime)
	c.hovered = pointInRect(mx, my, c.rect)

	if !c.options.Enabled {
		return false
	}

	if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, c.rect) {
		c.checked = !c.checked
		if c.callback != nil {
			c.callback(c.checked)
		}
		return true
	}

	return false
}

func (c *MenuCheckbox) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !c.visible || c.displayAlpha() <= 0.01 {
		return 0
	}

	height := c.options.Height
	alpha := float32(c.displayAlpha())

	c.rect = image.Rect(x, y, x+width, y+height)

	boxSize := c.options.BoxSize
	boxX := x
	boxY := y + (height-boxSize)/2

	boxColor := c.options.BoxColor
	borderColor := c.options.BorderColor
	checkColor := c.options.CheckColor

	if !c.options.Enabled {
		boxColor = c.options.DisabledColor
		borderColor = c.options.DisabledColor
		checkColor = c.options.DisabledColor
	} else if c.hovered {
		// Slightly brighten on hover
		boxColor = color.RGBA{uint8(math.Min(255, float64(boxColor.R)+10)), uint8(math.Min(255, float64(boxColor.G)+10)), uint8(math.Min(255, float64(boxColor.B)+10)), boxColor.A}
	}

	boxColor.A = uint8(float32(boxColor.A) * alpha)
	borderColor.A = uint8(float32(borderColor.A) * alpha)
	checkColor.A = uint8(float32(checkColor.A) * alpha)

	vector.DrawFilledRect(screen, float32(boxX), float32(boxY), float32(boxSize), float32(boxSize), boxColor, false)
	vector.StrokeRect(screen, float32(boxX), float32(boxY), float32(boxSize), float32(boxSize), 1, borderColor, false)

	if c.checked {
		margin := float32(3)
		vector.DrawFilledRect(screen, float32(boxX)+margin, float32(boxY)+margin, float32(boxSize)-margin*2, float32(boxSize)-margin*2, checkColor, false)
	}

	if font != nil && c.label != "" {
		labelColor := c.options.LabelColor
		if !c.options.Enabled {
			labelColor = c.options.DisabledColor
		}
		labelColor.A = uint8(float32(labelColor.A) * alpha)
		text.Draw(screen, c.label, font, boxX+boxSize+8, y+height/2+6, labelColor)
	}

	return height
}

func (c *MenuCheckbox) GetMinHeight() int {
	return c.options.Height
}

func (c *MenuCheckbox) SetChecked(value bool) {
	c.checked = value
}

func (c *MenuCheckbox) IsChecked() bool {
	return c.checked
}

// SliderOptions configures slider appearance and behavior
type SliderOptions struct {
	MinValue        float64
	MaxValue        float64
	Step            float64
	BackgroundColor color.RGBA
	FillColor       color.RGBA
	ExclusionFill   color.RGBA
	HandleColor     color.RGBA
	TextColor       color.RGBA
	Height          int
	FontSize        int
	ShowValue       bool
	ValueFormat     string // e.g., "%.1f", "%.0f%%"
	Enabled         bool   // Whether the slider is interactive
}

func DefaultSliderOptions() SliderOptions {
	return SliderOptions{
		MinValue:        0.0,
		MaxValue:        100.0,
		Step:            1.0,
		BackgroundColor: color.RGBA{40, 40, 50, 255},
		FillColor:       color.RGBA{100, 150, 255, 255},
		ExclusionFill:   color.RGBA{200, 110, 110, 255},
		HandleColor:     color.RGBA{255, 255, 255, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		Height:          35,
		FontSize:        16,
		ShowValue:       true,
		ValueFormat:     "%.0f",
		Enabled:         true,
	}
}

// RangeSliderOptions configures dual-handle slider appearance and behavior
type RangeSliderOptions struct {
	MinValue        float64
	MaxValue        float64
	Step            float64
	BackgroundColor color.RGBA
	FillColor       color.RGBA
	ExclusionFill   color.RGBA
	HandleColor     color.RGBA
	TextColor       color.RGBA
	Height          int
	FontSize        int
	ShowValue       bool
	ValueFormat     string
	ValueFormatter  func(low, high float64) string
	Enabled         bool
	CompactLabel    bool
}

func DefaultRangeSliderOptions() RangeSliderOptions {
	return RangeSliderOptions{
		MinValue:        0,
		MaxValue:        100,
		Step:            1,
		BackgroundColor: color.RGBA{40, 40, 50, 255},
		FillColor:       color.RGBA{100, 150, 255, 255},
		ExclusionFill:   color.RGBA{200, 110, 110, 255},
		HandleColor:     color.RGBA{255, 255, 255, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		Height:          40,
		FontSize:        14,
		ShowValue:       true,
		ValueFormat:     "%.0f",
		Enabled:         true,
	}
}

const (
	sliderValueAnimationDuration = 0.18
	sliderDragAnimationDuration  = 0.08
	sliderSnapOvershoot          = 0.85 // Controls how much the handle briefly overshoots when snapping
)

// MenuSlider represents a draggable slider element
type MenuSlider struct {
	BaseMenuElement
	label             string
	value             float64
	displayValue      float64
	options           SliderOptions
	callback          func(value float64)
	onDragEnd         func() // Called when dragging ends
	dragging          bool
	sliderRect        image.Rectangle
	animatingValue    bool
	valueAnimStart    float64
	valueAnimTarget   float64
	valueAnimElapsed  float64
	valueAnimDuration float64
	valueAnimSnap     bool
}

func NewMenuSlider(label string, initialValue float64, options SliderOptions, callback func(float64)) *MenuSlider {
	return &MenuSlider{
		BaseMenuElement:   NewBaseMenuElement(),
		label:             label,
		value:             initialValue,
		displayValue:      initialValue,
		options:           options,
		callback:          callback,
		valueAnimStart:    initialValue,
		valueAnimTarget:   initialValue,
		valueAnimDuration: sliderValueAnimationDuration,
	}
}

func (s *MenuSlider) Update(mx, my int, deltaTime float64) bool {
	if !s.visible {
		return false
	}

	s.updateAnimation(deltaTime)
	s.updateValueAnimation(deltaTime)

	if !s.options.Enabled {
		return false
	}

	if mx != -1 && my != -1 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, s.sliderRect) {
			s.dragging = true
			s.updateValueFromMouse(px, true, false)
			return true
		}
	}

	if s.dragging {
		if primaryPressed() {
			px, _ := primaryPointerPosition()
			s.updateValueFromMouse(px, true, true)
			return true
		}
		s.dragging = false
		// Snap gently when releasing the drag to emphasize the settled value
		s.startValueAnimation(s.value, sliderValueAnimationDuration, true)
		if s.onDragEnd != nil {
			s.onDragEnd()
		}
	}

	return false
}

func (s *MenuSlider) updateValueFromMouse(mx int, animate bool, isDrag bool) {
	sliderWidth := s.sliderRect.Dx()
	relativeX := mx - s.sliderRect.Min.X
	ratio := float64(relativeX) / float64(sliderWidth)
	ratio = math.Max(0, math.Min(1, ratio))

	newValue := s.options.MinValue + ratio*(s.options.MaxValue-s.options.MinValue)

	// Apply step
	if s.options.Step > 0 {
		newValue = math.Round(newValue/s.options.Step) * s.options.Step
	}

	if newValue != s.value {
		s.value = newValue
		// For active drags, keep the handle glued to the pointer to avoid “friction”.
		animateValue := animate && !isDrag
		if animateValue {
			duration := sliderValueAnimationDuration
			snap := true
			s.startValueAnimation(newValue, duration, snap)
		} else {
			s.animatingValue = false
			s.displayValue = newValue
			s.valueAnimElapsed = 0
		}
		if s.callback != nil {
			s.callback(s.value)
		}
	}
}

func (s *MenuSlider) startValueAnimation(target float64, duration float64, snap bool) {
	clampedTarget := math.Max(s.options.MinValue, math.Min(s.options.MaxValue, target))
	startValue := s.displayValue
	// If not currently animating, start from the stored value
	if !s.animatingValue {
		startValue = s.displayValue
	}
	if math.Abs(clampedTarget-startValue) < 1e-6 {
		s.animatingValue = false
		s.displayValue = clampedTarget
		return
	}

	s.valueAnimStart = startValue
	s.valueAnimTarget = clampedTarget
	s.valueAnimElapsed = 0
	s.valueAnimDuration = duration
	if s.valueAnimDuration <= 0 {
		s.valueAnimDuration = sliderValueAnimationDuration
	}
	s.valueAnimSnap = snap
	s.animatingValue = true
}

func (s *MenuSlider) updateValueAnimation(deltaTime float64) {
	if !s.animatingValue {
		s.displayValue = s.value
		return
	}

	s.valueAnimElapsed += deltaTime
	progress := s.valueAnimElapsed / s.valueAnimDuration
	if progress >= 1.0 {
		s.animatingValue = false
		s.displayValue = s.value
		return
	}

	eased := easeInOut(progress)
	if s.valueAnimSnap {
		eased = easeOutBack(progress, sliderSnapOvershoot)
	}
	s.displayValue = s.valueAnimStart + (s.valueAnimTarget-s.valueAnimStart)*eased
}

func easeInOut(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	t = (t - 1)
	return 1 + 4*t*t*t
}

func easeOutBack(t float64, s float64) float64 {
	// Standard back easing for a light snap toward the target
	t -= 1
	return t*t*((s+1)*t+s) + 1
}

func (s *MenuSlider) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !s.visible || s.displayAlpha() <= 0.01 {
		return 0
	}

	height := s.options.Height
	alpha := float32(s.displayAlpha())

	// Draw label
	labelY := y + 6
	textColor := s.options.TextColor
	textColor.A = uint8(float32(textColor.A) * alpha)
	text.Draw(screen, s.label, font, x, labelY, textColor)

	// Slider area - positioned below the label
	sliderY := y + 24 // Moved down to be below the label
	sliderHeight := 20
	sliderX := x + 5          // Reduced from 20 to 5 for better alignment with label
	sliderWidth := width - 10 // Adjusted to match the reduced left margin

	s.sliderRect = image.Rect(sliderX, sliderY-sliderHeight/2, sliderX+sliderWidth, sliderY+sliderHeight/2)

	// Draw background
	bgColor := s.options.BackgroundColor
	if !s.options.Enabled {
		// Gray out the background when disabled
		bgColor = color.RGBA{60, 60, 60, 255}
	}
	bgColor.A = uint8(float32(bgColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(sliderX), float32(sliderY-sliderHeight/2), float32(sliderWidth), float32(sliderHeight), bgColor, false)

	// Draw fill
	fillRatio := (s.displayValue - s.options.MinValue) / (s.options.MaxValue - s.options.MinValue)
	fillWidth := float32(sliderWidth) * float32(fillRatio)
	fillColor := s.options.FillColor
	if !s.options.Enabled {
		// Gray out the fill when disabled
		fillColor = color.RGBA{80, 80, 80, 255}
	}
	fillColor.A = uint8(float32(fillColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(sliderX), float32(sliderY-sliderHeight/2), fillWidth, float32(sliderHeight), fillColor, false)

	// Draw handle
	handleX := float32(sliderX) + fillWidth - 5
	handleColor := s.options.HandleColor
	if !s.options.Enabled {
		// Gray out the handle when disabled
		handleColor = color.RGBA{100, 100, 100, 255}
	}
	handleColor.A = uint8(float32(handleColor.A) * alpha)
	vector.DrawFilledRect(screen, handleX, float32(sliderY-sliderHeight/2), 10, float32(sliderHeight), handleColor, false)

	// Draw value if enabled
	if s.options.ShowValue {
		valueText := fmt.Sprintf(s.options.ValueFormat, s.displayValue)
		valueWidth := text.BoundString(font, valueText).Dx()
		valueX := x + width - valueWidth
		text.Draw(screen, valueText, font, valueX, labelY, textColor)
	}

	return height
}

func (s *MenuSlider) GetMinHeight() int {
	return s.options.Height
}

func (s *MenuSlider) SetValue(value float64) {
	s.value = math.Max(s.options.MinValue, math.Min(s.options.MaxValue, value))
	s.startValueAnimation(s.value, sliderValueAnimationDuration, true)
}

func (s *MenuSlider) SetOnDragEnd(callback func()) {
	s.onDragEnd = callback
}

func (s *MenuSlider) GetValue() float64 {
	return s.value
}

func (s *MenuSlider) SetFillColor(color color.RGBA) {
	s.options.FillColor = color
}

// MenuRangeSlider represents a dual-handle slider for selecting ranges.
type MenuRangeSlider struct {
	BaseMenuElement
	label         string
	lowValue      float64
	highValue     float64
	displayLow    float64
	displayHigh   float64
	options       RangeSliderOptions
	callback      func(low, high float64)
	onDragEnd     func()
	sliderRect    image.Rectangle
	dragging      bool
	dragHigh      bool
	dragSecondary bool
	baseFill      color.RGBA
	startLow      float64
	startHigh     float64
	startValue    float64
	exclusion     bool
}

func NewMenuRangeSlider(label string, low, high float64, options RangeSliderOptions, callback func(float64, float64)) *MenuRangeSlider {
	s := &MenuRangeSlider{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		options:         options,
		callback:        callback,
		baseFill:        options.FillColor,
	}
	s.SetValues(low, high)
	return s
}

func (s *MenuRangeSlider) Update(mx, my int, deltaTime float64) bool {
	if !s.visible {
		return false
	}

	s.updateAnimation(deltaTime)

	if !s.options.Enabled {
		return false
	}

	leftDown := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	rightDown := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	leftHeld := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rightHeld := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	leftUp := inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
	rightUp := inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight)

	clickInside := mx != -1 && my != -1 && pointInRect(mx, my, s.sliderRect)
	if clickInside && (leftDown || rightDown) {
		clickedValue := s.valueFromPosition(mx)
		useSecondary := rightDown
		if rightDown {
			// Right drag moves the whole block; capture starting span.
			s.startLow = s.lowValue
			s.startHigh = s.highValue
			s.startValue = clickedValue
			s.dragHigh = false
		} else {
			s.dragHigh = s.shouldDragHigh(mx)
		}
		s.dragSecondary = useSecondary
		s.dragging = true
		s.updateHandleFromPosition(mx)
		return true
	}

	if s.dragging {
		stillHeld := (s.dragSecondary && rightHeld) || (!s.dragSecondary && leftHeld)
		if stillHeld {
			if mx != -1 {
				s.updateHandleFromPosition(mx)
				return true
			}
		} else if leftUp || rightUp {
			s.dragging = false
			if s.onDragEnd != nil {
				s.onDragEnd()
			}
			return true
		}
	}

	return false
}

func (s *MenuRangeSlider) updateHandleFromPosition(mx int) {
	val := s.valueFromPosition(mx)
	if s.dragSecondary {
		width := s.startHigh - s.startLow
		delta := val - s.startValue
		newLow := s.startLow + delta
		newHigh := newLow + width
		if newLow < s.options.MinValue {
			newLow = s.options.MinValue
			newHigh = newLow + width
		}
		if newHigh > s.options.MaxValue {
			newHigh = s.options.MaxValue
			newLow = newHigh - width
		}
		s.lowValue, s.highValue = s.clampValues(newLow, newHigh)
	} else if s.dragHigh {
		newHigh := val
		if newHigh < s.lowValue {
			newHigh = s.lowValue
		}
		if newHigh > s.options.MaxValue {
			newHigh = s.options.MaxValue
		}
		s.highValue = newHigh
	} else {
		newLow := val
		if newLow > s.highValue {
			newLow = s.highValue
		}
		if newLow < s.options.MinValue {
			newLow = s.options.MinValue
		}
		s.lowValue = newLow
	}
	s.displayLow, s.displayHigh = s.lowValue, s.highValue
	s.invokeCallback()
}

func (s *MenuRangeSlider) shouldDragHigh(mx int) bool {
	if s.highValue == s.lowValue {
		// When collapsed, choose the closest handle by position.
		lowX := s.positionForValue(s.lowValue)
		highX := s.positionForValue(s.highValue)
		if math.Abs(float64(mx-lowX)) >= math.Abs(float64(mx-highX)) {
			return true
		}
		return false
	}
	lowX := s.positionForValue(s.lowValue)
	highX := s.positionForValue(s.highValue)
	return math.Abs(float64(mx-highX)) <= math.Abs(float64(mx-lowX))
}

func (s *MenuRangeSlider) valueFromPosition(mx int) float64 {
	sliderWidth := s.sliderRect.Dx()
	if sliderWidth == 0 {
		return s.lowValue
	}
	relativeX := mx - s.sliderRect.Min.X
	ratio := float64(relativeX) / float64(sliderWidth)
	ratio = math.Max(0, math.Min(1, ratio))
	val := s.options.MinValue + ratio*(s.options.MaxValue-s.options.MinValue)
	if s.options.Step > 0 {
		val = math.Round(val/s.options.Step) * s.options.Step
	}
	return val
}

func (s *MenuRangeSlider) clampValues(low, high float64) (float64, float64) {
	min := s.options.MinValue
	max := s.options.MaxValue
	if low < min {
		low = min
	}
	if high > max {
		high = max
	}
	if low > high {
		low = high
	}
	return low, high
}

func (s *MenuRangeSlider) invokeCallback() {
	if s.callback != nil {
		s.callback(s.lowValue, s.highValue)
	}
}

func (s *MenuRangeSlider) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !s.visible || s.displayAlpha() <= 0.01 {
		return 0
	}

	height := s.options.Height
	alpha := float32(s.displayAlpha())
	labelY := y + 8

	textColor := s.options.TextColor
	textColor.A = uint8(float32(textColor.A) * alpha)
	text.Draw(screen, s.label, font, x, labelY, textColor)

	valueText := ""
	if s.options.ShowValue {
		if s.options.ValueFormatter != nil {
			valueText = s.options.ValueFormatter(s.displayLow, s.displayHigh)
		} else {
			valueText = fmt.Sprintf(s.options.ValueFormat+" – "+s.options.ValueFormat, s.displayLow, s.displayHigh)
		}
		valueBounds := text.BoundString(font, valueText)
		text.Draw(screen, valueText, font, x+width-valueBounds.Dx(), labelY, textColor)
	}

	sliderY := y + 26
	sliderHeight := 14
	sliderX := x + 5
	sliderWidth := width - 10
	if s.options.CompactLabel {
		sliderY = y + 20
		height = s.options.Height - 6
	}

	s.sliderRect = image.Rect(sliderX, sliderY-sliderHeight/2, sliderX+sliderWidth, sliderY+sliderHeight/2)

	bg := s.options.BackgroundColor
	if !s.options.Enabled {
		bg = color.RGBA{60, 60, 60, 255}
	}
	bg.A = uint8(float32(bg.A) * alpha)
	vector.DrawFilledRect(screen, float32(s.sliderRect.Min.X), float32(s.sliderRect.Min.Y), float32(s.sliderRect.Dx()), float32(s.sliderRect.Dy()), bg, false)

	rangeStart := s.positionForValue(s.displayLow)
	rangeEnd := s.positionForValue(s.displayHigh)
	fillColor := s.baseFill
	if !s.options.Enabled {
		fillColor = color.RGBA{80, 80, 80, 255}
	}
	fillColor.A = uint8(float32(fillColor.A) * alpha)

	if s.exclusion {
		// Shade outer regions instead of the selected band.
		leftWidth := rangeStart - s.sliderRect.Min.X
		rightWidth := s.sliderRect.Max.X - rangeEnd
		if leftWidth > 0 {
			vector.DrawFilledRect(screen, float32(s.sliderRect.Min.X), float32(s.sliderRect.Min.Y), float32(leftWidth), float32(s.sliderRect.Dy()), fillColor, false)
		}
		if rightWidth > 0 {
			vector.DrawFilledRect(screen, float32(rangeEnd), float32(s.sliderRect.Min.Y), float32(rightWidth), float32(s.sliderRect.Dy()), fillColor, false)
		}
	} else {
		fillWidth := rangeEnd - rangeStart
		vector.DrawFilledRect(screen, float32(rangeStart), float32(s.sliderRect.Min.Y), float32(fillWidth), float32(s.sliderRect.Dy()), fillColor, false)
	}

	handleColor := s.options.HandleColor
	if !s.options.Enabled {
		handleColor = color.RGBA{100, 100, 100, 255}
	}
	handleColor.A = uint8(float32(handleColor.A) * alpha)

	vector.DrawFilledRect(screen, float32(rangeStart-5), float32(s.sliderRect.Min.Y), 10, float32(s.sliderRect.Dy()), handleColor, false)
	vector.DrawFilledRect(screen, float32(rangeEnd-5), float32(s.sliderRect.Min.Y), 10, float32(s.sliderRect.Dy()), handleColor, false)

	return height
}

func (s *MenuRangeSlider) positionForValue(value float64) int {
	rangeWidth := s.sliderRect.Dx()
	if rangeWidth <= 0 {
		return s.sliderRect.Min.X
	}
	clamped := math.Max(s.options.MinValue, math.Min(s.options.MaxValue, value))
	ratio := (clamped - s.options.MinValue) / (s.options.MaxValue - s.options.MinValue)
	return s.sliderRect.Min.X + int(float64(rangeWidth)*ratio)
}

func (s *MenuRangeSlider) GetMinHeight() int {
	return s.options.Height
}

func (s *MenuRangeSlider) SetValues(low, high float64) {
	s.lowValue, s.highValue = s.clampValues(low, high)
	s.displayLow, s.displayHigh = s.lowValue, s.highValue
}

func (s *MenuRangeSlider) SetFillColor(fill color.RGBA) {
	s.options.FillColor = fill
	s.baseFill = fill
}

func (s *MenuRangeSlider) SetExclusion(active bool) {
	s.exclusion = active
}

func (s *MenuRangeSlider) SetOnDragEnd(callback func()) {
	s.onDragEnd = callback
}

func (s *MenuRangeSlider) GetValues() (float64, float64) {
	return s.lowValue, s.highValue
}

func (s *MenuRangeSlider) SetEnabled(enabled bool) {
	s.options.Enabled = enabled
	if !enabled {
		s.dragging = false
	}
}

// CollapsibleMenuOptions configures collapsible section appearance
type CollapsibleMenuOptions struct {
	HeaderColor     color.RGBA
	BackgroundColor color.RGBA
	BorderColor     color.RGBA
	TextColor       color.RGBA
	HeaderHeight    int
	FontSize        int
	Collapsed       bool
}

func DefaultCollapsibleMenuOptions() CollapsibleMenuOptions {
	return CollapsibleMenuOptions{
		HeaderColor:     color.RGBA{60, 60, 80, 255},
		BackgroundColor: color.RGBA{40, 40, 55, 255},
		BorderColor:     color.RGBA{80, 80, 100, 255},
		TextColor:       color.RGBA{255, 255, 255, 255},
		HeaderHeight:    30,
		FontSize:        18,
		Collapsed:       false,
	}
}

// CollapsibleMenu represents a collapsible section containing other elements
type CollapsibleMenu struct {
	BaseMenuElement
	title         string
	options       CollapsibleMenuOptions
	elements      []EdgeMenuElement
	collapsed     bool
	headerRect    image.Rectangle
	hovered       bool
	parentMenu    *EdgeMenu
	lastX, lastY  int     // Store last known position from Draw call
	lastWidth     int     // Store last known width from Draw call
	lastClickTime float64 // Track when last click was processed to prevent double-clicks
	revealStart   map[EdgeMenuElement]time.Time
}

func NewCollapsibleMenu(title string, options CollapsibleMenuOptions) *CollapsibleMenu {
	return &CollapsibleMenu{
		BaseMenuElement: NewBaseMenuElement(),
		title:           title,
		options:         options,
		collapsed:       options.Collapsed,
		elements:        make([]EdgeMenuElement, 0),
		revealStart:     make(map[EdgeMenuElement]time.Time),
	}
}

func (c *CollapsibleMenu) AddElement(element EdgeMenuElement) *CollapsibleMenu {
	c.elements = append(c.elements, element)
	return c
}

func (c *CollapsibleMenu) Button(text string, options ButtonOptions, callback func()) *CollapsibleMenu {
	button := NewMenuButton(text, options, callback)
	c.elements = append(c.elements, button)
	return c
}

func (c *CollapsibleMenu) ColorButton(text string, swatch color.RGBA, options ButtonOptions, callback func()) *CollapsibleMenu {
	button := NewMenuColorButton(text, swatch, options, callback)
	c.elements = append(c.elements, button)
	return c
}

func (c *CollapsibleMenu) Text(text string, options TextOptions) *CollapsibleMenu {
	textElement := NewMenuText(text, options)
	c.elements = append(c.elements, textElement)
	return c
}

func (c *CollapsibleMenu) Checkbox(label string, checked bool, options CheckboxOptions, callback func(bool)) *CollapsibleMenu {
	checkbox := NewMenuCheckbox(label, checked, options, callback)
	c.elements = append(c.elements, checkbox)
	return c
}

func (c *CollapsibleMenu) ClickableText(text string, options TextOptions, callback func()) *CollapsibleMenu {
	clickableText := NewMenuClickableText(text, options, callback)
	c.elements = append(c.elements, clickableText)
	return c
}

func (c *CollapsibleMenu) Slider(label string, initialValue float64, options SliderOptions, callback func(float64)) *CollapsibleMenu {
	slider := NewMenuSlider(label, initialValue, options, callback)
	c.elements = append(c.elements, slider)
	return c
}

func (c *CollapsibleMenu) RangeSlider(label string, low, high float64, options RangeSliderOptions, callback func(float64, float64)) *MenuRangeSlider {
	slider := NewMenuRangeSlider(label, low, high, options, callback)
	c.elements = append(c.elements, slider)
	return slider
}

func (c *CollapsibleMenu) Card() *Card {
	card := NewCard()
	c.elements = append(c.elements, card)
	return card
}

func (c *CollapsibleMenu) UpgradeControl(label, upgradeType, territoryName string, currentLevel int) *CollapsibleMenu {
	upgradeControl := NewUpgradeControl(label, upgradeType, territoryName, currentLevel)
	// Set the parent menu reference for targeted updates
	if c.parentMenu != nil {
		upgradeControl.parentMenu = c.parentMenu
	}
	c.elements = append(c.elements, upgradeControl)
	return c
}

func (c *CollapsibleMenu) BonusControl(label, bonusType, territoryName string, currentLevel int, enabled ...bool) *CollapsibleMenu {
	isEnabled := true
	if len(enabled) > 0 {
		isEnabled = enabled[0]
	}
	bonusControl := NewBonusControl(label, bonusType, territoryName, currentLevel, isEnabled)
	// Set the parent menu reference for targeted updates
	if c.parentMenu != nil {
		bonusControl.parentMenu = c.parentMenu
		if c.parentMenu.refreshCallback != nil {
			bonusControl.SetRefreshCallback(c.parentMenu.refreshCallback)
		}
	}
	c.elements = append(c.elements, bonusControl)
	return c
}

func (c *CollapsibleMenu) ResourceStorageControl(resourceName, resourceType, territoryName string, currentValue, maxValue, transitValue, generationPerHour int, resourceColor color.RGBA) *CollapsibleMenu {
	storageControl := NewResourceStorageControl(resourceName, resourceType, territoryName, currentValue, maxValue, transitValue, generationPerHour, resourceColor)
	storageControl.parentMenu = c.parentMenu // Set parent menu for focus management
	c.elements = append(c.elements, storageControl)
	return c
}

func (c *CollapsibleMenu) CollapsibleMenu(title string, options CollapsibleMenuOptions) *CollapsibleMenu {
	nestedMenu := NewCollapsibleMenu(title, options)
	// nestedMenu.setParentMenu(c.parentMenu) // Set parent menu for focus management
	c.elements = append(c.elements, nestedMenu)
	return nestedMenu
}

func (c *CollapsibleMenu) Update(mx, my int, deltaTime float64) bool {
	if !c.visible {
		return false
	}

	c.updateAnimation(deltaTime)
	c.lastClickTime += deltaTime // Increment the click timer

	// Calculate header rectangle using last known position from Draw
	headerHeight := c.options.HeaderHeight
	c.headerRect = image.Rect(c.lastX, c.lastY, c.lastX+c.lastWidth, c.lastY+headerHeight)

	// Check if mouse is over header
	c.hovered = mx >= c.headerRect.Min.X && mx < c.headerRect.Max.X && my >= c.headerRect.Min.Y && my < c.headerRect.Max.Y

	// Check header click for collapse/expand with debounce protection
	if c.lastClickTime > 0.1 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, c.headerRect) {
			c.collapsed = !c.collapsed
			c.lastClickTime = 0.0 // Reset timer to prevent immediate re-triggering

			// If this is a trading route submenu, save its state
			if c.parentMenu != nil && strings.HasPrefix(c.title, "Route to ") {
				c.parentMenu.tradingRouteStates[c.title] = !c.collapsed
			}

			return true
		}
	}

	// Update child elements if not collapsed
	if !c.collapsed {
		for _, element := range c.elements {
			if element.Update(mx, my, deltaTime) {
				return true
			}
		}
	}

	return false
}

func (c *CollapsibleMenu) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !c.visible || c.displayAlpha() <= 0.01 {
		return 0
	}

	// Store position and width for use in Update method
	c.lastX = x
	c.lastY = y
	c.lastWidth = width

	alpha := float32(c.displayAlpha())
	currentY := y

	// Draw header
	headerHeight := c.options.HeaderHeight
	c.headerRect = image.Rect(x, currentY, x+width, currentY+headerHeight)

	// Header background
	headerColor := c.options.HeaderColor
	if c.hovered {
		// Brighten the color when hovered
		headerColor.R = uint8(math.Min(255, float64(headerColor.R)+20))
		headerColor.G = uint8(math.Min(255, float64(headerColor.G)+20))
		headerColor.B = uint8(math.Min(255, float64(headerColor.B)+20))
	}
	headerColor.A = uint8(float32(headerColor.A) * alpha)
	vector.DrawFilledRect(screen, float32(x), float32(currentY), float32(width), float32(headerHeight), headerColor, false)

	// Collapse/expand icon
	icon := "+"
	if !c.collapsed {
		icon = "−"
	}
	iconColor := c.options.TextColor
	iconColor.A = uint8(float32(iconColor.A) * alpha)
	text.Draw(screen, icon, font, x+10, currentY+headerHeight/2+6, iconColor)

	// Title
	text.Draw(screen, c.title, font, x+30, currentY+headerHeight/2+6, iconColor)

	currentY += headerHeight

	// Draw elements if not collapsed
	if !c.collapsed {
		if !eruntime.GetRuntimeOptions().SidemenuAnimations {
			c.revealStart = make(map[EdgeMenuElement]time.Time)
			for _, element := range c.elements {
				if element.IsVisible() {
					if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(1)
					}
					elementHeight := element.Draw(screen, x+10, currentY+5, width-20, font)
					currentY += elementHeight + 5
				}
			}
			currentY += 10 // Extra spacing at bottom
			return currentY - y
		}

		now := time.Now()
		revealDuration := 0.45
		revealDelayStep := 0.035
		revealOffset := 6.0
		easeOutCubic := func(t float64) float64 {
			if t <= 0 {
				return 0
			}
			if t >= 1 {
				return 1
			}
			p := 1 - t
			return 1 - p*p*p
		}
		visibleNow := make(map[EdgeMenuElement]struct{})
		viewIndex := 0
		for _, element := range c.elements {
			if element.IsVisible() {
				viewIndex++
				start, ok := c.revealStart[element]
				if !ok {
					delay := time.Duration(float64(viewIndex) * revealDelayStep * float64(time.Second))
					start = now.Add(delay)
					c.revealStart[element] = start
				}
				progress := (now.Sub(start).Seconds()) / revealDuration
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}
				progress = easeOutCubic(progress)
				if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
					r.SetRevealProgress(progress)
				}
				yOffset := int((1.0 - progress) * revealOffset)
				elementHeight := element.Draw(screen, x+10, currentY+5+yOffset, width-20, font)
				currentY += elementHeight + 5
				visibleNow[element] = struct{}{}
			}
		}
		for element := range c.revealStart {
			if _, ok := visibleNow[element]; !ok {
				delete(c.revealStart, element)
			}
		}
		currentY += 10 // Extra spacing at bottom
	}

	return currentY - y
}

func (c *CollapsibleMenu) GetMinHeight() int {
	height := c.options.HeaderHeight
	if !c.collapsed {
		for _, element := range c.elements {
			if element.IsVisible() {
				height += element.GetMinHeight() + 5
			}
		}
		height += 10 // Extra spacing
	}
	return height
}

func (c *CollapsibleMenu) SetCollapsed(collapsed bool) {
	c.collapsed = collapsed
	if collapsed {
		c.revealStart = make(map[EdgeMenuElement]time.Time)
	}
}

func (c *CollapsibleMenu) IsCollapsed() bool {
	return c.collapsed
}

// HasDraggingSliders checks if any sliders within this CollapsibleMenu are currently being dragged
func (c *CollapsibleMenu) HasDraggingSliders() bool {
	for _, element := range c.elements {
		if slider, ok := element.(*MenuSlider); ok {
			if slider.dragging {
				return true
			}
		}
		// Also check for sliders within nested elements like UpgradeControl and BonusControl
		if upgradeControl, ok := element.(*UpgradeControl); ok {
			if upgradeControl.slider != nil && upgradeControl.slider.dragging {
				return true
			}
		}
		if bonusControl, ok := element.(*BonusControl); ok {
			if bonusControl.slider != nil && bonusControl.slider.dragging {
				return true
			}
		}
	}
	return false
}

// EdgeMenu represents the main edge menu container
type EdgeMenu struct {
	title        string
	options      EdgeMenuOptions
	elements     []EdgeMenuElement
	visible      bool
	animating    bool
	animProgress float64
	animTarget   float64
	revealReady  bool
	revealStart  map[EdgeMenuElement]time.Time

	// Modal background
	modal                *TerritoryModal
	screenWidth          int
	screenHeight         int
	scrollOffset         int
	maxScroll            int
	scrollTarget         float64 // Target scroll position for smooth scrolling
	scrollVelocity       float64 // Current scroll velocity for smooth scrolling
	bounds               image.Rectangle
	closeButton          image.Rectangle
	font                 font.Face
	titleFont            font.Face
	contentHeight        int
	titleVisible         bool
	titleAnimating       bool
	titleProgress        float64
	titleTarget          float64
	lastScrollTime       float64
	lastUpdateTime       float64
	currentTerritory     string
	refreshCallback      func(string)
	territoryNavCallback func(string) // Callback for navigating to and opening a territory menu

	// Scrollbar interaction state
	scrollbarRect     image.Rectangle // The entire scrollbar track
	scrollbarHandle   image.Rectangle // The draggable handle
	scrollbarDragging bool            // Whether the handle is being dragged
	dragStartY        int             // Y position where drag started
	dragStartOffset   int             // Scroll offset when drag started

	// Delayed update system to prevent updates during slider dragging
	pendingUpdate       bool    // Whether an update is pending
	pendingFullRefresh  bool    // Whether a full refresh is needed (for Tower Multi-Attack)
	updateDelay         float64 // Time to wait after last change before updating
	timeSinceLastChange float64 // Time since the last change that requires an update

	// Mouse interaction tracking for preventing updates during drag operations
	needsTowerStatsUpdate   bool    // Whether tower stats need to be updated when mouse is released
	lastMousePressed        bool    // Previous frame's mouse button state
	towerStatsUpdateTime    float64 // Timer for periodic tower stats updates
	tradingRoutesUpdateTime float64 // Timer for periodic trading routes updates

	// State preservation for trading routes
	tradingRouteStates map[string]bool // Map of route title -> expanded state
}

// NewEdgeMenu creates a new edge menu with the specified title and options
func NewEdgeMenu(title string, options EdgeMenuOptions) *EdgeMenu {
	if options.Width <= 0 {
		options.Width = 400
	}

	// Initialize clipboard for text editing
	initClipboard()

	menu := &EdgeMenu{
		title:               title,
		options:             options,
		elements:            make([]EdgeMenuElement, 0),
		visible:             false,
		animating:           false,
		animProgress:        0.0,
		animTarget:          0.0,
		revealReady:         false,
		revealStart:         make(map[EdgeMenuElement]time.Time),
		modal:               NewTerritoryModal(),
		font:                loadWynncraftFont(16),
		titleFont:           loadWynncraftFont(22),
		titleVisible:        true,
		titleAnimating:      false,
		titleProgress:       1.0,
		titleTarget:         1.0,
		lastScrollTime:      0.0,
		scrollTarget:        0.0,
		scrollVelocity:      0.0,
		updateDelay:         0.3, // Wait 300ms after last change before updating
		pendingUpdate:       false,
		pendingFullRefresh:  false,
		timeSinceLastChange: 0.0,
		tradingRouteStates:  make(map[string]bool), // Initialize trading route states map
	}

	return menu
}

// Button adds a button to the menu
func (m *EdgeMenu) Button(text string, options ButtonOptions, callback func()) *EdgeMenu {
	button := NewMenuButton(text, options, callback)
	if m.revealReady {
		button.SetRevealProgress(0)
		delete(m.revealStart, button)
	}
	m.elements = append(m.elements, button)
	return m
}

// Text adds text to the menu
func (m *EdgeMenu) Text(text string, options TextOptions) *EdgeMenu {
	textElement := NewMenuText(text, options)
	if m.revealReady {
		textElement.SetRevealProgress(0)
		delete(m.revealStart, textElement)
	}
	m.elements = append(m.elements, textElement)
	return m
}

// Checkbox adds a checkbox to the menu
func (m *EdgeMenu) Checkbox(label string, checked bool, options CheckboxOptions, callback func(bool)) *MenuCheckbox {
	checkbox := NewMenuCheckbox(label, checked, options, callback)
	if m.revealReady {
		checkbox.SetRevealProgress(0)
		delete(m.revealStart, checkbox)
	}
	m.elements = append(m.elements, checkbox)
	return checkbox
}

// Slider adds a slider to the menu
func (m *EdgeMenu) Slider(label string, initialValue float64, options SliderOptions, callback func(float64)) *EdgeMenu {
	slider := NewMenuSlider(label, initialValue, options, callback)
	if m.revealReady {
		slider.SetRevealProgress(0)
		delete(m.revealStart, slider)
	}
	m.elements = append(m.elements, slider)
	return m
}

// RangeSlider adds a dual-handle slider to the menu
func (m *EdgeMenu) RangeSlider(label string, low, high float64, options RangeSliderOptions, callback func(float64, float64)) *MenuRangeSlider {
	slider := NewMenuRangeSlider(label, low, high, options, callback)
	if m.revealReady {
		slider.SetRevealProgress(0)
		delete(m.revealStart, slider)
	}
	m.elements = append(m.elements, slider)
	return slider
}

// Card adds a card container to the menu
func (m *EdgeMenu) Card() *Card {
	card := NewCard()
	if m.revealReady {
		if r, ok := interface{}(card).(interface{ SetRevealProgress(float64) }); ok {
			r.SetRevealProgress(0)
		}
		delete(m.revealStart, card)
	}
	m.elements = append(m.elements, card)
	return card
}

// Container adds a horizontal scrolling container to the menu
func (m *EdgeMenu) Container() *Container {
	container := NewContainer()
	if m.revealReady {
		if r, ok := interface{}(container).(interface{ SetRevealProgress(float64) }); ok {
			r.SetRevealProgress(0)
		}
		delete(m.revealStart, container)
	}
	m.elements = append(m.elements, container)
	return container
}

// CollapsibleMenu adds a collapsible section to the menu
func (m *EdgeMenu) CollapsibleMenu(title string, options CollapsibleMenuOptions) *CollapsibleMenu {
	collapsible := NewCollapsibleMenu(title, options)
	// collapsible.setParentMenu(m) // Set parent menu for focus management
	if m.revealReady {
		collapsible.SetRevealProgress(0)
		delete(m.revealStart, collapsible)
	}
	m.elements = append(m.elements, collapsible)
	return collapsible
}

// ClearElements removes all elements from the menu
func (m *EdgeMenu) ClearElements() {
	m.elements = make([]EdgeMenuElement, 0)
}

// SetTitle changes the menu title
func (m *EdgeMenu) SetTitle(title string) {
	m.title = title
}

// SaveCollapsedStates saves the current collapsed states of all collapsible sections
func (m *EdgeMenu) SaveCollapsedStates() map[string]bool {
	states := make(map[string]bool)
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			states[collapsible.title] = collapsible.collapsed
			// fmt.Printf("[DEBUG] Saving state for '%s': collapsed=%t\n", collapsible.title, collapsible.collapsed)
		}
	}
	// fmt.Printf("[DEBUG] Saved %d collapsed states\n", len(states))
	return states
}

// RestoreCollapsedStates restores the collapsed states of collapsible sections
func (m *EdgeMenu) RestoreCollapsedStates(states map[string]bool) {
	// fmt.Printf("[DEBUG] RestoreCollapsedStates called with %d states\n", len(states))
	// for title, collapsed := range states {
	// 	// fmt.Printf("[DEBUG] State to restore: '%s' = %t\n", title, collapsed)
	// }

	// fmt.Printf("[DEBUG] Checking %d elements for restoration\n", len(m.elements))
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			// fmt.Printf("[DEBUG] Found CollapsibleMenu[%d] with title '%s'\n", i, collapsible.title)
			if state, exists := states[collapsible.title]; exists {
				// fmt.Printf("[DEBUG] Restoring state for '%s': collapsed=%t\n", collapsible.title, state)
				collapsible.collapsed = state
			} else {
				// fmt.Printf("[DEBUG] No saved state for '%s', keeping default\n", collapsible.title)
			}
		} else {
			// fmt.Printf("[DEBUG] Element[%d] is not a CollapsibleMenu\n", i)
		}
	}
}

// Show makes the menu visible
func (m *EdgeMenu) Show() {
	m.visible = true
	m.animating = true
	m.animTarget = 1.0
	m.revealReady = false
	m.revealStart = make(map[EdgeMenuElement]time.Time)
	for _, element := range m.elements {
		if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
			r.SetRevealProgress(0)
		}
	}
	// Only show the modal if we're displaying territory content
	if m.modal != nil && m.currentTerritory != "" {
		m.modal.Show()
	}
}

// Hide conceals the menu with animation
func (m *EdgeMenu) Hide() {
	m.animTarget = 0.0
	m.animating = true
	m.revealReady = false
	// Always hide the modal when the menu is hidden
	if m.modal != nil {
		m.modal.Hide()
	}
}

// IsVisible returns whether the menu is currently visible
func (m *EdgeMenu) IsVisible() bool {
	return m.visible && m.animProgress > 0.01
}

// IsMouseInside checks if the given mouse coordinates are within the EdgeMenu bounds
func (m *EdgeMenu) IsMouseInside(mx, my int) bool {
	if !m.visible || m.animProgress < 0.1 {
		return false
	}
	return mx >= m.bounds.Min.X && mx <= m.bounds.Max.X &&
		my >= m.bounds.Min.Y && my <= m.bounds.Max.Y
}

// HasDraggingSliders checks if any sliders in the menu are currently being dragged
func (m *EdgeMenu) HasDraggingSliders() bool {
	for _, element := range m.elements {
		if hasSlidersDragging(element) {
			return true
		}
	}
	return false
}

// hasSlidersDragging recursively checks for dragging sliders in menu elements
func hasSlidersDragging(element EdgeMenuElement) bool {
	switch e := element.(type) {
	case *MenuSlider:
		if e.dragging {
			return true
		}
	case *MenuRangeSlider:
		if e.dragging {
			return true
		}
	case *UpgradeControl:
		if e.slider != nil && e.slider.dragging {
			return true
		}
	case *BonusControl:
		if e.slider != nil && e.slider.dragging {
			return true
		}
	case *CollapsibleMenu:
		for _, childElement := range e.elements {
			if hasSlidersDragging(childElement) {
				return true
			}
		}
	case *Card:
		for _, childElement := range e.elements {
			if hasSlidersDragging(childElement) {
				return true
			}
		}
	case *Container:
		for _, childElement := range e.elements {
			if hasSlidersDragging(childElement) {
				return true
			}
		}
	}
	return false
}

// ScheduleDelayedUpdate marks that an update is needed and resets the delay timer
func (m *EdgeMenu) ScheduleDelayedUpdate() {
	m.pendingUpdate = true
	m.timeSinceLastChange = 0.0
}

// ScheduleDelayedFullRefresh marks that a full refresh is needed and resets the delay timer
func (m *EdgeMenu) ScheduleDelayedFullRefresh() {
	m.pendingUpdate = true
	m.pendingFullRefresh = true
	m.timeSinceLastChange = 0.0
}

// ProcessDelayedUpdates handles delayed updates after the delay period has passed
func (m *EdgeMenu) ProcessDelayedUpdates(deltaTime float64) {
	if m.pendingUpdate {
		m.timeSinceLastChange += deltaTime

		// Only update if the delay has passed AND no sliders are being dragged
		if m.timeSinceLastChange >= m.updateDelay && !m.HasDraggingSliders() {
			m.pendingUpdate = false
			m.timeSinceLastChange = 0.0

			// Perform the delayed update
			if m.pendingFullRefresh {
				// Do full refresh for Tower Multi-Attack changes
				m.pendingFullRefresh = false
				m.refreshMenuData()
			} else if m.currentTerritory != "" {
				// Do targeted update for tower stats only
				m.UpdateTowerStats(m.currentTerritory)
				m.UpdateTotalCosts(m.currentTerritory)
			}
		}
	}
}

// Update handles input and animations
func (m *EdgeMenu) Update(screenWidth, screenHeight int, deltaTime float64) bool {
	m.screenWidth = screenWidth
	m.screenHeight = screenHeight
	m.lastScrollTime += deltaTime
	m.lastUpdateTime += deltaTime
	m.towerStatsUpdateTime += deltaTime
	m.tradingRoutesUpdateTime += deltaTime

	// Check if any pointer is currently pressed (mouse or touch)
	pointerPressed := primaryPressed() || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)

	// Update mouse state for next frame
	m.lastMousePressed = pointerPressed

	// Update tower stats every 1 second when no mouse buttons are pressed
	if m.towerStatsUpdateTime >= 1.0 && !pointerPressed && m.currentTerritory != "" {
		m.towerStatsUpdateTime = 0.0
		m.UpdateTowerStats(m.currentTerritory)
		// fmt.Printf("DEBUG: Periodic tower stats update for territory: %s\n", m.currentTerritory)
	}

	// Update trading routes every 2 seconds when no mouse buttons are pressed (less frequent than tower stats)
	if m.tradingRoutesUpdateTime >= 2.0 && !pointerPressed && m.currentTerritory != "" {
		m.tradingRoutesUpdateTime = 0.0
		m.UpdateTradingRoutes(m.currentTerritory)
		// fmt.Printf("DEBUG: Periodic trading routes update for territory: %s\n", m.currentTerritory)
	}

	// Update menu data every 50ms for territory stats, but only if no mouse buttons are pressed
	if m.lastUpdateTime >= 0.05 {
		m.lastUpdateTime = 0.0
		// Skip refreshing menu data if any mouse button is pressed to avoid interrupting interactions
		if !pointerPressed {
			// fmt.Printf("DEBUG: Regular refreshMenuData called\n")
			m.refreshMenuData()
		} else {
			// fmt.Printf("DEBUG: Skipping refreshMenuData - mouse button pressed\n")
		}
	}

	// Update main menu animations
	if m.animating {
		if math.Abs(m.animProgress-m.animTarget) > 0.01 {
			diff := m.animTarget - m.animProgress
			m.animProgress += diff * 16.0 * deltaTime
		} else {
			m.animProgress = m.animTarget
			m.animating = false
			if m.animProgress <= 0.01 {
				m.visible = false
			}
		}
	}

	if m.animTarget >= 1.0 && m.animProgress >= 0.99 {
		if !m.revealReady {
			m.revealReady = true
			if m.revealStart == nil {
				m.revealStart = make(map[EdgeMenuElement]time.Time)
			}
		}
	}

	// Update modal background animations
	if m.modal != nil {
		m.modal.SetScreenDimensions(screenWidth, screenHeight)
		m.modal.SetEdgeMenuPosition(m.options.Position)
		m.modal.Update(deltaTime)
	}

	// Update title animations
	if m.titleAnimating {
		if math.Abs(m.titleProgress-m.titleTarget) > 0.01 {
			diff := m.titleTarget - m.titleProgress
			m.titleProgress += diff * 16.0 * deltaTime
		} else {
			m.titleProgress = m.titleTarget
			m.titleAnimating = false
		}
	}

	// Only show title when at the very top - no auto-show timer

	if !m.IsVisible() {
		return false
	}

	// Calculate bounds
	m.calculateBounds()

	mx, my := primaryPointerPosition()

	// Handle close button (only accept clicks when significantly visible)
	if m.options.Closable && m.titleProgress > 0.5 {
		if px, py, pressed := primaryJustPressed(); pressed && pointInRect(px, py, m.closeButton) {
			m.Hide()
			return true
		}
	}

	// Calculate dynamic title height based on visibility
	titleHeight := int(float64(50) * m.titleProgress)
	contentY := m.bounds.Min.Y + titleHeight
	contentHeight := m.bounds.Dy() - titleHeight

	// Update elements first - let them handle scroll events before the menu does
	handled := false

	if m.options.HorizontalScroll {
		// Horizontal layout
		currentX := m.bounds.Min.X + 20 - m.scrollOffset
		cardWidth := 200
		cardSpacing := 20

		for _, element := range m.elements {
			if element.IsVisible() {
				// Always update animations for all visible elements regardless of mouse position
				elementInBounds := currentX+cardWidth > m.bounds.Min.X && currentX < m.bounds.Max.X
				mouseInElement := mx >= currentX && mx < currentX+cardWidth &&
					my >= contentY && my < contentY+contentHeight

				// Update element: pass mouse coordinates only if mouse is actually over the element
				if elementInBounds {
					var updateMx, updateMy int
					if mouseInElement {
						updateMx, updateMy = mx, my
					} else {
						// Pass coordinates outside element bounds to ensure animations continue
						// but input events (clicks, hovers) are not triggered
						updateMx, updateMy = -1, -1
					}

					if element.Update(updateMx, updateMy, deltaTime) && mouseInElement {
						handled = true
						break // Stop processing other elements when one handles input
					}
				}
				currentX += cardWidth + cardSpacing
			}
		}
	} else {
		// Vertical layout
		currentY := contentY - m.scrollOffset

		for _, element := range m.elements {
			if element.IsVisible() {
				elementHeight := element.GetMinHeight()

				// Always update animations for all visible elements regardless of mouse position
				elementInBounds := currentY+elementHeight > contentY && currentY < contentY+contentHeight
				mouseInElement := mx >= m.bounds.Min.X && mx < m.bounds.Max.X &&
					my >= contentY && my < contentY+contentHeight

				// Update element: pass mouse coordinates only if mouse is actually over the element
				if elementInBounds {
					var updateMx, updateMy int
					if mouseInElement {
						updateMx, updateMy = mx, my
					} else {
						// Pass coordinates outside element bounds to ensure animations continue
						// but input events (clicks, hovers) are not triggered
						updateMx, updateMy = -1, -1
					}

					if element.Update(updateMx, updateMy, deltaTime) && mouseInElement {
						handled = true
						break // Stop processing other elements when one handles input
					}
				}
				currentY += elementHeight + 10
			}
		}
	}

	// Handle scrollbar interactions first (before wheel scrolling)
	if !handled && m.options.Scrollable && m.maxScroll > 0 {
		// Handle scrollbar dragging
		if m.scrollbarDragging {
			if primaryPressed() {
				// Continue dragging
				if m.options.HorizontalScroll {
					// Horizontal scrollbar dragging
					deltaX := mx - m.dragStartY // Using dragStartY for X in horizontal mode
					trackWidth := m.scrollbarRect.Dx() - m.scrollbarHandle.Dx()
					if trackWidth > 0 {
						scrollDelta := float64(deltaX) * float64(m.maxScroll) / float64(trackWidth)
						newOffset := float64(m.dragStartOffset) + scrollDelta
						m.scrollTarget = math.Max(0, math.Min(float64(m.maxScroll), newOffset))
						m.scrollOffset = int(m.scrollTarget) // Immediate response while dragging
					}
				} else {
					// Vertical scrollbar dragging
					deltaY := my - m.dragStartY
					trackHeight := m.scrollbarRect.Dy() - m.scrollbarHandle.Dy()
					if trackHeight > 0 {
						scrollDelta := float64(deltaY) * float64(m.maxScroll) / float64(trackHeight)
						newOffset := float64(m.dragStartOffset) + scrollDelta
						m.scrollTarget = math.Max(0, math.Min(float64(m.maxScroll), newOffset))
						m.scrollOffset = int(m.scrollTarget) // Immediate response while dragging
					}
				}
				handled = true
			} else {
				// Stop dragging
				m.scrollbarDragging = false
			}
		} else {
			// Check for scrollbar interactions when not already dragging
			if px, py, pressed := primaryJustPressed(); pressed {
				// Check if clicking on scrollbar handle to start dragging
				if px >= m.scrollbarHandle.Min.X && px < m.scrollbarHandle.Max.X &&
					py >= m.scrollbarHandle.Min.Y && py < m.scrollbarHandle.Max.Y {
					m.scrollbarDragging = true
					if m.options.HorizontalScroll {
						m.dragStartY = px // Store X position for horizontal scrolling
					} else {
						m.dragStartY = py
					}
					m.dragStartOffset = m.scrollOffset
					handled = true
				} else if px >= m.scrollbarRect.Min.X && px < m.scrollbarRect.Max.X &&
					py >= m.scrollbarRect.Min.Y && py < m.scrollbarRect.Max.Y {
					// Clicking on scrollbar track (not handle) - jump to position
					if m.options.HorizontalScroll {
						// Horizontal scrollbar track click
						trackWidth := m.scrollbarRect.Dx() - m.scrollbarHandle.Dx()
						if trackWidth > 0 {
							relativeX := px - m.scrollbarRect.Min.X - m.scrollbarHandle.Dx()/2
							scrollRatio := float64(relativeX) / float64(trackWidth)
							scrollRatio = math.Max(0, math.Min(1, scrollRatio))
							m.scrollTarget = scrollRatio * float64(m.maxScroll)
						}
					} else {
						// Vertical scrollbar track click
						trackHeight := m.scrollbarRect.Dy() - m.scrollbarHandle.Dy()
						if trackHeight > 0 {
							relativeY := py - m.scrollbarRect.Min.Y - m.scrollbarHandle.Dy()/2
							scrollRatio := float64(relativeY) / float64(trackHeight)
							scrollRatio = math.Max(0, math.Min(1, scrollRatio))
							m.scrollTarget = scrollRatio * float64(m.maxScroll)
						}
					}
					m.lastScrollTime = 0.0
					handled = true
				}
			}
		}
	}

	// Stop scrollbar dragging if primary pointer is released anywhere
	if m.scrollbarDragging {
		if _, _, released := primaryJustReleased(); released {
			m.scrollbarDragging = false
		}
	}

	// Only handle wheel scrolling if no child element or scrollbar handled the input
	if !handled && m.options.Scrollable && mx >= m.bounds.Min.X && mx < m.bounds.Max.X && my >= m.bounds.Min.Y && my < m.bounds.Max.Y {
		scrollX, scrollY := ebiten.Wheel()

		if m.options.HorizontalScroll && scrollY != 0 {
			// Horizontal scrolling using vertical wheel when in horizontal mode
			m.scrollTarget -= scrollY * 120
			m.scrollTarget = math.Max(0, math.Min(float64(m.maxScroll), m.scrollTarget))
			m.lastScrollTime = 0.0
			return true
		} else if m.options.HorizontalScroll && scrollX != 0 {
			// True horizontal scrolling (from trackpad or special devices)
			m.scrollTarget -= scrollX * 120
			m.scrollTarget = math.Max(0, math.Min(float64(m.maxScroll), m.scrollTarget))
			m.lastScrollTime = 0.0
			return true
		} else if !m.options.HorizontalScroll && scrollY != 0 {
			// Vertical scrolling (default)
			m.scrollTarget -= scrollY * 120
			m.scrollTarget = math.Max(0, math.Min(float64(m.maxScroll), m.scrollTarget))
			m.lastScrollTime = 0.0
			return true
		}
	}
	// Update smooth scrolling animation (but not while dragging scrollbar)
	if !m.scrollbarDragging && math.Abs(float64(m.scrollOffset)-m.scrollTarget) > 0.1 {
		// Simple smooth interpolation without physics - like browser scrolling
		diff := m.scrollTarget - float64(m.scrollOffset)

		// Smooth interpolation factor - adjust this to change scroll speed
		smoothFactor := 8.0 * deltaTime

		// Move toward target position
		newOffset := float64(m.scrollOffset) + diff*smoothFactor
		m.scrollOffset = int(newOffset)

		// Clamp to bounds
		m.scrollOffset = int(math.Max(0, math.Min(float64(m.maxScroll), float64(m.scrollOffset))))
	}

	// Update title visibility based on current scroll position
	if m.scrollOffset == 0 {
		if !m.titleVisible {
			m.titleVisible = true
			m.titleTarget = 1.0
			m.titleAnimating = true
		}
	} else {
		// Hide title and X button when not at the very top
		if m.titleVisible {
			m.titleVisible = false
			m.titleTarget = 0.0
			m.titleAnimating = true
		}
	}

	// Consume all mouse input if within menu bounds to prevent click-through
	if mx >= m.bounds.Min.X && mx < m.bounds.Max.X && my >= m.bounds.Min.Y && my < m.bounds.Max.Y {
		// Consume all mouse button presses within menu bounds
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
			return true
		}
		// Also consume mouse button releases to prevent them from affecting background
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) ||
			inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonRight) ||
			inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonMiddle) {
			return true
		}
	}

	return handled
}

func (m *EdgeMenu) calculateBounds() {
	width := m.options.Width
	height := m.options.Height
	if height <= 0 {
		height = m.screenHeight
	}

	// For sliding animation, only animate the relevant dimension
	// and slide from the edge, not grow from corner
	switch m.options.Position {
	case EdgeMenuRight:
		// Slide from right edge - animate width only, keep full height
		animatedWidth := int(float64(width) * m.animProgress)
		m.bounds = image.Rect(m.screenWidth-animatedWidth, 0, m.screenWidth, height)
	case EdgeMenuLeft:
		// Slide from left edge - animate width only, keep full height
		animatedWidth := int(float64(width) * m.animProgress)
		m.bounds = image.Rect(0, 0, animatedWidth, height)
	case EdgeMenuTop:
		// Slide from top edge - animate height only, keep full width
		animatedHeight := int(float64(height) * m.animProgress)
		m.bounds = image.Rect(0, 0, width, animatedHeight)
	case EdgeMenuBottom:
		// Slide from bottom edge - animate height only, keep full width
		animatedHeight := int(float64(height) * m.animProgress)
		m.bounds = image.Rect(0, m.screenHeight-animatedHeight, width, m.screenHeight)
	}

	// Close button position
	if m.options.Closable {
		buttonSize := 24
		m.closeButton = image.Rect(
			m.bounds.Max.X-buttonSize-15,
			m.bounds.Min.Y+15,
			m.bounds.Max.X-15,
			m.bounds.Min.Y+15+buttonSize,
		)
	}
}

// Draw renders the menu
func (m *EdgeMenu) Draw(screen *ebiten.Image) {
	if !m.IsVisible() {
		return
	}

	// Draw modal background overlay if visible
	if m.modal != nil {
		m.modal.Draw(screen)
	}

	// Calculate dynamic title height based on animation
	titleHeight := int(float64(50) * m.titleProgress)

	// Calculate content dimensions for scrolling
	if m.options.HorizontalScroll {
		// For horizontal scrolling, calculate total width
		m.contentHeight = 20 // Start with some padding for width calculation
		for _, element := range m.elements {
			if element.IsVisible() {
				// For horizontal layout, sum widths instead of heights
				// Use a fixed card width for calculation
				m.contentHeight += 220 // 200px card width + 20px padding
			}
		}
		availableWidth := m.bounds.Dx()
		m.maxScroll = int(math.Max(0, float64(m.contentHeight-availableWidth)))
	} else {
		// For vertical scrolling, calculate total height
		m.contentHeight = 20 // Start with some padding
		for _, element := range m.elements {
			if element.IsVisible() {
				m.contentHeight += element.GetMinHeight() + 10
			}
		}
		availableHeight := m.bounds.Dy() - titleHeight
		m.maxScroll = int(math.Max(0, float64(m.contentHeight-availableHeight)))
	}

	// Draw background
	bgColor := m.options.Background
	bgColor.A = uint8(float32(bgColor.A) * float32(m.animProgress))
	vector.DrawFilledRect(screen, float32(m.bounds.Min.X), float32(m.bounds.Min.Y), float32(m.bounds.Dx()), float32(m.bounds.Dy()), bgColor, false)

	// Draw border
	borderColor := m.options.BorderColor
	borderColor.A = uint8(float32(borderColor.A) * float32(m.animProgress))

	switch m.options.Position {
	case EdgeMenuRight:
		vector.DrawFilledRect(screen, float32(m.bounds.Min.X), float32(m.bounds.Min.Y), 3, float32(m.bounds.Dy()), borderColor, false)
	case EdgeMenuLeft:
		vector.DrawFilledRect(screen, float32(m.bounds.Max.X-3), float32(m.bounds.Min.Y), 3, float32(m.bounds.Dy()), borderColor, false)
	case EdgeMenuTop:
		vector.DrawFilledRect(screen, float32(m.bounds.Min.X), float32(m.bounds.Max.Y-3), float32(m.bounds.Dx()), 3, borderColor, false)
	case EdgeMenuBottom:
		vector.DrawFilledRect(screen, float32(m.bounds.Min.X), float32(m.bounds.Min.Y), float32(m.bounds.Dx()), 3, borderColor, false)
	}

	// Draw title with animation
	if m.titleProgress > 0.01 {
		titleColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(m.animProgress) * float32(m.titleProgress))}
		titleY := m.bounds.Min.Y + int(float64(30)*m.titleProgress)
		text.Draw(screen, m.title, m.titleFont, m.bounds.Min.X+20, titleY, titleColor)
	}

	// Draw close button (with smooth fade animation)
	if m.options.Closable && m.titleProgress > 0.01 {
		mx, my := ebiten.CursorPosition()
		buttonColor := color.RGBA{200, 60, 60, uint8(float32(200) * float32(m.animProgress) * float32(m.titleProgress))}
		if mx >= m.closeButton.Min.X && mx < m.closeButton.Max.X && my >= m.closeButton.Min.Y && my < m.closeButton.Max.Y {
			buttonColor = color.RGBA{255, 60, 60, uint8(float32(255) * float32(m.animProgress) * float32(m.titleProgress))}
		}
		vector.DrawFilledRect(screen, float32(m.closeButton.Min.X), float32(m.closeButton.Min.Y), float32(m.closeButton.Dx()), float32(m.closeButton.Dy()), buttonColor, false)
		textColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(m.animProgress) * float32(m.titleProgress))}
		text.Draw(screen, "×", m.titleFont, m.closeButton.Min.X+6, m.closeButton.Min.Y+18, textColor)
	}

	// Draw elements with layout based on scroll direction
	contentY := m.bounds.Min.Y + titleHeight
	contentHeight := m.bounds.Dy() - titleHeight
	elementWidth := m.bounds.Dx() - 40
	if !eruntime.GetRuntimeOptions().SidemenuAnimations {
		if m.options.HorizontalScroll {
			currentX := m.bounds.Min.X + 20 - m.scrollOffset
			cardWidth := 200
			cardSpacing := 20

			for _, element := range m.elements {
				if element.IsVisible() {
					if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(1)
					}
					if currentX+cardWidth > m.bounds.Min.X && currentX < m.bounds.Max.X {
						element.Draw(screen, currentX, contentY+10, cardWidth, m.font)
					}
					currentX += cardWidth + cardSpacing
				}
			}
		} else {
			currentY := contentY - m.scrollOffset

			for _, element := range m.elements {
				if element.IsVisible() {
					elementHeight := element.GetMinHeight()
					if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(1)
					}
					if currentY+elementHeight > contentY && currentY < contentY+contentHeight {
						element.Draw(screen, m.bounds.Min.X+20, currentY, elementWidth, m.font)
					}
					currentY += elementHeight + 10
				}
			}
		}

		// Draw scrollbar if needed
		if m.options.Scrollable && m.maxScroll > 0 {
			m.drawScrollbar(screen, contentY, contentHeight)
		}
		return
	}
	if !m.revealReady {
		return
	}

	now := time.Now()
	revealDuration := 0.65
	revealDelayStep := 0.04
	revealOffset := 7.0
	easeOutCubic := func(t float64) float64 {
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		p := 1 - t
		return 1 - p*p*p
	}
	visibleNow := make(map[EdgeMenuElement]struct{})

	if m.options.HorizontalScroll {
		// Horizontal layout
		currentX := m.bounds.Min.X + 20 - m.scrollOffset
		cardWidth := 200
		cardSpacing := 20
		viewIndex := 0

		for _, element := range m.elements {
			if element.IsVisible() {
				inView := currentX+cardWidth > m.bounds.Min.X && currentX < m.bounds.Max.X
				if inView {
					viewIndex++
					start, ok := m.revealStart[element]
					if !ok {
						delay := time.Duration(float64(viewIndex) * revealDelayStep * float64(time.Second))
						start = now.Add(delay)
						m.revealStart[element] = start
					}
					progress := (now.Sub(start).Seconds()) / revealDuration
					if progress < 0 {
						progress = 0
					}
					if progress > 1 {
						progress = 1
					}
					progress = easeOutCubic(progress)
					if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(progress)
					}
					yOffset := int((1.0 - progress) * revealOffset)
					element.Draw(screen, currentX, contentY+10+yOffset, cardWidth, m.font)
					visibleNow[element] = struct{}{}
				}
				currentX += cardWidth + cardSpacing
			}
		}
	} else {
		// Vertical layout
		currentY := contentY - m.scrollOffset
		viewIndex := 0

		for _, element := range m.elements {
			if element.IsVisible() {
				elementHeight := element.GetMinHeight()

				inView := currentY+elementHeight > contentY && currentY < contentY+contentHeight
				if inView {
					viewIndex++
					start, ok := m.revealStart[element]
					if !ok {
						delay := time.Duration(float64(viewIndex) * revealDelayStep * float64(time.Second))
						start = now.Add(delay)
						m.revealStart[element] = start
					}
					progress := (now.Sub(start).Seconds()) / revealDuration
					if progress < 0 {
						progress = 0
					}
					if progress > 1 {
						progress = 1
					}
					progress = easeOutCubic(progress)
					if r, ok := element.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(progress)
					}
					yOffset := int((1.0 - progress) * revealOffset)
					element.Draw(screen, m.bounds.Min.X+20, currentY+yOffset, elementWidth, m.font)
					visibleNow[element] = struct{}{}
				}
				currentY += elementHeight + 10
			}
		}

	}

	for element := range m.revealStart {
		if _, ok := visibleNow[element]; !ok {
			delete(m.revealStart, element)
		}
	}

	// Draw scrollbar if needed
	if m.options.Scrollable && m.maxScroll > 0 {
		m.drawScrollbar(screen, contentY, contentHeight)
	}
}

func (m *EdgeMenu) drawScrollbar(screen *ebiten.Image, contentY, contentHeight int) {
	if m.options.HorizontalScroll {
		// Horizontal scrollbar
		scrollbarHeight := 12
		scrollbarY := m.bounds.Max.Y - scrollbarHeight - 5
		scrollbarWidth := m.bounds.Dx() - 40

		// Store scrollbar rect for interaction
		m.scrollbarRect = image.Rect(m.bounds.Min.X+20, scrollbarY, m.bounds.Min.X+20+scrollbarWidth, scrollbarY+scrollbarHeight)

		// Background
		bgColor := color.RGBA{60, 60, 60, uint8(float32(100) * float32(m.animProgress))}
		vector.DrawFilledRect(screen, float32(m.scrollbarRect.Min.X), float32(m.scrollbarRect.Min.Y), float32(scrollbarWidth), float32(scrollbarHeight), bgColor, false)

		// Handle
		scrollRatio := float64(m.scrollOffset) / float64(m.maxScroll)
		handleWidth := int(float64(scrollbarWidth) * float64(m.bounds.Dx()) / float64(m.contentHeight))
		handleWidth = int(math.Max(20, float64(handleWidth))) // Minimum handle size

		handleX := m.scrollbarRect.Min.X + int(scrollRatio*float64(scrollbarWidth-handleWidth))

		// Store handle rect for interaction
		m.scrollbarHandle = image.Rect(handleX, scrollbarY+2, handleX+handleWidth, scrollbarY+scrollbarHeight-2)

		// Draw handle with hover effect
		handleColor := color.RGBA{150, 150, 150, uint8(float32(200) * float32(m.animProgress))}
		mx, my := ebiten.CursorPosition()
		if (mx >= m.scrollbarHandle.Min.X && mx < m.scrollbarHandle.Max.X &&
			my >= m.scrollbarHandle.Min.Y && my < m.scrollbarHandle.Max.Y) || m.scrollbarDragging {
			handleColor = color.RGBA{180, 180, 180, uint8(float32(230) * float32(m.animProgress))}
		}
		vector.DrawFilledRect(screen, float32(m.scrollbarHandle.Min.X), float32(m.scrollbarHandle.Min.Y),
			float32(m.scrollbarHandle.Dx()), float32(m.scrollbarHandle.Dy()), handleColor, false)
	} else {
		// Vertical scrollbar
		scrollbarWidth := 12
		scrollbarX := m.bounds.Max.X - scrollbarWidth - 5

		// Store scrollbar rect for interaction
		m.scrollbarRect = image.Rect(scrollbarX, contentY, scrollbarX+scrollbarWidth, contentY+contentHeight)

		// Background
		bgColor := color.RGBA{60, 60, 60, uint8(float32(100) * float32(m.animProgress))}
		vector.DrawFilledRect(screen, float32(m.scrollbarRect.Min.X), float32(m.scrollbarRect.Min.Y), float32(scrollbarWidth), float32(contentHeight), bgColor, false)

		// Handle
		scrollRatio := float64(m.scrollOffset) / float64(m.maxScroll)
		handleHeight := int(float64(contentHeight) * float64(contentHeight) / float64(m.contentHeight))
		handleHeight = int(math.Max(20, float64(handleHeight))) // Minimum handle size

		handleY := contentY + int(scrollRatio*float64(contentHeight-handleHeight))

		// Store handle rect for interaction
		m.scrollbarHandle = image.Rect(scrollbarX+2, handleY, scrollbarX+scrollbarWidth-2, handleY+handleHeight)

		// Draw handle with hover effect
		handleColor := color.RGBA{150, 150, 150, uint8(float32(200) * float32(m.animProgress))}
		mx, my := ebiten.CursorPosition()
		if (mx >= m.scrollbarHandle.Min.X && mx < m.scrollbarHandle.Max.X &&
			my >= m.scrollbarHandle.Min.Y && my < m.scrollbarHandle.Max.Y) || m.scrollbarDragging {
			handleColor = color.RGBA{180, 180, 180, uint8(float32(230) * float32(m.animProgress))}
		}
		vector.DrawFilledRect(screen, float32(m.scrollbarHandle.Min.X), float32(m.scrollbarHandle.Min.Y),
			float32(m.scrollbarHandle.Dx()), float32(m.scrollbarHandle.Dy()), handleColor, false)
	}
}

// UpgradeControl represents a complete upgrade control with slider, buttons, and cost display
type UpgradeControl struct {
	BaseMenuElement
	label           string
	upgradeType     string
	territoryName   string
	currentLevel    int
	maxLevel        int
	slider          *MenuSlider
	decrementBtn    *MenuButton
	incrementBtn    *MenuButton
	costText        *MenuText
	refreshCallback func(string) // Callback to refresh menu when upgrade changes
	parentMenu      *EdgeMenu    // Reference to parent menu for targeted updates
}

func NewUpgradeControl(label, upgradeType, territoryName string, currentLevel int) *UpgradeControl {
	uc := &UpgradeControl{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		upgradeType:     upgradeType,
		territoryName:   territoryName,
		currentLevel:    currentLevel,
		maxLevel:        11,
		refreshCallback: nil, // Will be set by parent menu
	}

	// Create slider
	sliderOptions := DefaultSliderOptions()
	sliderOptions.MinValue = 0
	sliderOptions.MaxValue = 11
	sliderOptions.Step = 1
	sliderOptions.ShowValue = false
	sliderOptions.ValueFormat = "%.0f"

	uc.slider = NewMenuSlider("", float64(currentLevel), sliderOptions, func(value float64) {
		uc.setLevel(int(value))
	})

	// Set up drag end callback to update tower stats only when dragging finishes
	uc.slider.SetOnDragEnd(func() {
		// fmt.Printf("DEBUG: UpgradeControl drag ended for %s\n", uc.upgradeType)
		if uc.parentMenu != nil {
			uc.parentMenu.UpdateTowerStats(uc.territoryName)
		}
	})

	// Create decrement button
	decrementOptions := DefaultButtonOptions()
	decrementOptions.Height = 20
	decrementOptions.BackgroundColor = color.RGBA{128, 128, 128, 255} // Grey
	decrementOptions.HoverColor = color.RGBA{148, 148, 148, 255}      // Lighter grey on hover
	decrementOptions.PressedColor = color.RGBA{108, 108, 108, 255}    // Darker grey when pressed
	decrementOptions.BorderColor = color.RGBA{168, 168, 168, 255}     // Light grey border
	uc.decrementBtn = NewMenuButton("-", decrementOptions, func() {
		if uc.currentLevel > 0 {
			uc.setLevel(uc.currentLevel - 1)
		}
	})

	// Create increment button
	incrementOptions := DefaultButtonOptions()
	incrementOptions.Height = 20
	incrementOptions.BackgroundColor = color.RGBA{128, 128, 128, 255} // Grey
	incrementOptions.HoverColor = color.RGBA{148, 148, 148, 255}      // Lighter grey on hover
	incrementOptions.PressedColor = color.RGBA{108, 108, 108, 255}    // Darker grey when pressed
	incrementOptions.BorderColor = color.RGBA{168, 168, 168, 255}     // Light grey border
	uc.incrementBtn = NewMenuButton("+", incrementOptions, func() {
		if uc.currentLevel < uc.maxLevel {
			uc.setLevel(uc.currentLevel + 1)
		}
	})

	// Create cost text
	uc.updateCostDisplay()

	return uc
}

func (uc *UpgradeControl) setLevel(level int) {
	// fmt.Printf("DEBUG: UpgradeControl.setLevel called for %s, level %d\n", uc.upgradeType, level)
	if level < 0 || level > uc.maxLevel {
		return
	}

	uc.currentLevel = level
	uc.slider.SetValue(float64(level))
	uc.updateCostDisplay()

	// Update the territory in eruntime
	eruntime.SetTerritoryUpgrade(uc.territoryName, uc.upgradeType, level)

	// Update costs immediately (no visual disruption)
	if uc.parentMenu != nil {
		uc.parentMenu.UpdateTotalCosts(uc.territoryName)
		// Tower stats will be updated by the periodic timer or on mouse release
		// fmt.Printf("DEBUG: UpgradeControl - setLevel completed, tower stats will update periodically\n")
	}
}

func (uc *UpgradeControl) updateCostDisplay() {
	cost, resourceType := eruntime.GetUpgradeCost(uc.upgradeType, uc.currentLevel)
	if cost > 0 {
		costOptions := DefaultTextOptions()
		costOptions.Color = color.RGBA{200, 200, 150, 255} // Light yellow for cost
		uc.costText = NewMenuText(fmt.Sprintf("Cost: %d %s", cost, resourceType), costOptions)
	} else {
		costOptions := DefaultTextOptions()
		costOptions.Color = color.RGBA{150, 150, 150, 255} // Gray for no cost
		uc.costText = NewMenuText("Cost: 0", costOptions)
	}
}

// isAffordable checks if the current upgrade level is affordable by comparing Set vs At
func (uc *UpgradeControl) isAffordable() bool {
	territory := eruntime.GetTerritory(uc.territoryName)
	if territory == nil {
		return true // Default to affordable if territory not found
	}

	var setLevel, atLevel int
	switch uc.upgradeType {
	case "damage":
		setLevel = territory.Options.Upgrade.Set.Damage
		atLevel = territory.Options.Upgrade.At.Damage
	case "attack":
		setLevel = territory.Options.Upgrade.Set.Attack
		atLevel = territory.Options.Upgrade.At.Attack
	case "health":
		setLevel = territory.Options.Upgrade.Set.Health
		atLevel = territory.Options.Upgrade.At.Health
	case "defence":
		setLevel = territory.Options.Upgrade.Set.Defence
		atLevel = territory.Options.Upgrade.At.Defence
	default:
		return true
	}

	// If normally affordable, return true
	if setLevel == atLevel {
		return true
	}

	// At tick :59, check with tolerance if resources are slightly insufficient
	if setLevel > 0 && atLevel == 0 {
		return eruntime.CheckUpgradeAffordabilityWithTolerance(uc.territoryName, uc.upgradeType, setLevel)
	}

	return false
}

func (uc *UpgradeControl) Update(mx, my int, deltaTime float64) bool {
	uc.updateAnimation(deltaTime)

	// Update slider color based on affordability
	if uc.isAffordable() {
		// Blue color when affordable
		uc.slider.options.FillColor = color.RGBA{100, 150, 255, 255}
	} else {
		// Grey color when not affordable
		uc.slider.options.FillColor = color.RGBA{128, 128, 128, 255}
	}

	// Update all child components and check if any handled input
	handled := false

	// Update buttons first (they're smaller targets)
	if uc.decrementBtn.Update(mx, my, deltaTime) {
		handled = true
	}
	if uc.incrementBtn.Update(mx, my, deltaTime) {
		handled = true
	}

	// Update slider
	if uc.slider.Update(mx, my, deltaTime) {
		handled = true
	}

	return handled
}

func (uc *UpgradeControl) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !uc.visible || uc.displayAlpha() <= 0.01 {
		return 0
	}

	currentY := y
	alpha := float32(uc.displayAlpha())
	rowHeight := 30

	// Draw label
	labelText := fmt.Sprintf("%s: Level %d", uc.label, uc.currentLevel)
	textColor := color.RGBA{255, 255, 255, uint8(float32(255) * alpha)}
	text.Draw(screen, labelText, font, x, currentY+20, textColor)
	currentY += rowHeight

	// Draw buttons and slider on the same row
	buttonWidth := 30
	sliderWidth := width - (buttonWidth * 2) - 20 // Leave space for buttons and padding
	uc.decrementBtn.SetRevealProgress(uc.displayAlpha())
	uc.incrementBtn.SetRevealProgress(uc.displayAlpha())
	uc.slider.SetRevealProgress(uc.displayAlpha())
	if uc.costText != nil {
		uc.costText.SetRevealProgress(uc.displayAlpha())
	}

	// Calculate button Y offset to center with slider bar
	// Slider bar center is at currentY + 24 (see MenuSlider.Draw), button should be centered on that
	sliderBarCenterY := 24 // From MenuSlider.Draw: sliderY = y + 24
	buttonHeight := 20     // Button height
	buttonYOffset := sliderBarCenterY - (buttonHeight / 2)

	// Draw decrement button (offset to center with slider bar)
	uc.decrementBtn.Draw(screen, x, currentY+buttonYOffset, buttonWidth, font)

	// Draw slider
	uc.slider.Draw(screen, x+buttonWidth+5, currentY, sliderWidth, font)

	// Draw increment button (offset to center with slider bar)
	uc.incrementBtn.Draw(screen, x+buttonWidth+sliderWidth+10, currentY+buttonYOffset, buttonWidth, font)

	currentY += rowHeight

	// Draw cost display
	if uc.costText != nil {
		uc.costText.Draw(screen, x+10, currentY, width-20, font)
		currentY += uc.costText.GetMinHeight()
	}

	return currentY - y + 10 // Total height plus some padding
}

func (uc *UpgradeControl) GetMinHeight() int {
	return 90 // Label + controls + cost + padding
}

// refreshData updates the upgrade control with current territory data
func (uc *UpgradeControl) refreshData() {
	territory := eruntime.GetTerritory(uc.territoryName)
	if territory == nil {
		return
	}

	// Get current set level for this upgrade type
	var setLevel int
	switch uc.upgradeType {
	case "damage":
		setLevel = territory.Options.Upgrade.Set.Damage
	case "attack":
		setLevel = territory.Options.Upgrade.Set.Attack
	case "health":
		setLevel = territory.Options.Upgrade.Set.Health
	case "defence":
		setLevel = territory.Options.Upgrade.Set.Defence
	default:
		return
	}

	// Update current level if it changed
	if uc.currentLevel != setLevel {
		uc.currentLevel = setLevel
		// Only update slider value if it's not currently being dragged
		if !uc.slider.dragging {
			uc.slider.SetValue(float64(setLevel))
		}
		uc.updateCostDisplay()
	}
}

// BonusControl represents a complete bonus control with slider, buttons, and cost display
type BonusControl struct {
	BaseMenuElement
	label           string
	bonusType       string
	territoryName   string
	currentLevel    int
	maxLevel        int
	enabled         bool // Whether the control is enabled
	slider          *MenuSlider
	decrementBtn    *MenuButton
	incrementBtn    *MenuButton
	costText        *MenuText
	refreshCallback func(string) // Callback to refresh menu when bonus changes
	parentMenu      *EdgeMenu    // Reference to parent menu for targeted updates
}

func NewBonusControl(label, bonusType, territoryName string, currentLevel int, enabled bool) *BonusControl {
	// Get max level from costs
	costs := eruntime.GetCost()
	maxLevel := getBonusMaxLevel(costs, bonusType)

	bc := &BonusControl{
		BaseMenuElement: NewBaseMenuElement(),
		label:           label,
		bonusType:       bonusType,
		territoryName:   territoryName,
		currentLevel:    currentLevel,
		maxLevel:        maxLevel,
		enabled:         enabled,
		refreshCallback: nil, // Will be set by parent menu
	}

	// Create slider
	sliderOptions := DefaultSliderOptions()
	sliderOptions.MinValue = 0
	sliderOptions.MaxValue = float64(maxLevel)
	sliderOptions.Step = 1
	sliderOptions.ShowValue = false
	sliderOptions.ValueFormat = "%.0f"
	sliderOptions.Enabled = enabled

	bc.slider = NewMenuSlider("", float64(currentLevel), sliderOptions, func(value float64) {
		if enabled {
			bc.setLevel(int(value))
		}
	})

	// Set up drag end callback to update tower stats only when dragging finishes
	bc.slider.SetOnDragEnd(func() {
		// fmt.Printf("DEBUG: BonusControl drag ended for %s\n", bc.bonusType)
		if bc.parentMenu != nil {
			bc.parentMenu.UpdateTowerStats(bc.territoryName)
			// For Tower Multi-Attack, also do a full refresh when drag ends
			if bc.bonusType == "towerMultiAttack" {
				bc.parentMenu.refreshMenuData()
			}
		}
	})

	// Create decrement button
	decrementOptions := DefaultButtonOptions()
	decrementOptions.Height = 20
	decrementOptions.BackgroundColor = color.RGBA{128, 128, 128, 255} // Grey
	decrementOptions.HoverColor = color.RGBA{148, 148, 148, 255}      // Lighter grey on hover
	decrementOptions.PressedColor = color.RGBA{108, 108, 108, 255}    // Darker grey when pressed
	decrementOptions.BorderColor = color.RGBA{168, 168, 168, 255}     // Light grey border
	decrementOptions.Enabled = enabled
	bc.decrementBtn = NewMenuButton("-", decrementOptions, func() {
		if enabled && bc.currentLevel > 0 {
			bc.setLevel(bc.currentLevel - 1)
		}
	})

	// Create increment button
	incrementOptions := DefaultButtonOptions()
	incrementOptions.Height = 20
	incrementOptions.BackgroundColor = color.RGBA{128, 128, 128, 255} // Grey
	incrementOptions.HoverColor = color.RGBA{148, 148, 148, 255}      // Lighter grey on hover
	incrementOptions.PressedColor = color.RGBA{108, 108, 108, 255}    // Darker grey when pressed
	incrementOptions.BorderColor = color.RGBA{168, 168, 168, 255}     // Light grey border
	incrementOptions.Enabled = enabled
	bc.incrementBtn = NewMenuButton("+", incrementOptions, func() {
		if enabled && bc.currentLevel < bc.maxLevel {
			bc.setLevel(bc.currentLevel + 1)
		}
	})

	// Create cost text
	bc.updateCostDisplay()

	return bc
}

func getBonusMaxLevel(costs *typedef.Costs, bonusType string) int {
	switch bonusType {
	case "strongerMinions":
		return costs.Bonuses.StrongerMinions.MaxLevel
	case "towerMultiAttack":
		return costs.Bonuses.TowerMultiAttack.MaxLevel
	case "towerAura":
		return costs.Bonuses.TowerAura.MaxLevel
	case "towerVolley":
		return costs.Bonuses.TowerVolley.MaxLevel
	case "gatheringExperience":
		return costs.Bonuses.GatheringExperience.MaxLevel
	case "mobExperience":
		return costs.Bonuses.MobExperience.MaxLevel
	case "mobDamage":
		return costs.Bonuses.MobDamage.MaxLevel
	case "pvpDamage":
		return costs.Bonuses.PvPDamage.MaxLevel
	case "xpSeeking":
		return costs.Bonuses.XPSeeking.MaxLevel
	case "tomeSeeking":
		return costs.Bonuses.TomeSeeking.MaxLevel
	case "emeraldSeeking":
		return costs.Bonuses.EmeraldsSeeking.MaxLevel
	case "largerResourceStorage":
		return costs.Bonuses.LargerResourceStorage.MaxLevel
	case "largerEmeraldStorage":
		return costs.Bonuses.LargerEmeraldsStorage.MaxLevel
	case "efficientResource":
		return costs.Bonuses.EfficientResource.MaxLevel
	case "efficientEmerald":
		return costs.Bonuses.EfficientEmeralds.MaxLevel
	case "resourceRate":
		return costs.Bonuses.ResourceRate.MaxLevel
	case "emeraldRate":
		return costs.Bonuses.EmeraldsRate.MaxLevel
	default:
		return 10 // Default fallback
	}
}

func (bc *BonusControl) setLevel(level int) {
	if level < 0 || level > bc.maxLevel {
		return
	}

	// Prevent setting Tower Multi-Attack if the 5-per-guild limit is reached (UI-side guard)
	if bc.bonusType == "towerMultiAttack" && level > 0 {
		territory := eruntime.GetTerritory(bc.territoryName)
		if territory != nil {
			guildName := territory.Guild.Name
			count := 0
			for _, t := range eruntime.GetTerritories() {
				if t != nil && t.Guild.Name == guildName && t.Options.Bonus.Set.TowerMultiAttack > 0 {
					if t.Name != territory.Name || t.Options.Bonus.Set.TowerMultiAttack > 0 {
						count++
					}
				}
			}
			if count >= 5 && territory.Options.Bonus.Set.TowerMultiAttack == 0 {
				// Show error toast if available, else just return
				// If you have a global ShowErrorToast or similar, use it here. Otherwise, just return.
				return
			}
		}
	}

	bc.currentLevel = level
	bc.slider.SetValue(float64(level))
	bc.updateCostDisplay()

	// Update the territory in eruntime
	eruntime.SetTerritoryBonus(bc.territoryName, bc.bonusType, level)

	// Update costs immediately (no visual disruption)
	if bc.parentMenu != nil {
		bc.parentMenu.UpdateTotalCosts(bc.territoryName)
		// Tower stats will be updated by the periodic timer or on mouse release
		// fmt.Printf("DEBUG: BonusControl - setLevel completed, tower stats will update periodically\n")
	}
}

func (bc *BonusControl) updateCostDisplay() {
	if !bc.enabled && bc.bonusType == "towerMultiAttack" {
		costOptions := DefaultTextOptions()
		costOptions.Color = color.RGBA{255, 80, 80, 255} // Red for error
		bc.costText = NewMenuText("Maximum Multi-Attack Reached", costOptions)
		return
	}

	if !bc.enabled && (bc.bonusType == "xpSeeking" || bc.bonusType == "tomeSeeking" || bc.bonusType == "emeraldSeeking") {
		costOptions := DefaultTextOptions()
		costOptions.Color = color.RGBA{255, 80, 80, 255} // Red for error
		var bonusName string
		switch bc.bonusType {
		case "xpSeeking":
			bonusName = "XP Seeking"
		case "tomeSeeking":
			bonusName = "Tome Seeking"
		case "emeraldSeeking":
			bonusName = "Emerald Seeking"
		}
		bc.costText = NewMenuText(fmt.Sprintf("Maximum %s Reached (8/8)", bonusName), costOptions)
		return
	}

	cost, resourceType := eruntime.GetBonusCost(bc.bonusType, bc.currentLevel)
	if cost > 0 {
		costOptions := DefaultTextOptions()
		costOptions.FontSize = 14
		costText := fmt.Sprintf("Cost: %d %s", cost, resourceType)
		bc.costText = NewMenuText(costText, costOptions)
	} else {
		bc.costText = NewMenuText("", DefaultTextOptions())
	}
}

func (bc *BonusControl) isAffordable() bool {
	territory := eruntime.GetTerritory(bc.territoryName)
	if territory == nil {
		return false
	}

	var setLevel, atLevel int
	switch bc.bonusType {
	case "strongerMinions":
		setLevel = territory.Options.Bonus.Set.StrongerMinions
		atLevel = territory.Options.Bonus.At.StrongerMinions
	case "towerMultiAttack":
		setLevel = territory.Options.Bonus.Set.TowerMultiAttack
		atLevel = territory.Options.Bonus.At.TowerMultiAttack
	case "towerAura":
		setLevel = territory.Options.Bonus.Set.TowerAura
		atLevel = territory.Options.Bonus.At.TowerAura
	case "towerVolley":
		setLevel = territory.Options.Bonus.Set.TowerVolley
		atLevel = territory.Options.Bonus.At.TowerVolley
	case "largerResourceStorage":
		setLevel = territory.Options.Bonus.Set.LargerResourceStorage
		atLevel = territory.Options.Bonus.At.LargerResourceStorage
	case "largerEmeraldStorage":
		setLevel = territory.Options.Bonus.Set.LargerEmeraldStorage
		atLevel = territory.Options.Bonus.At.LargerEmeraldStorage
	case "efficientResource":
		setLevel = territory.Options.Bonus.Set.EfficientResource
		atLevel = territory.Options.Bonus.At.EfficientResource
	case "efficientEmerald":
		setLevel = territory.Options.Bonus.Set.EfficientEmerald
		atLevel = territory.Options.Bonus.At.EfficientEmerald
	case "resourceRate":
		setLevel = territory.Options.Bonus.Set.ResourceRate
		atLevel = territory.Options.Bonus.At.ResourceRate
	case "emeraldRate":
		setLevel = territory.Options.Bonus.Set.EmeraldRate
		atLevel = territory.Options.Bonus.At.EmeraldRate
	case "xpSeeking":
		setLevel = territory.Options.Bonus.Set.XPSeeking
		atLevel = territory.Options.Bonus.At.XPSeeking
	case "tomeSeeking":
		setLevel = territory.Options.Bonus.Set.TomeSeeking
		atLevel = territory.Options.Bonus.At.TomeSeeking
	case "emeraldSeeking":
		setLevel = territory.Options.Bonus.Set.EmeraldSeeking
		atLevel = territory.Options.Bonus.At.EmeraldSeeking
	default:
		return true
	}

	// If normally affordable, return true
	if setLevel == atLevel {
		return true
	}

	// At tick :59, check with tolerance if resources are slightly insufficient
	if setLevel > 0 && atLevel == 0 {
		return eruntime.CheckBonusAffordabilityWithTolerance(bc.territoryName, bc.bonusType, setLevel)
	}

	return false
}

func (bc *BonusControl) Update(mx, my int, deltaTime float64) bool {
	if !bc.visible {
		return false
	}

	bc.updateAnimation(deltaTime)

	// For Tower Multi-Attack, recalculate enabled state in real-time to ensure limits are enforced
	if bc.bonusType == "towerMultiAttack" {
		territory := eruntime.GetTerritory(bc.territoryName)
		if territory != nil {
			territories := eruntime.GetTerritories()
			currentGuild := territory.Guild.Name
			multiAttackCount := 0
			for _, t := range territories {
				if t != nil && t.Guild.Name == currentGuild && t.Options.Bonus.Set.TowerMultiAttack > 0 {
					multiAttackCount++
				}
			}
			// Enable if we have less than 5 OR this territory already has it enabled
			bc.enabled = multiAttackCount < 5 || territory.Options.Bonus.Set.TowerMultiAttack > 0
		}
	}

	// For seeking bonuses, recalculate enabled state in real-time to ensure limits are enforced
	if bc.bonusType == "xpSeeking" || bc.bonusType == "tomeSeeking" || bc.bonusType == "emeraldSeeking" {
		territory := eruntime.GetTerritory(bc.territoryName)
		if territory != nil && territory.Guild.Name != "" {
			territories := eruntime.GetTerritories()
			currentGuild := territory.Guild.Name
			seekingCount := 0

			// Count territories with the specific seeking bonus
			for _, t := range territories {
				if t != nil && t.Guild.Name != "" && t.Guild.Name == currentGuild {
					switch bc.bonusType {
					case "xpSeeking":
						if t.Options.Bonus.Set.XPSeeking > 0 {
							seekingCount++
						}
					case "tomeSeeking":
						if t.Options.Bonus.Set.TomeSeeking > 0 {
							seekingCount++
						}
					case "emeraldSeeking":
						if t.Options.Bonus.Set.EmeraldSeeking > 0 {
							seekingCount++
						}
					}
				}
			}

			// Enable if we have less than 8 OR this territory already has it enabled
			var currentSeekingLevel int
			switch bc.bonusType {
			case "xpSeeking":
				currentSeekingLevel = territory.Options.Bonus.Set.XPSeeking
			case "tomeSeeking":
				currentSeekingLevel = territory.Options.Bonus.Set.TomeSeeking
			case "emeraldSeeking":
				currentSeekingLevel = territory.Options.Bonus.Set.EmeraldSeeking
			}
			wasEnabled := bc.enabled
			bc.enabled = seekingCount < 8 || currentSeekingLevel > 0

			// Update slider and button enabled states if they changed
			if wasEnabled != bc.enabled {
				bc.slider.options.Enabled = bc.enabled
				bc.decrementBtn.options.Enabled = bc.enabled
				bc.incrementBtn.options.Enabled = bc.enabled
			}
		} else {
			// If territory is nil or has no guild, disable the control
			bc.enabled = false
		}
	}

	// Update slider color and enabled state based on affordability and enabled state
	bc.slider.options.Enabled = bc.enabled
	bc.decrementBtn.options.Enabled = bc.enabled
	bc.incrementBtn.options.Enabled = bc.enabled
	if !bc.enabled {
		bc.slider.options.FillColor = color.RGBA{80, 80, 80, 255} // Dark grey when disabled
	} else if bc.isAffordable() {
		bc.slider.options.FillColor = color.RGBA{100, 150, 255, 255} // Blue (default)
	} else {
		bc.slider.options.FillColor = color.RGBA{128, 128, 128, 255} // Grey
	}

	// Update elements
	handled := false
	if bc.decrementBtn.Update(mx, my, deltaTime) {
		handled = true
	}
	if bc.incrementBtn.Update(mx, my, deltaTime) {
		handled = true
	}
	if bc.slider.Update(mx, my, deltaTime) {
		handled = true
	}

	return handled
}

func (bc *BonusControl) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !bc.visible || bc.displayAlpha() <= 0.01 {
		return 0
	}

	currentY := y
	rowHeight := 30

	// Draw label
	labelText := fmt.Sprintf("%s: Level %d", bc.label, bc.currentLevel)
	textColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(bc.displayAlpha()))}
	text.Draw(screen, labelText, font, x, currentY+20, textColor)
	currentY += rowHeight

	// Draw buttons and slider on the same row
	buttonWidth := 30
	sliderWidth := width - (buttonWidth * 2) - 20 // Leave space for buttons and padding
	bc.decrementBtn.SetRevealProgress(bc.displayAlpha())
	bc.incrementBtn.SetRevealProgress(bc.displayAlpha())
	bc.slider.SetRevealProgress(bc.displayAlpha())
	if bc.costText != nil {
		bc.costText.SetRevealProgress(bc.displayAlpha())
	}

	// Calculate button Y offset to center with slider bar
	// Slider bar center is at currentY + 24 (see MenuSlider.Draw), button should be centered on that
	sliderBarCenterY := 24 // From MenuSlider.Draw: sliderY = y + 24
	buttonHeight := 20     // Button height
	buttonYOffset := sliderBarCenterY - (buttonHeight / 2)

	// Draw decrement button (offset to center with slider bar)
	bc.decrementBtn.Draw(screen, x, currentY+buttonYOffset, buttonWidth, font)

	// Draw slider
	bc.slider.Draw(screen, x+buttonWidth+5, currentY, sliderWidth, font)

	// Draw increment button (offset to center with slider bar)
	bc.incrementBtn.Draw(screen, x+buttonWidth+sliderWidth+10, currentY+buttonYOffset, buttonWidth, font)

	currentY += rowHeight

	// Draw cost display
	if bc.costText != nil {
		bc.costText.Draw(screen, x+10, currentY, width-20, font)
		currentY += bc.costText.GetMinHeight()
	}

	return currentY - y + 10 // Total height plus some padding
}

func (bc *BonusControl) GetMinHeight() int {
	baseHeight := 70 // Label + controls + padding
	if bc.costText != nil {
		baseHeight += bc.costText.GetMinHeight()
	}
	return baseHeight
}

// refreshData updates the bonus control with current territory data
func (bc *BonusControl) refreshData() {
	territory := eruntime.GetTerritory(bc.territoryName)
	if territory == nil {
		return
	}

	// Get current set level for this bonus type
	var setLevel int
	switch bc.bonusType {
	case "strongerMinions":
		setLevel = territory.Options.Bonus.Set.StrongerMinions
	case "towerMultiAttack":
		setLevel = territory.Options.Bonus.Set.TowerMultiAttack
	case "towerAura":
		setLevel = territory.Options.Bonus.Set.TowerAura
	case "towerVolley":
		setLevel = territory.Options.Bonus.Set.TowerVolley
	case "largerResourceStorage":
		setLevel = territory.Options.Bonus.Set.LargerResourceStorage
	case "largerEmeraldStorage":
		setLevel = territory.Options.Bonus.Set.LargerEmeraldStorage
	case "efficientResource":
		setLevel = territory.Options.Bonus.Set.EfficientResource
	case "efficientEmerald":
		setLevel = territory.Options.Bonus.Set.EfficientEmerald
	case "resourceRate":
		setLevel = territory.Options.Bonus.Set.ResourceRate
	case "emeraldRate":
		setLevel = territory.Options.Bonus.Set.EmeraldRate
	case "xpSeeking":
		setLevel = territory.Options.Bonus.Set.XPSeeking
	case "tomeSeeking":
		setLevel = territory.Options.Bonus.Set.TomeSeeking
	case "emeraldSeeking":
		setLevel = territory.Options.Bonus.Set.EmeraldSeeking
	default:
		return
	}

	// For Tower Multi-Attack, recalculate enabled state
	if bc.bonusType == "towerMultiAttack" {
		territories := eruntime.GetTerritories()
		currentGuild := territory.Guild.Name
		multiAttackCount := 0
		for _, t := range territories {
			if t != nil && t.Guild.Name == currentGuild && t.Options.Bonus.Set.TowerMultiAttack > 0 {
				multiAttackCount++
			}
		}
		// Enable if we have less than 5 OR this territory already has it enabled
		wasEnabled := bc.enabled
		bc.enabled = multiAttackCount < 5 || territory.Options.Bonus.Set.TowerMultiAttack > 0

		// Update slider and button enabled states if they changed
		if wasEnabled != bc.enabled {
			bc.slider.options.Enabled = bc.enabled
			bc.decrementBtn.options.Enabled = bc.enabled
			bc.incrementBtn.options.Enabled = bc.enabled
		}
	}

	// For seeking bonuses, recalculate enabled state based on 8 territory limit
	if bc.bonusType == "xpSeeking" || bc.bonusType == "tomeSeeking" || bc.bonusType == "emeraldSeeking" {
		territories := eruntime.GetTerritories()
		currentGuild := territory.Guild.Name
		seekingCount := 0

		// Count territories with the specific seeking bonus
		for _, t := range territories {
			if t != nil && t.Guild.Name == currentGuild {
				switch bc.bonusType {
				case "xpSeeking":
					if t.Options.Bonus.Set.XPSeeking > 0 {
						seekingCount++
					}
				case "tomeSeeking":
					if t.Options.Bonus.Set.TomeSeeking > 0 {
						seekingCount++
					}
				case "emeraldSeeking":
					if t.Options.Bonus.Set.EmeraldSeeking > 0 {
						seekingCount++
					}
				}
			}
		}

		// Enable if we have less than 8 OR this territory already has it enabled
		wasEnabled := bc.enabled
		var currentSeekingLevel int
		switch bc.bonusType {
		case "xpSeeking":
			currentSeekingLevel = territory.Options.Bonus.Set.XPSeeking
		case "tomeSeeking":
			currentSeekingLevel = territory.Options.Bonus.Set.TomeSeeking
		case "emeraldSeeking":
			currentSeekingLevel = territory.Options.Bonus.Set.EmeraldSeeking
		}
		bc.enabled = seekingCount < 8 || currentSeekingLevel > 0

		// Update slider and button enabled states if they changed
		if wasEnabled != bc.enabled {
			bc.slider.options.Enabled = bc.enabled
			bc.decrementBtn.options.Enabled = bc.enabled
			bc.incrementBtn.options.Enabled = bc.enabled
		}
	}

	// Update current level if it changed
	if bc.currentLevel != setLevel {
		bc.currentLevel = setLevel
		// Only update slider value if it's not currently being dragged
		if !bc.slider.dragging {
			bc.slider.SetValue(float64(setLevel))
		}
		bc.updateCostDisplay()
	} else if bc.bonusType == "towerMultiAttack" || bc.bonusType == "xpSeeking" || bc.bonusType == "tomeSeeking" || bc.bonusType == "emeraldSeeking" {
		// For bonuses with limits, always update cost display to show error messages if needed
		bc.updateCostDisplay()
	}
}

// SetRefreshCallback sets the callback function to be called when bonus changes
func (bc *BonusControl) SetRefreshCallback(callback func(string)) {
	bc.refreshCallback = callback
}

// refreshMenuData updates all menu content with current territory data
func (m *EdgeMenu) refreshMenuData() {
	// Always use selective updates instead of rebuilding the entire menu
	// This prevents text inputs from losing focus

	// Update total costs if we have a current territory
	// Tower stats are now updated only via drag end callbacks to prevent interruption
	if m.currentTerritory != "" {
		m.UpdateTotalCosts(m.currentTerritory)
	}

	for _, element := range m.elements {
		if upgradeControl, ok := element.(*UpgradeControl); ok {
			upgradeControl.refreshData()
		} else if bonusControl, ok := element.(*BonusControl); ok {
			bonusControl.refreshData()
		} else if storageControl, ok := element.(*ResourceStorageControl); ok {
			storageControl.refreshData()
		} else if collapsible, ok := element.(*CollapsibleMenu); ok {
			// Refresh upgrade and bonus controls in collapsible menus
			for _, subElement := range collapsible.elements {
				if upgradeControl, ok := subElement.(*UpgradeControl); ok {
					upgradeControl.refreshData()
				} else if bonusControl, ok := subElement.(*BonusControl); ok {
					bonusControl.refreshData()
				} else if storageControl, ok := subElement.(*ResourceStorageControl); ok {
					storageControl.refreshData()
				}
			}
		}
	}
}

// UpdateTotalCosts updates only the Total Costs section with new territory data
func (m *EdgeMenu) UpdateTotalCosts(territoryName string) {
	// Get fresh territory data
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return
	}

	// Find the "Total Costs" collapsible menu and update its text elements
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			if collapsible.title == "Total Costs" {
				// Clear existing cost text elements
				collapsible.elements = collapsible.elements[:0]
				if collapsible.revealStart == nil {
					collapsible.revealStart = make(map[EdgeMenuElement]time.Time)
				}

				// Re-add updated cost text elements
				emeraldCostOptions := DefaultTextOptions()
				emeraldCostOptions.Color = color.RGBA{0, 255, 0, 255} // Green for emeralds
				emeraldText := NewMenuText(fmt.Sprintf("%d Emerald per Hour", int(territory.Costs.Emeralds)), emeraldCostOptions)
				collapsible.elements = append(collapsible.elements, emeraldText)

				oreCostOptions := DefaultTextOptions()
				oreCostOptions.Color = color.RGBA{180, 180, 180, 255} // Light grey for ores
				oreText := NewMenuText(fmt.Sprintf("%d Ore per Hour", int(territory.Costs.Ores)), oreCostOptions)
				collapsible.elements = append(collapsible.elements, oreText)

				woodCostOptions := DefaultTextOptions()
				woodCostOptions.Color = color.RGBA{139, 69, 19, 255} // Brown for wood
				woodText := NewMenuText(fmt.Sprintf("%d Wood per Hour", int(territory.Costs.Wood)), woodCostOptions)
				collapsible.elements = append(collapsible.elements, woodText)

				fishCostOptions := DefaultTextOptions()
				fishCostOptions.Color = color.RGBA{0, 150, 255, 255} // Blue for fish
				fishText := NewMenuText(fmt.Sprintf("%d Fish per Hour", int(territory.Costs.Fish)), fishCostOptions)
				collapsible.elements = append(collapsible.elements, fishText)

				cropCostOptions := DefaultTextOptions()
				cropCostOptions.Color = color.RGBA{255, 255, 0, 255} // Yellow for crops
				cropText := NewMenuText(fmt.Sprintf("%d Crop per Hour", int(territory.Costs.Crops)), cropCostOptions)
				collapsible.elements = append(collapsible.elements, cropText)

				revealDoneAt := time.Now().Add(-time.Second)
				for _, child := range collapsible.elements {
					if r, ok := child.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(1)
					}
					collapsible.revealStart[child] = revealDoneAt
				}

				return // Found and updated, exit
			}
		}
	}
}

// SetRefreshCallback sets a callback function that will be called to refresh menu content
func (m *EdgeMenu) SetRefreshCallback(callback func(string)) {
	m.refreshCallback = callback
}

// SetTerritoryNavCallback sets a callback function that will be called to navigate to a territory
func (m *EdgeMenu) SetTerritoryNavCallback(callback func(string)) {
	m.territoryNavCallback = callback
}

// SetCurrentTerritory sets the territory name that this menu is displaying
func (m *EdgeMenu) SetCurrentTerritory(territoryName string) {
	m.currentTerritory = territoryName
	// Update the modal with the current territory so it shows the correct guild totals
	if m.modal != nil {
		m.modal.SetCurrentTerritory(territoryName)
	}
}

// ClearCurrentTerritory clears the current territory (for non-territory menu usage)
func (m *EdgeMenu) ClearCurrentTerritory() {
	m.currentTerritory = ""
	// Hide the modal when clearing territory since it's territory-specific
	if m.modal != nil {
		m.modal.Hide()
	}
}

// GetCurrentTerritory returns the territory name that this menu is displaying
func (m *EdgeMenu) GetCurrentTerritory() string {
	return m.currentTerritory
}

// initClipboard initializes the clipboard for the application
func initClipboard() {
	// Only initialize clipboard if not running in WebAssembly (js/wasm)
	if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
		clipboard.Init()
	}
}

// ResourceStorageControl represents an interactive resource storage display with editable current amount
type ResourceStorageControl struct {
	BaseMenuElement
	resourceName      string
	resourceType      string
	territoryName     string
	currentValue      int
	maxValue          int
	transitValue      int
	generationPerHour int // Added generation per hour field
	color             color.RGBA
	isEditing         bool
	inputText         string
	originalValue     int
	bounds            image.Rectangle
	parentMenu        *EdgeMenu
	// Text editing state
	cursorPos      int
	selectionStart int
	selectionEnd   int
}

func NewResourceStorageControl(resourceName, resourceType, territoryName string, currentValue, maxValue, transitValue, generationPerHour int, resourceColor color.RGBA) *ResourceStorageControl {
	return &ResourceStorageControl{
		BaseMenuElement:   NewBaseMenuElement(),
		resourceName:      resourceName,
		resourceType:      resourceType,
		territoryName:     territoryName,
		currentValue:      currentValue,
		maxValue:          maxValue,
		transitValue:      transitValue,
		generationPerHour: generationPerHour,
		color:             resourceColor,
		isEditing:         false,
		inputText:         "",
		originalValue:     currentValue,
		parentMenu:        nil, // Will be set later
		cursorPos:         0,
		selectionStart:    0,
		selectionEnd:      0,
	}
}

func (rsc *ResourceStorageControl) Update(mx, my int, deltaTime float64) bool {
	if !rsc.visible {
		return false
	}

	rsc.updateAnimation(deltaTime)

	// Calculate input box area
	resourceNameWidth := 80
	inputBoxWidth := 80
	inputX := rsc.bounds.Min.X + resourceNameWidth
	inputY := rsc.bounds.Min.Y + 2
	inputHeight := 21
	inputBoxRect := image.Rect(inputX, inputY, inputX+inputBoxWidth, inputY+inputHeight)

	// Handle clicking to start editing
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= inputBoxRect.Min.X && mx < inputBoxRect.Max.X && my >= inputBoxRect.Min.Y && my < inputBoxRect.Max.Y {
			if !rsc.isEditing {
				rsc.startEditing()
				return true
			}
		} else if rsc.isEditing {
			// Clicked outside input box, finish editing
			rsc.finishEditing()
			return true
		}
	}

	// Handle keyboard input when editing
	if rsc.isEditing {
		// Check for modifier keys
		ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControl)
		shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift)

		// Handle escape key
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			rsc.cancelEditing()
			return true
		}

		// Handle enter key
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			rsc.finishEditing()
			return true
		}

		// Handle Ctrl shortcuts
		if ctrlPressed {
			if inpututil.IsKeyJustPressed(ebiten.KeyA) {
				// Ctrl+A: Select all
				rsc.selectAll()
				return true
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyC) {
				// Ctrl+C: Copy
				rsc.copyToClipboard()
				return true
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyX) {
				// Ctrl+X: Cut
				rsc.cutToClipboard()
				return true
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyV) {
				// Ctrl+V: Paste
				rsc.pasteFromClipboard()
				return true
			}
		}

		// Handle arrow keys
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if ctrlPressed {
				// Ctrl+Left: Move to start
				rsc.moveCursor(0, shiftPressed)
			} else {
				// Left: Move cursor left
				rsc.moveCursor(rsc.cursorPos-1, shiftPressed)
			}
			return true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if ctrlPressed {
				// Ctrl+Right: Move to end
				rsc.moveCursor(len(rsc.inputText), shiftPressed)
			} else {
				// Right: Move cursor right
				rsc.moveCursor(rsc.cursorPos+1, shiftPressed)
			}
			return true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
			// Home: Move to start
			rsc.moveCursor(0, shiftPressed)
			return true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnd) {
			// End: Move to end
			rsc.moveCursor(len(rsc.inputText), shiftPressed)
			return true
		}

		// Handle backspace
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if rsc.hasSelection() {
				rsc.deleteSelection()
			} else if rsc.cursorPos > 0 {
				rsc.inputText = rsc.inputText[:rsc.cursorPos-1] + rsc.inputText[rsc.cursorPos:]
				rsc.cursorPos--
				rsc.selectionStart = rsc.cursorPos
				rsc.selectionEnd = rsc.cursorPos
			}
			return true
		}

		// Handle delete key
		if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
			if rsc.hasSelection() {
				rsc.deleteSelection()
			} else if rsc.cursorPos < len(rsc.inputText) {
				rsc.inputText = rsc.inputText[:rsc.cursorPos] + rsc.inputText[rsc.cursorPos+1:]
			}
			return true
		}

		// Handle character input (more reliable than key-based input)
		inputRunes := ebiten.AppendInputChars(nil)
		for _, r := range inputRunes {
			// Only accept numeric characters and enforce length limit
			if r >= '0' && r <= '9' && len(rsc.inputText) < 6 {
				rsc.insertText(string(r))
				return true
			}
		}

		return true
	}

	return false
}

// Text manipulation helper methods for ResourceStorageControl

// hasSelection returns true if there is currently selected text
func (rsc *ResourceStorageControl) hasSelection() bool {
	return rsc.selectionStart != rsc.selectionEnd
}

// getSelectedText returns the currently selected text
func (rsc *ResourceStorageControl) getSelectedText() string {
	if !rsc.hasSelection() {
		return ""
	}
	start := rsc.selectionStart
	end := rsc.selectionEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end > len(rsc.inputText) {
		end = len(rsc.inputText)
	}
	return rsc.inputText[start:end]
}

// deleteSelection deletes the currently selected text
func (rsc *ResourceStorageControl) deleteSelection() {
	if !rsc.hasSelection() {
		return
	}
	start := rsc.selectionStart
	end := rsc.selectionEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end > len(rsc.inputText) {
		end = len(rsc.inputText)
	}

	rsc.inputText = rsc.inputText[:start] + rsc.inputText[end:]
	rsc.cursorPos = start
	rsc.selectionStart = start
	rsc.selectionEnd = start
}

// insertText inserts text at the current cursor position, replacing selection if any
func (rsc *ResourceStorageControl) insertText(text string) {
	// Delete selection first if any
	if rsc.hasSelection() {
		rsc.deleteSelection()
	}

	// Check length limit before inserting
	if len(rsc.inputText)+len(text) > 6 {
		return
	}

	// Insert new text at cursor position
	if rsc.cursorPos < 0 {
		rsc.cursorPos = 0
	}
	if rsc.cursorPos > len(rsc.inputText) {
		rsc.cursorPos = len(rsc.inputText)
	}

	rsc.inputText = rsc.inputText[:rsc.cursorPos] + text + rsc.inputText[rsc.cursorPos:]
	rsc.cursorPos += len(text)
	rsc.selectionStart = rsc.cursorPos
	rsc.selectionEnd = rsc.cursorPos
}

// selectAll selects all text
func (rsc *ResourceStorageControl) selectAll() {
	rsc.selectionStart = 0
	rsc.selectionEnd = len(rsc.inputText)
	rsc.cursorPos = len(rsc.inputText)
}

// moveCursor moves the cursor to the specified position
func (rsc *ResourceStorageControl) moveCursor(pos int, extendSelection bool) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(rsc.inputText) {
		pos = len(rsc.inputText)
	}

	rsc.cursorPos = pos
	if !extendSelection {
		rsc.selectionStart = pos
		rsc.selectionEnd = pos
	} else {
		rsc.selectionEnd = pos
	}
}

// copyToClipboard copies the selected text to clipboard
func (rsc *ResourceStorageControl) copyToClipboard() {
	if rsc.hasSelection() {
		selectedText := rsc.getSelectedText()
		if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
			clipboard.Write(clipboard.FmtText, []byte(selectedText))
		}
	}
}

// cutToClipboard cuts the selected text to clipboard
func (rsc *ResourceStorageControl) cutToClipboard() {
	if rsc.hasSelection() {
		selectedText := rsc.getSelectedText()
		if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
			clipboard.Write(clipboard.FmtText, []byte(selectedText))
		}
		rsc.deleteSelection()
	}
}

// pasteFromClipboard pastes text from clipboard
func (rsc *ResourceStorageControl) pasteFromClipboard() {
	if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
		clipboardData := clipboard.Read(clipboard.FmtText)
		if len(clipboardData) > 0 {
			// Filter to only allow numbers for resource storage
			text := string(clipboardData)
			var filteredText strings.Builder
			for _, r := range text {
				if r >= '0' && r <= '9' {
					filteredText.WriteRune(r)
				}
			}
			if filteredText.Len() > 0 {
				rsc.insertText(filteredText.String())
			}
		}
	}
}

func (rsc *ResourceStorageControl) startEditing() {
	// Unfocus any other editing controls in the parent menu
	if rsc.parentMenu != nil {
		// rsc.parentMenu.unfocusAllStorageControlsExcept(rsc)
	}

	rsc.isEditing = true
	rsc.inputText = fmt.Sprintf("%d", rsc.currentValue)
	rsc.originalValue = rsc.currentValue

	// Initialize cursor and selection

	rsc.cursorPos = len(rsc.inputText)
	rsc.selectionStart = 0
	rsc.selectionEnd = len(rsc.inputText) // Select all initially
}

func (rsc *ResourceStorageControl) finishEditing() {
	if !rsc.isEditing {
		return
	}

	newValue := rsc.originalValue
	if rsc.inputText != "" {
		if parsed, err := strconv.Atoi(rsc.inputText); err == nil && parsed >= 0 {
			newValue = parsed
		}
	}

	// Only update if value changed
	if newValue != rsc.originalValue {
		rsc.updateStorage(newValue)
		rsc.currentValue = newValue
	}

	rsc.isEditing = false
	rsc.inputText = ""
	rsc.cursorPos = 0
	rsc.selectionStart = 0
	rsc.selectionEnd = 0
}

func (rsc *ResourceStorageControl) cancelEditing() {
	// Update to current value in case it changed while editing
	territory := eruntime.GetTerritory(rsc.territoryName)
	if territory != nil {
		switch rsc.resourceType {
		case "emeralds":
			rsc.currentValue = int(territory.Storage.At.Emeralds)
		case "ores":
			rsc.currentValue = int(territory.Storage.At.Ores)
		case "wood":
			rsc.currentValue = int(territory.Storage.At.Wood)
		case "fish":
			rsc.currentValue = int(territory.Storage.At.Fish)
		case "crops":
			rsc.currentValue = int(territory.Storage.At.Crops)
		}
	}

	rsc.isEditing = false
	rsc.inputText = ""
	rsc.cursorPos = 0
	rsc.selectionStart = 0
	rsc.selectionEnd = 0
}

func (rsc *ResourceStorageControl) updateStorage(newValue int) {
	// Create new storage state
	newStorage := typedef.BasicResources{}

	// Get current territory to preserve other resource values
	territory := eruntime.GetTerritory(rsc.territoryName)
	if territory != nil {
		newStorage = territory.Storage.At
	}

	// Update the specific resource type
	switch rsc.resourceType {
	case "emeralds":
		newStorage.Emeralds = float64(newValue)
	case "ores":
		newStorage.Ores = float64(newValue)
	case "wood":
		newStorage.Wood = float64(newValue)
	case "fish":
		newStorage.Fish = float64(newValue)
	case "crops":
		newStorage.Crops = float64(newValue)
	}

	// Update storage using the ModifyStorageState function
	eruntime.ModifyStorageState(rsc.territoryName, &newStorage)
}

func (rsc *ResourceStorageControl) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !rsc.visible || rsc.displayAlpha() <= 0.01 {
		return 0
	}

	rsc.bounds = image.Rect(x, y, x+width, y+25)

	// Calculate layout
	resourceNameWidth := 80
	inputBoxWidth := 80
	spacing := 5

	// Draw resource name
	nameColor := rsc.color
	nameColor.A = uint8(float32(nameColor.A) * float32(rsc.displayAlpha()))
	text.Draw(screen, rsc.resourceName+":", font, x, y+20, nameColor)

	// Draw input box for current value
	inputX := x + resourceNameWidth
	inputY := y + 2
	inputHeight := 21

	if rsc.isEditing {
		// Draw input box background (darker when editing)
		boxColor := color.RGBA{40, 40, 50, uint8(float32(255) * float32(rsc.displayAlpha()))}
		vector.DrawFilledRect(screen, float32(inputX), float32(inputY), float32(inputBoxWidth), float32(inputHeight), boxColor, false)

		// Draw border (highlighted when editing)
		borderColor := color.RGBA{100, 150, 255, uint8(float32(255) * float32(rsc.displayAlpha()))}
		vector.StrokeRect(screen, float32(inputX), float32(inputY), float32(inputBoxWidth), float32(inputHeight), 2, borderColor, false)

		// Draw text selection background if any
		if rsc.hasSelection() {
			start := rsc.selectionStart
			end := rsc.selectionEnd
			if start > end {
				start, end = end, start
			}

			// Calculate text width up to selection start and end
			leftText := rsc.inputText[:start]
			selectedText := rsc.inputText[start:end]

			leftWidth := 0
			if len(leftText) > 0 {
				leftWidth = text.BoundString(font, leftText).Dx()
			}
			selectedWidth := 0
			if len(selectedText) > 0 {
				selectedWidth = text.BoundString(font, selectedText).Dx()
			}

			// Draw selection background
			selectionColor := color.RGBA{100, 150, 255, 100}
			vector.DrawFilledRect(screen, float32(inputX+3+leftWidth), float32(inputY+2), float32(selectedWidth), float32(inputHeight-4), selectionColor, false)
		}

		// Draw input text
		textColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(rsc.displayAlpha()))}
		text.Draw(screen, rsc.inputText, font, inputX+3, y+17, textColor)

		// Draw cursor if no selection
		if !rsc.hasSelection() {
			leftText := rsc.inputText[:rsc.cursorPos]
			cursorX := inputX + 3
			if len(leftText) > 0 {
				cursorX += text.BoundString(font, leftText).Dx()
			}

			// Draw blinking cursor
			time := float64(ebiten.ActualTPS()) * 0.1
			if int(time)%2 == 0 {
				cursorColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(rsc.displayAlpha()))}
				vector.StrokeLine(screen, float32(cursorX), float32(inputY+3), float32(cursorX), float32(inputY+inputHeight-3), 1, cursorColor, false)
			}
		}
	} else {
		// Draw input box background (normal)
		boxColor := color.RGBA{30, 30, 35, uint8(float32(200) * float32(rsc.displayAlpha()))}
		vector.DrawFilledRect(screen, float32(inputX), float32(inputY), float32(inputBoxWidth), float32(inputHeight), boxColor, false)

		// Draw border (subtle when not editing)
		borderColor := color.RGBA{80, 80, 90, uint8(float32(150) * float32(rsc.displayAlpha()))}
		vector.StrokeRect(screen, float32(inputX), float32(inputY), float32(inputBoxWidth), float32(inputHeight), 1, borderColor, false)

		// Draw current value
		valueText := fmt.Sprintf("%d", rsc.currentValue)
		textColor := color.RGBA{255, 255, 255, uint8(float32(255) * float32(rsc.displayAlpha()))}
		text.Draw(screen, valueText, font, inputX+3, y+17, textColor)
	}

	// Draw the rest of the text (max capacity and traversing)
	restX := inputX + inputBoxWidth + spacing
	restText := fmt.Sprintf("/%d (%d traversing)", rsc.maxValue, rsc.transitValue)
	restColor := color.RGBA{200, 200, 200, uint8(float32(200) * float32(rsc.displayAlpha()))}
	text.Draw(screen, restText, font, restX, y+20, restColor)

	// Draw generation per hour on the second line
	generationY := y + 44 // Increased from 40 to 44 for more spacing
	generationText := fmt.Sprintf("+%d per hour", rsc.generationPerHour)
	generationColor := color.RGBA{150, 255, 150, uint8(float32(180) * float32(rsc.displayAlpha()))} // Light green
	text.Draw(screen, generationText, font, inputX, generationY, generationColor)

	return 49 // Increased from 45 to 49 to accommodate more spacing
}

func (rsc *ResourceStorageControl) GetMinHeight() int {
	return 49 // Increased to match the new height in Draw method
}

func (rsc *ResourceStorageControl) refreshData() {
	// Don't update if currently editing
	if rsc.isEditing {
		return
	}

	territory := eruntime.GetTerritory(rsc.territoryName)
	if territory == nil {
		return
	}

	// Get territory stats for generation data
	territoryStats := eruntime.GetTerritoryStats(rsc.territoryName)
	if territoryStats == nil {
		return
	}

	// Update current and max values and generation
	switch rsc.resourceType {
	case "emeralds":
		rsc.currentValue = int(territory.Storage.At.Emeralds)
		rsc.maxValue = int(territory.Storage.Capacity.Emeralds)
		rsc.generationPerHour = int(territoryStats.CurrentGeneration.Emeralds)
	case "ores":
		rsc.currentValue = int(territory.Storage.At.Ores)
		rsc.maxValue = int(territory.Storage.Capacity.Ores)
		rsc.generationPerHour = int(territoryStats.CurrentGeneration.Ores)
	case "wood":
		rsc.currentValue = int(territory.Storage.At.Wood)
		rsc.maxValue = int(territory.Storage.Capacity.Wood)
		rsc.generationPerHour = int(territoryStats.CurrentGeneration.Wood)
	case "fish":
		rsc.currentValue = int(territory.Storage.At.Fish)
		rsc.maxValue = int(territory.Storage.Capacity.Fish)
		rsc.generationPerHour = int(territoryStats.CurrentGeneration.Fish)
	case "crops":
		rsc.currentValue = int(territory.Storage.At.Crops)
		rsc.maxValue = int(territory.Storage.Capacity.Crops)
		rsc.generationPerHour = int(territoryStats.CurrentGeneration.Crops)
	}

	// Update transit value using the new decoupled transit system
	rsc.transitValue = 0
	transitResources := eruntime.GetTransitResourcesForTerritory(territory)
	for _, transit := range transitResources {
		switch rsc.resourceType {
		case "emeralds":
			rsc.transitValue += int(transit.Emeralds)
		case "ores":
			rsc.transitValue += int(transit.Ores)
		case "wood":
			rsc.transitValue += int(transit.Wood)
		case "fish":
			rsc.transitValue += int(transit.Fish)
		case "crops":
			rsc.transitValue += int(transit.Crops)
		}
	}
}

// HasTextInputFocused checks if any MenuTextInput is currently focused
func (m *EdgeMenu) HasTextInputFocused() bool {
	for _, element := range m.elements {
		if textInput, ok := element.(*MenuTextInput); ok {
			if textInput.focused {
				return true
			}
		} else if collapsible, ok := element.(*CollapsibleMenu); ok {
			for _, subElement := range collapsible.elements {
				if textInput, ok := subElement.(*MenuTextInput); ok {
					if textInput.focused {
						return true
					}
				}
			}
		}
	}
	return false
}
