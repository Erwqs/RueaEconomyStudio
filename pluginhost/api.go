package pluginhost

import (
	"maps"
	"image/color"
	"sync"
)

const (
	hostOK             = 0
	hostErrBadArgument = 2
	hostErrUnsupported = 3
	hostErrInternal    = 4

	// Exported aliases so app-level handlers can return structured codes.
	HostOK             = hostOK
	HostErrBadArgument = hostErrBadArgument
	HostErrUnsupported = hostErrUnsupported
	HostErrInternal    = hostErrInternal
)

// toastHandler allows the app layer to surface notifications.
var toastHandler func(msg string, c color.RGBA)
var toastMu sync.RWMutex

// commandHandler lets the app layer implement simulator verbs.
// It returns a status code and optional response payload.
var commandHandler func(verb string, args map[string]any) (int, map[string]any)
var commandMu sync.RWMutex

// RegisterToastHandler is called by the app to let plugins raise toasts.
func RegisterToastHandler(fn func(msg string, c color.RGBA)) {
	toastMu.Lock()
	toastHandler = fn
	toastMu.Unlock()
}

// RegisterCommandHandler registers the app handler for host commands invoked by plugins.
func RegisterCommandHandler(fn func(verb string, args map[string]any) (int, map[string]any)) {
	commandMu.Lock()
	commandHandler = fn
	commandMu.Unlock()
}

// overlayCache stores plugin-provided overlay colors keyed by territory name.
type overlayCache struct {
	mu     sync.RWMutex
	colors map[string]color.RGBA
}

var overlays = overlayCache{colors: make(map[string]color.RGBA)}

func setOverlayColor(name string, c color.RGBA) int {
	overlays.mu.Lock()
	overlays.colors[name] = c
	overlays.mu.Unlock()
	return 0
}

// SetOverlayColor sets a territory overlay color from the app layer.
func SetOverlayColor(name string, c color.RGBA) {
	setOverlayColor(name, c)
}

func clearOverlays() int {
	overlays.mu.Lock()
	overlays.colors = make(map[string]color.RGBA)
	overlays.mu.Unlock()
	return 0
}

func clearOverlay(name string) int {
	overlays.mu.Lock()
	delete(overlays.colors, name)
	overlays.mu.Unlock()
	return 0
}

// ClearOverlays clears all overlay colors from the app layer.
func ClearOverlays() {
	clearOverlays()
}

// ClearOverlay removes a single overlay color entry.
func ClearOverlay(name string) {
	clearOverlay(name)
}

// GetOverlayColor returns the overlay color for a territory, if present.
func GetOverlayColor(name string) (color.RGBA, bool) {
	overlays.mu.RLock()
	c, ok := overlays.colors[name]
	overlays.mu.RUnlock()
	return c, ok
}

// CopyOverlayCache returns a shallow copy of the overlay map.
func CopyOverlayCache() map[string]color.RGBA {
	overlays.mu.RLock()
	out := make(map[string]color.RGBA, len(overlays.colors))
	maps.Copy(out, overlays.colors)
	overlays.mu.RUnlock()
	return out
}

func showToast(msg string, c color.RGBA) int {
	toastMu.RLock()
	fn := toastHandler
	toastMu.RUnlock()
	if fn == nil {
		return 0
	}
	fn(msg, c)
	return 0
}

func runCommand(verb string, args map[string]any) (code int, resp map[string]any) {
	commandMu.RLock()
	fn := commandHandler
	commandMu.RUnlock()
	if fn == nil {
		return hostErrUnsupported, nil
	}
	defer func() {
		if recover() != nil {
			code = hostErrInternal
			resp = nil
		}
	}()
	return fn(verb, args)
}
