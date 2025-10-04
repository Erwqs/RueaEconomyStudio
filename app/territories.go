//lint:file-ignore SA1019 using deprecated text package for Draw
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"RueaES/assets"
	"RueaES/eruntime" // Add eruntime import
	"RueaES/typedef"

	"github.com/hajimehoshi/ebiten/v2"
)

// Constants for game to pixel coordinate conversion offsets
const (
	OffsetX float64 = 2382
	OffsetY float64 = 6572
)

// fontYOffset compensates for broken font being too high
const fontYOffset = 12

// getGuildColor generates a consistent color for a guild based on its name
func getGuildColor(guildName string) [3]float32 {
	if guildName == "" {
		return [3]float32{1, 1, 1} // White for no guild
	}

	// Simple hash function to generate consistent colors
	hash := uint32(0)
	for _, c := range guildName {
		hash = hash*31 + uint32(c)
	}

	// Generate RGB components with good color distribution
	// Ensure colors are bright enough to be visible
	r := float32((hash&0xFF))/255.0*0.7 + 0.3       // 0.3-1.0 range
	g := float32(((hash>>8)&0xFF))/255.0*0.7 + 0.3  // 0.3-1.0 range
	b := float32(((hash>>16)&0xFF))/255.0*0.7 + 0.3 // 0.3-1.0 range

	return [3]float32{r, g, b}
}

// UI constants for sliders and inputs
const (
	SliderWidth  = 280 // Increased to take most of sidebar width
	SliderHeight = 20
	InputWidth   = 60
	InputHeight  = 25
	ButtonSize   = 20
)

// Upgrade data structure loaded from upgrades.json
type UpgradeData struct {
	UpgradesCost map[string]struct {
		Value        []int  `json:"value"`
		ResourceType string `json:"resourceType"`
	} `json:"upgradesCost"`
	UpgradeMultiplier map[string][]float64   `json:"upgradeMultiplier"`
	UpgradeBaseStats  map[string]interface{} `json:"upgradeBaseStats"`
	Bonuses           map[string]struct {
		MaxLevel     int           `json:"maxLevel"`
		Cost         []int         `json:"cost"`
		ResourceType string        `json:"resourceType"`
		Value        []interface{} `json:"value"`
	} `json:"bonuses"`
}

// InputBox represents a text input field for numeric values
type InputBox struct {
	Value    string
	Active   bool
	Rect     image.Rectangle
	MaxValue int
	MinValue int
}

// TowerUpgrades holds the current levels for all tower upgrades
type TowerUpgrades struct {
	Damage  int `json:"damage"`
	Attack  int `json:"attack"`
	Health  int `json:"health"`
	Defence int `json:"defence"`

	// Bonus upgrades
	StrongerMinions       int `json:"strongerMinions"`
	TowerMultiAttack      int `json:"towerMultiAttack"`
	TowerAura             int `json:"towerAura"`
	TowerVolley           int `json:"towerVolley"`
	XpSeeking             int `json:"xpSeeking"`
	TomeSeeking           int `json:"tomeSeeking"`
	EmeraldsSeeking       int `json:"emeraldsSeeking"`
	LargerResourceStorage int `json:"largerResourceStorage"`
	LargerEmeraldsStorage int `json:"largerEmeraldsStorage"`
	EfficientResource     int `json:"efficientResource"`
	EfficientEmeralds     int `json:"efficientEmeralds"`
	ResourceRate          int `json:"resourceRate"`
	EmeraldsRate          int `json:"emeraldsRate"`
}

// Location represents a territory's location coordinates
type Location struct {
	Start []float64 `json:"start"`
	End   []float64 `json:"end"`
}

// Resource information for territories
type Resources struct {
	Emeralds string `json:"emeralds"`
	Ore      string `json:"ore"`
	Crops    string `json:"crops"`
	Fish     string `json:"fish"`
	Wood     string `json:"wood"`
}

// Guild information for territories
type Guild struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

// TerritoryClaim represents actual territory ownership data
type TerritoryClaim struct {
	Territory string `json:"territory"`
	GuildName string `json:"guild_name"`
	GuildTag  string `json:"guild_tag"`
}

// TerritoryState represents a snapshot of territory configuration for backup/restore
type TerritoryState struct {
	// Tower upgrades
	Upgrades struct {
		Damage  int `json:"damage"`
		Attack  int `json:"attack"`
		Health  int `json:"health"`
		Defence int `json:"defence"`
	} `json:"upgrades"`

	// Bonus upgrades
	Bonuses map[string]int `json:"bonuses"`

	// Tax settings
	Taxes struct {
		TaxRate     int `json:"taxRate"`
		AllyTaxRate int `json:"allyTaxRate"`
	} `json:"taxes"`

	// Border settings
	BorderSettings struct {
		RoutingStyle string `json:"routingStyle"`
		BorderState  string `json:"borderState"`
	} `json:"borderSettings"`

	// HQ status
	IsHQ bool `json:"isHQ"`
}

// Territory represents a territory in the game
type Territory struct {
	Resources     Resources `json:"resources"`
	TradingRoutes []string  `json:"Trading Routes"`
	Location      Location  `json:"Location"`
	Guild         Guild     `json:"Guild"`
	Acquired      string    `json:"Acquired"`

	// Per-territory upgrade levels (Guild Tower upgrades)
	Upgrades struct {
		Damage  int `json:"damage"`
		Attack  int `json:"attack"`
		Health  int `json:"health"`
		Defence int `json:"defence"`
	} `json:"upgrades"`

	// Per-territory bonus levels
	Bonuses map[string]int `json:"bonuses"`

	// Per-territory taxes
	Taxes struct {
		TaxRate     int `json:"taxRate"`
		AllyTaxRate int `json:"allyTaxRate"`
	} `json:"taxes"`

	// Per-territory border settings
	BorderSettings struct {
		RoutingStyle string `json:"routingStyle"`
		BorderState  string `json:"borderState"`
	} `json:"borderSettings"`

	// HQ status
	isHQ bool // Flag to indicate if this territory is the HQ for its guild
}

// Animation structures for smooth UI interactions

// SliderAnimation represents an animated slider position
type SliderAnimation struct {
	StartValue   float64       // Starting slider position (0.0 to 1.0)
	TargetValue  float64       // Target slider position (0.0 to 1.0)
	CurrentValue float64       // Current animated position (0.0 to 1.0)
	StartTime    time.Time     // When animation started
	Duration     time.Duration // How long the animation should take
	IsAnimating  bool          // Whether animation is currently active
}

// ToggleAnimation represents an animated toggle switch
type ToggleAnimation struct {
	StartPosition   float64       // Starting knob position (0.0 to 1.0)
	TargetPosition  float64       // Target knob position (0.0 to 1.0)
	CurrentPosition float64       // Current animated position (0.0 to 1.0)
	StartTime       time.Time     // When animation started
	Duration        time.Duration // How long the animation should take
	IsAnimating     bool          // Whether animation is currently active
}

