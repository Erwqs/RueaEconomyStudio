package app

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// WelcomeScreen represents the welcome screen modal that appears when no autosave is found
type WelcomeScreen struct {
	modal          *EnhancedModal
	visible        bool
	scrollOffset   int
	contentHeight  int
	maxScroll      int
	closeButton    *EnhancedButton
	getStartedBtn  *EnhancedButton
	font           font.Face
	titleFont      font.Face
	smallFont      font.Face
	onImportGuilds func() // Callback for importing guilds
	// Scroll bar dragging state
	isDraggingScrollBar   bool
	dragStartY            int
	dragStartScrollOffset int
}

// Global welcome screen instance
var globalWelcomeScreen *WelcomeScreen

// NewWelcomeScreen creates a new welcome screen
func NewWelcomeScreen() *WelcomeScreen {
	ws := &WelcomeScreen{
		modal:        NewEnhancedModal("Welcome to Ruea Economy Studio!", 900, 700),
		visible:      false,
		scrollOffset: 0,
		font:         loadWynncraftFont(14),
		titleFont:    loadWynncraftFont(20),
		smallFont:    loadWynncraftFont(12),
	}

	// Create close button with red color
	ws.closeButton = NewEnhancedButton("Close", 0, 0, 80, 30, func() {
		ws.Hide()
	})
	// Set red color for close button
	ws.closeButton.SetBackgroundColor(color.RGBA{200, 50, 50, 255}) // Red background
	ws.closeButton.SetHoverColor(color.RGBA{255, 80, 80, 255})      // Lighter red on hover

	// Create get started button
	ws.getStartedBtn = NewEnhancedButton("Guild Manager", 0, 0, 200, 30, func() {
		// Use callback to open guild manager with import mode
		if ws.onImportGuilds != nil {
			ws.onImportGuilds()
		}
		ws.Hide()
	})

	return ws
}

// GetWelcomeScreen returns the global welcome screen instance
func GetWelcomeScreen() *WelcomeScreen {
	if globalWelcomeScreen == nil {
		globalWelcomeScreen = NewWelcomeScreen()
	}
	return globalWelcomeScreen
}

// Show displays the welcome screen
func (ws *WelcomeScreen) Show() {
	ws.visible = true
	ws.modal.Show()
	ws.scrollOffset = 0
	fmt.Println("[WELCOME] Welcome screen shown")
}

// Hide hides the welcome screen
func (ws *WelcomeScreen) Hide() {
	ws.visible = false
	ws.modal.Hide()
	fmt.Println("[WELCOME] Welcome screen hidden")
}

// IsVisible returns whether the welcome screen is visible
func (ws *WelcomeScreen) IsVisible() bool {
	return ws.visible && ws.modal.IsVisible()
}

// SetImportGuildsCallback sets the callback function for importing guilds
func (ws *WelcomeScreen) SetImportGuildsCallback(callback func()) {
	ws.onImportGuilds = callback
}

