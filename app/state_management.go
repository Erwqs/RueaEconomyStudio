package app

import (
	"RueaES/eruntime"
	"RueaES/pluginhost"
	"RueaES/typedef"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// StateManagementMenu manages the state control edge menu
// Only shows in map view

// Logarithmic scaling helpers for the rate slider
// Maps linear slider position (0-100) to logarithmic rate value (1-7200)
func linearToLogRate(linear float64) float64 {
	if linear <= 0 {
		return 1.0 // Minimum rate value
	}
	// Use logarithmic scale: rate = 1 * (7200/1)^(linear/100)
	// This gives us a smooth scale from 1 to 7200 TPS
	ratio := linear / 100.0
	return 1.0 * math.Pow(7200.0/1.0, ratio)
}

func logToLinearRate(rate int) float64 {
	if rate <= 1 {
		return 0
	}
	// Inverse of the above: linear = 100 * log(rate/1) / log(7200/1)
	return 100.0 * math.Log(float64(rate)/1.0) / math.Log(7200.0/1.0)
}

// Map slider position (0-100) to emerald weight on a log scale between 0.1x and 10x.
func sliderToChokepointWeight(pos float64) float64 {
	minW, maxW := 0.1, 10.0
	if pos < 0 {
		pos = 0
	}
	if pos > 100 {
		pos = 100
	}
	ratio := pos / 100.0
	return minW * math.Pow(maxW/minW, ratio)
}

// Map emerald weight back to slider position so 1.0 sits at the midpoint.
func chokepointWeightToSlider(weight float64) float64 {
	minW, maxW := 0.1, 10.0
	if weight < minW {
		weight = minW
	}
	if weight > maxW {
		weight = maxW
	}
	return math.Log(weight/minW) / math.Log(maxW/minW) * 100.0
}

func advanceTickFunc(smm *StateManagementMenu) {
	// Limit the number of ticks to advance at once to prevent excessive processing
	maxTicks := smm.addTickValue
	if maxTicks > 12000 {
		maxTicks = 12000 // Cap at 12000 ticks maximum (about 3.3 hours of simulation)
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
	rateSlider      *MenuSlider
	rateTextInput   *MenuTextInput
	statsText       *MenuText // Reference to stats text for updates
	mapOpacity      *MenuSlider
	mapColorBtns    map[string]*MenuColorButton
	mapColorPicker  *ColorPicker
	currentColorKey string
	keybindInputs   map[string]*MenuTextInput

	// Track time for stats updates
	lastStatsUpdate float64
}

func rgbaToHex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func resourceColorForKey(opts typedef.ResourceColorOptions, key string) color.RGBA {
	switch key {
	case "Wood":
		return opts.Wood.ToRGBA()
	case "Crop":
		return opts.Crop.ToRGBA()
	case "Fish":
		return opts.Fish.ToRGBA()
	case "Ore":
		return opts.Ore.ToRGBA()
	case "Multi":
		return opts.Multi.ToRGBA()
	case "Emerald":
		return opts.Emerald.ToRGBA()
	default:
		return color.RGBA{255, 255, 255, 255}
	}
}

func (smm *StateManagementMenu) openMapColorPicker(key string) {
	if smm.mapColorPicker == nil {
		return
	}

	smm.currentColorKey = key
	runtimeOpts := eruntime.GetRuntimeOptions()
	currentColor := resourceColorForKey(runtimeOpts.ResourceColors, key)

	// Recenter picker each open
	screenW, screenH := ebiten.WindowSize()
	pickerW := smm.mapColorPicker.width
	pickerH := smm.mapColorPicker.height
	smm.mapColorPicker.x = (screenW - pickerW) / 2
	smm.mapColorPicker.y = (screenH - pickerH) / 2

	hex := rgbaToHex(currentColor)
	if hex == "" {
		hex = "#FFFFFF"
	}

	// Wire callbacks
	smm.mapColorPicker.onConfirm = func(selected color.RGBA) {
		opts := eruntime.GetRuntimeOptions()
		switch smm.currentColorKey {
		case "Wood":
			opts.ResourceColors.Wood = typedef.RGBAColorFromColor(selected)
		case "Crop":
			opts.ResourceColors.Crop = typedef.RGBAColorFromColor(selected)
		case "Fish":
			opts.ResourceColors.Fish = typedef.RGBAColorFromColor(selected)
		case "Ore":
			opts.ResourceColors.Ore = typedef.RGBAColorFromColor(selected)
		case "Multi":
			opts.ResourceColors.Multi = typedef.RGBAColorFromColor(selected)
		case "Emerald":
			opts.ResourceColors.Emerald = typedef.RGBAColorFromColor(selected)
		}
		eruntime.SetRuntimeOptions(opts)

		if btn, ok := smm.mapColorBtns[smm.currentColorKey]; ok {
			btn.SetColor(selected)
		}
	}

	smm.mapColorPicker.onCancel = func() {
		smm.currentColorKey = ""
	}

	smm.mapColorPicker.Show(-1, hex)
}

func NewStateManagementMenu() *StateManagementMenu {
	options := DefaultEdgeMenuOptions()
	options.Position = EdgeMenuLeft
	options.Width = 340
	options.Background = color.RGBA{30, 30, 45, 240}
	menu := NewEdgeMenu("State Management", options)
	smm := &StateManagementMenu{
		menu:          menu,
		rateValue:     1,   // Changed to int, default 1 tick
		rateInput:     "1", // Changed to match int value
		addTickValue:  60,
		addTickInput:  "60",
		halted:        eruntime.IsHalted(),
		mapColorBtns:  make(map[string]*MenuColorButton),
		keybindInputs: make(map[string]*MenuTextInput),
	}

	// Initialize map color picker centered on screen
	screenW, screenH := ebiten.WindowSize()
	pickerW := 400
	pickerH := 350
	pickerX := (screenW - pickerW) / 2
	pickerY := (screenH - pickerH) / 2
	smm.mapColorPicker = NewColorPicker(pickerX, pickerY, pickerW, pickerH)

	return smm
}

func (smm *StateManagementMenu) Show() {
	// Ensure rateInput is synchronized with rateValue before building UI
	smm.rateInput = strconv.Itoa(smm.rateValue)

	smm.menu.ClearElements()

	// Clear stored element references when rebuilding
	smm.rateSlider = nil
	smm.rateTextInput = nil
	smm.statsText = nil
	smm.mapOpacity = nil
	smm.mapColorBtns = make(map[string]*MenuColorButton)
	smm.keybindInputs = make(map[string]*MenuTextInput)

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
			return v >= 1 && v <= 7200 // Updated to match practical maximum
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
			} else if v > 7200 {
				v = 7200 // Cap at 7200 TPS (practical maximum simulation speed)
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

	optionsSection.Spacer(DefaultSpacerOptions())

	// Pathfinding Algorithm toggle switch
	pathfindingOptions := []string{"Dijkstra", "A*", "FloodFill"}
	pathfindingToggleOpts := DefaultToggleSwitchOptions()
	pathfindingToggleOpts.Options = pathfindingOptions
	pathfindingToggleOpts.Width = 210 // Wider to accommodate 3 options
	pathfindingToggleOpts.FontSize = 12
	pathfindingToggleCBFunc := func(index int, value string) {
		currOpts := eruntime.GetRuntimeOptions()
		switch value {
		case "Dijkstra":
			currOpts.PathfindingAlgorithm = typedef.PathfindingDijkstra
		case "A*":
			currOpts.PathfindingAlgorithm = typedef.PathfindingAstar
		case "FloodFill":
			currOpts.PathfindingAlgorithm = typedef.PathfindingFloodFill
		}
		eruntime.SetRuntimeOptions(currOpts)
	}

	optionsSection.Text("Pathfinding Alg for Cheapest", DefaultTextOptions())

	currentOpts := eruntime.GetRuntimeOptions()
	var currentPathfindingIndex int
	switch currentOpts.PathfindingAlgorithm {
	case typedef.PathfindingDijkstra:
		currentPathfindingIndex = 0
	case typedef.PathfindingAstar:
		currentPathfindingIndex = 1
	case typedef.PathfindingFloodFill:
		currentPathfindingIndex = 2
	default:
		currentPathfindingIndex = 0 // default when something doesnt work
	}

	optionsSection.ToggleSwitch("Algorithm", currentPathfindingIndex, pathfindingToggleOpts, pathfindingToggleCBFunc)

	// Pathfinder provider selector (dropdown to handle many entries)
	providers := pluginhost.ListPathfinderProviders()
	if len(providers) > 0 {
		providerOptions := []FilterableDropdownOption{{Display: "Built-in", Value: ""}}
		for _, p := range providers {
			display := fmt.Sprintf("%s (%s)", p.Name, p.PluginID)
			providerOptions = append(providerOptions, FilterableDropdownOption{Display: display, Value: p.Key})
		}

		dropdown := NewFilterableDropdown(0, 0, 260, 32, providerOptions, func(option FilterableDropdownOption) {
			opts := eruntime.GetRuntimeOptions()
			opts.PathfinderProvider = option.Value
			eruntime.SetRuntimeOptions(opts)
		})

		// Preselect current choice.
		dropdown.SetSelected(currentOpts.PathfinderProvider)

		optionsSection.Text("Pathfinder Provider", DefaultTextOptions())
		optionsSection.AddElement(newDropdownEdgeElement(dropdown, nil, 40))
		dropdown.SetPlaceholder("Select a pathfinder provider...")
	}

	// Cost provider selector (dropdown; only shown when cost plugins are present)
	costProviders := pluginhost.ListCostProviders()
	if len(costProviders) > 0 {
		costOptions := []FilterableDropdownOption{{Display: "Built-in", Value: ""}}
		for _, p := range costProviders {
			display := fmt.Sprintf("Plugin (%s)", p.PluginID)
			costOptions = append(costOptions, FilterableDropdownOption{Display: display, Value: p.PluginID})
		}

		costDropdown := NewFilterableDropdown(0, 0, 260, 32, costOptions, func(option FilterableDropdownOption) {
			opts := eruntime.GetRuntimeOptions()
			if option.Value == "" {
				if err := eruntime.ReloadDefaultCosts(); err != nil {
					return
				}
			} else {
				payload, ok := pluginhost.CostProviderPayload(option.Value)
				if !ok {
					return
				}
				if err := eruntime.SetCostsFromMap(payload); err != nil {
					return
				}
			}
			opts.CostProvider = option.Value
			eruntime.SetRuntimeOptions(opts)
		})

		costDropdown.SetSelected(currentOpts.CostProvider)

		optionsSection.Text("Cost Provider", DefaultTextOptions())
		optionsSection.AddElement(newDropdownEdgeElement(costDropdown, nil, 40))
		costDropdown.SetPlaceholder("Select a cost provider...")
	}

	// Keybind configuration
	optionsSection.Spacer(DefaultSpacerOptions())

	keybindSectionOptions := DefaultCollapsibleMenuOptions()
	keybindSectionOptions.Collapsed = true
	keybindSection := optionsSection.CollapsibleMenu("Keybinds", keybindSectionOptions)
	keybindSection.Text("Leave blank to disable.", DefaultTextOptions())

	keybindFields := []struct {
		name   string
		label  string
		getter func(typedef.Keybinds) string
		setter func(*typedef.Keybinds, string)
	}{
		{name: "analysis", label: "Analysis Modal", getter: func(k typedef.Keybinds) string { return k.AnalysisModal }, setter: func(k *typedef.Keybinds, v string) { k.AnalysisModal = v }},
		{name: "state", label: "State Menu", getter: func(k typedef.Keybinds) string { return k.StateMenu }, setter: func(k *typedef.Keybinds, v string) { k.StateMenu = v }},
		{name: "tribute", label: "Tribute Menu", getter: func(k typedef.Keybinds) string { return k.TributeMenu }, setter: func(k *typedef.Keybinds, v string) { k.TributeMenu = v }},
		{name: "guild", label: "Guild Manager", getter: func(k typedef.Keybinds) string { return k.GuildManager }, setter: func(k *typedef.Keybinds, v string) { k.GuildManager = v }},
		{name: "loadout", label: "Loadout Manager", getter: func(k typedef.Keybinds) string { return k.LoadoutManager }, setter: func(k *typedef.Keybinds, v string) { k.LoadoutManager = v }},
		{name: "script", label: "Script Manager", getter: func(k typedef.Keybinds) string { return k.ScriptManager }, setter: func(k *typedef.Keybinds, v string) { k.ScriptManager = v }},
		{name: "territory", label: "Toggle Territories", getter: func(k typedef.Keybinds) string { return k.TerritoryToggle }, setter: func(k *typedef.Keybinds, v string) { k.TerritoryToggle = v }},
		{name: "coords", label: "Toggle Coordinates", getter: func(k typedef.Keybinds) string { return k.Coordinates }, setter: func(k *typedef.Keybinds, v string) { k.Coordinates = v }},
		{name: "add_marker", label: "Add Marker", getter: func(k typedef.Keybinds) string { return k.AddMarker }, setter: func(k *typedef.Keybinds, v string) { k.AddMarker = v }},
		{name: "clear_markers", label: "Clear Markers", getter: func(k typedef.Keybinds) string { return k.ClearMarkers }, setter: func(k *typedef.Keybinds, v string) { k.ClearMarkers = v }},
		{name: "route_highlight", label: "Route Highlight", getter: func(k typedef.Keybinds) string { return k.RouteHighlight }, setter: func(k *typedef.Keybinds, v string) { k.RouteHighlight = v }},
	}

	pluginBinds := pluginhost.ListPluginKeybinds()

	checkDuplicates := func(b typedef.Keybinds, pluginBindings map[string]string) (string, string, bool) {
		seen := make(map[string]string)
		add := func(binding, label string) (string, string, bool) {
			if binding == "" {
				return "", "", false
			}
			if other, exists := seen[binding]; exists {
				return label, other, true
			}
			seen[binding] = label
			return "", "", false
		}
		for _, field := range keybindFields {
			if a, b, dup := add(field.getter(b), field.label); dup {
				return a, b, true
			}
		}
		for _, kb := range pluginBinds {
			key := kb.PluginID + "::" + kb.ID
			binding := pluginBindings[key]
			label := fmt.Sprintf("%s (%s)", kb.Label, kb.PluginID)
			if a, b, dup := add(binding, label); dup {
				return a, b, true
			}
		}
		return "", "", false
	}

	refreshInputs := func(bindings typedef.Keybinds, pluginBindings map[string]string) {
		for _, field := range keybindFields {
			if input, ok := smm.keybindInputs[field.name]; ok {
				input.SetValue(field.getter(bindings))
			}
		}
		for _, kb := range pluginBinds {
			fieldName := "plugin:" + kb.PluginID + "::" + kb.ID
			if input, ok := smm.keybindInputs[fieldName]; ok {
				key := kb.PluginID + "::" + kb.ID
				input.SetValue(pluginBindings[key])
			}
		}
	}

	keyInputOpts := DefaultTextInputOptions()
	keyInputOpts.Width = 120
	keyInputOpts.MaxLength = 12
	keyInputOpts.ValidateInput = func(newValue string) bool {
		if newValue == "" {
			return true // Allow clearing before typing replacement
		}
		if len(newValue) > 12 {
			return false
		}
		for _, r := range newValue {
			if r == ' ' {
				return false
			}
			if r >= 'A' && r <= 'Z' {
				continue
			}
			if r >= 'a' && r <= 'z' {
				continue
			}
			if r >= '0' && r <= '9' {
				continue
			}
			return false
		}
		return true
	}

	for _, field := range keybindFields {
		initial := field.getter(currentOpts.Keybinds)
		inputOpts := keyInputOpts
		var input *MenuTextInput
		input = NewMenuTextInput(field.label, initial, inputOpts, func(val string) {
			prevOpts := eruntime.GetRuntimeOptions()
			prevVal := field.getter(prevOpts.Keybinds)

			trimmed := strings.TrimSpace(val)

			normalized, ok := typedef.CanonicalizeBinding(trimmed)
			if !ok {
				return // Invalid so far; allow the user to continue typing
			}

			updated := prevOpts
			field.setter(&updated.Keybinds, normalized)

			if a, b, dup := checkDuplicates(updated.Keybinds, prevOpts.PluginKeybinds); dup {
				fmt.Printf("Keybind conflict between %s and %s\n", a, b)
				if input != nil {
					input.SetValue(prevVal)
					input.selectionStart = 0
					input.cursorPos = len(prevVal)
				}
				return
			}

			eruntime.SetRuntimeOptions(updated)
			refreshInputs(updated.Keybinds, prevOpts.PluginKeybinds)
		})

		keybindSection.AddElement(input)
		smm.keybindInputs[field.name] = input
	}

	// Plugin keybind inputs (only shown when plugins registered binds).
	if len(pluginBinds) > 0 {
		keybindSection.Text("Plugin keybinds", DefaultTextOptions())
		for _, kb := range pluginBinds {
			fieldName := "plugin:" + kb.PluginID + "::" + kb.ID
			label := fmt.Sprintf("%s (%s)", kb.Label, kb.PluginID)
			current := currentOpts.PluginKeybinds[kb.PluginID+"::"+kb.ID]
			inputOpts := keyInputOpts
			var input *MenuTextInput
			input = NewMenuTextInput(label, current, inputOpts, func(val string) {
				prevOpts := eruntime.GetRuntimeOptions()
				trimmed := strings.TrimSpace(val)

				normalized, ok := typedef.CanonicalizeBinding(trimmed)
				if !ok {
					return
				}

				updated := prevOpts
				if updated.PluginKeybinds == nil {
					updated.PluginKeybinds = make(map[string]string)
				}
				key := kb.PluginID + "::" + kb.ID
				prevVal := updated.PluginKeybinds[key]
				updated.PluginKeybinds[key] = normalized

				if a, b, dup := checkDuplicates(updated.Keybinds, updated.PluginKeybinds); dup {
					fmt.Printf("Keybind conflict between %s and %s\n", a, b)
					if input != nil {
						input.SetValue(prevVal)
						input.selectionStart = 0
						input.cursorPos = len(prevVal)
					}
					return
				}

				eruntime.SetRuntimeOptions(updated)
				refreshInputs(updated.Keybinds, updated.PluginKeybinds)
			})

			keybindSection.AddElement(input)
			smm.keybindInputs[fieldName] = input
		}
	}

	keybindSection.Spacer(DefaultSpacerOptions())
	resetKeybindsOpts := DefaultButtonOptions()
	resetKeybindsOpts.BackgroundColor = color.RGBA{120, 70, 70, 255}
	resetKeybindsOpts.HoverColor = color.RGBA{140, 90, 90, 255}
	resetKeybindsOpts.PressedColor = color.RGBA{100, 60, 60, 255}
	resetKeybindsOpts.BorderColor = color.RGBA{180, 110, 110, 255}
	keybindSection.Button("Reset Keybinds", resetKeybindsOpts, func() {
		opts := eruntime.GetRuntimeOptions()
		opts.Keybinds = typedef.DefaultKeybinds()
		if opts.PluginKeybinds == nil {
			opts.PluginKeybinds = make(map[string]string)
		}
		for _, kb := range pluginBinds {
			key := kb.PluginID + "::" + kb.ID
			opts.PluginKeybinds[key] = kb.DefaultBinding
		}
		eruntime.SetRuntimeOptions(opts)
		refreshInputs(opts.Keybinds, opts.PluginKeybinds)
	})

	// Map section
	mapSection := optionsSection.CollapsibleMenu("Map", DefaultCollapsibleMenuOptions())
	mapSection.Text("Visibility", DefaultTextOptions())
	mapSection.Text("0% = hidden, 100% = fully visible", DefaultTextOptions())

	mapOpacitySliderOpts := DefaultSliderOptions()
	mapOpacitySliderOpts.MinValue = 0
	mapOpacitySliderOpts.MaxValue = 100
	mapOpacitySliderOpts.Step = 1
	mapOpacitySliderOpts.ValueFormat = "%.0f%%"
	mapOpacitySliderOpts.ShowValue = true

	currentMapOpacity := currentOpts.MapOpacityPercent
	if currentMapOpacity <= 0 {
		currentMapOpacity = 100 // Fallback for older saves
	}

	smm.mapOpacity = NewMenuSlider("Map Opacity", currentMapOpacity, mapOpacitySliderOpts, func(val float64) {
		if val < 0 {
			val = 0
		}
		if val > 100 {
			val = 100
		}
		opts := eruntime.GetRuntimeOptions()
		opts.MapOpacityPercent = val
		eruntime.SetRuntimeOptions(opts)
	})
	mapSection.AddElement(smm.mapOpacity)

	mapSection.Spacer(DefaultSpacerOptions())
	mapSection.Text("Resource Colors", DefaultTextOptions())
	mapSection.Text("Click to customize resource palette", DefaultTextOptions())

	palette := currentOpts.ResourceColors
	addColorBtn := func(key, label string) {
		swatch := resourceColorForKey(palette, key)
		btnOpts := DefaultButtonOptions()
		btnOpts.Height = 32
		btn := NewMenuColorButton(label, swatch, btnOpts, func() {
			smm.openMapColorPicker(key)
		})
		smm.mapColorBtns[key] = btn
		mapSection.AddElement(btn)
	}

	addColorBtn("Ore", "Ore")
	addColorBtn("Wood", "Wood")
	addColorBtn("Crop", "Crop")
	addColorBtn("Fish", "Fish")
	addColorBtn("Multi", "Rainbow / Multi")
	addColorBtn("Emerald", "Emerald")

	// Toggle emerald generator highlighting just below the emerald palette.
	emeraldToggleOpts := DefaultToggleSwitchOptions()
	emeraldToggleOpts.Options = []string{"On", "Off"}
	showEmerald := currentOpts.ShowEmeraldGenerators
	emeraldIndex := 0
	if !showEmerald {
		emeraldIndex = 1
	}
	mapSection.ToggleSwitch("Show city colour", emeraldIndex, emeraldToggleOpts, func(index int, value string) {
		opts := eruntime.GetRuntimeOptions()
		opts.ShowEmeraldGenerators = index == 0
		eruntime.SetRuntimeOptions(opts)
	})

	mapSection.Spacer(DefaultSpacerOptions())
	resetBtnOpts := DefaultButtonOptions()
	resetBtnOpts.BackgroundColor = color.RGBA{180, 60, 60, 255}
	resetBtnOpts.HoverColor = color.RGBA{200, 80, 80, 255}
	resetBtnOpts.PressedColor = color.RGBA{160, 40, 40, 255}
	resetBtnOpts.BorderColor = color.RGBA{220, 100, 100, 255}
	mapSection.Button("Reset Map Colors", resetBtnOpts, func() {
		opts := eruntime.GetRuntimeOptions()
		opts.ResourceColors = typedef.DefaultResourceColors()
		opts.MapOpacityPercent = 100
		eruntime.SetRuntimeOptions(opts)
		if smm.mapOpacity != nil {
			smm.mapOpacity.SetValue(100)
		}
		// Refresh swatches
		for key, btn := range smm.mapColorBtns {
			btn.SetColor(resourceColorForKey(opts.ResourceColors, key))
		}
	})

	// Throughput curve slider: -5..5 (0 = linear, >0 brightens faster, <0 brightens slower)
	optionsSection.Spacer(DefaultSpacerOptions())
	optionsSection.Text("Throughput Curve", DefaultTextOptions())
	optionsSection.Text("Adjusts contrast scaling of throughput view", DefaultTextOptions())
	optionsSection.Text("Negative = inverse logarithmic scale", DefaultTextOptions())
	optionsSection.Text("0.0 = linear scale", DefaultTextOptions())
	optionsSection.Text("Positive = logarithmic scale", DefaultTextOptions())

	throughputSliderOpts := DefaultSliderOptions()
	throughputSliderOpts.MinValue = -5
	throughputSliderOpts.MaxValue = 5
	throughputSliderOpts.Step = 0.1
	throughputSliderOpts.ValueFormat = "%.1f"
	throughputSliderOpts.ShowValue = true

	currentThroughputCurve := currentOpts.ThroughputCurve

	throughputSlider := NewMenuSlider("Throughput Scale", currentThroughputCurve, throughputSliderOpts, func(val float64) {
		if val < throughputSliderOpts.MinValue {
			val = throughputSliderOpts.MinValue
		}
		if val > throughputSliderOpts.MaxValue {
			val = throughputSliderOpts.MaxValue
		}
		opts := eruntime.GetRuntimeOptions()
		opts.ThroughputCurve = val
		eruntime.SetRuntimeOptions(opts)
	})
	optionsSection.AddElement(throughputSlider)

	// Chokepoint analysis tuning
	optionsSection.Spacer(DefaultSpacerOptions())
	analysisOptions := DefaultCollapsibleMenuOptions()
	analysisOptions.Collapsed = false
	analysisSection := optionsSection.CollapsibleMenu("Analysis", analysisOptions)

	analysisSection.Text("Result scaling", DefaultTextOptions())
	chokeCurveOpts := DefaultSliderOptions()
	chokeCurveOpts.MinValue = -7.5
	chokeCurveOpts.MaxValue = 7.5
	chokeCurveOpts.Step = 0.1
	chokeCurveOpts.ValueFormat = "%.1f"
	chokeCurveOpts.ShowValue = true
	chokeCurveSlider := NewMenuSlider("Curve", currentOpts.ChokepointCurve, chokeCurveOpts, func(val float64) {
		if val < chokeCurveOpts.MinValue {
			val = chokeCurveOpts.MinValue
		}
		if val > chokeCurveOpts.MaxValue {
			val = chokeCurveOpts.MaxValue
		}
		opts := eruntime.GetRuntimeOptions()
		opts.ChokepointCurve = val
		eruntime.SetRuntimeOptions(opts)
	})
	analysisSection.AddElement(chokeCurveSlider)

	analysisSection.Spacer(DefaultSpacerOptions())
	analysisSection.Text("Emerald vs resource weight (log scale)", DefaultTextOptions())
	analysisSection.Text("0.10x..10.0x; default = 1.0x", DefaultTextOptions())
	currentWeight := currentOpts.ChokepointEmeraldWeight
	weightSliderValue := chokepointWeightToSlider(currentWeight)
	currentWeight = sliderToChokepointWeight(weightSliderValue) // clamp to range
	weightDisplay := NewMenuText(fmt.Sprintf("Current weight: %.2fx", currentWeight), DefaultTextOptions())
	analysisSection.AddElement(weightDisplay)

	weightOpts := DefaultSliderOptions()
	weightOpts.MinValue = 0
	weightOpts.MaxValue = 100
	weightOpts.Step = 1
	weightOpts.ValueFormat = ""
	weightOpts.ShowValue = false // we show the mapped weight manually
	weightSlider := NewMenuSlider("Emerald Weight", weightSliderValue, weightOpts, func(val float64) {
		weight := sliderToChokepointWeight(val)
		opts := eruntime.GetRuntimeOptions()
		opts.ChokepointEmeraldWeight = weight
		eruntime.SetRuntimeOptions(opts)
		if weightDisplay != nil {
			weightDisplay.SetText(fmt.Sprintf("Current weight: %.2fx", weight))
		}
	})
	analysisSection.AddElement(weightSlider)

	analysisSection.Spacer(DefaultSpacerOptions())
	analysisSection.Text("Scoring mode", DefaultTextOptions())
	modeOpts := DefaultToggleSwitchOptions()
	modeOpts.Options = []string{"Cardinal", "Ordinal"}
	currentMode := eruntime.GetRuntimeOptions().ChokepointMode
	modeIndex := 0
	if currentMode == "Ordinal" {
		modeIndex = 1
	}
	analysisSection.ToggleSwitch("Mode", modeIndex, modeOpts, func(index int, value string) {
		opts := eruntime.GetRuntimeOptions()
		if index == 1 {
			opts.ChokepointMode = "Ordinal"
		} else {
			opts.ChokepointMode = "Cardinal"
		}
		eruntime.SetRuntimeOptions(opts)
	})

	analysisSection.Spacer(DefaultSpacerOptions())
	analysisSection.Text("Include downstream production", DefaultTextOptions())
	downOpts := DefaultToggleSwitchOptions()
	downOpts.Options = []string{"On", "Off"}
	includeDownstream := eruntime.GetRuntimeOptions().ChokepointIncludeDownstream
	downIndex := 0
	if !includeDownstream {
		downIndex = 1
	}
	analysisSection.ToggleSwitch("Include Downstream", downIndex, downOpts, func(index int, value string) {
		opts := eruntime.GetRuntimeOptions()
		opts.ChokepointIncludeDownstream = index == 0
		eruntime.SetRuntimeOptions(opts)
	})

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
		// Trigger save file dialogue
		if fileManager := GetFileSystemManager(); fileManager != nil {
			// fmt.Println("[STATE_MGMT] File system manager found, showing save dialogue")
			fileManager.ShowSaveDialogue()
		} else {
			// fmt.Println("[STATE_MGMT] File system manager not available")
		}
	})
	loadSaveSection.Button("Load Session", saveLoadButtonOpts, func() {
		// Trigger load file dialogue
		if fileManager := GetFileSystemManager(); fileManager != nil {
			fileManager.ShowOpenDialogue()
		} else {
			// fmt.Println("[STATE_MGMT] File system manager not available")
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

	// "More!" section
	// moreSection := smm.menu.CollapsibleMenu("More!", DefaultCollapsibleMenuOptions())
	// moreSection.Text("If you're a developer and", DefaultTextOptions())
	// moreSection.Text("interested in writing extensions", DefaultTextOptions())
	// moreSection.Text("for the economy simulator,", DefaultTextOptions())
	// moreSection.Text("you can get the SDK and docs here!", DefaultTextOptions())
	// moreSection.Spacer(DefaultSpacerOptions())
	// moreSection.Button("Get SDK", DefaultButtonOptions(), func() {
	// 	// show save dialogue but with sdk file, in embed
	// 	fsd := NewFileSystemDialogue(FileDialogueSave, "SDK", []string{ ".h" })
	// 	fsd.
	// })

	smm.menu.Show()
}

// HandleInput processes high-priority interactions like the map color picker.
func (smm *StateManagementMenu) HandleInput() bool {
	if smm.mapColorPicker != nil && smm.mapColorPicker.IsVisible() {
		// Always treat the picker as owning input while visible
		smm.mapColorPicker.Update()
		return true
	}
	return false
}

// DrawExtras renders overlays owned by the state menu.
func (smm *StateManagementMenu) DrawExtras(screen *ebiten.Image) {
	if smm.mapColorPicker != nil && smm.mapColorPicker.IsVisible() {
		smm.mapColorPicker.Draw(screen)
	}
}

// Update method - only update stats text, not entire menu
func (smm *StateManagementMenu) Update(deltaTime float64) {
	// Update color picker animation/dismissal when open
	if smm.mapColorPicker != nil && smm.mapColorPicker.IsVisible() {
		smm.mapColorPicker.Update()
	}

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
