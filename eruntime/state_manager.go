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

	// Capture current state under read lock
	st.mu.RLock()

	// Create deep copies of territories to avoid holding the lock too long
	territoriesCopy := make([]*typedef.Territory, len(st.territories))
	for i, territory := range st.territories {
		if territory != nil {
			// Create a copy of the territory
			territory.Mu.RLock()
			territoryData := *territory
			territory.Mu.RUnlock()
			territoriesCopy[i] = &territoryData
		}
	}

	// Create deep copies of guilds
	guildsCopy := make([]*typedef.Guild, len(st.guilds))
	for i, guild := range st.guilds {
		if guild != nil {
			guildData := *guild
			guildsCopy[i] = &guildData
		}
	}

	// Copy other state data
	tickCopy := st.tick
	runtimeOptionsCopy := st.runtimeOptions
	costsCopy := st.costs

	st.mu.RUnlock()

	// Create state data structure
	stateData := StateData{
		Type:      "state_save",
		Version:   "1.0",
		Timestamp: time.Now(),

		Tick:           tickCopy,
		Territories:    territoriesCopy,
		Guilds:         guildsCopy,
		RuntimeOptions: runtimeOptionsCopy,
		Costs:          costsCopy,

		TotalTerritories: len(territoriesCopy),
		TotalGuilds:      len(guildsCopy),
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(stateData, "", "  ")
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
	// Read compressed file
	compressedData, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Decompress LZ4
	jsonData, err := decompressLZ4(compressedData)
	if err != nil {
		return fmt.Errorf("failed to decompress data: %v", err)
	}

	// Unmarshal JSON
	var stateData StateData
	err = json.Unmarshal(jsonData, &stateData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal state data: %v", err)
	}

	// Validate state data
	if stateData.Type != "state_save" {
		return fmt.Errorf("invalid file type: expected 'state_save', got '%s'", stateData.Type)
	}

	if stateData.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s", stateData.Version)
	}

	// Merge guilds from state file with existing guilds and update guilds.json
	err = mergeGuildsFromState(stateData.Guilds)
	if err != nil {
		return fmt.Errorf("failed to merge guilds from state: %v", err)
	}

	// Apply state under write lock
	st.mu.Lock()
	defer st.mu.Unlock()

	// Clear existing territories
	for _, territory := range st.territories {
		if territory != nil && territory.CloseCh != nil {
			territory.CloseCh()
		}
	}

	// Restore state (guilds are already merged above)
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

	// Rebuild guild relationships and update routes
	st.updateRoute()

	fmt.Printf("[STATE] Successfully loaded state from %s (Territories: %d, Guilds: %d, Tick: %d)\n",
		filepath, len(st.territories), len(st.guilds), st.tick)

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
	jsonData, err := json.MarshalIndent(rawGuilds, "", "  ")
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

		fmt.Printf("[STATE] Added %d new guilds from state file to guilds.json\n", len(newGuilds))
		for _, guild := range newGuilds {
			fmt.Printf("[STATE] - Added guild: %s [%s]\n", guild.Name, guild.Tag)
		}
	}

	// Save updated guilds list to guilds.json
	if len(newGuilds) > 0 || len(updatedGuilds) > 0 {
		err := saveGuildsToFile()
		if err != nil {
			return fmt.Errorf("failed to save updated guilds to guilds.json: %v", err)
		}
		fmt.Printf("[STATE] Successfully updated guilds.json with %d new and %d updated guilds\n",
			len(newGuilds), len(updatedGuilds))
	}

	return nil
}
