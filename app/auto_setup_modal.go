package app

import (
	"image"
	"sort"
	"strings"

	"RueaES/alg/auto"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	autoSetupAlgoLabel = "AutoSetup"
)

// AutoSetupModal shows controls for the auto economy optimizer.
type AutoSetupModal struct {
	modal *EnhancedModal
	cards []*Card

	closeButton *EnhancedButton
	statusText  *MenuText

	suppressInputFrames int

	guildDropdown *FilterableDropdown
	algDropdown   *FilterableDropdown

	guildOptions []FilterableDropdownOption
	algOptions   []FilterableDropdownOption

	guildValue string
	algValue   string
}

// NewAutoSetupModal creates the modal UI.
func NewAutoSetupModal() *AutoSetupModal {
	modal := NewEnhancedModal("Auto Setup", 720, 520)
	am := &AutoSetupModal{modal: modal}
	am.buildUI()
	return am
}

// SetGuildOptions updates available guild choices.
func (am *AutoSetupModal) SetGuildOptions(options []FilterableDropdownOption) {
	sorted := make([]FilterableDropdownOption, len(options))
	copy(sorted, options)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Display < sorted[j].Display
	})
	am.guildOptions = sorted
	if am.guildDropdown != nil {
		am.guildDropdown.SetOptions(sorted)
		if am.guildValue != "" {
			am.guildDropdown.SetSelected(am.guildValue)
		}
		if am.guildValue == "" && len(sorted) > 0 {
			am.guildDropdown.SetSelected(sorted[0].Value)
			am.guildValue = sorted[0].Value
		}
		if len(sorted) == 0 {
			am.guildDropdown.ClearSelection()
			am.guildValue = ""
		}
	}
}

// Toggle shows/hides the modal.
func (am *AutoSetupModal) Toggle() {
	if am.modal == nil {
		return
	}
	if am.modal.IsVisible() {
		am.Hide()
		return
	}
	am.Show()
}

// Show makes the modal visible.
func (am *AutoSetupModal) Show() {
	if am.modal != nil {
		am.modal.Show()
	}
	am.suppressInputFrames = 1
	if am.guildDropdown != nil {
		am.guildDropdown.InputFocused = true
		am.guildDropdown.IsOpen = false
	}
}

// Hide hides the modal.
func (am *AutoSetupModal) Hide() {
	if am.modal != nil {
		am.modal.Hide()
	}
	if am.guildDropdown != nil {
		am.guildDropdown.InputFocused = false
		am.guildDropdown.IsOpen = false
	}
}

// IsVisible reports whether the modal is open.
func (am *AutoSetupModal) IsVisible() bool {
	return am.modal != nil && am.modal.IsVisible()
}

// Update handles modal input.
func (am *AutoSetupModal) Update(mx, my int, deltaTime float64) {
	if !am.IsVisible() {
		return
	}

	am.modal.Update()
	if am.closeButton != nil {
		bounds := am.modal.GetBounds()
		am.closeButton.SetPosition(bounds.Max.X-am.closeButton.Width-14, bounds.Min.Y+6)
		am.closeButton.Update(mx, my)
	}

	if am.suppressInputFrames > 0 {
		am.suppressInputFrames--
		return
	}

	cardRects := am.layoutCards()
	for i, card := range am.cards {
		if card == nil || !card.IsVisible() || i >= len(cardRects) {
			continue
		}
		rect := cardRects[i]
		card.rect = image.Rect(rect.X, rect.Y, rect.X+rect.Width, rect.Y+rect.Height)
		card.Update(mx, my, deltaTime)
	}

	if am.statusText != nil {
		if auto.IsAutoRunning() {
			am.statusText.SetText("Status: Running")
		} else {
			am.statusText.SetText("Status: Idle")
		}
	}
}

// Draw renders the modal.
func (am *AutoSetupModal) Draw(screen *ebiten.Image, _ int, _ int) {
	if !am.IsVisible() {
		return
	}

	am.modal.Draw(screen)
	fontFace := loadWynncraftFont(16)
	cardRects := am.layoutCards()
	for i, card := range am.cards {
		if card == nil || !card.IsVisible() || i >= len(cardRects) {
			continue
		}
		rect := cardRects[i]
		card.Draw(screen, rect.X, rect.Y, rect.Width, fontFace)
	}

	if am.closeButton != nil {
		am.closeButton.Draw(screen)
	}

	if am.guildDropdown != nil && am.guildDropdown.IsOpen {
		am.guildDropdown.Draw(screen)
	}
	if am.algDropdown != nil && am.algDropdown.IsOpen {
		am.algDropdown.Draw(screen)
	}
}

// HasTextInputFocused reports whether any dropdown is active.
func (am *AutoSetupModal) HasTextInputFocused() bool {
	if !am.IsVisible() {
		return false
	}
	if am.guildDropdown != nil && (am.guildDropdown.InputFocused || am.guildDropdown.IsOpen) {
		return true
	}
	if am.algDropdown != nil && (am.algDropdown.InputFocused || am.algDropdown.IsOpen) {
		return true
	}
	return false
}

func (am *AutoSetupModal) buildUI() {
	am.closeButton = NewEnhancedButton("x", 0, 0, 28, 28, func() {
		am.Hide()
	})
	am.closeButton.SetRedButtonStyle()
	am.cards = []*Card{am.buildControlCard()}
}

