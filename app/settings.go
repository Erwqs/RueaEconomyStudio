package app

import (
	"fmt"
	"log"

	"RueaES/eruntime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"golang.org/x/image/font"
)

// SettingsModule demonstrates another example of subscribing to key events
type SettingsModule struct {
	keyEventCh      <-chan KeyEvent
	active          bool
	volume          int
	showFPS         bool
	windowMode      string // "Windowed", "Fullscreen", "Borderless"
	vsync           bool
	autoSave        bool
	language        string // "English", "Spanish", "French", etc.
	gpuAcceleration bool   // GPU acceleration for territory calculations
}

// SettingsScreen represents the settings UI screen
type SettingsScreen struct {
	font               font.Face
	baseFontSize       float64
	selectedIndex      int
	options            []string
	values             []interface{}
	callbacks          []func()
	settingsModule     *SettingsModule
	backToMenuCallback func()
	cachedFont         font.Face
	lastFontSize       float64
}

// NewSettingsScreen creates a new settings screen
func NewSettingsScreen(settingsModule *SettingsModule) *SettingsScreen {
	return &SettingsScreen{
		font:           loadWynncraftFont(14),
		baseFontSize:   14,
		selectedIndex:  0,
		options:        []string{"Volume", "Show FPS", "Window Mode", "VSync", "Auto Save", "Language", "GPU Acceleration", "Reset to Defaults", "Save Settings", "Back to Menu"},
		values:         make([]interface{}, 10),
		callbacks:      make([]func(), 10),
		settingsModule: settingsModule,
	}
}

// NewSettingsModule creates a new settings module that listens to key events
func NewSettingsModule(inputManager *InputManager) *SettingsModule {
	sm := &SettingsModule{
		keyEventCh:      inputManager.Subscribe(),
		active:          false,      // Start inactive
		volume:          50,         // Default volume
		showFPS:         true,       // Show FPS by default
		windowMode:      "Windowed", // Default window mode
		vsync:           true,       // VSync enabled by default
		autoSave:        true,       // Auto save enabled by default
		language:        "English",  // Default language
		gpuAcceleration: false,      // GPU acceleration disabled by default
	}

	// Check if GPU acceleration is available
	sm.checkGPUAvailability()
	return sm
}

// Update processes key events in the settings module
func (sm *SettingsModule) Update() {
	if !sm.active {
		return
	}

	// Process all available key events
	for {
		select {
		case event := <-sm.keyEventCh:
			sm.handleKeyEvent(event)
		default:
			// No more events to process
			return
		}
	}
}

// handleKeyEvent processes key events for settings
func (sm *SettingsModule) handleKeyEvent(event KeyEvent) {
	// Only handle key press events
	if !event.Pressed {
		return
	}

	switch event.Key {
	case ebiten.KeyEqual, ebiten.KeyNumpadAdd: // '+' key or numpad +
		if sm.volume < 100 {
			sm.volume += 10
			fmt.Printf("[SETTINGS] Volume increased to %d%%\n", sm.volume)
		}
	case ebiten.KeyMinus, ebiten.KeyNumpadSubtract: // '-' key or numpad -
		if sm.volume > 0 {
			sm.volume -= 10
			fmt.Printf("[SETTINGS] Volume decreased to %d%%\n", sm.volume)
		}
	case ebiten.KeyF:
		sm.showFPS = !sm.showFPS
		fmt.Printf("[SETTINGS] Show FPS: %t\n", sm.showFPS)
	case ebiten.KeyW:
		// Toggle window mode
		switch sm.windowMode {
		case "Windowed":
			sm.windowMode = "Fullscreen"
		case "Fullscreen":
			sm.windowMode = "Borderless"
		case "Borderless":
			sm.windowMode = "Windowed"
		default:
			sm.windowMode = "Windowed"
		}
		fmt.Printf("[SETTINGS] Window Mode: %s\n", sm.windowMode)
	case ebiten.KeyV:
		sm.vsync = !sm.vsync
		fmt.Printf("[SETTINGS] VSync: %t\n", sm.vsync)
	case ebiten.KeyA:
		sm.autoSave = !sm.autoSave
		fmt.Printf("[SETTINGS] Auto Save: %t\n", sm.autoSave)
	case ebiten.KeyL:
		// Cycle through languages
		switch sm.language {
		case "English":
			sm.language = "Spanish"
		case "Spanish":
			sm.language = "French"
		case "French":
			sm.language = "German"
		case "German":
			sm.language = "Japanese"
		case "Japanese":
			sm.language = "English"
		default:
			sm.language = "English"
		}
		fmt.Printf("[SETTINGS] Language: %s\n", sm.language)
	case ebiten.KeyG:
		sm.gpuAcceleration = !sm.gpuAcceleration
		fmt.Printf("[SETTINGS] GPU Acceleration: %t\n", sm.gpuAcceleration)
		// Apply the setting to the runtime
		sm.applyGPUAccelerationSetting()
	case ebiten.KeyR:
		sm.resetToDefaults()
	case ebiten.KeyS:
		sm.saveSettings()
	case ebiten.KeyEscape:
		sm.SetActive(false)
		fmt.Println("[SETTINGS] Settings menu closed")
	}
}

// SetActive enables or disables the settings module
func (sm *SettingsModule) SetActive(active bool) {
	sm.active = active
	if active {
		log.Println("[SETTINGS] Settings menu opened")
		sm.printHelp()
	} else {
		log.Println("[SETTINGS] Settings menu closed")
	}
}

// IsActive returns whether the settings module is active
func (sm *SettingsModule) IsActive() bool {
	return sm.active
}

