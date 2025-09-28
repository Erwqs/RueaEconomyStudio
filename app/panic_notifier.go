package app

import (
	"fmt"
	"image/color"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
)

// PanicNotifier handles critical panic situations with a modal dialog
type PanicNotifier struct {
	modal           *EnhancedModal
	panicInfo       *PanicInfo
	stackTrace      string
	visible         bool
	copyButton      *EnhancedButton
	terminateButton *EnhancedButton
	continueButton  *EnhancedButton
	scrollOffset    int
	maxScroll       int
	font            font.Face
	// Scrollbar interaction state
	scrollbarDragging bool
	dragStartY        int
	dragStartOffset   int
}

// PanicInfo contains information about a panic
type PanicInfo struct {
	Error      interface{}
	StackTrace []byte
	Time       time.Time
	GoVersion  string
	OS         string
	Arch       string
}

var globalPanicNotifier *PanicNotifier

// InitPanicNotifier initializes the global panic notification system
func InitPanicNotifier() {
	if globalPanicNotifier != nil {
		return
	}

	pn := &PanicNotifier{
		modal:        NewEnhancedModal("Runtime Error - Panic", 800, 600),
		font:         loadWynncraftFont(14),
		scrollOffset: 0,
	}

	// Create buttons
	pn.setupButtons()

	globalPanicNotifier = pn
}

func DeregisterPanicNotifier() {
	if globalPanicNotifier != nil {
		globalPanicNotifier.Hide() // Hide the modal if it's visible
		globalPanicNotifier = nil
	}
}

// GetPanicNotifier returns the global panic notifier instance
func GetPanicNotifier() *PanicNotifier {
	if globalPanicNotifier == nil {
		InitPanicNotifier()
	}
	return globalPanicNotifier
}

