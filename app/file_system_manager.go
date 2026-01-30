package app

import (
	"fmt"
	"image/color"
	"log"
	"path/filepath"
	"strings"
	"time"

	"RueaES/eruntime"
	"RueaES/storage"

	"github.com/hajimehoshi/ebiten/v2"
)

// Global file system manager instance
var globalFileSystemManager *FileSystemManager
var globalStateImportModal *StateImportModal

// GetFileSystemManager returns the global file system manager instance
func GetFileSystemManager() *FileSystemManager {
	return globalFileSystemManager
}

// InitializeFileSystemManager creates and initializes the global file system manager
func InitializeFileSystemManager(inputManager *InputManager) {
	// fmt.Println("[FILE] InitializeFileSystemManager called")
	globalFileSystemManager = NewFileSystemManager(inputManager)

	// Initialize the state import modal
	titleFont := GetDefaultFont(24)       // Increased from 16 for high DPI
	contentFont := GetDefaultFont(18)     // Increased from 14 for high DPI
	descriptionFont := GetDefaultFont(14) // Smaller font for descriptions
	globalStateImportModal = NewStateImportModal(titleFont, contentFont, descriptionFont)

	// Set up import modal callbacks
	globalStateImportModal.SetCallbacks(
		func(selectedOptions map[string]bool, filePath string) {
			// Handle import
			// fmt.Printf("[STATE] Importing state with options: %+v\n", selectedOptions)
			// Use the public API (it handles errors internally with logging)
			eruntime.LoadStateSelective(filePath, selectedOptions)

			// Show success toast
			NewToast().
				Text("State import initiated", ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
				AutoClose(time.Second * 3).
				Show()
		},
		func() {
			// Handle cancel
			// fmt.Printf("[STATE] Import cancelled\n")
		},
	)

	// Set up state save/load callbacks
	globalFileSystemManager.SetOnFileSaved(func(filepath string, content []byte) error {
		// fmt.Printf("[FILE] onFileSaved callback called for: %s\n", filepath)
		// For state saves, ignore content parameter and call SaveState API
		eruntime.SaveState(filepath)

		NewToast().
			Text("State saved successfully to "+filepath, ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
			AutoClose(time.Second * 3).
			Show()
		return nil
	})

	globalFileSystemManager.SetOnFileOpened(func(filepath string, content []byte) error {
		// fmt.Printf("[FILE] onFileOpened callback called for: %s\n", filepath)

		// Check if this is a state file (has .lz4 extension)
		if filepath != "" && (strings.HasSuffix(filepath, ".lz4") || strings.Contains(filepath, "state")) {
			// Show import modal for state files
			ShowStateImportModal(filepath)
		} else {
			// For other files, load directly
			eruntime.LoadState(filepath)
			NewToast().
				Text("Loading state data from "+filepath, ToastOption{Colour: color.RGBA{100, 255, 100, 255}}).
				AutoClose(time.Second * 3).
				Show()
		}
		return nil
	})
}

// FileSystemManager manages file operations and dialogues
type FileSystemManager struct {
	inputManager *InputManager
	keyEventCh   <-chan KeyEvent

	// Active dialogues
	openDialogue *FileSystemDialogue
	saveDialogue *FileSystemDialogue

	// Current file context
	currentFile      string
	workingDirectory string

	// File operation callbacks
	onFileOpened func(filepath string, content []byte) error
	onFileSaved  func(filepath string, content []byte) error

	// State
	active           bool
	lastSaveTime     int64
	autoSaveEnabled  bool
	autoSaveInterval int64 // in milliseconds
}

// NewFileSystemManager creates a new file system manager
func NewFileSystemManager(inputManager *InputManager) *FileSystemManager {
	fsm := &FileSystemManager{
		inputManager:     inputManager,
		keyEventCh:       inputManager.Subscribe(),
		autoSaveEnabled:  true,
		autoSaveInterval: 30000, // 30 seconds
		active:           true,
	}

	// Set default working directory to the app data directory
	fsm.workingDirectory = storage.DataDir()

	return fsm
}

// Update processes keyboard shortcuts and updates dialogues
func (fsm *FileSystemManager) Update() bool {
	if !fsm.active {
		return false
	}

	// Process key events for global shortcuts
	fsm.handleKeyEvents()

	// Update active dialogues and check if they're consuming input
	inputConsumed := false

	if fsm.openDialogue != nil && fsm.openDialogue.IsVisible() {
		if fsm.openDialogue.Update() {
			inputConsumed = true
		}
	}

	if fsm.saveDialogue != nil && fsm.saveDialogue.IsVisible() {
		if fsm.saveDialogue.Update() {
			inputConsumed = true
		}
	}

	// Update state import modal
	if globalStateImportModal != nil && globalStateImportModal.IsVisible() {
		globalStateImportModal.Update()
		inputConsumed = true
	}

	// Handle auto-save if enabled
	fsm.handleAutoSave()

	// Return true if any dialogue consumed input (blocks background input)
	return inputConsumed
}

// handleKeyEvents processes keyboard shortcuts
func (fsm *FileSystemManager) handleKeyEvents() {
	for {
		select {
		case event := <-fsm.keyEventCh:
			if event.Pressed {
				fsm.handleKeyEvent(event)
			}
		default:
			return
		}
	}
}

// handleKeyEvent processes individual key events for file operations
func (fsm *FileSystemManager) handleKeyEvent(event KeyEvent) {
	// Only process key events if no dialogue is currently visible
	// This prevents shortcuts from triggering while a dialogue is open
	if (fsm.openDialogue != nil && fsm.openDialogue.IsVisible()) ||
		(fsm.saveDialogue != nil && fsm.saveDialogue.IsVisible()) {
		return
	}

	// Check for Ctrl key combinations
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		switch event.Key {
		case ebiten.KeyS:
			// Ctrl+S - Save file
			fsm.ShowSaveDialogue()
		case ebiten.KeyO:
			// Ctrl+O - Open file
			fsm.ShowOpenDialogue()
		}
	}
}

// ShowOpenDialogue displays the open file dialogue
func (fsm *FileSystemManager) ShowOpenDialogue() {
	// Close existing dialogue if open
	if fsm.openDialogue != nil {
		fsm.openDialogue.Hide()
	}

	// Define supported file extensions for state files
	allowedExts := []string{".lz4", ".ruea"}

	// Create new open dialogue
	fsm.openDialogue = NewFileSystemDialogue(FileDialogueOpen, "Load State", allowedExts)
	fsm.openDialogue.SetCurrentPath(fsm.workingDirectory)

	// Set callbacks
	fsm.openDialogue.SetOnFileSelected(func(filepath string) {
		fsm.handleFileOpen(filepath)
	})

	fsm.openDialogue.SetOnCancelled(func() {
		log.Println("[FILE] Open dialogue cancelled")
	})

	// Show the dialogue
	fsm.openDialogue.Show()

	log.Println("[FILE] Open dialogue displayed")
}

// ShowSaveDialogue displays the save file dialogue
func (fsm *FileSystemManager) ShowSaveDialogue() {
	// fmt.Println("[FILE] ShowSaveDialogue called")

	// Close existing dialogue if open
	if fsm.saveDialogue != nil {
		fsm.saveDialogue.Hide()
	}

	// Define supported file extensions for state files
	allowedExts := []string{".lz4", ".ruea"}

	// Create new save dialogue
	fsm.saveDialogue = NewFileSystemDialogue(FileDialogueSave, "Save State", allowedExts)
	fsm.saveDialogue.SetCurrentPath(fsm.workingDirectory)

	// Set callbacks
	fsm.saveDialogue.SetOnFileSelected(func(filepath string) {
		// fmt.Printf("[FILE] Save file selected: %s\n", filepath)
		fsm.handleFileSave(filepath)
	})

	fsm.saveDialogue.SetOnCancelled(func() {
		// log.Println("[FILE] Save dialogue cancelled")
	})

	// Show the dialogue
	fsm.saveDialogue.Show()

	// log.Println("[FILE] Save dialogue displayed")
}

// handleFileOpen processes opening a selected file
func (fsm *FileSystemManager) handleFileOpen(filepath string) {
	// log.Printf("[FILE] Opening file: %s", filepath)

	// Read file content
	content, err := fsm.readFile(filepath)
	if err != nil {
		// log.Printf("[FILE] Error reading file %s: %v", filepath, err)
		return
	}

	// Update current file context
	fsm.currentFile = filepath
	if lastSlash := strings.LastIndex(filepath, "/"); lastSlash != -1 {
		fsm.workingDirectory = filepath[:lastSlash]
	} else {
		fsm.workingDirectory = "."
	}

	// Call callback if set
	if fsm.onFileOpened != nil {
		if err := fsm.onFileOpened(filepath, content); err != nil {
			// log.Printf("[FILE] Error processing opened file %s: %v", filepath, err)
			return
		}
	}

	// log.Printf("[FILE] Successfully opened file: %s (%d bytes)", filepath, len(content))
}

// handleFileSave processes saving to a selected file
func (fsm *FileSystemManager) handleFileSave(filepath string) {
	// fmt.Printf("[FILE] handleFileSave called with: %s\n", filepath)
	// log.Printf("[FILE] Saving file: %s", filepath)

	// Ensure file has proper extension
	if !fsm.hasValidExtension(filepath) {
		filepath = fsm.addDefaultExtension(filepath)
		// fmt.Printf("[FILE] Added default extension, new path: %s\n", filepath)
	}

	// Call the save callback
	if fsm.onFileSaved != nil {
		// fmt.Printf("[FILE] Calling onFileSaved callback for: %s\n", filepath)
		content := []byte("") // Placeholder - actual content handled by callback

		if err := fsm.onFileSaved(filepath, content); err != nil {
			// log.Printf("[FILE] Error saving file %s: %v", filepath, err)
			// fmt.Printf("[FILE] Error in save callback: %v\n", err)
			return
		}
		// fmt.Printf("[FILE] Save callback completed successfully\n")
	} else {
		// fmt.Printf("[FILE] No onFileSaved callback set!\n")
	}

	// Update current file context
	fsm.currentFile = filepath
	if lastSlash := strings.LastIndex(filepath, "/"); lastSlash != -1 {
		fsm.workingDirectory = filepath[:lastSlash]
	} else {
		fsm.workingDirectory = "."
	}

	// log.Printf("[FILE] Successfully saved file: %s", filepath)
}

// readFile reads content from a file
func (fsm *FileSystemManager) readFile(filepath string) ([]byte, error) {
	// This would implement actual file reading
	// For now, return empty content as placeholder
	return []byte{}, nil
}

// hasValidExtension checks if the file has a valid extension
func (fsm *FileSystemManager) hasValidExtension(filepath string) bool {
	if !strings.Contains(filepath, ".") {
		return false
	}
	ext := strings.ToLower(filepath[strings.LastIndex(filepath, "."):])
	validExts := []string{".json", ".lz4", ".ruea", ".txt", ".cfg", ".config"}

	for _, validExt := range validExts {
		if ext == validExt {
			return true
		}
	}
	return false
}

// addDefaultExtension adds a default extension to a filename
func (fsm *FileSystemManager) addDefaultExtension(filepath string) string {
	if !strings.Contains(filepath, ".") {
		return filepath + ".lz4" // Default to .lz4 for state files
	}
	return filepath
}

// handleAutoSave handles automatic saving if enabled
func (fsm *FileSystemManager) handleAutoSave() {
	if !fsm.autoSaveEnabled || fsm.currentFile == "" {
		return
	}

	// Implementation for auto-save logic would go here
	// This might check if content has changed and save periodically
}

// Draw renders any visible dialogues
func (fsm *FileSystemManager) Draw(screen *ebiten.Image) {
	if fsm.openDialogue != nil && fsm.openDialogue.IsVisible() {
		fsm.openDialogue.Draw(screen)
	}

	if fsm.saveDialogue != nil && fsm.saveDialogue.IsVisible() {
		fsm.saveDialogue.Draw(screen)
	}

	// Draw state import modal
	if globalStateImportModal != nil && globalStateImportModal.IsVisible() {
		globalStateImportModal.Draw(screen)
	}
}

// SetOnFileOpened sets the callback for when a file is opened
func (fsm *FileSystemManager) SetOnFileOpened(callback func(filepath string, content []byte) error) {
	fsm.onFileOpened = callback
}

// SetOnFileSaved sets the callback for when a file is saved
func (fsm *FileSystemManager) SetOnFileSaved(callback func(filepath string, content []byte) error) {
	fsm.onFileSaved = callback
}

// SetWorkingDirectory sets the default working directory
func (fsm *FileSystemManager) SetWorkingDirectory(path string) {
	if abs, err := filepath.Abs(path); err == nil {
		fsm.workingDirectory = abs
	}
}

// GetCurrentFile returns the currently active file path
func (fsm *FileSystemManager) GetCurrentFile() string {
	return fsm.currentFile
}

// SetCurrentFile sets the current file context
func (fsm *FileSystemManager) SetCurrentFile(filepath string) {
	fsm.currentFile = filepath
	if filepath != "" {
		if lastSlash := strings.LastIndex(filepath, "/"); lastSlash != -1 {
			fsm.workingDirectory = filepath[:lastSlash]
		} else {
			fsm.workingDirectory = "."
		}
	}
}

// IsDialogueVisible returns true if any file dialogue is currently visible
func (fsm *FileSystemManager) IsDialogueVisible() bool {
	return (fsm.openDialogue != nil && fsm.openDialogue.IsVisible()) ||
		(fsm.saveDialogue != nil && fsm.saveDialogue.IsVisible())
}

// SetActive enables or disables the file system manager
func (fsm *FileSystemManager) SetActive(active bool) {
	fsm.active = active
	if !active {
		// Hide any open dialogues when deactivated
		if fsm.openDialogue != nil {
			fsm.openDialogue.Hide()
		}
		if fsm.saveDialogue != nil {
			fsm.saveDialogue.Hide()
		}
	}
}

// IsActive returns whether the file system manager is active
func (fsm *FileSystemManager) IsActive() bool {
	return fsm.active
}

// SetAutoSave enables or disables auto-save functionality
func (fsm *FileSystemManager) SetAutoSave(enabled bool) {
	fsm.autoSaveEnabled = enabled
}

// SetAutoSaveInterval sets the auto-save interval in milliseconds
func (fsm *FileSystemManager) SetAutoSaveInterval(intervalMs int64) {
	fsm.autoSaveInterval = intervalMs
}

// QuickSave performs a quick save to the current file
func (fsm *FileSystemManager) QuickSave() error {
	if fsm.currentFile == "" {
		// No current file, show save dialogue
		fsm.ShowSaveDialogue()
		return fmt.Errorf("no current file set")
	}

	// Perform quick save to current file
	if fsm.onFileSaved != nil {
		content := []byte("") // Placeholder - should get actual content
		return fsm.onFileSaved(fsm.currentFile, content)
	}

	return fmt.Errorf("no save callback set")
}

// Cleanup cleans up resources when the manager is no longer needed
func (fsm *FileSystemManager) Cleanup() {
	if fsm.inputManager != nil {
		fsm.inputManager.Unsubscribe(fsm.keyEventCh)
	}

	if fsm.openDialogue != nil {
		fsm.openDialogue.Hide()
		fsm.openDialogue = nil
	}

	if fsm.saveDialogue != nil {
		fsm.saveDialogue.Hide()
		fsm.saveDialogue = nil
	}
}

// ShowStateImportModal displays the state import modal for the given file
func ShowStateImportModal(filePath string) {
	if globalStateImportModal == nil {
		return
	}

	// Validate the state file before showing the modal
	err := eruntime.ValidateStateFileAPI(filePath)
	if err != nil {
		// Show error toast for invalid/corrupted state files
		NewToast().
			Text("Invalid State File", ToastOption{Colour: color.RGBA{255, 100, 100, 255}}).
			Text(err.Error(), ToastOption{Colour: color.RGBA{255, 200, 200, 255}}).
			Button("Dismiss", func() {}, 0, 0, ToastOption{Colour: color.RGBA{255, 120, 120, 255}}).
			Show()
		return
	}

	// File is valid, show the import modal
	globalStateImportModal.Show(filePath)
}

// UpdateStateImportModal updates the state import modal if it's visible
func UpdateStateImportModal() {
	if globalStateImportModal != nil && globalStateImportModal.IsVisible() {
		globalStateImportModal.Update()
	}
}

// DrawStateImportModal draws the state import modal if it's visible
func DrawStateImportModal(screen *ebiten.Image) {
	if globalStateImportModal != nil && globalStateImportModal.IsVisible() {
		globalStateImportModal.Draw(screen)
	}
}
