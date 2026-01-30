package app

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

var cardCounter int

func generateCardID() string {
	cardCounter++
	return fmt.Sprintf("card_%d", cardCounter)
}

// Card represents a container of UI elements styled as a card.
// Implements EdgeMenuElement for use inside EdgeMenus.
type Card struct {
	BaseMenuElement
	id               string
	rect             image.Rectangle
	elements         []EdgeMenuElement
	inline           bool
	hoverCallback    func(*Card)
	notHoverCallback func(*Card)
	onClickCallback  func(*Card)
	backgroundColor  color.RGBA
	borderColor      color.RGBA
	padding          int
	margin           int
}

// NewCard creates a new card UI element.
func NewCard() *Card {
	return &Card{
		BaseMenuElement: NewBaseMenuElement(),
		id:              generateCardID(),
		elements:        []EdgeMenuElement{},
		backgroundColor: color.RGBA{40, 40, 55, 240},  // Dark blue background
		borderColor:     color.RGBA{80, 80, 255, 200}, // Blue border
		padding:         1,                            // Ultra small padding
		margin:          1,                            // Ultra small margin
		inline:          false,
	}
}

// ID returns the unique identifier of the card.
func (c *Card) ID() string {
	return c.id
}

// Inline sets whether components are laid out horizontally.
func (c *Card) Inline(flag bool) *Card {
	c.inline = flag
	return c
}

// OnHover sets a callback when the cursor enters the card.
func (c *Card) OnHover(f func(*Card)) *Card {
	c.hoverCallback = f
	return c
}

// OnNotHovered sets a callback when the cursor leaves the card.
func (c *Card) OnNotHovered(f func(*Card)) *Card {
	c.notHoverCallback = f
	return c
}

// SetBackgroundColor sets the card's background color
func (c *Card) SetBackgroundColor(bg color.RGBA) *Card {
	c.backgroundColor = bg
	return c
}

// SetBorderColor sets the card's border color
func (c *Card) SetBorderColor(border color.RGBA) *Card {
	c.borderColor = border
	return c
}

// Text adds a text element to the card with compact styling.
func (c *Card) Text(textStr string, options TextOptions) *Card {
	// Make text extremely compact
	options.Height = 12 // Even smaller height
	t := NewMenuText(textStr, options)
	c.elements = append(c.elements, t)
	return c
}

// Checkbox adds a checkbox element to the card.
func (c *Card) Checkbox(label string, checked bool, options CheckboxOptions, callback func(bool)) *Card {
	chk := NewMenuCheckbox(label, checked, options, callback)
	c.elements = append(c.elements, chk)
	return c
}

// Button adds a button element to the card with compact styling.
func (c *Card) Button(textStr string, options ButtonOptions, callback func()) *Card {
	// Make button extremely compact
	options.Height = 16 // Even smaller height
	b := NewMenuButton(textStr, options, callback)
	c.elements = append(c.elements, b)
	return c
}

// Slider adds a slider element to the card.
func (c *Card) Slider(label string, initial float64, options SliderOptions, callback func(float64)) *Card {
	s := NewMenuSlider(label, initial, options, callback)
	c.elements = append(c.elements, s)
	return c
}

// TextInput adds a text input element to the card.
func (c *Card) TextInput(initial string, options TextInputOptions, onChange func(string)) *Card {
	// Use MenuTextInput from edge_menu_extensions
	ti := NewMenuTextInput("", initial, options, onChange)
	c.elements = append(c.elements, ti)
	return c
}

// Update processes input and animations for the card and its children.
func (c *Card) Update(mx, my int, deltaTime float64) bool {
	// Always update animation
	c.updateAnimation(deltaTime)

	// If not visible or animation progress is too low, don't process input
	if !c.IsVisible() || c.displayAlpha() < 0.01 {
		return false
	}

	// Handle hover callbacks
	hovered := mx >= c.rect.Min.X && mx < c.rect.Max.X && my >= c.rect.Min.Y && my < c.rect.Max.Y
	if hovered && c.hoverCallback != nil {
		c.hoverCallback(c)
	}
	if !hovered && c.notHoverCallback != nil {
		c.notHoverCallback(c)
	}

	// Update child elements
	handled := false
	for _, el := range c.elements {
		if el.IsVisible() && el.Update(mx, my, deltaTime) {
			handled = true
			break
		}
	}

	return handled
}

