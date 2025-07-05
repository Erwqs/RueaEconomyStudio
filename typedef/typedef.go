package typedef

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"sync"
)

var (
	ErrTerritoryNameEmpty = errors.New("territory name cannot be empty")
)

var BaseResourceCapacity = BasicResources{
	Emeralds: 3000, // Base capacity for emeralds
	Ores:     300,  // Base capacity for ores
	Wood:     300,  // Base capacity for wood
	Fish:     300,  // Base capacity for fish
	Crops:    300,  // Base capacity for crops
}

type DefenceLevel uint8

const (
	DefenceLevelVeryLow DefenceLevel = iota
	DefenceLevelLow
	DefenceLevelMedium
	DefenceLevelHigh
	DefenceLevelVeryHigh
)

type TreasuryLevel uint8

const (
	TreasuryLevelVeryLow TreasuryLevel = iota
	TreasuryLevelLow
	TreasuryLevelMedium
	TreasuryLevelHigh
	TreasuryLevelVeryHigh
)

type TreasuryOverride int8

const (
	TreasuryOverrideNone     TreasuryOverride = iota // No override, treasury is calculated based on held time
	TreasuryOverrideVeryLow                          // Override to low treasury level
	TreasuryOverrideLow                              // Override to low treasury level
	TreasuryOverrideMedium                           // Override to medium treasury level
	TreasuryOverrideHigh                             // Override to high treasury level
	TreasuryOverrideVeryHigh                         // Override to very high treasury level
)

type Border int8

const (
	BorderClosed Border = iota // No trading routes to this territory
	BorderOpen                 // Trading routes to this territory are open
)

type Routing int8

const (
	RoutingCheapest Routing = iota // Route with the lowest cost
	RoutingFastest                 // Route with the lowest time
)

type Warning int8

// Bitfields for warnings
const (
	WarningOverflowEmerald Warning = 1 << iota
	WarningOverflowResources
	WarningUsageEmerald
	WarningUsageResources
)

// Struct for SetCh channel, used to set uppgrades, bonuses and more
type TerritoryOptions struct {
	Upgrades    Upgrade
	Bonuses     Bonus
	Tax         TerritoryTax
	RoutingMode Routing // Routing mode for the trading routes, can be RoutingCheapest or RoutingFastest
	Border      Border  // Border status of the territory, can be BorderClosed or BorderOpen
	HQ          bool    // If true, the territory is set as a HQ and other HQ will be unset
}

// Costs represents the costs associated with upgrades and bonuses in the system.
type Costs struct {
	UpgradesCost struct {
		Damage  UpgradeCosts `json:"damage"`
		Attack  UpgradeCosts `json:"attack"`
		Health  UpgradeCosts `json:"health"`
		Defence UpgradeCosts `json:"defence"`
	} `json:"upgradesCost"`
	UpgradeMultiplier struct {
		Damage  []float64 `json:"damage"`
		Attack  []float64 `json:"attack"`
		Health  []float64 `json:"health"`
		Defence []float64 `json:"defence"`
	} `json:"upgradeMultiplier"`
	UpgradeBaseStats struct {
		Damage  MinMax    `json:"damage"`
		Attack  []float64 `json:"attack"`
		Health  []int     `json:"health"`
		Defence []float64 `json:"defence"`
	} `json:"upgradeBaseStats"`
	Bonuses struct {
		StrongerMinions       BonusCosts `json:"strongerMinions"`
		TowerMultiAttack      BonusCosts `json:"towerMultiAttack"`
		TowerAura             BonusCosts `json:"towerAura"`
		TowerVolley           BonusCosts `json:"towerVolley"`
		XpSeeking             BonusCosts `json:"xpSeeking"`
		TomeSeeking           BonusCosts `json:"tomeSeeking"`
		EmeraldsSeeking       BonusCosts `json:"emeraldsSeeking"`
		LargerResourceStorage BonusCosts `json:"largerResourceStorage"`
		LargerEmeraldsStorage BonusCosts `json:"largerEmeraldsStorage"`
		EfficientResource     BonusCosts `json:"efficientResource"`
		EfficientEmeralds     BonusCosts `json:"efficientEmeralds"`
		ResourceRate          BonusCosts `json:"resourceRate"`
		EmeraldsRate          BonusCosts `json:"emeraldsRate"`
	} `json:"bonuses"`
}