// Update handles input for the welcome screen
func (ws *WelcomeScreen) Update() bool {
	if !ws.IsVisible() {
		return false
	}

	ws.modal.Update()

	// Handle scroll wheel for content scrolling
	_, scrollY := ebiten.Wheel()
	if scrollY != 0 {
		oldScrollOffset := ws.scrollOffset
		ws.scrollOffset -= int(scrollY * 30) // Scroll sensitivity
		if ws.scrollOffset < 0 {
			ws.scrollOffset = 0
		}

		// Recalculate maxScroll before applying limit
		bounds := ws.modal.GetBounds()
		contentHeight := bounds.Dy() - 110 // Same calculation as in Draw
		ws.calculateMaxScroll(bounds.Min.X+20, bounds.Min.Y+50, bounds.Dx()-40, contentHeight)

		if ws.scrollOffset > ws.maxScroll {
			ws.scrollOffset = ws.maxScroll
		}

		// Debug output
		if ws.scrollOffset != oldScrollOffset {
			fmt.Printf("[WELCOME] Scroll: %d/%d (delta: %.1f)\n", ws.scrollOffset, ws.maxScroll, scrollY)
		}
	}

	// Handle ESC key to close
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ws.Hide()
		return true
	}

	// Update button positions
	bounds := ws.modal.GetBounds()
	buttonAreaY := bounds.Min.Y + bounds.Dy() - 60

	ws.closeButton.X = bounds.Min.X + bounds.Dx() - 90
	ws.closeButton.Y = buttonAreaY

	ws.getStartedBtn.X = bounds.Min.X + 20
	ws.getStartedBtn.Y = buttonAreaY

	// Handle button updates and scroll bar dragging
	mx, my := ebiten.CursorPosition()

	// Handle scroll bar dragging
	if ws.maxScroll > 0 {
		contentY := bounds.Min.Y + 50
		contentHeight := bounds.Dy() - 110
		scrollBarX := bounds.Min.X + bounds.Dx() - 15
		scrollBarY := contentY
		scrollBarHeight := contentHeight

		// Calculate thumb position and size
		totalContentHeight := contentHeight + ws.maxScroll
		thumbHeight := maxInt(20, (contentHeight*scrollBarHeight)/totalContentHeight)
		thumbY := scrollBarY + (ws.scrollOffset*(scrollBarHeight-thumbHeight))/maxInt(1, ws.maxScroll)

		// Check if mouse is over scroll bar area
		mouseOverScrollBar := mx >= scrollBarX && mx <= scrollBarX+10 && my >= scrollBarY && my <= scrollBarY+scrollBarHeight
		mouseOverThumb := mx >= scrollBarX && mx <= scrollBarX+10 && my >= thumbY && my <= thumbY+thumbHeight

		// Handle mouse press on scroll bar
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mouseOverThumb {
				// Start dragging the thumb
				ws.isDraggingScrollBar = true
				ws.dragStartY = my
				ws.dragStartScrollOffset = ws.scrollOffset
			} else if mouseOverScrollBar {
				// Click on scroll bar but not on thumb - jump to that position
				clickRatio := float64(my-scrollBarY) / float64(scrollBarHeight)
				ws.scrollOffset = int(clickRatio * float64(ws.maxScroll))
				if ws.scrollOffset < 0 {
					ws.scrollOffset = 0
				}
				if ws.scrollOffset > ws.maxScroll {
					ws.scrollOffset = ws.maxScroll
				}
			}
		}

		// Handle dragging
		if ws.isDraggingScrollBar && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			deltaY := my - ws.dragStartY
			// Convert pixel movement to scroll offset
			scrollDelta := (deltaY * ws.maxScroll) / maxInt(1, scrollBarHeight-thumbHeight)
			ws.scrollOffset = ws.dragStartScrollOffset + scrollDelta

			if ws.scrollOffset < 0 {
				ws.scrollOffset = 0
			}
			if ws.scrollOffset > ws.maxScroll {
				ws.scrollOffset = ws.maxScroll
			}
		}

		// Stop dragging when mouse is released
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			ws.isDraggingScrollBar = false
		}
	}

	handled := ws.closeButton.Update(mx, my) || ws.getStartedBtn.Update(mx, my)

	return handled
}

// Draw renders the welcome screen
func (ws *WelcomeScreen) Draw(screen *ebiten.Image) {
	if !ws.IsVisible() {
		return
	}

	// Draw the modal background
	ws.modal.Draw(screen)

	// Get content area with better padding
	bounds := ws.modal.GetBounds()
	contentPadding := 30 // Increased padding
	contentX := bounds.Min.X + contentPadding
	contentY := bounds.Min.Y + 60                           // More top padding for title
	contentWidth := bounds.Dx() - (contentPadding * 2) - 20 // Extra space for scroll bar
	contentHeight := bounds.Dy() - 120                      // More space for buttons

	// Draw content with scrolling
	ws.drawContent(screen, contentX, contentY, contentWidth, contentHeight)

	// Draw buttons
	ws.closeButton.Draw(screen)
	ws.getStartedBtn.Draw(screen)

	// Draw scroll indicator if content is scrollable
	if ws.maxScroll > 0 {
		scrollBarX := bounds.Min.X + bounds.Dx() - 15
		scrollBarY := contentY
		scrollBarHeight := contentHeight

		// Background
		vector.DrawFilledRect(screen, float32(scrollBarX), float32(scrollBarY), 10, float32(scrollBarHeight),
			color.RGBA{60, 60, 60, 255}, false)

		// Thumb - calculate proportional size and position
		totalContentHeight := contentHeight + ws.maxScroll
		thumbHeight := maxInt(20, (contentHeight*scrollBarHeight)/totalContentHeight)
		thumbY := scrollBarY + (ws.scrollOffset*(scrollBarHeight-thumbHeight))/maxInt(1, ws.maxScroll)

		vector.DrawFilledRect(screen, float32(scrollBarX+1), float32(thumbY), 8, float32(thumbHeight),
			color.RGBA{120, 120, 120, 255}, false)
	}
}

