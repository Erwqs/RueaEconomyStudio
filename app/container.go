package app

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
)

// Container is a horizontal scrolling container of EdgeMenuElements.
// Implements EdgeMenuElement for use inside EdgeMenus.
// Elements are laid out side by side, with horizontal scroll support.
// Use NewContainer() to create and Add() to add elements.
type Container struct {
	BaseMenuElement
	rect          image.Rectangle
	elements      []EdgeMenuElement
	scrollOffset  int
	scrollTarget  float64
	onItemAdded   func(el EdgeMenuElement)
	onItemRemoved func(el EdgeMenuElement)
}

// NewContainer creates a new horizontal scrolling container.
func NewContainer() *Container {
	return &Container{
		BaseMenuElement: NewBaseMenuElement(),
		elements:        []EdgeMenuElement{},
	}
}

// Add adds an element to the container.
func (c *Container) Add(el EdgeMenuElement) *Container {
	c.elements = append(c.elements, el)
	return c
}

// RemoveElement removes a Card by its ID from the container.
// Only applies to Card elements.
func (c *Container) RemoveElement(id string) {
	for i, el := range c.elements {
		if card, ok := el.(*Card); ok && card.ID() == id {
			c.elements = append(c.elements[:i], c.elements[i+1:]...)
			break
		}
	}
}

// totalWidth computes the total width required to layout all elements with padding.
func (c *Container) totalWidth() int {
	total := 0
	padding := 20
	cardWidth := 200 // Fixed width for cards
	for range c.elements {
		total += cardWidth + padding
	}
	return total
}

// Update handles input and animation for the container and its children.
func (c *Container) Update(mx, my int, deltaTime float64) bool {
	if !c.visible {
		return false
	}
	c.updateAnimation(deltaTime)

	// Handle horizontal wheel scroll when mouse is over container
	wheelX, wheelY := WebSafeWheel()
	if (wheelX != 0 || wheelY != 0) && mx >= c.rect.Min.X && mx < c.rect.Max.X && my >= c.rect.Min.Y && my < c.rect.Max.Y {
		// Use horizontal wheel if available, otherwise use vertical wheel for horizontal scrolling
		scrollAmount := 0.0
		if wheelX != 0 {
			scrollAmount = wheelX * 50 // Horizontal wheel scrolling
		} else if wheelY != 0 {
			scrollAmount = -wheelY * 50 // Invert vertical wheel for horizontal scrolling (scroll down = move right)
		}

		c.scrollTarget += scrollAmount
		// Clamp target
		maxScroll := c.totalWidth() - c.rect.Dx()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if c.scrollTarget < 0 {
			c.scrollTarget = 0
		} else if c.scrollTarget > float64(maxScroll) {
			c.scrollTarget = float64(maxScroll)
		}
		return true
	}

	// Smooth scroll interpolation
	delta := c.scrollTarget - float64(c.scrollOffset)
	if math.Abs(delta) > 0.5 {
		c.scrollOffset += int(delta * 8.0 * deltaTime)
		// clamp offset
		if c.scrollOffset < 0 {
			c.scrollOffset = 0
		}
		maxScroll := c.totalWidth() - c.rect.Dx()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if c.scrollOffset > maxScroll {
			c.scrollOffset = maxScroll
		}
	}

	// Update child elements
	handled := false
	for _, el := range c.elements {
		if el.Update(mx, my, deltaTime) {
			handled = true
			break
		}
	}
	return handled
}

// Draw renders the container and its children horizontally.
// Returns the height used by the container.
func (c *Container) Draw(screen *ebiten.Image, x, y, width int, fontFace font.Face) int {
	if !c.visible || c.animProgress <= 0.01 {
		return 0
	}
	// Compute height based on tallest child
	height := 0
	for _, el := range c.elements {
		if h := el.GetMinHeight(); h > height {
			height = h
		}
	}
	height += 20 // vertical padding

	// Set container rect for input detection
	c.rect = image.Rect(x, y, x+width, y+height)

	// Draw children with horizontal offset
	padding := 20
	cardWidth := 200 // Fixed width for cards
	curX := x - c.scrollOffset
	for _, el := range c.elements {
		el.Draw(screen, curX, y+10, cardWidth, fontFace)
		curX += cardWidth + padding
	}

	return height
}

// GetMinHeight returns the minimal height required for the container.
func (c *Container) GetMinHeight() int {
	// same as Draw height computation
	height := 0
	for _, el := range c.elements {
		if h := el.GetMinHeight(); h > height {
			height = h
		}
	}
	return height + 20
}