type Guild struct {
	// TODO: Define guild structure with allies
	Name       string         `json:"Name"`       // Guild name
	Tag        string         `json:"Tag"`        // Guild tag
	Allies     []*Guild       `json:"Allies"`     // List of allied guilds
	TributeIn  BasicResources `json:"TributeIn"`  // Resources received from tributes (per hour)
	TributeOut BasicResources `json:"TributeOut"` // Resources sent as tributes (per hour)
}

type BasicResourcesInterface interface {
	Add(b BasicResourcesInterface) BasicResources
	Sub(b BasicResourcesInterface, clamp ...bool) BasicResources
	MulFloat(a float64) BasicResources
	MulResource(b BasicResourcesInterface) BasicResources
	DivFloat(a float64) BasicResources
	PerSecond() BasicResourcesSecond
	PerHour() BasicResources
	IsSecond() bool
}

type BasicResourcesSecond struct {
	Emeralds float64 `json:"EmeraldsPerSecond"`
	Ores     float64 `json:"OresPerSecond"`
	Wood     float64 `json:"WoodPerSecond"`
	Fish     float64 `json:"FishPerSecond"`
	Crops    float64 `json:"CropsPerSecond"`
}

func (brs *BasicResourcesSecond) IsSecond() bool {
	return true
}

func (brs *BasicResourcesSecond) PerSecond() BasicResourcesSecond {
	// Return itself as it is already per second
	return *brs
}

func (brs *BasicResourcesSecond) PerHour() BasicResources {
	return BasicResources{
		Emeralds: brs.Emeralds * 3600,
		Ores:     brs.Ores * 3600,
		Wood:     brs.Wood * 3600,
		Fish:     brs.Fish * 3600,
		Crops:    brs.Crops * 3600,
	}
}

func (brs *BasicResourcesSecond) Add(b BasicResourcesInterface) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds + b.PerSecond().Emeralds,
		Ores:     brs.Ores + b.PerSecond().Ores,
		Wood:     brs.Wood + b.PerSecond().Wood,
		Fish:     brs.Fish + b.PerSecond().Fish,
		Crops:    brs.Crops + b.PerSecond().Crops,
	}
}

func (brs *BasicResourcesSecond) Sub(b BasicResourcesInterface, clamp ...bool) BasicResourcesSecond {
	if len(clamp) > 0 && clamp[0] {
		return BasicResourcesSecond{
			Emeralds: max(0, brs.Emeralds-b.PerSecond().Emeralds),
			Ores:     max(0, brs.Ores-b.PerSecond().Ores),
			Wood:     max(0, brs.Wood-b.PerSecond().Wood),
			Fish:     max(0, brs.Fish-b.PerSecond().Fish),
			Crops:    max(0, brs.Crops-b.PerSecond().Crops),
		}
	}

	return BasicResourcesSecond{
		Emeralds: brs.Emeralds - b.PerSecond().Emeralds,
		Ores:     brs.Ores - b.PerSecond().Ores,
		Wood:     brs.Wood - b.PerSecond().Wood,
		Fish:     brs.Fish - b.PerSecond().Fish,
		Crops:    brs.Crops - b.PerSecond().Crops,
	}
}

func (brs *BasicResourcesSecond) MulFloat(a float64) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds * a,
		Ores:     brs.Ores * a,
		Wood:     brs.Wood * a,
		Fish:     brs.Fish * a,
		Crops:    brs.Crops * a,
	}
}

func (brs *BasicResourcesSecond) MulResource(b BasicResourcesInterface) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds * b.PerSecond().Emeralds,
		Ores:     brs.Ores * b.PerSecond().Ores,
		Wood:     brs.Wood * b.PerSecond().Wood,
		Fish:     brs.Fish * b.PerSecond().Fish,
		Crops:    brs.Crops * b.PerSecond().Crops,
	}
}

func (brs *BasicResourcesSecond) DivFloat(a float64) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds / a,
		Ores:     brs.Ores / a,
		Wood:     brs.Wood / a,
		Fish:     brs.Fish / a,
		Crops:    brs.Crops / a,
	}
}

