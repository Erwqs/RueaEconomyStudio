package app

import (
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"

	"RueaES/javascript"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ScriptManager handles the script management UI and functionality
type ScriptManager struct {
	visible         bool
	modal           *EnhancedModal
	scripts         []string
	scrollOffset    int
	hoveredIndex    int
	selectedIndex   int
	executeButton   *EnhancedButton
	openButton      *EnhancedButton
	runningScripts  map[string]chan struct{}             // Map of script name to cancel channel
	scriptRuntimes  map[string]*javascript.ScriptRuntime // Map of script name to runtime info
	erroredScripts  map[string]bool                      // Map of script name to error state
	framesAfterOpen int                                  // Frames since the manager was opened (to prevent immediate ESC closing)
}

// NewScriptManager creates a new script manager
func NewScriptManager() *ScriptManager {
	screenW, screenH := WebSafeWindowSize()
	modalWidth := 600
	modalHeight := 500
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	modal := NewEnhancedModal("Scripts", modalWidth, modalHeight)

	// Create buttons
	buttonWidth := 120
	buttonHeight := 40
	buttonY := modalY + modalHeight - 60

	executeButton := NewEnhancedButton("Execute", modalX+20, buttonY, buttonWidth, buttonHeight, nil)
	openButton := NewEnhancedButton("Open Externally", modalX+modalWidth-140, buttonY, buttonWidth, buttonHeight, nil)

	sm := &ScriptManager{
		visible:        false,
		modal:          modal,
		scripts:        []string{},
		scrollOffset:   0,
		hoveredIndex:   -1,
		selectedIndex:  -1,
		executeButton:  executeButton,
		openButton:     openButton,
		runningScripts: make(map[string]chan struct{}),
		scriptRuntimes: make(map[string]*javascript.ScriptRuntime),
		erroredScripts: make(map[string]bool),
	}

	// Load scripts from directory
	sm.loadScripts()

	// Set button callbacks
	executeButton.OnClick = func() {
		sm.executeSelectedScript()
	}

	openButton.OnClick = func() {
		sm.openSelectedScriptExternally()
	}

	return sm
}

// Show makes the script manager visible
func (sm *ScriptManager) Show() {
	sm.visible = true
	sm.framesAfterOpen = 0 // Reset frame counter when opening
	sm.loadScripts()       // Refresh script list when showing
	sm.modal.Show()
}

// Hide makes the script manager invisible
func (sm *ScriptManager) Hide() {
	sm.visible = false
	sm.modal.Hide()
	// Note: We don't stop running scripts when hiding the manager
	// Scripts should continue running in the background
	// Users can reopen the manager to terminate specific scripts if needed
}

// IsVisible returns true if the script manager is visible
func (sm *ScriptManager) IsVisible() bool {
	return sm.visible
}

// Update handles input and updates the script manager state
func (sm *ScriptManager) Update() {
	if !sm.visible {
		return
	}

	// Update modal and button positions based on current window size
	sm.updateLayout()

	// Increment frame counter
	sm.framesAfterOpen++

	// Get mouse position
	mx, my := ebiten.CursorPosition()

	// Check if clicked outside the modal to close it
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !sm.modal.Contains(mx, my) {
			sm.Hide()
			return
		}
	}

	// Close on ESC key, but only after a few frames to prevent immediate closing
	// from queued ESC key presses
	if sm.framesAfterOpen > 5 && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		sm.Hide()
		return
	}

	// Handle mouse back button (MouseButton3) - same as ESC
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
		sm.Hide()
		return
	}

	// Update modal animation
	sm.modal.Update()

	// Update buttons
	sm.executeButton.Update(mx, my)
	sm.openButton.Update(mx, my)

	// Reset hovered index
	sm.hoveredIndex = -1

	// Check for hovering/clicking on script items
	if len(sm.scripts) > 0 {
		itemHeight := 30
		listStartY := sm.modal.Y + 50

		// Calculate which items are visible based on scroll offset
		maxVisibleItems := (sm.modal.Height - 120) / itemHeight

		for i := 0; i < len(sm.scripts) && i < maxVisibleItems; i++ {
			itemIndex := i + sm.scrollOffset
			if itemIndex >= len(sm.scripts) {
				break
			}

			itemY := listStartY + (i * itemHeight)

			// Script item area
			itemRect := Rect{
				X:      sm.modal.X + 20,
				Y:      itemY,
				Width:  sm.modal.Width - 40,
				Height: itemHeight,
			}

			// Check if hovering over item
			if mx >= itemRect.X && mx < itemRect.X+itemRect.Width &&
				my >= itemRect.Y && my < itemRect.Y+itemRect.Height {
				sm.hoveredIndex = itemIndex

				// Handle click on item
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					sm.selectedIndex = itemIndex
					sm.updateExecuteButtonText() // Update button when selection changes
				}
			}
		}

		// Handle scroll wheel in script list area
		_, wy := ebiten.Wheel()
		if wy != 0 {
			// Check if mouse is in the script list area
			listRect := Rect{
				X:      sm.modal.X + 20,
				Y:      listStartY,
				Width:  sm.modal.Width - 40,
				Height: sm.modal.Height - 120,
			}

			if mx >= listRect.X && mx < listRect.X+listRect.Width &&
				my >= listRect.Y && my < listRect.Y+listRect.Height {

				sm.scrollOffset -= int(wy * 3)
				maxScroll := len(sm.scripts) - maxVisibleItems
				if maxScroll < 0 {
					maxScroll = 0
				}

				if sm.scrollOffset < 0 {
					sm.scrollOffset = 0
				} else if sm.scrollOffset > maxScroll {
					sm.scrollOffset = maxScroll
				}
			}
		}
	}

	// Enable/disable buttons based on selection
	sm.executeButton.enabled = sm.selectedIndex >= 0 && sm.selectedIndex < len(sm.scripts)
	sm.openButton.enabled = sm.selectedIndex >= 0 && sm.selectedIndex < len(sm.scripts)

	// Update execute button text based on running state
	sm.updateExecuteButtonText()
}

