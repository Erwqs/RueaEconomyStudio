package app

import (
	"fmt"
	"sort"
	"strings"

	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const territorySelectorOverlayHeight = 140

// TerritorySelectorOverlay reuses the loadout-apply style full-screen territory picker
// (click territories, ctrl-drag for area select, accept/cancel banner).
type TerritorySelectorOverlay struct {
	Title            string
	multi            bool
	visible          bool
	initialPreselect []string
	selections       map[string]bool
	OnAccept         func([]string)
	OnCancel         func()
}

var activeTerritorySelector *TerritorySelectorOverlay

// NewTerritorySelectorOverlay builds the overlay with optional preselected territories.
func NewTerritorySelectorOverlay(title string, multi bool, preselect []string) *TerritorySelectorOverlay {
	sel := &TerritorySelectorOverlay{
		Title:            strings.TrimSpace(title),
		multi:            multi,
		visible:          false,
		initialPreselect: make([]string, 0, len(preselect)),
		selections:       make(map[string]bool),
	}
	for _, name := range preselect {
		clean := strings.TrimSpace(name)
		if clean == "" {
			continue
		}
		sel.initialPreselect = append(sel.initialPreselect, clean)
	}
	return sel
}

// resetToInitialPreselect clears user selections and reapplies the original preselect set by the opener.
func (t *TerritorySelectorOverlay) resetToInitialPreselect() {
	t.selections = make(map[string]bool)
	for _, name := range t.initialPreselect {
		if name == "" {
			continue
		}
		t.selections[name] = true
	}
}

// GetActiveTerritorySelector returns the currently active overlay (if any).
func GetActiveTerritorySelector() *TerritorySelectorOverlay {
	return activeTerritorySelector
}

// Show activates the overlay and syncs highlights on the map.
func (t *TerritorySelectorOverlay) Show() {
	if t == nil {
		return
	}

	// Reset selections to the provided preselects each time we show the overlay.
	t.resetToInitialPreselect()

	t.visible = true
	activeTerritorySelector = t
	t.syncRenderer()
}

// Hide deactivates the overlay and clears highlights without firing callbacks.
func (t *TerritorySelectorOverlay) Hide() {
	t.visible = false
	if activeTerritorySelector == t {
		activeTerritorySelector = nil
	}
	t.clearRenderer()
	t.resetToInitialPreselect()
}

// IsVisible reports whether the overlay is active.
func (t *TerritorySelectorOverlay) IsVisible() bool {
	return t != nil && t.visible
}

// ToggleTerritorySelection flips selection state for a territory.
func (t *TerritorySelectorOverlay) ToggleTerritorySelection(name string) {
	if t == nil || !t.visible || name == "" {
		return
	}
	if !t.multi {
		t.selections = map[string]bool{name: true}
	} else {
		if t.selections[name] {
			delete(t.selections, name)
		} else {
			t.selections[name] = true
		}
	}
	t.syncRenderer()
}

// AddTerritorySelection marks a territory selected.
func (t *TerritorySelectorOverlay) AddTerritorySelection(name string) {
	if t == nil || !t.visible || name == "" {
		return
	}
	if !t.multi {
		t.selections = map[string]bool{name: true}
	} else {
		t.selections[name] = true
	}
	t.syncRenderer()
}

// RemoveTerritorySelection clears a territory from selection.
func (t *TerritorySelectorOverlay) RemoveTerritorySelection(name string) {
	if t == nil || !t.visible || name == "" {
		return
	}
	delete(t.selections, name)
	t.syncRenderer()
}

// GetSelectedTerritories returns a copy of the current selections.
func (t *TerritorySelectorOverlay) GetSelectedTerritories() map[string]bool {
	out := make(map[string]bool, len(t.selections))
	for k, v := range t.selections {
		if v {
			out[k] = true
		}
	}
	return out
}

// SelectedSlice returns the selected territory names (sorted for determinism).
func (t *TerritorySelectorOverlay) SelectedSlice() []string {
	names := make([]string, 0, len(t.selections))
	for name, ok := range t.selections {
		if ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// IsBannerArea reports whether a click hits the top banner area.
func (t *TerritorySelectorOverlay) IsBannerArea(x, y int) bool {
	return t != nil && t.visible && y <= territorySelectorOverlayHeight
}

// buttonRects returns the apply/cancel button rectangles.
func (t *TerritorySelectorOverlay) buttonRects() (apply Rect, cancel Rect) {
	screenW, _ := ebiten.WindowSize()
	buttonWidth := 70
	buttonHeight := 30
	buttonSpacing := 10
	applyX := screenW - (buttonWidth*2 + buttonSpacing + 20)
	applyY := territorySelectorOverlayHeight - 50
	cancelX := screenW - (buttonWidth + 20)
	cancelY := territorySelectorOverlayHeight - 50
	apply = Rect{X: applyX, Y: applyY, Width: buttonWidth, Height: buttonHeight}
	cancel = Rect{X: cancelX, Y: cancelY, Width: buttonWidth, Height: buttonHeight}
	return
}

// Update processes keyboard/mouse for the overlay; returns true if it consumed input.
func (t *TerritorySelectorOverlay) Update() bool {
	if t == nil || !t.visible {
		return false
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		t.finishCancel()
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		t.finishAccept()
		return true
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		applyRect, cancelRect := t.buttonRects()
		if applyRect.Contains(mx, my) {
			t.finishAccept()
			return true
		}
		if cancelRect.Contains(mx, my) {
			t.finishCancel()
			return true
		}
		if t.IsBannerArea(mx, my) {
			return true // Block map clicks through the banner
		}
	}

	return false
}

// Draw renders the overlay banner and buttons.
func (t *TerritorySelectorOverlay) Draw(screen *ebiten.Image) {
	if t == nil || !t.visible {
		return
	}

	screenW, _ := ebiten.WindowSize()
	overlayHeight := territorySelectorOverlayHeight
	overlayColor := color.RGBA{40, 40, 60, 200}
	vector.DrawFilledRect(screen, 0, 0, float32(screenW), float32(overlayHeight), overlayColor, false)
	vector.DrawFilledRect(screen, 0, float32(overlayHeight-3), float32(screenW), 3, color.RGBA{100, 150, 255, 255}, false)

	titleFont := loadWynncraftFont(24)
	subtitleFont := loadWynncraftFont(18)
	textFont := loadWynncraftFont(14)

	title := t.Title
	if title == "" {
		title = "Select Territories"
	}
	titleBounds := text.BoundString(titleFont, "Territory Selection")
	titleX := (screenW - titleBounds.Dx()) / 2
	text.Draw(screen, "Territory Selection", titleFont, titleX, 35, color.RGBA{255, 255, 255, 255})

	subtitleText := title
	subtitleBounds := text.BoundString(subtitleFont, subtitleText)
	subtitleX := (screenW - subtitleBounds.Dx()) / 2
	text.Draw(screen, subtitleText, subtitleFont, subtitleX, 60, color.RGBA{200, 220, 255, 255})

	instruction := "Click territories to select/deselect. Ctrl-drag to area select. Enter to accept, Esc to cancel."
	instructionBounds := text.BoundString(textFont, instruction)
	instructionX := (screenW - instructionBounds.Dx()) / 2
	text.Draw(screen, instruction, textFont, instructionX, 85, color.RGBA{180, 200, 240, 255})

	selectedCount := 0
	for _, v := range t.selections {
		if v {
			selectedCount++
		}
	}
	countText := fmt.Sprintf("Selected: %d territories", selectedCount)
	countBounds := text.BoundString(textFont, countText)
	countX := (screenW - countBounds.Dx()) / 2
	text.Draw(screen, countText, textFont, countX, 105, color.RGBA{150, 255, 150, 255})

	applyRect, cancelRect := t.buttonRects()
	mx, my := ebiten.CursorPosition()

	applyColor := color.RGBA{50, 150, 50, 255}
	if applyRect.Contains(mx, my) {
		applyColor = color.RGBA{70, 200, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(applyRect.X), float32(applyRect.Y), float32(applyRect.Width), float32(applyRect.Height), applyColor, false)
	vector.StrokeRect(screen, float32(applyRect.X), float32(applyRect.Y), float32(applyRect.Width), float32(applyRect.Height), 2, color.RGBA{100, 100, 120, 255}, false)
	buttonFont := loadWynncraftFont(14)
	applyText := "Accept"
	applyBounds := text.BoundString(buttonFont, applyText)
	text.Draw(screen, applyText, buttonFont,
		applyRect.X+(applyRect.Width-applyBounds.Dx())/2,
		applyRect.Y+(applyRect.Height+applyBounds.Dy())/2-2,
		color.RGBA{255, 255, 255, 255})

	cancelColor := color.RGBA{150, 50, 50, 255}
	if cancelRect.Contains(mx, my) {
		cancelColor = color.RGBA{200, 70, 70, 255}
	}
	vector.DrawFilledRect(screen, float32(cancelRect.X), float32(cancelRect.Y), float32(cancelRect.Width), float32(cancelRect.Height), cancelColor, false)
	vector.StrokeRect(screen, float32(cancelRect.X), float32(cancelRect.Y), float32(cancelRect.Width), float32(cancelRect.Height), 2, color.RGBA{100, 100, 120, 255}, false)
	cancelText := "Cancel"
	cancelBounds := text.BoundString(buttonFont, cancelText)
	text.Draw(screen, cancelText, buttonFont,
		cancelRect.X+(cancelRect.Width-cancelBounds.Dx())/2,
		cancelRect.Y+(cancelRect.Height+cancelBounds.Dy())/2-2,
		color.RGBA{255, 255, 255, 255})
}

func (t *TerritorySelectorOverlay) syncRenderer() {
	if t == nil || !t.visible {
		return
	}
	if mv := GetMapView(); mv != nil && mv.territoriesManager != nil {
		if renderer := mv.territoriesManager.GetRenderer(); renderer != nil {
			renderer.SetLoadoutApplicationMode(t.Title, t.selections)
			if cache := renderer.GetTerritoryCache(); cache != nil {
				cache.ForceRedraw()
			}
		}
	}
}

func (t *TerritorySelectorOverlay) clearRenderer() {
	if mv := GetMapView(); mv != nil && mv.territoriesManager != nil {
		if renderer := mv.territoriesManager.GetRenderer(); renderer != nil {
			renderer.ClearLoadoutApplicationMode()
			if cache := renderer.GetTerritoryCache(); cache != nil {
				cache.ForceRedraw()
			}
		}
	}
}

func (t *TerritorySelectorOverlay) finishAccept() {
	if !t.visible {
		return
	}
	selected := t.SelectedSlice()
	t.visible = false
	if activeTerritorySelector == t {
		activeTerritorySelector = nil
	}
	t.clearRenderer()
	// After closing, revert selections back to the original preselect only.
	t.resetToInitialPreselect()
	if t.OnAccept != nil {
		t.OnAccept(selected)
	}
}

func (t *TerritorySelectorOverlay) finishCancel() {
	if !t.visible {
		return
	}
	t.visible = false
	if activeTerritorySelector == t {
		activeTerritorySelector = nil
	}
	t.clearRenderer()
	// After closing, revert selections back to the original preselect only.
	t.resetToInitialPreselect()
	if t.OnCancel != nil {
		t.OnCancel()
	}
}
