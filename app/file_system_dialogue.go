package app

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

// FileDialogueType represents the type of file dialogue
type FileDialogueType int

const (
	FileDialogueOpen FileDialogueType = iota
	FileDialogueSave
)

// FileItem represents a file or directory in the file browser
type FileItem struct {
	Name      string
	Path      string
	IsDir     bool
	Size      int64
	ModTime   time.Time
	Extension string
}

// FileSystemDialogue represents a comprehensive file system dialogue modal
type FileSystemDialogue struct {
	*EnhancedModal
	dialogueType   FileDialogueType
	currentPath    string
	files          []FileItem
	filteredFiles  []FileItem
	selectedIndex  int
	scrollOffset   int
	filenameInput  *EnhancedTextInput
	pathInput      *EnhancedTextInput
	fileFilter     string
	allowedExts    []string
	selectedExt    string
	onFileSelected func(filepath string)
	onCancelled    func()

	// UI layout constants
	headerHeight    int
	footerHeight    int
	itemHeight      int
	sidebarWidth    int
	maxVisibleItems int

	// Interaction state
	lastClickTime  time.Time
	lastClickIndex int
	pathInputError bool

	// Navigation buttons
	upButton      *EnhancedButton
	homeButton    *EnhancedButton
	refreshButton *EnhancedButton
	cancelButton  *EnhancedButton
	confirmButton *EnhancedButton

	// Filter dropdown
	extensionDropdown *Dropdown

	// Current working directory for relative paths
	workingDir string
}

// NewFileSystemDialogue creates a new file system dialogue
func NewFileSystemDialogue(dialogueType FileDialogueType, title string, allowedExts []string) *FileSystemDialogue {
	// Get current working directory
	workingDir, _ := os.Getwd()

	// Create the enhanced modal - larger size for better visibility
	modal := NewEnhancedModal(title, 900, 700)

	fsd := &FileSystemDialogue{
		EnhancedModal:   modal,
		dialogueType:    dialogueType,
		currentPath:     workingDir,
		allowedExts:     allowedExts,
		headerHeight:    120, // Increased to accommodate moved-down path input
		footerHeight:    100, // Increased for better button spacing
		itemHeight:      36,  // Doubled from 24 to 36 for larger text
		sidebarWidth:    200,
		maxVisibleItems: 12, // Reduced further to prevent overflow
		workingDir:      workingDir,
		pathInputError:  false,
	}

	// Initialize UI components
	fsd.initializeUI()

	// Load initial directory
	fsd.loadDirectory(fsd.currentPath)

	return fsd
}

