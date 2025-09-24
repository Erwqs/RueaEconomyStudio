package app

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"runtime"
	"sync"

	"etools/assets"
	"etools/eruntime"
	"etools/fonts"
	"etools/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

// Global font cache to prevent memory leaks
var (
	fontCache        = make(map[float64]font.Face)
	fontCacheMux     sync.RWMutex
	fontData         []byte
	parsedFont       *opentype.Font
	fontLoadOnce     sync.Once
	fontLoadError    error
	textYOffset      int  // Global variable for text Y positioning offset
	maxFontCacheSize = 20 // Limit font cache to 20 different sizes
)

// initFont loads and parses the font file once
func initFont() {
	fontData, fontLoadError = fonts.FontFiles.ReadFile("Wynncraft-Regular.otf")
	if fontLoadError != nil {
		log.Printf("Failed to load Wynncraft font: %v, using default font", fontLoadError)
		return
	}

	parsedFont, fontLoadError = opentype.Parse(fontData)
	if fontLoadError != nil {
		log.Printf("Failed to parse Wynncraft font: %v, using default font", fontLoadError)
		return
	}
}

// ScaleInfo holds scaling information for responsive UI
type ScaleInfo struct {
	Factor     float64
	FontSize   float64
	ScreenW    int
	ScreenH    int
	BaseWidth  int
	BaseHeight int
}

// getScaleInfo calculates scaling factors based on current screen size
func getScaleInfo(screenW, screenH int) ScaleInfo {
	// Base resolution for UI design (reference size)
	baseWidth := 800
	baseHeight := 600

	// Calculate scale factor based on both width and height, using the smaller scale
	// to ensure UI elements fit on screen
	scaleX := float64(screenW) / float64(baseWidth)
	scaleY := float64(screenH) / float64(baseHeight)
	scaleFactor := math.Min(scaleX, scaleY)

	// Clamp scale factor to reasonable bounds
	if scaleFactor < 0.5 {
		scaleFactor = 0.5
	} else if scaleFactor > 3.0 {
		scaleFactor = 3.0
	}

	// Base font size that will be scaled
	baseFontSize := 16.0
	fontSize := baseFontSize * scaleFactor

	return ScaleInfo{
		Factor:     scaleFactor,
		FontSize:   fontSize,
		ScreenW:    screenW,
		ScreenH:    screenH,
		BaseWidth:  baseWidth,
		BaseHeight: baseHeight,
	}
}

// scaleInt scales an integer value by the scale factor
func (si ScaleInfo) scaleInt(value int) int {
	return int(float64(value) * si.Factor)
}

// MenuOption represents a menu option
type MenuOption struct {
	text   string
	action func()
}

// Menu represents a simple menu system
type Menu struct {
	options         []MenuOption
	selectedIndex   int
	baseFont        font.Face
	baseFontSize    float64
	optionHeight    int
	startY          int
	startX          int
	cachedFont      font.Face
	lastFontSize    float64
	backgroundImage *ebiten.Image // Add this field
}

// NewMenu creates a new menu
func NewMenu() *Menu {
	menu := &Menu{
		options:       make([]MenuOption, 0),
		selectedIndex: 0,
		baseFont:      loadWynncraftFont(16),
		baseFontSize:  16,
		optionHeight:  30,
		startY:        200,
		startX:        100,
	}

	// Load background image
	menu.loadBackgroundImage()

	return menu
}

// getScaledFont returns a cached font or creates a new one if size changed significantly
func (m *Menu) getScaledFont(fontSize float64) font.Face {
	// Use tolerance to avoid creating new fonts for tiny size changes
	const tolerance = 0.5
	if m.cachedFont != nil && math.Abs(m.lastFontSize-fontSize) < tolerance {
		return m.cachedFont
	}

	// Round font size to reduce cache misses
	roundedSize := math.Round(fontSize*2) / 2 // Round to nearest 0.5

	m.cachedFont = loadWynncraftFont(roundedSize)
	m.lastFontSize = roundedSize
	return m.cachedFont
}

// AddOption adds a menu option
func (m *Menu) AddOption(text string, action func()) {
	m.options = append(m.options, MenuOption{
		text:   text,
		action: action,
	})
}

// Update updates the menu (handles input)
func (m *Menu) Update(keyEvents <-chan KeyEvent, screenW, screenH int) {
	scaleInfo := getScaleInfo(screenW, screenH)

	// Get mouse position once and only check if mouse button is used
	mx, my := ebiten.CursorPosition()

	// Handle mouse click only if button is pressed
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		m.handleMouseClick(mx, my, scaleInfo)
	}

	// Handle mouse hover (only if mouse position might have changed)
	m.handleMouseHover(mx, my, scaleInfo)

	// Process key events
	for {
		select {
		case event := <-keyEvents:
			if event.Pressed {
				m.handleKeyEvent(event)
			}
		default:
			return
		}
	}
}

