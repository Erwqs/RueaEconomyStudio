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
	visible          bool    // If false, the entire card and its children are not drawn or updated.
	animProgress     float64 // Animation progress for showing/hiding
	anim2Progress    float64 // Secondary animation progress for cards getting added/removed

}

// NewCard creates a new card UI element.
func NewCard() *Card {
	return &Card{
		BaseMenuElement: NewBaseMenuElement(),
		id:              generateCardID(),
		elements:        []EdgeMenuElement{},
	}
}

// ID returns the unique identifier of the card.
func (c *Card) ID() string { return c.id }

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

// Text adds a text element to the card.
func (c *Card) Text(textStr string, options TextOptions) *Card {
	t := NewMenuText(textStr, options)
	c.elements = append(c.elements, t)
	return c
}

// Button adds a button element to the card.
func (c *Card) Button(textStr string, options ButtonOptions, callback func()) *Card {
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
	if !c.visible {
		return false
	}
	c.updateAnimation(deltaTime)

	// Handle hover callbacks
	hovered := mx >= c.rect.Min.X && mx < c.rect.Max.X && my >= c.rect.Min.Y && my < c.rect.Max.Y
	if hovered && c.hoverCallback != nil {
		c.hoverCallback(c)
	}
	if !hovered && c.notHoverCallback != nil {
		c.notHoverCallback(c)
	}

	handled := false
	// Update child elements
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
	if !c.visible || c.animProgress <= 0.01 {
		return 0
	}
	alpha := float32(c.animProgress)
	padding := 10

	// Calculate total height
	totalHeight := padding
	for _, el := range c.elements {
		totalHeight += el.GetMinHeight() + padding
	}

	// Draw background
	bg := color.RGBA{50, 50, 50, 200}
	bg.A = uint8(float32(bg.A) * alpha)
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(totalHeight), bg, false)
	// Draw border
	border := color.RGBA{200, 200, 200, 255}
	border.A = uint8(float32(border.A) * alpha)
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(totalHeight), 2, border, false)

	// Store rect for input detection
	c.rect = image.Rect(x, y, x+width, y+totalHeight)

	// Draw child elements vertically
	curY := y + padding
	for _, el := range c.elements {
		if el.IsVisible() {
			height := el.Draw(screen, x+padding, curY, width-2*padding, fontFace)
			curY += height + padding
		}
	}
	return totalHeight
}

// GetMinHeight returns the minimal height required for the card.
func (c *Card) GetMinHeight() int {
	height := 20
	for _, el := range c.elements {
		height += el.GetMinHeight() + 10
	}
	return height
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