// Draw renders the card background, border, and child elements, returning the height used.
func (c *Card) Draw(screen *ebiten.Image, x, y, width int, fontFace font.Face) int {
	// If not visible or animation progress is too low, don't draw
	if !c.IsVisible() || c.displayAlpha() < 0.01 {
		return 0
	}

	alpha := float32(c.displayAlpha())

	// Calculate content height first
	contentHeight := 0
	if c.inline {
		// Horizontal layout - find the tallest element
		maxHeight := 0
		for _, el := range c.elements {
			if el.IsVisible() {
				h := el.GetMinHeight()
				if h > maxHeight {
					maxHeight = h
				}
			}
		}
		contentHeight = maxHeight
	} else {
		// Vertical layout - sum all heights
		for _, el := range c.elements {
			if el.IsVisible() {
				contentHeight += el.GetMinHeight() + 3 // Increased from 1 to 3 pixels for better spacing
			}
		}
		if contentHeight > 0 {
			contentHeight -= 3 // Remove last spacing
		}
	}

	// Total card height including padding
	totalHeight := contentHeight + (c.padding * 2)
	if totalHeight < 8 {
		totalHeight = 8 // Ultra small minimum height
	}

	// Apply alpha to colors
	bg := c.backgroundColor
	bg.A = uint8(float32(bg.A) * alpha)
	border := c.borderColor
	border.A = uint8(float32(border.A) * alpha)

	// Draw card background with rounded corners effect
	cardX := float32(x + c.margin)
	cardY := float32(y + c.margin)
	cardWidth := float32(width - (c.margin * 2))
	cardHeight := float32(totalHeight)

	// Draw background
	vector.DrawFilledRect(screen, cardX, cardY, cardWidth, cardHeight, bg, false)

	// Draw border
	vector.StrokeRect(screen, cardX, cardY, cardWidth, cardHeight, 1, border, false)

	// Store rect for input detection (including margins)
	c.rect = image.Rect(x, y, x+width, y+totalHeight+(c.margin*2))

	// Draw child elements
	if len(c.elements) > 0 {
		if c.inline {
			// Horizontal layout
			curX := x + c.margin + c.padding
			spacing := 3 // Increased from 1 to 3 pixels for better spacing between elements
			elementWidth := (width - (c.margin * 2) - (c.padding * 2) - (spacing * (len(c.elements) - 1))) / len(c.elements)
			for _, el := range c.elements {
				if el.IsVisible() {
					if r, ok := el.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(c.displayAlpha())
					}
					el.Draw(screen, curX, y+c.margin+c.padding, elementWidth, fontFace)
					curX += elementWidth + spacing
				}
			}
		} else {
			// Vertical layout
			curY := y + c.margin + c.padding
			elementWidth := width - (c.margin * 2) - (c.padding * 2)
			for _, el := range c.elements {
				if el.IsVisible() {
					if r, ok := el.(interface{ SetRevealProgress(float64) }); ok {
						r.SetRevealProgress(c.displayAlpha())
					}
					height := el.Draw(screen, x+c.margin+c.padding, curY, elementWidth, fontFace)
					curY += height + 3 // Increased from 1 to 3 pixels for better spacing between elements
				}
			}
		}
	}

	return totalHeight + (c.margin * 2)
}

// GetMinHeight returns the minimal height required for the card.
func (c *Card) GetMinHeight() int {
	if !c.IsVisible() {
		return 0
	}

	contentHeight := 0
	if c.inline {
		// Horizontal layout - find the tallest element
		maxHeight := 0
		for _, el := range c.elements {
			if el.IsVisible() {
				h := el.GetMinHeight()
				if h > maxHeight {
					maxHeight = h
				}
			}
		}
		contentHeight = maxHeight
	} else {
		// Vertical layout - sum all heights
		for _, el := range c.elements {
			if el.IsVisible() {
				contentHeight += el.GetMinHeight() + 3 // Increased from 1 to 3 pixels for better spacing
			}
		}
		if contentHeight > 0 {
			contentHeight -= 3 // Remove last spacing
		}
	}

	totalHeight := contentHeight + (c.padding * 2) + (c.margin * 2)
	if totalHeight < 16 {
		totalHeight = 16 // Ultra small minimum height
	}

	return totalHeight
}

// IsVisible returns whether the card is currently visible
func (c *Card) IsVisible() bool {
	return c.BaseMenuElement.IsVisible()
}

// SetVisible sets the visibility of the card
func (c *Card) SetVisible(visible bool) {
	c.BaseMenuElement.SetVisible(visible)
}

// CardContainer manages a list of cards, allowing adding and removing by ID.
type CardContainer struct {
	elements []EdgeMenuElement
}

// NewCardContainer creates a new empty CardContainer.
func NewCardContainer() *CardContainer {
	return &CardContainer{elements: []EdgeMenuElement{}}
}

// AddElement adds an EdgeMenuElement (typically a Card) to the container.
func (cc *CardContainer) AddElement(e EdgeMenuElement) {
	cc.elements = append(cc.elements, e)
}

// RemoveElement removes a Card by its ID from the container.
func (cc *CardContainer) RemoveElement(id string) {
	for i, el := range cc.elements {
		if card, ok := el.(*Card); ok && card.ID() == id {
			cc.elements = append(cc.elements[:i], cc.elements[i+1:]...)
			break
		}
	}
}