type BasicResources struct {
	Emeralds float64 `etf:"tce" json:"emeralds"`
	Ores     float64 `etf:"tco" json:"ores"`
	Wood     float64 `etf:"tcw" json:"wood"`
	Fish     float64 `etf:"tcf" json:"fish"`
	Crops    float64 `etf:"tcc" json:"crops"`
}

func (br *BasicResources) IsSecond() bool {
	return false
}

func (br *BasicResources) PerSecond() BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: br.Emeralds / 3600,
		Ores:     br.Ores / 3600,
		Wood:     br.Wood / 3600,
		Fish:     br.Fish / 3600,
		Crops:    br.Crops / 3600,
	}
}

func (br *BasicResources) PerHour() BasicResources {
	// BasicResources is already per hour, so return itself
	return *br
}

func (br *BasicResources) Add(b BasicResourcesInterface) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds + b.PerHour().Emeralds,
		Ores:     br.Ores + b.PerHour().Ores,
		Wood:     br.Wood + b.PerHour().Wood,
		Fish:     br.Fish + b.PerHour().Fish,
		Crops:    br.Crops + b.PerHour().Crops,
	}
}

func (br *BasicResources) Sub(b BasicResourcesInterface, clamp ...bool) BasicResources {
	if len(clamp) > 0 && clamp[0] {
		return BasicResources{
			Emeralds: max(0, br.Emeralds-b.PerHour().Emeralds),
			Ores:     max(0, br.Ores-b.PerHour().Ores),
			Wood:     max(0, br.Wood-b.PerHour().Wood),
			Fish:     max(0, br.Fish-b.PerHour().Fish),
			Crops:    max(0, br.Crops-b.PerHour().Crops),
		}
	}

	return BasicResources{
		Emeralds: br.Emeralds - b.PerHour().Emeralds,
		Ores:     br.Ores - b.PerHour().Ores,
		Wood:     br.Wood - b.PerHour().Wood,
		Fish:     br.Fish - b.PerHour().Fish,
		Crops:    br.Crops - b.PerHour().Crops,
	}
}

func (br *BasicResources) MulFloat(a float64) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds * a,
		Ores:     br.Ores * a,
		Wood:     br.Wood * a,
		Fish:     br.Fish * a,
		Crops:    br.Crops * a,
	}
}

func (br *BasicResources) MulResource(b BasicResourcesInterface) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds * b.PerHour().Emeralds,
		Ores:     br.Ores * b.PerHour().Ores,
		Wood:     br.Wood * b.PerHour().Wood,
		Fish:     br.Fish * b.PerHour().Fish,
		Crops:    br.Crops * b.PerHour().Crops,
	}
}

func (br *BasicResources) DivFloat(a float64) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds / a,
		Ores:     br.Ores / a,
		Wood:     br.Wood / a,
		Fish:     br.Fish / a,
		Crops:    br.Crops / a,
	}
}

type BasicResourcesBroken struct {
	Emeralds string `json:"emeralds"`
	Ores     string `json:"ore"`
	Wood     string `json:"wood"`
	Fish     string `json:"fish"`
	Crops    string `json:"crops"`
}

func (br *BasicResourcesBroken) Cast() BasicResources {
	emeralds, _ := strconv.ParseFloat(br.Emeralds, 64)
	ores, _ := strconv.ParseFloat(br.Ores, 64)
	wood, _ := strconv.ParseFloat(br.Wood, 64)
	fish, _ := strconv.ParseFloat(br.Fish, 64)
	crops, _ := strconv.ParseFloat(br.Crops, 64)

	return BasicResources{
		Emeralds: emeralds,
		Ores:     ores,
		Wood:     wood,
		Fish:     fish,
		Crops:    crops,
	}
}

// UpgradeCosts represents the cost structure for upgrades in the system.
type UpgradeCosts struct {
	Value        []int  `json:"value"`
	ResourceType string `json:"resourceType"`
}

// BonusCosts represents the cost structure for bonuses in the system.
type BonusCosts struct {
	MaxLevel     int       `json:"maxLevel"`
	Cost         []int     `json:"cost"`
	ResourceType string    `json:"resourceType"`
	Value        []float64 `json:"value"`
}

