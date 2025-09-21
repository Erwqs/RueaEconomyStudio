package api

import (
	"encoding/json"
	"etools/eruntime"
	"etools/typedef"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin for development
		// In production, you should implement proper origin checking
		return true
	},
}

// Global API instance
var apiInstance *API

// Start a WebSocket server on port 42069
func StartWebSocketServer() {
	apiInstance = NewAPI()
	go apiInstance.run()

	http.HandleFunc("/ws", handleWebSocket)

	log.Println("WebSocket server starting on :42069")
	if err := http.ListenAndServe(":42069", nil); err != nil {
		log.Fatal("WebSocket server failed to start:", err)
	}
}

// Start HTTP server (placeholder for future REST endpoints)
func StartHTTPServer() {
	// Future implementation for REST API
}

// NewAPI creates a new API instance
func NewAPI() *API {
	api := &API{
		clients:       make(map[*WSClient]bool),
		clientOptions: make(map[*WSClient]*ClientOptions),
		broadcast:     make(chan WSMessage, 256),
		register:      make(chan *WSClient),
		unregister:    make(chan *WSClient),
		handlers:      make(map[MessageType]MessageHandler),
	}

	// Register message handlers
	api.registerHandlers()

	return api
}

// run handles the main WebSocket hub logic
func (api *API) run() {
	// Listen for state ticks from the simulation engine
	go api.listenForStateTicks()

	for {
		select {
		case client := <-api.register:
			api.clients[client] = true
			api.clientOptions[client] = &ClientOptions{
				IncludeTerritoryStats: false,
				IncludeGuildStats:     false,
				IncludeTributeStats:   false,
			}

			// Send acknowledgment
			ackMsg := WSMessage{
				Type:      MessageTypeAck,
				Data:      "Connected to simulation engine",
				Timestamp: time.Now(),
			}
			select {
			case client.send <- ackMsg:
			default:
				close(client.send)
				delete(api.clients, client)
				delete(api.clientOptions, client)
			}

			log.Printf("Client %s connected", client.id)

		case client := <-api.unregister:
			if _, ok := api.clients[client]; ok {
				delete(api.clients, client)
				delete(api.clientOptions, client)
				close(client.send)
				log.Printf("Client %s disconnected", client.id)
			}

		case message := <-api.broadcast:
			for client := range api.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(api.clients, client)
					delete(api.clientOptions, client)
				}
			}
		}
	}
}

// listenForStateTicks listens for state ticks and broadcasts them to clients
func (api *API) listenForStateTicks() {
	tickChannel := eruntime.GetStateTick()

	for tick := range tickChannel {
		// Create state tick data
		stateData := api.createStateTickData(tick)

		// Broadcast to all clients
		message := WSMessage{
			Type:      MessageTypeStateTick,
			Data:      stateData,
			Timestamp: time.Now(),
		}

		select {
		case api.broadcast <- message:
		default:
			// Channel is full, skip this tick
		}
	}
}

// createStateTickData creates the state data for a tick
func (api *API) createStateTickData(tick uint64) *StateTickData {
	actualTPS, tickTime, queueUtil := eruntime.GetTickProcessingPerformance()

	data := &StateTickData{
		Tick:             tick,
		TotalTerritories: len(eruntime.GetTerritories()),
		TotalGuilds:      len(eruntime.GetAllGuilds()),
		IsHalted:         eruntime.IsHalted(),
		ActualTPS:        actualTPS,
		TickProcessTime:  tickTime.String(),
		QueueUtilization: queueUtil,
	}

	// Add detailed stats if any client requests them
	includeTerritory := false
	includeGuild := false
	includeTribute := false

	for _, options := range api.clientOptions {
		if options.IncludeTerritoryStats {
			includeTerritory = true
		}
		if options.IncludeGuildStats {
			includeGuild = true
		}
		if options.IncludeTributeStats {
			includeTribute = true
		}
	}

	if includeTerritory {
		data.TerritoryStats = api.createTerritoryStats()
	}

	if includeGuild {
		data.GuildStats = api.createGuildStats()
	}

	if includeTribute {
		data.TributeStats = api.createTributeStats()
	}

	return data
}

// createTerritoryStats creates safe territory statistics
func (api *API) createTerritoryStats() map[string]*TerritoryStateSafe {
	territories := eruntime.GetTerritories()
	stats := make(map[string]*TerritoryStateSafe)

	for _, territory := range territories {
		if territory == nil {
			continue
		}

		territory.Mu.RLock()

		// Get connected territories and trading routes from eruntime - use unsafe versions to avoid deadlock
		connectedTerritories := eruntime.GetTerritoryConnectionsUnsafe(territory.Name)
		tradingRoutes := eruntime.GetTerritoryTradingRouteUnsafe(territory.Name)

		stats[territory.Name] = &TerritoryStateSafe{
			ID:                   territory.ID,
			Name:                 territory.Name,
			GuildName:            territory.Guild.Name,
			GuildTag:             territory.Guild.Tag,
			Location:             territory.Location,
			HQ:                   territory.HQ,
			Level:                territory.LevelInt,
			Storage:              territory.Storage,
			ResourceGeneration:   territory.ResourceGeneration,
			Treasury:             territory.Treasury,
			TreasuryOverride:     territory.TreasuryOverride,
			GenerationBonus:      territory.GenerationBonus,
			CapturedAt:           territory.CapturedAt,
			ConnectedTerritories: connectedTerritories, // Direct connections
			TradingRoutesJSON:    tradingRoutes,        // Actual trading routes
			RouteTax:             territory.RouteTax,
			RoutingMode:          territory.RoutingMode,
			Border:               territory.Border,
			Tax:                  territory.Tax,
			TransitResourceCount: len(territory.TransitResource),
			TowerStats:           territory.TowerStats,
			Upgrades:             territory.Options.Upgrade,
			Bonuses:              territory.Options.Bonus,
			Warning:              territory.Warning,
		}
		territory.Mu.RUnlock()
	}

	return stats
}

