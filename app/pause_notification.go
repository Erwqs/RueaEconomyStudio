package app

import (
	"RueaES/eruntime"
	"image/color"
)

// PauseNotificationManager manages the pause state toast notification
type PauseNotificationManager struct {
	isActive      bool
	wasHalted     bool
	pauseToastID  string
	toastManager  *ToastManager
	isInitialized bool
}

var globalPauseNotificationManager *PauseNotificationManager

// InitPauseNotificationManager initializes the global pause notification manager
func InitPauseNotificationManager() {
	if globalPauseNotificationManager == nil {
		globalPauseNotificationManager = &PauseNotificationManager{
			isActive:      false,
			wasHalted:     false,
			pauseToastID:  "",
			isInitialized: true,
		}
	}
}

// GetPauseNotificationManager returns the global pause notification manager
func GetPauseNotificationManager() *PauseNotificationManager {
	if globalPauseNotificationManager == nil {
		InitPauseNotificationManager()
	}
	return globalPauseNotificationManager
}

// Update checks the pause state and shows/hides the notification as needed
func (pnm *PauseNotificationManager) Update() {
	if !pnm.isInitialized {
		return
	}

	// Get current halt state
	currentHalted := eruntime.IsHalted()

	// Check if halt state changed
	if currentHalted != pnm.wasHalted {
		pnm.wasHalted = currentHalted

		if currentHalted {
			// State is now paused - show toast notification
			pnm.showPauseNotification()
		} else {
			// State is now resumed - hide toast notification
			pnm.hidePauseNotification()
		}
	}
}

// showPauseNotification displays the pause notification toast
func (pnm *PauseNotificationManager) showPauseNotification() {
	// Hide any existing pause notification first
	pnm.hidePauseNotification()
	// fmt.Println("call 1")

	// Ensure toast manager is initialized
	if globalToastManager == nil {
		InitToastManager()
		// fmt.Println("call 2")
	}

	// Create toast notification with resume button
	// fmt.Println("call 3")
	toast := NewToast().
		Text("State Halted", ToastOption{
			Colour: color.RGBA{255, 255, 100, 255}, // Yellow text
		}).
		Text("No calculation, generation and resource movement will be done while halted.", ToastOption{
			Colour: color.RGBA{200, 200, 200, 255}, // Gray text
		}).
		Text("Press space or click resume to continue.", ToastOption{
			Colour: color.RGBA{200, 200, 200, 255}, // Gray text
		}).
		Text("Or Shift + Space to add tick(s).", ToastOption{
			Colour: color.RGBA{200, 200, 200, 255}, // Gray text
		}).
		Button("Resume", func() {
			// Resume the simulation when button is clicked
			eruntime.Resume()
			pnm.hidePauseNotification()
		}, 0, 0, ToastOption{
			Colour: color.RGBA{64, 192, 64, 255}, // Green button text
		}).
		Background(color.RGBA{60, 60, 60, 240}).
		Border(color.RGBA{255, 200, 100, 255}) // Orange border

	// Indefinitely
	toast.Show()

	// Store the toast ID for later removal
	if len(globalToastManager.toasts) > 0 {
		pnm.pauseToastID = globalToastManager.toasts[len(globalToastManager.toasts)-1].ID
		pnm.isActive = true
	}
}

// hidePauseNotification removes the pause notification toast
func (pnm *PauseNotificationManager) hidePauseNotification() {
	if pnm.isActive && pnm.pauseToastID != "" {
		// Remove the toast by ID
		if globalToastManager != nil {
			globalToastManager.RemoveToast(pnm.pauseToastID)
		}
		pnm.pauseToastID = ""
		pnm.isActive = false
	}
}

// IsActive returns whether the pause notification is currently being shown
func (pnm *PauseNotificationManager) IsActive() bool {
	return pnm.isActive
}