// handleKeyEvent handles menu navigation
func (m *Menu) handleKeyEvent(event KeyEvent) {
	switch event.Key {
	case ebiten.KeyArrowUp, ebiten.KeyW:
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case ebiten.KeyArrowDown, ebiten.KeyS:
		if m.selectedIndex < len(m.options)-1 {
			m.selectedIndex++
		}
	case ebiten.KeyEnter, ebiten.KeySpace:
		if m.selectedIndex < len(m.options) && m.options[m.selectedIndex].action != nil {
			m.options[m.selectedIndex].action()
		}
	}
}

// handleMouseHover updates selection based on mouse position
func (m *Menu) handleMouseHover(mx, my int, scaleInfo ScaleInfo) {
	// Scale menu positioning for hit detection
	scaledStartY := scaleInfo.scaleInt(m.startY)
	scaledStartX := scaleInfo.scaleInt(m.startX)
	scaledOptionHeight := scaleInfo.scaleInt(m.optionHeight)

	for i := range m.options {
		y := scaledStartY + i*scaledOptionHeight + scaleInfo.scaleInt(10) // Account for moved menu items
		boxStartY := y - scaleInfo.scaleInt(8)
		boxEndY := boxStartY + (scaledOptionHeight - scaleInfo.scaleInt(5))
		hitboxWidth := scaleInfo.scaleInt(300)

		if my >= boxStartY && my <= boxEndY &&
			mx >= scaledStartX-scaleInfo.scaleInt(20) && mx <= scaledStartX+hitboxWidth {
			m.selectedIndex = i
			break
		}
	}
}

// handleMouseClick executes action if mouse clicked on menu option
func (m *Menu) handleMouseClick(mx, my int, scaleInfo ScaleInfo) {
	// Scale menu positioning for hit detection
	scaledStartY := scaleInfo.scaleInt(m.startY)
	scaledStartX := scaleInfo.scaleInt(m.startX)
	scaledOptionHeight := scaleInfo.scaleInt(m.optionHeight)

	for i := range m.options {
		y := scaledStartY + i*scaledOptionHeight + scaleInfo.scaleInt(10) // Account for moved menu items
		boxStartY := y - scaleInfo.scaleInt(8)
		boxEndY := boxStartY + (scaledOptionHeight - scaleInfo.scaleInt(5))
		hitboxWidth := scaleInfo.scaleInt(300)

		if my >= boxStartY && my <= boxEndY &&
			mx >= scaledStartX-scaleInfo.scaleInt(20) && mx <= scaledStartX+hitboxWidth {
			if m.options[i].action != nil {
				m.options[i].action()
			}
			break
		}
	}
}

