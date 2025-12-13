package app

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// primaryPointerPosition returns a best-effort pointer position prioritizing touch when present.
func primaryPointerPosition() (int, int) {
	if ids := ebiten.TouchIDs(); len(ids) > 0 {
		x, y := ebiten.TouchPosition(ids[0])
		return x, y
	}
	return ebiten.CursorPosition()
}

// primaryJustPressed reports a new primary press (mouse left or any touch) and its position.
func primaryJustPressed() (int, int, bool) {
	if ids := inpututil.AppendJustPressedTouchIDs(nil); len(ids) > 0 {
		x, y := ebiten.TouchPosition(ids[0])
		return x, y, true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		return x, y, true
	}
	return 0, 0, false
}

// primaryJustReleased reports a primary release (mouse left or any touch) and its position.
func primaryJustReleased() (int, int, bool) {
	if ids := inpututil.AppendJustReleasedTouchIDs(nil); len(ids) > 0 {
		x, y := ebiten.TouchPosition(ids[0])
		return x, y, true
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		return x, y, true
	}
	return 0, 0, false
}

// primaryPressed reports whether a primary pointer is held (mouse left or any touch).
func primaryPressed() bool {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		return true
	}
	return len(ebiten.TouchIDs()) > 0
}

// pointInRect checks if a point is within an image.Rectangle.
func pointInRect(x, y int, r image.Rectangle) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}
