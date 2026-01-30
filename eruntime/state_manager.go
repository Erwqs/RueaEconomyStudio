package eruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"RueaES/storage"
	"RueaES/typedef"

	"github.com/gookit/goutil/arrutil"
	"github.com/pierrec/lz4"
)

var supportedStateVersion = []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6", "1.7", "1.8", "1.9"}

// Callback functions for user data that persists through resets
var (
	getLoadoutsCallback      func() []typedef.Loadout
	setLoadoutsCallback      func([]typedef.Loadout)
	mergeLoadoutsCallback    func([]typedef.Loadout) // Merges loadouts, keeping existing ones with same names
	getGuildColorsCallback   func() map[string]map[string]string
	setGuildColorsCallback   func(map[string]map[string]string)
	mergeGuildColorsCallback func(map[string]map[string]string) // Merges guild colors
	getPluginsCallback       func() []typedef.PluginState
	setPluginsCallback       func([]typedef.PluginState)
)

func normalizeRuntimeOptions(opts *typedef.RuntimeOptions) {
	defaults := typedef.DefaultResourceColors()

	if opts.ResourceColors.Wood.A == 0 {
		opts.ResourceColors.Wood = defaults.Wood
	}
	if opts.ResourceColors.Crop.A == 0 {
		opts.ResourceColors.Crop = defaults.Crop
	}
	if opts.ResourceColors.Fish.A == 0 {
		opts.ResourceColors.Fish = defaults.Fish
	}
	if opts.ResourceColors.Ore.A == 0 {
		opts.ResourceColors.Ore = defaults.Ore
	}
	if opts.ResourceColors.Multi.A == 0 {
		opts.ResourceColors.Multi = defaults.Multi
	}
	if opts.ResourceColors.Emerald.A == 0 {
		opts.ResourceColors.Emerald = defaults.Emerald
	}

	// Normalize keybinds and ensure defaults are present.
	typedef.NormalizeKeybinds(&opts.Keybinds)

	// Normalize plugin keybind overrides if present.
	typedef.NormalizePluginKeybinds(&opts.PluginKeybinds)

	// Normalize computation source.
	switch opts.ComputationSource {
	case typedef.ComputationCPU, typedef.ComputationGPU:
		// ok
	default:
		opts.ComputationSource = typedef.ComputationCPU
	}

	// Normalize pathfinding algorithm.
	switch opts.PathfindingAlgorithm {
	case typedef.PathfindingDijkstra,
		typedef.PathfindingAstar,
		typedef.PathfindingFloodFill,
		typedef.PathfindingBellmanFord,
		typedef.PathfindingFloydWarshall:
		// ok
	default:
		opts.PathfindingAlgorithm = typedef.PathfindingDijkstra
	}
}

// sanitizeLoadedState normalizes guild names/tags and territory names to strip banned phrases from loaded saves.
func sanitizeLoadedState(state *StateData) {
	rewrite := func(value string) string {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "new moon") {
			value, lower, _ = replaceInsensitivePreserveCase(value, lower, "new moon", "full moon")
		}
		if strings.Contains(lower, "newm") {
			value, lower, _ = replaceInsensitivePreserveCase(value, lower, "newm", "fumo")
		}
		return value
	}

	// Update guilds
	for _, g := range state.Guilds {
		if g == nil {
			continue
		}
		g.Name = rewrite(g.Name)
		g.Tag = rewrite(g.Tag)
	}

	// Update guild color entries (keyed by guild name)
	if len(state.GuildColors) > 0 {
		updated := make(map[string]map[string]string, len(state.GuildColors))
		for name, data := range state.GuildColors {
			newName := rewrite(name)
			if data == nil {
				updated[newName] = nil
				continue
			}

			newData := make(map[string]string, len(data))
			for k, v := range data {
				if k == "tag" {
					newData[k] = rewrite(v)
				} else {
					newData[k] = v
				}
			}
			updated[newName] = newData
		}

		state.GuildColors = updated
	}

	// Update territories and linked names
	for _, t := range state.Territories {
		if t == nil {
			continue
		}

		t.Name = rewrite(t.Name)
		t.Guild.Name = rewrite(t.Guild.Name)
		t.Guild.Tag = rewrite(t.Guild.Tag)

		if len(t.ConnectedTerritories) > 0 {
			for i, name := range t.ConnectedTerritories {
				t.ConnectedTerritories[i] = rewrite(name)
			}
		}

		if len(t.TradingRoutesJSON) > 0 {
			for i, route := range t.TradingRoutesJSON {
				for j, name := range route {
					t.TradingRoutesJSON[i][j] = rewrite(name)
				}
			}
		}

		if len(t.TransitResource) > 0 {
			for i := range t.TransitResource {
				tr := &t.TransitResource[i]
				tr.OriginID = rewrite(tr.OriginID)
				tr.DestinationID = rewrite(tr.DestinationID)
				tr.NextID = rewrite(tr.NextID)
				if len(tr.Route2) > 0 {
					for j, name := range tr.Route2 {
						tr.Route2[j] = rewrite(name)
					}
				}
			}
		}
	}
}