func (am *AutoSetupModal) buildControlCard() *Card {
	card := NewCard().Inline(false)
	card.SetBackgroundColor(EnhancedUIColors.Surface)
	card.SetBorderColor(EnhancedUIColors.BorderActive)

	title := NewMenuText("Auto Economy Setup", TextOptions{FontSize: 18, Height: 26, Color: EnhancedUIColors.Text})
	intro := NewMenuText("Continuously optimizes a guild claim. Default loop: every 60 ticks.", TextOptions{FontSize: 14, Height: 20, Color: EnhancedUIColors.TextSecondary})

	am.algOptions = []FilterableDropdownOption{{Display: autoSetupAlgoLabel, Value: "auto_setup"}}
	am.algDropdown = NewFilterableDropdown(0, 0, 240, 32, am.algOptions, func(option FilterableDropdownOption) {
		am.algValue = strings.TrimSpace(option.Value)
	})
	am.algDropdown.SetSelected("auto_setup")
	am.algValue = "auto_setup"
	algDropdownElement := newDropdownEdgeElement(am.algDropdown, am.modal, 36)

	am.guildDropdown = NewFilterableDropdown(0, 0, 260, 32, am.guildOptions, func(option FilterableDropdownOption) {
		am.guildValue = strings.TrimSpace(option.Value)
	})
	if len(am.guildOptions) > 0 {
		am.guildDropdown.SetSelected(am.guildOptions[0].Value)
		am.guildValue = am.guildOptions[0].Value
	}
	guildDropdownElement := newDropdownEdgeElement(am.guildDropdown, am.modal, 36)

	btnOpts := DefaultButtonOptions()
	btnOpts.Height = 32
	btnOpts.FontSize = 15
	btnOpts.BackgroundColor = EnhancedUIColors.Button
	btnOpts.HoverColor = EnhancedUIColors.BorderActive
	btnOpts.PressedColor = EnhancedUIColors.ButtonActive
	btnOpts.BorderColor = EnhancedUIColors.BorderGreen

	startBtn := NewMenuButton("Start", btnOpts, func() {
		if am.algValue != "auto_setup" {
			return
		}
		tag := strings.TrimSpace(am.guildValue)
		if tag == "" {
			if am.statusText != nil {
				am.statusText.SetText("Status: Select a guild")
			}
			return
		}
		cfg := auto.AutoConfig{GuildTag: tag}
		loop := auto.LoopConfig{Mode: auto.LoopEveryTicks, EveryTicks: 60}
		auto.StartAuto(cfg, loop)
	})

	pauseBtn := NewMenuButton("Pause", btnOpts, func() {
		auto.StopAuto()
	})

	resumeBtn := NewMenuButton("Resume", btnOpts, func() {
		_ = auto.ResumeAuto()
	})

	stopBtn := NewMenuButton("Stop", btnOpts, func() {
		auto.StopAuto()
	})

	restartBtn := NewMenuButton("Restart", btnOpts, func() {
		_ = auto.RestartAuto()
	})

	am.statusText = NewMenuText("Status: Idle", TextOptions{FontSize: 14, Height: 20, Color: EnhancedUIColors.TextSecondary})
	hint := NewMenuText("Tip: Use Restart to clear all upgrades before re-optimizing.", TextOptions{FontSize: 12, Height: 18, Color: EnhancedUIColors.TextSecondary})

	card.elements = append(card.elements, title, intro, algDropdownElement, guildDropdownElement, startBtn, pauseBtn, resumeBtn, stopBtn, restartBtn, am.statusText, hint)
	return card
}

func (am *AutoSetupModal) layoutCards() []Rect {
	if am.modal == nil {
		return nil
	}

	cx, cy, cw, ch := am.modal.GetContentArea()
	contentBounds := image.Rect(cx, cy, cx+cw, cy+ch)

	padding := 12
	cx += padding
	cw -= padding * 2
	currentY := cy + padding
	spacing := 16
	rects := make([]Rect, len(am.cards))

	for i, card := range am.cards {
		if card == nil || !card.IsVisible() {
			rects[i] = Rect{}
			continue
		}
		height := card.GetMinHeight()
		rects[i] = Rect{X: cx, Y: currentY, Width: cw, Height: height}
		currentY += height + spacing
	}

	if am.guildDropdown != nil {
		am.guildDropdown.SetContainerBounds(contentBounds)
	}
	if am.algDropdown != nil {
		am.algDropdown.SetContainerBounds(contentBounds)
	}

	return rects
}

func (am *AutoSetupModal) SetStatus(msg string) {
	if am.statusText != nil {
		am.statusText.SetText(msg)
	}
}

func (am *AutoSetupModal) SetAlgorithmOptions(options []FilterableDropdownOption) {
	sorted := make([]FilterableDropdownOption, len(options))
	copy(sorted, options)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Display < sorted[j].Display
	})
	am.algOptions = sorted
	if am.algDropdown != nil {
		am.algDropdown.SetOptions(sorted)
		if am.algValue != "" {
			am.algDropdown.SetSelected(am.algValue)
		}
		if am.algValue == "" && len(sorted) > 0 {
			am.algDropdown.SetSelected(sorted[0].Value)
			am.algValue = sorted[0].Value
		}
	}
}
