package app

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// SelectionAnywhereItem represents an item in the context menu
type SelectionAnywhereItem struct {
	Text       string             // Left side text
	RightText  string             // Right side text (shortcut, etc.)
	Action     func()             // Callback function
	Enabled    bool               // Whether the item is clickable
	Divider    bool               // If true, renders a divider instead of clickable item
	SubMenu    *SelectionAnywhere // Nested context menu
	hasSubMenu bool               // Whether this item has a submenu
}

// SelectionAnywhere represents a context menu that appears on right-click
type SelectionAnywhere struct {
	X, Y          int
	Width, Height int
	items         []SelectionAnywhereItem
	isVisible     bool
	hoveredIndex  int
	font          font.Face

	// Animation properties
	animPhase   float64
	animSpeed   float64
	isAnimating bool

	// Highlight animation
	highlightPosY        float64
	highlightTargetY     float64
	highlightLastUpdate  time.Time
	highlightInitialized bool
	lastMouseX           int
	lastMouseY           int
	lastMouseMove        time.Time

	// Visual properties
	backgroundColor color.RGBA
	borderColor     color.RGBA
	textColor       color.RGBA
	rightTextColor  color.RGBA
	disabledColor   color.RGBA
	hoverColor      color.RGBA
	dividerColor    color.RGBA
	shadowColor     color.RGBA

	// Layout properties
	itemHeight int
	padding    int
	maxWidth   int
	minWidth   int

	// Interaction state
	lastShowTime  time.Time
	clickPos      image.Point        // Position where right-click occurred
	parentMenu    *SelectionAnywhere // Reference to parent menu for nested menus
	activeSubMenu *SelectionAnywhere // Currently active submenu
	subMenuTimer  time.Time          // Timer for submenu hover delay
	hoverDelay    time.Duration      // Delay before showing submenu
}

var activeContextMenu *SelectionAnywhere

// NewSelectionAnywhere creates a new context menu with builder pattern
func NewSelectionAnywhere() *SelectionAnywhere {
	return &SelectionAnywhere{
		Width:        200,
		Height:       0, // Will be calculated based on items
		items:        make([]SelectionAnywhereItem, 0),
		isVisible:    false,
		hoveredIndex: -1,
		font:         loadWynncraftFont(14),
		animPhase:    0.0,
		animSpeed:    8.0,
		isAnimating:  false,

		highlightPosY:        0,
		highlightTargetY:     0,
		highlightLastUpdate:  time.Now(),
		highlightInitialized: false,
		lastMouseX:           0,
		lastMouseY:           0,
		lastMouseMove:        time.Now(),

		// Use enhanced UI colors for consistency
		backgroundColor: EnhancedUIColors.ModalBackground,
		borderColor:     EnhancedUIColors.Border,
		textColor:       EnhancedUIColors.Text,
		rightTextColor:  EnhancedUIColors.TextSecondary,
		disabledColor:   EnhancedUIColors.TextPlaceholder,
		hoverColor:      EnhancedUIColors.ItemHover,
		dividerColor:    EnhancedUIColors.Border,
		shadowColor:     color.RGBA{0, 0, 0, 100},

		itemHeight: 28,
		padding:    6,
		maxWidth:   300,
		minWidth:   150,
		hoverDelay: 500 * time.Millisecond, // 500ms delay before showing submenu
	}
}

// Option adds a menu option with builder pattern
func (sa *SelectionAnywhere) Option(text, rightText string, enabled bool, action func()) *SelectionAnywhere {
	sa.items = append(sa.items, SelectionAnywhereItem{
		Text:       text,
		RightText:  rightText,
		Action:     action,
		Enabled:    enabled,
		Divider:    false,
		hasSubMenu: false,
	})
	sa.updateDimensions()
	return sa
}

// Divider adds a visual divider with builder pattern
func (sa *SelectionAnywhere) Divider() *SelectionAnywhere {
	sa.items = append(sa.items, SelectionAnywhereItem{
		Text:       "",
		RightText:  "",
		Action:     nil,
		Enabled:    false,
		Divider:    true,
		hasSubMenu: false,
	})
	sa.updateDimensions()
	return sa
}