// setupButtons creates the action buttons for the panic dialog
func (pn *PanicNotifier) setupButtons() {
	if pn.modal == nil {
		return
	}

	// Note: Button positions will be calculated dynamically in updateButtonPositions()
	// We create the buttons with temporary positions here
	buttonWidth := 120
	buttonHeight := 40

	// Copy Stacktrace button
	pn.copyButton = NewEnhancedButton("Copy Stacktrace", 0, 0, buttonWidth, buttonHeight, func() {
		if pn.stackTrace != "" {
			if runtime.GOOS == "js" {
				return
			}
			// Try to copy to clipboard
			clipboard.Write(clipboard.FmtText, []byte(pn.stackTrace))
			// Show brief success message via toast
			NewToast().
				Text("Stack trace copied to clipboard", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
				AutoClose(time.Second * 2).
				Show()
		}
	})

	// Terminate button
	pn.terminateButton = NewEnhancedButton("Terminate", 0, 0, buttonWidth, buttonHeight, func() {
		// Deregister panic handler and rethrow panic
		if pn.panicInfo != nil {
			DeregisterPanicNotifier()
			fmt.Printf("Panic occurred: %v\n", pn.panicInfo.Error)
			os.Remove(".rueaes.lock")
			os.Exit(1)
		}
	})

	// Continue button
	pn.continueButton = NewEnhancedButton("Continue", 0, 0, buttonWidth, buttonHeight, func() {
		pn.Hide()
	})

	// Note: EnhancedButton doesn't have SetBackgroundColor method
	// Button styling is handled internally based on their type/usage

	// now it is
	pn.copyButton.SetGrayButtonStyle()
	pn.terminateButton.SetRedButtonStyle()
	pn.continueButton.SetYellowButtonStyle()
}

// updateButtonPositions calculates and updates button positions based on current modal bounds
func (pn *PanicNotifier) updateButtonPositions() {
	if pn.modal == nil || pn.copyButton == nil || pn.terminateButton == nil || pn.continueButton == nil {
		return
	}

	buttonWidth := 120
	buttonSpacing := 15

	// Get modal content area
	modalBounds := pn.modal.GetBounds()
	buttonY := modalBounds.Max.Y - 60

	// Calculate button positions (centered)
	totalButtonWidth := 3*buttonWidth + 2*buttonSpacing
	startX := modalBounds.Min.X + (modalBounds.Dx()-totalButtonWidth)/2

	// Update button positions
	pn.copyButton.X = startX
	pn.copyButton.Y = buttonY

	pn.terminateButton.X = startX + buttonWidth + buttonSpacing
	pn.terminateButton.Y = buttonY

	pn.continueButton.X = startX + 2*(buttonWidth+buttonSpacing)
	pn.continueButton.Y = buttonY
}

// ShowPanic displays the panic notification with error details
func (pn *PanicNotifier) ShowPanic(panicValue interface{}) {
	// Capture panic information
	pn.panicInfo = &PanicInfo{
		Error:      panicValue,
		StackTrace: debug.Stack(),
		Time:       time.Now(),
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
	}

	// Format stack trace for display
	pn.stackTrace = pn.formatStackTrace()

	// Calculate scroll limits
	pn.calculateScrollLimits()

	// Reset scroll position
	pn.scrollOffset = 0

	// Show the modal
	pn.visible = true
	if pn.modal != nil {
		pn.modal.Show()
	}

	// Update button positions after modal is shown
	pn.updateButtonPositions()

	fmt.Printf("[PANIC] Displaying panic notification for: %v\n", panicValue)
}

// formatStackTrace formats the stack trace for display
func (pn *PanicNotifier) formatStackTrace() string {
	if pn.panicInfo == nil {
		return "No panic information available"
	}

	var sb strings.Builder

	// Header information
	sb.WriteString("CRITICAL ERROR DETAILS\n")
	sb.WriteString(fmt.Sprintf("Time: %s\n", pn.panicInfo.Time.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Go Version: %s\n", pn.panicInfo.GoVersion))
	sb.WriteString(fmt.Sprintf("OS/Arch: %s/%s\n", pn.panicInfo.OS, pn.panicInfo.Arch))
	sb.WriteString(fmt.Sprintf("\npanic: %v\n", pn.panicInfo.Error))
	sb.WriteString("\nStack Trace:\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n")

	// Clean up and format stack trace
	stackStr := string(pn.panicInfo.StackTrace)
	lines := strings.Split(stackStr, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Add line numbers for better readability
		if i > 0 && i%2 == 1 && strings.Contains(line, ":") {
			sb.WriteString(fmt.Sprintf("%3d: %s\n", (i+1)/2, line))
		} else {
			sb.WriteString(fmt.Sprintf("     %s\n", line))
		}
	}

	return sb.String()
}

// calculateScrollLimits calculates the maximum scroll offset
func (pn *PanicNotifier) calculateScrollLimits() {
	if pn.font == nil || pn.modal == nil {
		pn.maxScroll = 0
		return
	}

	// Get content area
	contentArea := pn.getContentArea()

	// Calculate total text height
	lines := strings.Split(pn.stackTrace, "\n")
	lineHeight := 16
	totalTextHeight := len(lines) * lineHeight

	// Calculate max scroll
	pn.maxScroll = totalTextHeight - contentArea.Height
	if pn.maxScroll < 0 {
		pn.maxScroll = 0
	}
}

// getContentArea returns the area available for content display
func (pn *PanicNotifier) getContentArea() (contentArea struct{ X, Y, Width, Height int }) {
	if pn.modal == nil {
		return
	}

	bounds := pn.modal.GetBounds()
	padding := 20
	buttonAreaHeight := 80

	contentArea.X = bounds.Min.X + padding
	contentArea.Y = bounds.Min.Y + 50 // Account for title bar
	contentArea.Width = bounds.Dx() - 2*padding
	contentArea.Height = bounds.Dy() - 50 - buttonAreaHeight - padding

	return
}

// Hide hides the panic notification
func (pn *PanicNotifier) Hide() {
	pn.visible = false
	if pn.modal != nil {
		pn.modal.Hide()
	}
}

// IsVisible returns whether the panic notification is currently visible
func (pn *PanicNotifier) IsVisible() bool {
	return pn.visible && pn.modal != nil && pn.modal.IsVisible()
}

// Update handles input for the panic notification
// Note: This modal CANNOT be closed by ESC, mouse back button, or clicking outside
// Users MUST choose one of the three action buttons: Copy, Terminate, or Continue
func (pn *PanicNotifier) Update() bool {
	if !pn.IsVisible() {
		return false
	}

	// Update modal animation
	if pn.modal != nil {
		pn.modal.Update()
	}

	// Update button positions in case modal moved or animated
	pn.updateButtonPositions()

	mx, my := ebiten.CursorPosition()
	modalBounds := pn.modal.GetBounds()

	// Check if mouse is over modal
	mouseOverModal := mx >= modalBounds.Min.X && mx <= modalBounds.Max.X &&
		my >= modalBounds.Min.Y && my <= modalBounds.Max.Y

	// Block all input when modal is visible
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) || // Back button
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) { // Forward button

		if !mouseOverModal {
			return true // Block input to background
		}

		// If clicking outside modal, do not close - force user to use buttons
		// Even mouse back button should not close the panic dialog
		if !mouseOverModal {
			return true
		}
	}

	// Handle scrolling in content area
	if mouseOverModal {
		contentArea := pn.getContentArea()

		// Handle scrollbar interaction first (if scrollbar is present)
		if pn.maxScroll > 0 {
			scrollbarWidth := 8
			scrollbarX := contentArea.X + contentArea.Width - scrollbarWidth

			// Calculate thumb position and size
			thumbHeight := int(float64(contentArea.Height) * float64(contentArea.Height) / float64(contentArea.Height+pn.maxScroll))
			if thumbHeight < 20 {
				thumbHeight = 20
			}
			thumbY := contentArea.Y + int(float64(pn.scrollOffset)/float64(pn.maxScroll)*float64(contentArea.Height-thumbHeight))

			// Check if mouse is over scrollbar area
			if mx >= scrollbarX && mx <= scrollbarX+scrollbarWidth &&
				my >= contentArea.Y && my <= contentArea.Y+contentArea.Height {

				// Handle scrollbar clicks
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					// Check if click is on thumb
					if my >= thumbY && my <= thumbY+thumbHeight {
						// Start dragging thumb
						pn.scrollbarDragging = true
						pn.dragStartY = my
						pn.dragStartOffset = pn.scrollOffset
					} else {
						// Click on track - jump to position
						relativeY := my - contentArea.Y
						newScrollOffset := int(float64(relativeY) / float64(contentArea.Height) * float64(pn.maxScroll))
						if newScrollOffset < 0 {
							newScrollOffset = 0
						} else if newScrollOffset > pn.maxScroll {
							newScrollOffset = pn.maxScroll
						}
						pn.scrollOffset = newScrollOffset
					}
					return true
				}
			}
		}

		// Handle scrollbar dragging
		if pn.scrollbarDragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			contentArea := pn.getContentArea()
			deltaY := my - pn.dragStartY
			scrollableHeight := contentArea.Height

			// Calculate how much scroll offset should change
			if pn.maxScroll > 0 {
				scrollDelta := int(float64(deltaY) / float64(scrollableHeight) * float64(pn.maxScroll))
				newScrollOffset := pn.dragStartOffset + scrollDelta

				if newScrollOffset < 0 {
					newScrollOffset = 0
				} else if newScrollOffset > pn.maxScroll {
					newScrollOffset = pn.maxScroll
				}
				pn.scrollOffset = newScrollOffset
			}
			return true
		}

		// Stop dragging when mouse is released
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			pn.scrollbarDragging = false
		}

		// Handle mouse wheel scrolling in content area (if not dragging scrollbar)
		if !pn.scrollbarDragging &&
			mx >= contentArea.X && mx <= contentArea.X+contentArea.Width &&
			my >= contentArea.Y && my <= contentArea.Y+contentArea.Height {

			_, wheelY := ebiten.Wheel()
			if wheelY != 0 {
				pn.scrollOffset -= int(wheelY * 20) // Scroll speed
				if pn.scrollOffset < 0 {
					pn.scrollOffset = 0
				} else if pn.scrollOffset > pn.maxScroll {
					pn.scrollOffset = pn.maxScroll
				}
				return true
			}
		}

		// Update buttons
		if pn.copyButton != nil {
			pn.copyButton.Update(mx, my)
		}
		if pn.terminateButton != nil {
			pn.terminateButton.Update(mx, my)
		}
		if pn.continueButton != nil {
			pn.continueButton.Update(mx, my)
		}
	}

	// Handle keyboard input - DO NOT allow ESC to close panic dialog
	// Users must explicitly choose an action (Copy, Terminate, or Continue)

	// Block all keyboard input (including ESC)
	for key := ebiten.Key0; key <= ebiten.KeyMax; key++ {
		if inpututil.IsKeyJustPressed(key) {
			return true // Consume all keyboard input without action
		}
	}

	return true // Always block input when panic modal is visible
}