// replaceInsensitivePreserveCase swaps target occurrences regardless of case while leaving other characters untouched.
// This is duplicated here to avoid importing app logic into eruntime.
func replaceInsensitivePreserveCase(value, lowerValue, target, replacement string) (string, string, bool) {
	targetLower := strings.ToLower(target)
	if !strings.Contains(lowerValue, targetLower) {
		return value, lowerValue, false
	}

	var b strings.Builder
	start := 0
	replaced := false

	for {
		idx := strings.Index(lowerValue[start:], targetLower)
		if idx == -1 {
			break
		}
		idx += start
		b.WriteString(value[start:idx])
		b.WriteString(replacement)
		start = idx + len(target)
		replaced = true
	}

	b.WriteString(value[start:])
	newValue := b.String()
	return newValue, strings.ToLower(newValue), replaced
}

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

// SetPluginCallbacks sets the callback functions for plugin persistence
func SetPluginCallbacks(getFunc func() []typedef.PluginState, setFunc func([]typedef.PluginState)) {
	getPluginsCallback = getFunc
	setPluginsCallback = setFunc
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

	// Transit system data (version 1.4+) - new TransitManager system
	Transits []*Transit `json:"transits,omitempty"` // Active transits from the new TransitManager

	// Transit resource interning (version 1.8+)
	ResourcePool       []typedef.BasicResources               `json:"resourcePool,omitempty"`       // Unique resource packets shared by reference
	TransitResourcesV2 map[string][]compressedTransitResource `json:"transitResourcesV2,omitempty"` // TerritoryID -> compressed transit resources
	TransitsV2         []compressedTransit                    `json:"transitsV2,omitempty"`         // TransitManager entries with shared resources

	// Persistent user data (version 1.3+)
	Loadouts    []typedef.Loadout            `json:"loadouts,omitempty"`    // Loadouts persist through resets
	GuildColors map[string]map[string]string `json:"guildColors,omitempty"` // Guild colors: guildName -> {tag, color}

	// Plugins (version 1.9+)
	Plugins []typedef.PluginState `json:"plugins,omitempty"`
}

// compressedTransitResource stores transit packets using either a pooled reference or inline resources.
// ResourceRef is 1-based to keep zero-values omittable in JSON; subtract 1 when decoding.
type compressedTransitResource struct {
	ResourceRef    int                     `json:"ref,omitempty"`
	ResourceInline *typedef.BasicResources `json:"res,omitempty"`
	OriginID       string                  `json:"o,omitempty"`
	DestinationID  string                  `json:"d,omitempty"`
	NextID         string                  `json:"n,omitempty"`
	Route          []string                `json:"r,omitempty"`
	RouteIndex     int                     `json:"ri,omitempty"`
	NextTax        float64                 `json:"t,omitempty"`
	Moved          bool                    `json:"m,omitempty"`
}

// compressedTransit mirrors Transit but references resources by pool when possible.
type compressedTransit struct {
	ID             string                  `json:"id"`
	ResourceRef    int                     `json:"ref,omitempty"`
	ResourceInline *typedef.BasicResources `json:"res,omitempty"`
	OriginID       string                  `json:"originId"`
	DestinationID  string                  `json:"destinationId"`
	Route          []string                `json:"route"`
	RouteIndex     int                     `json:"routeIndex,omitempty"`
	NextTax        float64                 `json:"nextTax,omitempty"`
	CreatedAt      uint64                  `json:"createdAt,omitempty"`
	Moved          bool                    `json:"moved,omitempty"`
}