// initializeUI sets up all UI components
func (fsd *FileSystemDialogue) initializeUI() {
	modalBounds := fsd.EnhancedModal.GetBounds()

	// Create path input (combines path display and editing) - moved down more
	pathY := modalBounds.Min.Y + 40     // Moved down from 25 to 40
	pathWidth := modalBounds.Dx() - 220 // Leave space for buttons
	fsd.pathInput = TextInput("", modalBounds.Min.X+15, pathY, pathWidth, 35, 200)
	fsd.pathInput.SetText(fsd.currentPath)

	// Create smaller navigation buttons on the right - moved down with path input
	buttonHeight := 30 // Smaller buttons
	buttonWidth := 50  // Smaller width
	buttonSpacing := 5

	startX := modalBounds.Max.X - 165 // Right aligned

	fsd.upButton = NewEnhancedButton("Up", startX, pathY, buttonWidth, buttonHeight, func() {
		parent := filepath.Dir(fsd.currentPath)
		if parent != fsd.currentPath {
			fsd.loadDirectory(parent)
		}
	})

	fsd.homeButton = NewEnhancedButton("Home", startX+buttonWidth+buttonSpacing, pathY, buttonWidth, buttonHeight, func() {
		if homeDir, err := os.UserHomeDir(); err == nil {
			fsd.loadDirectory(homeDir)
		}
	})

	fsd.refreshButton = NewEnhancedButton("Refresh", startX+2*(buttonWidth+buttonSpacing), pathY, buttonWidth+10, buttonHeight, func() {
		fsd.loadDirectory(fsd.currentPath)
	})

	// Create filename input (for save dialogue) with better positioning
	if fsd.dialogueType == FileDialogueSave {
		filenameY := modalBounds.Max.Y - fsd.footerHeight + 15
		fsd.filenameInput = TextInput("Enter filename...", modalBounds.Min.X+15, filenameY, 400, 40, 100)

		// Create extension filter dropdown beside filename input if extensions are specified
		// Make it compact for extension names (extensions are only 3-4 characters)
		if len(fsd.allowedExts) > 0 {
			dropdownX := modalBounds.Min.X + 425 // Beside filename input

			// Prepare dropdown options
			options := []string{}
			for _, ext := range fsd.allowedExts {
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				options = append(options, "*"+ext)
			}

			fsd.extensionDropdown = NewDropdown(dropdownX, filenameY, 100, 40, options, func(selected string) {
				fsd.selectedExt = selected
				fsd.filterFiles()
			})
			// Set container bounds so dropdown knows about modal boundaries
			fsd.extensionDropdown.SetContainerBounds(modalBounds)
			fsd.selectedExt = "All Files"
		}
	} else {
		// For open dialogue, put extension dropdown at top if extensions are specified
		if len(fsd.allowedExts) > 0 {
			dropdownY := pathY + 45 // Position below path input

			// Prepare dropdown options
			options := []string{}
			for _, ext := range fsd.allowedExts {
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				options = append(options, "*"+ext)
			}

			fsd.extensionDropdown = NewDropdown(modalBounds.Min.X+15, dropdownY, 100, 30, options, func(selected string) {
				fsd.selectedExt = selected
				fsd.filterFiles()
			})
			// Set container bounds so dropdown knows about modal boundaries
			fsd.extensionDropdown.SetContainerBounds(modalBounds)
			fsd.selectedExt = "All Files"
		}
	}

	// Create action buttons with better spacing
	buttonY := modalBounds.Max.Y - fsd.footerHeight + 55
	actionButtonHeight := 40
	fsd.cancelButton = NewEnhancedButton("Cancel", modalBounds.Max.X-200, buttonY, 90, actionButtonHeight, func() {
		if fsd.onCancelled != nil {
			fsd.onCancelled()
		}
		fsd.Hide()
	})

	confirmText := "Open"
	if fsd.dialogueType == FileDialogueSave {
		confirmText = "Save"
	}
	fsd.confirmButton = NewEnhancedButton(confirmText, modalBounds.Max.X-100, buttonY, 90, actionButtonHeight, func() {
		fsd.confirmSelection()
	})
}