// createGuildStats creates safe guild statistics
func (api *API) createGuildStats() map[string]*GuildStateSafe {
	guilds := getGuildsFromRuntime()
	stats := make(map[string]*GuildStateSafe)

	for _, guild := range guilds {
		if guild == nil {
			continue
		}

		// Use the safe AllyTags field instead of the potentially circular Allies field
		allyNames := make([]string, len(guild.AllyTags))
		allyTags := make([]string, len(guild.AllyTags))

		// For now, we'll just use the tags. If we need names, we'd need to look them up
		copy(allyTags, guild.AllyTags)

		// Look up ally names from tags if needed
		for i, tag := range guild.AllyTags {
			allyTags[i] = tag
			// Find ally name by tag - simple lookup
			for _, otherGuild := range guilds {
				if otherGuild != nil && otherGuild.Tag == tag {
					allyNames[i] = otherGuild.Name
					break
				}
			}
		}

		stats[guild.Name] = &GuildStateSafe{
			Name:       guild.Name,
			Tag:        guild.Tag,
			TributeIn:  guild.TributeIn,
			TributeOut: guild.TributeOut,
			AllyNames:  allyNames,
			AllyTags:   allyTags,
		}
	}

	return stats
}

// createTributeStats creates safe tribute statistics
func (api *API) createTributeStats() []*ActiveTributeSafe {
	tributes := eruntime.GetActiveTributes()
	stats := make([]*ActiveTributeSafe, 0, len(tributes))

	for _, tribute := range tributes {
		if tribute == nil {
			continue
		}

		stats = append(stats, &ActiveTributeSafe{
			ID:              tribute.ID,
			FromGuildName:   tribute.FromGuildName,
			ToGuildName:     tribute.ToGuildName,
			AmountPerHour:   tribute.AmountPerHour,
			AmountPerMinute: tribute.AmountPerMinute,
			IntervalMinutes: tribute.IntervalMinutes,
			LastTransfer:    tribute.LastTransfer,
			IsActive:        tribute.IsActive,
			CreatedAt:       tribute.CreatedAt,
		})
	}

	return stats
}

// getGuildsFromRuntime safely gets guilds from the eruntime state
func getGuildsFromRuntime() []*typedef.Guild {
	// Use reflection or call a function that accesses st.guilds safely
	// Since we can't access st.guilds directly from this package,
	// we'll create a helper function in eruntime package
	return eruntime.GetGuildsInternal()
}

// handleWebSocket handles WebSocket connections
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &WSClient{
		conn: conn,
		send: make(chan WSMessage, 256),
		api:  apiInstance,
		id:   fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	client.api.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// writePump pumps messages from the hub to the websocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteJSON(websocket.CloseMessage)
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Error writing message to client %s: %v", c.id, err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			if err := c.conn.WriteJSON(WSMessage{
				Type:      "ping",
				Timestamp: time.Now(),
			}); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.api.unregister <- c
		c.conn.Close()
	}()

	for {
		var message WSMessage
		if err := c.conn.ReadJSON(&message); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Set timestamp if not provided
		if message.Timestamp.IsZero() {
			message.Timestamp = time.Now()
		}

		// Handle the message
		if err := c.handleMessage(message); err != nil {
			errorMsg := WSMessage{
				Type:      MessageTypeError,
				RequestID: message.RequestID,
				Error:     err.Error(),
				Timestamp: time.Now(),
			}

			select {
			case c.send <- errorMsg:
			default:
				close(c.send)
				return
			}
		}
	}
}

// handleMessage processes incoming messages from clients
func (c *WSClient) handleMessage(message WSMessage) error {
	handler, exists := c.api.handlers[message.Type]
	if !exists {
		return fmt.Errorf("unknown message type: %s", message.Type)
	}

	return handler(c, message)
}

