package eruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"RueaES/typedef"

	"github.com/gookit/goutil/arrutil"
	"github.com/pierrec/lz4"
)

var supportedStateVersion = []string{"1.0", "1.1", "1.2", "1.3", "1.4"}

// Callback functions for user data that persists through resets
var (
	getLoadoutsCallback      func() []typedef.Loadout
	setLoadoutsCallback      func([]typedef.Loadout)
	mergeLoadoutsCallback    func([]typedef.Loadout) // Merges loadouts, keeping existing ones with same names
	getGuildColorsCallback   func() map[string]map[string]string
	setGuildColorsCallback   func(map[string]map[string]string)
	mergeGuildColorsCallback func(map[string]map[string]string) // Merges guild colors
	getUndoHistoryCallback   func() ([]byte, error)             // Returns serialized undo tree
	setUndoHistoryCallback   func([]byte) error                 // Restores undo tree from serialized data
)

// SetLoadoutCallbacks sets the callback functions for loadout persistence
func SetLoadoutCallbacks(getFunc func() []typedef.Loadout, setFunc func([]typedef.Loadout), mergeFunc func([]typedef.Loadout)) {
	getLoadoutsCallback = getFunc
	setLoadoutsCallback = setFunc
	mergeLoadoutsCallback = mergeFunc
}

// SetGuildColorCallbacks sets the callback functions for guild color persistence
func SetGuildColorCallbacks(getFunc func() map[string]map[string]string, setFunc func(map[string]map[string]string), mergeFunc func(map[string]map[string]string)) {
	getGuildColorsCallback = getFunc
	setGuildColorsCallback = setFunc
	mergeGuildColorsCallback = mergeFunc
}

// SetUndoHistoryCallbacks sets the callback functions for undo history persistence
func SetUndoHistoryCallbacks(getFunc func() ([]byte, error), setFunc func([]byte) error) {
	getUndoHistoryCallback = getFunc
	setUndoHistoryCallback = setFunc
}

// StateData represents the complete state that can be saved/loaded
type StateData struct {
	Type      string    `json:"type"` // "state_save"
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"` // When the state was saved

	// Core state data
	Tick           uint64                   `json:"tick"`
	Territories    []*typedef.Territory     `json:"territories"`
	Guilds         []*typedef.Guild         `json:"guilds"`
	ActiveTributes []*typedef.ActiveTribute `json:"activeTributes"`
	RuntimeOptions typedef.RuntimeOptions   `json:"runtimeOptions"`
	Costs          typedef.Costs            `json:"costs"`

	// Additional metadata
	TotalTerritories int `json:"totalTerritories"`
	TotalGuilds      int `json:"totalGuilds"`

	// Persistent user data (version 1.3+)
	Loadouts    []typedef.Loadout            `json:"loadouts,omitempty"`    // Loadouts persist through resets
	GuildColors map[string]map[string]string `json:"guildColors,omitempty"` // Guild colors: guildName -> {tag, color}

	// Undo/Redo history (version 1.4+)
	UndoHistory json.RawMessage `json:"undoHistory,omitempty"` // Serialized undo tree for debugging and future features
}

