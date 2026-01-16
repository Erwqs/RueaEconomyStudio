package app

import (
	"image"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
)

const (
	chokepointButtonLabel = "Analyse Chokepoints"
	hqButtonLabel         = "Find HQ Spots"
)

// AnalysisModal displays analysis tools using reusable menu components.
type AnalysisModal struct {
	modal      *EnhancedModal
	cards      []*Card
	statusText *MenuText
	// suppressInputFrames avoids capturing the toggle key on open
	suppressInputFrames int
	closeButton         *EnhancedButton

	guildDropdown   *FilterableDropdown
	guildOptions    []FilterableDropdownOption
	analyseButton   *MenuButton
	hqAnalyseButton *MenuButton
	analyzing       bool

	// Chokepoint module state
	onAnalyze   func(string)
	onHQAnalyze func(string)
	guildValue  string
}

// NewAnalysisModal creates a new analysis modal.
func NewAnalysisModal() *AnalysisModal {
	modal := NewEnhancedModal("Analysis", 760, 520)
	am := &AnalysisModal{modal: modal}
	am.buildUI()
	return am
}

// SetOnAnalyze registers the callback invoked when the Analyse button is pressed.
func (am *AnalysisModal) SetOnAnalyze(cb func(string)) {
	am.onAnalyze = cb
}

// SetOnHQAnalyze registers the callback invoked when the HQ analysis button is pressed.
func (am *AnalysisModal) SetOnHQAnalyze(cb func(string)) {
	am.onHQAnalyze = cb
}

// SetStatus updates the status text shown beneath the controls.
func (am *AnalysisModal) SetStatus(msg string) {
	if am.statusText != nil {
		am.statusText.SetText(msg)
	}
}

// SetAnalyzing toggles the UI state while analysis is running.
func (am *AnalysisModal) SetAnalyzing(inProgress bool) {
	am.analyzing = inProgress
	if am.analyseButton != nil {
		am.analyseButton.SetEnabled(!inProgress)
		if inProgress {
			am.analyseButton.SetText("Analysing...")
		} else {
			am.analyseButton.SetText(chokepointButtonLabel)
		}
	}
	if am.hqAnalyseButton != nil {
		am.hqAnalyseButton.SetEnabled(!inProgress)
		if inProgress {
			am.hqAnalyseButton.SetText("Analysing...")
		} else {
			am.hqAnalyseButton.SetText(hqButtonLabel)
		}
	}
}