// TerritoriesManager handles loading and displaying territories
type TerritoriesManager struct {
	Territories      map[string]Territory
	TerritoryBorders map[string][4]float64 // [x1, y1, x2, y2] in pixel coordinates
	BorderColor      color.RGBA
	RouteColor       color.RGBA
	FillColor        color.RGBA // Added for territory box fill
	BorderThickness  float64
	RouteThickness   float64
	isLoaded         bool
	loadError        error

	// Territory selection and UI
	selectedTerritory  string
	sideMenuOpen       bool
	sideMenuWidth      float64
	lastClickTime      time.Time
	lastClickTerritory string
	blinkTimer         float64
	isBlinking         bool

	// Animation for side menu
	menuAnimating    bool
	menuAnimProgress float64
	menuAnimSpeed    float64
	menuTargetOpen   bool

	// Offscreen buffer system
	offscreenBuffer     *ebiten.Image
	bufferMutex         sync.RWMutex
	bufferScale         float64
	bufferViewX         float64
	bufferViewY         float64
	bufferWidth         int
	bufferHeight        int
	bufferNeedsUpdate   bool
	bufferUpdateTicker  *time.Ticker
	bufferStopChan      chan bool
	bufferUpdateRunning bool

	// Spatial partitioning grid for fast culling
	grid         map[[2]int][]string // cell -> territory names
	gridCellSize float64
	gridMinX     float64
	gridMinY     float64
	gridMaxX     float64
	gridMaxY     float64
	gridCellsX   int
	gridCellsY   int

	// Collapsible sections state
	baseResourcesCollapsed   bool // Renamed from resourcesCollapsed
	connectionsCollapsed     bool // Renamed from tradingRoutesCollapsed
	guildTowerCollapsed      bool
	bonusesCollapsed         bool
	totalCostCollapsed       bool            // New section
	resourcesCollapsed       bool            // New detailed resources section
	tradingRoutesCollapsed   bool            // New trading routes section
	bordersRoutingCollapsed  bool            // New borders and routing section
	taxesCollapsed           bool            // New taxes section
	baseResourcesHeaderRect  image.Rectangle // Renamed from resourcesHeaderRect
	connectionsHeaderRect    image.Rectangle // Renamed from tradingHeaderRect
	guildTowerHeaderRect     image.Rectangle
	bonusesHeaderRect        image.Rectangle
	totalCostHeaderRect      image.Rectangle // New section
	resourcesHeaderRect      image.Rectangle // New detailed resources section
	tradingRoutesHeaderRect  image.Rectangle // New trading routes section
	bordersRoutingHeaderRect image.Rectangle // New borders and routing section
	taxesHeaderRect          image.Rectangle // New taxes section
	tradingRouteRects        []struct {
		name string
		rect image.Rectangle
	} // clickable trading routes
	connectedTerritoryRects []struct {
		name string
		rect image.Rectangle
	} // clickable connected territories
	pendingRoute string

	// Set as HQ button
	setAsHQButtonRect image.Rectangle
	isHQ              bool // Flag to track if this territory is the HQ

	// Bottom action buttons
	applyButtonRect   image.Rectangle
	loadoutButtonRect image.Rectangle
	revertButtonRect  image.Rectangle

	// Taxes input fields
	taxInput     InputBox
	allyTaxInput InputBox

	// Borders and Routing settings
	styleToggleRect  image.Rectangle // Rectangle for the routing style toggle switch
	borderToggleRect image.Rectangle // Rectangle for the border state toggle switch

	// Upgrade data loaded from JSON
	upgradeData *UpgradeData

	// UI state for sliders and text inputs
	activeTextInput     string            // which text input is currently active
	textInputValues     map[string]string // temporary text values during editing
	upgradeSliders      map[string]image.Rectangle
	upgradeInputs       map[string]image.Rectangle
	upgradeMinusButtons map[string]image.Rectangle
	upgradePlusButtons  map[string]image.Rectangle
	// Slider dragging state
	isDraggingSlider     bool              // whether a slider is currently being dragged
	dragUpgradeType      string            // which upgrade slider is being dragged
	dragTempValue        float64           // temporary continuous value while dragging (0.0 to 1.0)
	pendingSliderTargets map[string]int    // target levels for pending slider animations
	pendingToggleTargets map[string]string // target values for pending toggle animations

	// Animation state for UI elements
	sliderAnimations    map[string]SliderAnimation // slider position animations
	toggleAnimations    map[string]ToggleAnimation // toggle switch animations
	animationUpdateTime time.Time                  // last animation update time
	animationMutex      sync.RWMutex               // protects animation maps from concurrent access

	// Territory state management for Apply/Loadout/Revert functionality
	appliedStates map[string]TerritoryState // backup of applied territory states
	loadoutPath   string                    // current loadout file path

	// Territory data protection
	territoryMutex sync.RWMutex // protects Territories map from concurrent access

	// Territory claims mapping
	territoryClaims map[string]TerritoryClaim // Maps territory name -> guild claim
	claimsMutex     sync.RWMutex              // protects territory claims data

	// Side Menu Offscreen Buffer
	sideMenuBuffer            *ebiten.Image
	sideMenuContentHeight     int  // Total height of the drawn content in the side menu buffer
	sideMenuBufferNeedsUpdate bool // Flag to trigger full redraw of sideMenuBuffer content

	// Guild management
	guildManager *EnhancedGuildManager

	// Territory renderer for true transparency
	territoryRenderer *TerritoryRenderer

	// GPU overlay renderer (new)
	overlayGPU *TerritoryOverlayGPU

	// HQ icon overlay
	hqImage *ebiten.Image

	// Performance caches
	guildColorCache map[string][3]float32 // Cache for guild colors to avoid recalculation
	cacheLastFrame  int64                 // Frame number for cache invalidation

	// Data refresh timer for real-time updates
	dataRefreshTimer    time.Time
	dataRefreshInterval time.Duration

	// Route highlighting configuration
	showHoveredRoutes bool // Enable/disable white route highlighting for hovered territories
}

// NewTerritoriesManager creates a new territories manager
func NewTerritoriesManager() *TerritoriesManager {

	tm := &TerritoriesManager{
		Territories:         make(map[string]Territory),
		TerritoryBorders:    make(map[string][4]float64),
		BorderColor:         color.RGBA{R: 255, G: 255, B: 255, A: 180}, // 70% opacity for borders
		RouteColor:          color.RGBA{R: 255, G: 100, B: 100, A: 220}, // Improved visibility: bright red with high opacity
		FillColor:           color.RGBA{R: 255, G: 255, B: 255, A: 60},  // 23% opacity for fills
		BorderThickness:     4.0,
		RouteThickness:      3.0, // Increased from 2.0 to 3.0 for better visibility
		isLoaded:            false,
		bufferNeedsUpdate:   true,
		bufferUpdateRunning: false,
		bufferStopChan:      make(chan bool),

		// Initialize territory selection fields
		selectedTerritory:  "",
		sideMenuOpen:       false,
		sideMenuWidth:      400, // Increased from 300 to 400 for wider menu
		lastClickTime:      time.Time{},
		lastClickTerritory: "",
		blinkTimer:         0,
		isBlinking:         false,

		// Initialize animation fields
		menuAnimating:    false,
		menuAnimProgress: 0,
		menuAnimSpeed:    5.0, // Animation speed (higher = faster) - made 2x slower
		menuTargetOpen:   false,

		// Initialize collapsible sections state
		baseResourcesCollapsed:  false,
		connectionsCollapsed:    false,
		guildTowerCollapsed:     true,
		bonusesCollapsed:        true,
		totalCostCollapsed:      false,
		resourcesCollapsed:      false,
		tradingRoutesCollapsed:  false,
		bordersRoutingCollapsed: false,
		taxesCollapsed:          false,

		// Initialize tax inputs with minimum 5%
		taxInput: InputBox{
			Value:    "5",
			Active:   false,
			MinValue: 5,
			MaxValue: 70,
		},
		allyTaxInput: InputBox{
			Value:    "5",
			Active:   false,
			MinValue: 5,
			MaxValue: 70,
		},

		// Initialize upgrade fields
		textInputValues: make(map[string]string),

		// Initialize UI element maps
		upgradeSliders:      make(map[string]image.Rectangle),
		upgradeInputs:       make(map[string]image.Rectangle),
		upgradeMinusButtons: make(map[string]image.Rectangle),
		upgradePlusButtons:  make(map[string]image.Rectangle),

		// Initialize animation maps
		sliderAnimations:     make(map[string]SliderAnimation),
		toggleAnimations:     make(map[string]ToggleAnimation),
		animationUpdateTime:  time.Now(),
		pendingSliderTargets: make(map[string]int),
		pendingToggleTargets: make(map[string]string),

		// Initialize territory state management
		appliedStates: make(map[string]TerritoryState),
		loadoutPath:   "",

		// Initialize territory claims mapping
		territoryClaims: make(map[string]TerritoryClaim),

		// Initialize Side Menu Offscreen Buffer fields
		sideMenuBuffer:            nil, // Will be initialized on first DrawSideMenu call
		sideMenuContentHeight:     0,
		sideMenuBufferNeedsUpdate: true,

		// Initialize data refresh timer for real-time updates
		dataRefreshTimer:    time.Now(),
		dataRefreshInterval: 1 * time.Second, // Refresh every second for real-time feel

		// Initialize performance caches
		guildColorCache: make(map[string][3]float32),
		cacheLastFrame:  0,

		// Initialize route highlighting
		showHoveredRoutes: true, // Enable by default
	}

	// Initialize territory renderer
	tm.territoryRenderer = NewTerritoryRenderer(tm)

	// Initialize GPU overlay renderer
	overlayGPU, err := NewTerritoryOverlayGPU()
	if err != nil {
		panic("Failed to initialize TerritoryOverlayGPU: " + err.Error())
	}
	tm.overlayGPU = overlayGPU

	// Load HQ icon image
	tm.loadHQImage()

	// Start the buffer update goroutine
	tm.startBufferUpdateLoop()

	// Load upgrade data
	go tm.loadUpgradeData()

	return tm
}

