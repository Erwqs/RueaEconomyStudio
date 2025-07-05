package eruntime

import (
	"encoding/json"
	"etools/assets"
	"etools/typedef"
	"os"
	"runtime"
)

// TradingRoutesMap holds the connections between territories
var TradingRoutesMap map[string][]string
var TerritoryMap map[string]*typedef.Territory
var TerritoryClaimsMap map[string]*typedef.Guild // territory name -> guild

func loadTerritories() {
	var rawTerritories typedef.TerritoriesFileJSON
	f, err := assets.AssetFiles.ReadFile("territories.json")
	if err != nil {
		panic("failed to read territories.json: " + err.Error())
	}

	err = json.Unmarshal(f, &rawTerritories)
	if err != nil {
		panic("failed to unmarshal territories.json: " + err.Error())
	}

	// Initialize maps
	TradingRoutesMap = make(map[string][]string)
	TerritoryMap = make(map[string]*typedef.Territory)
	TerritoryClaimsMap = make(map[string]*typedef.Guild)

	// Load territory claims
	loadTerritoryClaims()

	// Now initialize the territories
	for name, t := range rawTerritories {
		territory, err := initializeTerritory(name, t)
		if err != nil {
			panic("failed to initialize territory " + name + ": " + err.Error())
		}

		generations := t.Resources.Cast()
		territory.ResourceGeneration.Base = generations

		// Set guild information from claims
		if guild, exists := TerritoryClaimsMap[name]; exists {
			territory.Guild = *guild
			// Find guild name from guild list
			for _, g := range st.guilds {
				if g != nil && g.Tag == territory.Guild.Tag {
					territory.Guild.Name = g.Name
					break
				}
			}
		}

		// Store trading routes connections
		TradingRoutesMap[name] = t.TradingRoutes
		TerritoryMap[name] = territory

		// Also populate the state's territory map for fast lookups by ID
		st.territoryMap[territory.ID] = territory

		st.territories = append(st.territories, territory)
	}

	// Now load guilds from guilds.json, skip if running in WASM
	if runtime.GOARCH != "wasm" {
		f, err = os.ReadFile("guilds.json")
		if err != nil {
			// Create an empty guild list if file doesn't exist
			st.guilds = []*typedef.Guild{}
			f, err := os.Create("guilds.json") // Create empty file if it doesn't exist
			if err != nil {
				panic("failed to create guilds.json: " + err.Error())
			}

			defer f.Close()

			f.WriteString("[]") // Write empty JSON array
			return
		}
	} else {
		// When running in WASM, initialize empty guilds list
		st.guilds = []*typedef.Guild{}
		return
	}

	var rawGuilds typedef.GuildsFileJSON

	json.Unmarshal(f, &rawGuilds)
	for _, g := range rawGuilds {
		st.guilds = append(st.guilds, &typedef.Guild{
			Name: g.Name,
			Tag:  g.Tag,
			// Not implemented yet
			Allies: []*typedef.Guild{},
		})
	}

	// After all territories and guilds are loaded, rebuild the HQ map for fast lookups
	rebuildHQMap()
}

func loadTerritoryClaims() {
	f, err := os.ReadFile("territory_claims.json")
	if err != nil {
		// Claims file might not exist, continue without it
		return
	}

	var claims []struct {
		Territory string `json:"territory"`
		GuildName string `json:"guild_name"`
		GuildTag  string `json:"guild_tag"`
	}

	err = json.Unmarshal(f, &claims)
	if err != nil {
		panic("failed to unmarshal territory_claims.json: " + err.Error())
	}

	for _, claim := range claims {
		TerritoryClaimsMap[claim.Territory] = &typedef.Guild{
			Name: claim.GuildName,
			Tag:  claim.GuildTag,
		}
	}
}

func loadCosts(st *state) {
	f, err := assets.AssetFiles.ReadFile("upgrades.json")
	if err != nil {
		panic("failed to read upgrades.json: " + err.Error())
	}

	var costs typedef.Costs
	err = json.Unmarshal(f, &costs)
	if err != nil {
		panic("failed to unmarshal upgrades.json: " + err.Error())
	}

	st.costs = costs
}

func initializeTerritory(name string, tj typedef.TerritoryJSON) (*typedef.Territory, error) {
	return typedef.NewTerritory(name, tj) // Default guild
}

// Loads the state from ETF file
func loadFromETF(path string) error {
	// Load ETF file and decode it into the state
	return nil
}

func (s *state) load(newState *state) {
	s.territories = newState.territories
	s.guilds = newState.guilds
	s.activeTributes = newState.activeTributes
	s.savedSnapshots = newState.savedSnapshots
	s.tick = newState.tick

	// Rebuild the territory map for fast lookups
	s.territoryMap = make(map[string]*typedef.Territory)
	for _, territory := range s.territories {
		if territory != nil {
			s.territoryMap[territory.ID] = territory
		}
	}
}