// MinMax represents a range with minimum and maximum values.
type MinMax struct {
	Min []int `json:"min"`
	Max []int `json:"max"`
}

// TerritoryGuild represents the guild information associated with a territory.
type TerritoryGuild struct {
	GuildName string `json:"GuildName"`
	GuildTag  string `json:"GuildTag"`
}

// TerritoryTower represents the tower upgrade and bonus of a territory
type TerritoryTower struct {
	Upgrade TerritoryUpgrade `json:"Upgrade"`
	Bonus   TerritoryBonus   `json:"Bonus"`
}

// TerritoryUpgrade represents the upgrade information for a territory's tower.
// It contains both the set upgrade and the at upgrade.
// Set represents the upgrades that are applied by user while At represents the upgrades that are currently active due to inefficient resource or emeralds.
type TerritoryUpgrade struct {
	Set Upgrade `json:"SetUpgrade"`
	At  Upgrade `json:"AtUpgrade"`
}

// TerritoryBonus represents the bonus information for a territory's tower.
// It contains both the set bonus and the at bonus.
// Set represents the bonuses that are applied by user while At represents the bonuses that are currently active due to inefficient resource or emeralds.
type TerritoryBonus struct {
	Set Bonus `json:"SetBonus"`
	At  Bonus `json:"AtBonus"`
}

// Upgrade represents the upgrade levels for a territory's tower.
type Upgrade struct {
	Damage  int `json:"DamageLevel"`
	Attack  int `json:"AttackLevel"`
	Health  int `json:"HealthLevel"`
	Defence int `json:"DefenceLevel"`
}

// Bonus represents the bonus levels for a territory's tower.
type Bonus struct {
	StrongerMinions       int `json:"StrongerMinionsLevel"`
	TowerMultiAttack      int `json:"TowerMultiAttackLevel"`
	TowerAura             int `json:"TowerAuraLevel"`
	TowerVolley           int `json:"TowerVolleyLevel"`
	XpSeeking             int `json:"XpSeekingLevel"`
	TomeSeeking           int `json:"TomeSeekingLevel"`
	EmeraldSeeking        int `json:"EmeraldSeekingLevel"`
	LargerResourceStorage int `json:"LargerResourceStorageLevel"`
	LargerEmeraldStorage  int `json:"LargerEmeraldStorageLevel"`
	EfficientResource     int `json:"EfficientResourceLevel"`
	EfficientEmerald      int `json:"EfficientEmeraldLevel"`
	ResourceRate          int `json:"ResourceRateLevel"`
	EmeraldRate           int `json:"EmeraldRateLevel"`
}

// Tax represents the tax set on the territory.
type TerritoryTax struct {
	Tax  float64 `json:"TaxRate"`     // Tax for non allied guilds, from 0.0 to 1.0
	Ally float64 `json:"AllyTaxRate"` // Tax for allies, from 0.0 to 1.0
}

type TerritoryStorage struct {
	Capacity BasicResources `json:"StorageCapacity"`  // Total capacity of the storage, calculated on the fly, not serialized
	At       BasicResources `json:"CurrentResources"` // Current amount of resources in the storage, calculated on the fly, not serialized
}

type InTransitResources struct {
	BasicResources `json:"Resources"` // Resources in transit
	Origin         *Territory         `json:"-"`                // Origin territory of the resources in transit
	Destination    *Territory         `json:"-"`                // Destination territory of the resources in transit
	Next           *Territory         `json:"-"`                // Next territory in the trading route, can be nil if this is the last territory in the route
	NextTax        float64            `json:"NextTerritoryTax"` // Tax for the next territory in the trading route, from 0.0 to 1.0, -1.0 for invalid tax due to no route or the territory is HQ
	Route          []*Territory       `json:"-"`                // Full route from origin to destination
	RouteIndex     int                `json:"RouteIndex"`       // Current position in the route (index of the territory that currently has this resource)

	// JSON safe fields
	OriginID      string   `json:"OriginID"`      // ID of the origin territory,
	DestinationID string   `json:"DestinationID"` // ID of the destination territory
	NextID        string   `json:"NextID"`        // ID of the next territory in
	Route2        []string `json:"Route"`         // IDs of the territories in the route

	Moved bool `json:"HasMoved"` // Ensure resouce gets moved once
}

