package main

import (
	"log"
	"runtime"

	"etools/app"
	_ "etools/eruntime"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	// Verify we're running in WASM environment
	if runtime.GOOS != "js" || runtime.GOARCH != "wasm" {
		log.Fatal("This build is specifically for WebAssembly (js/wasm)")
	}

	// fmt.Println("Starting Ruea Economy Studio - Web Version")

	// Initialize panic notification system (web compatible)
	app.InitPanicNotifier()
	app.InitToastManager()
	app.InitPauseNotificationManager()

	// Configure Ebitengine for web
	ebiten.SetWindowTitle("Ruea Economy Studio - Web")
	ebiten.SetTPS(ebiten.UncappedTPS) // Fixed TPS for consistent web performance

	// Web-specific settings - disable features that don't work in browsers
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	// Create the game instance
	game := app.New() // Use the regular constructor with web compatibility built-in
	app.SetCurrentApp(game)

	// Run the game with minimal options for web
	if err := ebiten.RunGame(game); err != nil {
		// In web environment, we can't easily show panic dialogs
		// so log to console and hope the browser dev tools catch it
		// fmt.Printf("Game error: %v\n", err)
		panic(err)
	}
}
