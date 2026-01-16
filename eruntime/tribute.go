/*
TRIBUTE SYSTEM - STATE TICK ALIGNED VERSION

The tribute system has been refactored to work directly with the game's state tick
and align with the 60-tick cycle used by the transit system.

KEY CHANGES:
- IntervalTicks (uint64) → IntervalMinutes (uint32)
- NextTransfer (uint64) → LastTransfer (uint64)
- Tributes now process only on 60-tick boundaries (every minute)
- Timing is based on real game minutes rather than arbitrary tick intervals

HOW IT WORKS:
1. Tributes are processed every 60 ticks (1 minute) in update2()
2. Each tribute has an IntervalMinutes field (e.g., 5 = every 5 minutes)
3. The system calculates if enough minutes have passed since the last transfer
4. All resource movement uses the existing transit system with proper pathfinding

BENEFITS:
- Perfectly aligned with transit system timing (60-tick cycles)
- Simpler logic (no complex NextTransfer calculations)
- More predictable behavior (always processes on minute boundaries)
- Better integration with existing resource movement infrastructure
- Easier to understand and debug

TIMING EXAMPLES:
- IntervalMinutes = 1: Transfer every minute (every 60 ticks)
- IntervalMinutes = 5: Transfer every 5 minutes (every 300 ticks)
- IntervalMinutes = 60: Transfer every hour (every 3600 ticks)
*/

package eruntime

import (
	"RueaES/typedef"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// TributeManager manages all active tributes in the system
type TributeManager struct {
	tributes map[string]*typedef.ActiveTribute
}

// NewTributeManager creates a new tribute manager
func NewTributeManager() *TributeManager {
	return &TributeManager{
		tributes: make(map[string]*typedef.ActiveTribute),
	}
}

// generateTributeID generates a unique ID for a tribute
func generateTributeID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "tribute_" + hex.EncodeToString(bytes)
}

// CreateTribute creates a new tribute between guilds
// fromGuildName: source guild (can be empty string for spawned resources)
// toGuildName: destination guild (can be empty string for resource sink)
// amountPerHour: resources to transfer per hour (user-friendly input)
// intervalMinutes: how often to transfer (in minutes, aligned with 60-tick cycles)
func CreateTribute(fromGuildName, toGuildName string, amountPerHour typedef.BasicResources, intervalMinutes uint32) (*typedef.ActiveTribute, error) {
	if intervalMinutes == 0 {
		return nil, fmt.Errorf("interval minutes must be greater than 0")
	}

	var fromGuild, toGuild *typedef.Guild

	// Find source guild if specified
	if fromGuildName != "" {
		fromGuild = GetGuildByName(fromGuildName)
		if fromGuild == nil {
			return nil, fmt.Errorf("source guild '%s' not found", fromGuildName)
		}
	}

	// Find destination guild if specified
	if toGuildName != "" {
		toGuild = GetGuildByName(toGuildName)
		if toGuild == nil {
			return nil, fmt.Errorf("destination guild '%s' not found", toGuildName)
		}
	}

	// Validate that at least one guild is specified
	if fromGuildName == "" && toGuildName == "" {
		return nil, fmt.Errorf("at least one guild must be specified")
	}

	// Convert hourly amounts to per-minute amounts for the transit system
	amountPerMinute := typedef.BasicResources{
		Emeralds: amountPerHour.Emeralds / 60.0,
		Ores:     amountPerHour.Ores / 60.0,
		Wood:     amountPerHour.Wood / 60.0,
		Fish:     amountPerHour.Fish / 60.0,
		Crops:    amountPerHour.Crops / 60.0,
	}

	tribute := &typedef.ActiveTribute{
		ID:              generateTributeID(),
		From:            fromGuild,
		FromGuildName:   fromGuildName,
		To:              toGuild,
		ToGuildName:     toGuildName,
		AmountPerHour:   amountPerHour,   // Store original hourly input
		AmountPerMinute: amountPerMinute, // Store calculated per-minute amount
		IntervalMinutes: intervalMinutes,
		LastTransfer:    0, // Never transferred yet
		IsActive:        true,
		CreatedAt:       st.tick,
	}

	return tribute, nil
}

