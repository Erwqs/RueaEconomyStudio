package eruntime

import (
	"etools/typedef"
	"fmt"
	"os"
	"time"
)

// StateChangeCallback is a function type for notifications when state changes
type StateChangeCallback func()

// GuildChangeCallback is a function type for notifications when guild data changes
type GuildChangeCallback func()

// Global callback for state changes
var stateChangeCallback StateChangeCallback

// Global callback for guild changes
var guildChangeCallback GuildChangeCallback

// SetStateChangeCallback allows external packages to register for state change notifications
func SetStateChangeCallback(callback StateChangeCallback) {
	stateChangeCallback = callback
}

// SetGuildChangeCallback allows external packages to register for guild change notifications
func SetGuildChangeCallback(callback GuildChangeCallback) {
	guildChangeCallback = callback
}

// TerritoryStats represents comprehensive territory statistics for GUI display
type TerritoryStats struct {
	// Basic info
	Name  string        `json:"name"`
	Guild typedef.Guild `json:"guild"`
	HQ    bool          `json:"hq"`

	// Generation data
	BaseGeneration      typedef.BasicResources       `json:"baseGeneration"`
	CurrentGeneration   typedef.BasicResources       `json:"currentGeneration"`
	GenerationPerSecond typedef.BasicResourcesSecond `json:"generationPerSecond"`

	// Storage data
	StoredResources typedef.BasicResources `json:"storedResources"`
	StorageCapacity typedef.BasicResources `json:"storageCapacity"`

	// Cost data
	TotalCosts      typedef.BasicResources       `json:"totalCosts"`
	AffordableCosts typedef.BasicResourcesSecond `json:"affordableCosts"`

	// Timing data
	ResourceDeltaTime uint8  `json:"resourceDeltaTime"`
	EmeraldDeltaTime  uint8  `json:"emeraldDeltaTime"`
	LastResourceTick  uint64 `json:"lastResourceTick"`
	LastEmeraldTick   uint64 `json:"lastEmeraldTick"`

	// Accumulators
	ResourceAccumulator typedef.BasicResourcesSecond `json:"resourceAccumulator"`
	EmeraldAccumulator  float64                      `json:"emeraldAccumulator"`

	// Territory settings
	Tax         typedef.TerritoryTax `json:"tax"`
	Border      typedef.Border       `json:"border"`
	RoutingMode typedef.Routing      `json:"routingMode"`

	// Upgrade/Bonus levels
	Upgrades typedef.Upgrade `json:"upgrades"`
	Bonuses  typedef.Bonus   `json:"bonuses"`

	// Treasury and warnings
	GenerationBonus float64         `json:"generationBonus"`
	Warning         typedef.Warning `json:"warning"`
}

func GetAllies() map[*typedef.Guild][]*typedef.Guild {
	allies := make(map[*typedef.Guild][]*typedef.Guild)

	return allies
}

// GetTerritoryStats returns comprehensive statistics for a territory
func GetTerritoryStats(territoryName string) *TerritoryStats {
	territory := GetTerritory(territoryName)
	if territory == nil {
		return nil
	}

	// Calculate current generation
	static, perSecond, totalCosts, affordableCosts := CalculateGeneration(territory)

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	return &TerritoryStats{
		Name:  territory.Name,
		Guild: territory.Guild,
		HQ:    territory.HQ,

		BaseGeneration:      territory.ResourceGeneration.Base,
		CurrentGeneration:   static,
		GenerationPerSecond: perSecond,

		StoredResources: territory.Storage.At,
		StorageCapacity: territory.Storage.Capacity,

		TotalCosts:      totalCosts,
		AffordableCosts: affordableCosts,

		ResourceDeltaTime: territory.ResourceGeneration.ResourceDeltaTime,
		EmeraldDeltaTime:  territory.ResourceGeneration.EmeraldDeltaTime,
		LastResourceTick:  territory.ResourceGeneration.LastResourceTick,
		LastEmeraldTick:   territory.ResourceGeneration.LastEmeraldTick,

		ResourceAccumulator: territory.ResourceGeneration.ResourceAccumulator,
		EmeraldAccumulator:  territory.ResourceGeneration.EmeraldAccumulator,

		Tax:         territory.Tax,
		Border:      territory.Border,
		RoutingMode: territory.RoutingMode,

		Upgrades: territory.Options.Upgrade.Set,
		Bonuses:  territory.Options.Bonus.Set,

		GenerationBonus: territory.GenerationBonus,
		Warning:         territory.Warning,
	}
}