// ContextMenu adds a submenu option with builder pattern
func (sa *SelectionAnywhere) ContextMenu(text, rightText string, enabled bool, subMenu *SelectionAnywhere) *SelectionAnywhere {
	if subMenu != nil {
		subMenu.parentMenu = sa
	}
	sa.items = append(sa.items, SelectionAnywhereItem{
		Text:       text,
		RightText:  rightText,
		Action:     nil, // Submenus don't have direct actions
		Enabled:    enabled,
		Divider:    false,
		SubMenu:    subMenu,
		hasSubMenu: true,
	})
	sa.updateDimensions()
	return sa
}

// updateDimensions calculates the menu dimensions based on items
func (sa *SelectionAnywhere) updateDimensions() {
	if len(sa.items) == 0 {
		sa.Height = 0
		return
	}

	// Calculate height based on items
	height := sa.padding * 2
	for _, item := range sa.items {
		if item.Divider {
			height += 8 // Divider height
		} else {
			height += sa.itemHeight
		}
	}
	sa.Height = height

	// Calculate width based on text content
	if sa.font != nil {
		maxWidth := sa.minWidth
		for _, item := range sa.items {
			if !item.Divider {
				// Measure left text
				leftBounds := text.BoundString(sa.font, item.Text)
				leftWidth := leftBounds.Dx()

				// Measure right text
				rightWidth := 0
				if item.RightText != "" {
					rightBounds := text.BoundString(sa.font, item.RightText)
					rightWidth = rightBounds.Dx()
				}

				// Add spacing for submenu arrow if needed
				arrowWidth := 0
				if item.hasSubMenu {
					arrowWidth = 20 // Space for ">" arrow
				}

				// Total width: padding + left text + spacing + right text + arrow + padding
				totalWidth := sa.padding*2 + leftWidth + 30 + rightWidth + arrowWidth

				if totalWidth > maxWidth {
					maxWidth = totalWidth
				}
			}
		}

		sa.Width = maxWidth
		if sa.Width > sa.maxWidth {
			sa.Width = sa.maxWidth
		}
	}
}

// Show displays the context menu at the specified position
func (sa *SelectionAnywhere) Show(x, y int, screenW, screenH int) {
	// Store the click position
	sa.clickPos = image.Point{X: x, Y: y}

	// Adjust position to keep menu on screen
	sa.X = x
	sa.Y = y

	// Ensure menu doesn't go off the right edge
	if sa.X+sa.Width > screenW {
		sa.X = screenW - sa.Width - 10
	}

	// Ensure menu doesn't go off the bottom edge
	if sa.Y+sa.Height > screenH {
		sa.Y = screenH - sa.Height - 10
	}

	// Ensure menu doesn't go off the left edge
	if sa.X < 10 {
		sa.X = 10
	}

	// Ensure menu doesn't go off the top edge
	if sa.Y < 10 {
		sa.Y = 10
	}

	sa.isVisible = true
	sa.isAnimating = true
	sa.animPhase = 0.0
	sa.hoveredIndex = -1
	sa.lastShowTime = time.Now()
	sa.activeSubMenu = nil
	sa.highlightInitialized = false
	sa.highlightLastUpdate = time.Now()
	activeContextMenu = sa
}

// Hide hides the context menu and any submenus
func (sa *SelectionAnywhere) Hide() {
	sa.isVisible = false
	sa.isAnimating = false
	sa.animPhase = 0.0
	sa.hoveredIndex = -1
	sa.highlightInitialized = false

	// Hide any active submenu
	if sa.activeSubMenu != nil {
		sa.activeSubMenu.Hide()
		sa.activeSubMenu = nil
	}
	if activeContextMenu == sa {
		activeContextMenu = nil
	}
}

// IsVisible returns whether the context menu is currently visible
func (sa *SelectionAnywhere) IsVisible() bool {
	return sa.isVisible
}

