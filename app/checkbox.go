package app

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// checkbox component
type Checkbox struct {
	X         int
	Y         int
	Size      int
	Checked   bool
	Label     string
	Enabled   bool // is it interactable
	labelFont font.Face
	onClick   func(bool)
	hovered   bool
}

func NewCheckbox(x, y, size int, label string, labelFont font.Face) *Checkbox {
	return &Checkbox{
		X:         x,
		Y:         y,
		Size:      size,
		Label:     label,
		Enabled:   true,
		labelFont: labelFont,
		Checked:   false,
		hovered:   false,
	}
}

// cb func
func (cb *Checkbox) SetOnClick(onClick func(bool)) {
	cb.onClick = onClick
}

func (cb *Checkbox) Update(mouseX, mouseY int, mousePressed bool) {
	if !cb.Enabled {
		cb.hovered = false
		return
	}

	// check if checkbox is hovered
	cb.hovered = mouseX >= cb.X && mouseX <= cb.X+cb.Size &&
		mouseY >= cb.Y && mouseY <= cb.Y+cb.Size

	// click
	if cb.hovered && mousePressed {
		cb.Checked = !cb.Checked
		if cb.onClick != nil {
			cb.onClick(cb.Checked)
		}
	}
}

func (cb *Checkbox) Draw(s *ebiten.Image) {
	// state based coloring
	boxColor := color.RGBA{100, 100, 100, 255}
	borderColor := color.RGBA{150, 150, 150, 255}
	textColor := color.RGBA{255, 255, 255, 255}

	if !cb.Enabled {
		boxColor = color.RGBA{60, 60, 60, 255}
		borderColor = color.RGBA{80, 80, 80, 255}
		textColor = color.RGBA{120, 120, 120, 255}
	} else if cb.hovered {
		boxColor = color.RGBA{120, 120, 120, 255}
		borderColor = color.RGBA{180, 180, 180, 255}
	}

	// outlines around the box
	if cb.Checked {
		// fill rect
		vector.DrawFilledRect(s, float32(cb.X), float32(cb.Y), float32(cb.Size), float32(cb.Size), boxColor, false)

		checkColor := color.RGBA{255, 255, 255, 255}
		if !cb.Enabled {
			checkColor = color.RGBA{180, 180, 180, 255}
		}

		// fill when checked
		margin := float32(2)
		vector.DrawFilledRect(s, float32(cb.X)+margin, float32(cb.Y)+margin, float32(cb.Size)-margin*2, float32(cb.Size)-margin*2, checkColor, false)
	
		// draw outer outline
		// vector.DrawFilledRect(s, float32(cb.X)+margin, float32(cb.Y)+margin, float32(cb.Size)-margin*2, float32(cb.Size)-margin*2, checkColor, false)
	} else {
		// outlines
		lineWidth := float32(2)
		// top
		vector.DrawFilledRect(s, float32(cb.X), float32(cb.Y), float32(cb.Size), lineWidth, borderColor, false)
		// down
		vector.DrawFilledRect(s, float32(cb.X), float32(cb.Y+cb.Size-int(lineWidth)), float32(cb.Size), lineWidth, borderColor, false)
		// left
		vector.DrawFilledRect(s, float32(cb.X), float32(cb.Y), lineWidth, float32(cb.Size), borderColor, false)
		// right
		vector.DrawFilledRect(s, float32(cb.X+cb.Size-int(lineWidth)), float32(cb.Y), lineWidth, float32(cb.Size), borderColor, false)
	}

	// label
	if cb.Label != "" && cb.labelFont != nil {
		labelX := cb.X + cb.Size + 10  // x padding
		labelY := cb.Y + cb.Size/2 + 4 // y padding
		text.Draw(s, cb.Label, cb.labelFont, labelX, labelY, textColor)
	}
}

func (cb *Checkbox) SetChecked(checked bool) {
	cb.Checked = checked
}

// make the button interactable or not
func (cb *Checkbox) SetEnabled(enabled bool) {
	cb.Enabled = enabled
	if !enabled {
		cb.hovered = false
	}
}

func (cb *Checkbox) IsChecked() bool {
	return cb.Checked
}

// for positioning other elems
func (cb *Checkbox) GetBounds() (x, y, width, height int) {
	labelWidth := 0
	if cb.Label != "" && cb.labelFont != nil {
		bounds := text.BoundString(cb.labelFont, cb.Label)
		labelWidth = bounds.Dx() + 10
	}
	return cb.X, cb.Y, cb.Size + labelWidth, cb.Size
}