// updateExecuteButtonText updates the execute button text based on running state
func (sm *ScriptManager) updateExecuteButtonText() {
	if sm.selectedIndex >= 0 && sm.selectedIndex < len(sm.scripts) {
		selectedScript := sm.scripts[sm.selectedIndex]
		if _, isRunning := sm.runningScripts[selectedScript]; isRunning {
			sm.executeButton.Text = "Terminate"
			sm.executeButton.SetRedButtonStyle()
		} else {
			sm.executeButton.Text = "Execute"
			sm.executeButton.ClearCustomStyling()
		}
	} else {
		sm.executeButton.Text = "Execute"
		sm.executeButton.ClearCustomStyling()
	}
}

// loadScripts loads the list of JavaScript files from the scripts directory
func (sm *ScriptManager) loadScripts() {
	// Create scripts directory if it doesn't exist
	if _, err := os.Stat("scripts"); os.IsNotExist(err) {
		os.Mkdir("scripts", 0755)
	}

	scripts, err := javascript.GetScripts()
	if err != nil {
		// fmt.Printf("[SCRIPT_MANAGER] Failed to load scripts: %v\n", err)
		sm.scripts = []string{}
		return
	}

	sm.scripts = scripts

	// Reset selection if it's out of bounds
	if sm.selectedIndex >= len(sm.scripts) {
		sm.selectedIndex = -1
	}
}

// executeSelectedScript executes the selected script
func (sm *ScriptManager) executeSelectedScript() {
	if sm.selectedIndex < 0 || sm.selectedIndex >= len(sm.scripts) {
		return
	}

	scriptName := sm.scripts[sm.selectedIndex]

	// If already running, terminate the script
	if runtime, isRunning := sm.scriptRuntimes[scriptName]; isRunning {
		if runtime != nil && runtime.Cancel != nil {
			// Send termination signal (don't close the channel, just send signal)
			select {
			case runtime.Cancel <- struct{}{}:
				// fmt.Printf("[SCRIPT_MANAGER] Sent termination signal to script %s\n", scriptName)
			default:
				// fmt.Printf("[SCRIPT_MANAGER] Script %s already terminating\n", scriptName)
			}
		}
		return
	}

	scriptPath := filepath.Join("scripts", scriptName)

	// Read script content
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		// fmt.Printf("[SCRIPT_MANAGER] Failed to read script %s: %v\n", scriptName, err)
		return
	}

	// fmt.Printf("[SCRIPT_MANAGER] Starting script: %s (with init() and tick() support)\n", scriptName)

	// Clear any previous error state
	delete(sm.erroredScripts, scriptName)

	// Use Run() instead of Execute() to support init() and tick() functions
	runtime := javascript.Run(string(content), scriptName)
	sm.runningScripts[scriptName] = runtime.Cancel
	sm.scriptRuntimes[scriptName] = runtime
	// fmt.Printf("[SCRIPT_MANAGER] Added script %s to running scripts map. Total running: %d\n", scriptName, len(sm.runningScripts))
	sm.updateExecuteButtonText()

	// Monitor the script in a separate goroutine
	go func() {
		defer func() {
			delete(sm.runningScripts, scriptName)
			delete(sm.scriptRuntimes, scriptName)
			sm.updateExecuteButtonText()
			// fmt.Printf("[SCRIPT_MANAGER] Script %s stopped\n", scriptName)
		}()

		// Wait for the script to complete or error
		select {
		case <-runtime.Done:
			// Script completed normally or was terminated
			// fmt.Printf("[SCRIPT_MANAGER] Script %s completed\n", scriptName)
		case <-runtime.Error:
			// Script errored
			// fmt.Printf("[SCRIPT_MANAGER] Script %s errored: %v\n", scriptName, err)
			sm.erroredScripts[scriptName] = true
			sm.updateExecuteButtonText()
		}
	}()
}

