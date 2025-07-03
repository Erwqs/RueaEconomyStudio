package eruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"etools/typedef"

	"github.com/pierrec/lz4"
)

// StateData represents the complete state that can be saved/loaded
type StateData struct {
	Type      string    `json:"type"`      // "state_save"
	Version   string    `json:"version"`   // "1.0"
	Timestamp time.Time `json:"timestamp"` // When the state was saved

	// Core state data
	Tick           uint64                 `json:"tick"`
	Territories    []*typedef.Territory   `json:"territories"`
	Guilds         []*typedef.Guild       `json:"guilds"`
	RuntimeOptions typedef.RuntimeOptions `json:"runtimeOptions"`
	Costs          typedef.Costs          `json:"costs"`

	// Additional metadata
	TotalTerritories int `json:"totalTerritories"`
	TotalGuilds      int `json:"totalGuilds"`
}

// SaveStateToFile saves the current state to a file with LZ4 compression
func SaveStateToFile(filepath string) error {
	fmt.Printf("[STATE] SaveStateToFile called with filepath: %s\n", filepath)

	// Capture current state under read lock - minimize lock time for better performance
	var stateData StateData

	st.mu.RLock()

	// Quick copy of basic state data (no deep copying yet)
	stateData.Type = "state_save"
	stateData.Version = "1.0"
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

	st.mu.RUnlock()

	// Now do the expensive deep copying WITHOUT holding any locks
	// This allows users to continue using the application while we save

	// Deep copy territories
	stateData.Territories = make([]*typedef.Territory, len(territoryRefs))
	for i, territory := range territoryRefs {
		if territory != nil {
			// Lock individual territory briefly to copy its data
			territory.Mu.RLock()
			territoryData := *territory
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

// LoadStateFromFile loads state from a file with LZ4 decompression
func LoadStateFromFile(filepath string) error {
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

	if stateData.Version != "1.0" {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("unsupported version: %s", stateData.Version)
	}

	// Merge guilds from state file with existing guilds and update guilds.json
	err = mergeGuildsFromState(stateData.Guilds)
	if err != nil {
		// Restore runtime state if we halted it
		if !wasHalted {
			st.start()
		}
		return fmt.Errorf("failed to merge guilds from state: %v", err)
	}

	// Apply state under write lock
	st.mu.Lock()

	// Set loading flag to prevent any other operations from modifying territories
	st.stateLoading = true

	// Clear existing territories
	for _, territory := range st.territories {
		if territory != nil && territory.CloseCh != nil {
			territory.CloseCh()
		}
	}

	// Directly restore state from file - trust the state file completely
	// The state file contains the exact state that was saved, including HQ status
	st.tick = stateData.Tick
	st.territories = stateData.Territories
	// Note: st.guilds is already updated by mergeGuildsFromState
	st.runtimeOptions = stateData.RuntimeOptions
	st.costs = stateData.Costs

	// Rebuild territory map for fast lookups
	TerritoryMap = make(map[string]*typedef.Territory)
	for _, territory := range st.territories {
		if territory != nil {
			TerritoryMap[territory.Name] = territory

			// Recreate channels for territory options
			setCh := make(chan typedef.TerritoryOptions, 1)
			territory.SetCh = setCh
			territory.CloseCh = func() {
				close(setCh)
			}
		}
	}

	// Rebuild HQ map for fast HQ lookups after state loading
	rebuildHQMap()

	// Rebuild guild relationships and update routes
	st.updateRoute()

	fmt.Printf("[STATE] Successfully loaded state from %s (Territories: %d, Guilds: %d, Tick: %d)\n",
		filepath, len(st.territories), len(st.guilds), st.tick)

	st.mu.Unlock()

	// Important: Clear loading flag FIRST, then restart timer
	// This ensures no ticks can run while stateLoading is true
	st.mu.Lock()
	st.stateLoading = false
	st.mu.Unlock()

	// Restart runtime if we halted it - do this AFTER clearing stateLoading flag
	if !wasHalted {
		st.start()
	}

	// Add a small delay to ensure HQ state is fully settled before any ticks process
	// This prevents race conditions where ticks might run immediately after timer start
	go func() {
		// Wait a brief moment for system to stabilize
		time.Sleep(100 * time.Millisecond)

		// Notify that state has changed and territory colors need updating
		NotifyTerritoryColorsUpdate()

		fmt.Printf("[STATE] State loading completed, notifications sent\n")
	}()

	// Add debug tracking to monitor HQ changes after state loading
	fmt.Printf("[STATE] Monitoring HQ status for the next few seconds after state load...\n")
	go func() {
		for i := 0; i < 3; i++ { // Monitor for 3 seconds
			time.Sleep(1 * time.Second)

			// Check all territories for HQ status
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