// resourceKey produces a stable string key for a BasicResources value (used for interning).
func resourceKey(br typedef.BasicResources) string {
	return fmt.Sprintf("%.4f|%.4f|%.4f|%.4f|%.4f", br.Emeralds, br.Ores, br.Wood, br.Fish, br.Crops)
}

// SaveStateToFile saves the current state to a file with LZ4 compression
func SaveStateToFile(filepath string) error {
	// Capture current state under read lock - minimize lock time for better performance
	var stateData StateData

	st.mu.RLock()

	// Quick copy of basic state data (no deep copying yet)
	stateData.Type = "state_save"
	stateData.Version = "1.9"
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

	// Copy transits from the new TransitManager (version 1.4+)
	if st.transitManager != nil {
		allTransits := st.transitManager.GetAllTransits()
		stateData.Transits = make([]*Transit, 0, len(allTransits))
		for _, transit := range allTransits {
			if transit != nil {
				// Deep copy transit
				transitCopy := *transit
				stateData.Transits = append(stateData.Transits, &transitCopy)
			}
		}
	}

	// Copy user loadouts (version 1.3+) - these persist through resets
	if getLoadoutsCallback != nil {
		stateData.Loadouts = getLoadoutsCallback()
	}

	// Copy guild colors (version 1.3+) - these persist through resets
	if getGuildColorsCallback != nil {
		stateData.GuildColors = getGuildColorsCallback()
	}

	// Copy plugin metadata and persisted state (version 1.9+)
	if getPluginsCallback != nil {
		stateData.Plugins = getPluginsCallback()
	}

	// All the expensive operations (JSON marshal, compression, file write) happen
	// without holding any locks, so users can continue working

	// Compress transit payloads by interning duplicate resource packets (version 1.8+)
	compressTransitPayloads(&stateData)

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

	return nil
}

// expandCompressedTransitPayloads restores transit payloads from the compact representation.
// It is safe to call when no compact data is present.
func expandCompressedTransitPayloads(state *StateData) {
	if state == nil {
		return
	}

	resolveResource := func(ref int, inline *typedef.BasicResources, pool []typedef.BasicResources) typedef.BasicResources {
		if inline != nil {
			return *inline
		}
		if ref <= 0 {
			return typedef.BasicResources{}
		}
		idx := ref - 1
		if idx >= 0 && idx < len(pool) {
			return pool[idx]
		}
		return typedef.BasicResources{}
	}

	pool := state.ResourcePool

	// Restore per-territory transit resources
	if len(state.TransitResourcesV2) > 0 && len(state.Territories) > 0 {
		terrByID := make(map[string]*typedef.Territory, len(state.Territories))
		for _, terr := range state.Territories {
			if terr != nil {
				terrByID[terr.ID] = terr
			}
		}

		for terrID, compacted := range state.TransitResourcesV2 {
			terr := terrByID[terrID]
			if terr == nil {
				continue
			}

			terr.TransitResource = make([]typedef.InTransitResources, 0, len(compacted))
			for _, c := range compacted {
				br := resolveResource(c.ResourceRef, c.ResourceInline, pool)
				terr.TransitResource = append(terr.TransitResource, typedef.InTransitResources{
					BasicResources: br,
					OriginID:       c.OriginID,
					DestinationID:  c.DestinationID,
					NextID:         c.NextID,
					Route2:         append([]string(nil), c.Route...),
					RouteIndex:     c.RouteIndex,
					NextTax:        c.NextTax,
					Moved:          c.Moved,
				})
			}
		}
	}

	// Restore TransitManager entries
	if len(state.TransitsV2) > 0 {
		rebuilt := make([]*Transit, 0, len(state.TransitsV2))
		for _, c := range state.TransitsV2 {
			br := resolveResource(c.ResourceRef, c.ResourceInline, pool)
			transit := &Transit{
				ID:             c.ID,
				BasicResources: br,
				OriginID:       c.OriginID,
				DestinationID:  c.DestinationID,
				Route:          append([]string(nil), c.Route...),
				RouteIndex:     c.RouteIndex,
				NextTax:        c.NextTax,
				CreatedAt:      c.CreatedAt,
				Moved:          c.Moved,
			}
			rebuilt = append(rebuilt, transit)
		}
		state.Transits = rebuilt
	}
}