// SaveStateToFile saves the current state to a file with LZ4 compression
func SaveStateToFile(filepath string) error {
	fmt.Printf("[STATE] SaveStateToFile called with filepath: %s\n", filepath)

	// Capture current state under read lock - minimize lock time for better performance
	var stateData StateData

	st.mu.RLock()

	// Quick copy of basic state data (no deep copying yet)
	stateData.Type = "state_save"
	stateData.Version = "1.4"
	stateData.Timestamp = time.Now()
	stateData.Tick = st.tick
	stateData.RuntimeOptions = st.runtimeOptions
	stateData.Costs = st.costs
	stateData.TotalTerritories = len(st.territories)
	stateData.TotalGuilds = len(st.guilds)

	// Create slices with the right capacity but don't copy data yet
	territoryRefs := make([]*typedef.Territory, len(st.territories))
	copy(territoryRefs, st.territories)

	guildRefs := make([]*typedef.Guild, len(st.guilds))
	copy(guildRefs, st.guilds)

	tributeRefs := make([]*typedef.ActiveTribute, len(st.activeTributes))
	copy(tributeRefs, st.activeTributes)

	st.mu.RUnlock()

	// Now do the expensive deep copying WITHOUT holding any locks
	// This allows users to continue using the application while we save

	// Deep copy territories
	stateData.Territories = make([]*typedef.Territory, len(territoryRefs))
	encodeTransit := st.runtimeOptions.EncodeInTransitResources
	for i, territory := range territoryRefs {
		if territory != nil {
			// Lock individual territory briefly to copy its data
			territory.Mu.RLock()
			// Create a copy without the mutex
			territoryData := typedef.Territory{
				ID:                   territory.ID,
				Name:                 territory.Name,
				Guild:                territory.Guild,
				Location:             territory.Location,
				Options:              territory.Options,
				Costs:                territory.Costs,
				Net:                  territory.Net,
				TowerStats:           territory.TowerStats,
				Level:                territory.Level,
				LevelInt:             territory.LevelInt,
				SetLevelInt:          territory.SetLevelInt,
				SetLevel:             territory.SetLevel,
				Links:                territory.Links,
				ResourceGeneration:   territory.ResourceGeneration,
				Treasury:             territory.Treasury,
				TreasuryOverride:     territory.TreasuryOverride,
				GenerationBonus:      territory.GenerationBonus,
				CapturedAt:           territory.CapturedAt,
				ConnectedTerritories: territory.ConnectedTerritories,
				TradingRoutes:        territory.TradingRoutes,
				TradingRoutesJSON:    territory.TradingRoutesJSON,
				RouteTax:             territory.RouteTax,
				RoutingMode:          territory.RoutingMode,
				Border:               territory.Border,
				Tax:                  territory.Tax,
				HQ:                   territory.HQ,
				NextTerritory:        territory.NextTerritory,
				Destination:          territory.Destination,
				Storage:              territory.Storage,
				TransitResource:      territory.TransitResource,
				Warning:              territory.Warning,
			}
			// If encoding in-transit resources is enabled, fill JSON-safe fields
			if encodeTransit {
				for j := range territoryData.TransitResource {
					tr := &territoryData.TransitResource[j]
					if tr.Origin != nil {
						tr.OriginID = tr.Origin.ID
					} else {
						tr.OriginID = ""
					}
					if tr.Destination != nil {
						tr.DestinationID = tr.Destination.ID
					} else {
						tr.DestinationID = ""
					}
					if tr.Next != nil {
						tr.NextID = tr.Next.ID
					} else {
						tr.NextID = ""
					}
					tr.Route2 = make([]string, 0, len(tr.Route))
					for _, t := range tr.Route {
						if t != nil {
							tr.Route2 = append(tr.Route2, t.ID)
						}
					}
				}
				// Also fill TradingRoutesJSON for the territory
				territoryData.TradingRoutesJSON = make([][]string, len(territoryData.TradingRoutes))
				for k, route := range territoryData.TradingRoutes {
					ids := make([]string, 0, len(route))
					for _, t := range route {
						if t != nil {
							ids = append(ids, t.ID)
						}
					}
					territoryData.TradingRoutesJSON[k] = ids
				}
			} else {
				// If not encoding, clear JSON-safe fields
				for j := range territoryData.TransitResource {
					tr := &territoryData.TransitResource[j]
					tr.OriginID = ""
					tr.DestinationID = ""
					tr.NextID = ""
					tr.Route2 = nil
				}
				territoryData.TradingRoutesJSON = nil
			}
			territory.Mu.RUnlock()
			stateData.Territories[i] = &territoryData
		}
	}

	// Deep copy guilds (these are small, so copying is fast)
	stateData.Guilds = make([]*typedef.Guild, len(guildRefs))
	for i, guild := range guildRefs {
		if guild != nil {
			guildData := *guild
			stateData.Guilds[i] = &guildData
		}
	}

	// Deep copy active tributes
	stateData.ActiveTributes = make([]*typedef.ActiveTribute, len(tributeRefs))
	for i, tribute := range tributeRefs {
		if tribute != nil {
			tributeData := *tribute
			stateData.ActiveTributes[i] = &tributeData
		}
	}

	// Copy user loadouts (version 1.3+) - these persist through resets
	if getLoadoutsCallback != nil {
		stateData.Loadouts = getLoadoutsCallback()
		fmt.Printf("[STATE] Saved %d loadouts to state\n", len(stateData.Loadouts))
	}

	// Copy guild colors (version 1.3+) - these persist through resets
	if getGuildColorsCallback != nil {
		stateData.GuildColors = getGuildColorsCallback()
		fmt.Printf("[STATE] Saved %d guild colors to state\n", len(stateData.GuildColors))
	}

	// Copy undo history (version 1.4+) - for debugging and future features
	if getUndoHistoryCallback != nil {
		undoData, err := getUndoHistoryCallback()
		if err == nil && len(undoData) > 0 {
			stateData.UndoHistory = json.RawMessage(undoData)
			fmt.Printf("[STATE] Saved undo history to state (%d bytes)\n", len(undoData))
		}
	}

	// All the expensive operations (JSON marshal, compression, file write) happen
	// without holding any locks, so users can continue working

	// Marshal to JSON
	jsonData, err := json.Marshal(stateData)
	if err != nil {
		return fmt.Errorf("failed to marshal state data: %v", err)
	}

	// Compress with LZ4
	compressedData, err := compressLZ4(jsonData)
	if err != nil {
		return fmt.Errorf("failed to compress data: %v", err)
	}

	// Write to file
	err = os.WriteFile(filepath, compressedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("[STATE] Successfully saved state to %s (JSON: %d bytes, Compressed: %d bytes, Ratio: %.1f%%)\n",
		filepath, len(jsonData), len(compressedData), float64(len(compressedData))/float64(len(jsonData))*100)

	return nil
}

// ValidateStateFile checks if a state file is valid without actually loading it
func ValidateStateFile(filepath string) error {
	// Read compressed file
	compressedData, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Check if file is empty
	if len(compressedData) == 0 {
		return fmt.Errorf("file is empty")
	}

	// Decompress LZ4
	jsonData, err := decompressLZ4(compressedData)
	if err != nil {
		return fmt.Errorf("failed to decompress data (corrupted or not a valid LZ4 file): %v", err)
	}

	// Check if decompressed data is empty
	if len(jsonData) == 0 {
		return fmt.Errorf("decompressed data is empty")
	}

	// Try to unmarshal JSON
	var stateData StateData
	err = json.Unmarshal(jsonData, &stateData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON data (corrupted state file): %v", err)
	}

	// Validate state data
	if stateData.Type != "state_save" {
		return fmt.Errorf("invalid file type: expected 'state_save', got '%s'", stateData.Type)
	}

	if !arrutil.Contains(supportedStateVersion, stateData.Version) {
		return fmt.Errorf("unsupported state version '%s'. Supported versions: %v", stateData.Version, supportedStateVersion)
	}

	// File is valid
	return nil
}

// LoadStateFromFileSelective loads state from a file with selective import options
func LoadStateFromFileSelective(filepath string, importOptions map[string]bool) error {
	return loadStateFromFileInternal(filepath, importOptions)
}

// LoadStateFromFile loads state from a file with LZ4 decompression (imports everything)
func LoadStateFromFile(filepath string) error {
	// Default: import everything
	importOptions := map[string]bool{
		"core":             true,
		"guilds":           true,
		"territories":      true,
		"territory_config": true,
		"territory_data":   true,
		"in_transit":       true,
		"tributes":         true,
		"loadouts":         true,
	}
	return loadStateFromFileInternal(filepath, importOptions)
}

// loadStateFromFileInternal is the internal implementation that handles selective loading
func loadStateFromFileInternal(filepath string, importOptions map[string]bool) error {
	// Halt the runtime during state loading to prevent flickering
	wasHalted := st.halted
	if !wasHalted {
		st.halt()
	}

	// Read compressed file
	compressedData, err := os.ReadFile(filepath)
	if err != nil {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Decompress LZ4
	jsonData, err := decompressLZ4(compressedData)
	if err != nil {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("failed to decompress data: %v", err)
	}

	// Unmarshal JSON
	var stateData StateData
	err = json.Unmarshal(jsonData, &stateData)
	if err != nil {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("failed to unmarshal state data: %v", err)
	}

	// Validate state data
	if stateData.Type != "state_save" {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("invalid file type: expected 'state_save', got '%s'", stateData.Type)
	}

	if !arrutil.Contains(supportedStateVersion, stateData.Version) {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("unsupported version: %s", stateData.Version)
	}

	// Import core runtime data (if requested)
	if importOptions["core"] {
		st.mu.Lock()
		st.tick = stateData.Tick
		st.runtimeOptions = stateData.RuntimeOptions
		st.mu.Unlock()
		fmt.Printf("[STATE] Imported core data (tick: %d)\n", stateData.Tick)
	} else {
		fmt.Printf("[STATE] Skipped importing core data\n")
	}

	// Merge guilds from state file with existing guilds and update guilds.json (if requested)
	if importOptions["guilds"] {
		err = mergeGuildsFromState(stateData.Guilds)
		if err != nil {
			// Restore runtime state if we halted it
			if !wasHalted {
				st.start()
			}
			return fmt.Errorf("failed to merge guilds from state: %v", err)
		}
		fmt.Printf("[STATE] Imported %d guilds from state\n", len(stateData.Guilds))
	} else {
		fmt.Printf("[STATE] Skipped importing guilds\n")
	}

	hasTransitFields := false
	for _, t := range stateData.Territories {
		if t == nil {
			continue
		}
		if len(t.TransitResource) > 0 {
			for _, tr := range t.TransitResource {
				if tr.OriginID != "" || len(tr.Route2) > 0 || tr.DestinationID != "" || tr.NextID != "" {
					hasTransitFields = true
					break
				}
			}
		}
		if len(t.TradingRoutesJSON) > 0 {
			hasTransitFields = true
		}
		if hasTransitFields {
			break
		}
	}

	// Apply state under write lock
	st.mu.Lock()
	st.stateLoading = true

	// Import territories and related data based on options
	if importOptions["territories"] || importOptions["territory_config"] || importOptions["territory_data"] || importOptions["in_transit"] {
		// Clear existing territories
		for _, territory := range st.territories {
			if territory != nil && territory.CloseCh != nil {
				territory.CloseCh()
			}
		}

		st.tick = stateData.Tick
		st.territories = stateData.Territories

		// Only import territory configurations if requested
		if !importOptions["territory_config"] {
			// Reset territory configurations to defaults but keep ownership
			for _, territory := range st.territories {
				if territory != nil {
					// Keep guild ownership but reset configs
					guild := territory.Guild
					*territory = typedef.Territory{
						ID:    territory.ID,
						Name:  territory.Name,
						Guild: guild,
					}
					// Apply default values here if needed
				}
			}
			fmt.Printf("[STATE] Imported territories but skipped configurations\n")
		} else {
			fmt.Printf("[STATE] Imported territories with configurations\n")
		}

		// Only import territory data if requested
		if !importOptions["territory_data"] {
			// Clear resource data
			for _, territory := range st.territories {
				if territory != nil {
					territory.Storage = typedef.TerritoryStorage{}
					territory.ResourceGeneration = typedef.ResourceGeneration{}
				}
			}
			fmt.Printf("[STATE] Cleared territory resource data\n")
		} else {
			fmt.Printf("[STATE] Imported territory resource data\n")
		}

		// Only import in-transit resources if requested
		if !importOptions["in_transit"] {
			// Clear transit resources
			for _, territory := range st.territories {
				if territory != nil {
					territory.TransitResource = []typedef.InTransitResources{}
					territory.TradingRoutesJSON = [][]string{}
				}
			}
			fmt.Printf("[STATE] Cleared in-transit resources\n")
		} else {
			fmt.Printf("[STATE] Imported in-transit resources\n")
		}

		// Import runtime options only if core is not being imported separately
		if !importOptions["core"] {
			st.runtimeOptions = stateData.RuntimeOptions
		}

		// Rebuild territory map for fast lookups
		TerritoryMap = make(map[string]*typedef.Territory)
		st.territoryMap = make(map[string]*typedef.Territory)
		for _, territory := range st.territories {
			if territory != nil {
				TerritoryMap[territory.Name] = territory
				st.territoryMap[territory.ID] = territory
				setCh := make(chan typedef.TerritoryOptions, 1)
				territory.SetCh = setCh
				territory.CloseCh = func() { close(setCh) }
			}
		}
	} else {
		fmt.Printf("[STATE] Skipped importing territories and related data\n")
	}

	rebuildHQMap()

	// Import tributes (if requested)
	if importOptions["tributes"] {
		st.activeTributes = stateData.ActiveTributes
		// Rebuild guild pointers in tributes
		rebuildTributeGuildPointers()
		fmt.Printf("[STATE] Imported tributes data\n")
	} else {
		fmt.Printf("[STATE] Skipped importing tributes\n")
		// Still need to rebuild pointers for existing tributes
		rebuildTributeGuildPointers()
	}

	// Only recalculate routes if new fields are missing
	if !hasTransitFields {
		st.updateRoute()
	} else {
		// Restore in-memory pointers from JSON-safe IDs
		RestorePointersFromIDs(st.territories)
	}

	loadCosts(&st)

	fmt.Printf("[STATE] Successfully loaded state from %s (Territories: %d, Guilds: %d, Tick: %d)\n",
		filepath, len(st.territories), len(st.guilds), st.tick)

	st.mu.Unlock()

	st.mu.Lock()
	st.stateLoading = false
	st.mu.Unlock()

	if !wasHalted {
		st.start()
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		NotifyTerritoryColorsUpdate()
		fmt.Printf("[STATE] State loading completed, notifications sent\n")
	}()

	// Load persistent user data (loadouts) - version 1.3+ (if requested)
	if importOptions["loadouts"] && mergeLoadoutsCallback != nil && stateData.Loadouts != nil {
		mergeLoadoutsCallback(stateData.Loadouts)
		fmt.Printf("[STATE] Merged %d loadouts from state\n", len(stateData.Loadouts))
	} else if !importOptions["loadouts"] {
		fmt.Printf("[STATE] Skipped importing loadouts\n")
	}

	// Load guild colors - version 1.3+ (if guilds are imported)
	if importOptions["guilds"] && mergeGuildColorsCallback != nil && stateData.GuildColors != nil {
		mergeGuildColorsCallback(stateData.GuildColors)
		fmt.Printf("[STATE] Merged %d guild colors from state\n", len(stateData.GuildColors))
	} else if !importOptions["guilds"] {
		fmt.Printf("[STATE] Skipped importing guild colors\n")
	}

	// Load undo history - version 1.4+ (if undo_history option is selected)
	if importOptions["undo_history"] && setUndoHistoryCallback != nil && len(stateData.UndoHistory) > 0 {
		if err := setUndoHistoryCallback([]byte(stateData.UndoHistory)); err != nil {
			fmt.Printf("[STATE] Warning: Failed to restore undo history: %v\n", err)
		} else {
			fmt.Printf("[STATE] Restored undo history (%d bytes)\n", len(stateData.UndoHistory))
		}
	} else if !importOptions["undo_history"] {
		fmt.Printf("[STATE] Skipped importing undo history\n")
	}

	fmt.Printf("[STATE] Monitoring HQ status for the next few seconds after state load...\n")
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Second)
			st.mu.RLock()
			hqCount := 0
			for _, territory := range st.territories {
				if territory != nil && territory.HQ {
					hqCount++
					fmt.Printf("[HQ_MONITOR] Tick %d: Territory %s (Guild: %s) is still HQ\n",
						i+1, territory.Name, territory.Guild.Name)
				}
			}
			if hqCount == 0 {
				fmt.Printf("[HQ_MONITOR] Tick %d: No HQ territories found!\n", i+1)
			}
			st.mu.RUnlock()
		}
		fmt.Printf("[HQ_MONITOR] Monitoring complete\n")
	}()

	return nil
}

