package eruntime

import (
	"etools/typedef"
	"fmt"
	"sync"
	"time"
)

// debugf prints debug information only if debug logging is enabled
func debugf(format string, args ...interface{}) {
	if st.debugLogging {
		// fmt.Printf("[DEBUG] "+format, args...)
	}
}

// World state for specific session
// This is the main runtime for the game, it will handle all the logic and state of the game
// and will be used for guild economy calculations
type state struct {
	mu             sync.RWMutex // Protects all state fields from concurrent access
	territories    []*typedef.Territory
	guilds         []*typedef.Guild
	activeTributes []*typedef.ActiveTribute // Active tributes in the system

	savedSnapshots [][]*typedef.Territory

	tick uint64 // tick elapsed since start

	// time.Ticker
	timerChan *time.Ticker
	halted    bool

	// Tick processing queue for high-performance tick processing
	tickQueue chan struct{}

	// Performance monitoring for high-rate tick processing
	lastTickTime    time.Time
	tickProcessTime time.Duration
	actualTPS       float64

	// Processing mode configuration
	useParallelProcessing bool

	costs typedef.Costs

	runtimeOptions typedef.RuntimeOptions

	// Transit system for managing resource movement
	transitManager *TransitManager

	// HQ map for fast HQ lookups - maps guild tag to HQ territory
	hqMap map[string]*typedef.Territory

	// Territory map for fast territory lookups by ID - maps territory ID to territory
	territoryMap map[string]*typedef.Territory

	// No need, each territory has its own mutex
	// mu sync.Mutex // mutex to protect state changes

	// Debug flag to enable/disable verbose logging (off by default for performance)
	debugLogging bool

	// State loading protection - when true, prevents any modifications to territories
	stateLoading bool
}

var st state

// Initialise all territories and spin up resource tick timer
func init() {
	// st cannot and never will be nil
	st = state{
		territories:    make([]*typedef.Territory, 0, 600),
		guilds:         make([]*typedef.Guild, 0, 100),        // Reduced from 5000 to 100
		activeTributes: make([]*typedef.ActiveTribute, 0, 50), // Initialize active tributes
		savedSnapshots: make([][]*typedef.Territory, 0),
		tick:           0,
		runtimeOptions: typedef.RuntimeOptions{
			TreasuryEnabled: true,
		},
		transitManager:        NewTransitManager(),
		hqMap:                 make(map[string]*typedef.Territory),
		territoryMap:          make(map[string]*typedef.Territory),
		tickQueue:             make(chan struct{}, 50000),
		useParallelProcessing: true, // Enable parallel processing by default for better performance
	}

	// Start the tick processing goroutine
	go st.processQueuedTicks()

	loadTerritories()
	loadCosts(&st)
	// fmt.Println("[ERUNTIME] Bootstrap complete. Loaded", len(st.territories), "territories and", len(st.guilds), "guilds.")

	// Start the timer for resource generation
	st.start()
	// fmt.Println("[ERUNTIME] Resource generation timer started.")

	// Attempt to load auto-save file if it exists
	if LoadAutoSave() {
		// fmt.Println("[ERUNTIME] Auto-save file loaded successfully on startup")
	} else {
		// fmt.Println("[ERUNTIME] No auto-save file found, starting with fresh state")
	}
}

// GetAllTransits returns all active transits in the system
func GetAllTransits() map[string]*Transit {
	if st.transitManager != nil {
		return st.transitManager.GetAllTransits()
	}
	return make(map[string]*Transit)
}

// RemoveTransit removes a transit by ID from the system
func RemoveTransit(transitID string) {
	if st.transitManager != nil {
		st.transitManager.removeTransit(transitID)
	}
}

// GetCurrentTick returns the current simulation tick
func GetCurrentTick() uint64 {
	return st.tick
}

// SetDebugLogging enables or disables verbose debug logging
// WARNING: Enabling debug logging at high tick rates can cause performance issues
func SetDebugLogging(enabled bool) {
	st.debugLogging = enabled
}

