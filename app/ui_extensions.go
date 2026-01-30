package app

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
)

// Rect represents a simple rectangle
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Contains checks if a point is inside the rectangle
func (r *Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// UITextInputExtended adds additional functionality to UITextInput
type UITextInputExtended struct {
	*UITextInput
	focused bool
}

// NewUITextInputExtended creates a new extended text input
func NewUITextInputExtended(placeholder string, x, y, width int, maxLength int) *UITextInputExtended {
	return &UITextInputExtended{
		UITextInput: NewUITextInput(placeholder, x, y, width, maxLength),
		focused:     false,
	}
}

// Update updates the text input state
func (t *UITextInputExtended) Update() bool {
	mx, my := ebiten.CursorPosition()

	if !t.focused && t.UITextInput.contextMenu != nil && t.UITextInput.contextMenu.IsVisible() {
		if t.UITextInput.contextMenu.Update() {
			return true
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		bounds := t.GetBounds()
		if mx >= bounds.Min.X && mx < bounds.Max.X && my >= bounds.Min.Y && my < bounds.Max.Y {
			t.focused = true
			t.UITextInput.Focused = true
			t.UITextInput.showContextMenu(mx, my)
			return true
		}
	}

	// Check for clicks on the input
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		bounds := t.GetBounds()
		if mx >= bounds.Min.X && mx < bounds.Max.X && my >= bounds.Min.Y && my < bounds.Max.Y {
			t.focused = true
		} else {
			t.focused = false
		}
	}

	// Handle text input if focused
	if t.focused {
		changed := t.UITextInput.Update(mx, my)

		// Handle keyboard input for text
		runes := ebiten.InputChars()
		if len(runes) > 0 {
			text := t.GetText()
			for _, r := range runes {
				// Don't allow control characters
				if r >= 32 {
					text += string(r)
				}
			}

			// Truncate to max length
			if t.MaxLength > 0 && len(text) > t.MaxLength {
				text = text[:t.MaxLength]
			}

			if text != t.GetText() {
				t.SetText(text)
				return true
			}
		}

		// Handle backspace
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			text := t.GetText()
			if len(text) > 0 {
				t.SetText(text[:len(text)-1])
				return true
			}
		}

		return changed
	}

	return false
}

// SetFocus sets focus to this input
func (t *UITextInputExtended) SetFocus() {
	t.focused = true
}

// ClearFocus removes focus from this input
func (t *UITextInputExtended) ClearFocus() {
	t.focused = false
}

// IsFocused returns whether this input has focus
func (t *UITextInputExtended) IsFocused() bool {
	return t.focused
}

// UIModalExtended adds additional functionality to UIModal
type UIModalExtended struct {
	*UIModal
}

// NewUIModalExtended creates a new extended modal
func NewUIModalExtended(title string, x, y, width, height int) *UIModalExtended {
	return &UIModalExtended{
		UIModal: NewUIModal(title, x, y, width, height),
	}
}

// Contains checks if a point is inside the modal
func (m *UIModalExtended) Contains(x, y int) bool {
	bounds := m.GetBounds()
	return x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y
}

// GetDefaultFont returns a default font at the specified size
func GetDefaultFont(size int) font.Face {
	return loadWynncraftFont(float64(size))
}