// GetAllTerritoryStats returns statistics for all territories
func GetAllTerritoryStats() map[string]*TerritoryStats {
	territories := GetTerritories()
	stats := make(map[string]*TerritoryStats)

	for _, territory := range territories {
		if territory != nil {
			stats[territory.Name] = GetTerritoryStats(territory.Name)
		}
	}

	return stats
}

// GetSystemStats returns overall system statistics
type SystemStats struct {
	CurrentTick      uint64 `json:"currentTick"`
	TotalTerritories int    `json:"totalTerritories"`
	Running          bool   `json:"running"`
}

func GetSystemStats() *SystemStats {
	territories := GetTerritories()
	return &SystemStats{
		CurrentTick:      st.tick,
		TotalTerritories: len(territories),
		Running:          !st.isHalted(),
	}
}

// GetResourceMovementTimer returns the time until next resource movement (in seconds)
func GetResourceMovementTimer() int {
	// Resource movement happens every 60 ticks
	nextMovement := ((st.tick / 60) + 1) * 60
	remaining := int(nextMovement - st.tick)
	return remaining
}

func GetTerritories() []*typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]*typedef.Territory, len(st.territories))
	copy(result, st.territories)
	return result
}

func GetTerritory(name string) *typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return getTerritoryUnsafe(name)
}

// getTerritoryUnsafe is an internal function that doesn't acquire locks
// Caller must ensure proper locking
func getTerritoryUnsafe(name string) *typedef.Territory {
	// Use the fast map lookup instead of linear search
	return TerritoryMap[name]
}

func SetGuild(territory string, guild typedef.Guild) *typedef.Territory {
	// Protect the entire operation with write lock to prevent concurrent access
	st.mu.Lock()
	defer st.mu.Unlock()

	// Don't allow modifications during state loading
	if st.stateLoading {
		fmt.Printf("[ERUNTIME] SetGuild blocked during state loading for territory: %s\n", territory)
		return nil
	}

	// Set territory to guild and call pathfinding to update connections
	var updatedTerritory *typedef.Territory
	for _, t := range st.territories {
		if t != nil && t.Name == territory {
			t.Mu.Lock()

			// Check if guild ownership is actually changing
			oldGuildName := t.Guild.Name
			oldGuildTag := t.Guild.Tag

			t.Guild = guild

			// If territory changes ownership, set captured time and reset treasury
			if oldGuildName != guild.Name || oldGuildTag != guild.Tag {
				t.CapturedAt = st.tick
				t.Treasury = typedef.TreasuryLevelVeryLow
				
				// If territory changes ownership, it should no longer be an HQ
				// The new guild must explicitly set a new HQ
				fmt.Printf("[HQ_DEBUG] Clearing HQ for territory %s due to guild change in SetGuild from %s[%s] to %s[%s]\n", 
					t.Name, oldGuildName, oldGuildTag, guild.Name, guild.Tag)
				t.HQ = false
			}

			t.Mu.Unlock()
			updatedTerritory = t
			break
		}
	}

	// Update all routes since territory ownership changed - safe since we have write lock
	st.updateRoute()

	// Trigger auto-save after user action
	TriggerAutoSave()

	return updatedTerritory
}