type ResourceGeneration struct {
	Base              BasicResources `json:"BaseGeneration"`    // Base resource generation, calculated on the fly, not serialized
	At                BasicResources `json:"CurrentGeneration"` // Current resource generation, calculated on the fly, not serialized
	ResourceDeltaTime uint8          `json:"ResourceDeltaTime"` // Modulo for the resource generation, used to calculate the next generation time
	EmeraldDeltaTime  uint8          `json:"EmeraldDeltaTime"`  // Modulo for the emerald generation, used to calculate the next generation time

	// Accumulators for resources that build up before being released
	ResourceAccumulator BasicResourcesSecond `json:"-"` // Accumulates resources per second until released, not serialized
	EmeraldAccumulator  float64              `json:"-"` // Accumulates emeralds per second until released, not serialized

	// Tracking for when to release accumulated resources
	LastResourceTick uint64 `json:"-"` // Last tick when resources were generated and released, not serialized
	LastEmeraldTick  uint64 `json:"-"` // Last tick when emeralds were generated and released, not serialized
}

type TowerStats struct {
	Damage  DamageRange `json:"DamageRange"` // Damage dealt by the tower
	Attack  float64     `json:"AttackSpeed"` // Attack speed of the tower
	Health  float64     `json:"Health"`      // Health of the tower
	Defence float64     `json:"Defence"`     // Defence of the tower
}

type DamageRange struct {
	Low  float64 `json:"LowDamage"`
	High float64 `json:"HighDamage"`
}

type Links struct {
	Direct    map[string]struct{} `json:"DirectConnections"`   // Adjacents territories
	Externals map[string]struct{} `json:"ExternalConnections"` // External territories, connections are considered as externals
}

// Territory represents a territory in the system.
type Territory struct {
	ID       string         `json:"ID"`
	Name     string         `json:"Name"`
	Guild    Guild          `json:"Guild"`
	Location LocationObject `json:"Location"`

	Options TerritoryTower `json:"TowerOptions"`

	Costs BasicResources `json:"-"` // Calculate on the fly, not serialized
	Net   BasicResources `json:"-"` // Calculate on the fly, not serialized

	TowerStats  TowerStats   `json:"-"`           // Tower stats, calculated on the fly, not serialized
	Level       DefenceLevel `json:"-"`           // Calculate on the fly, not serialized
	LevelInt    uint8        `json:"-"`           // Calculate on the fly, not serialized
	SetLevelInt uint8        `json:"SetLevelInt"` // Set level of the territory, used for serialization
	SetLevel    DefenceLevel `json:"SetLevel"`    // Set level of the territory, used for serialization

	Links Links `json:"Connections"` // Connections territories and externals

	ResourceGeneration ResourceGeneration `json:"ResourceGeneration"` // Resource generation, calculated on the fly, not serialized

	// Treasury level
	Treasury         TreasuryLevel    `json:"TreasuryLevel"`
	TreasuryOverride TreasuryOverride `json:"TreasuryOverride"` // Override for the treasury level, can be TreasuryOverrideNone, TreasuryOverrideVeryLow, TreasuryOverrideLow, TreasuryOverrideMedium, TreasuryOverrideHigh, or TreasuryOverrideVeryHigh
	GenerationBonus  float64          `json:"GenerationBonus"`  // Generation bonus in %
	CapturedAt       uint64           `json:"CapturedAt"`       // State tick when the territory was captured, used for calculating treasury

	// List of trading routes from this territory to HQ
	// Can be nil if territory is owned by No Guild [NONE]
	// Or there's no route to the destination
	// HQ can have many routes to territories, but only one route from teritories to HQ
	// TradingRoutes is a slice of slices, where each inner slice represents a route from this territory to the HQ
	ConnectedTerritories []string       `json:"ConnectedTerritories"` // Territories connected to this territory, used for trading routes
	TradingRoutes        [][]*Territory `json:"-"`
	TradingRoutesJSON    [][]string     `json:"TradingRoutes"` // Serialized trading routes, used for GUI and other purposes
	RouteTax             float64        `json:"RouteTax"`      // Tax for the trading routes, from 0.0 to 1.0, -1.0 for invalid tax due to no HQ, no route, or the territory is HQ
	RoutingMode          Routing        `json:"RoutingMode"`   // Routing mode for the trading routes, can be RoutingCheapest or RoutingFastest
	Border               Border         `json:"BorderControl"` // Border status of the territory, can be BorderClosed or BorderOpen

	// Tax set on the territory
	Tax TerritoryTax `json:"Tax"`

	HQ bool `json:"IsHQ"` // If true, the territory is set as a HQ and other HQ will be unset

	// Next territory in the trading route, can be nil if this is the last territory in the route
	NextTerritory *Territory `json:"-"`

	// Destination territory for the trading route, can be nil if this is the last territory in the route, territory owned by No Guild or there's no route to the destination
	Destination *Territory `json:"-"`

	// Storage of the territory, calculated on the fly, not serialized
	Storage TerritoryStorage `json:"Storage"`

	// TransitResource represents the resources in transit from one territory to another going through this territory
	TransitResource []InTransitResources `json:"TransitResources"`

	// Channel to set upgrades, bonuses and more, sent from GUI or other sources
	SetCh   chan<- TerritoryOptions `json:"-"`
	CloseCh func()                  `json:"-"` // Function to close the channel, used to clean up resources when the territory is no longer needed
	Reset   func()                  `json:"-"` // Function to reset the territory, used to reset the territory to its initial state

	Warning Warning `json:"ActiveWarnings"` // Warnings for the territory, can be WarningOverflowEmerald or WarningOverflowResources

	// Really important mutex to protect the territory from concurrent access
	Mu sync.RWMutex
}

