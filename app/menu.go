package app

import (
	"fmt"
	"os"
)

// createMenu creates and configures the main menu
func (g *Game) createMenu() *Menu {
	menu := NewMenu()

	menu.AddOption("New Session", func() {
		g.state.gameState = StateSession
		g.state.gameplayModule.SetActive(true)
	})

	menu.AddOption("Load Session", func() {
		g.state.gameState = StateSessionManager
	})

	menu.AddOption("Settings", func() {
		g.state.gameState = StateSettings
	})

	menu.AddOption("Help", func() {
		fmt.Println("Help - Coming Soon!")
		// TODO: Implement help screen
	})

	menu.AddOption("Credits", func() {
		fmt.Println("Credits - Coming Soon!")
		// TODO: Implement credits screen
	})

	menu.AddOption("Toggle Debug Mode", func() {
		g.state.debugModule.SetActive(!g.state.debugModule.IsActive())
	})

	menu.AddOption("Exit", func() {
		fmt.Println("Exit selected - close the window to quit")
		os.Exit(0)
	})

	return menu
}