// loadTerritoryClaims loads territory claims data from territory_claims.json
func (tm *TerritoriesManager) loadTerritoryClaims() error {
	tm.claimsMutex.Lock()
	defer tm.claimsMutex.Unlock()

	// Check if we're running in WASM
	if runtime.GOOS == "js" && runtime.GOARCH == "wasm" {
		// Use memory storage for WASM
		claims, err := loadTerritoryClaimsFromMemory()
		if err != nil {
			// fmt.Printf("Warning: Could not load territory claims from memory: %v\n", err)
			return nil
		}
		tm.territoryClaims = claims
		// fmt.Printf("[TERRITORY_STORAGE] Loaded %d territory claims from WASM memory\n", len(claims))
		return nil
	}

	// Read territory_claims.json file (non-WASM)
	data, err := os.ReadFile("territory_claims.json")
	if err != nil {
		// File doesn't exist or can't be read - not a fatal error
		// fmt.Printf("Warning: Could not load territory_claims.json: %v\n", err)
		return nil
	}

	// Parse JSON array
	var claims []TerritoryClaim
	if err := json.Unmarshal(data, &claims); err != nil {
		// fmt.Printf("Warning: Could not parse territory_claims.json: %v\n", err)
		return nil
	}

	// Build the mapping
	tm.territoryClaims = make(map[string]TerritoryClaim)

	// Prepare batch guild updates for eruntime
	guildUpdates := make(map[string]*typedef.Guild)

	for _, claim := range claims {
		tm.territoryClaims[claim.Territory] = claim

		// Get the current territory state from eruntime
		currentTerritory := eruntime.GetTerritory(claim.Territory)

		// Only synchronize if there's a difference in guild ownership
		needsSync := false

		if currentTerritory != nil {
			currentTerritory.Mu.RLock()
			currentGuildName := currentTerritory.Guild.Name
			currentGuildTag := currentTerritory.Guild.Tag
			currentTerritory.Mu.RUnlock()

			// Check if the guild ownership is different
			if currentGuildName != claim.GuildName || currentGuildTag != claim.GuildTag {
				needsSync = true
			}
		} else {
			// Territory doesn't exist in eruntime, so we need to sync it
			needsSync = true
		}

		if needsSync {
			// Prepare guild for batch update
			guild := &typedef.Guild{
				Name:   claim.GuildName,
				Tag:    claim.GuildTag,
				Allies: nil,
			}
			guildUpdates[claim.Territory] = guild
		}
	}

	// Synchronize all claims with the eruntime system in a single batch operation
	syncCount := 0
	if len(guildUpdates) > 0 {
		updatedTerritories := eruntime.SetGuildBatch(guildUpdates)
		syncCount = len(updatedTerritories)

		// fmt.Printf("[TERRITORIES] Batch synchronized %d/%d claims with eruntime\n",
		// syncCount, len(guildUpdates))

		// Log any failures
		if syncCount < len(guildUpdates) {
			for territoryName := range guildUpdates {
				found := false
				for _, updated := range updatedTerritories {
					if updated.Name == territoryName {
						found = true
						break
					}
				}
				if !found {
					// fmt.Printf("[TERRITORIES] Warning: Failed to sync claim %s with eruntime\n", territoryName)
				}
			}
		}
	}

	// fmt.Printf("Loaded %d territory claims, batch synchronized %d with eruntime\n", len(tm.territoryClaims), syncCount)
	return nil
}

// ReloadClaims reloads territory claims from the persistent storage file
func (tm *TerritoriesManager) ReloadClaims() error {
	// // fmt.Println("[DEBUG] TerritoriesManager.ReloadClaims called")
	return tm.loadTerritoryClaims()
}

// getActualGuildForTerritory returns the actual guild ownership for a territory
func (tm *TerritoriesManager) getActualGuildForTerritory(territoryName string) (string, string) {
	tm.claimsMutex.RLock()
	defer tm.claimsMutex.RUnlock()

	if claim, exists := tm.territoryClaims[territoryName]; exists {
		return claim.GuildName, claim.GuildTag
	}
	return "", ""
}

// getRouteToHQForTerritory returns the trading route from a territory to its HQ
// Returns nil if territory has no route to HQ, is an HQ itself, or has no guild
func (tm *TerritoriesManager) getRouteToHQForTerritory(territoryName string) []string {
	// Get territory from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return nil
	}

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	// Skip if territory is an HQ or has no guild
	if territory.HQ || territory.Guild.Tag == "" || territory.Guild.Tag == "NONE" {
		return nil
	}

	// Skip if territory has no trading routes
	if len(territory.TradingRoutes) == 0 {
		return nil
	}

	// Return the first route (there should only be one route from territory to HQ)
	route := territory.TradingRoutes[0]
	routeNames := make([]string, len(route))
	for i, t := range route {
		if t != nil {
			routeNames[i] = t.Name
		}
	}

	return routeNames
}

// LoadTerritoriesAsync loads territory data asynchronously
func (tm *TerritoriesManager) LoadTerritoriesAsync() {
	go func() {
		if err := tm.LoadTerritories(); err != nil {
			// Consider a more robust logging mechanism or error propagation if needed
			// fmt.Printf("Error loading territories data asynchronously: %v\n", err)
			// Potentially set tm.loadError here, carefully considering concurrency
		}
	}()
}

// LoadTerritories loads territory data from the JSON file
func (tm *TerritoriesManager) LoadTerritories() error {
	// Read and parse the JSON file
	if err := tm.readAndParseTerritoriesJSON(); err != nil {
		return err
	}

	// Load territory claims data
	if err := tm.loadTerritoryClaims(); err != nil {
		// This is not fatal, continue with empty claims
		// fmt.Printf("Warning: Failed to load territory claims: %v\n", err)
	}

	// Process territory coordinates and initialize defaults
	tm.processTerritoriesBorders()
	tm.initializeAllTerritories()

	// Build spatial partitioning grid for efficient rendering
	tm.buildSpatialGrid()

	// fmt.Printf("Loaded %d territories, %d with valid borders\n", len(tm.Territories), len(tm.TerritoryBorders))
	tm.isLoaded = true
	tm.loadError = nil // Clear any previous error on success
	return nil
}

// readAndParseTerritoriesJSON loads territories from eruntime instead of JSON file
func (tm *TerritoriesManager) readAndParseTerritoriesJSON() error {
	// Get territories from eruntime
	eruntimeTerritories := eruntime.GetTerritories()

	// Initialize the territories map
	tm.Territories = make(map[string]Territory)

	// Convert eruntime territories to app Territory format
	for _, eruntimeTerritory := range eruntimeTerritories {
		if eruntimeTerritory == nil {
			continue
		}

		eruntimeTerritory.Mu.RLock()

		// Convert location coordinates
		location := Location{
			Start: []float64{float64(eruntimeTerritory.Location.Start[0]), float64(eruntimeTerritory.Location.Start[1])},
			End:   []float64{float64(eruntimeTerritory.Location.End[0]), float64(eruntimeTerritory.Location.End[1])},
		}

		// Convert resources (note: eruntime uses different structure)
		resources := Resources{
			Emeralds: fmt.Sprintf("%d", eruntimeTerritory.ResourceGeneration.Base.Emeralds),
			Ore:      fmt.Sprintf("%d", eruntimeTerritory.ResourceGeneration.Base.Ores),
			Wood:     fmt.Sprintf("%d", eruntimeTerritory.ResourceGeneration.Base.Wood),
			Fish:     fmt.Sprintf("%d", eruntimeTerritory.ResourceGeneration.Base.Fish),
			Crops:    fmt.Sprintf("%d", eruntimeTerritory.ResourceGeneration.Base.Crops),
		}

		// Convert guild information
		guild := Guild{
			UUID:   "", // Not available in eruntime
			Name:   eruntimeTerritory.Guild.Name,
			Prefix: eruntimeTerritory.Guild.Tag,
		}

		// Get trading routes for this territory
		territoryRoutes := eruntime.GetTradingRoutesForTerritory(eruntimeTerritory.Name)

		// Create the app Territory struct
		territory := Territory{
			Resources:     resources,
			TradingRoutes: territoryRoutes,
			Location:      location,
			Guild:         guild,
			Acquired:      "", // Not available in eruntime
			isHQ:          eruntimeTerritory.HQ,
		}

		// Convert upgrades
		territory.Upgrades.Damage = int(eruntimeTerritory.Options.Upgrade.Set.Damage)
		territory.Upgrades.Attack = int(eruntimeTerritory.Options.Upgrade.Set.Attack)
		territory.Upgrades.Health = int(eruntimeTerritory.Options.Upgrade.Set.Health)
		territory.Upgrades.Defence = int(eruntimeTerritory.Options.Upgrade.Set.Defence)

		// Convert bonuses (using correct field names)
		territory.Bonuses = make(map[string]int)
		territory.Bonuses["strongerMinions"] = int(eruntimeTerritory.Options.Bonus.Set.StrongerMinions)
		territory.Bonuses["towerMultiAttack"] = int(eruntimeTerritory.Options.Bonus.Set.TowerMultiAttack)
		territory.Bonuses["towerAura"] = int(eruntimeTerritory.Options.Bonus.Set.TowerAura)
		territory.Bonuses["towerVolley"] = int(eruntimeTerritory.Options.Bonus.Set.TowerVolley)
		territory.Bonuses["xpSeeking"] = int(eruntimeTerritory.Options.Bonus.Set.XPSeeking)
		territory.Bonuses["tomeSeeking"] = int(eruntimeTerritory.Options.Bonus.Set.TomeSeeking)
		territory.Bonuses["emeraldSeeking"] = int(eruntimeTerritory.Options.Bonus.Set.EmeraldSeeking)

		// Convert taxes (using correct field names)
		territory.Taxes.TaxRate = int(eruntimeTerritory.Tax.Tax * 100)      // Convert to percentage
		territory.Taxes.AllyTaxRate = int(eruntimeTerritory.Tax.Ally * 100) // Convert to percentage

		// Convert border settings
		territory.BorderSettings.RoutingStyle = "cheapest" // Default, could map from eruntime.RoutingMode
		if eruntimeTerritory.RoutingMode == 1 {
			territory.BorderSettings.RoutingStyle = "fastest"
		}
		territory.BorderSettings.BorderState = "open" // Default, could map from eruntime.Border
		if eruntimeTerritory.Border == 0 {
			territory.BorderSettings.BorderState = "closed"
		}

		eruntimeTerritory.Mu.RUnlock()

		// Add to territories map
		tm.Territories[eruntimeTerritory.Name] = territory
	}

	return nil
}

