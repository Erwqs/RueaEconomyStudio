package app

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ColorPicker represents a color picker modal
type ColorPicker struct {
	visible       bool
	x, y          int
	width         int
	height        int
	selectedR     uint8
	selectedG     uint8
	selectedB     uint8
	hexInput      *EnhancedTextInput
	rInput        *EnhancedTextInput
	gInput        *EnhancedTextInput
	bInput        *EnhancedTextInput
	onConfirm     func(color.RGBA)
	onCancel      func()
	draggingHue   bool
	draggingSat   bool
	draggingRed   bool
	draggingGreen bool
	draggingBlue  bool
	hue           float64 // 0-360
	saturation    float64 // 0-1
	brightness    float64 // 0-1
	guildIndex    int     // Index of guild being edited
}

// Shaders for color picker rendering
var (
	colorPickerAreaShader *ebiten.Shader // HSV area (S across X, V across Y) with uniform Hue
	colorPickerHueShader  *ebiten.Shader // Hue slider gradient
	colorPickerRGBShader  *ebiten.Shader // Generic RGB slider gradient (mode-select)
)

// Shader sources (Kage DSL)
const colorPickerAreaShaderSrc = `
package main

//kage:unit pixels

var Hue float // degrees [0..360]
var OriginX float
var OriginY float
var Width float
var Height float

// Convert HSV to RGB. H in degrees [0..360], S,V in [0..1]
func hsv2rgb(h, s, v float) vec3 {
	if s <= 0.00001 {
		return vec3(v, v, v)
	}
	h = mod(h, 360.0)
	hh := h / 60.0
	i := floor(hh)
	f := hh - i
	p := v * (1.0 - s)
	q := v * (1.0 - s*f)
	t := v * (1.0 - s*(1.0 - f))

	// Branch by i in [0..5]
	if i == 0.0 {
		return vec3(v, t, p)
	}
	if i == 1.0 {
		return vec3(q, v, p)
	}
	if i == 2.0 {
		return vec3(p, v, t)
	}
	if i == 3.0 {
		return vec3(p, q, v)
	}
	if i == 4.0 {
		return vec3(t, p, v)
	}
	// i == 5
	return vec3(v, p, q)
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	// Compute local UV from absolute pixel position
	u := clamp((position.x - OriginX) / max(Width, 1.0), 0.0, 1.0)
	vpx := clamp((position.y - OriginY) / max(Height, 1.0), 0.0, 1.0)
	// S across X, V across Y (top bright -> v = 1 - y)
	s := u
	v := 1.0 - vpx
	rgb := hsv2rgb(Hue, s, v)
	return vec4(rgb, 1.0)
}
`

const colorPickerHueShaderSrc = `
package main

//kage:unit pixels

// Reuse HSV -> RGB
func hsv2rgb(h, s, v float) vec3 {
	if s <= 0.00001 {
		return vec3(v, v, v)
	}
	h = mod(h, 360.0)
	hh := h / 60.0
	i := floor(hh)
	f := hh - i
	p := v * (1.0 - s)
	q := v * (1.0 - s*f)
	t := v * (1.0 - s*(1.0 - f))

	if i == 0.0 { return vec3(v, t, p) }
	if i == 1.0 { return vec3(q, v, p) }
	if i == 2.0 { return vec3(p, v, t) }
	if i == 3.0 { return vec3(p, q, v) }
	if i == 4.0 { return vec3(t, p, v) }
	return vec3(v, p, q)
}

var OriginX float
var OriginY float
var Width float
var Height float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	u := clamp((position.x - OriginX) / max(Width, 1.0), 0.0, 1.0)
	h := u * 360.0
	rgb := hsv2rgb(h, 1.0, 1.0)
	return vec4(rgb, 1.0)
}
`

const colorPickerRGBShaderSrc = `
package main

//kage:unit pixels

// Mode: 0=R, 1=G, 2=B
var Mode float
// Base channels in [0..1]
var BaseR float
var BaseG float
var BaseB float
var OriginX float
var OriginY float
var Width float
var Height float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	intensity := clamp((position.x - OriginX) / max(Width, 1.0), 0.0, 1.0)
	r := BaseR
	g := BaseG
	b := BaseB

	if Mode < 0.5 { // R slider
		r = intensity
	} else if Mode < 1.5 { // G slider
		g = intensity
	} else { // B slider
		b = intensity
	}
	return vec4(r, g, b, 1.0)
}
`

func init() {
	// Compile shaders once; fall back to CPU drawing if any compilation fails
	var err error
	colorPickerAreaShader, err = ebiten.NewShader([]byte(colorPickerAreaShaderSrc))
	if err != nil {
		log.Println("color picker: failed to compile area shader:", err)
	}
	colorPickerHueShader, err = ebiten.NewShader([]byte(colorPickerHueShaderSrc))
	if err != nil {
		log.Println("color picker: failed to compile hue shader:", err)
	}
	colorPickerRGBShader, err = ebiten.NewShader([]byte(colorPickerRGBShaderSrc))
	if err != nil {
		log.Println("color picker: failed to compile RGB shader:", err)
	}
}