// SetGuildOptions updates the dropdown choices for guild selection.
func (am *AnalysisModal) SetGuildOptions(options []FilterableDropdownOption) {
	sorted := make([]FilterableDropdownOption, len(options))
	copy(sorted, options)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Display < sorted[j].Display
	})
	am.guildOptions = sorted
	if am.guildDropdown != nil {
		am.guildDropdown.SetOptions(sorted)
		// Preserve current selection when possible
		if am.guildValue != "" {
			am.guildDropdown.SetSelected(am.guildValue)
		}
		// Fallback to the first option if nothing is selected
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

// Toggle switches visibility on/off.
func (am *AnalysisModal) Toggle() {
	if am.modal == nil {
		return
	}
	if am.modal.IsVisible() {
		am.Hide()
		return
	}
	am.Show()
}

// Show makes the modal visible and focuses the input.
func (am *AnalysisModal) Show() {
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
func (am *AnalysisModal) Hide() {
	if am.modal != nil {
		am.modal.Hide()
	}
	if am.guildDropdown != nil {
		am.guildDropdown.InputFocused = false
		am.guildDropdown.IsOpen = false
	}
}

// IsVisible reports whether the modal is visible.
func (am *AnalysisModal) IsVisible() bool {
	return am.modal != nil && am.modal.IsVisible()
}

// Update processes input for the modal.
func (am *AnalysisModal) Update(mx, my int, deltaTime float64) {
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
}

// Draw renders the modal UI.
func (am *AnalysisModal) Draw(screen *ebiten.Image, _ int, _ int) {
	if !am.IsVisible() {
		return
	}

	am.modal.Draw(screen)
	cardRects := am.layoutCards()
	fontFace := loadWynncraftFont(16)

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

	// Redraw the dropdown last so its open menu sits above other content
	if am.guildDropdown != nil && am.guildDropdown.IsOpen {
		am.guildDropdown.Draw(screen)
	}
}

// HasTextInputFocused reports whether any analysis text input is active.
func (am *AnalysisModal) HasTextInputFocused() bool {
	if !am.IsVisible() {
		return false
	}
	if am.guildDropdown != nil && (am.guildDropdown.InputFocused || am.guildDropdown.IsOpen) {
		return true
	}
	return false
}

func (am *AnalysisModal) buildUI() {
	am.closeButton = NewEnhancedButton("x", 0, 0, 28, 28, func() {
		am.Hide()
	})
	am.closeButton.SetRedButtonStyle()
	am.cards = []*Card{am.buildChokepointCard(), am.buildPlaceholderCard()}
}

// buildChokepointCard builds the UI elements for chokepoint and HQ analyses.
func (am *AnalysisModal) buildChokepointCard() *Card {
	card := NewCard().Inline(false)
	card.SetBackgroundColor(EnhancedUIColors.Surface)
	card.SetBorderColor(EnhancedUIColors.BorderActive)

	title := NewMenuText("Analysis Tools", TextOptions{FontSize: 18, Height: 26, Color: EnhancedUIColors.Text})
	intro := NewMenuText("Score chokepoints for a guild and tint the map.", TextOptions{FontSize: 14, Height: 20, Color: EnhancedUIColors.TextSecondary})
	hqIntro := NewMenuText("Find the strongest HQ spots (connections + externals).", TextOptions{FontSize: 14, Height: 20, Color: EnhancedUIColors.TextSecondary})

	am.guildDropdown = NewFilterableDropdown(0, 0, 260, 32, am.guildOptions, func(option FilterableDropdownOption) {
		am.guildValue = strings.TrimSpace(option.Value)
	})
	if len(am.guildOptions) > 0 {
		// Default to the first option on initial build
		am.guildDropdown.SetSelected(am.guildOptions[0].Value)
		am.guildValue = am.guildOptions[0].Value
	}
	dropdownElement := newDropdownEdgeElement(am.guildDropdown, am.modal, 36)

	btnOpts := DefaultButtonOptions()
	btnOpts.Height = 32
	btnOpts.FontSize = 15
	btnOpts.BackgroundColor = EnhancedUIColors.Button
	btnOpts.HoverColor = EnhancedUIColors.BorderActive // More visible hover tint
	btnOpts.PressedColor = EnhancedUIColors.ButtonActive
	btnOpts.BorderColor = EnhancedUIColors.BorderGreen

	am.analyseButton = NewMenuButton(chokepointButtonLabel, btnOpts, func() {
		if am.onAnalyze != nil {
			if am.guildDropdown != nil {
				if selected, ok := am.guildDropdown.GetSelected(); ok {
					am.guildValue = strings.TrimSpace(selected.Value)
				}
			}
			if strings.TrimSpace(am.guildValue) == "" {
				am.SetStatus("Select a guild to analyse")
				return
			}
			am.onAnalyze(strings.TrimSpace(am.guildValue))
		}
	})

	hqBtnOpts := btnOpts
	am.hqAnalyseButton = NewMenuButton(hqButtonLabel, hqBtnOpts, func() {
		if am.onHQAnalyze != nil {
			if am.guildDropdown != nil {
				if selected, ok := am.guildDropdown.GetSelected(); ok {
					am.guildValue = strings.TrimSpace(selected.Value)
				}
			}
			if strings.TrimSpace(am.guildValue) == "" {
				am.SetStatus("Select a guild to analyse")
				return
			}
			am.onHQAnalyze(strings.TrimSpace(am.guildValue))
		}
	})

	if am.analyzing {
		am.SetAnalyzing(true)
	}

	am.statusText = NewMenuText("", TextOptions{FontSize: 14, Height: 18, Color: EnhancedUIColors.TextSecondary})
	hint := NewMenuText("Press A or Esc to close the modal.", TextOptions{FontSize: 12, Height: 16, Color: EnhancedUIColors.TextPlaceholder})

	card.elements = append(card.elements, title, intro, dropdownElement, am.analyseButton, hqIntro, am.hqAnalyseButton, am.statusText, hint)
	return card
}

// buildPlaceholderCard adds a secondary card for future analyses.
func (am *AnalysisModal) buildPlaceholderCard() *Card {
	card := NewCard().Inline(false)
	card.SetBackgroundColor(EnhancedUIColors.Surface)
	card.SetBorderColor(EnhancedUIColors.Border)

	title := NewMenuText("Additional Analyses", TextOptions{FontSize: 16, Height: 22, Color: EnhancedUIColors.Text})
	body := NewMenuText("More analysis tools will appear here soon.", TextOptions{FontSize: 14, Height: 20, Color: EnhancedUIColors.TextSecondary})

	card.elements = append(card.elements, title, body)
	return card
}

// layoutCards computes card rectangles within the modal content area.
func (am *AnalysisModal) layoutCards() []Rect {
	if am.modal == nil {
		return nil
	}

	cx, cy, cw, ch := am.modal.GetContentArea()
	padding := 16
	contentBounds := image.Rect(cx, cy, cx+cw, cy+ch)
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

	// Update dropdown container bounds to keep the menu within the modal
	if am.guildDropdown != nil {
		am.guildDropdown.SetContainerBounds(contentBounds)
	}

	return rects
}

// dropdownEdgeElement adapts FilterableDropdown to the EdgeMenuElement interface.
type dropdownEdgeElement struct {
	BaseMenuElement
	dropdown *FilterableDropdown
	modal    *EnhancedModal
	height   int
}

func newDropdownEdgeElement(fd *FilterableDropdown, modal *EnhancedModal, height int) *dropdownEdgeElement {
	return &dropdownEdgeElement{
		BaseMenuElement: NewBaseMenuElement(),
		dropdown:        fd,
		modal:           modal,
		height:          height,
	}
}

func (d *dropdownEdgeElement) Update(mx, my int, _ float64) bool {
	if !d.IsVisible() || d.dropdown == nil {
		return false
	}
	return d.dropdown.Update(mx, my)
}

func (d *dropdownEdgeElement) Draw(screen *ebiten.Image, x, y, width int, _ font.Face) int {
	if !d.IsVisible() || d.dropdown == nil {
		return 0
	}

	// Position the dropdown to the allotted space
	d.dropdown.X = x
	d.dropdown.Y = y
	d.dropdown.Width = width
	if d.height > 0 {
		d.dropdown.Height = d.height - 4
	}

	// Ensure dropdown knows its container bounds
	if d.modal != nil {
		cx, cy, cw, ch := d.modal.GetContentArea()
		d.dropdown.SetContainerBounds(image.Rect(cx, cy, cx+cw, cy+ch))
	}

	// Draw full dropdown (header + list); list will be redrawn in overlay pass for top layering.
	d.dropdown.Draw(screen)
	return d.height
}

func (d *dropdownEdgeElement) GetMinHeight() int {
	if !d.IsVisible() {
		return 0
	}
	return d.height
}