func SetGuildBatch(opts map[string]*typedef.Guild) []*typedef.Territory {
	// Protect the entire batch operation with write lock
	st.mu.Lock()
	defer st.mu.Unlock()

	// Don't allow modifications during state loading
	if st.stateLoading {
		fmt.Printf("[ERUNTIME] SetGuildBatch blocked during state loading for %d territories\n", len(opts))
		return nil
	}

	updatedTerritories := make([]*typedef.Territory, 0, len(opts))
	// dont set guilds for territory thats already have the same guild
	for territory, guild := range opts {
		t := getTerritoryUnsafe(territory) // Use unsafe version since we have the lock
		if t == nil {
			continue
		}

		t.Mu.Lock()

		if t.Guild.Name == guild.Name {
			t.Mu.Unlock()
			continue
		}

		// Check if guild ownership is actually changing
		oldGuildName := t.Guild.Name
		oldGuildTag := t.Guild.Tag

		t.Guild = *guild

		// Only clear HQ status if territory changes ownership
		// Don't clear HQ if it's just a guild update during state loading with same guild
		if oldGuildName != guild.Name || oldGuildTag != guild.Tag {
			fmt.Printf("[HQ_DEBUG] Clearing HQ for territory %s due to ownership change from %s[%s] to %s[%s]\n", 
				t.Name, oldGuildName, oldGuildTag, guild.Name, guild.Tag)
			t.HQ = false
		}

		updatedTerritories = append(updatedTerritories, t)
		t.Mu.Unlock()
	}

	// Update routes once for all changes - safe since we have write lock
	st.updateRoute()

	// Trigger auto-save after batch user action
	TriggerAutoSave()

	return updatedTerritories
}

func Set(territory string, opts typedef.TerritoryOptions) *typedef.Territory {
	// Always use write lock for Set operations to avoid lock upgrade issues
	// This prevents race conditions when multiple HQ operations happen simultaneously
	st.mu.Lock()
	defer st.mu.Unlock()

	// Don't allow modifications during state loading
	if st.stateLoading {
		fmt.Printf("[ERUNTIME] Set blocked during state loading for territory: %s\n", territory)
		return nil
	}

	t := getTerritoryUnsafe(territory) // Use internal function that doesn't acquire locks
	if t == nil {
		return nil
	}

	t.Mu.Lock()
	t.Options.Upgrade.Set = opts.Upgrades
	t.Options.Bonus.Set = opts.Bonuses

	needsRouteUpdate := false

	if opts.Border != t.Border ||
		opts.RoutingMode != t.RoutingMode ||
		opts.Tax.Tax != t.Tax.Tax ||
		opts.Tax.Ally != t.Tax.Ally {
		// Recalculate routes for ALL territories
		needsRouteUpdate = true
	}

	t.Tax = opts.Tax
	t.RoutingMode = opts.RoutingMode
	t.Border = opts.Border

	// Store HQ setting value before unlocking
	shouldSetHQ := opts.HQ && !t.HQ
	t.Mu.Unlock() // Unlock territory before route updates

	// Handle route updates and HQ setting atomically under the global write lock
	if needsRouteUpdate {
		st.updateRoute() // Safe to call since we have write lock
	}

	if shouldSetHQ {
		sethqUnsafe(t) // Call unsafe version since we already have the write lock
	}

	// Recalculate the generation potential for this territory
	_, _, _, _ = calculateGeneration(t)

	// Trigger auto-save after user action
	TriggerAutoSave()

	return t
}

func ModifyStorageState(territory string, newState typedef.BasicResourcesInterface) *typedef.Territory {
	t := GetTerritory(territory)
	t.Mu.Lock()
	t.Storage.At = newState.PerHour()
	t.Mu.Unlock()
	return t
}

func Halt() {
	st.halt()
}

func Resume() {
	st.resume()
}

func NextTick() {
	st.nexttick()
}

func StartTimer() {
	st.start()
}

func IsHalted() bool {
	return st.isHalted()
}

func SetTickRate(ticksPerSecond int) {
	st.setTickRate(ticksPerSecond)
}

