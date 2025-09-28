package app

import (
	"RueaES/eruntime"
	"fmt"
	"image/color"
	"math"
	"runtime"
	"strconv"
	"time"
)

// StateManagementMenu manages the state control edge menu
// Only shows in map view

// Logarithmic scaling helpers for the rate slider
// Maps linear slider position (0-100) to logarithmic rate value (1-300) for web, this is to prevent the page from locking up
func linearToLogRate(linear float64) float64 {
	if linear <= 0 {
		return 1.0 // Minimum rate value
	}
	// Use logarithmic scale: rate = 1 * (300/1)^(linear/100)
	// This gives us a smooth scale from 1 to 300 TPS
	ratio := linear / 100.0
	return 1.0 * math.Pow(300/1.0, ratio)
}

func logToLinearRate(rate int) float64 {
	if rate <= 1 {
		return 0
	}
	// Inverse of the above: linear = 100 * log(rate/1) / log(300/1)
	return 100.0 * math.Log(float64(rate)/1.0) / math.Log(300.0/1.0)
}

func advanceTickFunc(smm *StateManagementMenu) {
	// Limit the number of ticks to advance at once to prevent excessive processing
	maxTicks := smm.addTickValue
	if maxTicks > 600 {
		maxTicks = 600 // Cap at 12000 ticks maximum (about 3.3 hours of simulation)
	}

	// Always run tick advancement in a separate goroutine to prevent UI blocking
	// and ensure ticks are processed sequentially to avoid mutex deadlocks
	go func() {
		for i := 0; i < maxTicks; i++ {
			eruntime.NextTick()
			// Add a small delay every 60 ticks to prevent overwhelming the system
			if i%60 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()
}

type StateManagementMenu struct {
	menu         *EdgeMenu
	rateValue    int // Changed to int for whole number ticks
	rateInput    string
	addTickValue int
	addTickInput string
	halted       bool

	// Store references to UI elements for synchronization
	rateSlider    *MenuSlider
	rateTextInput *MenuTextInput
	statsText     *MenuText // Reference to stats text for updates

	// Track time for stats updates
	lastStatsUpdate float64
}

func NewStateManagementMenu() *StateManagementMenu {
	options := DefaultEdgeMenuOptions()
	options.Position = EdgeMenuLeft
	options.Width = 340
	options.Background = color.RGBA{30, 30, 45, 240}
	menu := NewEdgeMenu("State Management", options)
	return &StateManagementMenu{
		menu:         menu,
		rateValue:    1,   // Changed to int, default 1 tick
		rateInput:    "1", // Changed to match int value
		addTickValue: 60,
		addTickInput: "60",
		halted:       eruntime.IsHalted(),
	}
}

func (smm *StateManagementMenu) Show() {
	// Ensure rateInput is synchronized with rateValue before building UI
	smm.rateInput = strconv.Itoa(smm.rateValue)

	smm.menu.ClearElements()

	// Clear stored element references when rebuilding
	smm.rateSlider = nil
	smm.rateTextInput = nil
	smm.statsText = nil

	// --- Tick Control (collapsible) ---
	collapsibleOpts := DefaultCollapsibleMenuOptions()
	collapsibleOpts.Collapsed = false

	tickSection := smm.menu.CollapsibleMenu("Tick Control", collapsibleOpts)

	tickSection.Spacer(DefaultSpacerOptions())

	// Rate slider (logarithmic scale for better control)
	sliderOpts := DefaultSliderOptions()
	sliderOpts.MinValue = 0   // Linear slider range 0-100
	sliderOpts.MaxValue = 100 // Will be mapped to logarithmic 1-1000
	sliderOpts.Step = 1
	sliderOpts.ValueFormat = "%.0f"
	sliderOpts.ShowValue = false // Hide slider value, we'll show actual rate separately

	// Set fill color based on current halted state
	if eruntime.IsHalted() {
		sliderOpts.FillColor = color.RGBA{128, 128, 128, 255} // Grey when halted
	} else {
		sliderOpts.FillColor = color.RGBA{100, 150, 255, 255} // Blue when running
	}

	// Convert current rate to linear slider position
	sliderValue := logToLinearRate(smm.rateValue)

	// Create the slider and store the reference
	smm.rateSlider = NewMenuSlider("Rate", sliderValue, sliderOpts, func(val float64) {
		// Convert linear slider value to logarithmic rate and round to integer
		smm.rateValue = int(math.Round(linearToLogRate(val)))
		smm.rateInput = strconv.Itoa(smm.rateValue)
		// Update the text input to reflect the new value
		if smm.rateTextInput != nil {
			smm.rateTextInput.SetValue(smm.rateInput)
		}
		// Update the actual eruntime tick rate
		eruntime.SetTickRate(smm.rateValue)
		// Don't refresh menu during slider dragging to avoid interference
	})
	tickSection.AddElement(smm.rateSlider)

	// Rate text input (positioned to align with slider)
	textInputOpts := DefaultTextInputOptions()
	textInputOpts.Width = 120
	textInputOpts.Placeholder = "Enter rate..."
	textInputOpts.ValidateInput = func(newValue string) bool {
		if newValue == "" {
			return true
		}
		if v, err := strconv.Atoi(newValue); err == nil {
			return v >= 1 && v <= 300 // Updated to match practical maximum
		}
		return false
	}

	// Create the text input and store the reference
	smm.rateTextInput = NewMenuTextInput("Rate Value", smm.rateInput, textInputOpts, func(val string) {
		smm.rateInput = val
		if v, err := strconv.Atoi(val); err == nil {
			// Clamp the manually entered value to valid range
			if v < 1 {
				v = 1
			} else if v > 300 {
				v = 300 // Cap at 300 TPS (practical maximum simulation speed)
			}
			smm.rateValue = v
			// Update the slider to reflect the new value
			if smm.rateSlider != nil {
				newSliderValue := logToLinearRate(smm.rateValue)
				smm.rateSlider.SetValue(newSliderValue)
			}
			// Update the actual eruntime tick rate
			eruntime.SetTickRate(smm.rateValue)
		}
		// Don't refresh menu during text input to avoid interference
	})
	tickSection.AddElement(smm.rateTextInput)

	tickSection.Spacer(DefaultSpacerOptions())

	// Add Tick input (integer, default 60, cannot be negative)
	addTickInputOpts := DefaultTextInputOptions()
	addTickInputOpts.Width = 120
	addTickInputOpts.Placeholder = "Tick count"
	addTickInputOpts.ValidateInput = func(newValue string) bool {
		if newValue == "" {
			return true
		}
		if v, err := strconv.Atoi(newValue); err == nil {
			return v >= 0
		}
		return false
	}

	tickSection.TextInput("Add Tick", smm.addTickInput, addTickInputOpts, func(val string) {
		smm.addTickInput = val
		if v, err := strconv.Atoi(val); err == nil && v >= 0 {
			smm.addTickValue = v
		}
		// No need to refresh menu here
	})

	// Advance N tick button
	advanceLabel := "Advance Tick"
	advanceButtonOpts := DefaultButtonOptions()
	advanceButtonOpts.BackgroundColor = color.RGBA{100, 100, 100, 255} // Grey
	advanceButtonOpts.HoverColor = color.RGBA{120, 120, 120, 255}
	advanceButtonOpts.PressedColor = color.RGBA{80, 80, 80, 255}
	advanceButtonOpts.BorderColor = color.RGBA{140, 140, 140, 255}

	tickSection.Button(advanceLabel, advanceButtonOpts, func() {
		advanceTickFunc(smm)
	})

	tickSection.Spacer(DefaultSpacerOptions())

	// Halt/Resume button
	haltLabel := "Halt"
	if eruntime.IsHalted() {
		haltLabel = "Resume"
	}
	haltButtonOpts := DefaultButtonOptions()
	haltButtonOpts.BackgroundColor = color.RGBA{200, 180, 60, 255} // Yellow
	haltButtonOpts.HoverColor = color.RGBA{220, 200, 80, 255}
	haltButtonOpts.PressedColor = color.RGBA{180, 160, 40, 255}
	haltButtonOpts.BorderColor = color.RGBA{240, 220, 100, 255}
	haltButtonOpts.TextColor = color.RGBA{0, 0, 0, 255} // Black text for better readability on yellow

	tickSection.Button(haltLabel, haltButtonOpts, func() {
		if eruntime.IsHalted() {
			eruntime.Resume()
		} else {
			eruntime.Halt()
		}
		// Update slider fill color immediately based on new halted state
		if smm.rateSlider != nil {
			if smm.halted {
				// Set to grey when halted
				smm.rateSlider.SetFillColor(color.RGBA{128, 128, 128, 255})
			} else {
				// Set to blue when resumed
				smm.rateSlider.SetFillColor(color.RGBA{100, 150, 255, 255})
			}
		}

		// Refresh the menu to update the button label
		smm.Show()
	})

	// Stats display
	tickSection.Spacer(DefaultSpacerOptions())

	// Get elapsed ticks and convert to hours:minutes:seconds
	elapsedTicks := eruntime.Elapsed()
	hours := elapsedTicks / 3600
	minutes := (elapsedTicks % 3600) / 60
	seconds := elapsedTicks % 60

	statsText := fmt.Sprintf("Elapsed: %d tick (%02d:%02d:%02d)", elapsedTicks, hours, minutes, seconds)

	statsTextOpts := DefaultTextOptions()
	statsTextOpts.FontSize = 14
	statsTextOpts.Color = color.RGBA{200, 200, 200, 255} // Light grey

	// Create and store reference to stats text for later updates
	smm.statsText = NewMenuText(statsText, statsTextOpts)
	tickSection.AddElement(smm.statsText)

	// --- Options :3 ---
	optionsSection := smm.menu.CollapsibleMenu("Options", DefaultCollapsibleMenuOptions())

	// Treasure toggle switch
	Options := []string{"Enabled", "Disabled"}
	treasureToggleOpts := DefaultToggleSwitchOptions()
	treasureToggleOpts.Options = Options
	treasureToggleCBFunc := func(index int, value string) {
		if value == "Enabled" {
			currOpts := eruntime.GetRuntimeOptions()
			currOpts.TreasuryEnabled = true
			eruntime.SetRuntimeOptions(currOpts)
		} else {
			currOpts := eruntime.GetRuntimeOptions()
			currOpts.TreasuryEnabled = false
			eruntime.SetRuntimeOptions(currOpts)
		}
	}

	smallSpacerOpts := DefaultSpacerOptions()
	smallSpacerOpts.Height = 10 // Smaller spacer for compact layout

	optionsSection.Text("Treasury calculation is based on", DefaultTextOptions()) // Empty text to ensure proper spacing
	optionsSection.Text("territory time held", DefaultTextOptions())              // Empty text to ensure proper spacing
	optionsSection.Spacer(smallSpacerOpts)
	optionsSection.ToggleSwitch("Treasury Calculation", 0, treasureToggleOpts, treasureToggleCBFunc)

	optionsSection.Spacer(DefaultSpacerOptions())

	// Encode Treasury toggle switch
	options := []string{"Enabled", "Disabled"}
	encodeTreasuryToggleOpts := DefaultToggleSwitchOptions()
	encodeTreasuryToggleOpts.Options = options
	encodeTreasuryToggleCBFunc := func(index int, value string) {
		if value == "Enabled" {
			currOpts := eruntime.GetRuntimeOptions()
			currOpts.EncodeInTransitResources = true
			eruntime.SetRuntimeOptions(currOpts)
		} else {
			currOpts := eruntime.GetRuntimeOptions()
			currOpts.EncodeInTransitResources = false
			eruntime.SetRuntimeOptions(currOpts)
		}
	}

	optionsSection.Spacer(DefaultSpacerOptions())
	optionsSection.Text("Encoding In-Transit Resources", DefaultTextOptions())
	optionsSection.Text("may increase impact load time.", DefaultTextOptions())
	optionsSection.ToggleSwitch("Encode In-Transit Resources", 0, encodeTreasuryToggleOpts, encodeTreasuryToggleCBFunc)

	// --- Credits ---
	creditsSection := smm.menu.CollapsibleMenu("Credits", DefaultCollapsibleMenuOptions())
	creditsSection.Text("Ruea Economy Studio", DefaultTextOptions())
	creditsSection.Text("Developed by: AMS/tragedia/WayLessSad", DefaultTextOptions())
	creditsSection.Text("Inspired by farog's economy simulator", DefaultTextOptions())
	creditsSection.Spacer(DefaultSpacerOptions())
	creditsSection.Text("Add me on Discord: @.tragedia.", DefaultTextOptions())

	// --- Load and Save ---
	loadSaveSection := smm.menu.CollapsibleMenu("Load and Save", DefaultCollapsibleMenuOptions())

	// Save and Load buttons with greyish-blue colors
	saveLoadButtonOpts := DefaultButtonOptions()
	saveLoadButtonOpts.BackgroundColor = color.RGBA{70, 90, 120, 255} // Greyish-blue
	saveLoadButtonOpts.HoverColor = color.RGBA{90, 110, 140, 255}
	saveLoadButtonOpts.PressedColor = color.RGBA{50, 70, 100, 255}
	saveLoadButtonOpts.BorderColor = color.RGBA{100, 120, 150, 255}

	loadSaveSection.Button("Save Session", saveLoadButtonOpts, func() {
		// fmt.Println("[STATE_MGMT] Save Session button clicked")
		// Check if running in WASM environment
		if runtime.GOOS == "js" && runtime.GOARCH == "wasm" {
			// Use browser file download for WASM
			saveSessionToBrowser()
		} else {
			// Trigger save file dialogue for native builds
			if fileManager := GetFileSystemManager(); fileManager != nil {
				// fmt.Println("[STATE_MGMT] File system manager found, showing save dialogue")
				fileManager.ShowSaveDialogue()
			} else {
				// fmt.Println("[STATE_MGMT] File system manager not available")
			}
		}
	})
	loadSaveSection.Button("Load Session", saveLoadButtonOpts, func() {
		// fmt.Println("[STATE_MGMT] Load Session button clicked")
		// Check if running in WASM environment
		if runtime.GOOS == "js" && runtime.GOARCH == "wasm" {
			// Use browser file picker for WASM
			loadSessionFromBrowser()
		} else {
			// Trigger load file dialogue for native builds
			if fileManager := GetFileSystemManager(); fileManager != nil {
				fileManager.ShowOpenDialogue()
			} else {
				// fmt.Println("[STATE_MGMT] File system manager not available")
			}
		}
	})

	// Add spacer before Reset button
	loadSaveSection.Spacer(DefaultSpacerOptions())

	// Reset button with red color
	resetButtonOpts := DefaultButtonOptions()
	resetButtonOpts.BackgroundColor = color.RGBA{180, 60, 60, 255} // Red
	resetButtonOpts.HoverColor = color.RGBA{200, 80, 80, 255}
	resetButtonOpts.PressedColor = color.RGBA{160, 40, 40, 255}
	resetButtonOpts.BorderColor = color.RGBA{220, 100, 100, 255}

	loadSaveSection.Button("Reset", resetButtonOpts, func() {
		// fmt.Println("[STATE_MGMT] Calling eruntime.Reset()...")
		eruntime.Reset()
		// fmt.Println("[STATE_MGMT] Reset completed")
	})

	smm.menu.Show()
}

// Update method - only update stats text, not entire menu
func (smm *StateManagementMenu) Update(deltaTime float64) {
	// Update stats text every 200ms for smooth real-time updates
	smm.lastStatsUpdate += deltaTime
	if smm.lastStatsUpdate >= 0.05 {
		smm.lastStatsUpdate = 0.0

		// Only update if menu is visible
		if smm.menu != nil && smm.menu.IsVisible() {
			// Check if halt state changed
			currentHalted := eruntime.IsHalted()
			if currentHalted != smm.halted {
				smm.halted = currentHalted

				// Update slider fill color based on halted state
				if smm.rateSlider != nil {
					if currentHalted {
						// Set to grey when halted
						smm.rateSlider.SetFillColor(color.RGBA{128, 128, 128, 255})
					} else {
						// Set to blue when resumed
						smm.rateSlider.SetFillColor(color.RGBA{100, 150, 255, 255})
					}
				}

				smm.Show() // Full rebuild for halt state change
			} else if smm.statsText != nil {
				// Only update the stats text without rebuilding entire menu
				elapsedTicks := eruntime.Elapsed()
				hours := elapsedTicks / 3600
				minutes := (elapsedTicks % 3600) / 60
				seconds := elapsedTicks % 60

				newStatsText := fmt.Sprintf("Elapsed: %d tick (%02d:%02d:%02d)", elapsedTicks, hours, minutes, seconds)
				smm.statsText.SetText(newStatsText)
			}
		}
	}
}

// Attach to MapView and show only in map view
type MapViewWithStateMenu struct {
	*MapView
	stateMenu *StateManagementMenu
}

func NewMapViewWithStateMenu() *MapViewWithStateMenu {
	mv := NewMapView()
	stateMenu := NewStateManagementMenu()
	return &MapViewWithStateMenu{MapView: mv, stateMenu: stateMenu}
}

// Call this in your map view update/draw
func (m *MapViewWithStateMenu) ShowStateMenuIfMapView() {
	if m != nil && m.stateMenu != nil {
		m.stateMenu.Show()
	}
}

// saveSessionToBrowser saves the session using the browser's download API
func saveSessionToBrowser() {
	// fmt.Println("[STATE_MGMT] Saving session to browser")

	// Get state data as bytes using eruntime function
	stateBytes, err := eruntime.SaveStateToBytes()
	if err != nil {
		// fmt.Printf("[STATE_MGMT] Error getting state bytes: %v\n", err)
		NewToast().
			Text("Error saving session: "+err.Error(), ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			AutoClose(time.Second * 5).
			Show()
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("ruea_session_%s.lz4", timestamp)

	// Use web file operations to download
	err = WebSaveFile(filename, stateBytes)
	if err != nil {
		// fmt.Printf("[STATE_MGMT] Error downloading file: %v\n", err)
		NewToast().
			Text("Error downloading session: "+err.Error(), ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			AutoClose(time.Second * 5).
			Show()
		return
	}

	NewToast().
		Text("Session saved successfully", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// loadSessionFromBrowser loads the session using the browser's file picker API
func loadSessionFromBrowser() {
	// fmt.Println("[STATE_MGMT] Loading session from browser")

	// Use web file operations to pick file
	stateBytes, filename, err := WebLoadFile()
	if err != nil {
		if err.Error() != "user cancelled file selection" {
			// fmt.Printf("[STATE_MGMT] Error loading file: %v\n", err)
			NewToast().
				Text("Error loading session: "+err.Error(), ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
				AutoClose(time.Second * 5).
				Show()
		}
		return
	}

	// fmt.Printf("[STATE_MGMT] Loaded file: %s (%d bytes)\n", filename, len(stateBytes))

	// Load state from bytes using eruntime function
	err = eruntime.LoadStateFromBytes(stateBytes)
	if err != nil {
		// fmt.Printf("[STATE_MGMT] Error loading state: %v\n", err)
		NewToast().
			Text("Error loading session: "+err.Error(), ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			AutoClose(time.Second * 5).
			Show()
		return
	}

	NewToast().
		Text("Session loaded successfully from "+filename, ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
		AutoClose(time.Second * 3).
		Show()
}

// getCurrentSessionData returns the current session as JSON string
func getCurrentSessionData() string {
	// TODO: Implement this based on your session structure
	// This should serialize the current game state to JSON
	return "{}" // Placeholder
}

// loadSessionFromData loads session from JSON string
func loadSessionFromData(data string) error {
	// TODO: Implement this based on your session structure
	// This should parse and load the session data
	// fmt.Printf("[STATE_MGMT] Would load session data: %s\n", data)
	return nil
}