// Draw draws the menu
func (m *Menu) Draw(screen *ebiten.Image) {
	// Get scaling information
	scaleInfo := getScaleInfo(screen.Bounds().Dx(), screen.Bounds().Dy())

	// Draw background
	if m.backgroundImage != nil {
		// Scale and draw the background image to fit the screen
		m.drawScaledBackground(screen, scaleInfo)
	} else {
		// Fallback to solid color background
		screen.Fill(color.RGBA{20, 20, 30, 255})
	}

	// Get cached font with scaled size
	scaledFont := m.getScaledFont(scaleInfo.FontSize)

	// Draw title with a semi-transparent background for better readability
	title := "Wynncraft ETools"
	titleWidth := font.MeasureString(scaledFont, title).Ceil()
	titleX := (scaleInfo.ScreenW - titleWidth) / 2

	// Calculate title background dimensions
	titleBgPadding := scaleInfo.scaleInt(10)
	titleBgHeight := scaleInfo.scaleInt(40)
	titleBgY := scaleInfo.scaleInt(70) // Position the background box

	// Draw title background for better readability
	ebitenutil.DrawRect(screen,
		float64(titleX-titleBgPadding),
		float64(titleBgY),
		float64(titleWidth+2*titleBgPadding),
		float64(titleBgHeight),
		color.RGBA{0, 0, 0, 150}) // Semi-transparent black

	// Apply font offset for proper alignment and center text in the background box
	fontOffset := getFontVerticalOffset(scaleInfo.FontSize)
	titleTextYOffset := (titleBgY + (titleBgY + titleBgHeight)) / 2 // Calculate center of title box
	text.Draw(screen, title, scaledFont, titleX, titleTextYOffset+fontOffset, color.RGBA{255, 255, 255, 255})

	// Scale menu positioning
	scaledStartY := scaleInfo.scaleInt(m.startY)
	scaledStartX := scaleInfo.scaleInt(m.startX)
	scaledOptionHeight := scaleInfo.scaleInt(m.optionHeight)

	// Draw menu options with background for better readability
	menuBgWidth := scaleInfo.scaleInt(400)
	menuBgHeight := len(m.options)*scaledOptionHeight + scaleInfo.scaleInt(30) // Adjust for moved menu items
	ebitenutil.DrawRect(screen,
		float64(scaledStartX-scaleInfo.scaleInt(30)),
		float64(scaledStartY-scaleInfo.scaleInt(5)), // Adjust background position
		float64(menuBgWidth),
		float64(menuBgHeight),
		color.RGBA{0, 0, 0, 120}) // Semi-transparent background for menu

	// Draw menu options
	for i, option := range m.options {
		y := scaledStartY + i*scaledOptionHeight + scaleInfo.scaleInt(10) // Move menu items down
		x := scaledStartX

		// Calculate text Y offset based on menu item box center
		boxStartY := y - scaleInfo.scaleInt(8)
		boxEndY := boxStartY + (scaledOptionHeight - scaleInfo.scaleInt(5))
		textYOffset = (boxStartY + boxEndY) / 2

		// Highlight selected option - draw background FIRST before text
		optionColor := color.RGBA{200, 200, 200, 255}
		if i == m.selectedIndex {
			// Draw selection background BEFORE text
			ebitenutil.DrawRect(screen,
				float64(x-scaleInfo.scaleInt(25)),
				float64(boxStartY),
				float64(menuBgWidth-scaleInfo.scaleInt(10)),
				float64(boxEndY-boxStartY),
				color.RGBA{255, 255, 100, 50}) // Highlight background

			optionColor = color.RGBA{0, 0, 0, 255} // Dark text on yellow background for readability

			// Draw selection indicator with textYOffset
			indicatorX := x - scaleInfo.scaleInt(20)
			text.Draw(screen, "-", scaledFont, indicatorX, textYOffset+fontOffset, color.RGBA{0, 0, 0, 255})
		}

		// Draw option text with textYOffset - this comes AFTER background
		text.Draw(screen, option.text, scaledFont, x, textYOffset+fontOffset, optionColor)
	}

	// Draw instructions with background
	instructions := "Use Arrow Keys or WASD to navigate, Enter/Space to select, or click with mouse"
	instrY := scaledStartY + len(m.options)*scaledOptionHeight + scaleInfo.scaleInt(60) // Adjust for moved menu items
	instrX := scaleInfo.scaleInt(50)
	instrWidth := font.MeasureString(scaledFont, instructions).Ceil()

	// Instruction background
	instrBgStartY := instrY - scaleInfo.scaleInt(10)
	instrBgHeight := scaleInfo.scaleInt(25)
	ebitenutil.DrawRect(screen,
		float64(instrX-scaleInfo.scaleInt(10)),
		float64(instrBgStartY),
		float64(instrWidth+scaleInfo.scaleInt(20)),
		float64(instrBgHeight),
		color.RGBA{0, 0, 0, 100})

	// Calculate text Y offset for instructions
	instrTextYOffset := (instrBgStartY + (instrBgStartY + instrBgHeight)) / 2
	text.Draw(screen, instructions, scaledFont, instrX, instrTextYOffset+fontOffset, color.RGBA{150, 150, 150, 255})
}

// Add method to load background image
func (m *Menu) loadBackgroundImage() {
	// Try to load background image from embedded assets
	file, err := assets.AssetFiles.Open("bg.png")
	if err != nil {
		log.Printf("Could not load background image: %v", err)
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("Could not decode background image: %v", err)
		return
	}

	m.backgroundImage = ebiten.NewImageFromImage(img)
}

// Add method to draw scaled background
func (m *Menu) drawScaledBackground(screen *ebiten.Image, scaleInfo ScaleInfo) {
	if m.backgroundImage == nil {
		return
	}

	// Get original image dimensions
	imgW := m.backgroundImage.Bounds().Dx()
	imgH := m.backgroundImage.Bounds().Dy()

	// Calculate scale to cover the entire screen
	scaleX := float64(scaleInfo.ScreenW) / float64(imgW)
	scaleY := float64(scaleInfo.ScreenH) / float64(imgH)

	// Use the larger scale to ensure the image covers the screen
	scale := math.Max(scaleX, scaleY)

	// Calculate centered position
	scaledW := float64(imgW) * scale
	scaledH := float64(imgH) * scale
	offsetX := (float64(scaleInfo.ScreenW) - scaledW) / 2
	offsetY := (float64(scaleInfo.ScreenH) - scaledH) / 2

	// Draw the scaled background image
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(offsetX, offsetY)

	// Optional: Add some transparency to the background
	op.ColorM.Scale(1, 1, 1, 0.8) // 80% opacity

	screen.DrawImage(m.backgroundImage, op)
}