// registerHandlers registers all message handlers
func (api *API) registerHandlers() {
	api.handlers[MessageTypeSetGuild] = api.handleSetGuild
	api.handlers[MessageTypeSetTerritoryOpt] = api.handleSetTerritoryOptions
	api.handlers[MessageTypeSetHQ] = api.handleSetHQ
	api.handlers[MessageTypeModifyStorage] = api.handleModifyStorage
	api.handlers[MessageTypeSetTickRate] = api.handleSetTickRate
	api.handlers[MessageTypeHalt] = api.handleHalt
	api.handlers[MessageTypeResume] = api.handleResume
	api.handlers[MessageTypeNextTick] = api.handleNextTick
	api.handlers[MessageTypeReset] = api.handleReset
	api.handlers[MessageTypeLoadState] = api.handleLoadState
	api.handlers[MessageTypeSaveState] = api.handleSaveState
	api.handlers[MessageTypeStateData] = api.handleStateData

	// Territory query handlers
	api.handlers[MessageTypeGetTerritoryStats] = api.handleGetTerritoryStats
	api.handlers[MessageTypeGetAllTerritories] = api.handleGetAllTerritories
	api.handlers[MessageTypeGetTerritories] = api.handleGetTerritories

	// Territory editing handlers
	api.handlers[MessageTypeSetTerritoryBonuses] = api.handleSetTerritoryBonuses
	api.handlers[MessageTypeSetTerritoryUpgrades] = api.handleSetTerritoryUpgrades
	api.handlers[MessageTypeSetTerritoryTax] = api.handleSetTerritoryTax
	api.handlers[MessageTypeSetTerritoryBorder] = api.handleSetTerritoryBorder
	api.handlers[MessageTypeSetTerritoryRoutingMode] = api.handleSetTerritoryRoutingMode
	api.handlers[MessageTypeSetTerritoryTreasury] = api.handleSetTerritoryTreasury

	// Tribute management handlers
	api.handlers[MessageTypeCreateTribute] = api.handleCreateTribute
	api.handlers[MessageTypeEditTribute] = api.handleEditTribute
	api.handlers[MessageTypeDisableTribute] = api.handleDisableTribute
	api.handlers[MessageTypeEnableTribute] = api.handleEnableTribute
	api.handlers[MessageTypeDeleteTribute] = api.handleDeleteTribute
	api.handlers[MessageTypeGetTributes] = api.handleGetTributes
	api.handlers[MessageTypeGetTributeStats] = api.handleGetTributeStats

	// Guild management handlers
	api.handlers[MessageTypeCreateGuild] = api.handleCreateGuild
	api.handlers[MessageTypeDeleteGuild] = api.handleDeleteGuild
	api.handlers[MessageTypeGetGuilds] = api.handleGetGuilds
	api.handlers[MessageTypeSearchGuilds] = api.handleSearchGuilds
	api.handlers[MessageTypeEditGuild] = api.handleEditGuild
	api.handlers[MessageTypeSetGuildAllies] = api.handleSetGuildAllies
}

// Helper functions

func (api *API) getGuildNameByTag(tag string) (string, error) {
	guilds := eruntime.GetGuildsInternal()

	for _, guild := range guilds {
		if guild.Tag == tag {
			return guild.Name, nil
		}
	}

	return "", fmt.Errorf("guild with tag '%s' not found", tag)
}

// convertTradingRoutes converts trading routes to the JSON format expected by the client
func convertTradingRoutes(routes [][]string) [][]string {
	// Routes are already in the correct format, just return them
	return routes
}

// Message handlers

