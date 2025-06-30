package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GuildListResponse represents the response from the Athena API
type GuildListResponse []GuildInfo

// GuildInfo represents a guild from the Athena API
type GuildInfo struct {
	Name  string `json:"_id"` // Try both name and _id fields
	Tag   string `json:"prefix,Prefix"`
	Color string `json:"color,Color"`
}

// TerritoryListResponse represents the response from the Wynncraft API
type TerritoryListResponse map[string]TerritoryInfo

// TerritoryInfo represents a territory from the Wynncraft API
type TerritoryInfo struct {
	Guild GuildReference `json:"guild"`
}

// GuildReference represents a guild reference in the territory API
type GuildReference struct {
	Name string `json:"name"`
}

// ImportGuildsFromAPI imports guilds from the Athena API
func (gm *EnhancedGuildManager) ImportGuildsFromAPI() (int, int, error) {
	// Fetch guild list from Athena API
	resp, err := http.Get("https://athena.wynntils.com/cache/get/guildList")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch guild list: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var guildListResp GuildListResponse
	err = json.NewDecoder(resp.Body).Decode(&guildListResp)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse guild list: %v", err)
	}

	// Import guilds
	importedCount := 0
	skippedCount := 0

	fmt.Printf("[API_IMPORT] Got %d guilds from API\n", len(guildListResp))

	for _, guildInfo := range guildListResp {
		// Skip if name or tag is empty
		if guildInfo.Name == "" || guildInfo.Tag == "" {
			fmt.Printf("[API_IMPORT] Skipping guild with empty name or tag: %+v\n", guildInfo)
			continue
		}

		// Format tag (removing brackets if present)
		tag := strings.Trim(guildInfo.Tag, "[]")

		// Check if guild already exists
		exists := false
		for _, existingGuild := range gm.guilds {
			if strings.EqualFold(existingGuild.Name, guildInfo.Name) || strings.EqualFold(existingGuild.Tag, tag) {
				exists = true
				skippedCount++
				break
			}
		}

		if !exists {
			// Format color (making sure it starts with #)
			color := guildInfo.Color
			if !strings.HasPrefix(color, "#") {
				color = "#" + color
			}

			// Add the guild
			newGuild := EnhancedGuildData{
				Name:  guildInfo.Name,
				Tag:   tag,
				Color: color,
			}
			gm.guilds = append(gm.guilds, newGuild)
			gm.cachesDirty = true // Invalidate caches when adding from API
			importedCount++
		}
	}

	// Save to file and update filtered guilds
	if importedCount > 0 {
		gm.saveGuildsToFile()
		gm.filterGuilds()
	}

	return importedCount, skippedCount, nil
}

// ImportTerritoriesFromAPI imports territory claims from the Wynncraft API and assigns them to guilds
func (gm *EnhancedGuildManager) ImportTerritoriesFromAPI() (int, int, error) {
	// Get the guild claim manager
	claimManager := GetGuildClaimManager()
	if claimManager == nil {
		return 0, 0, fmt.Errorf("guild claim manager is not initialized")
	}

	// Fetch territory list from Wynncraft API
	resp, err := http.Get("https://api.wynncraft.com/v3/guild/list/territory")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch territory list: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var territoryListResp TerritoryListResponse
	err = json.NewDecoder(resp.Body).Decode(&territoryListResp)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse territory list: %v", err)
	}

	// IMPORTANT: Suspend automatic redraws during bulk import to avoid hundreds of redraw calls
	claimManager.suspendRedraws = true
	defer func() {
		// Re-enable redraws and trigger one comprehensive redraw at the end
		claimManager.suspendRedraws = false
		fmt.Printf("[API_IMPORT] Triggering final redraw after bulk territory import\n")
		claimManager.TriggerRedraw()
	}()

	// Import territories
	importedCount := 0
	skippedCount := 0

	for territoryName, territoryInfo := range territoryListResp {
		// Skip if the guild name is empty
		if territoryInfo.Guild.Name == "" {
			skippedCount++
			continue
		}

		// Find the guild in our list
		var guildName, guildTag string
		found := false

		for _, guild := range gm.guilds {
			// Try to match by name (which is how the Wynncraft API references guilds)
			if strings.EqualFold(guild.Name, territoryInfo.Guild.Name) {
				guildName = guild.Name
				guildTag = guild.Tag
				found = true
				break
			}
		}

		if found {
			// Add the claim
			claimManager.AddClaim(territoryName, guildName, guildTag)
			importedCount++
		} else {
			skippedCount++
		}
	}

	return importedCount, skippedCount, nil
}