// AddTribute adds a tribute to the active tributes list
func AddTribute(tribute *typedef.ActiveTribute) error {
	if tribute == nil {
		return fmt.Errorf("tribute cannot be nil")
	}

	if tribute.AmountPerHour.Crops < 0 ||
		tribute.AmountPerHour.Fish < 0 ||
		tribute.AmountPerHour.Ores < 0 ||
		tribute.AmountPerHour.Wood < 0 ||
		tribute.AmountPerHour.Emeralds < 0 {
		return fmt.Errorf("tribute amounts cannot be negative")
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	st.activeTributes = append(st.activeTributes, tribute)
	debugf("Added tribute %s from %s to %s\n", tribute.ID, tribute.FromGuildName, tribute.ToGuildName)

	// Recalculate guild tribute totals
	recalculateGuildTributes()

	return nil
}

// RemoveTribute removes a tribute by ID
func RemoveTribute(tributeID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	for i, tribute := range st.activeTributes {
		if tribute.ID == tributeID {
			// Remove from slice
			st.activeTributes = append(st.activeTributes[:i], st.activeTributes[i+1:]...)
			debugf("Removed tribute %s\n", tributeID)

			// Recalculate guild tribute totals
			recalculateGuildTributes()

			return nil
		}
	}

	return fmt.Errorf("tribute with ID '%s' not found", tributeID)
}

// GetActiveTributes returns all active tributes
func GetActiveTributes() []*typedef.ActiveTribute {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make([]*typedef.ActiveTribute, len(st.activeTributes))
	copy(result, st.activeTributes)
	return result
}

// GetTributeByID returns a tribute by its ID
func GetTributeByID(tributeID string) *typedef.ActiveTribute {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, tribute := range st.activeTributes {
		if tribute.ID == tributeID {
			return tribute
		}
	}
	return nil
}

// DisableTribute disables a tribute without removing it
func DisableTribute(tributeID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	for _, tribute := range st.activeTributes {
		if tribute.ID == tributeID {
			tribute.IsActive = false
			debugf("Disabled tribute %s\n", tributeID)

			// Recalculate guild tribute totals
			recalculateGuildTributes()

			return nil
		}
	}

	return fmt.Errorf("tribute with ID '%s' not found", tributeID)
}

// EnableTribute enables a previously disabled tribute
func EnableTribute(tributeID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	for _, tribute := range st.activeTributes {
		if tribute.ID == tributeID {
			tribute.IsActive = true
			tribute.LastTransfer = 0 // Reset transfer history when re-enabling
			debugf("Enabled tribute %s\n", tributeID)

			// Recalculate guild tribute totals
			recalculateGuildTributes()

			return nil
		}
	}

	return fmt.Errorf("tribute with ID '%s' not found", tributeID)
}

// UpdateTribute updates an existing tribute's amounts or interval.
// A nil amount or interval leaves that field unchanged.
func UpdateTribute(tributeID string, amount *typedef.BasicResources, interval *uint32) error {
	if amount == nil && interval == nil {
		return fmt.Errorf("no update payload provided")
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	var tribute *typedef.ActiveTribute
	for _, t := range st.activeTributes {
		if t != nil && t.ID == tributeID {
			tribute = t
			break
		}
	}

	if tribute == nil {
		return fmt.Errorf("tribute with ID '%s' not found", tributeID)
	}

	if amount != nil {
		if amount.Crops < 0 || amount.Fish < 0 || amount.Ores < 0 || amount.Wood < 0 || amount.Emeralds < 0 {
			return fmt.Errorf("tribute amounts cannot be negative")
		}
		tribute.AmountPerHour = *amount
		tribute.AmountPerMinute = typedef.BasicResources{
			Emeralds: amount.Emeralds / 60.0,
			Ores:     amount.Ores / 60.0,
			Wood:     amount.Wood / 60.0,
			Fish:     amount.Fish / 60.0,
			Crops:    amount.Crops / 60.0,
		}
	}

	if interval != nil {
		if *interval == 0 {
			return fmt.Errorf("interval minutes must be greater than 0")
		}
		tribute.IntervalMinutes = *interval
	}

	recalculateGuildTributes()
	return nil
}

// processTributes handles tribute transfers on each 60-tick cycle (called from update2)
func (s *state) processTributes() {
	if len(s.activeTributes) == 0 {
		return
	}

	// Only process tributes on 60-tick boundaries (every minute)
	if s.tick%60 != 0 {
		return
	}

	debugf("processTributes: Checking %d active tributes at tick %d (minute boundary)\n", len(s.activeTributes), s.tick)

	// Calculate how many minutes have passed since game start
	currentMinute := s.tick / 60

	for i, tribute := range s.activeTributes {
		if tribute == nil {
			continue
		}

		debugf("Tribute %d: ID=%s, Active=%v, IntervalMinutes=%d, LastTransfer=%d, CurrentMinute=%d\n",
			i, tribute.ID, tribute.IsActive, tribute.IntervalMinutes, tribute.LastTransfer, currentMinute)

		if !tribute.IsActive {
			debugf("Tribute %s is not active, skipping\n", tribute.ID)
			continue
		}

		// Check if enough time has passed since last transfer
		var shouldTransfer bool
		if tribute.LastTransfer == 0 {
			// First transfer - use creation time
			minutesSinceCreation := currentMinute - (tribute.CreatedAt / 60)
			shouldTransfer = minutesSinceCreation >= uint64(tribute.IntervalMinutes)
		} else {
			// Subsequent transfers - use last transfer time
			minutesSinceLastTransfer := currentMinute - (tribute.LastTransfer / 60)
			shouldTransfer = minutesSinceLastTransfer >= uint64(tribute.IntervalMinutes)
		}

		if !shouldTransfer {
			debugf("Tribute %s not ready yet (interval: %d minutes), skipping\n",
				tribute.ID, tribute.IntervalMinutes)
			continue
		}

		debugf("Processing tribute %s at tick %d (minute %d)\n", tribute.ID, s.tick, currentMinute)

		// Process this tribute transfer
		err := s.processTributeTransfer(tribute)
		if err != nil {
			debugf("Error processing tribute %s: %v\n", tribute.ID, err)
			continue
		}

		// Update last transfer time
		tribute.LastTransfer = s.tick
		debugf("Updated LastTransfer for tribute %s to tick %d\n", tribute.ID, tribute.LastTransfer)
	}
}

// processTributeTransfer handles a single tribute transfer
func (s *state) processTributeTransfer(tribute *typedef.ActiveTribute) error {
	debugf("Processing tribute %s: %s -> %s\n", tribute.ID, tribute.FromGuildName, tribute.ToGuildName)
	debugf("Tribute pointers: From=%v, To=%v\n", tribute.From != nil, tribute.To != nil)

	// Case 1: Resource spawn (From is nil, To is specified)
	if tribute.From == nil && tribute.To != nil {
		debugf("Processing as resource spawn (case 1)\n")
		return s.spawnResourcesToGuild(tribute)
	}

	// Case 2: Resource sink (From is specified, To is nil)
	if tribute.From != nil && tribute.To == nil {
		debugf("Processing as resource sink (case 2)\n")
		return s.removeResourcesFromGuild(tribute)
	}

	// Case 3: Guild to guild transfer (both specified)
	if tribute.From != nil && tribute.To != nil {
		debugf("Processing as guild-to-guild transfer (case 3)\n")
		return s.transferResourcesBetweenGuilds(tribute)
	}

	debugf("Invalid tribute configuration: From=%v, To=%v\n", tribute.From != nil, tribute.To != nil)
	return fmt.Errorf("invalid tribute configuration: both guilds cannot be nil")
}

// spawnResourcesToGuild adds resources directly to a guild's HQ
func (s *state) spawnResourcesToGuild(tribute *typedef.ActiveTribute) error {
	toHQ := s.findGuildHQ(tribute.To)
	if toHQ == nil {
		debugf("Guild %s has no HQ, cannot spawn resources\n", tribute.To.Name)
		return fmt.Errorf("guild %s has no HQ", tribute.To.Name)
	}

	toHQ.Mu.Lock()
	defer toHQ.Mu.Unlock()

	// Calculate actual amount to transfer based on interval
	// This gives us the proper amount for the time interval
	actualAmount := typedef.BasicResources{
		Emeralds: tribute.AmountPerMinute.Emeralds * float64(tribute.IntervalMinutes),
		Ores:     tribute.AmountPerMinute.Ores * float64(tribute.IntervalMinutes),
		Wood:     tribute.AmountPerMinute.Wood * float64(tribute.IntervalMinutes),
		Fish:     tribute.AmountPerMinute.Fish * float64(tribute.IntervalMinutes),
		Crops:    tribute.AmountPerMinute.Crops * float64(tribute.IntervalMinutes),
	}

	// Add resources to HQ storage
	toHQ.Storage.At.Emeralds += actualAmount.Emeralds
	toHQ.Storage.At.Ores += actualAmount.Ores
	toHQ.Storage.At.Wood += actualAmount.Wood
	toHQ.Storage.At.Fish += actualAmount.Fish
	toHQ.Storage.At.Crops += actualAmount.Crops

	debugf("Spawned %+v resources to %s HQ (%s)\n", actualAmount, tribute.To.Name, toHQ.Name)
	return nil
}

// removeResourcesFromGuild removes resources from a guild's HQ
func (s *state) removeResourcesFromGuild(tribute *typedef.ActiveTribute) error {
	fromHQ := s.findGuildHQ(tribute.From)
	if fromHQ == nil {
		debugf("Guild %s has no HQ, cannot remove resources\n", tribute.From.Name)
		return fmt.Errorf("guild %s has no HQ", tribute.From.Name)
	}

	fromHQ.Mu.Lock()
	defer fromHQ.Mu.Unlock()

	// Calculate actual amount to remove based on interval
	actualAmount := typedef.BasicResources{
		Emeralds: tribute.AmountPerMinute.Emeralds * float64(tribute.IntervalMinutes),
		Ores:     tribute.AmountPerMinute.Ores * float64(tribute.IntervalMinutes),
		Wood:     tribute.AmountPerMinute.Wood * float64(tribute.IntervalMinutes),
		Fish:     tribute.AmountPerMinute.Fish * float64(tribute.IntervalMinutes),
		Crops:    tribute.AmountPerMinute.Crops * float64(tribute.IntervalMinutes),
	}

	// Check if guild has enough resources
	if fromHQ.Storage.At.Emeralds < actualAmount.Emeralds ||
		fromHQ.Storage.At.Ores < actualAmount.Ores ||
		fromHQ.Storage.At.Wood < actualAmount.Wood ||
		fromHQ.Storage.At.Fish < actualAmount.Fish ||
		fromHQ.Storage.At.Crops < actualAmount.Crops {
		debugf("Guild %s HQ (%s) doesn't have enough resources for tribute %s\n", tribute.From.Name, fromHQ.Name, tribute.ID)
		return nil // Don't error, just skip this transfer
	}

	// Remove resources from HQ storage
	fromHQ.Storage.At.Emeralds -= actualAmount.Emeralds
	fromHQ.Storage.At.Ores -= actualAmount.Ores
	fromHQ.Storage.At.Wood -= actualAmount.Wood
	fromHQ.Storage.At.Fish -= actualAmount.Fish
	fromHQ.Storage.At.Crops -= actualAmount.Crops

	debugf("Removed %+v resources from %s HQ (%s)\n", actualAmount, tribute.From.Name, fromHQ.Name)
	return nil
}

// transferResourcesBetweenGuilds handles guild-to-guild resource transfer
func (s *state) transferResourcesBetweenGuilds(tribute *typedef.ActiveTribute) error {
	debugf("transferResourcesBetweenGuilds: Processing tribute %s from guild %s to guild %s\n",
		tribute.ID, tribute.FromGuildName, tribute.ToGuildName)

	if tribute.From == nil {
		debugf("transferResourcesBetweenGuilds: tribute.From is nil for guild name %s\n", tribute.FromGuildName)
		return fmt.Errorf("source guild pointer is nil for guild %s", tribute.FromGuildName)
	}
	if tribute.To == nil {
		debugf("transferResourcesBetweenGuilds: tribute.To is nil for guild name %s\n", tribute.ToGuildName)
		return fmt.Errorf("destination guild pointer is nil for guild %s", tribute.ToGuildName)
	}

	fromHQ := s.findGuildHQ(tribute.From)
	toHQ := s.findGuildHQ(tribute.To)

	if fromHQ == nil {
		debugf("transferResourcesBetweenGuilds: No HQ found for source guild %s (tag: %s)\n",
			tribute.From.Name, tribute.From.Tag)
		return fmt.Errorf("source guild %s has no HQ", tribute.From.Name)
	}
	if toHQ == nil {
		debugf("transferResourcesBetweenGuilds: No HQ found for destination guild %s (tag: %s)\n",
			tribute.To.Name, tribute.To.Tag)
		return fmt.Errorf("destination guild %s has no HQ", tribute.To.Name)
	}

	debugf("transferResourcesBetweenGuilds: Found HQs - From: %s (%s), To: %s (%s)\n",
		fromHQ.Name, fromHQ.Guild.Name, toHQ.Name, toHQ.Guild.Name)

	// Calculate actual amount to transfer based on interval
	actualAmount := typedef.BasicResources{
		Emeralds: tribute.AmountPerMinute.Emeralds * float64(tribute.IntervalMinutes),
		Ores:     tribute.AmountPerMinute.Ores * float64(tribute.IntervalMinutes),
		Wood:     tribute.AmountPerMinute.Wood * float64(tribute.IntervalMinutes),
		Fish:     tribute.AmountPerMinute.Fish * float64(tribute.IntervalMinutes),
		Crops:    tribute.AmountPerMinute.Crops * float64(tribute.IntervalMinutes),
	}

	// Check if source guild has enough resources
	fromHQ.Mu.RLock()
	hasEnoughResources := fromHQ.Storage.At.Emeralds >= actualAmount.Emeralds &&
		fromHQ.Storage.At.Ores >= actualAmount.Ores &&
		fromHQ.Storage.At.Wood >= actualAmount.Wood &&
		fromHQ.Storage.At.Fish >= actualAmount.Fish &&
		fromHQ.Storage.At.Crops >= actualAmount.Crops
	fromHQ.Mu.RUnlock()

	if !hasEnoughResources {
		debugf("Guild %s HQ (%s) doesn't have enough resources for tribute to %s\n",
			tribute.From.Name, fromHQ.Name, tribute.To.Name)
		return nil // Don't error, just skip this transfer
	}

	// Find route from source HQ to destination HQ
	route := s.findTributeRoute(fromHQ, toHQ)
	if route == nil {
		debugf("No route found from %s HQ to %s HQ for tribute %s\n",
			tribute.From.Name, tribute.To.Name, tribute.ID)
		return fmt.Errorf("no route found between guild HQs")
	}

	// Remove resources from source HQ
	fromHQ.Mu.Lock()
	fromHQ.Storage.At.Emeralds -= actualAmount.Emeralds
	fromHQ.Storage.At.Ores -= actualAmount.Ores
	fromHQ.Storage.At.Wood -= actualAmount.Wood
	fromHQ.Storage.At.Fish -= actualAmount.Fish
	fromHQ.Storage.At.Crops -= actualAmount.Crops
	fromHQ.Mu.Unlock()

	// Create transit using existing transit system
	transitID := s.transitManager.StartTransit(actualAmount, fromHQ.ID, toHQ.ID, route)

	debugf("Created tribute transit %s from %s to %s with %+v resources\n",
		transitID, tribute.From.Name, tribute.To.Name, actualAmount)

	return nil
}

// findGuildHQ finds the HQ territory for a guild
func (s *state) findGuildHQ(guild *typedef.Guild) *typedef.Territory {
	if guild == nil {
		return nil
	}

	// Use the existing HQ map for fast lookup
	return s.hqMap[guild.Tag]
}

// findTributeRoute finds a route between two HQs using ally tax rates
func (s *state) findTributeRoute(fromHQ, toHQ *typedef.Territory) []string {
	// Use the new dedicated tribute pathfinding function
	route, err := FindTributeRoute(fromHQ, toHQ)
	if err != nil {
		debugf("FindTributeRoute failed: %v\n", err)
		return nil
	}

	debugf("Found tribute route with %d territories\n", len(route))
	return route
}

// GetGuildByName finds a guild by name
func GetGuildByName(guildName string) *typedef.Guild {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, guild := range st.guilds {
		if guild != nil && guild.Name == guildName {
			return guild
		}
	}
	return nil
}

// Convenience functions for creating specific types of tributes

// CreateResourceSpawn creates a tribute that spawns resources into a guild's HQ
func CreateResourceSpawn(toGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (*typedef.ActiveTribute, error) {
	return CreateTribute("", toGuildName, amount, intervalMinutes)
}

// CreateResourceSink creates a tribute that removes resources from a guild's HQ
func CreateResourceSink(fromGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (*typedef.ActiveTribute, error) {
	return CreateTribute(fromGuildName, "", amount, intervalMinutes)
}

// CreateGuildTribute creates a tribute between two guilds
func CreateGuildTribute(fromGuildName, toGuildName string, amount typedef.BasicResources, intervalMinutes uint32) (*typedef.ActiveTribute, error) {
	return CreateTribute(fromGuildName, toGuildName, amount, intervalMinutes)
}

// rebuildTributeGuildPointers restores guild pointers in tributes after state loading
func rebuildTributeGuildPointers() {
	for _, tribute := range st.activeTributes {
		if tribute == nil {
			continue
		}

		// Restore From guild pointer
		if tribute.FromGuildName != "" {
			tribute.From = GetGuildByNameUnsafe(tribute.FromGuildName)
			if tribute.From == nil {
				debugf("Warning: Could not find guild '%s' for tribute %s From field\n", tribute.FromGuildName, tribute.ID)
			}
		}

		// Restore To guild pointer
		if tribute.ToGuildName != "" {
			tribute.To = GetGuildByNameUnsafe(tribute.ToGuildName)
			if tribute.To == nil {
				debugf("Warning: Could not find guild '%s' for tribute %s To field\n", tribute.ToGuildName, tribute.ID)
			}
		}
	}

	debugf("Rebuilt guild pointers for %d tributes\n", len(st.activeTributes))

	// Recalculate guild tribute totals after rebuilding pointers
	recalculateGuildTributes()
}

// ValidateAndFixTributePointers checks and fixes guild pointers in tributes
func ValidateAndFixTributePointers() {
	st.mu.Lock()
	defer st.mu.Unlock()

	fixedCount := 0
	for _, tribute := range st.activeTributes {
		if tribute == nil {
			continue
		}

		fixed := false

		// Fix From guild pointer if missing
		if tribute.FromGuildName != "" && tribute.From == nil {
			tribute.From = GetGuildByNameUnsafe(tribute.FromGuildName)
			if tribute.From != nil {
				debugf("Fixed From guild pointer for tribute %s (guild: %s)\n", tribute.ID, tribute.FromGuildName)
				fixed = true
			} else {
				debugf("Warning: Could not fix From guild pointer for tribute %s (guild: %s not found)\n", tribute.ID, tribute.FromGuildName)
			}
		}

		// Fix To guild pointer if missing
		if tribute.ToGuildName != "" && tribute.To == nil {
			tribute.To = GetGuildByNameUnsafe(tribute.ToGuildName)
			if tribute.To != nil {
				debugf("Fixed To guild pointer for tribute %s (guild: %s)\n", tribute.ID, tribute.ToGuildName)
				fixed = true
			} else {
				debugf("Warning: Could not fix To guild pointer for tribute %s (guild: %s not found)\n", tribute.ID, tribute.ToGuildName)
			}
		}

		if fixed {
			fixedCount++
		}
	}

	if fixedCount > 0 {
		debugf("Fixed guild pointers for %d tributes\n", fixedCount)
		recalculateGuildTributes()
	}
}

// recalculateGuildTributes updates TributeIn and TributeOut totals for all guilds
// Note: This function expects the mutex to already be held by the caller
func recalculateGuildTributes() {
	// Reset all guild tribute totals
	for _, guild := range st.guilds {
		if guild != nil {
			guild.TributeIn = typedef.BasicResources{}
			guild.TributeOut = typedef.BasicResources{}
		}
	}

	// Calculate tribute totals from active tributes
	for _, tribute := range st.activeTributes {
		if tribute == nil || !tribute.IsActive {
			continue
		}

		// For UI display, we want to show the per-minute rate
		// This is already calculated and stored in AmountPerMinute
		minuteAmount := tribute.AmountPerMinute

		debugf("Tribute %s: interval=%d minutes, per-minute amounts: %+v\n",
			tribute.ID, tribute.IntervalMinutes, minuteAmount)

		// Add to source guild's TributeOut
		if tribute.FromGuildName != "" {
			fromGuild := GetGuildByNameUnsafe(tribute.FromGuildName)
			if fromGuild != nil {
				fromGuild.TributeOut.Emeralds += minuteAmount.Emeralds
				fromGuild.TributeOut.Ores += minuteAmount.Ores
				fromGuild.TributeOut.Wood += minuteAmount.Wood
				fromGuild.TributeOut.Fish += minuteAmount.Fish
				fromGuild.TributeOut.Crops += minuteAmount.Crops
				debugf("Added TributeOut to guild %s: %+v\n", fromGuild.Name, minuteAmount)
			}
		}

		// Add to destination guild's TributeIn
		if tribute.ToGuildName != "" {
			toGuild := GetGuildByNameUnsafe(tribute.ToGuildName)
			if toGuild != nil {
				toGuild.TributeIn.Emeralds += minuteAmount.Emeralds
				toGuild.TributeIn.Ores += minuteAmount.Ores
				toGuild.TributeIn.Wood += minuteAmount.Wood
				toGuild.TributeIn.Fish += minuteAmount.Fish
				toGuild.TributeIn.Crops += minuteAmount.Crops
				debugf("Added TributeIn to guild %s: %+v\n", toGuild.Name, minuteAmount)
			}
		}
	}

	debugf("Recalculated tribute totals for %d guilds\n", len(st.guilds))
}

// GetGuildByNameUnsafe finds a guild by name without acquiring locks
// Note: This function expects the mutex to already be held by the caller
func GetGuildByNameUnsafe(guildName string) *typedef.Guild {
	for _, guild := range st.guilds {
		if guild != nil && guild.Name == guildName {
			return guild
		}
	}
	return nil
}

// CreateTributeUnsafe creates a new tribute between guilds (unsafe version for locked contexts)
// This version should only be called when st.mu is already locked
func CreateTributeUnsafe(fromGuildName, toGuildName string, amountPerHour typedef.BasicResources, intervalMinutes uint32) (*typedef.ActiveTribute, error) {
	if intervalMinutes == 0 {
		return nil, fmt.Errorf("interval minutes must be greater than 0")
	}

	var fromGuild, toGuild *typedef.Guild

	// Find source guild if specified (using unsafe version)
	if fromGuildName != "" {
		fromGuild = GetGuildByNameUnsafe(fromGuildName)
		if fromGuild == nil {
			return nil, fmt.Errorf("source guild '%s' not found", fromGuildName)
		}
	}

	// Find destination guild if specified (using unsafe version)
	if toGuildName != "" {
		toGuild = GetGuildByNameUnsafe(toGuildName)
		if toGuild == nil {
			return nil, fmt.Errorf("destination guild '%s' not found", toGuildName)
		}
	}

	// Validate that at least one guild is specified
	if fromGuildName == "" && toGuildName == "" {
		return nil, fmt.Errorf("at least one guild must be specified")
	}

	// Convert hourly amounts to per-minute amounts for the transit system
	amountPerMinute := typedef.BasicResources{
		Emeralds: amountPerHour.Emeralds / 60.0,
		Ores:     amountPerHour.Ores / 60.0,
		Wood:     amountPerHour.Wood / 60.0,
		Fish:     amountPerHour.Fish / 60.0,
		Crops:    amountPerHour.Crops / 60.0,
	}

	tribute := &typedef.ActiveTribute{
		ID:              generateTributeID(),
		From:            fromGuild,
		FromGuildName:   fromGuildName,
		To:              toGuild,
		ToGuildName:     toGuildName,
		AmountPerHour:   amountPerHour,   // Store original hourly input
		AmountPerMinute: amountPerMinute, // Store calculated per-minute amount
		IntervalMinutes: intervalMinutes,
		LastTransfer:    0, // Never transferred yet
		IsActive:        true,
		CreatedAt:       st.tick,
	}

	debugf("CreateTributeUnsafe: Created tribute %s from %s (ptr=%v) to %s (ptr=%v)\n",
		tribute.ID, fromGuildName, tribute.From != nil, toGuildName, tribute.To != nil)

	return tribute, nil
}

// GetGuildTributeStats returns tribute statistics for a guild
func GetGuildTributeStats(guildName string) *typedef.Guild {
	st.mu.RLock()
	defer st.mu.RUnlock()

	for _, guild := range st.guilds {
		if guild != nil && guild.Name == guildName {
			return guild
		}
	}
	return nil
}

// ListAllActiveTributes returns information about all active tributes for debugging
func ListAllActiveTributes() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var result []string
	for _, tribute := range st.activeTributes {
		if tribute == nil {
			continue
		}

		status := "ACTIVE"
		if !tribute.IsActive {
			status = "INACTIVE"
		}

		fromName := "NIL"
		if tribute.From != nil {
			fromName = tribute.From.Name
		} else if tribute.FromGuildName != "" {
			fromName = tribute.FromGuildName + " (missing pointer)"
		}

		toName := "NIL"
		if tribute.To != nil {
			toName = tribute.To.Name
		} else if tribute.ToGuildName != "" {
			toName = tribute.ToGuildName + " (missing pointer)"
		}

		info := fmt.Sprintf("ID: %s, From: %s, To: %s, AmountPerHour: %+v, Interval: %d minutes, Status: %s, LastTransfer: %d",
			tribute.ID, fromName, toName, tribute.AmountPerHour, tribute.IntervalMinutes, status, tribute.LastTransfer)
		result = append(result, info)
	}

	return result
}

// DiagnoseTributeSystem performs comprehensive diagnostics of the tribute system
func DiagnoseTributeSystem() string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var result []string
	result = append(result, "=== TRIBUTE SYSTEM DIAGNOSTICS ===")
	result = append(result, fmt.Sprintf("Total active tributes: %d", len(st.activeTributes)))
	result = append(result, fmt.Sprintf("Current tick: %d", st.tick))
	result = append(result, "")

	if len(st.activeTributes) == 0 {
		result = append(result, "No active tributes found.")
		return strings.Join(result, "\n")
	}

	for i, tribute := range st.activeTributes {
		if tribute == nil {
			result = append(result, fmt.Sprintf("Tribute %d: NIL", i))
			continue
		}

		result = append(result, fmt.Sprintf("--- Tribute %d ---", i))
		result = append(result, fmt.Sprintf("ID: %s", tribute.ID))
		result = append(result, fmt.Sprintf("From Guild: %s", tribute.FromGuildName))
		result = append(result, fmt.Sprintf("To Guild: %s", tribute.ToGuildName))
		result = append(result, fmt.Sprintf("From Pointer: %v", tribute.From != nil))
		result = append(result, fmt.Sprintf("To Pointer: %v", tribute.To != nil))
		result = append(result, fmt.Sprintf("AmountPerHour: %+v", tribute.AmountPerHour))
		result = append(result, fmt.Sprintf("AmountPerMinute: %+v", tribute.AmountPerMinute))
		result = append(result, fmt.Sprintf("Interval Minutes: %d", tribute.IntervalMinutes))
		result = append(result, fmt.Sprintf("Last Transfer: %d", tribute.LastTransfer))
		result = append(result, fmt.Sprintf("Is Active: %v", tribute.IsActive))
		result = append(result, fmt.Sprintf("Created At: %d", tribute.CreatedAt))

		// Check guild pointers
		if tribute.From != nil {
			result = append(result, fmt.Sprintf("From Guild Details: %s [%s]", tribute.From.Name, tribute.From.Tag))

			// Check HQ
			fromHQ := st.hqMap[tribute.From.Tag]
			if fromHQ != nil {
				result = append(result, fmt.Sprintf("From HQ: %s", fromHQ.Name))
			} else {
				result = append(result, "From HQ: NOT FOUND")
			}
		} else {
			result = append(result, "From Guild: POINTER MISSING")
		}

		if tribute.To != nil {
			result = append(result, fmt.Sprintf("To Guild Details: %s [%s]", tribute.To.Name, tribute.To.Tag))

			// Check HQ
			toHQ := st.hqMap[tribute.To.Tag]
			if toHQ != nil {
				result = append(result, fmt.Sprintf("To HQ: %s", toHQ.Name))
			} else {
				result = append(result, "To HQ: NOT FOUND")
			}
		} else {
			result = append(result, "To Guild: POINTER MISSING")
		}

		// Check if ready to process
		currentMinute := st.tick / 60
		var shouldTransfer bool
		if tribute.LastTransfer == 0 {
			// First transfer - use creation time
			minutesSinceCreation := currentMinute - (tribute.CreatedAt / 60)
			shouldTransfer = minutesSinceCreation >= uint64(tribute.IntervalMinutes)
		} else {
			// Subsequent transfers - use last transfer time
			minutesSinceLastTransfer := currentMinute - (tribute.LastTransfer / 60)
			shouldTransfer = minutesSinceLastTransfer >= uint64(tribute.IntervalMinutes)
		}

		if tribute.IsActive && shouldTransfer && st.tick%60 == 0 {
			result = append(result, "STATUS: READY TO PROCESS")
		} else if !tribute.IsActive {
			result = append(result, "STATUS: INACTIVE")
		} else {
			var minutesUntilNext uint64
			if tribute.LastTransfer == 0 {
				minutesSinceCreation := currentMinute - (tribute.CreatedAt / 60)
				if uint64(tribute.IntervalMinutes) > minutesSinceCreation {
					minutesUntilNext = uint64(tribute.IntervalMinutes) - minutesSinceCreation
				}
			} else {
				minutesSinceLastTransfer := currentMinute - (tribute.LastTransfer / 60)
				if uint64(tribute.IntervalMinutes) > minutesSinceLastTransfer {
					minutesUntilNext = uint64(tribute.IntervalMinutes) - minutesSinceLastTransfer
				}
			}
			result = append(result, fmt.Sprintf("STATUS: WAITING (%d minutes)", minutesUntilNext))
		}

		result = append(result, "")
	}

	// Check guild tribute totals
	result = append(result, "=== GUILD TRIBUTE TOTALS ===")
	for _, guild := range st.guilds {
		if guild != nil && (guild.TributeIn.Emeralds > 0 || guild.TributeOut.Emeralds > 0 ||
			guild.TributeIn.Ores > 0 || guild.TributeOut.Ores > 0 ||
			guild.TributeIn.Wood > 0 || guild.TributeOut.Wood > 0 ||
			guild.TributeIn.Fish > 0 || guild.TributeOut.Fish > 0 ||
			guild.TributeIn.Crops > 0 || guild.TributeOut.Crops > 0) {
		}
	}

	return strings.Join(result, "\n")
}