func Reset() {
	fmt.Println("[ERUNTIME] Reset called - reinitializing state to zero")

	// Stop the timer to prevent tick updates during reset
	st.halt()
	if st.timerChan != nil {
		st.timerChan.Stop()
		st.timerChan = nil // Important: set to nil so start() can create a new ticker
	}

	// Acquire write lock to prevent concurrent access during reset
	st.mu.Lock()
	defer st.mu.Unlock()

	// Set loading flag to prevent any other operations during reset
	st.stateLoading = true

	// Clean up transit manager
	if st.transitManager != nil {
		// Clear all existing transits
		for _, transit := range st.transitManager.GetAllTransits() {
			st.transitManager.removeTransit(transit.ID)
		}
	}

	// Clean up territory channels and reset territory state
	for _, territory := range st.territories {
		if territory != nil {
			territory.Mu.Lock()

			// Close the territory's SetCh channel if it exists and call cleanup
			if territory.CloseCh != nil {
				territory.CloseCh()
			}

			// Reset territory to default state but keep basic info
			baseGeneration := territory.ResourceGeneration.Base

			// Reset all upgrades and bonuses to 0
			territory.Options.Upgrade.Set = typedef.Upgrade{}
			territory.Options.Upgrade.At = typedef.Upgrade{}
			territory.Options.Bonus.Set = typedef.Bonus{}
			territory.Options.Bonus.At = typedef.Bonus{}

			// Reset storage to empty
			territory.Storage.At = typedef.BasicResources{}
			territory.Storage.Capacity = typedef.BaseResourceCapacity

			// Reset to No Guild
			territory.Guild = typedef.Guild{
				Name: "No Guild",
				Tag:  "NONE",
			}

			// Reset HQ status
			fmt.Printf("[HQ_DEBUG] Clearing HQ for territory %s due to Reset operation\n", territory.Name)
			territory.HQ = false

			// Reset treasury and generation bonus
			territory.Treasury = typedef.TreasuryLevelVeryLow
			territory.GenerationBonus = 0.0
			territory.CapturedAt = 0

			// Reset tax to default (5%)
			territory.Tax = typedef.TerritoryTax{
				Tax:  0.05,
				Ally: 0.05,
			}

			// Reset routing and border settings
			territory.RoutingMode = typedef.RoutingCheapest
			territory.Border = typedef.BorderOpen
			territory.RouteTax = -1.0

			// Reset resource generation state
			territory.ResourceGeneration = typedef.ResourceGeneration{
				Base:                baseGeneration, // Keep original base generation
				At:                  baseGeneration, // Reset current generation to base
				ResourceDeltaTime:   4,
				EmeraldDeltaTime:    4,
				ResourceAccumulator: typedef.BasicResourcesSecond{},
				EmeraldAccumulator:  0,
				LastResourceTick:    0,
				LastEmeraldTick:     0,
			}

			// Clear transit resources
			territory.TransitResource = []typedef.InTransitResources{}

			// Reset warnings and costs
			territory.Warning = 0
			territory.Costs = typedef.BasicResources{}
			territory.Net = typedef.BasicResources{}

			// Reset trading routes
			territory.TradingRoutes = [][]*typedef.Territory{}
			territory.NextTerritory = nil
			territory.Destination = nil

			// Recreate the SetCh channel for future use
			setCh := make(chan typedef.TerritoryOptions, 1)
			territory.SetCh = setCh
			territory.CloseCh = func() {
				close(setCh)
			}
			territory.Reset = func() {
				// Reset function implementation if needed
			}

			territory.Mu.Unlock()
		}
	}

	// Reset global state
	st.tick = 0
	st.savedSnapshots = make([][]*typedef.Territory, 0)

	// Reset runtime options to defaults
	st.runtimeOptions = typedef.RuntimeOptions{
		TreasuryEnabled: true,
		NoKSPrompt:      false,
		EnableShm:       false,
	}

	// Recreate transit manager
	st.transitManager = NewTransitManager()

	// Clear global maps and rebuild them
	TradingRoutesMap = make(map[string][]string)
	TerritoryMap = make(map[string]*typedef.Territory)

	// Rebuild TerritoryMap and TradingRoutesMap for fast lookups
	for _, territory := range st.territories {
		if territory != nil {
			TerritoryMap[territory.Name] = territory
			// Rebuild trading routes map from territory's connected territories
			TradingRoutesMap[territory.Name] = territory.ConnectedTerritories
		}
	}

	// Update all routes to recalculate with reset state
	st.updateRoute()

	// Clear halted state and restart timer
	st.halted = false
	st.start()

	// Clear loading flag - reset is complete and ready for normal operations
	st.stateLoading = false

	// Notify territories manager to update colors after reset (asynchronously to avoid deadlock)
	go notifyTerritoryColorsUpdate()
}

