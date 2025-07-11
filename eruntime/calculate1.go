package eruntime

import (
	"etools/numbers"
	"etools/typedef"
	"fmt"
	"math"
)

// Cost calculation constants for better performance
const (
	COST_PER_HOUR_TO_PER_SECOND_OLD = 1.0 / 3600.0
)

var COST_PER_HOUR_TO_PER_SECOND = numbers.FixedPoint128{
	Whole:    277,
	Fraction: 778,
}

var THREE_SIX_HUNDRED = numbers.FixedPoint128{
	Whole:    3600,
	Fraction: 0,
}

// Upgrade type constants for faster comparisons
const (
	UPGRADE_DAMAGE = iota
	UPGRADE_ATTACK
	UPGRADE_HEALTH
	UPGRADE_DEFENCE
)

// Bonus type constants for faster comparisons
const (
	BONUS_STRONGER_MINIONS = iota
	BONUS_TOWER_MULTI_ATTACK
	BONUS_TOWER_AURA
	BONUS_TOWER_VOLLEY
	BONUS_GATHERING_EXPERIENCE
	BONUS_MOB_EXPERIENCE
	BONUS_MOB_DAMAGE
	BONUS_PVP_DAMAGE
	BONUS_XP_SEEKING
	BONUS_TOME_SEEKING
	BONUS_EMERALD_SEEKING
	BONUS_LARGER_RESOURCE_STORAGE
	BONUS_LARGER_EMERALD_STORAGE
	BONUS_EFFICIENT_RESOURCE
	BONUS_EFFICIENT_EMERALD
	BONUS_RESOURCE_RATE
	BONUS_EMERALD_RATE
)

// Pre-computed upgrade type mapping for faster lookups
var upgradeTypeMap = map[string]int{
	"damage":  UPGRADE_DAMAGE,
	"attack":  UPGRADE_ATTACK,
	"health":  UPGRADE_HEALTH,
	"defence": UPGRADE_DEFENCE,
}

// Pre-computed bonus type mapping for faster lookups
var bonusTypeMap = map[string]int{
	"StrongerMinions":       BONUS_STRONGER_MINIONS,
	"TowerMultiAttack":      BONUS_TOWER_MULTI_ATTACK,
	"TowerAura":             BONUS_TOWER_AURA,
	"TowerVolley":           BONUS_TOWER_VOLLEY,
	"GatheringExperience":   BONUS_GATHERING_EXPERIENCE,
	"MobExperience":         BONUS_MOB_EXPERIENCE,
	"MobDamage":             BONUS_MOB_DAMAGE,
	"PvPDamage":             BONUS_PVP_DAMAGE,
	"XPSeeking":             BONUS_XP_SEEKING,
	"TomeSeeking":           BONUS_TOME_SEEKING,
	"EmeraldSeeking":        BONUS_EMERALD_SEEKING,
	"LargerResourceStorage": BONUS_LARGER_RESOURCE_STORAGE,
	"LargerEmeraldStorage":  BONUS_LARGER_EMERALD_STORAGE,
	"EfficientResource":     BONUS_EFFICIENT_RESOURCE,
	"EfficientEmerald":      BONUS_EFFICIENT_EMERALD,
	"ResourceRate":          BONUS_RESOURCE_RATE,
	"EmeraldRate":           BONUS_EMERALD_RATE,
}

// Helper function to check if a bonus level can be afforded
func canAffordBonus(storage typedef.BasicResources, bonusType string, level int) bool {
	bonusID, exists := bonusTypeMap[bonusType]
	if !exists {
		return false
	}

	var cost numbers.FixedPoint128
	var resourceType string

	switch bonusID {
	case BONUS_STRONGER_MINIONS:
		if level >= len(st.costs.Bonuses.StrongerMinions.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.StrongerMinions.Cost[level])
		resourceType = st.costs.Bonuses.StrongerMinions.ResourceType
	case BONUS_TOWER_MULTI_ATTACK:
		if level >= len(st.costs.Bonuses.TowerMultiAttack.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerMultiAttack.Cost[level])
		resourceType = st.costs.Bonuses.TowerMultiAttack.ResourceType
	case BONUS_TOWER_AURA:
		if level >= len(st.costs.Bonuses.TowerAura.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerAura.Cost[level])
		resourceType = st.costs.Bonuses.TowerAura.ResourceType
	case BONUS_TOWER_VOLLEY:
		if level >= len(st.costs.Bonuses.TowerVolley.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerVolley.Cost[level])
		resourceType = st.costs.Bonuses.TowerVolley.ResourceType
	case BONUS_GATHERING_EXPERIENCE:
		if level >= len(st.costs.Bonuses.GatheringExperience.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.GatheringExperience.Cost[level])
		resourceType = st.costs.Bonuses.GatheringExperience.ResourceType
	case BONUS_MOB_EXPERIENCE:
		if level >= len(st.costs.Bonuses.MobExperience.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.MobExperience.Cost[level])
		resourceType = st.costs.Bonuses.MobExperience.ResourceType
	case BONUS_MOB_DAMAGE:
		if level >= len(st.costs.Bonuses.MobDamage.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.MobDamage.Cost[level])
		resourceType = st.costs.Bonuses.MobDamage.ResourceType
	case BONUS_PVP_DAMAGE:
		if level >= len(st.costs.Bonuses.PvPDamage.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.PvPDamage.Cost[level])
		resourceType = st.costs.Bonuses.PvPDamage.ResourceType
	case BONUS_XP_SEEKING:
		if level >= len(st.costs.Bonuses.XPSeeking.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.XPSeeking.Cost[level])
		resourceType = st.costs.Bonuses.XPSeeking.ResourceType
	case BONUS_TOME_SEEKING:
		if level >= len(st.costs.Bonuses.TomeSeeking.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.TomeSeeking.Cost[level])
		resourceType = st.costs.Bonuses.TomeSeeking.ResourceType
	case BONUS_EMERALD_SEEKING:
		if level >= len(st.costs.Bonuses.EmeraldsSeeking.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.EmeraldsSeeking.Cost[level])
		resourceType = st.costs.Bonuses.EmeraldsSeeking.ResourceType
	case BONUS_LARGER_RESOURCE_STORAGE:
		if level >= len(st.costs.Bonuses.LargerResourceStorage.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.LargerResourceStorage.Cost[level])
		resourceType = st.costs.Bonuses.LargerResourceStorage.ResourceType
	case BONUS_LARGER_EMERALD_STORAGE:
		if level >= len(st.costs.Bonuses.LargerEmeraldsStorage.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.LargerEmeraldsStorage.Cost[level])
		resourceType = st.costs.Bonuses.LargerEmeraldsStorage.ResourceType
	case BONUS_EFFICIENT_RESOURCE:
		if level >= len(st.costs.Bonuses.EfficientResource.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.EfficientResource.Cost[level])
		resourceType = st.costs.Bonuses.EfficientResource.ResourceType
	case BONUS_EFFICIENT_EMERALD:
		if level >= len(st.costs.Bonuses.EfficientEmeralds.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.EfficientEmeralds.Cost[level])
		resourceType = st.costs.Bonuses.EfficientEmeralds.ResourceType
	case BONUS_RESOURCE_RATE:
		if level >= len(st.costs.Bonuses.ResourceRate.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.ResourceRate.Cost[level])
		resourceType = st.costs.Bonuses.ResourceRate.ResourceType
	case BONUS_EMERALD_RATE:
		if level >= len(st.costs.Bonuses.EmeraldsRate.Cost) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.Bonuses.EmeraldsRate.Cost[level])
		resourceType = st.costs.Bonuses.EmeraldsRate.ResourceType
	default:
		return false
	}

	// Pre-compute per second cost
	costPerSec := cost

	switch resourceType {
	case "emeralds":
		return storage.Emeralds.GreaterThanOrEqual(costPerSec)
	case "ores", "ore":
		return storage.Ores.GreaterThanOrEqual(costPerSec)
	case "wood":
		return storage.Wood.GreaterThanOrEqual(costPerSec)
	case "fish":
		return storage.Fish.GreaterThanOrEqual(costPerSec)
	case "crops":
		return storage.Crops.GreaterThanOrEqual(costPerSec)
	default:
		return false
	}
}

