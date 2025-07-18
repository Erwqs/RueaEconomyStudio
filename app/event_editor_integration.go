package app

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// HandleEventEditorInput handles input events for the event editor
func (m *MapView) HandleEventEditorInput() bool {
	return false // this shit doesnt work

	// Check if E key is pressed to open event editor
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		// Don't open event editor if any text input is currently focused
		textInputFocused := false

		// Check if guild manager is open and has text input focused
		if m.territoriesManager != nil {
			guildManager := m.territoriesManager.guildManager
			if guildManager != nil && guildManager.IsVisible() && guildManager.HasTextInputFocused() {
				textInputFocused = true
			}
		}

		// Check if loadout manager is open and has text input focused
		loadoutManager := GetLoadoutManager()
		if loadoutManager != nil && loadoutManager.IsVisible() && loadoutManager.HasTextInputFocused() {
			textInputFocused = true
		}

		// Check if transit resource menu is open and has text input focused
		if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() && m.transitResourceMenu.HasTextInputFocused() {
			textInputFocused = true
		}

		// Check if event editor has text input focused
		if m.eventEditor != nil && m.eventEditor.IsVisible() && m.eventEditor.IsTextInputFocused() {
			textInputFocused = true
		}

		// Only open event editor if no text input is focused
		if !textInputFocused {
			m.OpenEventEditor()
			return true // Input was handled
		}
	}

	return false // Input was not handled
}

// OpenEventEditor opens the event editor and closes all other UI elements
func (m *MapView) OpenEventEditor() {
	// Close all modals and UI elements except panic notifier
	m.CloseAllUIElements()

	// Show the event editor
	if m.eventEditor != nil {
		m.eventEditor.Show()
	}
}

// CloseAllUIElements closes all UI elements except the panic notifier
func (m *MapView) CloseAllUIElements() {
	// Close state management menu
	if m.stateManagementMenu != nil && m.stateManagementMenu.menu.IsVisible() {
		m.stateManagementMenu.menu.Hide()
		m.stateManagementMenuVisible = false
	}

	// Close transit resource menu
	if m.transitResourceMenu != nil && m.transitResourceMenu.IsVisible() {
		m.transitResourceMenu.Hide()
	}

	// Close edge menu
	if m.edgeMenu != nil && m.edgeMenu.IsVisible() {
		m.edgeMenu.Hide()
		// Deselect territory when EdgeMenu is closed
		if m.territoriesManager != nil {
			m.territoriesManager.DeselectTerritory()
		}
	}

	// Close side menu (territories manager)
	if m.territoriesManager != nil && m.territoriesManager.IsSideMenuOpen() {
		m.territoriesManager.CloseSideMenu()
	}

	// Close tribute menu
	if m.tributeMenu != nil && m.tributeMenu.IsVisible() {
		m.tributeMenu.Hide()
	}

	// Close loadout manager
	loadoutManager := GetLoadoutManager()
	if loadoutManager != nil && loadoutManager.IsVisible() {
		loadoutManager.Hide()
	}

	// Close guild manager if open
	if m.territoriesManager != nil {
		guildManager := m.territoriesManager.guildManager
		if guildManager != nil && guildManager.IsVisible() {
			guildManager.Hide()
		}
	}

	// Exit claim editing mode if active
	if m.isEditingClaims {
		m.CancelClaimEditing()
	}
}

// UpdateEventEditor updates the event editor
func (m *MapView) UpdateEventEditor(screenW, screenH int) bool {
	if m.eventEditor == nil {
		return false
	}

	return m.eventEditor.Update(screenW, screenH)
}

// DrawEventEditor draws the event editor
func (m *MapView) DrawEventEditor(screen *ebiten.Image) {
	if m.eventEditor != nil {
		m.eventEditor.Draw(screen)
	}
}

// IsEventEditorVisible returns whether the event editor is currently visible
func (m *MapView) IsEventEditorVisible() bool {
	return m.eventEditor != nil && m.eventEditor.IsVisible()
}

// IsEventEditorInMinimalMode returns whether the event editor is visible and in minimal mode
func (m *MapView) IsEventEditorInMinimalMode() bool {
	return m.eventEditor != nil && m.eventEditor.IsVisible() && m.eventEditor.GetLayoutAnimationPhase() >= 0.5
}

// IsTerritoryPickerActive returns whether the event editor's territory picker is currently active
func (m *MapView) IsTerritoryPickerActive() bool {
	return m.eventEditor != nil && m.eventEditor.territoryPicker != nil && m.eventEditor.territoryPicker.IsActive()
}