// drawContent renders the welcome screen content
func (ws *WelcomeScreen) drawContent(screen *ebiten.Image, x, y, width, height int) {
	currentY := y - ws.scrollOffset
	lineHeight := 22     // Increased line height for better readability
	sectionSpacing := 35 // Increased spacing between sections
	itemPadding := 5     // Padding between items within sections

	// Title
	titleText := "Ruea Economy Studio - Wynncraft Guild Economy Simulator"
	titleBounds := text.BoundString(ws.titleFont, titleText)
	titleX := x + (width-titleBounds.Dx())/2
	if currentY > y-lineHeight && currentY < y+height {
		text.Draw(screen, titleText, ws.titleFont, titleX, currentY+20, color.RGBA{100, 149, 237, 255})
	}
	currentY += 40

	// Subtitle
	subtitleText := "As far as I know, this is the most comprehensive one out there"
	subtitleBounds := text.BoundString(ws.font, subtitleText)
	subtitleX := x + (width-subtitleBounds.Dx())/2
	if currentY > y-lineHeight && currentY < y+height {
		text.Draw(screen, subtitleText, ws.font, subtitleX, currentY+15, color.RGBA{200, 200, 200, 255})
	}
	currentY += sectionSpacing + itemPadding

	// Description
	descText := "It aims to closely replicate the economy system in game as much as possible"
	descBounds := text.BoundString(ws.font, descText)
	descX := x + (width-descBounds.Dx())/2
	if currentY > y-lineHeight && currentY < y+height {
		text.Draw(screen, descText, ws.font, descX, currentY+15, color.RGBA{180, 180, 180, 255})
	}
	currentY += sectionSpacing + itemPadding

	// First Time Setup section
	currentY = ws.drawSection(screen, "First Time Setup",
		[]string{
			"To get things started quick, you want to import all guilds the first time you run the app",
			"Press G to bring up guild management",
			"Then press API import to import all guilds and the map",
		}, x, currentY, width, y, height, color.RGBA{255, 215, 0, 255})

	// Features section
	currentY = ws.drawSection(screen, "Features",
		[]string{
			"Multiple guild supports",
			"Resource/storage based on how much time has elapsed",
			"Route calculation, affected by tax, border closure and trading style",
			"Ability to time travel into the future with manual tick advancement or stop the clock with state halting",
			"Save and load state/session to file for sharing/later use",
			"Import current map data directly from the API (this only imports guild claims, not their upgrades)",
			"Ability to enable/disable treasury calculation",
			"Treasury overrides, allow you to explicitly set a territory to treasury level of your choice",
			"Manually editable resource storage at runtime, so your experiment isn't limited by available resource",
			"Loadouts, apply mode (just like in game) and merge mode",
			"Tribute system with an option to spawn in or void resource through \"Nil Sink/Source\" guild instead of from other guild on map",
			"In-Transit resource inspector, see where all the resources in transit system go!",
			"Scriptable, write your own JavaScript code to be run within the simulation context",
		}, x, currentY, width, y, height, color.RGBA{144, 238, 144, 255})

	// Keybinds section
	currentY = ws.drawSection(screen, "Keybinds",
		[]string{
			"G - Guild management: you can add/remove guilds or edit guild's claim",
			"P - State management: this menu lets you control the tick rate, modify the logic/calculation/behavior or save and load state session to and from file",
			"L - Loadout menu: create loadout to apply to many territories, there are two application modes: apply which overrides the previous territory setting and apply the loadout's one and merge which merge non-zero data from loadout to territory",
			"B - Tribute configuration: set up your tribute here. you can set up a tribute between 2 guilds on the map or spawn in tribute from nothing to the hq (source) or simulate sending out tribute to non-existent guild on the map (sink)",
			"I - Resource inspector: unfinished and abandoned in transit resource inspector and editor",
			"S - Script: run your diy javascript code that will be interacted with the economy simulator here",
			"Tab - Territory view switcher: switch between guild view, resource view, defence and more!",
		}, x, currentY, width, y, height, color.RGBA{173, 216, 230, 255})

	// Usage Tips section
	currentY = ws.drawSection(screen, "Usage Tips",
		[]string{
			"Double click on a territory to open territory menu",
			"Note: if everything shows up as 0 or seems like nothing is working, press P and click Resume (if it says resume) to un-halt the state",
		}, x, currentY, width, y, height, color.RGBA{255, 182, 193, 255})

	// Calculate max scroll based on content height
	ws.calculateMaxScroll(x, y, width, height)
}

