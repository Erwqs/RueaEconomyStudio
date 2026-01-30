package app

import (
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// LockWarningModal shows a startup warning when an existing lock file is detected.
type LockWarningModal struct {
	modal          *EnhancedModal
	continueButton *EnhancedButton
	exitButton     *EnhancedButton
	visible        bool
	messageLines   []string
	lockPath       string
	onContinue     func()
	onExit         func()
	font           font.Face
	buttonWidth    int
	buttonHeight   int
}

var (
	globalLockWarning *LockWarningModal
	lockWarningOnce   sync.Once
	lockWarningTask   struct {
		pending    bool
		lockPath   string
		onContinue func()
		onExit     func()
	}
)

// InitLockWarningModal prepares the global lock warning modal instance.
func InitLockWarningModal() {
	if globalLockWarning != nil {
		return
	}

	m := &LockWarningModal{
		modal:        NewEnhancedModal("Lock file found", 760, 320),
		font:         loadWynncraftFont(16),
		buttonWidth:  150,
		buttonHeight: 44,
		messageLines: []string{
			"Only one instance of RueaES should be running; multiple instances can corrupt data.",
			"If no other instances are running, click Continue. Otherwise, proceed at your own risk.",
		},
	}

	m.continueButton = NewEnhancedButton("Continue", 0, 0, m.buttonWidth, m.buttonHeight, func() {
		if m.onContinue != nil {
			m.onContinue()
		}
		m.Hide()
	})
	m.continueButton.SetGreenButtonStyle()

	m.exitButton = NewEnhancedButton("Exit", 0, 0, m.buttonWidth, m.buttonHeight, func() {
		if m.onExit != nil {
			m.onExit()
		}
	})
	m.exitButton.SetRedButtonStyle()

	globalLockWarning = m
}

// GetLockWarningModal returns the global lock warning modal instance.
func GetLockWarningModal() *LockWarningModal {
	lockWarningOnce.Do(func() {
		InitLockWarningModal()
	})
	return globalLockWarning
}

// ScheduleLockWarning defers showing the lock warning until the UI is initialized.
func ScheduleLockWarning(lockPath string, onContinue, onExit func()) {
	lockWarningTask.pending = true
	lockWarningTask.lockPath = lockPath
	lockWarningTask.onContinue = onContinue
	lockWarningTask.onExit = onExit
}

// ShowScheduledLockWarning displays the warning if it was scheduled before UI init.
func ShowScheduledLockWarning() {
	if !lockWarningTask.pending {
		return
	}
	modal := GetLockWarningModal()
	modal.Show(lockWarningTask.lockPath, lockWarningTask.onContinue, lockWarningTask.onExit)
	lockWarningTask.pending = false
	lockWarningTask.lockPath = ""
	lockWarningTask.onContinue = nil
	lockWarningTask.onExit = nil
}

// Show makes the warning visible with the provided callbacks.
func (m *LockWarningModal) Show(lockPath string, onContinue, onExit func()) {
	m.lockPath = lockPath
	m.onContinue = onContinue
	m.onExit = onExit
	m.visible = true
	if m.modal != nil {
		m.modal.Show()
		m.updateButtonPositions()
	}
}

// Hide hides the modal.
func (m *LockWarningModal) Hide() {
	m.visible = false
	if m.modal != nil {
		m.modal.Hide()
	}
}

// IsVisible returns whether the modal is currently shown.
func (m *LockWarningModal) IsVisible() bool {
	return m != nil && m.visible
}

// Update processes input for the modal and returns true if it consumed input.
func (m *LockWarningModal) Update() bool {
	if !m.IsVisible() {
		return false
	}

	if m.modal != nil {
		m.modal.Update()
	}

	mx, my := ebiten.CursorPosition()
	if m.continueButton != nil {
		_ = m.continueButton.Update(mx, my)
	}
	if m.exitButton != nil {
		_ = m.exitButton.Update(mx, my)
	}

	// Always consume input while visible to block the rest of the app.
	return true
}

// Draw renders the modal contents.
func (m *LockWarningModal) Draw(screen *ebiten.Image) {
	if !m.IsVisible() {
		return
	}

	if m.modal != nil {
		m.modal.Draw(screen)
	}

	contentX, contentY, _, _ := m.modal.GetContentArea()
	y := contentY + 10

	// Title line (reinforce warning)
	titleColor := color.RGBA{255, 213, 77, 255}
	text.Draw(screen, "Lock file found", m.font, contentX, y+6, titleColor)
	y += 30

	bodyColor := EnhancedUIColors.Text
	lines := make([]string, 0, len(m.messageLines)+1)
	if m.lockPath != "" {
		lines = append(lines, "Detected lock: "+m.lockPath)
	}
	lines = append(lines, m.messageLines...)

	for _, line := range lines {
		text.Draw(screen, line, m.font, contentX, y, bodyColor)
		y += 22
	}

	if m.continueButton != nil {
		m.continueButton.Draw(screen)
	}
	if m.exitButton != nil {
		m.exitButton.Draw(screen)
	}
}

func (m *LockWarningModal) updateButtonPositions() {
	if m.modal == nil || m.continueButton == nil || m.exitButton == nil {
		return
	}

	bounds := m.modal.GetBounds()
	spacing := 16
	totalWidth := 2*m.buttonWidth + spacing
	startX := bounds.Min.X + (bounds.Dx()-totalWidth)/2
	buttonY := bounds.Max.Y - 70

	m.continueButton.SetPosition(startX, buttonY)
	m.exitButton.SetPosition(startX+m.buttonWidth+spacing, buttonY)
}