type GameState int

const (
	StateMenu GameState = iota
	StateSession
	StateSettings
	StateSessionManager
)

type State struct {
	gameState         GameState
	menu              *Menu
	settingsScreen    *SettingsScreen
	sessionManager    *SessionManager
	inputManager      *InputManager
	keyEventCh        <-chan KeyEvent    // Channel to receive key events
	debugModule       *DebugModule       // Example module that listens to key events
	settingsModule    *SettingsModule    // Another example module
	gameplayModule    *GameplayModule    // Gameplay module
	fileSystemManager *FileSystemManager // File system operations and dialogues
}

func (s *State) Update() error {
	// Handle panic recovery for the Update loop
	defer HandlePanic()

	// Update panic notification first and check if it consumes input
	panicNotifier := GetPanicNotifier()
	if panicNotifier.IsVisible() {
		if panicNotifier.Update() {
			// Panic notification is consuming input, skip all other updates
			return nil
		}
	}

	// Update input manager ONLY ONCE per frame
	s.inputManager.Update()

	// Update toast manager
	GetToastManager().Update()
	
	// Handle toast input FIRST - if any toast consumes input, stop all other processing
	if GetToastManager().HandleInput() {
		// Toast consumed the input, skip all other input handling
		return nil
	}

	// Update pause notification manager
	GetPauseNotificationManager().Update()
	// Update file system manager FIRST - if it consumes input, stop all other processing
	if s.fileSystemManager != nil {
		if s.fileSystemManager.Update() {
			// File system dialogue is active and consuming input
			// Don't process any other updates to prevent background interaction
			return nil
		}
	}

	// Also update global file system manager for state save/load
	if globalFileManager := GetFileSystemManager(); globalFileManager != nil {
		if globalFileManager.Update() {
			// Global file system dialogue is active and consuming input
			return nil
		}
	}

	// Get screen dimensions ONLY ONCE per frame
	screenW, screenH := WebSafeWindowSize()

	// Process key events from channel ONLY ONCE
	for {
		select {
		case event := <-s.keyEventCh:
			s.handleKeyEvent(event)
		default:
			// No more events to process, exit loop
			goto eventsDone
		}
	}
eventsDone:

	// Update only the relevant modules based on current state
	switch s.gameState {
	case StateMenu:
		// Only update modules needed for menu
		s.menu.Update(s.keyEventCh, screenW, screenH)
	case StateSettings:
		// Only update modules needed for settings
		s.settingsScreen.Update(s.keyEventCh, screenW, screenH)
	case StateSessionManager:
		// Update session manager
		s.sessionManager.Update()
	case StateSession:
		// Update debug module only in game mode if active
		if s.debugModule.IsActive() {
			s.debugModule.Update()
		}
		// Update settings module only if active
		if s.settingsModule.IsActive() {
			s.settingsModule.Update()
		}
		// Update gameplay module
		s.gameplayModule.Update()
	}

	return nil
}

