package app

import (
	"image"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// PointerEventType enumerates generic pointer actions.
type PointerEventType int

const (
	PointerDown PointerEventType = iota
	PointerUp
	PointerMove
	PointerLongPress // maps to right-click semantics
	PointerPinchZoom // scale > 1 zoom in, < 1 zoom out
)

// PointerEvent represents a unified mouse/touch action.
type PointerEvent struct {
	Type      PointerEventType
	ID        ebiten.TouchID
	Position  image.Point
	Delta     image.Point
	Scale     float64 // for pinch
	IsPrimary bool
	IsMouse   bool
	Time      time.Time
}

type touchState struct {
	start         image.Point
	last          image.Point
	startTime     time.Time
	longPressSent bool
}

type pinchState struct {
	id1, id2 ebiten.TouchID
	lastDist float64
}

// PointerInput normalizes mouse and touch input into pointer events.
type PointerInput struct {
	events           []PointerEvent
	touches          map[ebiten.TouchID]*touchState
	mouseDown        bool
	mouseStart       image.Point
	mouseLast        image.Point
	mouseStartTime   time.Time
	pinch            pinchState
	longPressAfter   time.Duration
	moveThresholdSq  int
	pinchThresholdSq int
}

// NewPointerInput builds a pointer input helper with sensible defaults.
func NewPointerInput() *PointerInput {
	return &PointerInput{
		touches:          make(map[ebiten.TouchID]*touchState),
		longPressAfter:   500 * time.Millisecond,
		moveThresholdSq:  64, // 8px
		pinchThresholdSq: 36, // 6px
	}
}

// Events returns the collected pointer events for the last frame.
func (p *PointerInput) Events() []PointerEvent { return p.events }

// Update polls ebiten input APIs and emits normalized pointer events.
func (p *PointerInput) Update() {
	now := time.Now()
	p.events = p.events[:0]

	// Skip capturing pointer input when the window is unfocused.
	if !ebiten.IsFocused() {
		p.resetState()
		return
	}

	// Mouse as a pointer
	mx, my := ebiten.CursorPosition()
	mousePos := image.Pt(mx, my)
	mouseDown := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	if mouseDown && !p.mouseDown {
		p.mouseDown = true
		p.mouseStart = mousePos
		p.mouseLast = mousePos
		p.mouseStartTime = now
		p.events = append(p.events, PointerEvent{Type: PointerDown, Position: mousePos, IsPrimary: true, IsMouse: true, Time: now})
	}

	if p.mouseDown && mouseDown {
		if mousePos != p.mouseLast {
			delta := mousePos.Sub(p.mouseLast)
			p.events = append(p.events, PointerEvent{Type: PointerMove, Position: mousePos, Delta: delta, IsPrimary: true, IsMouse: true, Time: now})
		}
		p.mouseLast = mousePos
	} else if p.mouseDown && !mouseDown {
		p.events = append(p.events, PointerEvent{Type: PointerUp, Position: mousePos, IsPrimary: true, IsMouse: true, Time: now})
		p.mouseDown = false
	}

	// Touch pointers
	active := make(map[ebiten.TouchID]bool)
	touchIDs := ebiten.TouchIDs()
	for _, id := range touchIDs {
		active[id] = true
		tx, ty := ebiten.TouchPosition(id)
		pos := image.Pt(tx, ty)
		st, ok := p.touches[id]
		if !ok {
			st = &touchState{start: pos, last: pos, startTime: now}
			p.touches[id] = st
			p.events = append(p.events, PointerEvent{Type: PointerDown, ID: id, Position: pos, IsPrimary: len(p.touches) == 1, Time: now})
		} else {
			if pos != st.last {
				delta := pos.Sub(st.last)
				p.events = append(p.events, PointerEvent{Type: PointerMove, ID: id, Position: pos, Delta: delta, IsPrimary: false, Time: now})
				st.last = pos
			}
			if !st.longPressSent && now.Sub(st.startTime) >= p.longPressAfter && distSq(st.start, pos) <= p.moveThresholdSq {
				st.longPressSent = true
				p.events = append(p.events, PointerEvent{Type: PointerLongPress, ID: id, Position: pos, IsPrimary: false, Time: now})
			}
		}
	}

	// Touch releases
	for id, st := range p.touches {
		if !active[id] {
			p.events = append(p.events, PointerEvent{Type: PointerUp, ID: id, Position: st.last, IsPrimary: false, Time: now})
			delete(p.touches, id)
		}
	}

	// Pinch zoom (use the first two touches)
	if len(touchIDs) >= 2 {
		id1, id2 := touchIDs[0], touchIDs[1]
		x1, y1 := ebiten.TouchPosition(id1)
		x2, y2 := ebiten.TouchPosition(id2)
		dx, dy := float64(x2-x1), float64(y2-y1)
		dist := math.Hypot(dx, dy)

		if p.pinch.id1 != id1 || p.pinch.id2 != id2 {
			p.pinch = pinchState{id1: id1, id2: id2, lastDist: dist}
		} else if dist > 0 && p.pinch.lastDist > 0 && math.Abs(dist-p.pinch.lastDist) > 0.5 {
			scale := dist / p.pinch.lastDist
			mid := image.Pt((x1+x2)/2, (y1+y2)/2)
			p.events = append(p.events, PointerEvent{Type: PointerPinchZoom, ID: id1, Position: mid, Scale: scale, Time: now})
			p.pinch.lastDist = dist
		} else {
			p.pinch.lastDist = dist
		}
	} else {
		p.pinch = pinchState{}
	}
}

// Reset clears all pointer state and outstanding events.
func (p *PointerInput) Reset() {
	p.resetState()
}

// resetState is the internal helper that clears pointer state without exporting implementation details.
func (p *PointerInput) resetState() {
	p.events = p.events[:0]
	p.mouseDown = false
	p.mouseStart = image.Point{}
	p.mouseLast = image.Point{}
	p.mouseStartTime = time.Time{}
	p.pinch = pinchState{}

	for id := range p.touches {
		delete(p.touches, id)
	}
}

func distSq(a, b image.Point) int {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}
