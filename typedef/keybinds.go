package typedef

import (
	"strconv"
	"strings"
)

// Keybinds stores user-configurable keyboard shortcuts for major UI actions.
type Keybinds struct {
	AnalysisModal   string `json:"analysisModal,omitempty"`
	AutoSetupModal  string `json:"autoSetupModal,omitempty"`
	StateMenu       string `json:"stateMenu,omitempty"`
	TributeMenu     string `json:"tributeMenu,omitempty"`
	FilterMenu      string `json:"filterMenu,omitempty"`
	GuildManager    string `json:"guildManager,omitempty"`
	LoadoutManager  string `json:"loadoutManager,omitempty"`
	ScriptManager   string `json:"scriptManager,omitempty"`
	TerritoryToggle string `json:"territoryToggle,omitempty"`
	Coordinates     string `json:"coordinates,omitempty"`
	AddMarker       string `json:"addMarker,omitempty"`
	ClearMarkers    string `json:"clearMarkers,omitempty"`
	RouteHighlight  string `json:"routeHighlight,omitempty"`
}

// DefaultKeybinds returns the baseline key configuration.
func DefaultKeybinds() Keybinds {
	return Keybinds{
		AnalysisModal:   "A",
		AutoSetupModal:  "K",
		StateMenu:       "P",
		TributeMenu:     "B",
		FilterMenu:      "F",
		GuildManager:    "G",
		LoadoutManager:  "L",
		ScriptManager:   "S",
		TerritoryToggle: "T",
		Coordinates:     "C",
		AddMarker:       "M",
		ClearMarkers:    "X",
		RouteHighlight:  "H",
	}
}

// CanonicalizeBinding trims, uppercases, and validates supported key names.
// Allowed values: empty string (disabled), single letters A-Z, function keys F1-F12, and common names like SPACE, ESCAPE, ENTER, TAB, BACKSPACE, DELETE, INSERT, HOME, END, PAGEUP, PAGEDOWN, and arrow keys (UP/DOWN/LEFT/RIGHT).
// Returns the canonical uppercase name and true when valid.
func CanonicalizeBinding(binding string) (string, bool) {
	val := strings.TrimSpace(binding)
	if val == "" {
		return "", true // empty means unbound/disabled
	}
	upper := strings.ToUpper(val)

	// Single-letter A-Z
	if len(upper) == 1 {
		ch := upper[0]
		if ch >= 'A' && ch <= 'Z' {
			return upper, true
		}
	}

	// Function keys F1-F12
	if strings.HasPrefix(upper, "F") && len(upper) > 1 {
		if n, err := strconv.Atoi(upper[1:]); err == nil && n >= 1 && n <= 12 {
			return "F" + strconv.Itoa(n), true
		}
	}

	switch upper {
	case "SPACE", "SPACEBAR":
		return "SPACE", true
	case "ESC", "ESCAPE":
		return "ESCAPE", true
	case "ENTER", "RETURN":
		return "ENTER", true
	case "TAB":
		return "TAB", true
	case "BACKSPACE":
		return "BACKSPACE", true
	case "DELETE", "DEL":
		return "DELETE", true
	case "INSERT", "INS":
		return "INSERT", true
	case "HOME":
		return "HOME", true
	case "END":
		return "END", true
	case "PAGEUP", "PGUP":
		return "PAGEUP", true
	case "PAGEDOWN", "PGDN":
		return "PAGEDOWN", true
	case "UP", "ARROWUP":
		return "UP", true
	case "DOWN", "ARROWDOWN":
		return "DOWN", true
	case "LEFT", "ARROWLEFT":
		return "LEFT", true
	case "RIGHT", "ARROWRIGHT":
		return "RIGHT", true
	default:
		return "", false
	}
}

// NormalizeKeybinds uppercases, canonicalizes, and fills defaults when missing or invalid.
func NormalizeKeybinds(k *Keybinds) {
	if k == nil {
		return
	}
	defaults := DefaultKeybinds()
	normalize := func(target *string, fallback string) {
		if val, ok := CanonicalizeBinding(*target); ok {
			*target = val
			return
		}
		if val, ok := CanonicalizeBinding(fallback); ok {
			*target = val
		} else {
			*target = fallback
		}
	}

	normalize(&k.AnalysisModal, defaults.AnalysisModal)
	normalize(&k.AutoSetupModal, defaults.AutoSetupModal)
	normalize(&k.StateMenu, defaults.StateMenu)
	normalize(&k.TributeMenu, defaults.TributeMenu)
	normalize(&k.FilterMenu, defaults.FilterMenu)
	normalize(&k.GuildManager, defaults.GuildManager)
	normalize(&k.LoadoutManager, defaults.LoadoutManager)
	normalize(&k.ScriptManager, defaults.ScriptManager)
	normalize(&k.TerritoryToggle, defaults.TerritoryToggle)
	normalize(&k.Coordinates, defaults.Coordinates)
	normalize(&k.AddMarker, defaults.AddMarker)
	normalize(&k.ClearMarkers, defaults.ClearMarkers)
	normalize(&k.RouteHighlight, defaults.RouteHighlight)
}

// NormalizePluginKeybinds canonicalizes plugin-provided keybind overrides in-place.
// Unknown keys are left untouched by design; values are forced to canonical form or cleared when invalid.
func NormalizePluginKeybinds(m *map[string]string) {
	if m == nil {
		return
	}
	if *m == nil {
		*m = make(map[string]string)
		return
	}
	for key, val := range *m {
		if normalized, ok := CanonicalizeBinding(val); ok {
			(*m)[key] = normalized
			continue
		}
		(*m)[key] = ""
	}
}
