package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// MANUAL MOUSE OFFSET ADJUSTMENT - Adjust these if mouse detection feels off
// Positive X moves the detection area right, negative moves it left
// Positive Y moves the detection area down, negative moves it up
const (
	MOUSE_OFFSET_X = 90.0
	MOUSE_OFFSET_Y = 128.0
)

// UndoExplorer provides a GUI for visualizing and navigating the undo tree
type UndoExplorer struct {
	visible     bool
	modal       *EnhancedModal
	offsetX     float64 // Camera offset X
	offsetY     float64 // Camera offset Y
	scale       float64 // Zoom scale
	isDragging  bool
	lastMouseX  int
	lastMouseY  int
	hoveredNode *UndoNode
	font        font.Face
	smallFont   font.Face
}

var (
	globalUndoExplorer *UndoExplorer
)

// GetUndoExplorer returns the global undo explorer instance
func GetUndoExplorer() *UndoExplorer {
	if globalUndoExplorer == nil {
		globalUndoExplorer = NewUndoExplorer()
	}
	return globalUndoExplorer
}

// NewUndoExplorer creates a new undo explorer
func NewUndoExplorer() *UndoExplorer {
	screenW, screenH := ebiten.WindowSize()
	modalWidth := int(float64(screenW) * 0.9)
	modalHeight := int(float64(screenH) * 0.9)

	return &UndoExplorer{
		visible:    false,
		modal:      NewEnhancedModal("Edit History Explorer", modalWidth, modalHeight),
		offsetX:    0,
		offsetY:    0,
		scale:      1.0,
		isDragging: false,
		font:       loadWynncraftFont(14),
		smallFont:  loadWynncraftFont(12),
	}
}

// Show displays the undo explorer
func (ue *UndoExplorer) Show() {
	ue.visible = true

	// Update modal position for current window size
	ue.updateModalPosition()

	ue.modal.Show()

	// Center on current node
	um := GetUndoManager()
	tree := um.GetTree()
	if tree.CurrentNode != nil {
		// Center the view on the current node within the content area
		_, _, contentW, contentH := ue.modal.GetContentArea()
		// The formula is: screenX = nodeX * scale + offsetX
		// We want: screenX = contentW/2
		// So: offsetX = contentW/2 - nodeX * scale
		ue.offsetX = float64(contentW)/2 - tree.CurrentNode.X*ue.scale
		ue.offsetY = float64(contentH-35)/2 - tree.CurrentNode.Y*ue.scale // Account for title bar
	}
}

// updateModalPosition updates the modal's position and size based on current window size
func (ue *UndoExplorer) updateModalPosition() {
	screenW, screenH := ebiten.WindowSize()
	modalWidth := int(float64(screenW) * 0.9)
	modalHeight := int(float64(screenH) * 0.9)

	// Center the modal
	ue.modal.X = (screenW - modalWidth) / 2
	ue.modal.Y = (screenH - modalHeight) / 2
	ue.modal.Width = modalWidth
	ue.modal.Height = modalHeight
}

// Hide hides the undo explorer
func (ue *UndoExplorer) Hide() {
	ue.visible = false
	ue.modal.Hide()
}

// IsVisible returns whether the explorer is visible
func (ue *UndoExplorer) IsVisible() bool {
	return ue.visible
}