func SaveState(path string) {
	fmt.Printf("[STATE] SaveState called with path: '%s'\n", path)
	if path == "" {
		// Trigger file dialogue from the app layer - this will be handled by app
		fmt.Println("[STATE] SaveState called - triggering save dialogue")
		return
	}

	fmt.Printf("[STATE] Calling SaveStateToFile with path: %s\n", path)
	err := SaveStateToFile(path)
	if err != nil {
		fmt.Printf("[STATE] Error saving state: %v\n", err)
	} else {
		fmt.Printf("[STATE] Successfully saved state to: %s\n", path)
	}
}

func LoadState(path string) {
	fmt.Printf("[STATE] LoadState called with path: '%s'\n", path)
	if path == "" {
		// Trigger file dialogue from the app layer - this will be handled by app
		fmt.Println("[STATE] LoadState called - triggering load dialogue")
		return
	}

	fmt.Printf("[STATE] Calling LoadStateFromFile with path: %s\n", path)
	err := LoadStateFromFile(path)
	if err != nil {
		fmt.Printf("[STATE] Error loading state: %v\n", err)
	} else {
		fmt.Printf("[STATE] Successfully loaded state from: %s\n", path)
	}
}

func Elapsed() uint64 {
	return st.tick
}

// LocationOf returns the location coordinates for a given territory name.
// Returns [start_x, start_y] and [end_x, end_y] as [2][2]int.
// Returns empty coordinates if territory is not found.
func LocationOf(territoryName string) [2][2]int {
	territory := GetTerritory(territoryName)
	if territory == nil {
		return [2][2]int{{0, 0}, {0, 0}}
	}

	territory.Mu.RLock()
	defer territory.Mu.RUnlock()

	return [2][2]int{
		{territory.Location.Start[0], territory.Location.Start[1]},
		{territory.Location.End[0], territory.Location.End[1]},
	}
}

// GetTradingRoutes returns the trading routes map
func GetTradingRoutes() map[string][]string {
	return TradingRoutesMap
}

// GetTradingRoutesForTerritory returns the trading routes for a specific territory
func GetTradingRoutesForTerritory(territoryName string) []string {
	if routes, exists := TradingRoutesMap[territoryName]; exists {
		return routes
	}
	return []string{}
}

// GetAllGuilds returns a list of all guild names
func GetAllGuilds() []string {
	var guildNames []string
	guildNames = append(guildNames, "No Guild [NONE]") // Always include the no guild option first

	for _, guild := range st.guilds {
		if guild != nil && guild.Name != "" && guild.Name != "No Guild [NONE]" {
			guildNames = append(guildNames, guild.Name)
		}
	}

	return guildNames
}

// GetUpgradeCost returns the cost for a specific upgrade at a given level
func GetUpgradeCost(upgradeType string, level int) (int, string) {
	if level < 0 || level >= len(st.costs.UpgradesCost.Damage.Value) {
		return 0, ""
	}

	switch upgradeType {
	case "damage":
		return st.costs.UpgradesCost.Damage.Value[level], st.costs.UpgradesCost.Damage.ResourceType
	case "attack":
		return st.costs.UpgradesCost.Attack.Value[level], st.costs.UpgradesCost.Attack.ResourceType
	case "health":
		return st.costs.UpgradesCost.Health.Value[level], st.costs.UpgradesCost.Health.ResourceType
	case "defence":
		return st.costs.UpgradesCost.Defence.Value[level], st.costs.UpgradesCost.Defence.ResourceType
	default:
		return 0, ""
	}
}

