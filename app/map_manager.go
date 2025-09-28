package app

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"RueaES/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// MapInfo represents the overall map configuration
type MapInfo struct {
	Width  int // Map width in pixels
	Height int // Map height in pixels
}

// MapManager handles loading and managing the local map image
type MapManager struct {
	mapImage  *ebiten.Image
	mapInfo   *MapInfo
	isLoaded  bool
	loadError error
}

// NewMapManager creates a new map manager instance
func NewMapManager() *MapManager {
	return &MapManager{
		isLoaded: false,
	}
}

// LoadMapAsync loads map data asynchronously in a separate goroutine
func (mm *MapManager) LoadMapAsync() {
	go func() {
		if err := mm.LoadMapData(); err != nil {
			fmt.Printf("Error loading map data: %v\n", err)
		}
	}()
}

// LoadMapData loads the local map image
func (mm *MapManager) LoadMapData() error {
	fmt.Println("Loading local map image...")

	// Load the embedded map image
	file, err := assets.AssetFiles.Open("main-map.png")
	if err != nil {
		mm.loadError = fmt.Errorf("failed to open embedded map file: %v", err)
		return mm.loadError
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		mm.loadError = fmt.Errorf("failed to decode map image: %v", err)
		return mm.loadError
	}

	// Convert to Ebiten image
	mm.mapImage = ebiten.NewImageFromImage(img)

	// Create map info
	bounds := img.Bounds()
	mm.mapInfo = &MapInfo{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
	}

	mm.isLoaded = true
	fmt.Printf("Map loaded successfully: %dx%d pixels\n", mm.mapInfo.Width, mm.mapInfo.Height)

	return nil
}

// GetMapImage returns the loaded map image
func (mm *MapManager) GetMapImage() *ebiten.Image {
	return mm.mapImage
}

// IsLoaded returns whether the map data has been loaded
func (mm *MapManager) IsLoaded() bool {
	return mm.isLoaded
}

// GetLoadError returns any error that occurred during loading
func (mm *MapManager) GetLoadError() error {
	return mm.loadError
}

// GetMapInfo returns the loaded map information
func (mm *MapManager) GetMapInfo() *MapInfo {
	return mm.mapInfo
}

// Cleanup clears any resources (not needed for single image, but kept for compatibility)
func (mm *MapManager) Cleanup() {
	// Nothing to cleanup for single image
}