// handleKeyEvent processes key events received from the input manager
func (s *State) handleKeyEvent(event KeyEvent) {
	// Handle global key events
	if event.Pressed {
		switch event.Key {
		case ebiten.KeyEscape:
			// Completely disable ESC handling to prevent weird menu behavior
			// Since we start directly in game mode and don't want main menu
			// if s.gameState == StateSession {
			// 	// Prevent ESC from returning to menu if loadout manager is visible or in apply mode
			// 	loadoutManager := GetLoadoutManager()
			// 	if loadoutManager != nil && (loadoutManager.IsVisible() || loadoutManager.IsApplyingLoadout()) {
			// 		// Let the loadout manager handle ESC/back, do not close gameplay or go to menu
			// 		return
			// 	}
			// 	// Only go to menu if neither territory menu nor guild manager nor EdgeMenu is open
			// 	if s.gameplayModule != nil {
			// 		// Check if EdgeMenu is open first
			// 		if s.gameplayModule.GetMapView() != nil && s.gameplayModule.GetMapView().IsEdgeMenuOpen() {
			// 			// Do nothing - the map view will handle closing the EdgeMenu
			// 		} else if s.gameplayModule.GetMapView() != nil && s.gameplayModule.GetMapView().IsTransitResourceMenuOpen() {
			// 			// Do nothing - the map view will handle closing the transit resource menu
			// 		} else if s.gameplayModule.GetMapView() != nil && s.gameplayModule.GetMapView().IsEditingClaims() {
			// 			// Do nothing - the map view will handle cancelling claim edit mode
			// 		} else if s.gameplayModule.GetMapView() != nil && s.gameplayModule.GetMapView().JustHandledEscKey() {
			// 			// Do nothing - the map view already handled the ESC key
			// 		} else if s.gameplayModule.GetMapView() != nil && s.gameplayModule.GetMapView().TributeMenuJustHandledEscKey() {
			// 			// Do nothing - the tribute menu already handled the ESC key
			// 		} else if s.gameplayModule.IsTerritoryMenuOpen() {
			// 			// Do nothing - the map handler will close the territory menu
			// 			// and we don't want to go back to the main menu
			// 		} else if s.gameplayModule.guildManager != nil && s.gameplayModule.guildManager.IsVisible() {
			// 			// Do nothing - the guild manager will handle its own closure
			// 		} else {
			// 			// Commented out to disable main menu return
			// 			// s.gameState = StateMenu
			// 			// s.gameplayModule.SetActive(false)
			// 		}
			// 	}
			// } else if s.gameState == StateSettings {
			// 	// Commented out to disable main menu return
			// 	// s.gameState = StateMenu
			// } else if s.gameState == StateSessionManager {
			// 	// Commented out to disable main menu return
			// 	// s.gameState = StateMenu
			// }
		case ebiten.KeyF11:
			// Toggle fullscreen (example)
			if ebiten.IsFullscreen() {
				ebiten.SetFullscreen(false)
			} else {
				ebiten.SetFullscreen(true)
			}
		case ebiten.KeyF12:
			// Toggle debug module
			s.debugModule.SetActive(!s.debugModule.IsActive())
		case ebiten.KeyF10:
			// Toggle settings module
			s.settingsModule.SetActive(!s.settingsModule.IsActive())
		case ebiten.KeyP:
			// CTRL + ALT + P - Trigger panic for demonstration
			if ebiten.IsKeyPressed(ebiten.KeyControl) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
				// // fmt.Println("[PANIC_DEMO] User triggered panic via CTRL+ALT+P")
				panic("User-triggered panic for demonstration (CTRL+ALT+P)")
			}
		}
	}
}

func (s *State) Draw(screen *ebiten.Image) {
	// Handle panic recovery for the Draw loop
	defer HandlePanic()

	switch s.gameState {
	case StateMenu:
		// Draw menu
		s.menu.Draw(screen)
	case StateSessionManager:
		// Draw session manager
		s.sessionManager.Draw(screen)
	case StateSession:
		// Draw gameplay module instead of the debug text
		screen.Fill(color.RGBA{10, 10, 20, 255})
		s.gameplayModule.Draw(screen)

	case StateSettings:
		// Draw settings screen
		s.settingsScreen.Draw(screen)
	}

	// Draw file system manager in all states (renders dialogues if visible)
	if s.fileSystemManager != nil {
		s.fileSystemManager.Draw(screen)
	}

	// Also draw global file system manager for state save/load
	if globalFileManager := GetFileSystemManager(); globalFileManager != nil {
		globalFileManager.Draw(screen)
	}

	// Draw toasts on top of everything else
	GetToastManager().Draw(screen)

	// Draw panic notification on top of absolutely everything
	panicNotifier := GetPanicNotifier()
	if panicNotifier.IsVisible() {
		panicNotifier.Draw(screen)
	}
}

// Game implements ebiten.Game interface
type Game struct {
	state *State
}