// stripJSONComments removes comment lines that start with // from JSON text
func (tm *TerritoriesManager) stripJSONComments(jsonText string) string {
	var jsonStrBuilder strings.Builder
	for _, line := range strings.Split(jsonText, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			jsonStrBuilder.WriteString(line)
			jsonStrBuilder.WriteString("\n")
		}
	}
	return jsonStrBuilder.String()
}

// processTerritoriesBorders converts game coordinates to pixel coordinates for all territories
func (tm *TerritoriesManager) processTerritoriesBorders() {
	for name, territory := range tm.Territories {
		if len(territory.Location.Start) >= 2 && len(territory.Location.End) >= 2 {
			x1 := territory.Location.Start[0] + OffsetX
			y1 := territory.Location.Start[1] + OffsetY
			x2 := territory.Location.End[0] + OffsetX
			y2 := territory.Location.End[1] + OffsetY

			// Ensure x1,y1 is the top-left corner and x2,y2 is bottom-right
			if x1 > x2 {
				x1, x2 = x2, x1
			}
			if y1 > y2 {
				y1, y2 = y2, y1
			}

			tm.TerritoryBorders[name] = [4]float64{x1, y1, x2, y2}
		}
	}
}

// initializeAllTerritories sets default values for all loaded territories
func (tm *TerritoriesManager) initializeAllTerritories() {
	for territoryName := range tm.Territories {
		// Initialize territory defaults (this was previously part of side menu functionality)
		// For now, just do nothing since we've disabled the side menu
		_ = territoryName
	}
}

// SaveTerritories saves territory data to the JSON file
func (tm *TerritoriesManager) SaveTerritories() error {
	if !tm.isLoaded {
		return fmt.Errorf("territories data not loaded")
	}

	data, err := json.Marshal(tm.Territories)
	if err != nil {
		return fmt.Errorf("failed to marshal territories data: %v", err)
	}

	err = os.WriteFile("assets/territories.json", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write territories file: %v", err)
	}

	// fmt.Println("Successfully saved territories data")
	return nil
}

// buildSpatialGrid builds a uniform grid for spatial partitioning
func (tm *TerritoriesManager) buildSpatialGrid() {
	// Determine bounds
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, border := range tm.TerritoryBorders {
		if border[0] < minX {
			minX = border[0]
		}
		if border[1] < minY {
			minY = border[1]
		}
		if border[2] > maxX {
			maxX = border[2]
		}
		if border[3] > maxY {
			maxY = border[3]
		}
	}
	cellSize := 400.0 // Tune this for your map scale (pixels)
	cellsX := int(math.Ceil((maxX - minX) / cellSize))
	cellsY := int(math.Ceil((maxY - minY) / cellSize))
	grid := make(map[[2]int][]string)
	// Assign each territory to all grid cells it overlaps
	for name, border := range tm.TerritoryBorders {
		ix1 := int((border[0] - minX) / cellSize)
		iy1 := int((border[1] - minY) / cellSize)
		ix2 := int((border[2] - minX) / cellSize)
		iy2 := int((border[3] - minY) / cellSize)
		for ix := ix1; ix <= ix2; ix++ {
			for iy := iy1; iy <= iy2; iy++ {
				cell := [2]int{ix, iy}
				grid[cell] = append(grid[cell], name)
			}
		}
	}
	tm.grid = grid
	tm.gridCellSize = cellSize
	tm.gridMinX = minX
	tm.gridMinY = minY
	tm.gridMaxX = maxX
	tm.gridMaxY = maxY
	tm.gridCellsX = cellsX
	tm.gridCellsY = cellsY
}

// startBufferUpdateLoop starts the background goroutine for updating the offscreen buffer
func (tm *TerritoriesManager) startBufferUpdateLoop() {
	if tm.bufferUpdateRunning {
		return
	}

	tm.bufferUpdateRunning = true
	tm.bufferUpdateTicker = time.NewTicker(100 * time.Millisecond) // Update every 100ms (10 FPS max) instead of 7ms

	go func() {
		defer func() {
			tm.bufferUpdateRunning = false
		}()

		for {
			select {
			case <-tm.bufferUpdateTicker.C:
				tm.bufferMutex.RLock()
				needsUpdate := tm.bufferNeedsUpdate
				tm.bufferMutex.RUnlock()

				if needsUpdate && tm.isLoaded {
					tm.updateOffscreenBuffer()
				}
			case <-tm.bufferStopChan:
				tm.bufferUpdateTicker.Stop()
				return
			}
		}
	}()
}

// stopBufferUpdateLoop stops the background goroutine
func (tm *TerritoriesManager) stopBufferUpdateLoop() {
	if tm.bufferUpdateRunning {
		tm.bufferStopChan <- true
	}
}

// UpdateBufferIfNeeded marks the buffer as needing an update
func (tm *TerritoriesManager) UpdateBufferIfNeeded(screenWidth, screenHeight int, scale, viewX, viewY float64) {
	tm.bufferMutex.Lock()
	defer tm.bufferMutex.Unlock()

	// Check if buffer parameters have changed significantly
	scaleChanged := math.Abs(tm.bufferScale-scale) > 0.01
	viewChanged := math.Abs(tm.bufferViewX-viewX) > 0.1 || math.Abs(tm.bufferViewY-viewY) > 0.1
	sizeChanged := tm.bufferWidth != screenWidth || tm.bufferHeight != screenHeight

	if scaleChanged || viewChanged || sizeChanged || tm.offscreenBuffer == nil {
		tm.bufferNeedsUpdate = true
		tm.bufferScale = scale
		tm.bufferViewX = viewX
		tm.bufferViewY = viewY
		tm.bufferWidth = screenWidth
		tm.bufferHeight = screenHeight
	}
}

// updateOffscreenBuffer renders territories to the offscreen buffer
func (tm *TerritoriesManager) updateOffscreenBuffer() {
	// profileStart := time.Now()
	tm.bufferMutex.Lock()
	defer tm.bufferMutex.Unlock()

	if !tm.bufferNeedsUpdate {
		return
	}

	// Check if buffer dimensions are valid before creating
	if tm.bufferWidth <= 0 || tm.bufferHeight <= 0 {
		return // Skip if dimensions are not set yet
	}

	// Limit buffer size to prevent excessive memory usage
	maxBufferSize := 4096 // 4K maximum for each dimension
	if tm.bufferWidth > maxBufferSize {
		tm.bufferWidth = maxBufferSize
	}
	if tm.bufferHeight > maxBufferSize {
		tm.bufferHeight = maxBufferSize
	}

	// bufferCreateStart := time.Now()
	if tm.offscreenBuffer == nil || tm.offscreenBuffer.Bounds().Dx() != tm.bufferWidth || tm.offscreenBuffer.Bounds().Dy() != tm.bufferHeight {
		tm.offscreenBuffer = ebiten.NewImage(tm.bufferWidth, tm.bufferHeight)
		// // fmt.Printf("[PROFILE] Buffer creation took %v\n", time.Since(bufferCreateStart))
	}

	// clearStart := time.Now()
	tm.offscreenBuffer.Clear()
	// // fmt.Printf("[PROFILE] Buffer clear took %v\n", time.Since(clearStart))

	// drawStart := time.Now()
	tm.drawTerritoriesToBuffer(tm.offscreenBuffer, tm.bufferScale, tm.bufferViewX, tm.bufferViewY, "")
	// // fmt.Printf("[PROFILE] drawTerritoriesToBuffer took %v\n", time.Since(drawStart))

	tm.bufferNeedsUpdate = false
	// // fmt.Printf("[PROFILE] updateOffscreenBuffer total took %v\n", time.Since(profileStart))
}