// NewColorPicker creates a new color picker
func NewColorPicker(x, y, width, height int) *ColorPicker {
	cp := &ColorPicker{
		visible:    false,
		x:          x,
		y:          y,
		width:      width,
		height:     height + 120, // Make it taller for RGB sliders
		selectedR:  255,
		selectedG:  0,
		selectedB:  0,
		hue:        0,
		saturation: 1,
		brightness: 1,
		guildIndex: -1,
	}

	// Create input fields - initial positions relative to constructor; will be re-laid on Show
	cp.hexInput = TextInput("#FF0000", x+80, y+390, 100, 25, 7)
	cp.rInput = TextInput("255", x+width-70, y+305, 50, 25, 3)
	cp.gInput = TextInput("0", x+width-70, y+330, 50, 25, 3)
	cp.bInput = TextInput("0", x+width-70, y+355, 50, 25, 3)
	cp.layoutInputs()

	cp.updateFromRGB()
	return cp
}

// Show displays the color picker for a specific guild
func (cp *ColorPicker) Show(guildIndex int, currentColor string) {
	cp.visible = true
	cp.guildIndex = guildIndex
	cp.layoutInputs()

	// Parse current color if provided
	if currentColor != "" && len(currentColor) == 7 && currentColor[0] == '#' {
		if r, g, b, ok := parseHexColor(currentColor); ok {
			cp.selectedR = r
			cp.selectedG = g
			cp.selectedB = b
			cp.updateFromRGB()
		}
	}

	cp.updateInputFields()
}

// layoutInputs repositions text inputs based on current picker origin/size.
func (cp *ColorPicker) layoutInputs() {
	if cp.hexInput != nil {
		cp.hexInput.X = cp.x + 80
		cp.hexInput.Y = cp.y + 390
	}
	if cp.rInput != nil {
		cp.rInput.X = cp.x + cp.width - 70
		cp.rInput.Y = cp.y + 305
	}
	if cp.gInput != nil {
		cp.gInput.X = cp.x + cp.width - 70
		cp.gInput.Y = cp.y + 330
	}
	if cp.bInput != nil {
		cp.bInput.X = cp.x + cp.width - 70
		cp.bInput.Y = cp.y + 355
	}
}

// Hide hides the color picker
func (cp *ColorPicker) Hide() {
	cp.visible = false
	cp.guildIndex = -1
}

// IsVisible returns true if the color picker is visible
func (cp *ColorPicker) IsVisible() bool {
	return cp.visible
}