// New creates a new Game instance
func New() *Game {
	inputManager := NewInputManager()
	keyEventCh := inputManager.Subscribe()
	debugModule := NewDebugModule(inputManager)
	settingsModule := NewSettingsModule(inputManager)
	gameplayModule := NewGameplayModule(inputManager)
	fileSystemManager := NewFileSystemManager(inputManager)

	// Initialize global file system manager for state save/load
	InitializeFileSystemManager(inputManager)

	// Initialize loadout persistence callbacks
	eruntime.SetLoadoutCallbacks(
		func() []typedef.Loadout {
			return GetLoadoutManager().GetLoadouts()
		},
		func(loadouts []typedef.Loadout) {
			GetLoadoutManager().SetLoadouts(loadouts)
		},
		func(loadouts []typedef.Loadout) {
			GetLoadoutManager().MergeLoadouts(loadouts)
		},
	)

	game := &Game{
		state: &State{
			gameState: StateSession, // Skip main menu, go straight to game
			// gameState:         StateMenu, // Original main menu state
			inputManager:      inputManager,
			keyEventCh:        keyEventCh,
			debugModule:       debugModule,
			settingsModule:    settingsModule,
			gameplayModule:    gameplayModule,
			fileSystemManager: fileSystemManager,
		},
	}

	// Initialize the menu and settings screen
	game.state.menu = game.createMenu()
	game.state.settingsScreen = NewSettingsScreen(settingsModule)

	// Since we're starting in StateSession, activate the gameplay module
	if game.state.gameplayModule != nil {
		game.state.gameplayModule.SetActive(true)
	}

	// Initialize session manager
	game.state.sessionManager = NewSessionManager(func() {
		// Commented out to disable main menu return
		// game.state.gameState = StateMenu
	})

	// Set up the callback for returning to menu from settings
	game.state.settingsScreen.backToMenuCallback = func() {
		// Commented out to disable main menu return
		// game.state.gameState = StateMenu
	}

	// Check if autosave was loaded on startup - if not, show welcome screen
	if !eruntime.WasAutoSaveLoadedOnStartup() {
		// No autosave file was found/loaded, show welcome screen
		welcomeScreen := GetWelcomeScreen()

		// Set up callback to access guild manager
		welcomeScreen.SetImportGuildsCallback(func() {
			if game.state.gameplayModule != nil && game.state.gameplayModule.guildManager != nil {
				game.state.gameplayModule.guildManager.Show()
			}
		})

		welcomeScreen.Show()
		// fmt.Println("[APP] No autosave loaded on startup, showing welcome screen")
	}

	return game
}

// Update updates the game state
func (g *Game) Update() error {
	return g.state.Update()
}

// Draw draws the game state to the screen
func (g *Game) Draw(screen *ebiten.Image) {
	g.state.Draw(screen)
}

// Layout returns the layout of the game
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if runtime.GOOS == "js" && runtime.GOARCH == "wasm" {
		// Web-specific handling with more aggressive fallbacks

		// Debug logging (will appear in browser console)
		if outsideWidth == 0 || outsideHeight == 0 {
			// fmt.Printf("WARNING: Layout received 0x0 dimensions, using fallback\n")
		}

		// Ensure we always return reasonable dimensions
		finalWidth := outsideWidth
		finalHeight := outsideHeight

		// Use browser viewport as fallback (typical web app sizes)
		if finalWidth <= 0 {
			finalWidth = 1920 // Common desktop width
		}
		if finalHeight <= 0 {
			finalHeight = 1080 // Common desktop height
		}

		// Enforce minimums to prevent UI issues
		if finalWidth < 800 {
			finalWidth = 800
		}
		if finalHeight < 600 {
			finalHeight = 600
		}

		// Log the final decision
		if finalWidth != outsideWidth || finalHeight != outsideHeight {
			// fmt.Printf("Layout: adjusted %dx%d -> %dx%d\n",
			// outsideWidth, outsideHeight, finalWidth, finalHeight)
		}

		return finalWidth, finalHeight
	} else {
		// Desktop: use provided dimensions or reasonable defaults
		if outsideWidth <= 0 || outsideHeight <= 0 {
			return 1280, 720
		}
		return outsideWidth, outsideHeight
	}
}

// getScaledFont returns a cached font or creates a new one if size changed significantly
func (ss *SettingsScreen) getScaledFont(fontSize float64) font.Face {
	// Use tolerance to avoid creating new fonts for tiny size changes
	const tolerance = 0.5
	if ss.cachedFont != nil && math.Abs(ss.lastFontSize-fontSize) < tolerance {
		return ss.cachedFont
	}

	// Round font size to reduce cache misses
	roundedSize := math.Round(fontSize*2) / 2 // Round to nearest 0.5

	ss.cachedFont = loadWynncraftFont(roundedSize)
	ss.lastFontSize = roundedSize
	return ss.cachedFont
}

// Update handles input for the settings screen
func (ss *SettingsScreen) Update(keyEvents <-chan KeyEvent, screenW, screenH int) {
	scaleInfo := getScaleInfo(screenW, screenH)

	// Get mouse position once
	mx, my := ebiten.CursorPosition()

	// Handle mouse click only if button is pressed
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		ss.handleMouseClick(mx, my, scaleInfo)
	}

	// Handle mouse hover
	ss.handleMouseHover(mx, my, scaleInfo)

	// Process key events
	for {
		select {
		case event := <-keyEvents:
			if event.Pressed {
				ss.handleKeyEvent(event)
			}
		default:
			return
		}
	}
}

// handleMouseHover updates selection based on mouse position
func (ss *SettingsScreen) handleMouseHover(mx, my int, scaleInfo ScaleInfo) {
	startY := scaleInfo.scaleInt(150)
	optionHeight := scaleInfo.scaleInt(40)
	for i := range ss.options {
		optionY := startY + i*optionHeight
		hitboxPadding := scaleInfo.scaleInt(15)
		if my >= optionY-hitboxPadding && my <= optionY+hitboxPadding &&
			mx >= scaleInfo.scaleInt(100) && mx <= scaleInfo.scaleInt(500) {
			ss.selectedIndex = i
			break
		}
	}
}