// compressLZ4 compresses data using LZ4
func compressLZ4(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	writer.CompressionLevel = 4
	writer.WithConcurrency(-1)

	_, err := writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompressLZ4 decompresses LZ4 data
func decompressLZ4(data []byte) ([]byte, error) {
	reader := lz4.NewReader(bytes.NewReader(data))

	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// saveGuildsToFile saves the current guild list to guilds.json
func saveGuildsToFile() error {
	// Convert guilds to the format expected by guilds.json
	var rawGuilds typedef.GuildsFileJSON

	st.mu.RLock()
	for _, guild := range st.guilds {
		if guild != nil {
			rawGuilds = append(rawGuilds, typedef.GuildJSON{
				Name: guild.Name,
				Tag:  guild.Tag,
			})
		}
	}
	st.mu.RUnlock()

	// Marshal to JSON
	jsonData, err := json.Marshal(rawGuilds)
	if err != nil {
		return fmt.Errorf("failed to marshal guilds data: %v", err)
	}

	// Write to file
	err = os.WriteFile("guilds.json", jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write guilds.json: %v", err)
	}

	return nil
}

// mergeGuildsFromState merges guilds from loaded state with existing guilds and saves to guilds.json
func mergeGuildsFromState(loadedGuilds []*typedef.Guild) error {
	if len(loadedGuilds) == 0 {
		return nil
	}

	// Create a map of existing guilds for fast lookup
	existingGuilds := make(map[string]*typedef.Guild)

	st.mu.RLock()
	for _, guild := range st.guilds {
		if guild != nil {
			existingGuilds[guild.Tag] = guild
		}
	}
	st.mu.RUnlock()

	// Track new guilds that need to be added
	var newGuilds []*typedef.Guild
	var updatedGuilds []*typedef.Guild

	// Check each loaded guild
	for _, loadedGuild := range loadedGuilds {
		if loadedGuild == nil {
			continue
		}

		if existingGuild, exists := existingGuilds[loadedGuild.Tag]; exists {
			// Guild exists, but check if name needs updating
			if existingGuild.Name != loadedGuild.Name {
				existingGuild.Name = loadedGuild.Name
				updatedGuilds = append(updatedGuilds, existingGuild)
			}
		} else {
			// New guild from state file
			newGuildCopy := &typedef.Guild{
				Name:   loadedGuild.Name,
				Tag:    loadedGuild.Tag,
				Allies: []*typedef.Guild{}, // Initialize empty ally list
			}
			newGuilds = append(newGuilds, newGuildCopy)
		}
	}

	// Add new guilds to the state
	if len(newGuilds) > 0 {
		st.mu.Lock()
		st.guilds = append(st.guilds, newGuilds...)
		st.mu.Unlock()

		fmt.Printf("[STATE] Added %d new guilds from state file\n", len(newGuilds))
		for _, guild := range newGuilds {
			fmt.Printf("[STATE] - Added guild: %s [%s]\n", guild.Name, guild.Tag)
		}
	}

	// Only notify guild managers to handle the merge themselves, don't overwrite their files
	if len(newGuilds) > 0 || len(updatedGuilds) > 0 {
		fmt.Printf("[STATE] Found %d new and %d updated guilds from state - notifying UI to merge\n",
			len(newGuilds), len(updatedGuilds))

		// Notify guild managers to merge new guilds while preserving local data like colors
		// This is now safe because we have state loading protection
		go NotifyGuildManagerUpdate()
	}

	return nil
}

// validateAndFixHQConflicts ensures that each guild has at most one HQ
// When loading from state file, the state file's HQ settings take precedence
func validateAndFixHQConflicts() {
	// Track HQ territories per guild
	guildHQs := make(map[string][]*typedef.Territory)

	// Find all HQ territories for each guild
	for _, territory := range st.territories {
		if territory != nil && territory.HQ && territory.Guild.Tag != "" && territory.Guild.Tag != "NONE" {
			guildHQs[territory.Guild.Tag] = append(guildHQs[territory.Guild.Tag], territory)
		}
	}

	// Fix conflicts: each guild should have at most one HQ
	for guildTag, hqTerritories := range guildHQs {
		if len(hqTerritories) > 1 {
			fmt.Printf("[STATE] HQ conflict detected for guild %s: multiple HQs found\n", guildTag)

			// Keep the first HQ found (from state file) and clear others
			// This ensures state file takes precedence
			for i, territory := range hqTerritories {
				if i == 0 {
					fmt.Printf("[STATE] Keeping HQ at territory: %s\n", territory.Name)
				} else {
					territory.Mu.Lock()
					territory.HQ = false
					territory.Mu.Unlock()
					fmt.Printf("[STATE] Cleared HQ status from territory: %s\n", territory.Name)
				}
			}
		}
	}
}

// RestorePointersFromIDs restores in-memory pointers from JSON-safe string IDs after loading state.
func RestorePointersFromIDs(territories []*typedef.Territory) {
	// Build a map from ID to *Territory
	idMap := make(map[string]*typedef.Territory)
	for _, t := range territories {
		if t != nil {
			idMap[t.ID] = t
		}
	}

	for _, t := range territories {
		if t == nil {
			continue
		}
		// Restore TradingRoutes
		if len(t.TradingRoutesJSON) > 0 {
			t.TradingRoutes = make([][]*typedef.Territory, len(t.TradingRoutesJSON))
			for i, routeIDs := range t.TradingRoutesJSON {
				route := make([]*typedef.Territory, 0, len(routeIDs))
				for _, id := range routeIDs {
					if terr, ok := idMap[id]; ok {
						route = append(route, terr)
					}
				}
				t.TradingRoutes[i] = route
			}
		}
		// Restore InTransitResources pointers
		for i := range t.TransitResource {
			tr := &t.TransitResource[i]
			tr.Origin = idMap[tr.OriginID]
			tr.Destination = idMap[tr.DestinationID]
			tr.Next = idMap[tr.NextID]
			if len(tr.Route2) > 0 {
				tr.Route = make([]*typedef.Territory, 0, len(tr.Route2))
				for _, id := range tr.Route2 {
					if terr, ok := idMap[id]; ok {
						tr.Route = append(tr.Route, terr)
					}
				}
			}
		}
	}
}
