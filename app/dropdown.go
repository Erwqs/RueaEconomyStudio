package app

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Dropdown represents a dropdown selection component
type Dropdown struct {
	X, Y          int
	Width, Height int
	options       []string
	selectedIndex int
	isOpen        bool
	onSelected    func(string)

	// Visual properties
	backgroundColor color.RGBA
	borderColor     color.RGBA
	textColor       color.RGBA
	hoverColor      color.RGBA

	// Interaction state
	hoveredIndex    int
	maxVisibleItems int

	// Boundary constraints
	containerBounds image.Rectangle
	openUpward      bool
}

// NewDropdown creates a new dropdown component
func NewDropdown(x, y, width, height int, options []string, onSelected func(string)) *Dropdown {
	return &Dropdown{
		X:               x,
		Y:               y,
		Width:           width,
		Height:          height,
		options:         options,
		selectedIndex:   0,
		isOpen:          false,
		onSelected:      onSelected,
		backgroundColor: color.RGBA{40, 40, 40, 255},
		borderColor:     color.RGBA{100, 100, 100, 255},
		textColor:       color.RGBA{200, 200, 200, 255},
		hoverColor:      color.RGBA{80, 80, 80, 255},
		hoveredIndex:    -1,
		maxVisibleItems: 8,
		openUpward:      false,
	}
}

// SetContainerBounds sets the boundary constraints for the dropdown
func (d *Dropdown) SetContainerBounds(bounds image.Rectangle) {
	d.containerBounds = bounds
	d.calculateOpenDirection()
}

// calculateOpenDirection determines whether to open upward or downward
func (d *Dropdown) calculateOpenDirection() {
	if d.containerBounds.Empty() {
		d.openUpward = false
		return
	}

	// Calculate space available below and above
	spaceBelow := d.containerBounds.Max.Y - (d.Y + d.Height)
	spaceAbove := d.Y - d.containerBounds.Min.Y

	// Calculate required space for dropdown
	visibleItems := len(d.options)
	if visibleItems > d.maxVisibleItems {
		visibleItems = d.maxVisibleItems
	}
	requiredSpace := visibleItems * d.Height

	// Decide direction based on available space
	if spaceBelow >= requiredSpace {
		d.openUpward = false
	} else if spaceAbove >= requiredSpace {
		d.openUpward = true
	} else {
		// Neither direction has enough space, use the direction with more space
		// and limit the visible items
		if spaceBelow >= spaceAbove {
			d.openUpward = false
			d.maxVisibleItems = spaceBelow / d.Height
		} else {
			d.openUpward = true
			d.maxVisibleItems = spaceAbove / d.Height
		}

		// Ensure at least 2 items are visible
		if d.maxVisibleItems < 2 {
			d.maxVisibleItems = 2
		}
	}
}

// Update handles input for the dropdown
func (d *Dropdown) Update(mx, my int) {
	// Reset hover state
	d.hoveredIndex = -1

	// Check if mouse is over dropdown header
	headerBounds := image.Rect(d.X, d.Y, d.X+d.Width, d.Y+d.Height)
	mouseOverHeader := mx >= headerBounds.Min.X && mx <= headerBounds.Max.X &&
		my >= headerBounds.Min.Y && my <= headerBounds.Max.Y

	// Handle click on dropdown header
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mouseOverHeader {
			d.calculateOpenDirection() // Recalculate when opening
			d.isOpen = !d.isOpen
			return
		}

		// Check if clicking on dropdown items when open
		if d.isOpen {
			visibleItems := len(d.options)
			if visibleItems > d.maxVisibleItems {
				visibleItems = d.maxVisibleItems
			}

			for i := 0; i < visibleItems; i++ {
				var itemY int
				if d.openUpward {
					itemY = d.Y - (visibleItems-i)*d.Height
				} else {
					itemY = d.Y + d.Height + i*d.Height
				}

				itemBounds := image.Rect(d.X, itemY, d.X+d.Width, itemY+d.Height)

				if mx >= itemBounds.Min.X && mx <= itemBounds.Max.X &&
					my >= itemBounds.Min.Y && my <= itemBounds.Max.Y {
					// Item clicked
					d.selectedIndex = i
					d.isOpen = false
					if d.onSelected != nil {
						d.onSelected(d.options[i])
					}
					return
				}
			}
		}

		// Click outside dropdown, close it
		d.isOpen = false
	}

	// Handle hover over dropdown items when open
	if d.isOpen {
		visibleItems := len(d.options)
		if visibleItems > d.maxVisibleItems {
			visibleItems = d.maxVisibleItems
		}

		for i := 0; i < visibleItems; i++ {
			var itemY int
			if d.openUpward {
				itemY = d.Y - (visibleItems-i)*d.Height
			} else {
				itemY = d.Y + d.Height + i*d.Height
			}

			itemBounds := image.Rect(d.X, itemY, d.X+d.Width, itemY+d.Height)

			if mx >= itemBounds.Min.X && mx <= itemBounds.Max.X &&
				my >= itemBounds.Min.Y && my <= itemBounds.Max.Y {
				d.hoveredIndex = i
				break
			}
		}
	}
}