// Update handles input for the color picker
func (cp *ColorPicker) Update() bool {
	if !cp.visible {
		return false
	}

	mx, my := ebiten.CursorPosition()

	// Handle input field focus clicks first (before any dragging)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check hex input
		hexBounds := image.Rect(cp.hexInput.X, cp.hexInput.Y, cp.hexInput.X+cp.hexInput.Width, cp.hexInput.Y+cp.hexInput.Height)
		if mx >= hexBounds.Min.X && mx < hexBounds.Max.X && my >= hexBounds.Min.Y && my < hexBounds.Max.Y {
			// If not already focused, clear the value and select all
			if !cp.hexInput.Focused {
				cp.hexInput.Value = ""
				cp.hexInput.cursorPos = 0
			}
			cp.hexInput.Focused = true
			cp.rInput.Focused = false
			cp.gInput.Focused = false
			cp.bInput.Focused = false
			return true
		}

		// Check R input
		rBounds := image.Rect(cp.rInput.X, cp.rInput.Y, cp.rInput.X+cp.rInput.Width, cp.rInput.Y+cp.rInput.Height)
		if mx >= rBounds.Min.X && mx < rBounds.Max.X && my >= rBounds.Min.Y && my < rBounds.Max.Y {
			// If not already focused, clear the value
			if !cp.rInput.Focused {
				cp.rInput.Value = ""
				cp.rInput.cursorPos = 0
			}
			cp.rInput.Focused = true
			cp.hexInput.Focused = false
			cp.gInput.Focused = false
			cp.bInput.Focused = false
			return true
		}

		// Check G input
		gBounds := image.Rect(cp.gInput.X, cp.gInput.Y, cp.gInput.X+cp.gInput.Width, cp.gInput.Y+cp.gInput.Height)
		if mx >= gBounds.Min.X && mx < gBounds.Max.X && my >= gBounds.Min.Y && my < gBounds.Max.Y {
			// If not already focused, clear the value
			if !cp.gInput.Focused {
				cp.gInput.Value = ""
				cp.gInput.cursorPos = 0
			}
			cp.gInput.Focused = true
			cp.hexInput.Focused = false
			cp.rInput.Focused = false
			cp.bInput.Focused = false
			return true
		}

		// Check B input
		bBounds := image.Rect(cp.bInput.X, cp.bInput.Y, cp.bInput.X+cp.bInput.Width, cp.bInput.Y+cp.bInput.Height)
		if mx >= bBounds.Min.X && mx < bBounds.Max.X && my >= bBounds.Min.Y && my < bBounds.Max.Y {
			// If not already focused, clear the value
			if !cp.bInput.Focused {
				cp.bInput.Value = ""
				cp.bInput.cursorPos = 0
			}
			cp.bInput.Focused = true
			cp.hexInput.Focused = false
			cp.rInput.Focused = false
			cp.gInput.Focused = false
			return true
		}

		// If clicking elsewhere, clear all input focus
		if !cp.isInsideColorPicker(mx, my) {
			cp.hexInput.Focused = false
			cp.rInput.Focused = false
			cp.gInput.Focused = false
			cp.bInput.Focused = false
		}
	}

	// Update input fields
	cp.hexInput.UpdateWithSkipInput(false)
	cp.rInput.UpdateWithSkipInput(false)
	cp.gInput.UpdateWithSkipInput(false)
	cp.bInput.UpdateWithSkipInput(false)

	// Handle hex input changes - improved validation and real-time updates
	if cp.hexInput.Focused && cp.hexInput.Value != "" {
		// Try to parse hex color with improved flexibility
		if r, g, b, ok := parseHexColor(cp.hexInput.Value); ok {
			// Only update if the values actually changed to avoid feedback loops
			if cp.selectedR != r || cp.selectedG != g || cp.selectedB != b {
				cp.selectedR = r
				cp.selectedG = g
				cp.selectedB = b
				cp.updateFromRGB()
				// Only update RGB inputs if they're not being actively edited
				if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
					cp.updateRGBInputs()
				}
			}
		}
	} else if !cp.hexInput.Focused && cp.hexInput.Value != "" && len(cp.hexInput.Value) == 7 && cp.hexInput.Value[0] == '#' {
		// Fallback for when hex input loses focus - strict validation
		if len(cp.hexInput.Value) >= 7 {
			if r, g, b, ok := parseHexColor(cp.hexInput.Value); ok {
				cp.selectedR = r
				cp.selectedG = g
				cp.selectedB = b
				cp.updateFromRGB()
				if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
					cp.updateRGBInputs()
				}
			}
		}
	}

	// Handle RGB input changes - process live updates when user is editing
	if !cp.draggingSat && !cp.draggingHue && !cp.draggingRed && !cp.draggingGreen && !cp.draggingBlue {
		changed := false

		// Process R input if focused - allow empty values or incomplete typing
		if cp.rInput.Focused && cp.rInput.Value != "" {
			if r, err := strconv.Atoi(cp.rInput.Value); err == nil && r >= 0 && r <= 255 && cp.selectedR != uint8(r) {
				cp.selectedR = uint8(r)
				changed = true
			}
		}

		// Process G input if focused - allow empty values or incomplete typing
		if cp.gInput.Focused && cp.gInput.Value != "" {
			if g, err := strconv.Atoi(cp.gInput.Value); err == nil && g >= 0 && g <= 255 && cp.selectedG != uint8(g) {
				cp.selectedG = uint8(g)
				changed = true
			}
		}

		// Process B input if focused - allow empty values or incomplete typing
		if cp.bInput.Focused && cp.bInput.Value != "" {
			if b, err := strconv.Atoi(cp.bInput.Value); err == nil && b >= 0 && b <= 255 && cp.selectedB != uint8(b) {
				cp.selectedB = uint8(b)
				changed = true
			}
		}

		// Only update HSV and hex if RGB actually changed
		if changed {
			cp.updateFromRGBPreserveHue()
			// Only update hex input if none of the RGB inputs are focused to avoid interference
			if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
				cp.updateHexInput()
			} else if !cp.hexInput.Focused {
				// Update hex if hex input is not focused but RGB inputs are being used
				cp.updateHexInput()
			}
		}
	}

	// Handle color area interaction (saturation/brightness only, don't affect hue)
	pickerX := cp.x + 20
	pickerY := cp.y + 50
	pickerWidth := cp.width - 140 // Leave space for color preview
	pickerHeight := 200

	if mx >= pickerX && mx < pickerX+pickerWidth && my >= pickerY && my < pickerY+pickerHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			cp.draggingSat = true
		}
	}

	if cp.draggingSat && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Calculate saturation and brightness from mouse position
		cp.saturation = float64(mx-pickerX) / float64(pickerWidth)
		cp.brightness = 1.0 - float64(my-pickerY)/float64(pickerHeight)

		// Clamp values
		if cp.saturation < 0 {
			cp.saturation = 0
		}
		if cp.saturation > 1 {
			cp.saturation = 1
		}
		if cp.brightness < 0 {
			cp.brightness = 0
		}
		if cp.brightness > 1 {
			cp.brightness = 1
		}

		// Only update RGB values, don't recalculate HSV (which would change hue)
		r, g, b := hsvToRGB(cp.hue, cp.saturation, cp.brightness)
		cp.selectedR = r
		cp.selectedG = g
		cp.selectedB = b
		// Update input fields in real-time during dragging (if not focused)
		if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
			cp.updateRGBInputs()
		}
		if !cp.hexInput.Focused {
			cp.updateHexInput()
		}
	}

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		// Update all input fields when dragging stops
		wasAnyDragging := cp.draggingSat || cp.draggingHue || cp.draggingRed || cp.draggingGreen || cp.draggingBlue

		cp.draggingSat = false
		cp.draggingHue = false
		cp.draggingRed = false
		cp.draggingGreen = false
		cp.draggingBlue = false

		// Update input fields after dragging stops (if no input is currently focused)
		if wasAnyDragging {
			if !cp.hexInput.Focused {
				cp.updateHexInput()
			}
			if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
				cp.updateRGBInputs()
			}
		}
	}

	// Handle hue slider
	hueSliderX := cp.x + 20
	hueSliderY := cp.y + 270
	hueSliderWidth := cp.width - 140 // Match picker width
	hueSliderHeight := 20

	if mx >= hueSliderX && mx < hueSliderX+hueSliderWidth && my >= hueSliderY && my < hueSliderY+hueSliderHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			cp.draggingHue = true
		}
	}

	if cp.draggingHue && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cp.hue = float64(mx-hueSliderX) / float64(hueSliderWidth) * 360
		if cp.hue < 0 {
			cp.hue = 0
		}
		if cp.hue > 360 {
			cp.hue = 360
		}
		cp.updateFromHSV()
		// Update input fields in real-time during hue dragging (if not focused)
		if !cp.rInput.Focused && !cp.gInput.Focused && !cp.bInput.Focused {
			cp.updateRGBInputs()
		}
		if !cp.hexInput.Focused {
			cp.updateHexInput()
		}
	}

	// Handle R slider
	rSliderX := cp.x + 20
	rSliderY := cp.y + 310
	rSliderWidth := cp.width - 140
	rSliderHeight := 15

	if mx >= rSliderX && mx < rSliderX+rSliderWidth && my >= rSliderY && my < rSliderY+rSliderHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			cp.draggingRed = true
		}
	}

	if cp.draggingRed && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		value := float64(mx-rSliderX) / float64(rSliderWidth) * 255
		if value < 0 {
			value = 0
		}
		if value > 255 {
			value = 255
		}
		cp.selectedR = uint8(value)
		cp.updateHexInput()
		// Update R input field in real-time during dragging (if not focused)
		if !cp.rInput.Focused {
			cp.rInput.Value = fmt.Sprintf("%d", cp.selectedR)
		}
	}

	// Handle G slider
	gSliderX := cp.x + 20
	gSliderY := cp.y + 335
	gSliderWidth := cp.width - 140
	gSliderHeight := 15

	if mx >= gSliderX && mx < gSliderX+gSliderWidth && my >= gSliderY && my < gSliderY+gSliderHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			cp.draggingGreen = true
		}
	}

	if cp.draggingGreen && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		value := float64(mx-gSliderX) / float64(gSliderWidth) * 255
		if value < 0 {
			value = 0
		}
		if value > 255 {
			value = 255
		}
		cp.selectedG = uint8(value)
		cp.updateHexInput()
		// Update G input field in real-time during dragging (if not focused)
		if !cp.gInput.Focused {
			cp.gInput.Value = fmt.Sprintf("%d", cp.selectedG)
		}
	}

	// Handle B slider
	bSliderX := cp.x + 20
	bSliderY := cp.y + 360
	bSliderWidth := cp.width - 140
	bSliderHeight := 15

	if mx >= bSliderX && mx < bSliderX+bSliderWidth && my >= bSliderY && my < bSliderY+bSliderHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			cp.draggingBlue = true
		}
	}

	if cp.draggingBlue && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		value := float64(mx-bSliderX) / float64(bSliderWidth) * 255
		if value < 0 {
			value = 0
		}
		if value > 255 {
			value = 255
		}
		cp.selectedB = uint8(value)
		cp.updateHexInput()
		// Update B input field in real-time during dragging (if not focused)
		if !cp.bInput.Focused {
			cp.bInput.Value = fmt.Sprintf("%d", cp.selectedB)
		}
	}

	// Handle Done button
	buttonX := cp.x + cp.width - 100
	buttonWidth := 70
	buttonHeight := 30
	buttonSpacing := 10
	doneButtonY := cp.y + 150 + buttonHeight + buttonSpacing

	if mx >= buttonX && mx < buttonX+buttonWidth && my >= doneButtonY && my < doneButtonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if cp.onConfirm != nil {
				cp.onConfirm(color.RGBA{cp.selectedR, cp.selectedG, cp.selectedB, 255})
			}
			cp.Hide()
			return true
		}
	}

	// Handle Cancel button
	cancelButtonY := cp.y + 150

	if mx >= buttonX && mx < buttonX+buttonWidth && my >= cancelButtonY && my < cancelButtonY+buttonHeight {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if cp.onCancel != nil {
				cp.onCancel()
			}
			cp.Hide()
			return true
		}
	}

	// Handle X button click
	closeButtonSize := 24
	closeButtonX := cp.x + cp.width - closeButtonSize - 15
	closeButtonY := cp.y + 15

	if mx >= closeButtonX && mx < closeButtonX+closeButtonSize &&
		my >= closeButtonY && my < closeButtonY+closeButtonSize {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if cp.onCancel != nil {
				cp.onCancel()
			}
			cp.Hide()
			return true
		}
	}

	// Handle ESC key
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if cp.onCancel != nil {
			cp.onCancel()
		}
		cp.Hide()
		return true
	}

	return true // Block input when visible
}

