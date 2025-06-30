package app

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// TextRenderer provides helper methods for text rendering
type TextRenderer struct {
	face font.Face
}

// NewTextRenderer creates a new text renderer for the given font face
func NewTextRenderer(face font.Face) *TextRenderer {
	return &TextRenderer{face: face}
}

// DrawText draws text at the given position with the specified color
func (tr *TextRenderer) DrawText(screen *ebiten.Image, textStr string, x, y int, clr color.Color) {
	text.Draw(screen, textStr, tr.face, x, y, clr)
}

// MeasureString returns the pixel width of the given text
func (tr *TextRenderer) MeasureString(str string) int {
	bounds := text.BoundString(tr.face, str)
	return bounds.Dx()
}

// GetLineHeight returns the pixel height of a line of text
func (tr *TextRenderer) GetLineHeight() int {
	metrics := tr.face.Metrics()
	return (metrics.Ascent + metrics.Descent).Round()
}