// Draw renders the panic notification
func (pn *PanicNotifier) Draw(screen *ebiten.Image) {
	if !pn.IsVisible() {
		return
	}

	// Draw modal background
	if pn.modal != nil {
		pn.modal.Draw(screen)
	}

	// Draw content
	pn.drawContent(screen)

	// Draw buttons
	if pn.copyButton != nil {
		pn.copyButton.Draw(screen)
	}
	if pn.terminateButton != nil {
		pn.terminateButton.Draw(screen)
	}
	if pn.continueButton != nil {
		pn.continueButton.Draw(screen)
	}
}

// drawContent renders the panic information and stack trace
func (pn *PanicNotifier) drawContent(screen *ebiten.Image) {
	if pn.font == nil || pn.stackTrace == "" {
		return
	}

	contentArea := pn.getContentArea()

	// Draw content background
	vector.DrawFilledRect(screen,
		float32(contentArea.X), float32(contentArea.Y),
		float32(contentArea.Width), float32(contentArea.Height),
		color.RGBA{20, 20, 30, 255}, false)

	// Draw content border
	vector.StrokeRect(screen,
		float32(contentArea.X), float32(contentArea.Y),
		float32(contentArea.Width), float32(contentArea.Height),
		1, color.RGBA{100, 100, 120, 255}, false)

	// Draw scrollable text content
	pn.drawScrollableText(screen, contentArea)

	// Draw scrollbar if needed
	if pn.maxScroll > 0 {
		pn.drawScrollbar(screen, contentArea)
	}
}