// printHelp displays available settings controls
func (sm *SettingsModule) printHelp() {
	fmt.Println("[SETTINGS] Controls:")
	fmt.Println("  +/-     : Adjust volume")
	fmt.Println("  F       : Toggle FPS display")
	fmt.Println("  W       : Cycle window mode")
	fmt.Println("  V       : Toggle VSync")
	fmt.Println("  A       : Toggle auto save")
	fmt.Println("  L       : Cycle language")
	fmt.Println("  G       : Toggle GPU acceleration")
	fmt.Println("  R       : Reset to defaults")
	fmt.Println("  S       : Save settings")
	fmt.Println("  ESC     : Close settings")
	fmt.Printf("[SETTINGS] Current - Volume: %d%%, Show FPS: %t, Window: %s, VSync: %t, Auto Save: %t, Language: %s, GPU: %t\n",
		sm.volume, sm.showFPS, sm.windowMode, sm.vsync, sm.autoSave, sm.language, sm.gpuAcceleration)
}

// applyGPUAccelerationSetting applies the GPU acceleration setting to the runtime
func (sm *SettingsModule) applyGPUAccelerationSetting() {
	// Import the eruntime package to access GPU compute functionality
	if sm.gpuAcceleration {
		// Enable GPU acceleration
		if err := sm.setComputeMode("hybrid"); err != nil {
			fmt.Printf("[SETTINGS] Failed to enable GPU acceleration: %v\n", err)
			sm.gpuAcceleration = false // Revert on failure
		}
	} else {
		// Disable GPU acceleration (CPU only)
		if err := sm.setComputeMode("cpu"); err != nil {
			fmt.Printf("[SETTINGS] Failed to disable GPU acceleration: %v\n", err)
		}
	}
}

// checkGPUAvailability checks if GPU acceleration is available and updates settings accordingly
func (sm *SettingsModule) checkGPUAvailability() {
	if eruntime.GlobalComputeController != nil && eruntime.GlobalComputeController.IsGPUAvailable() {
		fmt.Println("[SETTINGS] GPU acceleration is available")
		// GPU is available but we start with it disabled for safety
	} else {
		fmt.Println("[SETTINGS] GPU acceleration is not available")
		sm.gpuAcceleration = false
	}
}

// setComputeMode sets the compute mode in the runtime
func (sm *SettingsModule) setComputeMode(mode string) error {
	if eruntime.GlobalComputeController != nil {
		return eruntime.GlobalComputeController.SetComputeMode(mode)
	}
	fmt.Printf("[SETTINGS] GPU compute system not available, mode: %s\n", mode)
	return fmt.Errorf("GPU compute system not available")
}

// resetToDefaults resets all settings to their default values
func (sm *SettingsModule) resetToDefaults() {
	sm.volume = 50
	sm.showFPS = true
	sm.windowMode = "Windowed"
	sm.vsync = true
	sm.autoSave = true
	sm.language = "English"
	sm.gpuAcceleration = false
	sm.applyGPUAccelerationSetting() // Apply the reset GPU setting
	fmt.Println("[SETTINGS] Settings reset to defaults")
}

// saveSettings simulates saving settings to a file
func (sm *SettingsModule) saveSettings() {
	fmt.Printf("[SETTINGS] Settings saved - Volume: %d%%, Show FPS: %t, Window Mode: %s, VSync: %t, Auto Save: %t, Language: %s\n",
		sm.volume, sm.showFPS, sm.windowMode, sm.vsync, sm.autoSave, sm.language)
}

// GetVolume returns the current volume setting
func (sm *SettingsModule) GetVolume() int {
	return sm.volume
}

// GetShowFPS returns whether FPS should be shown
func (sm *SettingsModule) GetShowFPS() bool {
	return sm.showFPS
}

// GetWindowMode returns the current window mode
func (sm *SettingsModule) GetWindowMode() string {
	return sm.windowMode
}

// GetVSync returns whether VSync is enabled
func (sm *SettingsModule) GetVSync() bool {
	return sm.vsync
}

// GetAutoSave returns whether auto save is enabled
func (sm *SettingsModule) GetAutoSave() bool {
	return sm.autoSave
}

// GetLanguage returns the current language
func (sm *SettingsModule) GetLanguage() string {
	return sm.language
}

// DrawSettings draws the settings menu on the screen
func (sm *SettingsModule) DrawSettings(screen *ebiten.Image) {
	if !sm.active {
		return
	}

	// Get scaling information based on screen size
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	// Calculate scale factor (same logic as in app.go)
	baseWidth := 800
	baseHeight := 600
	scaleX := float64(screenW) / float64(baseWidth)
	scaleY := float64(screenH) / float64(baseHeight)
	scaleFactor := scaleX
	if scaleY < scaleX {
		scaleFactor = scaleY
	}
	if scaleFactor < 0.5 {
		scaleFactor = 0.5
	} else if scaleFactor > 3.0 {
		scaleFactor = 3.0
	}

	// Calculate scaled position for the overlay
	x := int(20 * scaleFactor)
	y := int(20 * scaleFactor)

	// Draw a simple settings menu overlay
	text := fmt.Sprintf("Settings Menu\nVolume: %d%%\nShow FPS: %t\nWindow Mode: %s\nVSync: %t\nAuto Save: %t\nLanguage: %s\n\nControls:\n+/- Volume | F FPS | W Window | V VSync\nA AutoSave | L Language | R Reset | S Save\nPress ESC to close",
		sm.volume, sm.showFPS, sm.windowMode, sm.vsync, sm.autoSave, sm.language)
	ebitenutil.DebugPrintAt(screen, text, x, y)

	// Additional drawing logic can be added here
}