// loadDirectory loads files from the specified directory
func (fsd *FileSystemDialogue) loadDirectory(path string) {
	// Validate and clean the path
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	// Check if directory exists and is readable
	if info, err := os.Stat(cleanPath); err != nil || !info.IsDir() {
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return
	}

	// Clear existing files
	fsd.files = make([]FileItem, 0, len(entries))

	// Add parent directory entry (if not at root)
	if parent := filepath.Dir(cleanPath); parent != cleanPath {
		fsd.files = append(fsd.files, FileItem{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		})
	}

	// Process directory entries
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(cleanPath, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		fileItem := FileItem{
			Name:      entry.Name(),
			Path:      fullPath,
			IsDir:     entry.IsDir(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Extension: ext,
		}

		fsd.files = append(fsd.files, fileItem)
	}

	// Sort files: directories first, then by name
	sort.Slice(fsd.files, func(i, j int) bool {
		if fsd.files[i].IsDir != fsd.files[j].IsDir {
			return fsd.files[i].IsDir
		}
		return strings.ToLower(fsd.files[i].Name) < strings.ToLower(fsd.files[j].Name)
	})

	// Update current path and reset selection
	fsd.currentPath = cleanPath
	fsd.selectedIndex = 0
	fsd.scrollOffset = 0
	fsd.pathInputError = false

	// Update path input text
	if fsd.pathInput != nil {
		fsd.pathInput.SetText(cleanPath)
	}

	// Apply current filter
	fsd.filterFiles()
}

// filterFiles applies the current filter to the file list
func (fsd *FileSystemDialogue) filterFiles() {
	fsd.filteredFiles = make([]FileItem, 0, len(fsd.files))

	for _, file := range fsd.files {
		// Always include directories
		if file.IsDir {
			fsd.filteredFiles = append(fsd.filteredFiles, file)
			continue
		}

		// Apply extension filter if specified
		if fsd.selectedExt != "" && fsd.selectedExt != "All Files" {
			// Extract extension from dropdown selection (e.g., "*.txt" -> ".txt")
			filterExt := strings.TrimPrefix(fsd.selectedExt, "*")
			if filterExt != "" && !strings.EqualFold(file.Extension, filterExt) {
				continue
			}
		}

		fsd.filteredFiles = append(fsd.filteredFiles, file)
	}

	// Reset selection if it's out of bounds
	if fsd.selectedIndex >= len(fsd.filteredFiles) {
		fsd.selectedIndex = 0
	}
	fsd.scrollOffset = 0
}

// Update handles input and updates the dialogue state
func (fsd *FileSystemDialogue) Update() bool {
	if !fsd.IsVisible() {
		return false
	}

	// Update base modal
	fsd.EnhancedModal.Update()

	// Get mouse position
	mx, my := ebiten.CursorPosition()
	modalBounds := fsd.EnhancedModal.GetBounds()

	// Check if mouse is over the modal - if so, block all input
	mouseOverModal := mx >= modalBounds.Min.X && mx <= modalBounds.Max.X &&
		my >= modalBounds.Min.Y && my <= modalBounds.Max.Y

	// ALWAYS block ALL mouse input when modal is visible
	// This completely prevents any clicks from reaching background elements
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) {

		// Handle mouse back button (typically button 3) to close dialogue
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) {
			if fsd.onCancelled != nil {
				fsd.onCancelled()
			}
			fsd.Hide()
			return true
		}

		// Don't process other clicks if mouse is outside modal - just block them
		if !mouseOverModal {
			return true // Consume the input without processing
		}
		// Continue processing click if over modal
	}

	// Block ALL keyboard input - no exceptions
	// Check for any key presses and consume them all
	anyKeyPressed := false
	for key := ebiten.Key0; key <= ebiten.KeyMax; key++ {
		if inpututil.IsKeyJustPressed(key) {
			anyKeyPressed = true
			// Handle specific modal keys
			if key == ebiten.KeyEscape {
				if fsd.onCancelled != nil {
					fsd.onCancelled()
				}
				fsd.Hide()
				return true
			}
			if key == ebiten.KeyEnter && fsd.pathInput.Focused {
				fsd.navigateToPath(fsd.pathInput.GetText())
				return true
			}
			// Let text inputs handle their keys, but still block from background
			break
		}
	}

	// Block character input completely
	inputChars := ebiten.AppendInputChars(nil)
	if len(inputChars) > 0 || anyKeyPressed {
		// Always return true to block ALL input from background
		// Text inputs will still receive their input through their own Update() calls
	}

	// Update UI components (only if mouse is over modal)
	if mouseOverModal {
		fsd.upButton.Update(mx, my)
		fsd.homeButton.Update(mx, my)
		fsd.refreshButton.Update(mx, my)
		fsd.cancelButton.Update(mx, my)
		fsd.confirmButton.Update(mx, my)

		// Update extension dropdown if it exists
		if fsd.extensionDropdown != nil {
			fsd.extensionDropdown.Update(mx, my)
		}

		// Handle focus management for path input
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Check if clicked on path input
			pathClicked := mx >= fsd.pathInput.X && mx < fsd.pathInput.X+fsd.pathInput.Width &&
				my >= fsd.pathInput.Y && my < fsd.pathInput.Y+fsd.pathInput.Height

			// Check if clicked on filename input (if it exists)
			filenameClicked := false
			if fsd.filenameInput != nil {
				filenameClicked = mx >= fsd.filenameInput.X && mx < fsd.filenameInput.X+fsd.filenameInput.Width &&
					my >= fsd.filenameInput.Y && my < fsd.filenameInput.Y+fsd.filenameInput.Height
			}

			// Update focus based on clicks
			if pathClicked {
				if fsd.filenameInput != nil {
					fsd.filenameInput.Focused = false
				}
				fsd.pathInput.Focused = true
				fsd.pathInput.cursorBlink = time.Now()

				// Calculate cursor position based on click location
				clickOffset := mx - fsd.pathInput.X - 8 // Account for text padding
				if clickOffset <= 0 {
					fsd.pathInput.cursorPos = 0
				} else {
					// Load font to calculate precise position
					font := loadWynncraftFont(16)
					if font != nil {
						// Find the character position closest to click
						bestPos := len(fsd.pathInput.Value)
						bestDistance := float64(clickOffset)

						for i := 0; i <= len(fsd.pathInput.Value); i++ {
							textWidth := 0
							if i > 0 {
								bounds := text.BoundString(font, fsd.pathInput.Value[:i])
								textWidth = bounds.Dx()
							}
							distance := float64(clickOffset - textWidth)
							if distance < 0 {
								distance = -distance
							}
							if distance < bestDistance {
								bestDistance = distance
								bestPos = i
							}
						}
						fsd.pathInput.cursorPos = bestPos
					} else {
						// Fallback: estimate based on character width
						charWidth := 8
						fsd.pathInput.cursorPos = clickOffset / charWidth
						if fsd.pathInput.cursorPos > len(fsd.pathInput.Value) {
							fsd.pathInput.cursorPos = len(fsd.pathInput.Value)
						}
					}
				}

				fsd.pathInput.selStart = -1
				fsd.pathInput.selEnd = -1
				fsd.pathInputError = false // Clear error when user starts editing
			} else if filenameClicked && fsd.filenameInput != nil {
				fsd.pathInput.Focused = false
				fsd.filenameInput.Focused = true
				fsd.filenameInput.cursorBlink = time.Now()

				// Calculate cursor position based on click location for filename input
				clickOffset := mx - fsd.filenameInput.X - 8 // Account for text padding
				if clickOffset <= 0 {
					fsd.filenameInput.cursorPos = 0
				} else {
					// Load font to calculate precise position
					font := loadWynncraftFont(16)
					if font != nil {
						// Find the character position closest to click
						bestPos := len(fsd.filenameInput.Value)
						bestDistance := float64(clickOffset)

						for i := 0; i <= len(fsd.filenameInput.Value); i++ {
							textWidth := 0
							if i > 0 {
								bounds := text.BoundString(font, fsd.filenameInput.Value[:i])
								textWidth = bounds.Dx()
							}
							distance := float64(clickOffset - textWidth)
							if distance < 0 {
								distance = -distance
							}
							if distance < bestDistance {
								bestDistance = distance
								bestPos = i
							}
						}
						fsd.filenameInput.cursorPos = bestPos
					} else {
						// Fallback: estimate based on character width
						charWidth := 8
						fsd.filenameInput.cursorPos = clickOffset / charWidth
						if fsd.filenameInput.cursorPos > len(fsd.filenameInput.Value) {
							fsd.filenameInput.cursorPos = len(fsd.filenameInput.Value)
						}
					}
				}

				fsd.filenameInput.selStart = -1
				fsd.filenameInput.selEnd = -1
			} else {
				// Clicked elsewhere in modal - clear focus from text inputs
				fsd.pathInput.Focused = false
				if fsd.filenameInput != nil {
					fsd.filenameInput.Focused = false
				}
			}
		}
	}

	// Always update text inputs as they handle their own focus
	fsd.pathInput.Update()
	if fsd.filenameInput != nil {
		fsd.filenameInput.Update()
	}

	// Handle file list interaction
	if fsd.handleFileListInput(mx, my) {
		return true
	}

	// Handle keyboard navigation
	if fsd.handleKeyboardInput() {
		return true
	}

	// Always return true when modal is visible to block background input
	return true
}