// Helper function to set bonus levels based on affordability
func setAffordableBonuses(territory *typedef.Territory, storage typedef.BasicResources) {
	bonuses := []struct {
		name string
		set  int
		at   *int
	}{
		{"StrongerMinions", territory.Options.Bonus.Set.StrongerMinions, &territory.Options.Bonus.At.StrongerMinions},
		{"TowerMultiAttack", territory.Options.Bonus.Set.TowerMultiAttack, &territory.Options.Bonus.At.TowerMultiAttack},
		{"TowerAura", territory.Options.Bonus.Set.TowerAura, &territory.Options.Bonus.At.TowerAura},
		{"TowerVolley", territory.Options.Bonus.Set.TowerVolley, &territory.Options.Bonus.At.TowerVolley},

		{"GatheringExperience", territory.Options.Bonus.Set.GatheringExperience, &territory.Options.Bonus.At.GatheringExperience},
		{"MobExperience", territory.Options.Bonus.Set.MobExperience, &territory.Options.Bonus.At.MobExperience},
		{"MobDamage", territory.Options.Bonus.Set.MobDamage, &territory.Options.Bonus.At.MobDamage},
		{"PvPDamage", territory.Options.Bonus.Set.PvPDamage, &territory.Options.Bonus.At.PvPDamage},
		{"TowerVolley", territory.Options.Bonus.Set.TowerVolley, &territory.Options.Bonus.At.TowerVolley},
		{"TowerVolley", territory.Options.Bonus.Set.TowerVolley, &territory.Options.Bonus.At.TowerVolley},
		{"TowerVolley", territory.Options.Bonus.Set.TowerVolley, &territory.Options.Bonus.At.TowerVolley},
		{"XPSeeking", territory.Options.Bonus.Set.XPSeeking, &territory.Options.Bonus.At.XPSeeking},
		{"TomeSeeking", territory.Options.Bonus.Set.TomeSeeking, &territory.Options.Bonus.At.TomeSeeking},
		{"EmeraldSeeking", territory.Options.Bonus.Set.EmeraldSeeking, &territory.Options.Bonus.At.EmeraldSeeking},

		{"LargerResourceStorage", territory.Options.Bonus.Set.LargerResourceStorage, &territory.Options.Bonus.At.LargerResourceStorage},
		{"LargerEmeraldStorage", territory.Options.Bonus.Set.LargerEmeraldStorage, &territory.Options.Bonus.At.LargerEmeraldStorage},
		{"EfficientResource", territory.Options.Bonus.Set.EfficientResource, &territory.Options.Bonus.At.EfficientResource},
		{"EfficientEmerald", territory.Options.Bonus.Set.EfficientEmerald, &territory.Options.Bonus.At.EfficientEmerald},
		{"ResourceRate", territory.Options.Bonus.Set.ResourceRate, &territory.Options.Bonus.At.ResourceRate},
		{"EmeraldRate", territory.Options.Bonus.Set.EmeraldRate, &territory.Options.Bonus.At.EmeraldRate},
	}

	for _, bonus := range bonuses {
		if bonus.set > 0 && canAffordBonus(storage, bonus.name, bonus.set) {
			*bonus.at = bonus.set
		} else {
			*bonus.at = 0
		}
	}
}

// Helper function to set upgrade levels based on affordability
func setAffordableUpgrades(territory *typedef.Territory, storage typedef.BasicResources) {
	upgrades := []struct {
		upgradeType string
		set         int
		at          *int
	}{
		{"damage", territory.Options.Upgrade.Set.Damage, &territory.Options.Upgrade.At.Damage},
		{"attack", territory.Options.Upgrade.Set.Attack, &territory.Options.Upgrade.At.Attack},
		{"health", territory.Options.Upgrade.Set.Health, &territory.Options.Upgrade.At.Health},
		{"defence", territory.Options.Upgrade.Set.Defence, &territory.Options.Upgrade.At.Defence},
	}

	for _, upgrade := range upgrades {
		if upgrade.set > 0 && canAffordUpgrade(storage, upgrade.upgradeType, upgrade.set) {
			*upgrade.at = upgrade.set
		} else {
			*upgrade.at = 0
		}
	}
}