// Draw renders the dropdown
func (d *Dropdown) Draw(screen *ebiten.Image) {
	// Draw dropdown header
	d.drawHeader(screen)

	// Draw dropdown items if open
	if d.isOpen {
		d.drawItems(screen)
	}
}

// drawHeader draws the main dropdown header
func (d *Dropdown) drawHeader(screen *ebiten.Image) {
	// Draw background
	ebitenutil.DrawRect(screen, float64(d.X), float64(d.Y), float64(d.Width), float64(d.Height), d.backgroundColor)

	// Draw border
	ebitenutil.DrawRect(screen, float64(d.X-1), float64(d.Y-1), float64(d.Width+2), float64(d.Height+2), d.borderColor)
	ebitenutil.DrawRect(screen, float64(d.X), float64(d.Y), float64(d.Width), float64(d.Height), d.backgroundColor)

	// Draw selected text
	if d.selectedIndex >= 0 && d.selectedIndex < len(d.options) {
		font := loadWynncraftFont(14)
		text := d.options[d.selectedIndex]
		drawTextWithOffset(screen, text, font, d.X+8, d.Y+d.Height/2+4, d.textColor)
	}

	// Draw dropdown arrow
	arrowX := d.X + d.Width - 20
	arrowY := d.Y + d.Height/2
	arrow := "v" // Down arrow using regular character
	if d.isOpen {
		arrow = "^" // Up arrow using regular character
	}
	font := loadWynncraftFont(12)
	drawTextWithOffset(screen, arrow, font, arrowX, arrowY+3, d.textColor)
}

// drawItems draws the dropdown items when open
func (d *Dropdown) drawItems(screen *ebiten.Image) {
	visibleItems := len(d.options)
	if visibleItems > d.maxVisibleItems {
		visibleItems = d.maxVisibleItems
	}

	// Calculate the starting position for items
	var itemsStartY int
	if d.openUpward {
		itemsStartY = d.Y - visibleItems*d.Height
	} else {
		itemsStartY = d.Y + d.Height
	}

	// Draw background for all items
	totalHeight := visibleItems * d.Height
	ebitenutil.DrawRect(screen, float64(d.X), float64(itemsStartY), float64(d.Width), float64(totalHeight), d.backgroundColor)

	// Draw border around items
	ebitenutil.DrawRect(screen, float64(d.X-1), float64(itemsStartY-1), float64(d.Width+2), float64(totalHeight+2), d.borderColor)
	ebitenutil.DrawRect(screen, float64(d.X), float64(itemsStartY), float64(d.Width), float64(totalHeight), d.backgroundColor)

	font := loadWynncraftFont(14)

	// Draw each item
	for i := 0; i < visibleItems; i++ {
		itemY := itemsStartY + i*d.Height

		// Draw hover highlight
		if i == d.hoveredIndex {
			ebitenutil.DrawRect(screen, float64(d.X), float64(itemY), float64(d.Width), float64(d.Height), d.hoverColor)
		}

		// Draw selection highlight
		if i == d.selectedIndex {
			ebitenutil.DrawRect(screen, float64(d.X), float64(itemY), float64(d.Width), float64(d.Height),
				color.RGBA{100, 150, 255, 100})
		}

		// Draw item text
		text := d.options[i]
		drawTextWithOffset(screen, text, font, d.X+8, itemY+d.Height/2+4, d.textColor)
	}
}

// SetSelected sets the selected option by index
func (d *Dropdown) SetSelected(index int) {
	if index >= 0 && index < len(d.options) {
		d.selectedIndex = index
		if d.onSelected != nil {
			d.onSelected(d.options[index])
		}
	}
}

// GetSelected returns the currently selected option
func (d *Dropdown) GetSelected() string {
	if d.selectedIndex >= 0 && d.selectedIndex < len(d.options) {
		return d.options[d.selectedIndex]
	}
	return ""
}

// SetOptions updates the dropdown options
func (d *Dropdown) SetOptions(options []string) {
	d.options = options
	if d.selectedIndex >= len(options) {
		d.selectedIndex = 0
	}
	d.isOpen = false
}