// Draw renders the color picker
func (cp *ColorPicker) Draw(screen *ebiten.Image) {
	if !cp.visible {
		return
	}

	// Draw background overlay
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(screenH), color.RGBA{0, 0, 0, 128}, false)

	// Draw modal background
	vector.DrawFilledRect(screen, float32(cp.x), float32(cp.y), float32(cp.width), float32(cp.height), EnhancedUIColors.ModalBackground, false)
	vector.StrokeRect(screen, float32(cp.x), float32(cp.y), float32(cp.width), float32(cp.height), 2, EnhancedUIColors.Primary, false)

	// Draw title
	font := loadWynncraftFont(18)
	if font != nil {
		text.Draw(screen, "Color Picker", font, cp.x+20, cp.y+30, EnhancedUIColors.Text)
	}

	// Draw color picker area (saturation/brightness)
	cp.drawColorArea(screen)

	// Draw hue slider
	cp.drawHueSlider(screen)

	// Draw RGB sliders
	cp.drawRGBSliders(screen)

	// Draw color preview
	cp.drawColorPreview(screen)

	// Draw input fields
	cp.hexInput.Draw(screen)
	cp.rInput.Draw(screen)
	cp.gInput.Draw(screen)
	cp.bInput.Draw(screen)

	// Draw labels
	if font != nil {
		text.Draw(screen, "Hex:", font, cp.x+20, cp.y+405, EnhancedUIColors.Text) // Positioned near hex input
	}

	// Draw buttons
	cp.drawButtons(screen)
}