// handleMouseClick executes action if mouse clicked on settings option
func (ss *SettingsScreen) handleMouseClick(mx, my int, scaleInfo ScaleInfo) {
	startY := scaleInfo.scaleInt(150)
	optionHeight := scaleInfo.scaleInt(40)
	for i := range ss.options {
		optionY := startY + i*optionHeight
		hitboxPadding := scaleInfo.scaleInt(15)
		if my >= optionY-hitboxPadding && my <= optionY+hitboxPadding &&
			mx >= scaleInfo.scaleInt(100) && mx <= scaleInfo.scaleInt(500) {
			ss.selectedIndex = i
			ss.executeAction(i)
			break
		}
	}
}

// handleKeyEvent handles keyboard input for settings
func (ss *SettingsScreen) handleKeyEvent(event KeyEvent) {
	switch event.Key {
	case ebiten.KeyArrowUp, ebiten.KeyW:
		if ss.selectedIndex > 0 {
			ss.selectedIndex--
		}
	case ebiten.KeyArrowDown, ebiten.KeyS:
		if ss.selectedIndex < len(ss.options)-1 {
			ss.selectedIndex++
		}
	case ebiten.KeyEnter, ebiten.KeySpace:
		ss.executeAction(ss.selectedIndex)
	case ebiten.KeyArrowLeft, ebiten.KeyA:
		ss.adjustValue(-1)
	case ebiten.KeyArrowRight, ebiten.KeyD:
		ss.adjustValue(1)
	}
}

// adjustValue adjusts the current setting value
func (ss *SettingsScreen) adjustValue(direction int) {
	switch ss.selectedIndex {
	case 0: // Volume
		volume := ss.settingsModule.GetVolume()
		newVolume := volume + direction*10
		if newVolume >= 0 && newVolume <= 100 {
			// This would need to be implemented in SettingsModule
			if direction > 0 {
				ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyEqual, Pressed: true})
			} else {
				ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyMinus, Pressed: true})
			}
		}
	case 1: // Show FPS
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyF, Pressed: true})
	case 2: // Window Mode
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyW, Pressed: true})
	case 3: // VSync
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyV, Pressed: true})
	case 4: // Auto Save
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyA, Pressed: true})
	case 5: // Language
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyL, Pressed: true})
	}
}

// executeAction executes the action for the selected setting
func (ss *SettingsScreen) executeAction(index int) {
	switch index {
	case 0, 1, 2, 3, 4, 5: // Volume, FPS, Window Mode, VSync, Auto Save, Language - handled by adjustValue
		// These are handled by arrow keys or clicking multiple times
	case 6: // Reset to Defaults
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyR, Pressed: true})
	case 7: // Save Settings
		ss.settingsModule.handleKeyEvent(KeyEvent{Key: ebiten.KeyS, Pressed: true})
	case 8: // Back to Menu
		if ss.backToMenuCallback != nil {
			ss.backToMenuCallback()
		}
	}
}

