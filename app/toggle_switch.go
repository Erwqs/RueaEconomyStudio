// Toggle switch implementation for Wynncraft RueaES
package app

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// drawToggleSwitch draws a toggle switch with two options and returns the click area
func drawToggleSwitch(
	screen *ebiten.Image,
	x, y int,
	width, height int,
	options []string,
	currentValue string,
	font font.Face,
	contentOffset int,
	mx, my int, // Mouse coordinates - use -1, -1 to disable hover effects
) image.Rectangle {
	// Create the overall rectangle for the toggle switch
	toggleRect := image.Rect(x, y, x+width, y+height)

	// Calculate width for each option (dividing the total width between the options)
	optionWidth := width / 2

	// Calculate positions for each option
	option1X := x
	option2X := x + optionWidth

	// Draw the base background (darker)
	baseColor := color.RGBA{40, 40, 60, 255}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), baseColor, false)

	// Determine which option is active
	option1Active := currentValue == options[0]
	option2Active := currentValue == options[1]

	// Draw the first option (left side)
	var option1Color color.Color
	if option1Active {
		option1Color = color.RGBA{80, 120, 200, 255} // Highlighted when active
	} else {
		option1Color = color.RGBA{60, 60, 80, 255} // Normal background when inactive
	}

	// Check if option 1 is hovered (only if valid mouse coordinates are provided)
	option1Hovered := mx >= 0 && my >= 0 && mx >= option1X && mx <= option1X+optionWidth &&
		my >= y && my <= y+height && !option1Active
	if option1Hovered {
		option1Color = color.RGBA{70, 70, 100, 255} // Subtle highlight on hover
	}

	// Draw option 1 background
	vector.DrawFilledRect(screen, float32(option1X), float32(y), float32(optionWidth), float32(height), option1Color, false)

	// Draw the second option (right side)
	var option2Color color.Color
	if option2Active {
		option2Color = color.RGBA{80, 120, 200, 255} // Highlighted when active
	} else {
		option2Color = color.RGBA{60, 60, 80, 255} // Normal background when inactive
	}

	// Check if option 2 is hovered (only if valid mouse coordinates are provided)
	option2Hovered := mx >= 0 && my >= 0 && mx >= option2X && mx <= option2X+optionWidth &&
		my >= y && my <= y+height && !option2Active
	if option2Hovered {
		option2Color = color.RGBA{70, 70, 100, 255} // Subtle highlight on hover
	}

	// Draw option 2 background
	vector.DrawFilledRect(screen, float32(option2X), float32(y), float32(optionWidth), float32(height), option2Color, false)

	// Draw divider between options
	dividerColor := color.RGBA{30, 30, 40, 255}
	vector.DrawFilledRect(screen, float32(option2X-1), float32(y+2), 2, float32(height-4), dividerColor, false)

	// Draw option 1 text
	var option1TextColor color.Color
	if option1Active {
		option1TextColor = color.White
	} else {
		option1TextColor = color.RGBA{180, 180, 180, 255}
	}
	option1Text := options[0]
	// Calculate text width using text measurement function
	option1TextBounds := text.BoundString(font, option1Text)
	option1TextWidth := option1TextBounds.Dx()
	option1TextX := option1X + (optionWidth-option1TextWidth)/2
	option1TextY := y + (height-16)/2
	drawTextOffset(screen, option1Text, font, option1TextX, option1TextY+contentOffset, option1TextColor)

	// Draw option 2 text
	var option2TextColor color.Color
	if option2Active {
		option2TextColor = color.White
	} else {
		option2TextColor = color.RGBA{180, 180, 180, 255}
	}
	option2Text := options[1]
	// Calculate text width using text measurement function
	option2TextBounds := text.BoundString(font, option2Text)
	option2TextWidth := option2TextBounds.Dx()
	option2TextX := option2X + (optionWidth-option2TextWidth)/2
	option2TextY := y + (height-16)/2
	drawTextOffset(screen, option2Text, font, option2TextX, option2TextY+contentOffset, option2TextColor)

	// Draw border around the entire toggle
	borderColor := color.RGBA{80, 80, 120, 255}
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 1, borderColor, false)

	return toggleRect
}