// drawColorArea draws the saturation/brightness picking area
func (cp *ColorPicker) drawColorArea(screen *ebiten.Image) {
	pickerX := cp.x + 20
	pickerY := cp.y + 50
	pickerWidth := cp.width - 140 // Leave space for color preview
	pickerHeight := 200

	// Prefer shader rendering for smooth gradient
	if colorPickerAreaShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(float64(pickerX), float64(pickerY))
		op.Uniforms = map[string]interface{}{
			"Hue":     float32(cp.hue),
			"OriginX": float32(pickerX),
			"OriginY": float32(pickerY),
			"Width":   float32(pickerWidth),
			"Height":  float32(pickerHeight),
		}
		screen.DrawRectShader(pickerWidth, pickerHeight, colorPickerAreaShader, op)
	} else {
		// Fallback: CPU-drawn grid
		stepSize := 4
		for i := 0; i < pickerWidth; i += stepSize {
			for j := 0; j < pickerHeight; j += stepSize {
				saturation := float64(i) / float64(pickerWidth)
				brightness := 1.0 - float64(j)/float64(pickerHeight)

				r, g, b := hsvToRGB(cp.hue, saturation, brightness)
				fillColor := color.RGBA{r, g, b, 255}

				vector.DrawFilledRect(screen,
					float32(pickerX+i), float32(pickerY+j),
					float32(stepSize), float32(stepSize),
					fillColor, false)
			}
		}
	}

	// Draw border
	vector.StrokeRect(screen, float32(pickerX), float32(pickerY), float32(pickerWidth), float32(pickerHeight), 1, EnhancedUIColors.Border, false)

	// Draw cursor for current selection
	cursorX := pickerX + int(cp.saturation*float64(pickerWidth))
	cursorY := pickerY + int((1.0-cp.brightness)*float64(pickerHeight))

	// Clamp cursor position to picker bounds
	if cursorX < pickerX {
		cursorX = pickerX
	}
	if cursorX >= pickerX+pickerWidth {
		cursorX = pickerX + pickerWidth - 1
	}
	if cursorY < pickerY {
		cursorY = pickerY
	}
	if cursorY >= pickerY+pickerHeight {
		cursorY = pickerY + pickerHeight - 1
	}

	vector.StrokeRect(screen, float32(cursorX-3), float32(cursorY-3), 6, 6, 2, color.RGBA{255, 255, 255, 255}, false)
}