// Helper function to check if an upgrade level can be afforded
func canAffordUpgrade(storage typedef.BasicResources, upgradeType string, level int) bool {
	upgradeID, exists := upgradeTypeMap[upgradeType]
	if !exists {
		return false
	}

	var cost numbers.FixedPoint128
	var resourceType string

	switch upgradeID {
	case UPGRADE_DAMAGE:
		if level >= len(st.costs.UpgradesCost.Damage.Value) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Damage.Value[level])
		resourceType = st.costs.UpgradesCost.Damage.ResourceType
	case UPGRADE_ATTACK:
		if level >= len(st.costs.UpgradesCost.Attack.Value) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Attack.Value[level])
		resourceType = st.costs.UpgradesCost.Attack.ResourceType
	case UPGRADE_HEALTH:
		if level >= len(st.costs.UpgradesCost.Health.Value) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Health.Value[level])
		resourceType = st.costs.UpgradesCost.Health.ResourceType
	case UPGRADE_DEFENCE:
		if level >= len(st.costs.UpgradesCost.Defence.Value) {
			return false
		}
		cost = numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Defence.Value[level])
		resourceType = st.costs.UpgradesCost.Defence.ResourceType
	default:
		return false
	}

	// Pre-compute per second cost
	costPerSec := cost.Multiply(COST_PER_HOUR_TO_PER_SECOND)

	switch resourceType {
	case "emeralds":
		return storage.Emeralds.GreaterThanOrEqual(costPerSec)
	case "ores", "ore":
		return storage.Ores.GreaterThanOrEqual(costPerSec)
	case "wood":
		return storage.Wood.GreaterThanOrEqual(costPerSec)
	case "fish":
		return storage.Fish.GreaterThanOrEqual(costPerSec)
	case "crops":
		return storage.Crops.GreaterThanOrEqual(costPerSec)
	default:
		return false
	}
}

// CalculateGeneration is the exported version of calculateGeneration for external use
func CalculateGeneration(territory *typedef.Territory) (static typedef.BasicResources, now typedef.BasicResourcesSecond, costPerHr typedef.BasicResources, costNow typedef.BasicResourcesSecond) {
	return calculateGeneration(territory)
}

// Called every second to calculate the generation bonus for each territory
func calculateGeneration(territory *typedef.Territory) (static typedef.BasicResources, now typedef.BasicResourcesSecond, costPerHr typedef.BasicResources, costNow typedef.BasicResourcesSecond) {
	// Lock territory RWMutex for writing
	territory.Mu.Lock()
	defer territory.Mu.Unlock()

	return calculateGenerationInternal(territory)
}

// Internal function that doesn't lock (used when already locked)
func calculateGenerationInternal(territory *typedef.Territory) (static typedef.BasicResources, now typedef.BasicResourcesSecond, costPerHr typedef.BasicResources, costNow typedef.BasicResourcesSecond) {
	// Calculate total costs for all upgrades and bonuses (regardless of affordability)
	costPerHr = calculateTotalCosts(territory)

	// Apply route tax adjustment to costs if this territory has a route tax
	// This accounts for resources lost to taxes when being sent from HQ
	if territory.RouteTax > 0 {
		// If territory is being taxed, the HQ needs to send more resources to cover the tax
		// Formula: adjustedCost = originalCost / (1 - taxRate)
		// Round to 4 decimal places before converting to FixedPoint128
		taxMultiplierFloat := 1.0 / (1.0 - territory.RouteTax)
		taxMultiplierRounded := math.Round(taxMultiplierFloat*10000) / 10000
		taxMultiplier := numbers.NewFixedPointFromFloat(taxMultiplierRounded)

		// Create adjusted costs using the tax multiplier
		adjustedCostPerHr := typedef.BasicResources{
			Emeralds: costPerHr.Emeralds.Multiply(taxMultiplier),
			Ores:     costPerHr.Ores.Multiply(taxMultiplier),
			Wood:     costPerHr.Wood.Multiply(taxMultiplier),
			Fish:     costPerHr.Fish.Multiply(taxMultiplier),
			Crops:    costPerHr.Crops.Multiply(taxMultiplier),
		}
		costPerHr = adjustedCostPerHr
	}

	// Store the tax-adjusted costs in the territory object
	territory.Costs = costPerHr

	// Check affordability for each individual bonus and set "At" values accordingly
	setAffordableBonuses(territory, territory.Storage.At)

	// Check affordability for each individual upgrade and set "At" values accordingly
	setAffordableUpgrades(territory, territory.Storage.At)

	// Calculate costNow based on what can actually be afforded
	costNow = calculateAffordableCosts(territory)

	// Apply route tax adjustment to costs per second if needed
	if territory.RouteTax > 0 {
		taxMultiplierFloat := 1.0 / (1.0 - territory.RouteTax)
		taxMultiplierRounded := math.Round(taxMultiplierFloat*10000) / 10000
		taxMultiplier := numbers.NewFixedPointFromFloat(taxMultiplierRounded)

		// Create adjusted costs using the tax multiplier
		adjustedCostNow := typedef.BasicResourcesSecond{
			Emeralds: costNow.Emeralds.Multiply(taxMultiplier),
			Ores:     costNow.Ores.Multiply(taxMultiplier),
			Wood:     costNow.Wood.Multiply(taxMultiplier),
			Fish:     costNow.Fish.Multiply(taxMultiplier),
			Crops:    costNow.Crops.Multiply(taxMultiplier),
		}
		costNow = adjustedCostNow
	}

	// Get the actual levels that can be afforded
	actualResourceRate := territory.Options.Bonus.At.ResourceRate
	actualEmeraldRate := territory.Options.Bonus.At.EmeraldRate
	actualEfficientResource := territory.Options.Bonus.At.EfficientResource
	actualEfficientEmerald := territory.Options.Bonus.At.EfficientEmerald

	// Calculate generation using actual affordable levels
	static, now = calculateResourceGeneration(territory, actualResourceRate, actualEmeraldRate, actualEfficientResource, actualEfficientEmerald)

	// Apply route tax to resource generation if territory is not an HQ
	// This accounts for resources lost to taxes when being sent to HQ
	if !territory.HQ && territory.RouteTax > 0 {
		// Resources sent to HQ are reduced by the tax rate
		taxMultiplierFloat := 1.0 - territory.RouteTax
		taxMultiplierRounded := math.Round(taxMultiplierFloat*10000) / 10000
		taxMultiplier := numbers.NewFixedPointFromFloat(taxMultiplierRounded)

		// Apply tax to static generation (per hour)
		taxedStatic := typedef.BasicResources{
			Emeralds: static.Emeralds.Multiply(taxMultiplier),
			Ores:     static.Ores.Multiply(taxMultiplier),
			Wood:     static.Wood.Multiply(taxMultiplier),
			Fish:     static.Fish.Multiply(taxMultiplier),
			Crops:    static.Crops.Multiply(taxMultiplier),
		}
		static = taxedStatic

		// Apply tax to per-second generation
		taxedNow := typedef.BasicResourcesSecond{
			Emeralds: now.Emeralds.Multiply(taxMultiplier),
			Ores:     now.Ores.Multiply(taxMultiplier),
			Wood:     now.Wood.Multiply(taxMultiplier),
			Fish:     now.Fish.Multiply(taxMultiplier),
			Crops:    now.Crops.Multiply(taxMultiplier),
		}
		now = taxedNow
	}

	// Update territory's current generation
	territory.ResourceGeneration.At = static

	// Calculate and update net resources (generation minus costs)
	// For territories with resource generation, this shows what actually reaches the HQ after tax
	// For territories without generation but with costs, this shows what the HQ must provide with tax
	perSecondCosts := typedef.BasicResourcesSecond{
		Emeralds: costPerHr.Emeralds.Divide(THREE_SIX_HUNDRED),
		Ores:     costPerHr.Ores.Divide(THREE_SIX_HUNDRED),
		Wood:     costPerHr.Wood.Divide(THREE_SIX_HUNDRED),
		Fish:     costPerHr.Fish.Divide(THREE_SIX_HUNDRED),
		Crops:    costPerHr.Crops.Divide(THREE_SIX_HUNDRED),
	}

	// Convert per-second costs back to per-hour for the net calculation
	costsPerHour := typedef.BasicResources{
		Emeralds: perSecondCosts.Emeralds.Multiply(THREE_SIX_HUNDRED),
		Ores:     perSecondCosts.Ores.Multiply(THREE_SIX_HUNDRED),
		Wood:     perSecondCosts.Wood.Multiply(THREE_SIX_HUNDRED),
		Fish:     perSecondCosts.Fish.Multiply(THREE_SIX_HUNDRED),
		Crops:    perSecondCosts.Crops.Multiply(THREE_SIX_HUNDRED),
	}

	// Calculate net resources with tax adjustment considered
	netResources := typedef.BasicResources{
		Emeralds: static.Emeralds.Subtract(costsPerHour.Emeralds),
		Ores:     static.Ores.Subtract(costsPerHour.Ores),
		Wood:     static.Wood.Subtract(costsPerHour.Wood),
		Fish:     static.Fish.Subtract(costsPerHour.Fish),
		Crops:    static.Crops.Subtract(costsPerHour.Crops),
	}
	territory.Net = netResources

	return static, now, costPerHr, costNow
}

