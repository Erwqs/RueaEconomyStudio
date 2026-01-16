package app

import (
	"image"

	"RueaES/pluginhost"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type pluginModalOverlay struct {
	host     *pluginhost.Host
	modal    *EnhancedModal
	pluginID string
	modalID  string
	controls []pluginhost.UIControl
	values   map[string]any

	buttons    map[string]*EnhancedButton
	checkboxes map[string]*Checkbox
	sliders    map[string]*MenuSlider
	textInputs map[string]*MenuTextInput
	selects    map[string]*Dropdown
	layout     map[string]pluginModalSlot
	contentY   int
	contentH   int
}

type pluginModalSlot struct {
	x      int
	y      int
	width  int
	height int
}

func newPluginModalOverlay(host *pluginhost.Host) *pluginModalOverlay {
	return &pluginModalOverlay{host: host}
}

func (po *pluginModalOverlay) IsVisible() bool {
	return po.modal != nil && po.modal.IsVisible()
}

// HasTextInputFocused reports whether any plugin modal input is active, so host keybinds can be paused.
func (po *pluginModalOverlay) HasTextInputFocused() bool {
	if po == nil || po.modal == nil || !po.modal.IsVisible() {
		return false
	}
	for _, ti := range po.textInputs {
		if ti != nil && ti.focused {
			return true
		}
	}
	return false
}

func (po *pluginModalOverlay) Show(spec pluginhost.ModalSpec) {
	width := 720
	// Grow height when many controls are present to avoid overflow.
	estimated := 200 + len(spec.Controls)*(35+12) // base padding + control height + spacing
	height := estimated
	if height < 520 {
		height = 520
	}
	po.modal = NewEnhancedModal(spec.Title, width, height)
	po.pluginID = spec.PluginID
	po.modalID = spec.ModalID
	po.controls = spec.Controls
	po.values = make(map[string]any)
	for k, v := range spec.InitialValues {
		po.values[k] = v
	}
	po.modal.Show()
	po.buildControls()
}

func (po *pluginModalOverlay) Hide() {
	if po.modal != nil {
		po.modal.Hide()
	}
}

func (po *pluginModalOverlay) buildControls() {
	po.buttons = make(map[string]*EnhancedButton)
	po.checkboxes = make(map[string]*Checkbox)
	po.sliders = make(map[string]*MenuSlider)
	po.textInputs = make(map[string]*MenuTextInput)
	po.selects = make(map[string]*Dropdown)
	po.layout = make(map[string]pluginModalSlot)

	if po.modal == nil || !po.modal.IsVisible() {
		return
	}

	cx, cy, cw, ch := po.modal.GetContentArea()
	po.contentY = cy
	po.contentH = ch
	startX := cx
	startY := cy
	btnW := cw
	btnH := 32
	spacing := 12
	curY := startY

	for _, c := range po.controls {
		key := po.pluginID + "::" + po.modalID + "::" + c.ID
		switch c.Kind {
		case pluginhost.UIControlButton:
			btn := NewEnhancedButton(c.Label, startX, curY, btnW, btnH, nil)
			btn.OnClick = func(control pluginhost.UIControl) func() {
				return func() {
					if po.host != nil {
						_ = po.host.SendUIEvent(po.pluginID, pluginhost.UIEvent{ModalID: po.modalID, ControlID: control.ID, Kind: pluginhost.UIControlButton})
					}
				}
			}(c)
			po.buttons[key] = btn
			po.layout[key] = pluginModalSlot{x: startX, y: curY, width: btnW, height: btnH}
			curY += btnH + spacing
		case pluginhost.UIControlCheckbox:
			cb := NewCheckbox(startX, curY, 18, c.Label, loadWynncraftFont(14))
			if val, ok := po.values[c.ID]; ok {
				if b, bok := toBool(val); bok {
					cb.SetChecked(b)
				}
			}
			cb.SetOnClick(func(checked bool) {
				po.values[c.ID] = checked
				if po.host != nil {
					_ = po.host.SendUIEvent(po.pluginID, pluginhost.UIEvent{ModalID: po.modalID, ControlID: c.ID, Kind: pluginhost.UIControlCheckbox, I64: boolToI64(checked)})
				}
			})
			po.checkboxes[key] = cb
			po.layout[key] = pluginModalSlot{x: startX, y: curY, width: cb.Size, height: cb.Size}
			curY += cb.Size + spacing
		case pluginhost.UIControlSlider:
			sOpts := DefaultSliderOptions()
			sOpts.MinValue = c.MinValue
			sOpts.MaxValue = c.MaxValue
			if c.Step > 0 {
				sOpts.Step = c.Step
			}
			sOpts.ShowValue = true
			sOpts.ValueFormat = "%.2f"
			initial := c.MinValue
			if val, ok := po.values[c.ID]; ok {
				if f, fok := toFloat(val); fok {
					initial = f
				}
			}
			slider := NewMenuSlider(c.Label, initial, sOpts, func(val float64) {
				po.values[c.ID] = val
				if po.host != nil {
					_ = po.host.SendUIEvent(po.pluginID, pluginhost.UIEvent{ModalID: po.modalID, ControlID: c.ID, Kind: pluginhost.UIControlSlider, F64: val})
				}
			})
			po.sliders[key] = slider
			po.layout[key] = pluginModalSlot{x: startX, y: curY, width: btnW, height: sOpts.Height}
			curY += sOpts.Height + spacing
		case pluginhost.UIControlText:
			tOpts := DefaultTextInputOptions()
			tOpts.Width = btnW
			tOpts.Height = 28
			initial := ""
			if val, ok := po.values[c.ID]; ok {
				if s, ok := val.(string); ok {
					initial = s
				}
			}
			textInput := NewMenuTextInput(c.Label, initial, tOpts, func(val string) {
				po.values[c.ID] = val
				if po.host != nil {
					_ = po.host.SendUIEvent(po.pluginID, pluginhost.UIEvent{ModalID: po.modalID, ControlID: c.ID, Kind: pluginhost.UIControlText, Str: val})
				}
			})
			po.textInputs[key] = textInput
			po.layout[key] = pluginModalSlot{x: startX, y: curY, width: btnW, height: tOpts.Height}
			curY += tOpts.Height + spacing
		case pluginhost.UIControlSelect:
			if len(c.Options) == 0 {
				continue
			}
			dd := NewDropdown(startX, curY, btnW, btnH, c.Options, func(selected string) {
				idx := indexOf(c.Options, selected)
				po.values[c.ID] = selected
				if po.host != nil {
					_ = po.host.SendUIEvent(po.pluginID, pluginhost.UIEvent{ModalID: po.modalID, ControlID: c.ID, Kind: pluginhost.UIControlSelect, I64: int64(idx)})
				}
			})
			if val, ok := po.values[c.ID]; ok {
				switch v := val.(type) {
				case string:
					if idx := indexOf(c.Options, v); idx >= 0 {
						dd.selectedIndex = idx
					}
				case int:
					if v >= 0 && v < len(c.Options) {
						dd.selectedIndex = v
					}
				case int64:
					if v >= 0 && int(v) < len(c.Options) {
						dd.selectedIndex = int(v)
					}
				}
			}
			po.selects[key] = dd
			po.layout[key] = pluginModalSlot{x: startX, y: curY, width: btnW, height: btnH}
			curY += btnH + spacing
		}
	}
}

func (po *pluginModalOverlay) Update() bool {
	if po.modal == nil || !po.modal.IsVisible() {
		return false
	}

	po.modal.Update()
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !po.modal.Contains(mx, my) {
		po.Hide()
		return true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		po.Hide()
		return true
	}

	for key, btn := range po.buttons {
		if slot, ok := po.layout[key]; ok {
			btn.X = slot.x
			btn.Y = slot.y
		}
		btn.Update(mx, my)
	}
	mPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	for key, cb := range po.checkboxes {
		if slot, ok := po.layout[key]; ok {
			cb.X = slot.x
			cb.Y = slot.y
		}
		cb.Update(mx, my, mPressed)
	}
	delta := 1.0 / 60.0
	for key, sl := range po.sliders {
		if slot, ok := po.layout[key]; ok {
			sl.sliderRect = image.Rect(slot.x+5, slot.y+14, slot.x+slot.width-5, slot.y+34)
		}
		sl.Update(mx, my, delta)
	}
	for key, ti := range po.textInputs {
		if slot, ok := po.layout[key]; ok {
			ti.rect = image.Rect(slot.x, slot.y, slot.x+slot.width, slot.y+slot.height)
		}
		ti.Update(mx, my, delta)
	}
	for key, dd := range po.selects {
		if slot, ok := po.layout[key]; ok {
			dd.X = slot.x
			dd.Y = slot.y
			dd.Width = slot.width
		}
		dd.Update(mx, my)
	}

	return true
}

func (po *pluginModalOverlay) Draw(screen *ebiten.Image) {
	if po.modal == nil || !po.modal.IsVisible() {
		return
	}

	po.modal.Draw(screen)
	font := loadWynncraftFont(14)

	for _, btn := range po.buttons {
		btn.Draw(screen)
	}
	for _, cb := range po.checkboxes {
		cb.Draw(screen)
	}
	for key, sl := range po.sliders {
		if slot, ok := po.layout[key]; ok {
			sl.Draw(screen, slot.x, slot.y, slot.width, font)
		}
	}
	for key, ti := range po.textInputs {
		if slot, ok := po.layout[key]; ok {
			ti.Draw(screen, slot.x, slot.y, slot.width, font)
		}
	}
	for _, dd := range po.selects {
		dd.Draw(screen)
	}
}