// handleFileListInput handles mouse interaction with the file list
func (fsd *FileSystemDialogue) handleFileListInput(mx, my int) bool {
	modalBounds := fsd.EnhancedModal.GetBounds()

	// Calculate file list area
	listX := modalBounds.Min.X + 10
	listY := modalBounds.Min.Y + fsd.headerHeight
	listWidth := modalBounds.Dx() - 20
	listHeight := modalBounds.Dy() - fsd.headerHeight - fsd.footerHeight

	// Check if mouse is in file list area
	if mx < listX || mx > listX+listWidth || my < listY || my > listY+listHeight {
		return false
	}

	// Handle mouse wheel scrolling
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		fsd.scrollOffset -= int(wheelY * 3)
		fsd.clampScrollOffset()
		return true
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Calculate which item was clicked
		relativeY := my - listY
		clickedIndex := (relativeY / fsd.itemHeight) + fsd.scrollOffset

		if clickedIndex >= 0 && clickedIndex < len(fsd.filteredFiles) {
			currentTime := time.Now()

			// Check for double-click
			if clickedIndex == fsd.lastClickIndex && currentTime.Sub(fsd.lastClickTime) < 500*time.Millisecond {
				fsd.handleDoubleClick(clickedIndex)
			} else {
				fsd.selectedIndex = clickedIndex
			}

			fsd.lastClickIndex = clickedIndex
			fsd.lastClickTime = currentTime
			return true
		}
	}

	return false
}

