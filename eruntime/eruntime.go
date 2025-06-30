package eruntime

import (
	"etools/typedef"
	"fmt"
	"sync"
	"time"
)

// World state for specific session
// This is the main runtime for the game, it will handle all the logic and state of the game
// and will be used for guild economy calculations
type state struct {
	mu          sync.RWMutex // Protects all state fields from concurrent access
	territories []*typedef.Territory
	guilds      []*typedef.Guild

	savedSnapshots [][]*typedef.Territory

	tick uint64 // tick elapsed since start

	// time.Ticker
	timerChan *time.Ticker
	halted    bool

	costs typedef.Costs

	runtimeOptions typedef.RuntimeOptions

	// Transit system for managing resource movement
	transitManager *TransitManager

	// No need, each territory has its own mutex
	// mu sync.Mutex // mutex to protect state changes
}

var st state

// Initialise all territories and spin up resource tick timer
func init() {
	// Read from territories.json
	st = state{
		territories:    make([]*typedef.Territory, 0, 600),
		guilds:         make([]*typedef.Guild, 0, 100), // Reduced from 5000 to 100
		savedSnapshots: make([][]*typedef.Territory, 0),
		tick:           0,
		runtimeOptions: typedef.RuntimeOptions{
			TreasuryEnabled: true,
		},
		transitManager: NewTransitManager(),
	}

	loadTerritories()
	loadCosts(&st)
	fmt.Println("[ERUNTIME] Bootstrap complete. Loaded", len(st.territories), "territories and", len(st.guilds), "guilds.")

	// Start the timer for resource generation
	st.start()
	fmt.Println("[ERUNTIME] Resource generation timer started.")
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