// Update handles input and updates the explorer
func (ue *UndoExplorer) Update() bool {
	if !ue.visible {
		return false
	}

	// Update modal position in case window was resized
	ue.updateModalPosition()

	// Check for close (Escape key only - Z is handled by gameplay)
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		ue.Hide()
		return true
	}

	// Get mouse position
	mx, my := ebiten.CursorPosition()

	// Check if mouse is over modal
	if !ue.modal.Contains(mx, my) {
		// Click outside to close
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			ue.Hide()
			return true
		}
		return true
	}

	// Handle mouse wheel for zooming
	_, dy := ebiten.Wheel()
	if dy != 0 {
		oldScale := ue.scale
		ue.scale += dy * 0.1

		// Clamp scale
		if ue.scale < 0.2 {
			ue.scale = 0.2
		} else if ue.scale > 3.0 {
			ue.scale = 3.0
		}

		// Adjust offset to zoom towards mouse position
		contentX, contentY, _, _ := ue.modal.GetContentArea()
		localMX := float64(mx - contentX)
		localMY := float64(my - contentY - 35) // Account for title bar

		// Adjust offset to keep the point under the mouse in the same place
		scaleRatio := ue.scale / oldScale
		ue.offsetX = localMX - (localMX-ue.offsetX)*scaleRatio
		ue.offsetY = localMY - (localMY-ue.offsetY)*scaleRatio
	}

	// Handle mouse dragging
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		ue.isDragging = true
		ue.lastMouseX = mx
		ue.lastMouseY = my
	}

	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if ue.isDragging {
			// Check if we're over a node and not dragging (click)
			if math.Abs(float64(mx-ue.lastMouseX)) < 5 && math.Abs(float64(my-ue.lastMouseY)) < 5 {
				if ue.hoveredNode != nil {
					// Navigate to this node
					um := GetUndoManager()
					um.SetCurrentNode(ue.hoveredNode.ID)

					// Show toast
					ShowSimpleToast(fmt.Sprintf("Jumped to: %s", ue.hoveredNode.Description))
				}
			}
		}
		ue.isDragging = false
	}

	if ue.isDragging && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		// Update offset based on mouse movement
		dx := mx - ue.lastMouseX
		dy := my - ue.lastMouseY
		ue.offsetX += float64(dx)
		ue.offsetY += float64(dy)
		ue.lastMouseX = mx
		ue.lastMouseY = my
	}

	// Update hovered node
	ue.updateHoveredNode(mx, my)

	return true
}

// updateHoveredNode determines which node the mouse is hovering over
func (ue *UndoExplorer) updateHoveredNode(mx, my int) {
	ue.hoveredNode = nil

	contentX, contentY, contentW, contentH := ue.modal.GetContentArea()
	if contentW == 0 || contentH == 0 {
		return
	}

	localMX := float64(mx-contentX) + MOUSE_OFFSET_X
	localMY := float64(my-contentY-35) + MOUSE_OFFSET_Y // Account for title bar

	// Check all nodes and find the closest one within hover distance
	um := GetUndoManager()
	tree := um.GetTree()

	var closestNode *UndoNode
	var closestDist float64 = math.MaxFloat64
	nodeRadius := 20.0 * ue.scale

	for _, node := range tree.Nodes {
		// Transform node position to screen space (within SubImage coordinates)
		screenX := node.X*ue.scale + ue.offsetX
		screenY := node.Y*ue.scale + ue.offsetY

		// Calculate distance to node
		dx := localMX - screenX
		dy := localMY - screenY
		distSq := dx*dx + dy*dy
		dist := math.Sqrt(distSq)

		// Check if this is the closest node within hover radius
		if dist < nodeRadius && dist < closestDist {
			closestNode = node
			closestDist = dist
		}
	}

	ue.hoveredNode = closestNode
}

// Draw renders the undo explorer
func (ue *UndoExplorer) Draw(screen *ebiten.Image) {
	if !ue.visible {
		return
	}

	// Draw modal background
	ue.modal.Draw(screen)

	// Draw content
	contentX, contentY, contentW, contentH := ue.modal.GetContentArea()
	if contentW == 0 || contentH == 0 {
		return
	}

	// Create a sub-image for the content area
	contentArea := screen.SubImage(image.Rect(
		contentX, contentY+35, // Account for title bar
		contentX+contentW, contentY+contentH,
	)).(*ebiten.Image)

	// Draw the graph (SubImage coordinates start at 0,0)
	ue.drawGraph(contentArea, 0, 0)

	// Draw instructions at the bottom
	ue.drawInstructions(screen, contentX, contentY, contentW, contentH)
}

// drawGraph renders the undo tree graph
func (ue *UndoExplorer) drawGraph(screen *ebiten.Image, offsetX, offsetY int) {
	um := GetUndoManager()
	tree := um.GetTree()

	if tree.Root == nil {
		return
	}

	// Draw all edges first
	ue.drawEdges(screen, tree, offsetX, offsetY)

	// Draw all nodes
	ue.drawNodes(screen, tree, offsetX, offsetY)
}