// handleDoubleClick handles double-click on file list items
func (fsd *FileSystemDialogue) handleDoubleClick(index int) {
	if index < 0 || index >= len(fsd.filteredFiles) {
		return
	}

	file := fsd.filteredFiles[index]

	if file.IsDir {
		// Navigate to directory
		fsd.loadDirectory(file.Path)
	} else {
		// Select file (for open dialogue) or set filename (for save dialogue)
		if fsd.dialogueType == FileDialogueOpen {
			fsd.confirmSelection()
		} else if fsd.filenameInput != nil {
			fsd.filenameInput.SetText(file.Name)
		}
	}
}

// handleKeyboardInput handles keyboard navigation
func (fsd *FileSystemDialogue) handleKeyboardInput() bool {
	// Handle arrow key navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if fsd.selectedIndex > 0 {
			fsd.selectedIndex--
			fsd.ensureVisible()
		}
		return true
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if fsd.selectedIndex < len(fsd.filteredFiles)-1 {
			fsd.selectedIndex++
			fsd.ensureVisible()
		}
		return true
	}

	// Handle Enter key
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if fsd.selectedIndex >= 0 && fsd.selectedIndex < len(fsd.filteredFiles) {
			fsd.handleDoubleClick(fsd.selectedIndex)
		}
		return true
	}

	// Handle Escape key
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if fsd.onCancelled != nil {
			fsd.onCancelled()
		}
		fsd.Hide()
		return true
	}

	return false
}

// ensureVisible ensures the selected item is visible in the scroll area
func (fsd *FileSystemDialogue) ensureVisible() {
	if fsd.selectedIndex < fsd.scrollOffset {
		fsd.scrollOffset = fsd.selectedIndex
	} else if fsd.selectedIndex >= fsd.scrollOffset+fsd.maxVisibleItems {
		fsd.scrollOffset = fsd.selectedIndex - fsd.maxVisibleItems + 1
	}
	fsd.clampScrollOffset()
}

// clampScrollOffset ensures scroll offset is within valid bounds
func (fsd *FileSystemDialogue) clampScrollOffset() {
	maxScroll := len(fsd.filteredFiles) - fsd.maxVisibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}

	if fsd.scrollOffset < 0 {
		fsd.scrollOffset = 0
	} else if fsd.scrollOffset > maxScroll {
		fsd.scrollOffset = maxScroll
	}
}

