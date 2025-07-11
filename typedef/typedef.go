package typedef

import (
	"crypto/sha256"
	"errors"
	"etools/numbers"
	"fmt"
	"strconv"
	"sync"
)

var (
	ErrTerritoryNameEmpty = errors.New("territory name cannot be empty")
)

var BaseResourceCapacity = BasicResources{
	Emeralds: numbers.NewFixedPoint(3000, 0),
	Ores:     numbers.NewFixedPoint(300, 0),
	Wood:     numbers.NewFixedPoint(300, 0),
	Fish:     numbers.NewFixedPoint(300, 0),
	Crops:    numbers.NewFixedPoint(300, 0),
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
		GatheringExperience   BonusCosts `json:"gatheringExperience"`
		MobExperience         BonusCosts `json:"mobExperience"`
		MobDamage             BonusCosts `json:"mobDamage"`
		PvPDamage             BonusCosts `json:"pvpDamage"`
		XPSeeking             BonusCosts `json:"xpSeeking"`
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
	MulFloat(a numbers.FixedPoint128) BasicResources
	MulResource(b BasicResourcesInterface) BasicResources
	DivFloat(a numbers.FixedPoint128) BasicResources
	PerSecond() BasicResourcesSecond
	PerHour() BasicResources
	IsSecond() bool
}

type BasicResourcesSecond struct {
	Emeralds numbers.FixedPoint128 `json:"EmeraldsPerSecond"`
	Ores     numbers.FixedPoint128 `json:"OresPerSecond"`
	Wood     numbers.FixedPoint128 `json:"WoodPerSecond"`
	Fish     numbers.FixedPoint128 `json:"FishPerSecond"`
	Crops    numbers.FixedPoint128 `json:"CropsPerSecond"`
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
		Emeralds: brs.Emeralds.Multiply(numbers.NewFixedPoint(3600, 0)),
		Ores:     brs.Ores.Multiply(numbers.NewFixedPoint(3600, 0)),
		Wood:     brs.Wood.Multiply(numbers.NewFixedPoint(3600, 0)),
		Fish:     brs.Fish.Multiply(numbers.NewFixedPoint(3600, 0)),
		Crops:    brs.Crops.Multiply(numbers.NewFixedPoint(3600, 0)),
	}
}

func (brs *BasicResourcesSecond) Add(b BasicResourcesInterface) BasicResourcesSecond {
	other := b.PerSecond()
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds.Add(other.Emeralds),
		Ores:     brs.Ores.Add(other.Ores),
		Wood:     brs.Wood.Add(other.Wood),
		Fish:     brs.Fish.Add(other.Fish),
		Crops:    brs.Crops.Add(other.Crops),
	}
}

func (brs *BasicResourcesSecond) Sub(b BasicResourcesInterface, clamp ...bool) BasicResourcesSecond {
	other := b.PerSecond()
	if len(clamp) > 0 && clamp[0] {
		return BasicResourcesSecond{
			Emeralds: maxFP(numbers.NewFixedPoint(0, 0), brs.Emeralds.Subtract(other.Emeralds)),
			Ores:     maxFP(numbers.NewFixedPoint(0, 0), brs.Ores.Subtract(other.Ores)),
			Wood:     maxFP(numbers.NewFixedPoint(0, 0), brs.Wood.Subtract(other.Wood)),
			Fish:     maxFP(numbers.NewFixedPoint(0, 0), brs.Fish.Subtract(other.Fish)),
			Crops:    maxFP(numbers.NewFixedPoint(0, 0), brs.Crops.Subtract(other.Crops)),
		}
	}
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds.Subtract(other.Emeralds),
		Ores:     brs.Ores.Subtract(other.Ores),
		Wood:     brs.Wood.Subtract(other.Wood),
		Fish:     brs.Fish.Subtract(other.Fish),
		Crops:    brs.Crops.Subtract(other.Crops),
	}
}

func (brs *BasicResourcesSecond) MulFloat(a numbers.FixedPoint128) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds.Multiply(a),
		Ores:     brs.Ores.Multiply(a),
		Wood:     brs.Wood.Multiply(a),
		Fish:     brs.Fish.Multiply(a),
		Crops:    brs.Crops.Multiply(a),
	}
}

func (brs *BasicResourcesSecond) MulResource(b BasicResourcesInterface) BasicResourcesSecond {
	other := b.PerSecond()
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds.Multiply(other.Emeralds),
		Ores:     brs.Ores.Multiply(other.Ores),
		Wood:     brs.Wood.Multiply(other.Wood),
		Fish:     brs.Fish.Multiply(other.Fish),
		Crops:    brs.Crops.Multiply(other.Crops),
	}
}

func (brs *BasicResourcesSecond) DivFloat(a numbers.FixedPoint128) BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: brs.Emeralds.Divide(a),
		Ores:     brs.Ores.Divide(a),
		Wood:     brs.Wood.Divide(a),
		Fish:     brs.Fish.Divide(a),
		Crops:    brs.Crops.Divide(a),
	}
}

type BasicResources struct {
	Emeralds numbers.FixedPoint128 `etf:"tce" json:"emeralds"`
	Ores     numbers.FixedPoint128 `etf:"tco" json:"ores"`
	Wood     numbers.FixedPoint128 `etf:"tcw" json:"wood"`
	Fish     numbers.FixedPoint128 `etf:"tcf" json:"fish"`
	Crops    numbers.FixedPoint128 `etf:"tcc" json:"crops"`
}

func (br *BasicResources) IsSecond() bool {
	return false
}