// --- Get visible territory names using the grid ---
func (tm *TerritoriesManager) getVisibleTerritories(scale, viewX, viewY, screenWidth, screenHeight float64) map[string]struct{} {
	if tm.grid == nil {
		// Fallback: return all
		all := make(map[string]struct{})
		for name := range tm.TerritoryBorders {
			all[name] = struct{}{}
		}
		return all
	}
	// Compute world coordinates of the screen corners
	invScale := 1.0 / scale
	worldX1 := (-viewX) * invScale
	worldY1 := (-viewY) * invScale
	worldX2 := (screenWidth - viewX) * invScale
	worldY2 := (screenHeight - viewY) * invScale
	if worldX1 > worldX2 {
		worldX1, worldX2 = worldX2, worldX1
	}
	if worldY1 > worldY2 {
		worldY1, worldY2 = worldY2, worldY1
	}
	// Find grid cells overlapping the view
	ix1 := int((worldX1 - tm.gridMinX) / tm.gridCellSize)
	iy1 := int((worldY1 - tm.gridMinY) / tm.gridCellSize)
	ix2 := int((worldX2 - tm.gridMinX) / tm.gridCellSize)
	iy2 := int((worldY2 - tm.gridMinY) / tm.gridCellSize)
	territorySet := make(map[string]struct{})
	for ix := ix1; ix <= ix2; ix++ {
		for iy := iy1; iy <= iy2; iy++ {
			cell := [2]int{ix, iy}
			for _, name := range tm.grid[cell] {
				territorySet[name] = struct{}{}
			}
		}
	}
	return territorySet
}

// isLowDetail determines if we should use simplified rendering based on zoom level
func (tm *TerritoriesManager) isLowDetail(scale float64) bool {
	// When scale is small (zoomed out), use low detail rendering
	return scale < 0.3
}

// drawTerritoriesToBuffer renders territories directly to the specified image buffer
func (tm *TerritoriesManager) drawTerritoriesToBuffer(buffer *ebiten.Image, scale, viewX, viewY float64, hoveredTerritory string) {
	if !tm.isLoaded {
		return
	}

	// Use the new territory renderer for true transparency
	if tm.territoryRenderer != nil {
		tm.territoryRenderer.RenderToBuffer(buffer, scale, viewX, viewY, hoveredTerritory)
	}
}