// calculateMaxScroll determines how much the content can be scrolled
func (ws *WelcomeScreen) calculateMaxScroll(x, y, width, height int) {
	// Calculate total content height by simulating the drawing process
	currentY := y
	lineHeight := 22     // Match the increased line height
	sectionSpacing := 35 // Match the increased section spacing

	// Title
	currentY += 40

	// Subtitle
	currentY += 25

	// Description
	descText := "It aims to closely replicate the economy system in game as much as possible"
	descLines := ws.wrapText(descText, width-40, ws.font)
	currentY += len(descLines)*lineHeight + sectionSpacing

	// First Time Setup section
	setupItems := []string{
		"To get things started quick, you want to import all guilds the first time you run the app",
		"Press G to bring up guild management",
		"Then press API import to import all guilds and the map",
	}
	currentY += 25 // title
	currentY += len(setupItems)*lineHeight + sectionSpacing

	// Features section (large section)
	featureItems := []string{
		"Multiple guild supports",
		"Resource/storage based on how much time has elapsed",
		"Route calculation, affected by tax, border closure and trading style",
		"Ability to live travel into the future with manual tick advancement or stop the clock with state halting",
		"Save and load state/session to file for sharing/later use",
		"Import current map data directly from the API (this only imports guild claims, not their upgrades)",
		"Ability to enable/disable treasury calculation",
		"Treasury overrides, allow you to explicitly set a territory to treasury level of your choice",
		"Manually editable resource storage at runtime, so your experiment isn't limited by available resource",
		"Loadouts, apply mode (just like in game) and merge mode",
		"Tribute system with an option to spawn in or void resources through \"M Src/Source\" guild instead of from other guild on map",
		"In-transit resource inspector, see where all the resources in transit system go",
		"Scriptable, write your own Javascript code to be run within the simulation context",
	}
	currentY += 25 // title
	currentY += len(featureItems)*lineHeight + sectionSpacing

	// Keybinds section
	keybindItems := []string{
		"G - Guild management; you can add/remove guilds or edit guild's clan",
		"P - State management; this menu lets you control the tick rate, modify the logic/calculation/behavior or save and load state session to and from file",
		"L - Loadout menu; create loadout to apply to many territories, there are two application modes: apply which overrides the previous territory setting and apply the loadout's one and merge which merge non-zero data from loadout to territory",
		"B - Tribute configuration; set up your tribute here you can set up a tribute between 2 guilds on the map or spawn in tribute from nothing to the HQ (Source) or simulate sending out tribute to non-existent guild on the map where (Void)",
		"I - Resource inspector; unfinished and abandoned in transit resource inspector and editor",
		"S - Script; run your own Javascript code that will be interacted with the economy simulator here",
		"Tab - Territory view switcher; switch between guild view, resource view, defence and moral",
	}
	currentY += 25 // title
	currentY += len(keybindItems)*lineHeight + sectionSpacing

	// Usage Tips section
	usageTipItems := []string{
		"Double click on a territory to open territory menu",
		"Note: if everything shows up as 0 or seems like nothing is working, press P and click Resume (if it says resume) to un-halt the state",
	}
	currentY += 25 // title
	currentY += len(usageTipItems)*lineHeight + sectionSpacing

	// Calculate max scroll needed
	ws.maxScroll = maxInt(0, currentY-y-height+50)
}

// drawSection renders a section with title and bullet points
func (ws *WelcomeScreen) drawSection(screen *ebiten.Image, title string, items []string, x, startY, width, viewY, viewHeight int, titleColor color.RGBA) int {
	currentY := startY
	lineHeight := 22 // Match the main content line height
	itemPadding := 5 // Padding between items

	// Draw section title with more padding
	if currentY > viewY-lineHeight && currentY < viewY+viewHeight {
		text.Draw(screen, title, ws.titleFont, x+10, currentY+18, titleColor)
	}
	currentY += 35 // More space after title

	// Draw items
	for i, item := range items {
		// Add extra padding between items (except the first one)
		if i > 0 {
			currentY += itemPadding
		}

		// Word wrap long items
		wrappedLines := ws.wrapText(item, width-20, ws.font)
		for j, line := range wrappedLines {
			if currentY > viewY-lineHeight && currentY < viewY+viewHeight {
				bulletText := line
				if j == 0 {
					bulletText = "• " + line
				} else {
					bulletText = "  " + line // Indent continuation lines
				}
				text.Draw(screen, bulletText, ws.font, x+15, currentY+15, color.RGBA{220, 220, 220, 255}) // More indentation
			}
			currentY += lineHeight
		}
	}

	return currentY + 20 // Add spacing after section
}

// wrapText wraps text to fit within the given width
func (ws *WelcomeScreen) wrapText(textContent string, maxWidth int, fontFace font.Face) []string {
	words := strings.Fields(textContent)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		testLine := currentLine + " " + word
		bounds := text.BoundString(fontFace, testLine)

		if bounds.Dx() <= maxWidth {
			currentLine = testLine
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	lines = append(lines, currentLine)
	return lines
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