// Update handles input and animation for the context menu
func (sa *SelectionAnywhere) Update() bool {
	if !sa.isVisible {
		return false
	}

	now := time.Now()

	// Update animation
	if sa.isAnimating && sa.animPhase < 1.0 {
		sa.animPhase += sa.animSpeed / 60.0 // Assuming 60 FPS
		if sa.animPhase >= 1.0 {
			sa.animPhase = 1.0
			sa.isAnimating = false
		}
	}

	// Update active submenu first
	if sa.activeSubMenu != nil && sa.activeSubMenu.IsVisible() {
		if sa.activeSubMenu.Update() {
			return true // Submenu consumed the input
		}
	}

	// Get primary pointer position (touch first, mouse fallback)
	mx, my := primaryPointerPosition()
	if mx != sa.lastMouseX || my != sa.lastMouseY {
		sa.lastMouseMove = now
		sa.lastMouseX = mx
		sa.lastMouseY = my
	}

	// Update hover state
	oldHoveredIndex := sa.hoveredIndex
	sa.hoveredIndex = -1
	isPointerOverMenu := mx >= sa.X && mx < sa.X+sa.Width && my >= sa.Y && my < sa.Y+sa.Height

	if isPointerOverMenu {
		// Calculate which item is hovered
		relativeY := my - sa.Y - sa.padding
		currentY := 0

		for i, item := range sa.items {
			if item.Divider {
				currentY += 8
			} else {
				if relativeY >= currentY && relativeY < currentY+sa.itemHeight {
					sa.hoveredIndex = i
					break
				}
				currentY += sa.itemHeight
			}
		}
	}

	// Handle submenu logic
	if sa.hoveredIndex != oldHoveredIndex {
		// Hover changed
		if sa.hoveredIndex >= 0 && sa.hoveredIndex < len(sa.items) {
			item := sa.items[sa.hoveredIndex]
			if item.hasSubMenu && item.Enabled {
				// Start timer for submenu
				sa.subMenuTimer = time.Now()
			} else {
				// Hide any active submenu if hovering over non-submenu item
				if sa.activeSubMenu != nil {
					sa.activeSubMenu.Hide()
					sa.activeSubMenu = nil
				}
			}
		} else {
			// No item hovered, hide submenu
			if sa.activeSubMenu != nil {
				sa.activeSubMenu.Hide()
				sa.activeSubMenu = nil
			}
		}
	}

	// Update highlight animation target
	if sa.hoveredIndex >= 0 && sa.hoveredIndex < len(sa.items) {
		item := sa.items[sa.hoveredIndex]
		if !item.Divider && item.Enabled {
			itemY, itemHeight, ok := sa.itemTop(sa.hoveredIndex)
			if ok {
				mouseMovedRecently := now.Sub(sa.lastMouseMove) < 150*time.Millisecond
				if mouseMovedRecently {
					pointerOffset := float64(my - (sa.Y + sa.padding + itemY))
					desiredTop := float64(itemY) + (pointerOffset-float64(itemHeight)/2.0)*0.6
					maxShift := float64(itemHeight) * 0.35
					minTop := float64(itemY) - maxShift
					maxTop := float64(itemY) + maxShift
					if desiredTop < minTop {
						desiredTop = minTop
					}
					if desiredTop > maxTop {
						desiredTop = maxTop
					}
					stickiness := 0.7
					sa.highlightTargetY = desiredTop*(1.0-stickiness) + float64(itemY)*stickiness
				} else {
					sa.highlightTargetY = float64(itemY)
				}

				if !sa.highlightInitialized {
					sa.highlightPosY = sa.highlightTargetY
					sa.highlightInitialized = true
					sa.highlightLastUpdate = now
				}
			}
		}
	}

	if sa.highlightInitialized {
		dt := now.Sub(sa.highlightLastUpdate).Seconds()
		if dt < 0 {
			dt = 0
		}
		if dt > 0.1 {
			dt = 0.1
		}
		sa.highlightLastUpdate = now

		moveSpeed := 90.0
		alpha := 1 - math.Exp(-moveSpeed*dt)
		ease := smoothstep(alpha)
		sa.highlightPosY = sa.highlightPosY + (sa.highlightTargetY-sa.highlightPosY)*ease
	}

	// Check if we should show submenu after hover delay
	if sa.hoveredIndex >= 0 && sa.hoveredIndex < len(sa.items) {
		item := sa.items[sa.hoveredIndex]
		if item.hasSubMenu && item.Enabled && sa.activeSubMenu == nil {
			if time.Since(sa.subMenuTimer) > sa.hoverDelay {
				// Show submenu
				sa.activeSubMenu = item.SubMenu
				if sa.activeSubMenu != nil {
					// Position submenu to the right of the current menu
					subX := sa.X + sa.Width - 5 // Slight overlap
					subY := sa.Y + sa.padding + sa.hoveredIndex*sa.itemHeight

					// Adjust for dividers before this item
					dividerOffset := 0
					for i := 0; i < sa.hoveredIndex; i++ {
						if sa.items[i].Divider {
							dividerOffset += 8 - sa.itemHeight
						}
					}
					subY += dividerOffset

					sa.activeSubMenu.Show(subX, subY, 1920, 1080) // Use reasonable screen bounds
				}
			}
		}
	}

	// Handle mouse clicks
	if px, py, pressed := primaryJustPressed(); pressed {
		if px >= sa.X && px < sa.X+sa.Width && py >= sa.Y && py < sa.Y+sa.Height {
			// Tap/click inside menu
			if sa.hoveredIndex >= 0 && sa.hoveredIndex < len(sa.items) {
				item := sa.items[sa.hoveredIndex]
				if item.Enabled {
					if item.hasSubMenu && item.SubMenu != nil {
						// Open submenu immediately on tap
						sa.activeSubMenu = item.SubMenu
						// Position submenu to the right of the current menu
						subX := sa.X + sa.Width - 5 // Slight overlap
						subY := sa.Y + sa.padding + sa.hoveredIndex*sa.itemHeight

						// Adjust for dividers before this item
						dividerOffset := 0
						for i := 0; i < sa.hoveredIndex; i++ {
							if sa.items[i].Divider {
								dividerOffset += 8 - sa.itemHeight
							}
						}
						subY += dividerOffset

						sa.activeSubMenu.Show(subX, subY, 1920, 1080)
						return true
					}

					if !item.hasSubMenu && item.Action != nil {
						item.Action()
						sa.Hide()
						return true
					}
				}
			}
		} else {
			// Tap/click outside menu - hide it
			sa.Hide()
			return true
		}
	}

	// Handle right-click to hide menu (ignore immediate right-click after show)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if time.Since(sa.lastShowTime) > 120*time.Millisecond {
			sa.Hide()
			return true
		}
	}

	// Handle escape key to hide menu
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		sa.Hide()
		return true
	}

	// If menu is visible, consume all input to prevent interaction with background
	return sa.isVisible
}

