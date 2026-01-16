package app

import (
	"strconv"
	"strings"

	"RueaES/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// keyFromBinding converts a canonical binding (letter, F-key, or named key) to an ebiten.Key.
func keyFromBinding(binding string) (ebiten.Key, bool) {
	canonical, ok := typedef.CanonicalizeBinding(binding)
	if !ok {
		return 0, false
	}
	if canonical == "" {
		return 0, false // disabled binding
	}

	if len(canonical) == 1 {
		ch := canonical[0]
		return ebiten.KeyA + ebiten.Key(ch-'A'), true
	}

	if strings.HasPrefix(canonical, "F") {
		n, err := strconv.Atoi(canonical[1:])
		if err == nil && n >= 1 && n <= 12 {
			return ebiten.KeyF1 + ebiten.Key(n-1), true
		}
	}

	switch canonical {
	case "SPACE":
		return ebiten.KeySpace, true
	case "ESCAPE":
		return ebiten.KeyEscape, true
	case "ENTER":
		return ebiten.KeyEnter, true
	case "TAB":
		return ebiten.KeyTab, true
	case "BACKSPACE":
		return ebiten.KeyBackspace, true
	case "DELETE":
		return ebiten.KeyDelete, true
	case "INSERT":
		return ebiten.KeyInsert, true
	case "HOME":
		return ebiten.KeyHome, true
	case "END":
		return ebiten.KeyEnd, true
	case "PAGEUP":
		return ebiten.KeyPageUp, true
	case "PAGEDOWN":
		return ebiten.KeyPageDown, true
	case "UP":
		return ebiten.KeyArrowUp, true
	case "DOWN":
		return ebiten.KeyArrowDown, true
	case "LEFT":
		return ebiten.KeyArrowLeft, true
	case "RIGHT":
		return ebiten.KeyArrowRight, true
	default:
		return 0, false
	}
}

// bindingMatches reports whether the given ebiten key equals the binding.
func bindingMatches(key ebiten.Key, binding string) bool {
	if k, ok := keyFromBinding(binding); ok {
		return key == k
	}
	return false
}

// bindingJustPressed reports whether the configured binding was just pressed this frame.
func bindingJustPressed(binding string) bool {
	if k, ok := keyFromBinding(binding); ok {
		return inpututil.IsKeyJustPressed(k)
	}
	return false
}