// Draw draws the settings screen
func (ss *SettingsScreen) Draw(screen *ebiten.Image) {
	// Get scaling information
	scaleInfo := getScaleInfo(screen.Bounds().Dx(), screen.Bounds().Dy())

	// Get cached font with scaled size (slightly smaller than menu font)
	scaledFont := ss.getScaledFont(scaleInfo.FontSize * 0.875)

	// Draw background
	screen.Fill(color.RGBA{15, 25, 35, 255})

	// Draw title
	title := "Settings"
	titleWidth := font.MeasureString(scaledFont, title).Ceil()
	titleX := (scaleInfo.ScreenW - titleWidth) / 2
	titleY := scaleInfo.scaleInt(80)

	// Apply font offset for proper alignment
	fontOffset := getFontVerticalOffset(scaleInfo.FontSize * 0.875)
	text.Draw(screen, title, scaledFont, titleX, titleY+fontOffset, color.RGBA{255, 255, 255, 255})

	// Scale settings positioning
	startY := scaleInfo.scaleInt(150)
	optionHeight := scaleInfo.scaleInt(40)
	startX := scaleInfo.scaleInt(100)

	for i, option := range ss.options {
		y := startY + i*optionHeight
		x := startX

		// Highlight selected option
		optionColor := color.RGBA{200, 200, 200, 255}
		if i == ss.selectedIndex {
			optionColor = color.RGBA{255, 255, 100, 255}
			// Draw selection indicator with font offset
			indicatorX := x - scaleInfo.scaleInt(20)
			text.Draw(screen, "- ", scaledFont, indicatorX, y+fontOffset, optionColor)
		}

		// Draw option text
		optionText := option
		if i == 0 { // Volume
			optionText = fmt.Sprintf("%s: %d%%", option, ss.settingsModule.GetVolume())
		} else if i == 1 { // Show FPS
			optionText = fmt.Sprintf("%s: %t", option, ss.settingsModule.GetShowFPS())
		} else if i == 2 { // Window Mode
			optionText = fmt.Sprintf("%s: %s", option, ss.settingsModule.GetWindowMode())
		} else if i == 3 { // VSync
			optionText = fmt.Sprintf("%s: %t", option, ss.settingsModule.GetVSync())
		} else if i == 4 { // Auto Save
			optionText = fmt.Sprintf("%s: %t", option, ss.settingsModule.GetAutoSave())
		} else if i == 5 { // Language
			optionText = fmt.Sprintf("%s: %s", option, ss.settingsModule.GetLanguage())
		}

		text.Draw(screen, optionText, scaledFont, x, y+fontOffset, optionColor)
	}

	// Draw instructions
	instructions := "Use Arrow Keys/WASD to navigate, Enter/Space to select, Left/Right to adjust values"
	instrY := startY + len(ss.options)*optionHeight + scaleInfo.scaleInt(50)
	instrX := scaleInfo.scaleInt(50)
	text.Draw(screen, instructions, scaledFont, instrX, instrY+fontOffset, color.RGBA{150, 150, 150, 255})

	instructions2 := "Click with mouse to select options"
	instrY2 := instrY + scaleInfo.scaleInt(20)
	text.Draw(screen, instructions2, scaledFont, instrX, instrY2+fontOffset, color.RGBA{150, 150, 150, 255})
}

// getFontVerticalOffset calculates the vertical offset needed for proper font alignment
// This compensates for fonts that have alignment issues (like Wynncraft font)
func getFontVerticalOffset(fontSize float64) int {
	// Base offset - adjust this value based on your font's specific alignment issue
	// Positive values move text DOWN, negative values move text UP
	baseOffset := fontSize * 0.2 // Adjust this multiplier as needed (0.1-0.3 typically works)

	// You can also add conditional logic for different font sizes if needed
	if fontSize < 12 {
		baseOffset = fontSize * 0.25 // Smaller fonts might need different offset
	} else if fontSize > 24 {
		baseOffset = fontSize * 0.15 // Larger fonts might need less offset
	}

	return int(baseOffset)
}

// drawTextWithOffset draws text with automatic font alignment compensation
func drawTextWithOffset(screen *ebiten.Image, str string, face font.Face, x, y int, clr color.Color) {
	fontSize := 16.0 // Default size, you might want to track this per font face

	// Try to extract font size from face if possible (this is a simplified approach)
	// In a more robust implementation, you'd track the font size when creating the face
	if metrics := face.Metrics(); metrics.Height.Ceil() > 0 {
		fontSize = float64(metrics.Height.Ceil())
	}

	offset := getFontVerticalOffset(fontSize)
	text.Draw(screen, str, face, x, y+offset, clr)
}

// Alternative: Add offset to ScaleInfo for consistent scaling
func (si ScaleInfo) getFontOffset() int {
	return getFontVerticalOffset(si.FontSize)
}

// loadWynncraftFont loads the Wynncraft font from cache or creates it
func loadWynncraftFont(size float64) font.Face {
	// Initialize font data once
	fontLoadOnce.Do(initFont)

	// Check cache first
	fontCacheMux.RLock()
	if cachedFont, exists := fontCache[size]; exists {
		fontCacheMux.RUnlock()
		return cachedFont
	}
	fontCacheMux.RUnlock()

	// If font loading failed, return fallback
	if fontLoadError != nil || parsedFont == nil {
		return createSimpleFont()
	}

	// Create new font face
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("Failed to create Wynncraft font face: %v, using default font", err)
		return createSimpleFont()
	}

	// Cache the font with size limiting
	fontCacheMux.Lock()
	// If cache is full, remove oldest entry (simple cleanup)
	if len(fontCache) >= maxFontCacheSize {
		// Remove one arbitrary entry to make space
		for key := range fontCache {
			delete(fontCache, key)
			break
		}
	}
	fontCache[size] = face
	fontCacheMux.Unlock()

	return face
}

// createSimpleFont creates a basic fallback font
func createSimpleFont() font.Face {
	return basicfont.Face7x13
}

// GetGameplayModule returns the gameplay module
func (g *Game) GetGameplayModule() *GameplayModule {
	if g.state != nil {
		return g.state.gameplayModule
	}
	return nil
}
