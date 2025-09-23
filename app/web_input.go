//go:build !js || !wasm
// +build !js !wasm

package app

import (
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
)

// getBrowserCanvasSize gets the actual canvas size from the browser (WASM only)
func getBrowserCanvasSize() (int, int) {
	if runtime.GOOS != "js" || runtime.GOARCH != "wasm" {
		return 0, 0
	}

	// For now, return defaults - we'll implement JS bridge later
	return 1280, 720
}

// WebInputUtils provides web-specific input handling utilities

// WebSafeWindowSize returns safe window dimensions for web environment
func WebSafeWindowSize() (int, int) {
	// For non-WASM platforms, use ebiten directly
	return ebiten.WindowSize()
}

// WebSafeWheel returns wheel input with appropriate dampening for web environment
func WebSafeWheel() (float64, float64) {
	wheelX, wheelY := ebiten.Wheel()

	if runtime.GOOS != "js" || runtime.GOARCH != "wasm" {
		// Return normal wheel input for non-web builds
		return wheelX, wheelY
	}

	// In WASM, wheel events can be extremely sensitive
	// Apply dampening to make scrolling more reasonable
	const webWheelDampening = 0.05 // Reduce sensitivity significantly

	return wheelX * webWheelDampening, wheelY * webWheelDampening
}

// WebSafeMousePosition returns mouse position with proper bounds checking
func WebSafeMousePosition() (int, int) {
	x, y := ebiten.CursorPosition()

	if runtime.GOOS != "js" || runtime.GOARCH != "wasm" {
		return x, y
	}

	// Ensure mouse position is within reasonable bounds
	width, height := WebSafeWindowSize()

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > width {
		x = width
	}
	if y > height {
		y = height
	}

	return x, y
}