// Helper function to calculate total costs for all upgrades and bonuses
func calculateTotalCosts(territory *typedef.Territory) typedef.BasicResources {
	cost := typedef.BasicResources{}

	// Helper function to safely get cost with bounds checking
	getCost := func(costArray []int, level int) numbers.FixedPoint128 {
		if level >= 0 && level < len(costArray) {
			return numbers.NewFixedPointFromInt(costArray[level])
		}
		return numbers.FixedPoint128{}
	}

	// Upgrades
	cost.Ores = cost.Ores.Add(getCost(st.costs.UpgradesCost.Damage.Value, territory.Options.Upgrade.Set.Damage))
	cost.Crops = cost.Crops.Add(getCost(st.costs.UpgradesCost.Attack.Value, territory.Options.Upgrade.Set.Attack))
	cost.Fish = cost.Fish.Add(getCost(st.costs.UpgradesCost.Defence.Value, territory.Options.Upgrade.Set.Defence))
	cost.Wood = cost.Wood.Add(getCost(st.costs.UpgradesCost.Health.Value, territory.Options.Upgrade.Set.Health))

	// Bonuses with bounds checking
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.StrongerMinions.Cost, territory.Options.Bonus.Set.StrongerMinions))
	cost.Fish = cost.Fish.Add(getCost(st.costs.Bonuses.TowerMultiAttack.Cost, territory.Options.Bonus.Set.TowerMultiAttack))
	cost.Crops = cost.Crops.Add(getCost(st.costs.Bonuses.TowerAura.Cost, territory.Options.Bonus.Set.TowerAura))
	cost.Ores = cost.Ores.Add(getCost(st.costs.Bonuses.TowerVolley.Cost, territory.Options.Bonus.Set.TowerVolley))
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.GatheringExperience.Cost, territory.Options.Bonus.Set.GatheringExperience))
	cost.Fish = cost.Fish.Add(getCost(st.costs.Bonuses.MobExperience.Cost, territory.Options.Bonus.Set.MobExperience))
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.MobDamage.Cost, territory.Options.Bonus.Set.MobDamage))
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.PvPDamage.Cost, territory.Options.Bonus.Set.PvPDamage))
	cost.Emeralds = cost.Emeralds.Add(getCost(st.costs.Bonuses.XPSeeking.Cost, territory.Options.Bonus.Set.XPSeeking))
	cost.Fish = cost.Fish.Add(getCost(st.costs.Bonuses.TomeSeeking.Cost, territory.Options.Bonus.Set.TomeSeeking))
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.EmeraldsSeeking.Cost, territory.Options.Bonus.Set.EmeraldSeeking))
	cost.Emeralds = cost.Emeralds.Add(getCost(st.costs.Bonuses.LargerResourceStorage.Cost, territory.Options.Bonus.Set.LargerResourceStorage))
	cost.Wood = cost.Wood.Add(getCost(st.costs.Bonuses.LargerEmeraldsStorage.Cost, territory.Options.Bonus.Set.LargerEmeraldStorage))
	cost.Emeralds = cost.Emeralds.Add(getCost(st.costs.Bonuses.EfficientResource.Cost, territory.Options.Bonus.Set.EfficientResource))
	cost.Ores = cost.Ores.Add(getCost(st.costs.Bonuses.EfficientEmeralds.Cost, territory.Options.Bonus.Set.EfficientEmerald))
	cost.Emeralds = cost.Emeralds.Add(getCost(st.costs.Bonuses.ResourceRate.Cost, territory.Options.Bonus.Set.ResourceRate))
	cost.Crops = cost.Crops.Add(getCost(st.costs.Bonuses.EmeraldsRate.Cost, territory.Options.Bonus.Set.EmeraldRate))

	return cost
}

