//go:build js && wasm
// +build js,wasm

package app

import (
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

func WebSafeWindowSize() (int, int) {
	// In WASM, ebiten.WindowSize() doesn't work properly, so always use canvas size
	canvasWidth := js.Global().Call("getCanvasWidth").Int()
	canvasHeight := js.Global().Call("getCanvasHeight").Int()
	// fmt.Printf("WebSafeWindowSize: Using browser canvas size %dx%d\n", canvasWidth, canvasHeight)

	if canvasWidth > 0 && canvasHeight > 0 {
		return canvasWidth, canvasHeight
	}

	// Fallback to reasonable defaults if JS fails
	// fmt.Println("WebSafeWindowSize: Canvas size failed, using fallback 1280x720")
	return 1280, 720
}

func WebSafeWheel() (float64, float64) {
	// Get wheel input from ebiten
	wheelX, wheelY := ebiten.Wheel()

	// Dampen wheel input for web to prevent excessive zooming
	return wheelX * 0.05, wheelY * 0.05
}

func WebSafeMousePosition() (int, int) {
	return ebiten.CursorPosition()
}