// SetTerritoryUpgrade sets a single upgrade for a territory
func SetTerritoryUpgrade(territoryName string, upgradeType string, level int) *typedef.Territory {
	territory := GetTerritory(territoryName)
	if territory == nil {
		return nil
	}

	// Clamp level to valid range
	if level < 0 {
		level = 0
	}
	if level > 11 {
		level = 11
	}

	// Create new options based on current territory settings
	opts := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	// Update the specific upgrade
	switch upgradeType {
	case "damage":
		opts.Upgrades.Damage = level
	case "attack":
		opts.Upgrades.Attack = level
	case "health":
		opts.Upgrades.Health = level
	case "defence":
		opts.Upgrades.Defence = level
	default:
		return territory // No change for invalid upgrade type
	}

	// Apply the changes using the existing Set function
	return Set(territoryName, opts)
}

// SetTerritoryBonus sets a single bonus for a territory
func SetTerritoryBonus(territoryName string, bonusType string, level int) *typedef.Territory {
	territory := GetTerritory(territoryName)
	if territory == nil {
		return nil
	}

	// Clamp level to valid range (bonus levels typically go from 0 to MaxLevel)
	if level < 0 {
		level = 0
	}

	// Get max level for this bonus type
	costs := GetCost()
	maxLevel := getBonusMaxLevelFromCosts(costs, bonusType)
	if level > maxLevel {
		level = maxLevel
	}

	// Create new options based on current territory settings
	opts := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	// Update the specific bonus
	switch bonusType {
	case "strongerMinions":
		opts.Bonuses.StrongerMinions = level
	case "towerMultiAttack":
		// Enforce max 5 per guild
		if level > 0 {
			guildName := territory.Guild.Name
			count := 0
			for _, t := range GetTerritories() {
				if t != nil && t.Guild.Name == guildName && t.Options.Bonus.Set.TowerMultiAttack > 0 {
					if t.Name != territory.Name || t.Options.Bonus.Set.TowerMultiAttack > 0 {
						count++
					}
				}
			}
			if count >= 5 && territory.Options.Bonus.Set.TowerMultiAttack == 0 {
				panic("Cannot enable Tower Multi-Attack on more than 5 territories per guild!")
			}
		}
		opts.Bonuses.TowerMultiAttack = level
	case "towerAura":
		opts.Bonuses.TowerAura = level
	case "towerVolley":
		opts.Bonuses.TowerVolley = level
	case "largerResourceStorage":
		opts.Bonuses.LargerResourceStorage = level
	case "largerEmeraldStorage":
		opts.Bonuses.LargerEmeraldStorage = level
	case "efficientResource":
		opts.Bonuses.EfficientResource = level
	case "efficientEmerald":
		opts.Bonuses.EfficientEmerald = level
	case "resourceRate":
		opts.Bonuses.ResourceRate = level
	case "emeraldRate":
		opts.Bonuses.EmeraldRate = level
	default:
		return territory // No change for invalid bonus type
	}

	// Apply the changes using the existing Set function
	return Set(territoryName, opts)
}

func getBonusMaxLevelFromCosts(costs *typedef.Costs, bonusType string) int {
	switch bonusType {
	case "strongerMinions":
		return costs.Bonuses.StrongerMinions.MaxLevel
	case "towerMultiAttack":
		return costs.Bonuses.TowerMultiAttack.MaxLevel
	case "towerAura":
		return costs.Bonuses.TowerAura.MaxLevel
	case "towerVolley":
		return costs.Bonuses.TowerVolley.MaxLevel
	case "largerResourceStorage":
		return costs.Bonuses.LargerResourceStorage.MaxLevel
	case "largerEmeraldStorage":
		return costs.Bonuses.LargerEmeraldsStorage.MaxLevel
	case "efficientResource":
		return costs.Bonuses.EfficientResource.MaxLevel
	case "efficientEmerald":
		return costs.Bonuses.EfficientEmeralds.MaxLevel
	case "resourceRate":
		return costs.Bonuses.ResourceRate.MaxLevel
	case "emeraldRate":
		return costs.Bonuses.EmeraldsRate.MaxLevel
	default:
		return 10 // Default fallback
	}
}