// drawEdges draws connections between nodes
func (ue *UndoExplorer) drawEdges(screen *ebiten.Image, tree *UndoTree, offsetX, offsetY int) {
	for _, node := range tree.Nodes {
		for _, child := range node.Children {
			// Calculate screen positions
			x1 := float32((node.X*ue.scale + ue.offsetX))
			y1 := float32((node.Y*ue.scale + ue.offsetY))
			x2 := float32((child.X*ue.scale + ue.offsetX))
			y2 := float32((child.Y*ue.scale + ue.offsetY))

			// Choose color based on whether the edge is in the active path
			edgeColor := color.RGBA{100, 100, 100, 255}
			if node.IsActive && child.IsActive {
				edgeColor = color.RGBA{100, 150, 255, 255} // Blue for active path
			}

			// Draw line
			vector.StrokeLine(screen, x1, y1, x2, y2, 2, edgeColor, false)
		}
	}
}

// drawNodes draws all nodes in the tree
func (ue *UndoExplorer) drawNodes(screen *ebiten.Image, tree *UndoTree, offsetX, offsetY int) {
	for _, node := range tree.Nodes {
		ue.drawNode(screen, node, tree.CurrentNode == node, offsetX, offsetY)
	}
}

// drawNode draws a single node
func (ue *UndoExplorer) drawNode(screen *ebiten.Image, node *UndoNode, isCurrent bool, offsetX, offsetY int) {
	// Calculate screen position
	screenX := float32(node.X*ue.scale + ue.offsetX)
	screenY := float32(node.Y*ue.scale + ue.offsetY)

	// Node appearance
	nodeRadius := float32(20.0 * ue.scale)

	// Choose color
	var nodeColor color.RGBA
	if isCurrent {
		nodeColor = color.RGBA{60, 255, 60, 255} // Bright green for current
	} else if node.IsActive {
		nodeColor = color.RGBA{100, 150, 255, 255} // Blue for active path
	} else {
		nodeColor = color.RGBA{150, 150, 150, 255} // Gray for inactive
	}

	// Highlight if hovered
	if ue.hoveredNode == node {
		nodeColor = color.RGBA{255, 200, 100, 255} // Orange when hovered
	}

	// Draw node circle
	vector.DrawFilledCircle(screen, screenX, screenY, nodeRadius, nodeColor, false)

	// Draw border
	borderColor := color.RGBA{255, 255, 255, 255}
	if isCurrent {
		borderColor = color.RGBA{255, 255, 100, 255} // Yellow border for current
	}
	vector.StrokeCircle(screen, screenX, screenY, nodeRadius, 2, borderColor, false)

	// Draw label (only if zoomed in enough)
	if ue.scale > 0.5 && ue.smallFont != nil {
		// Draw territory name below node
		labelColor := color.RGBA{255, 255, 255, 255}

		// Get territory name from description
		label := node.Description
		if len(label) > 30 {
			label = label[:27] + "..."
		}

		// Measure text to center it
		bounds := text.BoundString(ue.smallFont, label)
		textWidth := bounds.Dx()

		textX := int(screenX) - textWidth/2
		textY := int(screenY) + int(nodeRadius) + 15

		// Draw background for text
		bgPadding := 3
		vector.DrawFilledRect(screen,
			float32(textX-bgPadding),
			float32(textY-int(ue.smallFont.Metrics().Ascent.Ceil())-bgPadding),
			float32(textWidth+bgPadding*2),
			float32(ue.smallFont.Metrics().Height.Ceil()+bgPadding*2),
			color.RGBA{20, 20, 30, 200},
			false)

		text.Draw(screen, label, ue.smallFont, textX, textY, labelColor)
	}
}

// drawInstructions draws usage instructions
func (ue *UndoExplorer) drawInstructions(screen *ebiten.Image, x, y, w, h int) {
	if ue.font == nil {
		return
	}

	instructions := []string{
		"Mouse Wheel: Zoom",
		"Drag: Pan view",
		"Click Node: Jump to that state",
		"ESC/Z: Close",
	}

	instructionColor := color.RGBA{200, 200, 200, 255}
	startY := y + h - 80

	for i, instruction := range instructions {
		text.Draw(screen, instruction, ue.font, x+20, startY+i*18, instructionColor)
	}
}

// ShowSimpleToast is a helper to show a simple toast message
func ShowSimpleToast(message string) {
	NewToast().
		Text(message, ToastOption{Colour: color.RGBA{255, 255, 255, 255}}).
		AutoClose(3 * time.Second).
		Show()
}