func (br *BasicResources) PerSecond() BasicResourcesSecond {
	return BasicResourcesSecond{
		Emeralds: br.Emeralds.Divide(numbers.NewFixedPoint(3600, 0)),
		Ores:     br.Ores.Divide(numbers.NewFixedPoint(3600, 0)),
		Wood:     br.Wood.Divide(numbers.NewFixedPoint(3600, 0)),
		Fish:     br.Fish.Divide(numbers.NewFixedPoint(3600, 0)),
		Crops:    br.Crops.Divide(numbers.NewFixedPoint(3600, 0)),
	}
}

func (br *BasicResources) PerHour() BasicResources {
	// BasicResources is already per hour, so return itself
	return *br
}

func (br *BasicResources) Add(b BasicResourcesInterface) BasicResources {
	other := b.PerHour()
	return BasicResources{
		Emeralds: br.Emeralds.Add(other.Emeralds),
		Ores:     br.Ores.Add(other.Ores),
		Wood:     br.Wood.Add(other.Wood),
		Fish:     br.Fish.Add(other.Fish),
		Crops:    br.Crops.Add(other.Crops),
	}
}

func (br *BasicResources) Sub(b BasicResourcesInterface, clamp ...bool) BasicResources {
	other := b.PerHour()
	if len(clamp) > 0 && clamp[0] {
		return BasicResources{
			Emeralds: maxFP(numbers.NewFixedPoint(0, 0), br.Emeralds.Subtract(other.Emeralds)),
			Ores:     maxFP(numbers.NewFixedPoint(0, 0), br.Ores.Subtract(other.Ores)),
			Wood:     maxFP(numbers.NewFixedPoint(0, 0), br.Wood.Subtract(other.Wood)),
			Fish:     maxFP(numbers.NewFixedPoint(0, 0), br.Fish.Subtract(other.Fish)),
			Crops:    maxFP(numbers.NewFixedPoint(0, 0), br.Crops.Subtract(other.Crops)),
		}
	}
	return BasicResources{
		Emeralds: br.Emeralds.Subtract(other.Emeralds),
		Ores:     br.Ores.Subtract(other.Ores),
		Wood:     br.Wood.Subtract(other.Wood),
		Fish:     br.Fish.Subtract(other.Fish),
		Crops:    br.Crops.Subtract(other.Crops),
	}
}

func (br *BasicResources) MulFloat(a numbers.FixedPoint128) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds.Multiply(a),
		Ores:     br.Ores.Multiply(a),
		Wood:     br.Wood.Multiply(a),
		Fish:     br.Fish.Multiply(a),
		Crops:    br.Crops.Multiply(a),
	}
}

func (br *BasicResources) MulResource(b BasicResourcesInterface) BasicResources {
	other := b.PerHour()
	return BasicResources{
		Emeralds: br.Emeralds.Multiply(other.Emeralds),
		Ores:     br.Ores.Multiply(other.Ores),
		Wood:     br.Wood.Multiply(other.Wood),
		Fish:     br.Fish.Multiply(other.Fish),
		Crops:    br.Crops.Multiply(other.Crops),
	}
}

func (br *BasicResources) DivFloat(a numbers.FixedPoint128) BasicResources {
	return BasicResources{
		Emeralds: br.Emeralds.Divide(a),
		Ores:     br.Ores.Divide(a),
		Wood:     br.Wood.Divide(a),
		Fish:     br.Fish.Divide(a),
		Crops:    br.Crops.Divide(a),
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
	ore, _ := strconv.ParseFloat(br.Ores, 64)
	wood, _ := strconv.ParseFloat(br.Wood, 64)
	fish, _ := strconv.ParseFloat(br.Fish, 64)
	crops, _ := strconv.ParseFloat(br.Crops, 64)

	return BasicResources{
		Emeralds: numbers.NewFixedPointFromFloat(emeralds),
		Ores:     numbers.NewFixedPointFromFloat(ore),
		Wood:     numbers.NewFixedPointFromFloat(wood),
		Fish:     numbers.NewFixedPointFromFloat(fish),
		Crops:    numbers.NewFixedPointFromFloat(crops),
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
	GatheringExperience   int `json:"GatheringExperienceLevel"`
	MobExperience         int `json:"MobExperienceLevel"`
	MobDamage             int `json:"MobDamageLevel"`
	PvPDamage             int `json:"PvPDamageLevel"`
	XPSeeking             int `json:"XPSeekingLevel"`
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

// maxFP returns the greater of two FixedPoint128 values
func maxFP(a, b numbers.FixedPoint128) numbers.FixedPoint128 {
	if a.Whole > b.Whole || (a.Whole == b.Whole && a.Fraction > b.Fraction) {
		return a
	}
	return b
}

// Comparison helpers for FixedPoint128 (standalone functions)
func FixedPointGreaterThan(a, b numbers.FixedPoint128) bool {
	if a.Whole > b.Whole {
		return true
	}
	if a.Whole == b.Whole && a.Fraction > b.Fraction {
		return true
	}
	return false
}

func FixedPointLessThan(a, b numbers.FixedPoint128) bool {
	if a.Whole < b.Whole {
		return true
	}
	if a.Whole == b.Whole && a.Fraction < b.Fraction {
		return true
	}
	return false
}

func FixedPointGreaterThanOrEqual(a, b numbers.FixedPoint128) bool {
	return !FixedPointLessThan(a, b)
}

func FixedPointLessThanOrEqual(a, b numbers.FixedPoint128) bool {
	return !FixedPointGreaterThan(a, b)
}