// NewTerritory creates a new Territory instance from the given TerritoryJSON.
func NewTerritory(name string, t TerritoryJSON) (*Territory, error) {
	// Convert BasicResourcesBroken to BasicResources
	resources := t.Resources.Cast()

	// Create a channel for territory options
	setCh := make(chan TerritoryOptions, 1)

	// Create the territory instance
	territory := &Territory{
		ID:   generateUUID(name), // Will be set externally when territory name is known
		Name: name,               // Will be set externally
		Guild: Guild{
			Name: "No Guild",
			Tag:  "NONE",
		},
		Location: t.Location,
		Options: TerritoryTower{
			Upgrade: TerritoryUpgrade{
				Set: Upgrade{},
				At:  Upgrade{},
			},
			Bonus: TerritoryBonus{
				Set: Bonus{},
				At:  Bonus{},
			},
		},
		Costs:    BasicResources{},
		Level:    DefenceLevelVeryLow,
		LevelInt: 0,
		ResourceGeneration: ResourceGeneration{
			Base:                resources,
			At:                  resources,
			ResourceDeltaTime:   4, // Default to 4 seconds
			EmeraldDeltaTime:    4, // Default to 4 seconds
			ResourceAccumulator: BasicResourcesSecond{},
			EmeraldAccumulator:  0,
			LastResourceTick:    0, // Will be set when first generation calculation happens
			LastEmeraldTick:     0, // Will be set when first generation calculation happens
		},
		Treasury:             TreasuryLevelVeryLow,
		GenerationBonus:      0.0,
		ConnectedTerritories: t.TradingRoutes,
		TradingRoutes:        make([][]*Territory, 0),
		RoutingMode:          RoutingCheapest,
		Border:               BorderOpen,
		Tax: TerritoryTax{
			Tax:  0.05, // 5% as decimal (0.05)
			Ally: 0.05, // 5% as decimal (0.05)
		},
		RouteTax:      -1.0, // Invalid tax by default, will be set when the route is calculated
		HQ:            false,
		NextTerritory: nil,
		Destination:   nil,
		Storage: TerritoryStorage{
			Capacity: BaseResourceCapacity, // Initialize with base resource capacity
			At:       BasicResources{},     // Initialize with the resources from JSON
		},
		TransitResource: make([]InTransitResources, 0), // Initialize with 0 capacity (old system, being phased out)
		SetCh:           setCh,
		CloseCh: func() {
			close(setCh)
		},
		Reset: func() {
			// TODO: reset sets everything to its initial state

		},
		Warning: 0, // No warnings initially
		Mu:      sync.RWMutex{},
	}

	return territory, nil
}

