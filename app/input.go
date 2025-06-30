package app

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// KeyEvent represents a key press event
type KeyEvent struct {
	Key     ebiten.Key
	Pressed bool // true for press, false for release
}

// MouseButtonEvent represents a mouse button event
type MouseButtonEvent struct {
	Button  ebiten.MouseButton
	Pressed bool // true for press, false for release
}

// InputManager handles all input events and distributes them via channels
type InputManager struct {
	keyEventCh         chan KeyEvent
	mouseButtonEventCh chan MouseButtonEvent
	subscribers        []chan KeyEvent
	mouseSubscribers   []chan MouseButtonEvent
	subscribersMu      sync.RWMutex
}

// NewInputManager creates a new InputManager
func NewInputManager() *InputManager {
	return &InputManager{
		keyEventCh:         make(chan KeyEvent, 100),         // Buffer to prevent blocking
		mouseButtonEventCh: make(chan MouseButtonEvent, 100), // Buffer for mouse events
		subscribers:        make([]chan KeyEvent, 0),
		mouseSubscribers:   make([]chan MouseButtonEvent, 0),
	}
}

// Subscribe returns a channel that will receive key events
// The caller is responsible for reading from this channel to prevent blocking
func (im *InputManager) Subscribe() <-chan KeyEvent {
	im.subscribersMu.Lock()
	defer im.subscribersMu.Unlock()

	ch := make(chan KeyEvent, 50) // Buffered channel for each subscriber
	im.subscribers = append(im.subscribers, ch)
	return ch
}

// SubscribeMouseEvents returns a channel that will receive mouse button events
// The caller is responsible for reading from this channel to prevent blocking
func (im *InputManager) SubscribeMouseEvents() <-chan MouseButtonEvent {
	im.subscribersMu.Lock()
	defer im.subscribersMu.Unlock()

	ch := make(chan MouseButtonEvent, 50) // Buffered channel for each subscriber
	im.mouseSubscribers = append(im.mouseSubscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel (optional cleanup)
func (im *InputManager) Unsubscribe(ch <-chan KeyEvent) {
	im.subscribersMu.Lock()
	defer im.subscribersMu.Unlock()

	for i, sub := range im.subscribers {
		if sub == ch {
			close(sub)
			im.subscribers = append(im.subscribers[:i], im.subscribers[i+1:]...)
			break
		}
	}
}

// UnsubscribeMouseEvents removes a mouse subscriber channel (optional cleanup)
func (im *InputManager) UnsubscribeMouseEvents(ch <-chan MouseButtonEvent) {
	im.subscribersMu.Lock()
	defer im.subscribersMu.Unlock()

	for i, sub := range im.mouseSubscribers {
		if sub == ch {
			close(sub)
			im.mouseSubscribers = append(im.mouseSubscribers[:i], im.mouseSubscribers[i+1:]...)
			break
		}
	}
}

// Update should be called every frame to check for input changes
func (im *InputManager) Update() {
	// Use efficient inpututil functions to get only changed keys
	// This is much more efficient than checking all possible keys every frame

	// Get just pressed keys
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		event := KeyEvent{Key: key, Pressed: true}
		im.broadcastEvent(event)
	}

	// Get just released keys
	for _, key := range inpututil.AppendJustReleasedKeys(nil) {
		event := KeyEvent{Key: key, Pressed: false}
		im.broadcastEvent(event)
	}

	// Check for mouse button events
	// Mouse buttons 3 and 4 are typically the "back" and "forward" buttons on many mice
	for btn := ebiten.MouseButton0; btn <= ebiten.MouseButton4; btn++ {
		// Check for just pressed mouse buttons
		if inpututil.IsMouseButtonJustPressed(btn) {
			event := MouseButtonEvent{Button: btn, Pressed: true}
			im.broadcastMouseEvent(event)

			// For the back button (MouseButton3), also simulate an Escape key press
			// but only if we're not in claim editing mode
			if btn == ebiten.MouseButton3 {
				// Check if we're in claim editing mode
				skipEsc := false
				if app := GetCurrentApp(); app != nil {
					if gameplayModule := app.GetGameplayModule(); gameplayModule != nil {
						if mapView := gameplayModule.GetMapView(); mapView != nil {
							if mapView.IsEditingClaims() {
								skipEsc = true
							}
						}
					}
				}

				// Only simulate ESC if we're not in claim editing mode
				if !skipEsc {
					escEvent := KeyEvent{Key: ebiten.KeyEscape, Pressed: true}
					im.broadcastEvent(escEvent)
					// Immediately follow with a release event
					escReleaseEvent := KeyEvent{Key: ebiten.KeyEscape, Pressed: false}
					im.broadcastEvent(escReleaseEvent)
				}
			}
		}

		// Check for just released mouse buttons
		if inpututil.IsMouseButtonJustReleased(btn) {
			event := MouseButtonEvent{Button: btn, Pressed: false}
			im.broadcastMouseEvent(event)
		}
	}
}

// broadcastEvent sends the event to all subscribers
func (im *InputManager) broadcastEvent(event KeyEvent) {
	im.subscribersMu.RLock()
	defer im.subscribersMu.RUnlock()

	for _, subscriber := range im.subscribers {
		select {
		case subscriber <- event:
			// Event sent successfully
		default:
			// Channel is full, skip this subscriber to prevent blocking
		}
	}
}

// broadcastMouseEvent sends the mouse event to all mouse subscribers
func (im *InputManager) broadcastMouseEvent(event MouseButtonEvent) {
	im.subscribersMu.RLock()
	defer im.subscribersMu.RUnlock()

	for _, subscriber := range im.mouseSubscribers {
		select {
		case subscriber <- event:
			// Event sent successfully
		default:
			// Channel is full, skip this subscriber to prevent blocking
		}
	}
}

// GetKeyName returns a human-readable name for the key
func GetKeyName(key ebiten.Key) string {
	keyNames := map[ebiten.Key]string{
		ebiten.KeyEscape:     "Escape",
		ebiten.KeySpace:      "Space",
		ebiten.KeyEnter:      "Enter",
		ebiten.KeyTab:        "Tab",
		ebiten.KeyShift:      "Shift",
		ebiten.KeyControl:    "Control",
		ebiten.KeyAlt:        "Alt",
		ebiten.KeyArrowUp:    "Arrow Up",
		ebiten.KeyArrowDown:  "Arrow Down",
		ebiten.KeyArrowLeft:  "Arrow Left",
		ebiten.KeyArrowRight: "Arrow Right",
		ebiten.KeyBackspace:  "Backspace",
		ebiten.KeyDelete:     "Delete",
	}

	if name, exists := keyNames[key]; exists {
		return name
	}

	// For letters and numbers, convert to string
	if key >= ebiten.KeyA && key <= ebiten.KeyZ {
		return string(rune('A' + int(key-ebiten.KeyA)))
	}

	if key >= ebiten.Key0 && key <= ebiten.Key9 {
		return string(rune('0' + int(key-ebiten.Key0)))
	}

	if key >= ebiten.KeyF1 && key <= ebiten.KeyF12 {
		return "F" + string(rune('1'+int(key-ebiten.KeyF1)))
	}

	return "Unknown"
}