// Draw renders the context menu
func (sa *SelectionAnywhere) Draw(screen *ebiten.Image) {
	if !sa.isVisible || len(sa.items) == 0 {
		return
	}

	// Calculate animation scale and alpha
	scale := 1.0
	alpha := uint8(255)

	if sa.isAnimating {
		// Ease-out animation
		t := sa.animPhase
		scale = 0.8 + 0.2*easeOutCubic(t)
		alpha = uint8(255 * easeOutCubic(t))
	}

	// Calculate scaled dimensions
	scaledWidth := int(float64(sa.Width) * scale)
	scaledHeight := int(float64(sa.Height) * scale)

	// Calculate centered position for scaling
	offsetX := (sa.Width - scaledWidth) / 2
	offsetY := (sa.Height - scaledHeight) / 2

	drawX := sa.X + offsetX
	drawY := sa.Y + offsetY

	// Draw shadow first
	shadowOffset := 3
	shadowColor := color.RGBA{
		R: sa.shadowColor.R,
		G: sa.shadowColor.G,
		B: sa.shadowColor.B,
		A: uint8(float64(sa.shadowColor.A) * float64(alpha) / 255),
	}
	ebitenutil.DrawRect(screen,
		float64(drawX+shadowOffset), float64(drawY+shadowOffset),
		float64(scaledWidth), float64(scaledHeight),
		shadowColor)

	// Draw background
	bgColor := color.RGBA{
		R: sa.backgroundColor.R,
		G: sa.backgroundColor.G,
		B: sa.backgroundColor.B,
		A: uint8(float64(sa.backgroundColor.A) * float64(alpha) / 255),
	}
	ebitenutil.DrawRect(screen,
		float64(drawX), float64(drawY),
		float64(scaledWidth), float64(scaledHeight),
		bgColor)

	// Draw border
	borderColor := color.RGBA{
		R: sa.borderColor.R,
		G: sa.borderColor.G,
		B: sa.borderColor.B,
		A: uint8(float64(sa.borderColor.A) * float64(alpha) / 255),
	}

	// Draw border lines
	ebitenutil.DrawRect(screen, float64(drawX), float64(drawY), float64(scaledWidth), 1, borderColor)
	ebitenutil.DrawRect(screen, float64(drawX), float64(drawY+scaledHeight-1), float64(scaledWidth), 1, borderColor)
	ebitenutil.DrawRect(screen, float64(drawX), float64(drawY), 1, float64(scaledHeight), borderColor)
	ebitenutil.DrawRect(screen, float64(drawX+scaledWidth-1), float64(drawY), 1, float64(scaledHeight), borderColor)

	// Draw items
	if sa.font != nil && alpha > 0 {
		// Draw animated highlight
		if sa.hoveredIndex >= 0 && sa.hoveredIndex < len(sa.items) {
			item := sa.items[sa.hoveredIndex]
			if !item.Divider && item.Enabled && sa.highlightInitialized {
				hoverColor := color.RGBA{
					R: sa.hoverColor.R,
					G: sa.hoverColor.G,
					B: sa.hoverColor.B,
					A: uint8(float64(sa.hoverColor.A) * float64(alpha) / 255),
				}
				itemHeight := int(float64(sa.itemHeight) * scale)
				highlightY := drawY + int(float64(sa.padding)*scale) + int(sa.highlightPosY*scale)
				ebitenutil.DrawRect(screen,
					float64(drawX), float64(highlightY),
					float64(scaledWidth), float64(itemHeight),
					hoverColor)
			}
		}

		currentY := drawY + int(float64(sa.padding)*scale)

		for _, item := range sa.items {
			if item.Divider {
				// Draw divider line
				dividerY := currentY + 4
				dividerColor := color.RGBA{
					R: sa.dividerColor.R,
					G: sa.dividerColor.G,
					B: sa.dividerColor.B,
					A: uint8(float64(sa.dividerColor.A) * float64(alpha) / 255),
				}
				ebitenutil.DrawRect(screen,
					float64(drawX+int(float64(sa.padding)*scale)), float64(dividerY),
					float64(scaledWidth-2*int(float64(sa.padding)*scale)), 1,
					dividerColor)
				currentY += 8
			} else {
				itemY := currentY
				itemHeight := int(float64(sa.itemHeight) * scale)

				// Choose text color based on enabled state
				leftTextColor := sa.textColor
				if !item.Enabled {
					leftTextColor = sa.disabledColor
				}
				leftTextColor = color.RGBA{
					R: leftTextColor.R,
					G: leftTextColor.G,
					B: leftTextColor.B,
					A: uint8(float64(leftTextColor.A) * float64(alpha) / 255),
				}

				// Draw left text
				textX := drawX + int(float64(sa.padding)*scale)
				textY := itemY + itemHeight/2 + 4 // Center text vertically
				drawTextWithOffset(screen, item.Text, sa.font, textX, textY, leftTextColor)

				// Draw right text if present
				if item.RightText != "" {
					rightTextColor := color.RGBA{
						R: sa.rightTextColor.R,
						G: sa.rightTextColor.G,
						B: sa.rightTextColor.B,
						A: uint8(float64(sa.rightTextColor.A) * float64(alpha) / 255),
					}
					rightTextBounds := text.BoundString(sa.font, item.RightText)
					rightTextWidth := rightTextBounds.Dx()

					arrowOffset := 0
					if item.hasSubMenu {
						arrowOffset = 20
					}

					rightTextX := drawX + scaledWidth - int(float64(sa.padding)*scale) - rightTextWidth - arrowOffset
					drawTextWithOffset(screen, item.RightText, sa.font, rightTextX, textY, rightTextColor)
				}

				// Draw submenu arrow if this item has a submenu
				if item.hasSubMenu {
					arrowColor := leftTextColor
					arrowX := drawX + scaledWidth - int(float64(sa.padding)*scale) - 12
					drawTextWithOffset(screen, ">", sa.font, arrowX, textY, arrowColor)
				}

				currentY += itemHeight
			}
		}
	}

	// Draw active submenu on top
	if sa.activeSubMenu != nil && sa.activeSubMenu.IsVisible() {
		sa.activeSubMenu.Draw(screen)
	}
}

func (sa *SelectionAnywhere) itemTop(index int) (int, int, bool) {
	currentY := 0
	for i, item := range sa.items {
		if item.Divider {
			currentY += 8
			continue
		}
		if i == index {
			return currentY, sa.itemHeight, true
		}
		currentY += sa.itemHeight
	}
	return 0, 0, false
}

// GetClickPosition returns the position where the context menu was triggered
func (sa *SelectionAnywhere) GetClickPosition() image.Point {
	return sa.clickPos
}

// SetMaxWidth sets the maximum width for the context menu
func (sa *SelectionAnywhere) SetMaxWidth(width int) {
	sa.maxWidth = width
	sa.updateDimensions()
}

func SetActiveContextMenu(menu *SelectionAnywhere) {
	activeContextMenu = menu
}

func DrawActiveContextMenu(screen *ebiten.Image) {
	if activeContextMenu != nil && activeContextMenu.IsVisible() {
		activeContextMenu.Draw(screen)
	}
}