// compressTransitPayloads deduplicates identical BasicResources across transit packets.
// It builds a shared resource pool for packets used more than once and rewrites transit
// payloads to reference that pool; unique packets are stored inline. This trims the
// serialized state size when many transits carry the same amounts.
func compressTransitPayloads(state *StateData) {
	if state == nil {
		return
	}

	// First pass: count how many times each BasicResources appears.
	resourceCounts := make(map[string]int)
	resourceSamples := make(map[string]typedef.BasicResources)
	collect := func(br typedef.BasicResources) {
		key := resourceKey(br)
		resourceCounts[key]++
		if _, ok := resourceSamples[key]; !ok {
			resourceSamples[key] = br
		}
	}

	for _, territory := range state.Territories {
		if territory == nil {
			continue
		}
		for _, tr := range territory.TransitResource {
			collect(tr.BasicResources)
		}
	}

	for _, tr := range state.Transits {
		if tr == nil {
			continue
		}
		collect(tr.BasicResources)
	}

	// Build pool from resources that are reused.
	keys := make([]string, 0, len(resourceCounts))
	for key, count := range resourceCounts {
		if count > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	pool := make([]typedef.BasicResources, 0, len(keys))
	keyToIndex := make(map[string]int, len(keys))
	for _, key := range keys {
		keyToIndex[key] = len(pool)
		pool = append(pool, resourceSamples[key])
	}

	// Second pass: rewrite transit payloads to use references where possible.
	if len(state.Territories) > 0 {
		if state.TransitResourcesV2 == nil {
			state.TransitResourcesV2 = make(map[string][]compressedTransitResource)
		}

		for _, territory := range state.Territories {
			if territory == nil || len(territory.TransitResource) == 0 {
				continue
			}

			compacted := make([]compressedTransitResource, 0, len(territory.TransitResource))
			for _, tr := range territory.TransitResource {
				key := resourceKey(tr.BasicResources)
				if idx, ok := keyToIndex[key]; ok {
					compacted = append(compacted, compressedTransitResource{
						ResourceRef:   idx + 1,
						OriginID:      tr.OriginID,
						DestinationID: tr.DestinationID,
						NextID:        tr.NextID,
						Route:         append([]string(nil), tr.Route2...),
						RouteIndex:    tr.RouteIndex,
						NextTax:       tr.NextTax,
						Moved:         tr.Moved,
					})
				} else {
					resCopy := tr.BasicResources
					compacted = append(compacted, compressedTransitResource{
						ResourceInline: &resCopy,
						OriginID:       tr.OriginID,
						DestinationID:  tr.DestinationID,
						NextID:         tr.NextID,
						Route:          append([]string(nil), tr.Route2...),
						RouteIndex:     tr.RouteIndex,
						NextTax:        tr.NextTax,
						Moved:          tr.Moved,
					})
				}
			}

			// Replace in-structure transit payloads with compacted form to save space.
			state.TransitResourcesV2[territory.ID] = compacted
			territory.TransitResource = nil
		}
	}

	if len(state.Transits) > 0 {
		for _, tr := range state.Transits {
			if tr == nil {
				continue
			}

			key := resourceKey(tr.BasicResources)
			compacted := compressedTransit{
				ID:            tr.ID,
				OriginID:      tr.OriginID,
				DestinationID: tr.DestinationID,
				Route:         append([]string(nil), tr.Route...),
				RouteIndex:    tr.RouteIndex,
				NextTax:       tr.NextTax,
				CreatedAt:     tr.CreatedAt,
				Moved:         tr.Moved,
			}

			if idx, ok := keyToIndex[key]; ok {
				compacted.ResourceRef = idx + 1
			} else {
				resCopy := tr.BasicResources
				compacted.ResourceInline = &resCopy
			}

			state.TransitsV2 = append(state.TransitsV2, compacted)
		}

		// Clear verbose transit payloads once compact versions exist.
		state.Transits = nil
	}

	// Only attach the pool if we actually compacted something; inline-only saves don't need it.
	if len(state.TransitResourcesV2) > 0 || len(state.TransitsV2) > 0 {
		state.ResourcePool = pool
	}
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
		"plugins":          true,
	}
	return loadStateFromFileInternal(filepath, importOptions)
}