// drawScrollableText renders the scrollable stack trace text
func (pn *PanicNotifier) drawScrollableText(screen *ebiten.Image, contentArea struct{ X, Y, Width, Height int }) {
	lines := strings.Split(pn.stackTrace, "\n")
	lineHeight := 16
	startY := contentArea.Y + 10 - pn.scrollOffset

	// Create clipping region
	clipX := contentArea.X + 5
	clipY := contentArea.Y
	clipW := contentArea.Width - 10
	clipH := contentArea.Height

	for i, line := range lines {
		lineY := startY + i*lineHeight

		// Skip lines that are outside the visible area
		if lineY < contentArea.Y-lineHeight || lineY > contentArea.Y+contentArea.Height {
			continue
		}

		// Choose color based on line content
		textColor := color.RGBA{220, 220, 220, 255} // Default light gray
		if strings.Contains(line, "panic:") || strings.Contains(line, "CRITICAL ERROR") {
			textColor = color.RGBA{255, 100, 100, 255} // Red for critical
		} else if strings.Contains(line, "RueaES/") {
			textColor = color.RGBA{100, 200, 255, 255} // Blue for our code
		} else if strings.Contains(line, ".go:") {
			textColor = color.RGBA{255, 200, 100, 255} // Orange for file references
		}

		// Clip text to content area
		if lineY >= clipY && lineY <= clipY+clipH {
			// Truncate line if too long
			maxWidth := clipW - 10
			if pn.font != nil {
				bounds := text.BoundString(pn.font, line)
				if bounds.Dx() > maxWidth {
					// Find a good truncation point
					for len(line) > 0 {
						testBounds := text.BoundString(pn.font, line+"...")
						if testBounds.Dx() <= maxWidth {
							line = line + "..."
							break
						}
						if len(line) > 0 {
							line = line[:len(line)-1]
						}
					}
				}
			}

			text.Draw(screen, line, pn.font, clipX, lineY, textColor)
		}
	}
}

// drawScrollbar renders a scrollbar for the content area
func (pn *PanicNotifier) drawScrollbar(screen *ebiten.Image, contentArea struct{ X, Y, Width, Height int }) {
	scrollbarWidth := 8
	scrollbarX := contentArea.X + contentArea.Width - scrollbarWidth

	// Draw scrollbar track
	vector.DrawFilledRect(screen,
		float32(scrollbarX), float32(contentArea.Y),
		float32(scrollbarWidth), float32(contentArea.Height),
		color.RGBA{40, 40, 50, 255}, false)

	// Calculate thumb position and size
	thumbHeight := int(float64(contentArea.Height) * float64(contentArea.Height) / float64(contentArea.Height+pn.maxScroll))
	if thumbHeight < 20 {
		thumbHeight = 20
	}

	thumbY := contentArea.Y + int(float64(pn.scrollOffset)/float64(pn.maxScroll)*float64(contentArea.Height-thumbHeight))

	// Draw scrollbar thumb
	vector.DrawFilledRect(screen,
		float32(scrollbarX), float32(thumbY),
		float32(scrollbarWidth), float32(thumbHeight),
		color.RGBA{120, 120, 140, 255}, false)
}

// HandlePanic is a global function to handle panics and show the notification
func HandlePanic() {
	if r := recover(); r != nil {
		fmt.Printf("[PANIC] Critical error caught: %v\n", r)

		// Initialize panic notifier if not already done
		pn := GetPanicNotifier()

		// Show panic notification
		pn.ShowPanic(r)

		// Note: We don't re-panic here to allow the application to potentially continue
		// The user can choose to terminate or continue via the dialog
	}
}

// InstallPanicHandler installs a global panic handler that will show the notification
func InstallPanicHandler() {
	defer HandlePanic()
}