// DrawTerritories draws all territory boundaries and trading routes using the new GPU overlay system
func (tm *TerritoriesManager) DrawTerritories(screen *ebiten.Image, scale, viewX, viewY float64, hoveredTerritory string) {
	defer func() {
		if r := recover(); r != nil {
			// fmt.Printf("[PANIC] DrawTerritories recovered: %v\n", r)
			// fmt.Printf("[PANIC] Stack trace:\n%s\n", debug.Stack())
		}
	}()

	if !tm.isLoaded || tm.overlayGPU == nil {
		return
	}

	// Acquire read lock to prevent concurrent modification of territories map
	tm.territoryMutex.RLock()
	defer tm.territoryMutex.RUnlock()

	// GPU overlay system is now working correctly - green stripes issue was fixed in drawClaimedTerritoryOverlay
	// No need to disable during claim editing anymore

	// Get the map image (assume MapView provides it, fallback to assets)
	mapView := GetMapView()
	if mapView == nil || mapView.mapManager == nil {
		return
	}
	mapImg := mapView.mapManager.GetMapImage()
	if mapImg == nil {
		return
	}

	screenW := screen.Bounds().Dx()
	screenH := screen.Bounds().Dy()

	// Build overlays for all visible territories
	visible := tm.getVisibleTerritories(scale, viewX, viewY, float64(screenW), float64(screenH))
	polygons := make([]OverlayPolygon, 0, len(visible)*2) // Pre-allocate for territories + routes

	// Calculate blink phase with 330ms cycles (0.33 seconds for lighten, 0.33 seconds for darken)
	blinkCycle := 0.66 // Total cycle time: 330ms light + 330ms dark = 660ms
	blinkPhase := float32(math.Mod(tm.blinkTimer, blinkCycle))

	// Get current frame number for cache management
	frameNumber := time.Now().UnixNano() / int64(time.Millisecond/16) // ~60fps frame number

	// Check if we should use low detail rendering for performance
	lowDetail := tm.isLowDetail(scale)

	// Process territories directly but maintain consistent ordering to prevent flickering
	// We'll process routes and territories in separate loops for predictable ordering

	// Convert visible map to sorted slice for consistent rendering order
	territoryNames := make([]string, 0, len(visible))
	for name := range visible {
		territoryNames = append(territoryNames, name)
	}
	// Sort for consistent order - this is still faster than doing it per-frame
	// but we only do it once here instead of multiple times
	sort.Strings(territoryNames)

	// Get routes to highlight if feature is enabled and there's a hovered territory
	var routesToHighlight map[string]bool
	if tm.showHoveredRoutes && hoveredTerritory != "" {
		routesToHQ := tm.getRouteToHQForTerritory(hoveredTerritory)
		if len(routesToHQ) > 1 { // Must have at least origin and destination
			routesToHighlight = make(map[string]bool)
			// Mark route segments for highlighting
			for i := 0; i < len(routesToHQ)-1; i++ {
				routeKey := routesToHQ[i] + "->" + routesToHQ[i+1]
				routesToHighlight[routeKey] = true
			}
			// Debug: print route highlighting info (can be removed later)
			fmt.Printf("[ROUTE_HIGHLIGHT] Highlighting %d route segments for territory %s to HQ\n", len(routesToHighlight), hoveredTerritory)
		}
	}

	// First pass: Add trading routes as line polygons so they render behind territories
	// Skip routes in low detail mode for better performance
	if !lowDetail {
		for _, name := range territoryNames {
			territory, ok := tm.Territories[name]
			if !ok {
				continue
			}
			border1, exists := tm.TerritoryBorders[name]
			if !exists {
				continue
			}

			centerX1 := float32((border1[0]+border1[2])/2*scale + viewX)
			centerY1 := float32((border1[1]+border1[3])/2*scale + viewY)

			// Process trading routes with minimal sorting only when there are multiple routes
			// This maintains consistency while keeping good performance
			routes := territory.TradingRoutes
			if len(routes) > 1 {
				// Only sort when there are multiple routes to maintain consistency
				sortedRoutes := make([]string, len(routes))
				copy(sortedRoutes, routes)
				sort.Strings(sortedRoutes)
				routes = sortedRoutes
			}

			for _, routeName := range routes {
				border2, exists := tm.TerritoryBorders[routeName]
				if !exists {
					continue
				}

				centerX2 := float32((border2[0]+border2[2])/2*scale + viewX)
				centerY2 := float32((border2[1]+border2[3])/2*scale + viewY)

				// Create a thick line as a rectangle
				thickness := float32(1.5) // Route thickness (was 3.0, thick functionality preserved for future)
				dx := centerX2 - centerX1
				dy := centerY2 - centerY1
				length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if length == 0 {
					continue
				}

				// Normalize direction vector
				dx /= length
				dy /= length

				// Perpendicular vector for width
				perpX := -dy * thickness / 2
				perpY := dx * thickness / 2

				// Create rectangle points for the line
				routePts := [][2]float32{
					{centerX1 + perpX, centerY1 + perpY},
					{centerX2 + perpX, centerY2 + perpY},
					{centerX2 - perpX, centerY2 - perpY},
					{centerX1 - perpX, centerY1 - perpY},
				}

				// Check if this route segment should be highlighted
				routeKey := name + "->" + routeName
				isHighlighted := routesToHighlight != nil && routesToHighlight[routeKey]

				// Determine route color and state
				var routeColor [3]float32
				var routeState OverlayState
				var borderColor [3]float32

				if isHighlighted {
					// White for highlighted routes to HQ
					routeColor = [3]float32{1.0, 1.0, 1.0}
					routeState = OverlayRouteHighlighted
					borderColor = [3]float32{0.8, 0.8, 0.8} // Light gray border for highlighted routes
				} else {
					// Black for normal routes
					routeColor = [3]float32{0.0, 0.0, 0.0}
					routeState = OverlayNormal
					borderColor = [3]float32{0.2, 0.2, 0.2} // Dark gray border
				}

				routePoly := OverlayPolygon{
					Points:      routePts,
					Color:       routeColor,
					State:       routeState,
					BlinkPhase:  blinkPhase,
					BorderWidth: 1.0,
					BorderColor: borderColor,
				}
				polygons = append(polygons, routePoly)
			}
		}
	} // End of !lowDetail check for trading routes

	// Batch territory color lookup for performance optimization
	var territoryColors map[string]color.RGBA
	if mapView := GetMapView(); mapView != nil && mapView.territoryViewSwitcher != nil {
		territoryColors = mapView.territoryViewSwitcher.GetTerritoryColorsForCurrentView(territoryNames)
	}

	// Second pass: Add territories AFTER routes so they render on top
	for _, name := range territoryNames {
		territory, ok := tm.Territories[name]
		if !ok || len(territory.Location.Start) < 2 || len(territory.Location.End) < 2 {
			continue
		}
		// Rectangle as polygon (future: use real shapes)
		x1 := float32((territory.Location.Start[0]+OffsetX)*scale + viewX)
		y1 := float32((territory.Location.Start[1]+OffsetY)*scale + viewY)
		x2 := float32((territory.Location.End[0]+OffsetX)*scale + viewX)
		y2 := float32((territory.Location.End[1]+OffsetY)*scale + viewY)
		pts := [][2]float32{{x1, y1}, {x2, y1}, {x2, y2}, {x1, y2}}

		// Use cached guild color for better performance
		col := tm.getCachedGuildColor(territory.Guild.Name, frameNumber)

		// Get actual guild ownership from territory claims
		actualGuildName, actualGuildTag := tm.getActualGuildForTerritory(name)

		// Check if this territory is being edited - override persistent claims
		if tm.territoryRenderer != nil && tm.territoryRenderer.editingGuildName != "" && tm.territoryRenderer.editingClaims != nil {
			// Check if the territory is in the editing claims map
			if claimedValue, exists := tm.territoryRenderer.editingClaims[name]; exists {
				if claimedValue {
					// Territory is claimed by the editing guild
					actualGuildName = tm.territoryRenderer.editingGuildName
					actualGuildTag = tm.territoryRenderer.editingGuildTag
				} else {
					// Territory is explicitly unclaimed (set to false) - show as no guild
					actualGuildName = ""
					actualGuildTag = ""
				}
			}
		}

		// Try to get color from enhanced guild manager first (if available)
		if tm.guildManager != nil && actualGuildName != "" {
			if guildColor, found := tm.guildManager.GetGuildColor(actualGuildName, actualGuildTag); found {
				// fmt.Printf("[TERRITORIES] Using guild color for territory %s (guild %s [%s]): R=%d G=%d B=%d\n",
				// name, actualGuildName, actualGuildTag, guildColor.R, guildColor.G, guildColor.B)
				// Convert RGBA to [3]float32
				col = [3]float32{
					float32(guildColor.R) / 255.0,
					float32(guildColor.G) / 255.0,
					float32(guildColor.B) / 255.0,
				}
			} else {
				// fmt.Printf("[TERRITORIES] Guild color not found for territory %s (guild %s [%s])\n",
				// name, actualGuildName, actualGuildTag)
			}
		} else if tm.guildManager == nil {
			// fmt.Printf("[TERRITORIES] Warning: Guild manager is nil for territory %s\n", name)
		} else if actualGuildName == "" {
			// fmt.Printf("[TERRITORIES] No guild name for territory %s\n", name)
		}

		// Check territory view switcher for alternative coloring modes (using batched lookup)
		if territoryColors != nil {
			if territoryColor, hasCustomColor := territoryColors[name]; hasCustomColor {
				// Override with territory view switcher color
				col = [3]float32{
					float32(territoryColor.R) / 255.0,
					float32(territoryColor.G) / 255.0,
					float32(territoryColor.B) / 255.0,
				}
			}
		}

		// Check if this territory is selected for loadout application (highest priority override)
		if tm.territoryRenderer != nil && tm.territoryRenderer.isLoadoutApplicationMode && tm.territoryRenderer.IsTerritorySelectedForLoadout(name) {
			// fmt.Printf("[TERRITORIES] Territory %s is selected for loadout application - applying bright yellow color in GPU overlay\n", name)
			// Use bright yellow for selected territories (same as in renderer)
			col = [3]float32{1.0, 1.0, 0.0} // Bright yellow (RGB: 255, 255, 0)
		}

		// Determine state - blinking takes priority over hover
		state := OverlayNormal
		if name == tm.selectedTerritory && tm.isBlinking {
			state = OverlaySelected
		} else if name == hoveredTerritory {
			state = OverlayHovered
		}

		// Check if we're in claim editing mode and should dim territories not belonging to the editing guild
		if mapView := GetMapView(); mapView != nil && mapView.IsEditingClaims() {
			editingGuildName := ""
			if tm.territoryRenderer != nil {
				editingGuildName = tm.territoryRenderer.GetEditingGuildName()
			}

			// If we have an editing guild and this territory doesn't belong to it, dim it
			if editingGuildName != "" && actualGuildName != editingGuildName {
				// Only dim if this territory is not hovered or selected (those states take priority)
				if state == OverlayNormal {
					state = OverlayDimmed
				}
			}
		}

		// Determine border properties based on state (but keep guild color)
		borderWidth := float32(3.0) // Default border width
		borderColor := col          // Use guild color for borders

		// Use thinner borders in low detail mode for better performance
		if lowDetail {
			borderWidth = 1.0
		}

		// Adjust border width based on state (but keep guild color)
		switch state {
		case OverlayHovered:
			borderWidth = 3.0
			if lowDetail {
				borderWidth = 1.5
			}
			// Keep guild color but make it slightly brighter
			borderColor = [3]float32{
				clamp01(col[0] * 1.2),
				clamp01(col[1] * 1.2),
				clamp01(col[2] * 1.2),
			}
		case OverlaySelected:
			borderWidth = 4.0
			if lowDetail {
				borderWidth = 2.0
			}
			// For selected, use a bright version of guild color
			borderColor = [3]float32{
				clamp01(col[0] * 1.5),
				clamp01(col[1] * 1.5),
				clamp01(col[2] * 1.5),
			}
		}

		poly := OverlayPolygon{
			Points:      pts,
			Color:       col,
			State:       state,
			BlinkPhase:  blinkPhase,
			BorderWidth: borderWidth,
			BorderColor: borderColor,
		}
		polygons = append(polygons, poly)
	}

	// Draw overlays using GPU
	tm.overlayGPU.Draw(screen, mapImg, polygons, blinkPhase)

	// Draw HQ icons on top of everything else (screen-space, not affected by zoom)
	tm.drawHQOverlays(screen, scale, viewX, viewY, territoryNames)
}

// Cleanup stops the buffer update loop - should be called when the manager is no longer needed
func (tm *TerritoriesManager) Cleanup() {
	tm.stopBufferUpdateLoop()
}

// IsLoaded returns whether the territories data is loaded
func (tm *TerritoriesManager) IsLoaded() bool {
	return tm.isLoaded
}

// GetLoadError returns any error that occurred during loading
func (tm *TerritoriesManager) GetLoadError() error {
	return tm.loadError
}

// GetPendingRoute returns and clears the pending territory selection from clicked items
// in either the Connected Territories or Trading Routes sections
func (tm *TerritoriesManager) GetPendingRoute() string {
	route := tm.pendingRoute
	if route != "" {
		tm.pendingRoute = ""
	}
	return route
}

// OpenGuildManagement shows the guild management interface
func (tm *TerritoriesManager) OpenGuildManagement() {
	// The GuildManager reference is passed in from GameplayModule
	// This function is called when the guild manager should be shown
	if tm.guildManager != nil {
		// fmt.Printf("[TERRITORIES] Opening guild management menu\n")

		// Close loadout manager if it's open (mutually exclusive)
		loadoutManager := GetLoadoutManager()
		if loadoutManager != nil && loadoutManager.IsVisible() {
			loadoutManager.Hide()
		}

		tm.guildManager.Show()
	} else {
		// fmt.Printf("[TERRITORIES] Guild manager is nil, can't open\n")
	}
}

