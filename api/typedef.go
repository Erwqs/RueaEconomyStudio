package api

import (
	"RueaES/typedef"
	"time"
)

// WebSocket message types
type MessageType string

const (
	// Outgoing message types (server to client)
	MessageTypeStateTick MessageType = "state_tick"
	MessageTypeStateData MessageType = "state_data"
	MessageTypeError     MessageType = "error"
	MessageTypeAck       MessageType = "ack"

	// Incoming message types (client to server)
	MessageTypeSetGuild        MessageType = "set_guild"
	MessageTypeSetTerritoryOpt MessageType = "set_territory_options"
	MessageTypeSetHQ           MessageType = "set_hq"
	MessageTypeModifyStorage   MessageType = "modify_storage"
	MessageTypeSetTickRate     MessageType = "set_tick_rate"
	MessageTypeHalt            MessageType = "halt"
	MessageTypeResume          MessageType = "resume"
	MessageTypeNextTick        MessageType = "next_tick"
	MessageTypeReset           MessageType = "reset"
	MessageTypeLoadState       MessageType = "load_state"
	MessageTypeSaveState       MessageType = "save_state"

	// Territory query message types
	MessageTypeGetTerritoryStats MessageType = "get_territory_stats"
	MessageTypeGetAllTerritories MessageType = "get_all_territories"
	MessageTypeGetTerritories    MessageType = "get_territories"
	MessageTypeGetAlternativeRoutes MessageType = "get_alternative_routes"

	// Territory editing message types
	MessageTypeSetTerritoryBonuses     MessageType = "set_territory_bonuses"
	MessageTypeSetTerritoryUpgrades    MessageType = "set_territory_upgrades"
	MessageTypeSetTerritoryTax         MessageType = "set_territory_tax"
	MessageTypeSetTerritoryBorder      MessageType = "set_territory_border"
	MessageTypeSetTerritoryRoutingMode MessageType = "set_territory_routing_mode"
	MessageTypeSetTerritoryTreasury    MessageType = "set_territory_treasury"
	MessageTypeSetTradingRoute         MessageType = "set_trading_route"

	// Tribute management message types
	MessageTypeCreateTribute   MessageType = "create_tribute"
	MessageTypeEditTribute     MessageType = "edit_tribute"
	MessageTypeDisableTribute  MessageType = "disable_tribute"
	MessageTypeEnableTribute   MessageType = "enable_tribute"
	MessageTypeDeleteTribute   MessageType = "delete_tribute"
	MessageTypeGetTributes     MessageType = "get_tributes"
	MessageTypeGetTributeStats MessageType = "get_tribute_stats"

	// Guild management message types
	MessageTypeCreateGuild    MessageType = "create_guild"
	MessageTypeDeleteGuild    MessageType = "delete_guild"
	MessageTypeGetGuilds      MessageType = "get_guilds"
	MessageTypeSearchGuilds   MessageType = "search_guilds"
	MessageTypeEditGuild      MessageType = "edit_guild"
	MessageTypeSetGuildAllies MessageType = "set_guild_allies"
)