// openSelectedScriptExternally opens the selected script in the default external editor
func (sm *ScriptManager) openSelectedScriptExternally() {
	if sm.selectedIndex < 0 || sm.selectedIndex >= len(sm.scripts) {
		return
	}

	scriptName := sm.scripts[sm.selectedIndex]
	scriptPath := filepath.Join("scripts", scriptName)

	// Try to open with default system editor
	var cmd *exec.Cmd
	switch {
	case fileExists("/usr/bin/xdg-open"): // Linux
		cmd = exec.Command("xdg-open", scriptPath)
	case fileExists("/usr/bin/open"): // macOS
		cmd = exec.Command("open", scriptPath)
	default: // Windows or fallback
		cmd = exec.Command("cmd", "/c", "start", scriptPath)
	}

	err := cmd.Start()
	if err != nil {
		// fmt.Printf("[SCRIPT_MANAGER] Failed to open script externally: %v\n", err)
	} else {
		// fmt.Printf("[SCRIPT_MANAGER] Opened script %s externally\n", scriptName)
	}
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// Draw renders the script manager
func (sm *ScriptManager) Draw(screen *ebiten.Image) {
	if !sm.visible {
		return
	}

	// Draw modal background
	sm.modal.Draw(screen)

	// Draw script list
	sm.drawScriptList(screen)

	// Draw buttons
	sm.executeButton.Draw(screen)
	sm.openButton.Draw(screen)
}

// drawScriptList draws the list of scripts
func (sm *ScriptManager) drawScriptList(screen *ebiten.Image) {
	if len(sm.scripts) == 0 {
		// Draw "No scripts found" message
		noScriptsText := "No .js files found in scripts directory"
		font := loadWynncraftFont(14)
		bounds := text.BoundString(font, noScriptsText)
		textX := sm.modal.X + (sm.modal.Width-bounds.Dx())/2
		textY := sm.modal.Y + 150
		text.Draw(screen, noScriptsText, font, textX, textY, EnhancedUIColors.TextSecondary)
		return
	}

	itemHeight := 30
	listStartY := sm.modal.Y + 50
	maxVisibleItems := (sm.modal.Height - 120) / itemHeight

	// Draw list background
	listRect := image.Rect(
		sm.modal.X+20,
		listStartY,
		sm.modal.X+sm.modal.Width-20,
		listStartY+(maxVisibleItems*itemHeight),
	)
	vector.DrawFilledRect(screen, float32(listRect.Min.X), float32(listRect.Min.Y),
		float32(listRect.Dx()), float32(listRect.Dy()), EnhancedUIColors.Surface, false)

	// Draw border around list
	vector.StrokeRect(screen, float32(listRect.Min.X), float32(listRect.Min.Y),
		float32(listRect.Dx()), float32(listRect.Dy()), 1, EnhancedUIColors.Border, false)

	// Draw scripts
	font := loadWynncraftFont(14)
	for i := 0; i < maxVisibleItems && i+sm.scrollOffset < len(sm.scripts); i++ {
		scriptIndex := i + sm.scrollOffset
		scriptName := sm.scripts[scriptIndex]

		itemY := listStartY + (i * itemHeight)
		itemRect := image.Rect(
			sm.modal.X+20,
			itemY,
			sm.modal.X+sm.modal.Width-20,
			itemY+itemHeight,
		)

		// Determine item color
		var bgColor color.RGBA
		if scriptIndex == sm.selectedIndex {
			bgColor = EnhancedUIColors.ItemSelected
		} else if scriptIndex == sm.hoveredIndex {
			bgColor = EnhancedUIColors.ItemHover
		} else if _, hasError := sm.erroredScripts[scriptName]; hasError {
			// Give errored scripts a subtle red background
			bgColor = color.RGBA{60, 40, 40, 255}
		} else if _, isRunning := sm.runningScripts[scriptName]; isRunning {
			// Give running scripts a subtle green background
			bgColor = color.RGBA{40, 60, 40, 255}
		} else {
			bgColor = EnhancedUIColors.ItemBackground
		}

		// Draw item background
		vector.DrawFilledRect(screen, float32(itemRect.Min.X), float32(itemRect.Min.Y),
			float32(itemRect.Dx()), float32(itemRect.Dy()), bgColor, false)

		// Determine text color - green for running scripts, red for errored scripts
		var textColor color.RGBA
		if _, hasError := sm.erroredScripts[scriptName]; hasError {
			textColor = color.RGBA{255, 100, 100, 255} // Red for errored scripts
		} else if _, isRunning := sm.runningScripts[scriptName]; isRunning {
			textColor = color.RGBA{100, 255, 100, 255} // Brighter green for running scripts
		} else {
			textColor = EnhancedUIColors.Text
		}

		// Draw item text
		textY := itemY + itemHeight/2 + 5 // Center vertically
		text.Draw(screen, scriptName, font, itemRect.Min.X+10, textY, textColor)

		// Draw status indicator circles
		if _, hasError := sm.erroredScripts[scriptName]; hasError {
			// Draw red circle for errored scripts
			indicatorX := float32(itemRect.Max.X - 15)
			indicatorY := float32(itemY + itemHeight/2)
			vector.DrawFilledCircle(screen, indicatorX, indicatorY, 5, color.RGBA{255, 100, 100, 255}, false)
		} else if _, isRunning := sm.runningScripts[scriptName]; isRunning {
			// Draw green circle for running scripts
			indicatorX := float32(itemRect.Max.X - 15)
			indicatorY := float32(itemY + itemHeight/2)
			vector.DrawFilledCircle(screen, indicatorX, indicatorY, 5, color.RGBA{100, 255, 100, 255}, false)
		}

		// Draw separator line
		if i < maxVisibleItems-1 && scriptIndex < len(sm.scripts)-1 {
			vector.StrokeLine(screen, float32(itemRect.Min.X), float32(itemRect.Max.Y),
				float32(itemRect.Max.X), float32(itemRect.Max.Y), 1, EnhancedUIColors.Border, false)
		}
	}

	// Draw scroll indicator if needed
	if len(sm.scripts) > maxVisibleItems {
		sm.drawScrollIndicator(screen, listRect, maxVisibleItems)
	}
}

// drawScrollIndicator draws a scroll indicator on the right side of the list
func (sm *ScriptManager) drawScrollIndicator(screen *ebiten.Image, listRect image.Rectangle, maxVisibleItems int) {
	scrollbarWidth := 6
	scrollbarX := listRect.Max.X - scrollbarWidth - 2
	scrollbarY := listRect.Min.Y + 2
	scrollbarHeight := listRect.Dy() - 4

	// Draw scrollbar track
	vector.DrawFilledRect(screen, float32(scrollbarX), float32(scrollbarY),
		float32(scrollbarWidth), float32(scrollbarHeight), EnhancedUIColors.Border, false)

	// Calculate thumb position and size
	totalItems := len(sm.scripts)
	thumbHeight := int(float64(scrollbarHeight) * float64(maxVisibleItems) / float64(totalItems))
	if thumbHeight < 20 {
		thumbHeight = 20 // Minimum thumb size
	}

	thumbPosition := int(float64(scrollbarHeight-thumbHeight) * float64(sm.scrollOffset) / float64(totalItems-maxVisibleItems))

	// Draw scrollbar thumb
	vector.DrawFilledRect(screen, float32(scrollbarX), float32(scrollbarY+thumbPosition),
		float32(scrollbarWidth), float32(thumbHeight), EnhancedUIColors.Text, false)
}

// updateLayout updates the positions of UI elements based on current window size
func (sm *ScriptManager) updateLayout() {
	screenW, screenH := WebSafeWindowSize()
	modalWidth := 600
	modalHeight := 500
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	// Update modal position
	sm.modal.X = modalX
	sm.modal.Y = modalY
	sm.modal.Width = modalWidth
	sm.modal.Height = modalHeight

	// Update button positions
	buttonWidth := 120
	buttonHeight := 40
	buttonY := modalY + modalHeight - 60

	sm.executeButton.X = modalX + 20
	sm.executeButton.Y = buttonY
	sm.executeButton.Width = buttonWidth
	sm.executeButton.Height = buttonHeight

	sm.openButton.X = modalX + modalWidth - 140
	sm.openButton.Y = buttonY
	sm.openButton.Width = buttonWidth
	sm.openButton.Height = buttonHeight
}