// loadStateFromFileInternal is the internal implementation that handles selective loading
func loadStateFromFileInternal(filepath string, importOptions map[string]bool) error {
	if importOptions == nil {
		importOptions = make(map[string]bool)
	}

	// Ensure plugins import defaults to true if not specified
	if _, ok := importOptions["plugins"]; !ok {
		importOptions["plugins"] = true
	}

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

	// Sanitize loaded names to strip banned guild/territory strings while preserving other content.
	sanitizeLoadedState(&stateData)

	// Rehydrate transit payloads from compact form (1.8+) before applying import options
	expandCompressedTransitPayloads(&stateData)

	// Import core runtime data (if requested)
	if importOptions["core"] {
		st.mu.Lock()
		st.tick = stateData.Tick
		st.runtimeOptions = stateData.RuntimeOptions
		if stateData.Version != "1.5" && st.runtimeOptions.MapOpacityPercent == 0 {
			st.runtimeOptions.MapOpacityPercent = 100 // Backward compatibility for older saves
		}
		// Default new chokepoint options for older saves
		if st.runtimeOptions.ChokepointEmeraldWeight == 0 {
			st.runtimeOptions.ChokepointEmeraldWeight = 1
		}
		if st.runtimeOptions.ChokepointMode == "" {
			st.runtimeOptions.ChokepointMode = "Cardinal"
		}
		if st.runtimeOptions.ChokepointMode == "Cardinal" && !st.runtimeOptions.ChokepointIncludeDownstream {
			// For legacy saves that lacked the field, default to true unless explicitly stored
			st.runtimeOptions.ChokepointIncludeDownstream = true
		}
		normalizeRuntimeOptions(&st.runtimeOptions)
		st.mu.Unlock()
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
	}

	rebuildHQMap()

	// Import tributes (if requested)
	if importOptions["tributes"] {
		st.activeTributes = stateData.ActiveTributes
		// Rebuild guild pointers in tributes
		rebuildTributeGuildPointers()
	} else {
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

	ReloadDefaultCosts()

	// st.mu.Unlock()

	// st.mu.Lock()
	st.stateLoading = false
	st.mu.Unlock()

	if !wasHalted {
		st.start()
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		NotifyTerritoryColorsUpdate()
	}()

	// Load persistent user data (loadouts) - version 1.3+ (if requested)
	if importOptions["loadouts"] && mergeLoadoutsCallback != nil && stateData.Loadouts != nil {
		mergeLoadoutsCallback(stateData.Loadouts)
	}

	// Load guild colors - version 1.3+ (if guilds are imported)
	if importOptions["guilds"] && mergeGuildColorsCallback != nil && stateData.GuildColors != nil {
		mergeGuildColorsCallback(stateData.GuildColors)
	}

	// Load plugins - version 1.9+
	if importOptions["plugins"] && setPluginsCallback != nil {
		setPluginsCallback(stateData.Plugins)
	}

	// Load transits from TransitManager - version 1.4+ (if requested)
	if importOptions["in_transit"] && st.transitManager != nil && stateData.Transits != nil {
		st.transitManager.LoadTransits(stateData.Transits)
	} else if !importOptions["in_transit"] && st.transitManager != nil {
		// Clear transits if not importing
		st.transitManager.ClearAllTransits()
	}

	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Second)
			st.mu.RLock()
			hqCount := 0
			for _, territory := range st.territories {
				if territory != nil && territory.HQ {
					hqCount++
				}
			}
			st.mu.RUnlock()
		}
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
	err = storage.WriteDataFile("guilds.json", jsonData, 0644)
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
	}

	// Only notify guild managers to handle the merge themselves, don't overwrite their files
	if len(newGuilds) > 0 || len(updatedGuilds) > 0 {

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
	for _, hqTerritories := range guildHQs {
		if len(hqTerritories) > 1 {
			// Keep the first HQ found (from state file) and clear others
			// This ensures state file takes precedence
			for i, territory := range hqTerritories {
				if i == 0 {
				} else {
					territory.Mu.Lock()
					territory.HQ = false
					territory.Mu.Unlock()
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