// drawHueSlider draws the hue selection slider
func (cp *ColorPicker) drawHueSlider(screen *ebiten.Image) {
	sliderX := cp.x + 20
	sliderY := cp.y + 270
	sliderWidth := cp.width - 140 // Match picker width
	sliderHeight := 20

	// Draw hue gradient with shader if available
	if colorPickerHueShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(float64(sliderX), float64(sliderY))
		op.Uniforms = map[string]interface{}{
			"OriginX": float32(sliderX),
			"OriginY": float32(sliderY),
			"Width":   float32(sliderWidth),
			"Height":  float32(sliderHeight),
		}
		screen.DrawRectShader(sliderWidth, sliderHeight, colorPickerHueShader, op)
	} else {
		// Fallback CPU gradient
		stepSize := 2
		for i := 0; i < sliderWidth; i += stepSize {
			hue := float64(i) / float64(sliderWidth) * 360
			r, g, b := hsvToRGB(hue, 1.0, 1.0)
			fillColor := color.RGBA{r, g, b, 255}
			vector.DrawFilledRect(screen, float32(sliderX+i), float32(sliderY), float32(stepSize), float32(sliderHeight), fillColor, false)
		}
	}

	// Draw border
	vector.StrokeRect(screen, float32(sliderX), float32(sliderY), float32(sliderWidth), float32(sliderHeight), 1, EnhancedUIColors.Border, false)

	// Draw cursor for current hue
	cursorX := sliderX + int(cp.hue/360*float64(sliderWidth))
	vector.StrokeRect(screen, float32(cursorX-2), float32(sliderY-2), 4, float32(sliderHeight+4), 2, color.RGBA{255, 255, 255, 255}, false)
}

// drawRGBSliders draws the RGB sliders
func (cp *ColorPicker) drawRGBSliders(screen *ebiten.Image) {
	font := loadWynncraftFont(14)
	sliderWidth := cp.width - 140
	sliderHeight := 15

	// Red slider
	rSliderX := cp.x + 20
	rSliderY := cp.y + 310

	// Draw red gradient (shader preferred)
	if colorPickerRGBShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(float64(rSliderX), float64(rSliderY))
		op.Uniforms = map[string]interface{}{
			"Mode":    float32(0),
			"BaseR":   float32(cp.selectedR) / 255.0,
			"BaseG":   float32(cp.selectedG) / 255.0,
			"BaseB":   float32(cp.selectedB) / 255.0,
			"OriginX": float32(rSliderX),
			"OriginY": float32(rSliderY),
			"Width":   float32(sliderWidth),
			"Height":  float32(sliderHeight),
		}
		screen.DrawRectShader(sliderWidth, sliderHeight, colorPickerRGBShader, op)
	} else {
		for i := 0; i < sliderWidth; i += 2 {
			intensity := uint8(float64(i) / float64(sliderWidth) * 255)
			fillColor := color.RGBA{intensity, cp.selectedG, cp.selectedB, 255}
			vector.DrawFilledRect(screen, float32(rSliderX+i), float32(rSliderY), 2, float32(sliderHeight), fillColor, false)
		}
	}

	// Draw border and cursor
	vector.StrokeRect(screen, float32(rSliderX), float32(rSliderY), float32(sliderWidth), float32(sliderHeight), 1, EnhancedUIColors.Border, false)
	cursorX := rSliderX + int(float64(cp.selectedR)/255*float64(sliderWidth))
	vector.StrokeRect(screen, float32(cursorX-2), float32(rSliderY-2), 4, float32(sliderHeight+4), 2, color.RGBA{255, 255, 255, 255}, false)

	// Draw label
	if font != nil {
		text.Draw(screen, "R", font, rSliderX+sliderWidth+10, rSliderY+12, EnhancedUIColors.Text)
	}

	// Green slider
	gSliderX := cp.x + 20
	gSliderY := cp.y + 335

	// Draw green gradient
	if colorPickerRGBShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(float64(gSliderX), float64(gSliderY))
		op.Uniforms = map[string]interface{}{
			"Mode":    float32(1),
			"BaseR":   float32(cp.selectedR) / 255.0,
			"BaseG":   float32(cp.selectedG) / 255.0,
			"BaseB":   float32(cp.selectedB) / 255.0,
			"OriginX": float32(gSliderX),
			"OriginY": float32(gSliderY),
			"Width":   float32(sliderWidth),
			"Height":  float32(sliderHeight),
		}
		screen.DrawRectShader(sliderWidth, sliderHeight, colorPickerRGBShader, op)
	} else {
		for i := 0; i < sliderWidth; i += 2 {
			intensity := uint8(float64(i) / float64(sliderWidth) * 255)
			fillColor := color.RGBA{cp.selectedR, intensity, cp.selectedB, 255}
			vector.DrawFilledRect(screen, float32(gSliderX+i), float32(gSliderY), 2, float32(sliderHeight), fillColor, false)
		}
	}

	// Draw border and cursor
	vector.StrokeRect(screen, float32(gSliderX), float32(gSliderY), float32(sliderWidth), float32(sliderHeight), 1, EnhancedUIColors.Border, false)
	cursorX = gSliderX + int(float64(cp.selectedG)/255*float64(sliderWidth))
	vector.StrokeRect(screen, float32(cursorX-2), float32(gSliderY-2), 4, float32(sliderHeight+4), 2, color.RGBA{255, 255, 255, 255}, false)

	// Draw label
	if font != nil {
		text.Draw(screen, "G", font, gSliderX+sliderWidth+10, gSliderY+12, EnhancedUIColors.Text)
	}

	// Blue slider
	bSliderX := cp.x + 20
	bSliderY := cp.y + 360

	// Draw blue gradient
	if colorPickerRGBShader != nil {
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(float64(bSliderX), float64(bSliderY))
		op.Uniforms = map[string]interface{}{
			"Mode":    float32(2),
			"BaseR":   float32(cp.selectedR) / 255.0,
			"BaseG":   float32(cp.selectedG) / 255.0,
			"BaseB":   float32(cp.selectedB) / 255.0,
			"OriginX": float32(bSliderX),
			"OriginY": float32(bSliderY),
			"Width":   float32(sliderWidth),
			"Height":  float32(sliderHeight),
		}
		screen.DrawRectShader(sliderWidth, sliderHeight, colorPickerRGBShader, op)
	} else {
		for i := 0; i < sliderWidth; i += 2 {
			intensity := uint8(float64(i) / float64(sliderWidth) * 255)
			fillColor := color.RGBA{cp.selectedR, cp.selectedG, intensity, 255}
			vector.DrawFilledRect(screen, float32(bSliderX+i), float32(bSliderY), 2, float32(sliderHeight), fillColor, false)
		}
	}

	// Draw border and cursor
	vector.StrokeRect(screen, float32(bSliderX), float32(bSliderY), float32(sliderWidth), float32(sliderHeight), 1, EnhancedUIColors.Border, false)
	cursorX = bSliderX + int(float64(cp.selectedB)/255*float64(sliderWidth))
	vector.StrokeRect(screen, float32(cursorX-2), float32(bSliderY-2), 4, float32(sliderHeight+4), 2, color.RGBA{255, 255, 255, 255}, false)

	// Draw label
	if font != nil {
		text.Draw(screen, "B", font, bSliderX+sliderWidth+10, bSliderY+12, EnhancedUIColors.Text)
	}
}