// Base WebSocket message structure
type WSMessage struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"request_id,omitempty"` // For correlating responses
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Outgoing message data structures

// StateTickData is sent every tick
type StateTickData struct {
	Tick             uint64                         `json:"tick"`
	TotalTerritories int                            `json:"total_territories"`
	TotalGuilds      int                            `json:"total_guilds"`
	IsHalted         bool                           `json:"is_halted"`
	ActualTPS        float64                        `json:"actual_tps"`
	TickProcessTime  string                         `json:"tick_process_time"` // Duration as string
	QueueUtilization float64                        `json:"queue_utilization"`
	TerritoryStats   map[string]*TerritoryStateSafe `json:"territory_stats,omitempty"` // Optional: only include if requested
	GuildStats       map[string]*GuildStateSafe     `json:"guild_stats,omitempty"`     // Optional: only include if requested
	TributeStats     []*ActiveTributeSafe           `json:"tribute_stats,omitempty"`   // Optional: only include if requested
}

// Safe versions of structs that avoid circular references for JSON serialization
type TerritoryStateSafe struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	GuildName            string                     `json:"guild_name"`
	GuildTag             string                     `json:"guild_tag"`
	Location             typedef.LocationObject     `json:"location"`
	HQ                   bool                       `json:"is_hq"`
	Level                uint8                      `json:"level"`
	Storage              typedef.TerritoryStorage   `json:"storage"`
	ResourceGeneration   typedef.ResourceGeneration `json:"resource_generation"`
	Treasury             typedef.TreasuryLevel      `json:"treasury"`
	TreasuryOverride     typedef.TreasuryOverride   `json:"treasury_override"`
	GenerationBonus      float64                    `json:"generation_bonus"`
	CapturedAt           uint64                     `json:"captured_at"`
	ConnectedTerritories []string                   `json:"connected_territories"`
	TradingRoutesJSON    [][]string                 `json:"trading_routes"`
	RouteTax             float64                    `json:"route_tax"`
	RoutingMode          typedef.Routing            `json:"routing_mode"`
	Border               typedef.Border             `json:"border"`
	Tax                  typedef.TerritoryTax       `json:"tax"`
	TransitResourceCount int                        `json:"transit_resource_count"`
	TowerStats           typedef.TowerStats         `json:"tower_stats"`
	Upgrades             typedef.TerritoryUpgrade   `json:"upgrades"`
	Bonuses              typedef.TerritoryBonus     `json:"bonuses"`
	Warning              typedef.Warning            `json:"warnings"`
}

type GuildStateSafe struct {
	Name       string                 `json:"name"`
	Tag        string                 `json:"tag"`
	TributeIn  typedef.BasicResources `json:"tribute_in"`
	TributeOut typedef.BasicResources `json:"tribute_out"`
	AllyNames  []string               `json:"ally_names"` // Just names, not pointers
	AllyTags   []string               `json:"ally_tags"`
}

type ActiveTributeSafe struct {
	ID              string                 `json:"id"`
	FromGuildName   string                 `json:"from_guild"`
	ToGuildName     string                 `json:"to_guild"`
	AmountPerHour   typedef.BasicResources `json:"amount_per_hour"`
	AmountPerMinute typedef.BasicResources `json:"amount_per_minute"`
	IntervalMinutes uint32                 `json:"interval_minutes"`
	LastTransfer    uint64                 `json:"last_transfer"`
	IsActive        bool                   `json:"is_active"`
	CreatedAt       uint64                 `json:"created_at"`
}

// Incoming message data structures

type SetGuildData struct {
	TerritoryName string `json:"territory_name"`
	GuildName     string `json:"guild_name"`
	GuildTag      string `json:"guild_tag"`
}

type SetTerritoryOptionsData struct {
	TerritoryName string                   `json:"territory_name"`
	Options       typedef.TerritoryOptions `json:"options"`
}

type SetHQData struct {
	TerritoryName string `json:"territory_name"`
}

type ModifyStorageData struct {
	TerritoryName string                 `json:"territory_name"`
	NewState      typedef.BasicResources `json:"new_state"`
}

type SetTickRateData struct {
	TicksPerSecond int `json:"ticks_per_second"`
}

type LoadStateData struct {
	Filepath string `json:"filepath"`
}

type SaveStateData struct {
	Filepath string `json:"filepath"`
}

// Territory query message data structures
type GetTerritoryStatsData struct {
	TerritoryName string `json:"territory_name"`
}

// GetAlternativeRoutesData requests alternative routes for a territory.
// Direction can be "return", "bounded", or "both" (default is "return").
type GetAlternativeRoutesData struct {
	TerritoryName string `json:"territory_name"`
	Direction     string `json:"direction,omitempty"`
}

// AlternativeRouteInfo describes a single alternative route.
type AlternativeRouteInfo struct {
	ID    int      `json:"id"`
	Route []string `json:"route"`
}

// AlternativeRoutesResponse returns alternative routes and selected route ids.
type AlternativeRoutesResponse struct {
	TerritoryName      string                `json:"territory_name"`
	Direction          string                `json:"direction"`
	SelectedID         int                   `json:"selected_id,omitempty"`
	Routes             []AlternativeRouteInfo `json:"routes,omitempty"`
	SelectedReturnID   int                   `json:"selected_return_id,omitempty"`
	SelectedBoundedID  int                   `json:"selected_bounded_id,omitempty"`
	ReturnRoutes       []AlternativeRouteInfo `json:"return_routes,omitempty"`
	BoundedRoutes      []AlternativeRouteInfo `json:"bounded_routes,omitempty"`
}

// Territory editing message data structures
type SetTerritoryBonusesData struct {
	TerritoryName string        `json:"territory_name"`
	Bonuses       typedef.Bonus `json:"bonuses"`
}

type SetTerritoryUpgradesData struct {
	TerritoryName string          `json:"territory_name"`
	Upgrades      typedef.Upgrade `json:"upgrades"`
}

type SetTerritoryTaxData struct {
	TerritoryName string               `json:"territory_name"`
	Tax           typedef.TerritoryTax `json:"tax"`
}

type SetTerritoryBorderData struct {
	TerritoryName string         `json:"territory_name"`
	Border        typedef.Border `json:"border"`
}

type SetTerritoryRoutingModeData struct {
	TerritoryName string          `json:"territory_name"`
	RoutingMode   typedef.Routing `json:"routing_mode"`
}

type SetTerritoryTreasuryData struct {
	TerritoryName    string                   `json:"territory_name"`
	TreasuryOverride typedef.TreasuryOverride `json:"treasury_override"`
}

// SetTradingRouteData selects a tiebreak route by ID.
// Direction can be "return", "bounded", or "both" (default is "return").
type SetTradingRouteData struct {
	TerritoryName string `json:"territory_name"`
	RouteID       int    `json:"route_id"`
	Direction     string `json:"direction,omitempty"`
}

// Tribute management data structures
type CreateTributeData struct {
	FromGuildTag    string                 `json:"from_guild_tag"`
	ToGuildTag      string                 `json:"to_guild_tag"`
	AmountPerHour   typedef.BasicResources `json:"amount_per_hour"`
	IntervalMinutes uint32                 `json:"interval_minutes"`
}

type EditTributeData struct {
	TributeID       string                  `json:"tribute_id"`
	AmountPerHour   *typedef.BasicResources `json:"amount_per_hour,omitempty"`  // Optional: update amount
	IntervalMinutes *uint32                 `json:"interval_minutes,omitempty"` // Optional: update interval
}

type TributeActionData struct {
	TributeID string `json:"tribute_id"`
}

type GetTributesData struct {
	GuildTag        string `json:"guild_tag,omitempty"` // Optional: filter by guild
	IncludeActive   bool   `json:"include_active"`      // Include active tributes
	IncludeInactive bool   `json:"include_inactive"`    // Include disabled tributes
}

type GetTributeStatsData struct {
	TributeID string `json:"tribute_id,omitempty"` // Optional: specific tribute, if empty returns all
}

// Guild management data structures
type CreateGuildData struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type DeleteGuildData struct {
	GuildTag string `json:"guild_tag"`
}

type SearchGuildsData struct {
	Query      string `json:"query"`       // Search term (name or tag)
	Exact      bool   `json:"exact"`       // Exact match or partial search
	IncludeTag bool   `json:"include_tag"` // Include tag in search
}

type EditGuildData struct {
	OldTag string `json:"old_tag"`        // Current guild tag
	Name   string `json:"name,omitempty"` // Optional: new name
	Tag    string `json:"tag,omitempty"`  // Optional: new tag
}

type SetGuildAlliesData struct {
	GuildTag string   `json:"guild_tag"`
	AllyTags []string `json:"ally_tags"` // List of ally guild tags
}

// Client options for controlling what data to include in state ticks
type ClientOptions struct {
	IncludeTerritoryStats bool `json:"include_territory_stats"`
	IncludeGuildStats     bool `json:"include_guild_stats"`
	IncludeTributeStats   bool `json:"include_tribute_stats"`
}

// API struct for WebSocket server
type API struct {
	clients       map[*WSClient]bool
	clientOptions map[*WSClient]*ClientOptions
	broadcast     chan WSMessage
	register      chan *WSClient
	unregister    chan *WSClient
	handlers      map[MessageType]MessageHandler
}

// WebSocket client representation
type WSClient struct {
	conn WSConnection
	send chan WSMessage
	api  *API
	id   string
}

// Interface for WebSocket connection (for easier testing)
type WSConnection interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
}

// Message handler function type
type MessageHandler func(*WSClient, WSMessage) error