// handleToggleSwitchClick handles clicks on toggle switches
func handleToggleSwitchClick(
	mx, my int,
	toggleRect image.Rectangle,
	options []string,
	currentValue string,
) (bool, string) {
	// If click is not within toggle area, return false with unchanged value
	if mx < toggleRect.Min.X || mx > toggleRect.Max.X ||
		my < toggleRect.Min.Y || my > toggleRect.Max.Y {
		return false, currentValue
	}

	// Calculate option widths and positions
	optionWidth := (toggleRect.Max.X - toggleRect.Min.X) / 2
	option1X := toggleRect.Min.X
	option2X := toggleRect.Min.X + optionWidth

	// Check which option was clicked
	if mx >= option1X && mx < option2X {
		// First option clicked
		if currentValue != options[0] {
			return true, options[0]
		}
	} else {
		// Second option clicked
		if currentValue != options[1] {
			return true, options[1]
		}
	}

	// No change made
	return false, currentValue
}

// drawAnimatedToggleSwitch draws a toggle switch with smooth sliding animation
func drawAnimatedToggleSwitch(
	screen *ebiten.Image,
	x, y int,
	width, height int,
	options []string,
	currentValue string,
	animatedPosition float64, // 0.0 = first option, 1.0 = second option
	font font.Face,
	contentOffset int,
) image.Rectangle {
	// Create the overall rectangle for the toggle switch
	toggleRect := image.Rect(x, y, x+width, y+height)

	// Draw the base background (darker)
	baseColor := color.RGBA{40, 40, 60, 255}
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), baseColor, false)

	// Calculate sliding indicator position based on animation
	indicatorWidth := width / 2
	indicatorX := float32(x) + float32(animatedPosition)*float32(width-indicatorWidth)

	// Draw sliding indicator background
	indicatorColor := color.RGBA{80, 120, 200, 255}
	vector.DrawFilledRect(screen, indicatorX, float32(y), float32(indicatorWidth), float32(height), indicatorColor, false)

	// Calculate positions for each option text
	option1X := x + width/4
	option2X := x + 3*width/4

	// Draw option 1 text
	var option1TextColor color.Color
	if animatedPosition < 0.5 {
		// Interpolate between active and inactive colors
		alpha := (0.5 - animatedPosition) * 2 // 0.0 to 1.0
		option1TextColor = color.RGBA{
			R: uint8(180 + 75*alpha), // 180 to 255
			G: uint8(180 + 75*alpha), // 180 to 255
			B: uint8(180 + 75*alpha), // 180 to 255
			A: 255,
		}
	} else {
		option1TextColor = color.RGBA{180, 180, 180, 255}
	}

	option1Text := options[0]
	option1TextBounds := text.BoundString(font, option1Text)
	option1TextWidth := option1TextBounds.Dx()
	option1TextX := option1X - option1TextWidth/2
	option1TextY := y + (height-16)/2
	drawTextOffset(screen, option1Text, font, option1TextX, option1TextY+contentOffset, option1TextColor)

	// Draw option 2 text
	var option2TextColor color.Color
	if animatedPosition > 0.5 {
		// Interpolate between inactive and active colors
		alpha := (animatedPosition - 0.5) * 2 // 0.0 to 1.0
		option2TextColor = color.RGBA{
			R: uint8(180 + 75*alpha), // 180 to 255
			G: uint8(180 + 75*alpha), // 180 to 255
			B: uint8(180 + 75*alpha), // 180 to 255
			A: 255,
		}
	} else {
		option2TextColor = color.RGBA{180, 180, 180, 255}
	}

	option2Text := options[1]
	option2TextBounds := text.BoundString(font, option2Text)
	option2TextWidth := option2TextBounds.Dx()
	option2TextX := option2X - option2TextWidth/2
	option2TextY := y + (height-16)/2
	drawTextOffset(screen, option2Text, font, option2TextX, option2TextY+contentOffset, option2TextColor)

	// Draw border around the entire toggle
	borderColor := color.RGBA{80, 80, 120, 255}
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 1, borderColor, false)

	return toggleRect
}