// GetBonusCost returns the cost and resource type for a specific bonus at a given level
func GetBonusCost(bonusType string, level int) (int, string) {
	costs := GetCost()

	var bonusCosts typedef.BonusCosts
	switch bonusType {
	case "strongerMinions":
		bonusCosts = costs.Bonuses.StrongerMinions
	case "towerMultiAttack":
		bonusCosts = costs.Bonuses.TowerMultiAttack
	case "towerAura":
		bonusCosts = costs.Bonuses.TowerAura
	case "towerVolley":
		bonusCosts = costs.Bonuses.TowerVolley
	case "largerResourceStorage":
		bonusCosts = costs.Bonuses.LargerResourceStorage
	case "largerEmeraldStorage":
		bonusCosts = costs.Bonuses.LargerEmeraldsStorage
	case "efficientResource":
		bonusCosts = costs.Bonuses.EfficientResource
	case "efficientEmerald":
		bonusCosts = costs.Bonuses.EfficientEmeralds
	case "resourceRate":
		bonusCosts = costs.Bonuses.ResourceRate
	case "emeraldRate":
		bonusCosts = costs.Bonuses.EmeraldsRate
	default:
		return 0, ""
	}

	// Return cost for the given level
	if level >= 0 && level < len(bonusCosts.Cost) {
		return bonusCosts.Cost[level], bonusCosts.ResourceType
	}

	return 0, bonusCosts.ResourceType
}

func GetCost() *typedef.Costs {
	return &st.costs
}

// Auto-save functionality
var lastAutoSaveTime time.Time
var autoSaveEnabled = true

// EnableAutoSave enables or disables auto-save functionality
func EnableAutoSave(enabled bool) {
	autoSaveEnabled = enabled
	if enabled {
		fmt.Println("[AUTOSAVE] Auto-save enabled")
	} else {
		fmt.Println("[AUTOSAVE] Auto-save disabled")
	}
}

// IsAutoSaveEnabled returns whether auto-save is currently enabled
func IsAutoSaveEnabled() bool {
	return autoSaveEnabled
}

// TriggerAutoSave performs an auto-save if enough time has passed and auto-save is enabled
func TriggerAutoSave() {
	if !autoSaveEnabled {
		return
	}

	// Don't auto-save during state loading to prevent corruption
	if st.stateLoading {
		return
	}

	now := time.Now()
	// Only auto-save if at least 5 seconds have passed since last auto-save
	// This prevents excessive auto-saving during rapid user actions
	if now.Sub(lastAutoSaveTime) < 5*time.Second {
		return
	}

	// Perform auto-save in a goroutine to avoid blocking the main thread
	go func() {
		err := SaveStateToFile("autosave.lz4")
		if err != nil {
			fmt.Printf("[AUTOSAVE] Failed to auto-save: %v\n", err)
		} else {
			fmt.Println("[AUTOSAVE] Auto-save completed successfully")
		}
		lastAutoSaveTime = time.Now()
	}()
}

// LoadAutoSave attempts to load the auto-save file if it exists
func LoadAutoSave() bool {
	// Check if autosave.lz4 exists
	if _, err := os.Stat("autosave.lz4"); os.IsNotExist(err) {
		fmt.Println("[AUTOSAVE] No auto-save file found")
		return false
	}

	fmt.Println("[AUTOSAVE] Auto-save file found, loading...")
	err := LoadStateFromFile("autosave.lz4")
	if err != nil {
		fmt.Printf("[AUTOSAVE] Failed to load auto-save: %v\n", err)
		return false
	}

	fmt.Println("[AUTOSAVE] Auto-save loaded successfully")
	return true
}

// notifyTerritoryColorsUpdate triggers territory color update in the app layer
func notifyTerritoryColorsUpdate() {
	if stateChangeCallback != nil {
		stateChangeCallback()
		fmt.Println("[ERUNTIME] Territory colors update notification sent")
	}
}

// notifyGuildManagerUpdate triggers guild data update in the app layer
func notifyGuildManagerUpdate() {
	if guildChangeCallback != nil {
		guildChangeCallback()
		fmt.Println("[ERUNTIME] Guild manager update notification sent")
	}
}
