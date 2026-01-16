package app

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
)

// ScissorContext tracks the current drawing target and its origin relative to the
// root screen. It enables nesting clipped regions without losing global coords.
type ScissorContext struct {
	Image  *ebiten.Image
	Origin image.Point
}

// NewScissorContext creates a root scissor context for the provided screen.
func NewScissorContext(img *ebiten.Image) ScissorContext {
	return ScissorContext{Image: img, Origin: image.Point{0, 0}}
}

// Push returns a new context clipped to the requested rect. The rect is
// specified in the root/global coordinate space; the returned context origin is
// updated so children can convert their global positions back to local by
// subtracting Origin.
func (c ScissorContext) Push(rect image.Rectangle) (ScissorContext, bool) {
	// Convert the requested rectangle into the local coordinates of the current image.
	rel := image.Rect(
		rect.Min.X-c.Origin.X,
		rect.Min.Y-c.Origin.Y,
		rect.Max.X-c.Origin.X,
		rect.Max.Y-c.Origin.Y,
	)

	clip := rel.Intersect(c.Image.Bounds())
	if clip.Empty() || clip.Dx() <= 0 || clip.Dy() <= 0 {
		return ScissorContext{}, false
	}

	sub := c.Image.SubImage(clip).(*ebiten.Image)
	return ScissorContext{
		Image:  sub,
		Origin: image.Pt(c.Origin.X+clip.Min.X, c.Origin.Y+clip.Min.Y),
	}, true
}

// LocalPoint converts a global point into the coordinate space of the context.
func (c ScissorContext) LocalPoint(x, y int) (int, int) {
	return x - c.Origin.X, y - c.Origin.Y
}

// ScissorContainer wraps a single EdgeMenuElement and clips its rendering
// region. It preserves the child's global coordinate expectations by passing
// local coordinates for drawing while forwarding mouse coordinates relative to
// its own origin.
type ScissorContainer struct {
	BaseMenuElement
	child EdgeMenuElement
	rect  image.Rectangle
}

// NewScissorContainer creates a container that clips its child.
func NewScissorContainer(child EdgeMenuElement) *ScissorContainer {
	return &ScissorContainer{BaseMenuElement: NewBaseMenuElement(), child: child}
}

// Update forwards input to the child with coordinates relative to the
// container's origin so hit-testing remains correct after clipping.
func (c *ScissorContainer) Update(mx, my int, deltaTime float64) bool {
	if !c.visible || c.child == nil {
		return false
	}

	// Convert to local coordinates; when outside, pass sentinel to avoid clicks.
	if mx >= c.rect.Min.X && mx < c.rect.Max.X && my >= c.rect.Min.Y && my < c.rect.Max.Y {
		return c.child.Update(mx-c.rect.Min.X, my-c.rect.Min.Y, deltaTime)
	}
	return c.child.Update(-1, -1, deltaTime)
}

// Draw renders the child into a clipped subimage.
func (c *ScissorContainer) Draw(screen *ebiten.Image, x, y, width int, font font.Face) int {
	if !c.visible || c.child == nil {
		return 0
	}

	childHeight := c.child.GetMinHeight()
	c.rect = image.Rect(x, y, x+width, y+childHeight)

	ctx := NewScissorContext(screen)
	clipped, ok := ctx.Push(c.rect)
	if !ok {
		return childHeight
	}

	localX, localY := clipped.LocalPoint(x, y)
	c.child.Draw(clipped.Image, localX, localY, width, font)
	return childHeight
}

// GetMinHeight returns the child's height for layout purposes.
func (c *ScissorContainer) GetMinHeight() int {
	if c.child == nil {
		return 0
	}
	return c.child.GetMinHeight()
}