// navigateToPath attempts to navigate to the specified path
func (fsd *FileSystemDialogue) navigateToPath(path string) {
	// Clean and validate the path
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		fsd.pathInputError = true
		return
	}

	// Check if directory exists and is readable
	if info, err := os.Stat(cleanPath); err != nil || !info.IsDir() {
		fsd.pathInputError = true
		return
	}

	// Path is valid, navigate to it
	fsd.loadDirectory(cleanPath)
}

// confirmSelection confirms the current selection
func (fsd *FileSystemDialogue) confirmSelection() {
	var selectedPath string

	if fsd.dialogueType == FileDialogueSave {
		// For save dialogue, use filename input
		if fsd.filenameInput != nil && fsd.filenameInput.GetText() != "" {
			selectedPath = filepath.Join(fsd.currentPath, fsd.filenameInput.GetText())
		}
	} else {
		// For open dialogue, use selected file
		if fsd.selectedIndex >= 0 && fsd.selectedIndex < len(fsd.filteredFiles) {
			file := fsd.filteredFiles[fsd.selectedIndex]
			if !file.IsDir {
				selectedPath = file.Path
			}
		}
	}

	// Validate selection
	if selectedPath == "" {
		return
	}

	// If selected path is a directory, navigate into it
	if info, err := os.Stat(selectedPath); err == nil && info.IsDir() {
		fsd.loadDirectory(selectedPath)
		return
	}

	// But if it isn't ended with an extension, add the default extension of .lz4
	if fsd.dialogueType == FileDialogueSave && !strings.HasSuffix(selectedPath, ".lz4") {
		selectedPath += ".lz4"
	}

	// Check file extension if filters are specified
	if len(fsd.allowedExts) > 0 {
		ext := strings.ToLower(filepath.Ext(selectedPath))
		allowed := false
		for _, allowedExt := range fsd.allowedExts {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}

	// Call callback and hide dialogue
	if fsd.onFileSelected != nil {
		fsd.onFileSelected(selectedPath)
	}
	fsd.Hide()
}

// Draw renders the file system dialogue
func (fsd *FileSystemDialogue) Draw(screen *ebiten.Image) {
	if !fsd.IsVisible() {
		return
	}

	// Draw base modal
	fsd.EnhancedModal.Draw(screen)

	modalBounds := fsd.EnhancedModal.GetBounds()

	// Draw path input with error highlighting
	if fsd.pathInputError {
		// Draw red border for invalid path
		pathBounds := image.Rect(fsd.pathInput.X-2, fsd.pathInput.Y-2,
			fsd.pathInput.X+fsd.pathInput.Width+2,
			fsd.pathInput.Y+fsd.pathInput.Height+2)
		ebitenutil.DrawRect(screen, float64(pathBounds.Min.X), float64(pathBounds.Min.Y),
			float64(pathBounds.Dx()), float64(pathBounds.Dy()),
			color.RGBA{255, 100, 100, 255})
	}
	fsd.pathInput.Draw(screen)

	// Draw navigation buttons
	fsd.upButton.Draw(screen)
	fsd.homeButton.Draw(screen)
	fsd.refreshButton.Draw(screen)

	// Draw file list
	fsd.drawFileList(screen, modalBounds)

	// Draw filename input (for save dialogue)
	if fsd.filenameInput != nil {
		fsd.filenameInput.Draw(screen)
	}

	// Draw action buttons
	fsd.cancelButton.Draw(screen)
	fsd.confirmButton.Draw(screen)

	// Draw extension dropdown LAST so it appears on top of everything else
	if fsd.extensionDropdown != nil {
		fsd.extensionDropdown.Draw(screen)
	}
}

// drawFileList renders the file list area
func (fsd *FileSystemDialogue) drawFileList(screen *ebiten.Image, modalBounds image.Rectangle) {
	listX := modalBounds.Min.X + 10
	listY := modalBounds.Min.Y + fsd.headerHeight
	listWidth := modalBounds.Dx() - 20
	listHeight := modalBounds.Dy() - fsd.headerHeight - fsd.footerHeight

	// Draw list background
	ebitenutil.DrawRect(screen, float64(listX), float64(listY), float64(listWidth), float64(listHeight),
		color.RGBA{30, 30, 30, 255})

	// Draw list border
	ebitenutil.DrawRect(screen, float64(listX-1), float64(listY-1), float64(listWidth+2), float64(listHeight+2),
		color.RGBA{100, 100, 100, 255})
	ebitenutil.DrawRect(screen, float64(listX), float64(listY), float64(listWidth), float64(listHeight),
		color.RGBA{30, 30, 30, 255})

	// Calculate visible items based on filtered files
	startIndex := fsd.scrollOffset
	endIndex := startIndex + fsd.maxVisibleItems
	if endIndex > len(fsd.filteredFiles) {
		endIndex = len(fsd.filteredFiles)
	}

	font := loadWynncraftFont(18) // Doubled from 11 to 18 for larger text

	// Draw visible files
	for i := startIndex; i < endIndex; i++ {
		file := fsd.filteredFiles[i]
		y := listY + (i-startIndex)*fsd.itemHeight

		// Draw selection highlight
		if i == fsd.selectedIndex {
			ebitenutil.DrawRect(screen, float64(listX), float64(y), float64(listWidth), float64(fsd.itemHeight),
				color.RGBA{100, 150, 255, 100})
		}

		// Choose icon and color based on file type - using regular characters
		icon := "[F]" // File
		textColor := color.RGBA{200, 200, 200, 255}
		if file.IsDir {
			icon = "[D]" // Directory
			textColor = color.RGBA{255, 255, 150, 255}
		} else if file.Name == ".." {
			icon = "[^]" // Parent directory
			textColor = color.RGBA{150, 150, 255, 255}
		}

		// Draw file item with better positioning
		itemText := fmt.Sprintf("%s %s", icon, file.Name)
		drawTextWithOffset(screen, itemText, font, listX+10, y+fsd.itemHeight/2+6, textColor)

		// Draw file size for regular files with better positioning
		if !file.IsDir && file.Name != ".." {
			sizeText := formatFileSize(file.Size)
			drawTextWithOffset(screen, sizeText, font, listX+listWidth-120, y+fsd.itemHeight/2+6,
				color.RGBA{150, 150, 150, 255})
		}
	}

	// Draw scrollbar if needed
	if len(fsd.filteredFiles) > fsd.maxVisibleItems {
		fsd.drawScrollbar(screen, listX+listWidth-10, listY, 10, listHeight)
	}
}

// drawScrollbar draws a vertical scrollbar
func (fsd *FileSystemDialogue) drawScrollbar(screen *ebiten.Image, x, y, width, height int) {
	// Draw scrollbar background
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(width), float64(height),
		color.RGBA{50, 50, 50, 255})

	// Calculate thumb size and position
	thumbHeight := (height * fsd.maxVisibleItems) / len(fsd.filteredFiles)
	if thumbHeight < 20 {
		thumbHeight = 20
	}

	thumbY := y + (height-thumbHeight)*fsd.scrollOffset/(len(fsd.filteredFiles)-fsd.maxVisibleItems)

	// Draw scrollbar thumb
	ebitenutil.DrawRect(screen, float64(x+1), float64(thumbY), float64(width-2), float64(thumbHeight),
		color.RGBA{150, 150, 150, 255})
}

// formatFileSize formats file size in human-readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// SetOnFileSelected sets the callback for when a file is selected
func (fsd *FileSystemDialogue) SetOnFileSelected(callback func(string)) {
	fsd.onFileSelected = callback
}

// SetOnCancelled sets the callback for when the dialogue is cancelled
func (fsd *FileSystemDialogue) SetOnCancelled(callback func()) {
	fsd.onCancelled = callback
}

// SetCurrentPath sets the current directory path
func (fsd *FileSystemDialogue) SetCurrentPath(path string) {
	fsd.loadDirectory(path)
}

// GetCurrentPath returns the current directory path
func (fsd *FileSystemDialogue) GetCurrentPath() string {
	return fsd.currentPath
}