func (api *API) handleSetGuild(client *WSClient, message WSMessage) error {
	var data SetGuildData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get the current territory to check if guild is actually changing
	currentTerritory := eruntime.GetTerritory(data.TerritoryName)
	if currentTerritory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Check if the guild is actually changing
	oldGuildName := currentTerritory.Guild.Name
	oldGuildTag := currentTerritory.Guild.Tag
	isGuildChanging := oldGuildName != data.GuildName || oldGuildTag != data.GuildTag

	guild := typedef.Guild{
		Name: data.GuildName,
		Tag:  data.GuildTag,
	}

	// Set the new guild (this handles HQ clearing and treasury reset automatically)
	eruntime.SetGuild(data.TerritoryName, guild)

	// If guild ownership changed, reset all territory configurations to defaults
	if isGuildChanging {
		// Reset upgrades to level 0
		defaultUpgrades := typedef.Upgrade{
			Damage:  0,
			Attack:  0,
			Health:  0,
			Defence: 0,
		}

		// Reset bonuses to level 0
		defaultBonuses := typedef.Bonus{
			StrongerMinions:       0,
			TowerMultiAttack:      0,
			TowerAura:             0,
			TowerVolley:           0,
			GatheringExperience:   0,
			MobExperience:         0,
			MobDamage:             0,
			PvPDamage:             0,
			XPSeeking:             0,
			TomeSeeking:           0,
			EmeraldSeeking:        0,
			LargerResourceStorage: 0,
			LargerEmeraldStorage:  0,
			EfficientResource:     0,
			EfficientEmerald:      0,
			ResourceRate:          0,
			EmeraldRate:           0,
		}

		// Reset tax to 0 (no tax)
		defaultTax := typedef.TerritoryTax{
			Tax:  0.0,
			Ally: 0.0,
		}

		// Set territory options to defaults
		defaultOptions := typedef.TerritoryOptions{
			Upgrades:    defaultUpgrades,
			Bonuses:     defaultBonuses,
			Tax:         defaultTax,
			RoutingMode: typedef.RoutingCheapest, // Default to cheapest routing
			Border:      typedef.BorderOpen,      // Default to open border
			HQ:          false,                   // Already cleared by SetGuild
		}

		// Apply the default options
		eruntime.Set(data.TerritoryName, defaultOptions)

		// Clear storage to empty
		emptyStorage := typedef.BasicResources{
			Emeralds: 0,
			Ores:     0,
			Wood:     0,
			Fish:     0,
			Crops:    0,
		}
		eruntime.ModifyStorageState(data.TerritoryName, &emptyStorage)

		// Notify the GUI about territory color and visual state changes
		eruntime.NotifyTerritoryColorsUpdate()

		// Notify both old and new guilds about the changes
		if oldGuildName != "" {
			eruntime.NotifyGuildSpecificUpdate(oldGuildName)
		}
		if data.GuildName != oldGuildName {
			eruntime.NotifyGuildSpecificUpdate(data.GuildName)
		}
	}

	// Send acknowledgment
	var acknowledgmentMessage string
	if isGuildChanging {
		acknowledgmentMessage = fmt.Sprintf("Guild changed for territory %s from %s[%s] to %s[%s] - all settings reset to defaults",
			data.TerritoryName, oldGuildName, oldGuildTag, data.GuildName, data.GuildTag)
	} else {
		acknowledgmentMessage = fmt.Sprintf("Guild reconfirmed for territory %s as %s[%s]",
			data.TerritoryName, data.GuildName, data.GuildTag)
	}

	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      acknowledgmentMessage,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleStateData(client *WSClient, message WSMessage) error {
	// Get current simulation state
	territories := eruntime.GetTerritories()
	guilds := eruntime.GetGuildsInternal()

	// Convert territories to safe types for JSON serialization
	territoryStats := make(map[string]*TerritoryStateSafe)
	for _, territory := range territories {
		if territory != nil {
			// Get connected territories and trading routes from eruntime
			connectedTerritories := eruntime.GetTerritoryConnections(territory.Name)
			tradingRoutes := eruntime.GetTerritoryTradingRoute(territory.Name)

			territoryStats[territory.Name] = &TerritoryStateSafe{
				ID:                   territory.ID,
				Name:                 territory.Name,
				GuildName:            territory.Guild.Name, // From Guild field
				GuildTag:             territory.Guild.Tag,  // From Guild field
				Location:             territory.Location,
				HQ:                   territory.HQ,
				Level:                uint8(territory.Level), // Convert DefenceLevel to uint8
				Storage:              territory.Storage,
				ResourceGeneration:   territory.ResourceGeneration,
				Treasury:             territory.Treasury,
				TreasuryOverride:     territory.TreasuryOverride,
				GenerationBonus:      territory.GenerationBonus,
				CapturedAt:           territory.CapturedAt,
				ConnectedTerritories: connectedTerritories, // Direct connections
				TradingRoutesJSON:    tradingRoutes,        // Actual trading routes
				RouteTax:             territory.RouteTax,
				RoutingMode:          territory.RoutingMode,
				Border:               territory.Border,
				Tax:                  territory.Tax,
				TransitResourceCount: 0, // Not available in Territory struct
				TowerStats:           territory.TowerStats,
				Upgrades:             territory.Options.Upgrade, // From Options.Upgrade
				Bonuses:              territory.Options.Bonus,   // From Options.Bonus
				Warning:              territory.Warning,
			}
		}
	}

	// Convert guilds to safe types
	guildStats := make(map[string]*GuildStateSafe)
	for _, guild := range guilds {
		if guild != nil {
			allyNames := make([]string, 0, len(guild.Allies))
			allyTags := make([]string, 0, len(guild.Allies))
			for _, ally := range guild.Allies {
				if ally != nil {
					allyNames = append(allyNames, ally.Name)
					allyTags = append(allyTags, ally.Tag)
				}
			}

			guildStats[guild.Tag] = &GuildStateSafe{
				Name:       guild.Name,
				Tag:        guild.Tag,
				TributeIn:  guild.TributeIn,
				TributeOut: guild.TributeOut,
				AllyNames:  allyNames,
				AllyTags:   allyTags,
			}
		}
	}

	// Create state data response using basic simulation state
	stateData := StateTickData{
		Tick:             eruntime.GetCurrentTick(),
		TotalTerritories: len(territories),
		TotalGuilds:      len(guilds),
		IsHalted:         eruntime.IsHalted(),
		ActualTPS:        0.0,   // Not available, set to default
		TickProcessTime:  "0ms", // Not available, set to default
		QueueUtilization: 0.0,   // Not available, set to default
		TerritoryStats:   territoryStats,
		GuildStats:       guildStats,
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeStateTick,
		RequestID: message.RequestID,
		Data:      stateData,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryOptions(client *WSClient, message WSMessage) error {
	var data SetTerritoryOptionsData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	eruntime.Set(data.TerritoryName, data.Options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Options set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetHQ(client *WSClient, message WSMessage) error {
	var data SetHQData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	options := typedef.TerritoryOptions{HQ: true}
	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("HQ set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleModifyStorage(client *WSClient, message WSMessage) error {
	var data ModifyStorageData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	eruntime.ModifyStorageState(data.TerritoryName, &data.NewState)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Storage modified for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTickRate(client *WSClient, message WSMessage) error {
	var data SetTickRateData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	eruntime.SetTickRate(data.TicksPerSecond)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tick rate set to %d TPS", data.TicksPerSecond),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleHalt(client *WSClient, message WSMessage) error {
	eruntime.Halt()

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      "Simulation halted",
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleResume(client *WSClient, message WSMessage) error {
	eruntime.Resume()

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      "Simulation resumed",
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleNextTick(client *WSClient, message WSMessage) error {
	eruntime.NextTick()

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      "Advanced one tick",
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleReset(client *WSClient, message WSMessage) error {
	eruntime.Reset()

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      "Simulation reset",
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleLoadState(client *WSClient, message WSMessage) error {
	var data LoadStateData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	eruntime.LoadState(data.Filepath)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("State loaded from %s", data.Filepath),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSaveState(client *WSClient, message WSMessage) error {
	var data SaveStateData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	eruntime.SaveState(data.Filepath)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("State saved to %s", data.Filepath),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

// Territory query handlers

func (api *API) handleGetTerritoryStats(client *WSClient, message WSMessage) error {
	var data GetTerritoryStatsData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get territory stats from eruntime
	stats := eruntime.GetTerritoryStats(data.TerritoryName)
	if stats == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Get the actual territory for location and other data
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Convert to safe type for JSON serialization
	safeStats := &TerritoryStateSafe{
		ID:        stats.Name, // Using name as ID for now
		Name:      stats.Name,
		GuildName: stats.Guild.Name,
		GuildTag:  stats.Guild.Tag,
		Location:  territory.Location, // Use actual location from territory
		HQ:        stats.HQ,
		Level:     uint8(stats.Upgrades.Defence), // Use defence level
		Storage: typedef.TerritoryStorage{
			Capacity: stats.StorageCapacity,
			At:       stats.StoredResources,
		},
		ResourceGeneration:   typedef.ResourceGeneration{}, // Will need to populate
		Treasury:             typedef.TreasuryLevel(0),     // Will need to get from territory
		TreasuryOverride:     typedef.TreasuryOverride(0),  // Will need to get from territory
		GenerationBonus:      stats.GenerationBonus,
		CapturedAt:           0,                          // Will need to get from territory
		ConnectedTerritories: stats.ConnectedTerritories, // Direct connections from eruntime
		TradingRoutesJSON:    stats.TradingRoutes,        // Actual trading routes from eruntime
		RouteTax:             stats.RouteTax,
		RoutingMode:          stats.RoutingMode,
		Border:               stats.Border,
		Tax:                  stats.Tax,
		TransitResourceCount: 0,
		TowerStats:           typedef.TowerStats{}, // Will need to populate
		Upgrades: typedef.TerritoryUpgrade{
			Set: stats.Upgrades,
			At:  stats.Upgrades, // For now, assume set and at are the same
		},
		Bonuses: typedef.TerritoryBonus{
			Set: stats.Bonuses,
			At:  stats.Bonuses, // For now, assume set and at are the same
		},
		Warning: stats.Warning,
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      safeStats,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleGetAllTerritories(client *WSClient, message WSMessage) error {
	// Get all territories from eruntime
	territories := eruntime.GetTerritories()

	// Convert to safe types for JSON serialization
	territoryStats := make(map[string]*TerritoryStateSafe)
	for _, territory := range territories {
		if territory != nil {
			// Get connected territories and trading routes from eruntime
			connectedTerritories := eruntime.GetTerritoryConnections(territory.Name)
			tradingRoutes := eruntime.GetTerritoryTradingRoute(territory.Name)

			territoryStats[territory.Name] = &TerritoryStateSafe{
				ID:                   territory.ID,
				Name:                 territory.Name,
				GuildName:            territory.Guild.Name,
				GuildTag:             territory.Guild.Tag,
				Location:             territory.Location,
				HQ:                   territory.HQ,
				Level:                uint8(territory.Level),
				Storage:              territory.Storage,
				ResourceGeneration:   territory.ResourceGeneration,
				Treasury:             territory.Treasury,
				TreasuryOverride:     territory.TreasuryOverride,
				GenerationBonus:      territory.GenerationBonus,
				CapturedAt:           territory.CapturedAt,
				ConnectedTerritories: connectedTerritories, // Direct connections
				TradingRoutesJSON:    tradingRoutes,        // Actual trading routes
				RouteTax:             territory.RouteTax,
				RoutingMode:          territory.RoutingMode,
				Border:               territory.Border,
				Tax:                  territory.Tax,
				TransitResourceCount: 0,
				TowerStats:           territory.TowerStats,
				Upgrades:             territory.Options.Upgrade,
				Bonuses:              territory.Options.Bonus,
				Warning:              territory.Warning,
			}
		}
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      territoryStats,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleGetTerritories(client *WSClient, message WSMessage) error {
	// Get basic territory list (just names and basic info)
	territories := eruntime.GetTerritories()

	// Create a simplified response with just essential info
	territoryList := make([]map[string]interface{}, 0, len(territories))
	for _, territory := range territories {
		if territory != nil {
			territoryInfo := map[string]interface{}{
				"id":         territory.ID,
				"name":       territory.Name,
				"guild_name": territory.Guild.Name,
				"guild_tag":  territory.Guild.Tag,
				"is_hq":      territory.HQ,
				"location":   territory.Location,
			}
			territoryList = append(territoryList, territoryInfo)
		}
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      territoryList,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

// Territory editing handlers

func (api *API) handleSetTerritoryBonuses(client *WSClient, message WSMessage) error {
	var data SetTerritoryBonusesData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory options and update bonuses
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Create new options with updated bonuses
	options := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     data.Bonuses,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Bonuses set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryUpgrades(client *WSClient, message WSMessage) error {
	var data SetTerritoryUpgradesData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory options and update upgrades
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Create new options with updated upgrades
	options := typedef.TerritoryOptions{
		Upgrades:    data.Upgrades,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Upgrades set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryTax(client *WSClient, message WSMessage) error {
	var data SetTerritoryTaxData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory options and update tax
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Create new options with updated tax
	options := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         data.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tax set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryBorder(client *WSClient, message WSMessage) error {
	var data SetTerritoryBorderData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory options and update border
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Create new options with updated border
	options := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: territory.RoutingMode,
		Border:      data.Border,
		HQ:          territory.HQ,
	}

	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Border control set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryRoutingMode(client *WSClient, message WSMessage) error {
	var data SetTerritoryRoutingModeData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory options and update routing mode
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Create new options with updated routing mode
	options := typedef.TerritoryOptions{
		Upgrades:    territory.Options.Upgrade.Set,
		Bonuses:     territory.Options.Bonus.Set,
		Tax:         territory.Tax,
		RoutingMode: data.RoutingMode,
		Border:      territory.Border,
		HQ:          territory.HQ,
	}

	eruntime.Set(data.TerritoryName, options)

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Routing mode set for territory %s", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetTerritoryTreasury(client *WSClient, message WSMessage) error {
	var data SetTerritoryTreasuryData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get current territory to update treasury override
	territory := eruntime.GetTerritory(data.TerritoryName)
	if territory == nil {
		return fmt.Errorf("territory not found: %s", data.TerritoryName)
	}

	// Note: TreasuryOverride is not part of TerritoryOptions, it's a direct field on Territory
	// We'll need to use a different approach or modify the territory directly
	// For now, let's try to set it via the existing options structure if possible

	// This might not work as expected since TreasuryOverride might not be in TerritoryOptions
	// We might need to add a specific function in eruntime for this

	// Send acknowledgment (even if the operation might not fully work)
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Treasury override requested for territory %s (implementation may be limited)", data.TerritoryName),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

// Tribute management handlers

func (api *API) handleCreateTribute(client *WSClient, message WSMessage) error {
	var data CreateTributeData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Convert guild tags to guild names
	fromGuildName, err := api.getGuildNameByTag(data.FromGuildTag)
	if err != nil {
		return fmt.Errorf("from guild with tag '%s' not found", data.FromGuildTag)
	}

	toGuildName, err := api.getGuildNameByTag(data.ToGuildTag)
	if err != nil {
		return fmt.Errorf("to guild with tag '%s' not found", data.ToGuildTag)
	}

	// Create the tribute using eruntime function
	tributeID, err := eruntime.CreateGuildToGuildTribute(
		fromGuildName,
		toGuildName,
		data.AmountPerHour,
		data.IntervalMinutes,
	)
	if err != nil {
		return fmt.Errorf("failed to create tribute: %w", err)
	}

	// Send acknowledgment with the tribute ID
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tribute created with ID: %s", tributeID),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleEditTribute(client *WSClient, message WSMessage) error {
	var data EditTributeData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get the existing tribute
	tribute := eruntime.GetTribute(data.TributeID)
	if tribute == nil {
		return fmt.Errorf("tribute with ID %s not found", data.TributeID)
	}

	// Update fields if provided
	if data.AmountPerHour != nil {
		tribute.AmountPerHour = *data.AmountPerHour
		// Recalculate amount per minute
		tribute.AmountPerMinute = typedef.BasicResources{
			Emeralds: data.AmountPerHour.Emeralds / 60,
			Ores:     data.AmountPerHour.Ores / 60,
			Wood:     data.AmountPerHour.Wood / 60,
			Fish:     data.AmountPerHour.Fish / 60,
			Crops:    data.AmountPerHour.Crops / 60,
		}
	}

	if data.IntervalMinutes != nil {
		tribute.IntervalMinutes = *data.IntervalMinutes
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tribute %s updated", data.TributeID),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleDisableTribute(client *WSClient, message WSMessage) error {
	var data TributeActionData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	err := eruntime.DisableTributeByID(data.TributeID)
	if err != nil {
		return fmt.Errorf("failed to disable tribute: %w", err)
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tribute %s disabled", data.TributeID),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleEnableTribute(client *WSClient, message WSMessage) error {
	var data TributeActionData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	err := eruntime.EnableTributeByID(data.TributeID)
	if err != nil {
		return fmt.Errorf("failed to enable tribute: %w", err)
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tribute %s enabled", data.TributeID),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleDeleteTribute(client *WSClient, message WSMessage) error {
	var data TributeActionData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	err := eruntime.DeleteTribute(data.TributeID)
	if err != nil {
		return fmt.Errorf("failed to delete tribute: %w", err)
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Tribute %s deleted", data.TributeID),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleGetTributes(client *WSClient, message WSMessage) error {
	var data GetTributesData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Get all active tributes
	allTributes := eruntime.GetAllActiveTributes()

	// Filter tributes based on criteria
	var filteredTributes []*typedef.ActiveTribute
	for _, tribute := range allTributes {
		// Filter by guild if specified
		if data.GuildTag != "" {
			if tribute.From != nil && tribute.From.Tag != data.GuildTag &&
				tribute.To != nil && tribute.To.Tag != data.GuildTag {
				continue
			}
		}

		// Filter by active/inactive status
		if tribute.IsActive && !data.IncludeActive {
			continue
		}
		if !tribute.IsActive && !data.IncludeInactive {
			continue
		}

		filteredTributes = append(filteredTributes, tribute)
	}

	// Convert to safe format (avoiding circular references)
	safeTributes := make([]*ActiveTributeSafe, 0, len(filteredTributes))
	for _, tribute := range filteredTributes {
		safeTributes = append(safeTributes, &ActiveTributeSafe{
			ID:              tribute.ID,
			FromGuildName:   tribute.FromGuildName,
			ToGuildName:     tribute.ToGuildName,
			AmountPerHour:   tribute.AmountPerHour,
			AmountPerMinute: tribute.AmountPerMinute,
			IntervalMinutes: tribute.IntervalMinutes,
			LastTransfer:    tribute.LastTransfer,
			IsActive:        tribute.IsActive,
			CreatedAt:       tribute.CreatedAt,
		})
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      safeTributes,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleGetTributeStats(client *WSClient, message WSMessage) error {
	var data GetTributeStatsData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	var responseData interface{}

	if data.TributeID != "" {
		// Get specific tribute
		tribute := eruntime.GetTribute(data.TributeID)
		if tribute == nil {
			return fmt.Errorf("tribute with ID %s not found", data.TributeID)
		}

		responseData = &ActiveTributeSafe{
			ID:              tribute.ID,
			FromGuildName:   tribute.FromGuildName,
			ToGuildName:     tribute.ToGuildName,
			AmountPerHour:   tribute.AmountPerHour,
			AmountPerMinute: tribute.AmountPerMinute,
			IntervalMinutes: tribute.IntervalMinutes,
			LastTransfer:    tribute.LastTransfer,
			IsActive:        tribute.IsActive,
			CreatedAt:       tribute.CreatedAt,
		}
	} else {
		// Get all tributes
		allTributes := eruntime.GetAllActiveTributes()
		safeTributes := make([]*ActiveTributeSafe, 0, len(allTributes))

		for _, tribute := range allTributes {
			safeTributes = append(safeTributes, &ActiveTributeSafe{
				ID:              tribute.ID,
				FromGuildName:   tribute.FromGuildName,
				ToGuildName:     tribute.ToGuildName,
				AmountPerHour:   tribute.AmountPerHour,
				AmountPerMinute: tribute.AmountPerMinute,
				IntervalMinutes: tribute.IntervalMinutes,
				LastTransfer:    tribute.LastTransfer,
				IsActive:        tribute.IsActive,
				CreatedAt:       tribute.CreatedAt,
			})
		}

		responseData = safeTributes
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      responseData,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

// Guild management handlers

func (api *API) handleCreateGuild(client *WSClient, message WSMessage) error {
	var data CreateGuildData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Check if guild already exists
	existingGuilds := eruntime.GetGuildsInternal()
	for _, guild := range existingGuilds {
		if guild.Tag == data.Tag {
			return fmt.Errorf("guild with tag '%s' already exists", data.Tag)
		}
		if guild.Name == data.Name {
			return fmt.Errorf("guild with name '%s' already exists", data.Name)
		}
	}

	// Create new guild - since there's no public API, we'll access the state directly
	// Note: This might need to be implemented in eruntime package for better encapsulation
	newGuild := &typedef.Guild{
		Name:       data.Name,
		Tag:        data.Tag,
		TributeIn:  typedef.BasicResources{},
		TributeOut: typedef.BasicResources{},
		Allies:     []*typedef.Guild{},
	}

	// This is a simplified implementation - in a real scenario, you'd want a proper
	// AddGuild function in the eruntime package
	err := api.addGuildToState(newGuild)
	if err != nil {
		return fmt.Errorf("failed to create guild: %w", err)
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Guild '%s [%s]' created successfully", data.Name, data.Tag),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleDeleteGuild(client *WSClient, message WSMessage) error {
	var data DeleteGuildData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	// Find and remove guild
	err := api.removeGuildFromState(data.GuildTag)
	if err != nil {
		return fmt.Errorf("failed to delete guild: %w", err)
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Guild with tag '%s' deleted successfully", data.GuildTag),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleGetGuilds(client *WSClient, message WSMessage) error {
	guilds := eruntime.GetGuildsInternal()

	// Convert to safe format
	safeGuilds := make([]*GuildStateSafe, 0, len(guilds))
	for _, guild := range guilds {
		if guild != nil {
			allyNames := make([]string, 0, len(guild.Allies))
			allyTags := make([]string, 0, len(guild.Allies))
			for _, ally := range guild.Allies {
				if ally != nil {
					allyNames = append(allyNames, ally.Name)
					allyTags = append(allyTags, ally.Tag)
				}
			}

			safeGuilds = append(safeGuilds, &GuildStateSafe{
				Name:       guild.Name,
				Tag:        guild.Tag,
				TributeIn:  guild.TributeIn,
				TributeOut: guild.TributeOut,
				AllyNames:  allyNames,
				AllyTags:   allyTags,
			})
		}
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      safeGuilds,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleSearchGuilds(client *WSClient, message WSMessage) error {
	var data SearchGuildsData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	guilds := eruntime.GetGuildsInternal()
	var matchedGuilds []*GuildStateSafe

	for _, guild := range guilds {
		if guild != nil {
			var matches bool

			if data.Exact {
				// Exact match
				matches = guild.Name == data.Query || (data.IncludeTag && guild.Tag == data.Query)
			} else {
				// Partial match (case-insensitive)
				lowerQuery := strings.ToLower(data.Query)
				lowerName := strings.ToLower(guild.Name)
				lowerTag := strings.ToLower(guild.Tag)

				matches = strings.Contains(lowerName, lowerQuery) ||
					(data.IncludeTag && strings.Contains(lowerTag, lowerQuery))
			}

			if matches {
				allyNames := make([]string, 0, len(guild.Allies))
				allyTags := make([]string, 0, len(guild.Allies))
				for _, ally := range guild.Allies {
					if ally != nil {
						allyNames = append(allyNames, ally.Name)
						allyTags = append(allyTags, ally.Tag)
					}
				}

				matchedGuilds = append(matchedGuilds, &GuildStateSafe{
					Name:       guild.Name,
					Tag:        guild.Tag,
					TributeIn:  guild.TributeIn,
					TributeOut: guild.TributeOut,
					AllyNames:  allyNames,
					AllyTags:   allyTags,
				})
			}
		}
	}

	// Send response
	response := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      matchedGuilds,
		Timestamp: time.Now(),
	}

	select {
	case client.send <- response:
	default:
	}

	return nil
}

func (api *API) handleEditGuild(client *WSClient, message WSMessage) error {
	var data EditGuildData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	guilds := eruntime.GetGuildsInternal()
	var targetGuild *typedef.Guild

	// Find the guild to edit
	for _, guild := range guilds {
		if guild != nil && guild.Tag == data.OldTag {
			targetGuild = guild
			break
		}
	}

	if targetGuild == nil {
		return fmt.Errorf("guild with tag '%s' not found", data.OldTag)
	}

	// Update fields if provided
	if data.Name != "" {
		targetGuild.Name = data.Name
	}
	if data.Tag != "" {
		targetGuild.Tag = data.Tag
	}

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Guild '%s' updated successfully", data.OldTag),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) handleSetGuildAllies(client *WSClient, message WSMessage) error {
	var data SetGuildAlliesData
	if err := api.parseMessageData(message.Data, &data); err != nil {
		return err
	}

	guilds := eruntime.GetGuildsInternal()
	var targetGuild *typedef.Guild

	// Find the target guild
	for _, guild := range guilds {
		if guild != nil && guild.Tag == data.GuildTag {
			targetGuild = guild
			break
		}
	}

	if targetGuild == nil {
		return fmt.Errorf("guild with tag '%s' not found", data.GuildTag)
	}

	// Find ally guilds
	var allyGuilds []*typedef.Guild
	for _, allyTag := range data.AllyTags {
		for _, guild := range guilds {
			if guild != nil && guild.Tag == allyTag {
				allyGuilds = append(allyGuilds, guild)
				break
			}
		}
	}

	// Update allies
	targetGuild.Allies = allyGuilds

	// Send acknowledgment
	ackMsg := WSMessage{
		Type:      MessageTypeAck,
		RequestID: message.RequestID,
		Data:      fmt.Sprintf("Allies set for guild '%s': %v", data.GuildTag, data.AllyTags),
		Timestamp: time.Now(),
	}

	select {
	case client.send <- ackMsg:
	default:
	}

	return nil
}

func (api *API) addGuildToState(guild *typedef.Guild) error {
	// This is a simplified implementation
	// In reality, you'd want a proper AddGuild function in eruntime
	// that handles all the necessary state management

	// For now, we'll just return an error suggesting manual implementation
	return fmt.Errorf("guild creation not yet implemented - please add guild manually to guilds.json")
}

func (api *API) removeGuildFromState(guildTag string) error {
	// This is a simplified implementation
	// In reality, you'd want a proper RemoveGuild function in eruntime
	// that handles all the necessary cleanup (territories, tributes, etc.)

	return fmt.Errorf("guild deletion not yet implemented - please remove guild manually from guilds.json")
}

// parseMessageData parses message data into the specified struct
func (api *API) parseMessageData(data interface{}, target interface{}) error {
	// Convert to JSON and back to ensure proper type conversion
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return nil
}