// drawColorPreview draws the selected color preview
func (cp *ColorPicker) drawColorPreview(screen *ebiten.Image) {
	previewX := cp.x + cp.width - 100 // Move further right
	previewY := cp.y + 50
	previewSize := 80 // Make it bigger

	// Draw preview square
	previewColor := color.RGBA{cp.selectedR, cp.selectedG, cp.selectedB, 255}
	vector.DrawFilledRect(screen, float32(previewX), float32(previewY), float32(previewSize), float32(previewSize), previewColor, false)
	vector.StrokeRect(screen, float32(previewX), float32(previewY), float32(previewSize), float32(previewSize), 1, EnhancedUIColors.Border, false)
}

// drawButtons draws the Cancel and Done buttons and X button
func (cp *ColorPicker) drawButtons(screen *ebiten.Image) {
	font := loadWynncraftFont(16)
	if font == nil {
		return
	}

	mx, my := ebiten.CursorPosition()

	// Draw X button (top right corner)
	closeButtonSize := 24
	closeButtonX := cp.x + cp.width - closeButtonSize - 15
	closeButtonY := cp.y + 15

	closeButtonColor := EnhancedUIColors.RemoveButton
	if mx >= closeButtonX && mx < closeButtonX+closeButtonSize &&
		my >= closeButtonY && my < closeButtonY+closeButtonSize {
		closeButtonColor = EnhancedUIColors.RemoveHover
	}

	vector.DrawFilledRect(screen, float32(closeButtonX), float32(closeButtonY), float32(closeButtonSize), float32(closeButtonSize), closeButtonColor, false)
	text.Draw(screen, "x", font, closeButtonX+8, closeButtonY+16, EnhancedUIColors.Text)

	// Position buttons below the color preview, stacked vertically
	buttonX := cp.x + cp.width - 100
	buttonWidth := 70
	buttonHeight := 30
	buttonSpacing := 10

	// Cancel button (top)
	cancelButtonY := cp.y + 150
	cancelColor := EnhancedUIColors.ButtonRed
	if mx >= buttonX && mx < buttonX+buttonWidth && my >= cancelButtonY && my < cancelButtonY+buttonHeight {
		cancelColor = color.RGBA{200, 80, 80, 255} // Lighter red on hover
	}

	vector.DrawFilledRect(screen, float32(buttonX), float32(cancelButtonY), float32(buttonWidth), float32(buttonHeight), cancelColor, false)
	vector.StrokeRect(screen, float32(buttonX), float32(cancelButtonY), float32(buttonWidth), float32(buttonHeight), 1, EnhancedUIColors.Border, false)
	text.Draw(screen, "Cancel", font, buttonX+12, cancelButtonY+20, EnhancedUIColors.Text)

	// Done button (bottom)
	doneButtonY := cancelButtonY + buttonHeight + buttonSpacing
	doneColor := EnhancedUIColors.ButtonGreen
	if mx >= buttonX && mx < buttonX+buttonWidth && my >= doneButtonY && my < doneButtonY+buttonHeight {
		doneColor = EnhancedUIColors.ButtonHover
	}

	vector.DrawFilledRect(screen, float32(buttonX), float32(doneButtonY), float32(buttonWidth), float32(buttonHeight), doneColor, false)
	vector.StrokeRect(screen, float32(buttonX), float32(doneButtonY), float32(buttonWidth), float32(buttonHeight), 1, EnhancedUIColors.BorderGreen, false)
	text.Draw(screen, "Done", font, buttonX+20, doneButtonY+20, EnhancedUIColors.Text)
}