// Generate UUID guaranteed to be unique for each territory but a territory will have the same UUID each time it is loaded
func generateUUID(territory string) string {
	// Use SHA-256 to create a deterministic hash from the territory name
	hash := sha256.Sum256([]byte(territory))

	// Convert the first 16 bytes of the hash to a UUID-like format
	hashBytes := hash[:16]

	// Format as UUID: 8-4-4-4-12 hex digits separated by hyphens
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		hashBytes[0:4],
		hashBytes[4:6],
		hashBytes[6:8],
		hashBytes[8:10],
		hashBytes[10:16])
}

type TerritoriesFileJSON map[string]TerritoryJSON

type TerritoryJSON struct {
	Resources     BasicResourcesBroken `json:"resources"`
	TradingRoutes []string             `json:"Trading Routes"`
	Location      LocationObject       `json:"Location"`
}

type LocationObject struct {
	Start [2]int `json:"start"`
	End   [2]int `json:"end"` // End coordinates of the territory
}

type GuildsFileJSON []GuildJSON

type GuildJSON struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type ActiveTribute struct {
	ID              string         `json:"ID"`              // Unique identifier for the tribute
	From            *Guild         `json:"-"`               // Can be nil if the tribute is spawned in (not from any guild but gets added to HQ storage periodically)
	FromGuildName   string         `json:"FromGuild"`       // Guild name of the tribute sender
	To              *Guild         `json:"-"`               // Can be nil if the tribute is spawned in (not to any guild but gets removed HQ storage periodically)
	ToGuildName     string         `json:"ToGuild"`         // Guild name of the tribute receiver
	AmountPerHour   BasicResources `json:"AmountPerHour"`   // Amount of resources per hour (user input value)
	AmountPerMinute BasicResources `json:"AmountPerMinute"` // Amount of resources per minute (calculated from AmountPerHour)
	IntervalMinutes uint32         `json:"IntervalMinutes"` // How often the tribute transfers (in minutes, aligned with 60-tick cycles)
	LastTransfer    uint64         `json:"LastTransfer"`    // Tick when the last transfer happened (0 if never transferred)
	IsActive        bool           `json:"IsActive"`        // Whether this tribute is currently active
	CreatedAt       uint64         `json:"CreatedAt"`       // Tick when this tribute was created
}

// EventAction represents the type of action that can be performed in an event.
type EventAction int8

const (
	EventActionGuildChange EventAction = iota // Guild capture, transfer, or loss
	EventActionSetHQ
	EventActionTerritoryQueue // Territory queue event, used for locking a territory to prevent HQ migration
)

// EventSequence represents a sequence of events in the game.
type EventSequence []Event

// Event represents a single event, programmable by the user or game's RNG
type Event struct {
	// Basic event information
	ID          string `json:"ID"`          // Unique identifier for the event
	Name        string `json:"Name"`        // Name of the event
	Description string `json:"Description"` // Description of the event

	// Event operations.
	// Where and when
	DeltaTime     uint64 `json:"DeltaTime"` // Delta time in milliseconds from the last event
	TerritoryName string `json:"TerritoryName"`

	// Which guild took the territory
	GuildName string `json:"GuildName"` // Guild that took the territory, can be nil if the event is not related to a guild
	GuildTag  string `json:"GuildTag"`  // Guild tag that took the territory, can be nil if the event is not related to a guild

	Action EventAction `json:"Action"` // Action to perform, can be EventActionGuildChange or EventActionSetHQ
}

type Loadout struct {
	Name string `json:"Name"` // Name of the loadout
	TerritoryOptions
}

// Tracks user options and settings
type RuntimeOptions struct {
	TreasuryEnabled bool `json:"TreasuryEnabled"` // If true, the treasury is enabled and will be used for resource generation and storage

	// NoKSPrompt indicates whether user will be prompted with available keyboard shortcuts upon starting the app
	NoKSPrompt bool `json:"NoKSPrompt"` // If true, the user will not be prompted to set the KS (Kingdom Status) when starting the game

	// EnableShm indicates whether the user wants to enable the SHM (Shared Memory) feature
	EnableShm bool `json:"EnableShm"` //

	EncodeInTransitResources bool `json:"EncodeTreasury"` // If true, the treasury will be encoded in the JSON format for easier storage and retrieval
}
