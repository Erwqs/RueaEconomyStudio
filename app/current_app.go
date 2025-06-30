package app

// GlobalAppInstance holds a reference to the current game instance
var GlobalAppInstance *Game

// GetCurrentApp returns the current game instance
func GetCurrentApp() *Game {
	return GlobalAppInstance
}

// SetCurrentApp sets the current game instance
func SetCurrentApp(app *Game) {
	GlobalAppInstance = app
}