// Helper function to calculate costs only for what can actually be afforded (per second)
func calculateAffordableCosts(territory *typedef.Territory) typedef.BasicResourcesSecond {
	costNow := typedef.BasicResourcesSecond{}
	storage := territory.Storage.At

	// Pre-compute upgrade costs per second to avoid repeated division
	damageLevel := territory.Options.Upgrade.Set.Damage
	if damageLevel < len(st.costs.UpgradesCost.Damage.Value) {
		damagePerSecond := numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Damage.Value[damageLevel]).Multiply(COST_PER_HOUR_TO_PER_SECOND)
		if storage.Ores.GreaterThanOrEqual(damagePerSecond) {
			costNow.Ores = costNow.Ores.Add(damagePerSecond)
		}
	}

	attackLevel := territory.Options.Upgrade.Set.Attack
	if attackLevel < len(st.costs.UpgradesCost.Attack.Value) {
		attackPerSecond := numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Attack.Value[attackLevel]).Multiply(COST_PER_HOUR_TO_PER_SECOND)
		if storage.Crops.GreaterThanOrEqual(attackPerSecond) {
			costNow.Crops = costNow.Crops.Add(attackPerSecond)
		}
	}

	defenceLevel := territory.Options.Upgrade.Set.Defence
	if defenceLevel < len(st.costs.UpgradesCost.Defence.Value) {
		defencePerSecond := numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Defence.Value[defenceLevel]).Multiply(COST_PER_HOUR_TO_PER_SECOND)
		if storage.Fish.GreaterThanOrEqual(defencePerSecond) {
			costNow.Fish = costNow.Fish.Add(defencePerSecond)
		}
	}

	healthLevel := territory.Options.Upgrade.Set.Health
	if healthLevel < len(st.costs.UpgradesCost.Health.Value) {
		healthPerSecond := numbers.NewFixedPointFromInt(st.costs.UpgradesCost.Health.Value[healthLevel]).Multiply(COST_PER_HOUR_TO_PER_SECOND)
		if storage.Wood.GreaterThanOrEqual(healthPerSecond) {
			costNow.Wood = costNow.Wood.Add(healthPerSecond)
		}
	}

	// For bonuses, use the "At" levels and pre-compute costs
	bonuses := territory.Options.Bonus.At

	if bonuses.StrongerMinions < len(st.costs.Bonuses.StrongerMinions.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.StrongerMinions.Cost[bonuses.StrongerMinions]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.TowerMultiAttack < len(st.costs.Bonuses.TowerMultiAttack.Cost) {
		costNow.Fish = costNow.Fish.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerMultiAttack.Cost[bonuses.TowerMultiAttack]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.TowerAura < len(st.costs.Bonuses.TowerAura.Cost) {
		costNow.Crops = costNow.Crops.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerAura.Cost[bonuses.TowerAura]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.TowerVolley < len(st.costs.Bonuses.TowerVolley.Cost) {
		costNow.Ores = costNow.Ores.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.TowerVolley.Cost[bonuses.TowerVolley]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.GatheringExperience < len(st.costs.Bonuses.GatheringExperience.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.GatheringExperience.Cost[bonuses.GatheringExperience]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.MobExperience < len(st.costs.Bonuses.MobExperience.Cost) {
		costNow.Fish = costNow.Fish.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.MobExperience.Cost[bonuses.MobExperience]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.MobDamage < len(st.costs.Bonuses.MobDamage.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.MobDamage.Cost[bonuses.MobDamage]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.PvPDamage < len(st.costs.Bonuses.PvPDamage.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.PvPDamage.Cost[bonuses.PvPDamage]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.XPSeeking < len(st.costs.Bonuses.XPSeeking.Cost) {
		costNow.Emeralds = costNow.Emeralds.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.XPSeeking.Cost[bonuses.XPSeeking]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.TomeSeeking < len(st.costs.Bonuses.TomeSeeking.Cost) {
		costNow.Fish = costNow.Fish.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.TomeSeeking.Cost[bonuses.TomeSeeking]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.EmeraldSeeking < len(st.costs.Bonuses.EmeraldsSeeking.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.EmeraldsSeeking.Cost[bonuses.EmeraldSeeking]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.LargerResourceStorage < len(st.costs.Bonuses.LargerResourceStorage.Cost) {
		costNow.Emeralds = costNow.Emeralds.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.LargerResourceStorage.Cost[bonuses.LargerResourceStorage]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.LargerEmeraldStorage < len(st.costs.Bonuses.LargerEmeraldsStorage.Cost) {
		costNow.Wood = costNow.Wood.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.LargerEmeraldsStorage.Cost[bonuses.LargerEmeraldStorage]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.EfficientResource < len(st.costs.Bonuses.EfficientResource.Cost) {
		costNow.Emeralds = costNow.Emeralds.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.EfficientResource.Cost[bonuses.EfficientResource]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.EfficientEmerald < len(st.costs.Bonuses.EfficientEmeralds.Cost) {
		costNow.Ores = costNow.Ores.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.EfficientEmeralds.Cost[bonuses.EfficientEmerald]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.ResourceRate < len(st.costs.Bonuses.ResourceRate.Cost) {
		costNow.Emeralds = costNow.Emeralds.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.ResourceRate.Cost[bonuses.ResourceRate]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}
	if bonuses.EmeraldRate < len(st.costs.Bonuses.EmeraldsRate.Cost) {
		costNow.Crops = costNow.Crops.Add(numbers.NewFixedPointFromInt(st.costs.Bonuses.EmeraldsRate.Cost[bonuses.EmeraldRate]).Multiply(COST_PER_HOUR_TO_PER_SECOND))
	}

	return costNow
}

// Helper function to calculate resource generation based on affordable levels
func calculateResourceGeneration(territory *typedef.Territory, resourceRate, emeraldRate, efficientResource, efficientEmerald int) (typedef.BasicResources, typedef.BasicResourcesSecond) {
	// Get base generation
	baseGen := territory.ResourceGeneration.Base

	// Calculate multipliers (these affect total generation per hour)
	resourceMultiplier := numbers.NewFixedPointFromFloat(st.costs.Bonuses.EfficientResource.Value[efficientResource])
	emeraldMultiplier := numbers.NewFixedPointFromFloat(st.costs.Bonuses.EfficientEmeralds.Value[efficientEmerald])

	// Apply treasury bonus (percentage boost)
	treasuryBonus := numbers.NewFixedPointFromFloat(1.0)
	numbers.NewFixedPointFromFloat(territory.GenerationBonus).Divide(numbers.FixedPoint128{Fraction: 1})

	// Calculate base generation per hour with efficiency and treasury bonuses
	baseResourceGenPerHour := typedef.BasicResources{
		Ores:  baseGen.Ores.Multiply(resourceMultiplier).Multiply(treasuryBonus),
		Wood:  baseGen.Wood.Multiply(resourceMultiplier).Multiply(treasuryBonus),
		Fish:  baseGen.Fish.Multiply(resourceMultiplier).Multiply(treasuryBonus),
		Crops: baseGen.Crops.Multiply(resourceMultiplier).Multiply(treasuryBonus),
	}

	baseEmeraldGenPerHour := baseGen.Emeralds.Multiply(emeraldMultiplier) * treasuryBonus

	// Get rate intervals (how often generation happens)
	resourceRateSeconds := float64(st.costs.Bonuses.ResourceRate.Value[resourceRate])
	emeraldRateSeconds := float64(st.costs.Bonuses.EmeraldsRate.Value[emeraldRate])

	// Set DeltaTime for resource and emerald generation
	territory.ResourceGeneration.ResourceDeltaTime = uint8(resourceRateSeconds)
	territory.ResourceGeneration.EmeraldDeltaTime = uint8(emeraldRateSeconds)

	// Calculate rate multipliers - lower interval means more frequent generation
	// Base interval is 4 seconds (level 0), so we calculate how much faster generation is
	baseResourceInterval := 4.0
	baseEmeraldInterval := 4.0
	resourceRateMultiplier := baseResourceInterval / resourceRateSeconds
	emeraldRateMultiplier := baseEmeraldInterval / emeraldRateSeconds

	// Static generation is the total per hour (for display purposes)
	// This must include the rate multiplier to show the correct generation per hour
	static := typedef.BasicResources{
		Emeralds: baseEmeraldGenPerHour * emeraldRateMultiplier,
		Ores:     baseResourceGenPerHour.Ores * resourceRateMultiplier,
		Wood:     baseResourceGenPerHour.Wood * resourceRateMultiplier,
		Fish:     baseResourceGenPerHour.Fish * resourceRateMultiplier,
		Crops:    baseResourceGenPerHour.Crops * resourceRateMultiplier,
	}

	// Calculate the actual per-second generation rates
	// This represents the true per-second rate, not amount per interval
	// The rate multipliers are already factored into the static generation above
	now := typedef.BasicResourcesSecond{
		// Per-second rate: total per hour (including rate multipliers) divided by 3600 seconds
		Emeralds: (baseEmeraldGenPerHour * emeraldRateMultiplier) / 3600.0,
		Ores:     (baseResourceGenPerHour.Ores * resourceRateMultiplier) / 3600.0,
		Wood:     (baseResourceGenPerHour.Wood * resourceRateMultiplier) / 3600.0,
		Fish:     (baseResourceGenPerHour.Fish * resourceRateMultiplier) / 3600.0,
		Crops:    (baseResourceGenPerHour.Crops * resourceRateMultiplier) / 3600.0,
	}

	return static, now
}

func doGenerate(territory *typedef.Territory) {
	// Lock territory for writing to prevent race conditions
	territory.Mu.Lock()
	defer territory.Mu.Unlock()

	// Clear previous warnings
	territory.Warning = 0

	// Calculate generation and costs WITHOUT re-locking (already locked)
	staticGen, _, _, costNow := calculateGenerationInternal(territory)

	// Calculate storage capacity with bonuses
	baseCapacity := typedef.BaseResourceCapacity
	storageMultiplier := float64(st.costs.Bonuses.LargerResourceStorage.Value[territory.Options.Bonus.At.LargerResourceStorage])
	emeraldStorageMultiplier := float64(st.costs.Bonuses.LargerEmeraldsStorage.Value[territory.Options.Bonus.At.LargerEmeraldStorage])

	// Apply HQ multiplier if this territory is an HQ
	hqMultiplier := 1.0
	if territory.HQ {
		hqMultiplier = 5.0
	}

	// Fix: For HQ, use explicit emerald storage values by level
	var emeraldHQCapByLevel = []float64{5000, 10000, 20000, 40000, 75000, 170000, 400000}
	emeraldStorageLevel := territory.Options.Bonus.At.LargerEmeraldStorage
	var emeraldCap float64
	if territory.HQ && emeraldStorageLevel >= 0 && emeraldStorageLevel < len(emeraldHQCapByLevel) {
		emeraldCap = emeraldHQCapByLevel[emeraldStorageLevel]
	} else {
		emeraldCap = typedef.BaseResourceCapacity.Emeralds * emeraldStorageMultiplier
	}

	maxStorage := typedef.BasicResources{
		Emeralds: emeraldCap,
		Ores:     baseCapacity.Ores * storageMultiplier * hqMultiplier,
		Wood:     baseCapacity.Wood * storageMultiplier * hqMultiplier,
		Fish:     baseCapacity.Fish * storageMultiplier * hqMultiplier,
		Crops:    baseCapacity.Crops * storageMultiplier * hqMultiplier,
	}

	// Start with current storage as the base
	currentStorage := territory.Storage.At
	newStorage := currentStorage

	// STEP 1: Check if it's time to release accumulated resources based on rate intervals
	currentTick := st.tick

	// Initialize last tick values if this is the first generation calculation
	isFirstCall := false
	if territory.ResourceGeneration.LastResourceTick == 0 {
		territory.ResourceGeneration.LastResourceTick = currentTick
		isFirstCall = true
	}
	if territory.ResourceGeneration.LastEmeraldTick == 0 {
		territory.ResourceGeneration.LastEmeraldTick = currentTick
		isFirstCall = true
	}

	// Only accumulate if this is NOT the first call (avoid accumulating on initialization tick)
	if !isFirstCall {
		// STEP 2: Accumulate generation CONTINUOUSLY (every tick)
		// Calculate the amount that should be accumulated per tick for each interval type
		resourcePerTickAmount := staticGen.PerSecond()
		emeraldPerTickAmount := staticGen.Emeralds / 3600.0

		// Add per-tick generation to accumulators
		territory.ResourceGeneration.ResourceAccumulator.Ores += resourcePerTickAmount.Ores
		territory.ResourceGeneration.ResourceAccumulator.Wood += resourcePerTickAmount.Wood
		territory.ResourceGeneration.ResourceAccumulator.Fish += resourcePerTickAmount.Fish
		territory.ResourceGeneration.ResourceAccumulator.Crops += resourcePerTickAmount.Crops
		territory.ResourceGeneration.EmeraldAccumulator += emeraldPerTickAmount
	}

	// Check resource generation interval
	resourceInterval := uint64(territory.ResourceGeneration.ResourceDeltaTime)
	if resourceInterval > 0 && (currentTick-territory.ResourceGeneration.LastResourceTick) >= resourceInterval {
		// Time to release accumulated resources
		generatedOres := territory.ResourceGeneration.ResourceAccumulator.Ores
		generatedWood := territory.ResourceGeneration.ResourceAccumulator.Wood
		generatedFish := territory.ResourceGeneration.ResourceAccumulator.Fish
		generatedCrops := territory.ResourceGeneration.ResourceAccumulator.Crops

		// Calculate how much we can actually add without exceeding capacity
		availableOresCapacity := max(0, maxStorage.Ores-newStorage.Ores)
		availableWoodCapacity := max(0, maxStorage.Wood-newStorage.Wood)
		availableFishCapacity := max(0, maxStorage.Fish-newStorage.Fish)
		availableCropsCapacity := max(0, maxStorage.Crops-newStorage.Crops)

		// Add generated resources, capped at available capacity
		actualOresAdded := min(generatedOres, availableOresCapacity)
		actualWoodAdded := min(generatedWood, availableWoodCapacity)
		actualFishAdded := min(generatedFish, availableFishCapacity)
		actualCropsAdded := min(generatedCrops, availableCropsCapacity)

		newStorage.Ores += actualOresAdded
		newStorage.Wood += actualWoodAdded
		newStorage.Fish += actualFishAdded
		newStorage.Crops += actualCropsAdded

		// Set overflow warning if any generated resource was capped
		if actualOresAdded < generatedOres || actualWoodAdded < generatedWood ||
			actualFishAdded < generatedFish || actualCropsAdded < generatedCrops {
			territory.Warning |= typedef.WarningOverflowResources
		}

		// Reset resource accumulator and update last tick
		territory.ResourceGeneration.ResourceAccumulator.Ores = 0
		territory.ResourceGeneration.ResourceAccumulator.Wood = 0
		territory.ResourceGeneration.ResourceAccumulator.Fish = 0
		territory.ResourceGeneration.ResourceAccumulator.Crops = 0
		territory.ResourceGeneration.LastResourceTick = currentTick
	}

	// Check emerald generation interval
	emeraldInterval := uint64(territory.ResourceGeneration.EmeraldDeltaTime)
	if emeraldInterval > 0 && (currentTick-territory.ResourceGeneration.LastEmeraldTick) >= emeraldInterval {
		// Time to release accumulated emeralds
		generatedEmeralds := territory.ResourceGeneration.EmeraldAccumulator

		// Calculate how much we can actually add without exceeding capacity
		availableEmeraldCapacity := max(0, maxStorage.Emeralds-newStorage.Emeralds)

		// Add generated emeralds, capped at available capacity
		actualEmeraldsAdded := min(generatedEmeralds, availableEmeraldCapacity)
		newStorage.Emeralds += actualEmeraldsAdded

		// Set overflow warning if generated emeralds were capped
		if actualEmeraldsAdded < generatedEmeralds {
			territory.Warning |= typedef.WarningOverflowEmerald
		}

		// Reset emerald accumulator and update last tick
		territory.ResourceGeneration.EmeraldAccumulator = 0
		territory.ResourceGeneration.LastEmeraldTick = currentTick
	}

	// STEP 3: Consume costs AFTER generation has been processed
	newStorage = typedef.BasicResources{
		Emeralds: max(0, newStorage.Emeralds-costNow.Emeralds),
		Ores:     max(0, newStorage.Ores-costNow.Ores),
		Wood:     max(0, newStorage.Wood-costNow.Wood),
		Fish:     max(0, newStorage.Fish-costNow.Fish),
		Crops:    max(0, newStorage.Crops-costNow.Crops),
	}

	// STEP 4: Set overflow warnings and handle HQ clamping
	// For HQ territories: clamp manual edits to capacity
	// For normal territories: allow manual edits to exceed capacity
	if territory.HQ {
		// HQ territories: clamp storage to capacity limits
		if newStorage.Emeralds > maxStorage.Emeralds {
			newStorage.Emeralds = maxStorage.Emeralds
		}

		if newStorage.Ores > maxStorage.Ores ||
			newStorage.Wood > maxStorage.Wood ||
			newStorage.Fish > maxStorage.Fish ||
			newStorage.Crops > maxStorage.Crops {
			territory.Warning |= typedef.WarningOverflowResources
			newStorage.Ores = min(newStorage.Ores, maxStorage.Ores)
			newStorage.Wood = min(newStorage.Wood, maxStorage.Wood)
			newStorage.Fish = min(newStorage.Fish, maxStorage.Fish)
			newStorage.Crops = min(newStorage.Crops, maxStorage.Crops)
		}
	} else {
		// Normal territories: only set warnings, don't clamp manual edits
		if newStorage.Emeralds > maxStorage.Emeralds {
			territory.Warning |= typedef.WarningOverflowEmerald
		}

		if newStorage.Ores > maxStorage.Ores ||
			newStorage.Wood > maxStorage.Wood ||
			newStorage.Fish > maxStorage.Fish ||
			newStorage.Crops > maxStorage.Crops {
			territory.Warning |= typedef.WarningOverflowResources
		}
	}

	// STEP 5: Update territory storage and capacity
	territory.Storage.At = newStorage
	territory.Storage.Capacity = maxStorage
}

// calculateTowerStats calculates the current tower stats based on the "At" upgrade levels
func calculateTowerStats(territory *typedef.Territory) typedef.TowerStats {
	// Get the actual affordable upgrade levels (At values)
	damageLevel := territory.Options.Upgrade.At.Damage
	attackLevel := territory.Options.Upgrade.At.Attack
	healthLevel := territory.Options.Upgrade.At.Health
	defenceLevel := territory.Options.Upgrade.At.Defence

	// Clamp levels to valid ranges
	if damageLevel < 0 {
		damageLevel = 0
	} else if damageLevel >= len(st.costs.UpgradeMultiplier.Damage) {
		damageLevel = len(st.costs.UpgradeMultiplier.Damage) - 1
	}

	if attackLevel < 0 {
		attackLevel = 0
	} else if attackLevel >= len(st.costs.UpgradeMultiplier.Attack) {
		attackLevel = len(st.costs.UpgradeMultiplier.Attack) - 1
	}

	if healthLevel < 0 {
		healthLevel = 0
	} else if healthLevel >= len(st.costs.UpgradeMultiplier.Health) {
		healthLevel = len(st.costs.UpgradeMultiplier.Health) - 1
	}

	if defenceLevel < 0 {
		defenceLevel = 0
	} else if defenceLevel >= len(st.costs.UpgradeMultiplier.Defence) {
		defenceLevel = len(st.costs.UpgradeMultiplier.Defence) - 1
	}

	// Base stats according to game documentation
	baseDamageLow := 1000.0
	baseDamageHigh := 1500.0
	baseAttack := 0.5
	baseHealth := 300000.0
	baseDefence := 0.1 // 10%

	// Apply upgrade multipliers
	damageMultiplier := st.costs.UpgradeMultiplier.Damage[damageLevel]
	attackMultiplier := st.costs.UpgradeMultiplier.Attack[attackLevel]
	healthMultiplier := st.costs.UpgradeMultiplier.Health[healthLevel]
	defenceMultiplier := st.costs.UpgradeMultiplier.Defence[defenceLevel]

	newDamageLow := baseDamageLow * damageMultiplier
	newDamageHigh := baseDamageHigh * damageMultiplier
	newAttack := baseAttack * attackMultiplier
	newHealth := baseHealth * healthMultiplier
	newDefence := baseDefence * defenceMultiplier // Defense is already in decimal form (0.1 = 10%)

	// Calculate territory level for display purposes (based on actual affordable levels)
	// Calculate Aura bonus: Aura 0 = +0, Aura 1 = +5, Aura 2 = +6, Aura 3 = +7, Aura 4 = +8, etc.
	calcAuraBonus := func(aura int) int {
		if aura == 0 {
			return 0
		}
		return 4 + aura
	}

	// Calculate Volley bonus: Volley 0 = +0, Volley 1 = +3, Volley 2 = +4, Volley 3 = +5, etc.
	calcVolleyBonus := func(volley int) int {
		if volley == 0 {
			return 0
		}
		return 2 + volley
	}

	activeAuraLv := calcAuraBonus(territory.Options.Bonus.At.TowerAura)
	activeVolleyLv := calcVolleyBonus(territory.Options.Bonus.At.TowerVolley)

	territory.LevelInt = uint8(damageLevel + attackLevel + healthLevel + defenceLevel +
		territory.Options.Bonus.At.TowerAura + territory.Options.Bonus.At.TowerVolley +
		activeAuraLv + activeVolleyLv)

	// Calculate set level for display purposes (based on user-configured levels)
	setAuraLv := calcAuraBonus(territory.Options.Bonus.Set.TowerAura)
	setVolleyLv := calcVolleyBonus(territory.Options.Bonus.Set.TowerVolley)

	territory.SetLevelInt = uint8(territory.Options.Upgrade.Set.Damage + territory.Options.Upgrade.Set.Attack +
		territory.Options.Upgrade.Set.Health + territory.Options.Upgrade.Set.Defence +
		territory.Options.Bonus.Set.TowerAura + territory.Options.Bonus.Set.TowerVolley +
		setAuraLv + setVolleyLv)

	switch {
	case territory.LevelInt >= 49:
		territory.Level = typedef.DefenceLevelVeryHigh
	case territory.LevelInt >= 31:
		territory.Level = typedef.DefenceLevelHigh
	case territory.LevelInt >= 19:
		territory.Level = typedef.DefenceLevelMedium
	case territory.LevelInt >= 6:
		territory.Level = typedef.DefenceLevelLow
	default:
		territory.Level = typedef.DefenceLevelVeryLow
	}

	switch {
	case territory.SetLevelInt >= 49:
		territory.SetLevel = typedef.DefenceLevelVeryHigh
	case territory.SetLevelInt >= 31:
		territory.SetLevel = typedef.DefenceLevelHigh
	case territory.SetLevelInt >= 19:
		territory.SetLevel = typedef.DefenceLevelMedium
	case territory.SetLevelInt >= 6:
		territory.SetLevel = typedef.DefenceLevelLow
	default:
		territory.SetLevel = typedef.DefenceLevelVeryLow
	}

	if territory.HQ {
		// HQ gets +1 tier
		if territory.Level < typedef.DefenceLevelVeryHigh {
			territory.Level++
		}

		if territory.SetLevel < typedef.DefenceLevelVeryHigh {
			territory.SetLevel++
		}
	}

	// Calculate link bonus and external connection bonus
	linkBonus := calculateLinkBonus(&territory.Links)
	externalBonus := calculateExternalBonus(&territory.Links, territory.HQ)

	return typedef.TowerStats{
		Damage: typedef.DamageRange{
			Low:  newDamageLow * linkBonus * externalBonus,
			High: newDamageHigh * linkBonus * externalBonus,
		},
		Attack:  newAttack, // Attack rate is not affected by bonuses
		Health:  newHealth * linkBonus * externalBonus,
		Defence: newDefence, // Defense is not affected by bonuses
	}
}

// calculateLinkBonus calculates the link bonus (30% per direct connection)
func calculateLinkBonus(tl *typedef.Links) float64 {
	if tl == nil || len(tl.Direct) == 0 {
		return 1.0 // No connections, no bonus
	}

	// Each link adds 30% bonus
	return 1.0 + (0.3 * float64(len(tl.Direct)))
}

// calculateExternalBonus calculates the external connection bonus (only for HQ)
func calculateExternalBonus(tl *typedef.Links, isHQ bool) float64 {
	if !isHQ {
		return 1.0 // Regular towers don't get external bonus
	}

	if tl == nil || len(tl.Externals) == 0 {
		return 1.5 // HQ base bonus of 50% (1 + 0.5)
	}

	// Base HQ bonus (50%) + 25% per external territory
	return 1.5 + (0.25 * float64(len(tl.Externals)))
}

func FormatValue(value float64) string {
	if value < 100 {
		// Show 1 decimal place
		return fmt.Sprintf("%.1f", value)
	} else if value < 1000 {
		return fmt.Sprintf("%.0f", value)
	} else if value < 1000000 {
		// Format as k notation
		thousands := int(value / 1000)
		remainder := int(value) % 1000
		if remainder == 0 {
			return fmt.Sprintf("%dk", thousands)
		}
		// Show 2 decimal places (hundredths), but remove trailing zero
		decimal := remainder / 10
		if decimal%10 == 0 {
			return fmt.Sprintf("%dk%d", thousands, decimal/10)
		}
		return fmt.Sprintf("%dk%02d", thousands, decimal)
	} else if value < 1000000000 {
		// Format as M notation
		millions := int(value / 1000000)
		remainder := int(value) % 1000000
		if remainder == 0 {
			return fmt.Sprintf("%dM", millions)
		}
		// Show 2 decimal places (ten-thousands), but remove trailing zero
		decimal := remainder / 10000
		if decimal%10 == 0 {
			return fmt.Sprintf("%dM%d", millions, decimal/10)
		}
		return fmt.Sprintf("%dM%02d", millions, decimal)
	} else {
		// Format as B notation
		billions := int(value / 1000000000)
		remainder := int(value) % 1000000000
		if remainder == 0 {
			return fmt.Sprintf("%dB", billions)
		}
		// Show 2 decimal places (ten-millions), but remove trailing zero
		decimal := remainder / 10000000
		if decimal%10 == 0 {
			return fmt.Sprintf("%dB%d", billions, decimal/10)
		}
		return fmt.Sprintf("%dB%02d", billions, decimal)
	}
}