// loadUpgradeData loads upgrade configuration from JSON
func (tm *TerritoriesManager) loadUpgradeData() {
	data, err := assets.AssetFiles.ReadFile("upgrades.json")
	if err != nil {
		// fmt.Printf("Failed to read upgrades file: %v\n", err)
		return
	}

	// Strip comments like in LoadTerritories
	var jsonStrBuilder strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			jsonStrBuilder.WriteString(line)
			jsonStrBuilder.WriteString("\n")
		}
	}
	jsonStr := jsonStrBuilder.String()

	var upgradeData UpgradeData
	if err := json.Unmarshal([]byte(jsonStr), &upgradeData); err != nil {
		// fmt.Printf("Warning: Failed to parse upgrades data after stripping comments: %v. Trying raw data.\n", err)
		if errFallback := json.Unmarshal(data, &upgradeData); errFallback != nil {
			// fmt.Printf("Failed to parse upgrades data: %v\n", errFallback)
			return
		}
	}

	tm.upgradeData = &upgradeData
	// fmt.Println("Successfully loaded upgrade data")
}

// loadHQImage loads the HQ icon from embedded assets
func (tm *TerritoriesManager) loadHQImage() {
	// Load HQ image from embedded assets
	hqImageData, err := assets.AssetFiles.ReadFile("hq.png")
	if err != nil {
		// fmt.Printf("Warning: Could not load HQ image: %v\n", err)
		return
	}

	// Decode the image
	img, err := png.Decode(bytes.NewReader(hqImageData))
	if err != nil {
		// fmt.Printf("Warning: Could not decode HQ image: %v\n", err)
		return
	}

	// Convert to ebiten.Image
	tm.hqImage = ebiten.NewImageFromImage(img)
	// fmt.Printf("Successfully loaded HQ image (%dx%d)\n", img.Bounds().Dx(), img.Bounds().Dy())
}

// drawTextOffset is a stub function (was part of side menu functionality)
func drawTextOffset(screen *ebiten.Image, str string, font interface{}, x, y int, clr color.Color) {
	// NO-OP: Side menu related functionality disabled - text drawing not needed
}

// IsTextInputActive is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) IsTextInputActive() bool {
	return false // Side menu related functionality disabled
}

// HandleKeyboardInput is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) HandleKeyboardInput(key ebiten.Key, justPressed bool) bool {
	return false // Side menu related functionality disabled
}

// UpdateRealTimeData is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) UpdateRealTimeData() {
	// NO-OP: Side menu related functionality disabled
}

// GetTerritoryAtPosition returns territory at given position (still used for hover)
func (tm *TerritoriesManager) GetTerritoryAtPosition(mouseX, mouseY int, scale, viewX, viewY float64) string {
	// This is still used for hover effects, so keep the basic functionality
	// Convert screen coordinates to world coordinates
	// Reverse the transformation: screen -> world
	// The map transformation is: scale * world + offset = screen
	// So: world = (screen - offset) / scale
	worldX := (float64(mouseX) - viewX) / scale
	worldY := (float64(mouseY) - viewY) / scale

	// Debug output (can be removed later)
	// // fmt.Printf("Mouse: (%d, %d), Scale: %.3f, Offset: (%.1f, %.1f), World: (%.1f, %.1f)\n",
	//     mouseX, mouseY, scale, viewX, viewY, worldX, worldY)

	// Collect all territories that contain the mouse position
	var candidates []string
	for name, border := range tm.TerritoryBorders {
		// border is [x1, y1, x2, y2]
		x1, y1, x2, y2 := border[0], border[1], border[2], border[3]

		// Ensure proper min/max bounds
		minX, maxX := math.Min(x1, x2), math.Max(x1, x2)
		minY, maxY := math.Min(y1, y2), math.Max(y1, y2)

		if worldX >= minX && worldX <= maxX && worldY >= minY && worldY <= maxY {
			candidates = append(candidates, name)
		}
	}

	// If multiple territories overlap, return the smallest one (most specific)
	if len(candidates) > 0 {
		if len(candidates) == 1 {
			return candidates[0]
		}

		// Find the territory with the smallest area
		smallestArea := math.Inf(1)
		smallestTerritory := candidates[0]

		for _, name := range candidates {
			if border, exists := tm.TerritoryBorders[name]; exists {
				x1, y1, x2, y2 := border[0], border[1], border[2], border[3]
				area := math.Abs(x2-x1) * math.Abs(y2-y1)
				if area < smallestArea {
					smallestArea = area
					smallestTerritory = name
				}
			}
		}
		return smallestTerritory
	}

	return ""
}

// HandleSideMenuClick is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) HandleSideMenuClick(mx, my, screenW int) bool {
	return false // Side menu related functionality disabled
}

// handleMouseRelease is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) handleMouseRelease() {
	// NO-OP: Side menu related functionality disabled
}

// handleSliderDrag is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) handleSliderDrag(mx, my int) bool {
	return false // Side menu related functionality disabled
}

// TerritoriesManager methods continued...

// isPointInTerritory checks if a point is inside a territory (legacy method, prefer direct TerritoryBorders lookup)
func (tm *TerritoriesManager) isPointInTerritory(x, y float64, territory Territory) bool {
	// Simple bounding box check using Start and End coordinates
	if len(territory.Location.Start) >= 2 && len(territory.Location.End) >= 2 {
		x1, y1 := territory.Location.Start[0], territory.Location.Start[1]
		x2, y2 := territory.Location.End[0], territory.Location.End[1]

		// Ensure proper min/max bounds
		minX, maxX := math.Min(x1, x2), math.Max(x1, x2)
		minY, maxY := math.Min(y1, y2), math.Max(y1, y2)

		return x >= minX && x <= maxX && y >= minY && y <= maxY
	}
	return false
}

// GetSelectedTerritory is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) GetSelectedTerritory() string {
	return "" // Side menu related functionality disabled
}

// PrepareToCloseMenu is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) PrepareToCloseMenu() {
	// NO-OP: Side menu related functionality disabled
}

// UpdateBlink is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) UpdateBlink(deltaTime float64) {
	// NO-OP: Side menu related functionality disabled
}

// HandleMouseClick returns territory at click position (still used for territory selection)
func (tm *TerritoriesManager) HandleMouseClick(mouseX, mouseY int, scale, viewX, viewY float64) string {
	// This is still used for territory clicking, so keep the basic functionality
	return tm.GetTerritoryAtPosition(mouseX, mouseY, scale, viewX, viewY)
}

// GetRenderer returns the territory renderer (if available)
func (tm *TerritoriesManager) GetRenderer() *TerritoryRenderer {
	// Return the territory renderer if it exists
	return tm.territoryRenderer
}

// SetGuildManager sets the guild manager reference
func (tm *TerritoriesManager) SetGuildManager(gm interface{}) {
	if enhancedGM, ok := gm.(*EnhancedGuildManager); ok {
		tm.guildManager = enhancedGM
	}
}

// InvalidateTerritoryCache is a no-op (was part of side menu functionality)
func (tm *TerritoriesManager) InvalidateTerritoryCache() {
	// NO-OP: Side menu related functionality disabled
}

// IsSideMenuOpen method stub (commented out functionality)
func (tm *TerritoriesManager) IsSideMenuOpen() bool {
	return false // Side menu is disabled
}

// GetSideMenuWidth method stub (commented out functionality)
func (tm *TerritoriesManager) GetSideMenuWidth() float64 {
	return 0 // Side menu is disabled
}

// CloseSideMenu method stub (commented out functionality)
func (tm *TerritoriesManager) CloseSideMenu() {
	// NO-OP: Side menu is disabled
}

// SelectTerritory method stub (commented out functionality)
func (tm *TerritoriesManager) SelectTerritory(territoryName string, directTransition ...bool) {
	// NO-OP: Side menu is disabled, EdgeMenu is used instead
	// fmt.Printf("SelectTerritory called for %s but old side menu is disabled\n", territoryName)
}

// GetSelectedTerritoryName returns the name of the currently selected/blinking territory
func (tm *TerritoriesManager) GetSelectedTerritoryName() string {
	if tm.isBlinking {
		return tm.selectedTerritory
	}
	return ""
}

// DeselectTerritory stops blinking and clears the selected territory
func (tm *TerritoriesManager) DeselectTerritory() {
	tm.selectedTerritory = ""
	tm.isBlinking = false
	tm.blinkTimer = 0
	tm.bufferNeedsUpdate = true
}

// SetSelectedTerritory sets the selected territory for blinking effect
func (tm *TerritoriesManager) SetSelectedTerritory(territoryName string) {
	// If selecting the same territory that's already selected, deselect it
	if tm.selectedTerritory == territoryName && tm.isBlinking {
		tm.DeselectTerritory()
		return
	}

	// Set new selected territory and start blinking
	tm.selectedTerritory = territoryName
	tm.isBlinking = true
	tm.blinkTimer = 0
	tm.bufferNeedsUpdate = true
}