// Helper functions
func (cp *ColorPicker) updateFromRGB() {
	r := float64(cp.selectedR) / 255.0
	g := float64(cp.selectedG) / 255.0
	b := float64(cp.selectedB) / 255.0

	newHue, newSaturation, newBrightness := rgbToHSV(r, g, b)

	// Preserve hue when saturation or brightness is very low to prevent jumping
	// Use a more generous threshold to improve stability
	if newSaturation < 0.05 || newBrightness < 0.05 {
		// Keep the current hue, only update saturation and brightness
		cp.saturation = newSaturation
		cp.brightness = newBrightness
	} else {
		// Normal update - saturation is high enough for reliable hue calculation
		cp.hue = newHue
		cp.saturation = newSaturation
		cp.brightness = newBrightness
	}
}

func (cp *ColorPicker) updateFromRGBPreserveHue() {
	r := float64(cp.selectedR) / 255.0
	g := float64(cp.selectedG) / 255.0
	b := float64(cp.selectedB) / 255.0

	newHue, newSat, newBright := rgbToHSV(r, g, b)

	// Only update hue if saturation is high enough (avoid unstable hue in gray/black areas)
	if newSat > 0.1 || (cp.selectedR == cp.selectedG && cp.selectedG == cp.selectedB) {
		cp.hue = newHue
	}

	cp.saturation = newSat
	cp.brightness = newBright
}

func (cp *ColorPicker) updateFromHSV() {
	r, g, b := hsvToRGB(cp.hue, cp.saturation, cp.brightness)
	cp.selectedR = r
	cp.selectedG = g
	cp.selectedB = b
	cp.updateInputFields()
}

func (cp *ColorPicker) updateInputFields() {
	cp.updateHexInput()
	cp.updateRGBInputs()
}

func (cp *ColorPicker) updateHexInput() {
	cp.hexInput.Value = fmt.Sprintf("#%02X%02X%02X", cp.selectedR, cp.selectedG, cp.selectedB)
}

func (cp *ColorPicker) updateRGBInputs() {
	// Only update RGB inputs if they're not currently being edited by the user
	if !cp.rInput.Focused {
		cp.rInput.Value = fmt.Sprintf("%d", cp.selectedR)
	}
	if !cp.gInput.Focused {
		cp.gInput.Value = fmt.Sprintf("%d", cp.selectedG)
	}
	if !cp.bInput.Focused {
		cp.bInput.Value = fmt.Sprintf("%d", cp.selectedB)
	}
}

// Color conversion functions
func hsvToRGB(hue, saturation, brightness float64) (uint8, uint8, uint8) {
	if saturation == 0 {
		v := uint8(brightness * 255)
		return v, v, v
	}

	hue = hue / 60.0
	i := math.Floor(hue)
	f := hue - i
	p := brightness * (1 - saturation)
	q := brightness * (1 - saturation*f)
	t := brightness * (1 - saturation*(1-f))

	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = brightness, t, p
	case 1:
		r, g, b = q, brightness, p
	case 2:
		r, g, b = p, brightness, t
	case 3:
		r, g, b = p, q, brightness
	case 4:
		r, g, b = t, p, brightness
	case 5:
		r, g, b = brightness, p, q
	}

	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}

func rgbToHSV(r, g, b float64) (float64, float64, float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	// Brightness (Value)
	v := max

	// Saturation
	var s float64
	if max != 0 {
		s = delta / max
	}

	// Hue
	var h float64
	if delta != 0 {
		switch max {
		case r:
			h = 60 * (math.Mod((g-b)/delta, 6))
		case g:
			h = 60 * ((b-r)/delta + 2)
		case b:
			h = 60 * ((r-g)/delta + 4)
		}
	}

	if h < 0 {
		h += 360
	}

	return h, s, v
}

func parseHexColor(input string) (uint8, uint8, uint8, bool) {
	if input == "" {
		return 0, 0, 0, false
	}

	// Remove any whitespace
	input = strings.TrimSpace(input)

	// Convert to uppercase for consistency
	input = strings.ToUpper(input)

	var hex string

	// Handle different formats
	if strings.HasPrefix(input, "#") {
		hex = input[1:]
	} else if strings.HasPrefix(input, "0X") {
		hex = input[2:]
	} else {
		hex = input
	}

	// Check if we have exactly 6 hex characters
	if len(hex) != 6 {
		return 0, 0, 0, false
	}

	// Validate that all characters are hex digits
	for _, r := range hex {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F')) {
			return 0, 0, 0, false
		}
	}

	r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
	g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
	b, err3 := strconv.ParseUint(hex[4:6], 16, 8)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}

	return uint8(r), uint8(g), uint8(b), true
}

// isInsideColorPicker checks if the given coordinates are inside the color picker area
func (cp *ColorPicker) isInsideColorPicker(mx, my int) bool {
	return mx >= cp.x && mx < cp.x+cp.width && my >= cp.y && my < cp.y+cp.height
}