// GetTickQueueStatus returns information about the tick processing queue
// Useful for monitoring performance at high tick rates
func GetTickQueueStatus() (queueLength int, queueCapacity int) {
	return len(st.tickQueue), cap(st.tickQueue)
}

// GetTickQueueUtilization returns the current utilization of the tick queue as a percentage
func GetTickQueueUtilization() float64 {
	length := len(st.tickQueue)
	capacity := cap(st.tickQueue)
	if capacity == 0 {
		return 0
	}
	return float64(length) / float64(capacity) * 100.0
}

// GetTickProcessingPerformance returns performance metrics for tick processing
func GetTickProcessingPerformance() (actualTPS float64, avgTickTime time.Duration, queueUtilization float64) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return st.actualTPS, st.tickProcessTime, GetTickQueueUtilization()
}

// GetPerformanceInfo returns a formatted string with performance information
func GetPerformanceInfo() string {
	actualTPS, tickTime, queueUtil := GetTickProcessingPerformance()
	return fmt.Sprintf("Actual TPS: %.1f | Tick Time: %v | Queue: %.1f%%",
		actualTPS, tickTime, queueUtil)
}

// SetParallelProcessing enables or disables parallel territory processing
// Parallel processing can significantly improve performance at high tick rates
func SetParallelProcessing(enabled bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.useParallelProcessing = enabled
}

// IsParallelProcessingEnabled returns whether parallel processing is currently enabled
func IsParallelProcessingEnabled() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.useParallelProcessing
}

// Tick returns the current tick value for caching and UI refresh logic
func Tick() uint64 {
	return st.tick
}

// TRIBUTE SYSTEM API

// CreateNewTribute creates a new tribute between guilds or for spawning/sinking resources
func CreateNewTribute(fromGuildName, toGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (string, error) {
	tribute, err := CreateTribute(fromGuildName, toGuildName, amount, intervalMinutes)
	if err != nil {
		return "", err
	}

	err = AddTribute(tribute)
	if err != nil {
		return "", err
	}

	return tribute.ID, nil
}

// CreateResourceSpawnTribute creates a tribute that spawns resources into a guild's HQ
func CreateResourceSpawnTribute(toGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (string, error) {
	tribute, err := CreateResourceSpawn(toGuildName, amount, intervalMinutes)
	if err != nil {
		return "", err
	}

	err = AddTribute(tribute)
	if err != nil {
		return "", err
	}

	return tribute.ID, nil
}

// CreateResourceSinkTribute creates a tribute that removes resources from a guild's HQ
func CreateResourceSinkTribute(fromGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (string, error) {
	tribute, err := CreateResourceSink(fromGuildName, amount, intervalMinutes)
	if err != nil {
		return "", err
	}

	err = AddTribute(tribute)
	if err != nil {
		return "", err
	}

	return tribute.ID, nil
}

// CreateGuildToGuildTribute creates a tribute between two guilds
func CreateGuildToGuildTribute(fromGuildName, toGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (string, error) {
	tribute, err := CreateGuildTribute(fromGuildName, toGuildName, amount, intervalMinutes)
	if err != nil {
		return "", err
	}

	err = AddTribute(tribute)
	if err != nil {
		return "", err
	}

	return tribute.ID, nil
}

// GetAllActiveTributes returns all active tributes in the system
func GetAllActiveTributes() []*typedef.ActiveTribute {
	return GetActiveTributes()
}

// GetTribute returns a tribute by its ID
func GetTribute(tributeID string) *typedef.ActiveTribute {
	return GetTributeByID(tributeID)
}

// DeleteTribute removes a tribute by ID
func DeleteTribute(tributeID string) error {
	return RemoveTribute(tributeID)
}

// EnableTributeByID enables a tribute that was previously disabled
func EnableTributeByID(tributeID string) error {
	return EnableTribute(tributeID)
}

// DisableTributeByID disables a tribute without removing it
func DisableTributeByID(tributeID string) error {
	return DisableTribute(tributeID)
}