// Update handles territory animations including blinking
func (tm *TerritoriesManager) Update(deltaTime float64) {
	if tm.isBlinking {
		tm.blinkTimer += deltaTime

		// Keep blinking indefinitely - only stop when deselected
		// Reset timer periodically to prevent overflow
		if tm.blinkTimer >= 60.0 { // Reset every 60 seconds to prevent overflow
			tm.blinkTimer = 0
		}
	}
}

// RefreshFromEruntime reloads territory data from eruntime
func (tm *TerritoriesManager) RefreshFromEruntime() error {
	// // fmt.Println("[DEBUG] TerritoriesManager.RefreshFromEruntime called")

	// Acquire write lock to prevent concurrent access during refresh
	tm.territoryMutex.Lock()
	defer tm.territoryMutex.Unlock()

	// Reload territories from eruntime
	err := tm.readAndParseTerritoriesJSON()
	if err != nil {
		tm.loadError = err
		return err
	}

	// Process borders with new data
	tm.processTerritoriesBorders()

	// Initialize territories with new data
	tm.initializeAllTerritories()

	// Rebuild spatial grid
	tm.buildSpatialGrid()

	// Mark buffer as needing update (with proper mutex)
	tm.bufferMutex.Lock()
	tm.bufferNeedsUpdate = true
	tm.bufferMutex.Unlock()

	// fmt.Printf("[DEBUG] RefreshFromEruntime complete. Loaded %d territories, buffer marked for update\n", len(tm.Territories))

	tm.isLoaded = true
	tm.loadError = nil
	return nil
}

// getCachedGuildColor returns a cached guild color, computing it only if needed
func (tm *TerritoriesManager) getCachedGuildColor(guildName string, frameNumber int64) [3]float32 {
	// Clear cache if this is a new frame to prevent memory growth
	if frameNumber != tm.cacheLastFrame {
		tm.guildColorCache = make(map[string][3]float32)
		tm.cacheLastFrame = frameNumber
	}

	// Check cache first
	if color, exists := tm.guildColorCache[guildName]; exists {
		return color
	}

	// Compute and cache the color
	color := getGuildColor(guildName)
	tm.guildColorCache[guildName] = color
	return color
}

// drawHQOverlays draws HQ icons on territories marked as HQ
// The icons are drawn in screen space and maintain constant size regardless of zoom level
func (tm *TerritoriesManager) drawHQOverlays(screen *ebiten.Image, scale, viewX, viewY float64, territoryNames []string) {
	if tm.hqImage == nil {
		return
	}

	// Get HQ image dimensions
	hqWidth := tm.hqImage.Bounds().Dx()
	hqHeight := tm.hqImage.Bounds().Dy()

	// Desired size of HQ icon on screen (constant regardless of zoom)
	const iconSize = 32.0 // pixels

	// Calculate scale factor to maintain constant icon size
	iconScale := iconSize / float64(max(hqWidth, hqHeight))

	for _, name := range territoryNames {
		territory, ok := tm.Territories[name]
		if !ok {
			continue
		}

		// Check if this territory is marked as HQ
		// First check the actual territory data from eruntime
		eruntimeTerritory := eruntime.GetTerritory(name)
		isHQ := false
		if eruntimeTerritory != nil {
			eruntimeTerritory.Mu.RLock()
			isHQ = eruntimeTerritory.HQ
			eruntimeTerritory.Mu.RUnlock()
		}

		// Also check the local territory data as fallback
		if !isHQ && territory.isHQ {
			isHQ = true
		}

		if !isHQ {
			continue
		}

		// Calculate the center of the territory in screen coordinates
		if len(territory.Location.Start) < 2 || len(territory.Location.End) < 2 {
			continue
		}

		centerX := (territory.Location.Start[0] + territory.Location.End[0]) / 2
		centerY := (territory.Location.Start[1] + territory.Location.End[1]) / 2

		// Convert to screen coordinates
		screenX := (centerX+OffsetX)*scale + viewX
		screenY := (centerY+OffsetY)*scale + viewY

		// Calculate icon position (centered on territory)
		iconX := screenX - (iconSize / 2)
		iconY := screenY - (iconSize / 2)

		// Only draw if the icon would be visible on screen
		screenBounds := screen.Bounds()
		if iconX+iconSize >= 0 && iconX < float64(screenBounds.Max.X) &&
			iconY+iconSize >= 0 && iconY < float64(screenBounds.Max.Y) {

			// Draw the HQ icon with scaling
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(iconScale, iconScale)
			op.GeoM.Translate(iconX, iconY)

			// Add a subtle glow effect
			op.ColorScale.SetA(0.9) // Slightly transparent for better blending

			screen.DrawImage(tm.hqImage, op)
		}
	}
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SetTerritoryHQ sets or removes HQ status for a territory
func (tm *TerritoriesManager) SetTerritoryHQ(territoryName string, isHQ bool) bool {
	eruntimeTerritory := eruntime.GetTerritory(territoryName)
	if eruntimeTerritory == nil {
		return false
	}

	if isHQ {
		// Use the proper HQ setting mechanism which handles clearing old HQs and notifications
		opts := eruntime.GetTerritoryStats(territoryName)
		if opts != nil {
			territoryOpts := typedef.TerritoryOptions{
				Upgrades:    opts.Upgrades,
				Bonuses:     opts.Bonuses,
				Tax:         opts.Tax,
				RoutingMode: opts.RoutingMode,
				Border:      opts.Border,
				HQ:          true, // This will trigger the proper HQ management
			}
			eruntime.Set(territoryName, territoryOpts)
		}
	} else {
		// For removing HQ, we can directly set it to false since there's no conflict
		eruntimeTerritory.Mu.Lock()
		guildName := eruntimeTerritory.Guild.Name // Capture guild name before unlocking
		eruntimeTerritory.HQ = false
		eruntimeTerritory.Mu.Unlock()
		// fmt.Printf("[TERRITORIES] Removed HQ status from territory %s\n", territoryName)

		// Trigger UI updates for HQ removal with specific guild notification
		go func() {
			time.Sleep(50 * time.Millisecond)
			// Update visual representation after HQ removal
			tm.bufferMutex.Lock()
			tm.bufferNeedsUpdate = true
			tm.bufferMutex.Unlock()

			// Notify only the specific guild that had its HQ removed
			eruntime.NotifyGuildSpecificUpdate(guildName)
		}()
	}

	// Update local territory data
	tm.territoryMutex.Lock()
	defer tm.territoryMutex.Unlock()

	if territory, exists := tm.Territories[territoryName]; exists {
		territory.isHQ = isHQ
		tm.Territories[territoryName] = territory
		// fmt.Printf("[TERRITORIES] Updated local HQ status for %s to %v\n", territoryName, isHQ)
		return true
	}

	return false
}

// UpdateTerritoryHQStatus updates the local HQ status for a territory
func (tm *TerritoriesManager) UpdateTerritoryHQStatus(territoryName string, isHQ bool) {
	tm.territoryMutex.Lock()
	defer tm.territoryMutex.Unlock()

	if territory, exists := tm.Territories[territoryName]; exists {
		if territory.isHQ != isHQ {
			territory.isHQ = isHQ
			tm.Territories[territoryName] = territory
			// fmt.Printf("[TERRITORIES] Updated HQ status for %s to %v\n", territoryName, isHQ)

			// Mark buffer as needing update to refresh visual display
			tm.bufferMutex.Lock()
			tm.bufferNeedsUpdate = true
			tm.bufferMutex.Unlock()
		}
	}
}

// IsHQTerritory checks if a territory is marked as HQ
func (tm *TerritoriesManager) IsHQTerritory(territoryName string) bool {
	// Check eruntime first (authoritative source)
	eruntimeTerritory := eruntime.GetTerritory(territoryName)
	if eruntimeTerritory != nil {
		eruntimeTerritory.Mu.RLock()
		isHQ := eruntimeTerritory.HQ
		eruntimeTerritory.Mu.RUnlock()
		return isHQ
	}

	// Fallback to local data
	tm.territoryMutex.RLock()
	defer tm.territoryMutex.RUnlock()

	if territory, exists := tm.Territories[territoryName]; exists {
		return territory.isHQ
	}

	return false
}

// GetShowHoveredRoutes returns whether route highlighting for hovered territories is enabled
func (tm *TerritoriesManager) GetShowHoveredRoutes() bool {
	return tm.showHoveredRoutes
}

// SetShowHoveredRoutes sets whether route highlighting for hovered territories is enabled
func (tm *TerritoriesManager) SetShowHoveredRoutes(enabled bool) {
	tm.showHoveredRoutes = enabled
}
