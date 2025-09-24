package app

import (
	"etools/eruntime"
	"etools/typedef"
	"fmt"
	"image/color"
)

// UpdateTowerStats updates only the Tower Stats section with new territory data
func (m *EdgeMenu) UpdateTowerStats(territoryName string) {
	fmt.Printf("DEBUG: UpdateTowerStats called for territory: %s\n", territoryName)
	// Get fresh territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return
	}

	// Calculate configured stats based on Set upgrades/bonuses
	configuredStats := m.calculateConfiguredStats(territory)
	
	// Calculate territory levels
	configuredLevel, configuredLevelString := m.calculateConfiguredTerritoryLevel(territory)
	currentLevel, currentLevelString := m.calculateCurrentTerritoryLevel(territory)

	// Find the "Tower Stats" collapsible menu
	for _, element := range m.elements {
		if towerStatsMenu, ok := element.(*CollapsibleMenu); ok {
			if towerStatsMenu.title == "Tower Stats" {
				// Update both Configured and Current submenus
				for _, subElement := range towerStatsMenu.elements {
					if submenu, ok := subElement.(*CollapsibleMenu); ok {
						if submenu.title == "Configured" {
							// Update Configured stats with calculated values
							m.updateStatsElementsWithLevelAndColor(submenu, &configuredStats, configuredLevelString, configuredLevel)
							fmt.Printf("DEBUG: Configured tower stats updated in place for %s\n", territoryName)
						} else if submenu.title == "Current" {
							// Update Current stats with territory values
							m.updateStatsElementsWithLevelAndColor(submenu, &territory.TowerStats, currentLevelString, currentLevel)
							fmt.Printf("DEBUG: Current tower stats updated in place for %s\n", territoryName)
						}
					}
				}
				return // Found Tower Stats, exit
			}
		}
	}
}

// updateStatsElements updates the text elements within a stats menu (Configured or Current)
func (m *EdgeMenu) updateStatsElements(statsMenu *CollapsibleMenu, stats *typedef.TowerStats) {
	m.updateStatsElementsWithLevelAndColor(statsMenu, stats, "", typedef.DefenceLevelVeryLow)
}

// updateStatsElementsWithLevel updates the text elements within a stats menu with territory level
func (m *EdgeMenu) updateStatsElementsWithLevelAndColor(statsMenu *CollapsibleMenu, stats *typedef.TowerStats, levelString string, level typedef.DefenceLevel) {
	elementIndex := 0

	// Update damage text (element 0)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = fmt.Sprintf("Damage: %.0f - %.0f", stats.Damage.Low, stats.Damage.High)
		}
	}
	elementIndex++

	// Update Wynn's Math damage text (element 1)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = fmt.Sprintf("Wynn's Math Damage: %.0f - %.0f", stats.Damage.Low*2, stats.Damage.High*2)
		}
	}
	elementIndex++

	// Update attack text (element 2)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = fmt.Sprintf("Attack: %.1f/s", stats.Attack)
		}
	}
	elementIndex++

	// Update health text (element 3)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = fmt.Sprintf("Health: %.0f", stats.Health)
		}
	}
	elementIndex++

	// Update defence text (element 4)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = fmt.Sprintf("Defence: %.1f%%", stats.Defence*100)
		}
	}
	elementIndex++

	// Skip spacer (element 5)
	elementIndex++

	// Update average DPS text (element 6)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			averageDPS := stats.Attack * ((stats.Damage.High + stats.Damage.Low) / 2)
			textElement.text = fmt.Sprintf("Average DPS: %.0f", averageDPS)
		}
	}
	elementIndex++

	// Update Wynn's math average DPS text (element 7)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			averageDPS2 := stats.Attack * ((stats.Damage.High*2 + stats.Damage.Low*2) / 2)
			textElement.text = fmt.Sprintf("Wynn's Math Average DPS: %.0f", averageDPS2)
		}
	}
	elementIndex++

	// Update effective HP text (element 8)
	if elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			ehp := stats.Health / (1.0 - stats.Defence)
			textElement.text = fmt.Sprintf("Effective HP: %.0f", ehp)
		}
	}
	elementIndex++

	// Update territory level if provided (element 9)
	if levelString != "" && elementIndex < len(statsMenu.elements) {
		if textElement, ok := statsMenu.elements[elementIndex].(*MenuText); ok {
			textElement.text = levelString
			// Apply the correct color for the defence level
			_, levelColor := getLevelString(level)
			textElement.options.Color = levelColor
		}
	}
}

// calculateConfiguredStats calculates tower stats based on Set (configured) upgrades and bonuses
// This replicates the eruntime calculation logic but uses Set values instead of At values
func (m *EdgeMenu) calculateConfiguredStats(territory *typedef.Territory) typedef.TowerStats {
	// Get costs for calculations
	costs := eruntime.GetCost()
	
	// Get the configured upgrade levels
	damageLevel := territory.Options.Upgrade.Set.Damage
	attackLevel := territory.Options.Upgrade.Set.Attack
	healthLevel := territory.Options.Upgrade.Set.Health
	defenceLevel := territory.Options.Upgrade.Set.Defence

	// Clamp levels to valid ranges (same as eruntime)
	if damageLevel < 0 {
		damageLevel = 0
	} else if damageLevel >= len(costs.UpgradeMultiplier.Damage) {
		damageLevel = len(costs.UpgradeMultiplier.Damage) - 1
	}

	if attackLevel < 0 {
		attackLevel = 0
	} else if attackLevel >= len(costs.UpgradeMultiplier.Attack) {
		attackLevel = len(costs.UpgradeMultiplier.Attack) - 1
	}

	if healthLevel < 0 {
		healthLevel = 0
	} else if healthLevel >= len(costs.UpgradeMultiplier.Health) {
		healthLevel = len(costs.UpgradeMultiplier.Health) - 1
	}

	if defenceLevel < 0 {
		defenceLevel = 0
	} else if defenceLevel >= len(costs.UpgradeMultiplier.Defence) {
		defenceLevel = len(costs.UpgradeMultiplier.Defence) - 1
	}

	// Base stats (same as eruntime)
	baseDamageLow := 1000.0
	baseDamageHigh := 1500.0
	baseAttack := 0.5
	baseHealth := 300000.0
	baseDefence := 0.1 // 10%

	// Apply upgrade multipliers (same as eruntime)
	damageMultiplier := costs.UpgradeMultiplier.Damage[damageLevel]
	attackMultiplier := costs.UpgradeMultiplier.Attack[attackLevel]
	healthMultiplier := costs.UpgradeMultiplier.Health[healthLevel]
	defenceMultiplier := costs.UpgradeMultiplier.Defence[defenceLevel]

	newDamageLow := baseDamageLow * damageMultiplier
	newDamageHigh := baseDamageHigh * damageMultiplier
	newAttack := baseAttack * attackMultiplier
	newHealth := baseHealth * healthMultiplier
	newDefence := baseDefence * defenceMultiplier

	// Calculate link bonus and external connection bonus (same as eruntime)
	linkBonus := 1.0
	if territory.Links.Direct != nil && len(territory.Links.Direct) > 0 {
		// Each link adds 30% bonus
		linkBonus = 1.0 + (0.3 * float64(len(territory.Links.Direct)))
	}

	externalBonus := 1.0
	if territory.HQ && territory.Links.Externals != nil && len(territory.Links.Externals) > 0 {
		// HQ gets 50% bonus plus 25% per external connection
		externalBonus = 1.5 + (0.25 * float64(len(territory.Links.Externals)))
	}

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

// calculateCurrentTerritoryLevel calculates territory level using At (current affordable) values
func (m *EdgeMenu) calculateCurrentTerritoryLevel(territory *typedef.Territory) (typedef.DefenceLevel, string) {
	// Helper functions (same as eruntime)
	calcAuraBonus := func(aura int) int {
		if aura == 0 {
			return 0
		}
		return 4 + aura
	}

	calcVolleyBonus := func(volley int) int {
		if volley == 0 {
			return 0
		}
		return 2 + volley
	}

	// Use At values (current affordable levels)
	damageLevel := territory.Options.Upgrade.At.Damage
	attackLevel := territory.Options.Upgrade.At.Attack
	healthLevel := territory.Options.Upgrade.At.Health
	defenceLevel := territory.Options.Upgrade.At.Defence

	activeAuraLv := calcAuraBonus(territory.Options.Bonus.At.TowerAura)
	activeVolleyLv := calcVolleyBonus(territory.Options.Bonus.At.TowerVolley)

	levelInt := uint16(damageLevel + attackLevel + healthLevel + defenceLevel +
		territory.Options.Bonus.At.TowerAura + territory.Options.Bonus.At.TowerVolley +
		activeAuraLv + activeVolleyLv)

	if territory.HQ && territory.Links.Externals != nil {
		exts := len(territory.Links.Externals)
		levelInt += uint16(exts) * 4
	}

	// Determine defence level
	var level typedef.DefenceLevel
	switch {
	case levelInt >= 49:
		level = typedef.DefenceLevelVeryHigh
	case levelInt >= 31:
		level = typedef.DefenceLevelHigh
	case levelInt >= 19:
		level = typedef.DefenceLevelMedium
	case levelInt >= 6:
		level = typedef.DefenceLevelLow
	default:
		level = typedef.DefenceLevelVeryLow
	}

	// HQ gets +1 tier bonus
	if territory.HQ && level < typedef.DefenceLevelVeryHigh {
		level++
	}

	levelString, _ := getLevelString(level)
	return level, fmt.Sprintf("%s (%d)", levelString, levelInt)
}

// calculateConfiguredTerritoryLevel calculates territory level using Set (configured) values
func (m *EdgeMenu) calculateConfiguredTerritoryLevel(territory *typedef.Territory) (typedef.DefenceLevel, string) {
	// Helper functions (same as eruntime)
	calcAuraBonus := func(aura int) int {
		if aura == 0 {
			return 0
		}
		return 4 + aura
	}

	calcVolleyBonus := func(volley int) int {
		if volley == 0 {
			return 0
		}
		return 2 + volley
	}

	// Use Set values (configured levels)
	damageLevel := territory.Options.Upgrade.Set.Damage
	attackLevel := territory.Options.Upgrade.Set.Attack
	healthLevel := territory.Options.Upgrade.Set.Health
	defenceLevel := territory.Options.Upgrade.Set.Defence

	setAuraLv := calcAuraBonus(territory.Options.Bonus.Set.TowerAura)
	setVolleyLv := calcVolleyBonus(territory.Options.Bonus.Set.TowerVolley)

	levelInt := uint16(damageLevel + attackLevel + healthLevel + defenceLevel +
		territory.Options.Bonus.Set.TowerAura + territory.Options.Bonus.Set.TowerVolley +
		setAuraLv + setVolleyLv)

	if territory.HQ && territory.Links.Externals != nil {
		exts := len(territory.Links.Externals)
		levelInt += uint16(exts) * 4
	}

	// Determine defence level
	var level typedef.DefenceLevel
	switch {
	case levelInt >= 49:
		level = typedef.DefenceLevelVeryHigh
	case levelInt >= 31:
		level = typedef.DefenceLevelHigh
	case levelInt >= 19:
		level = typedef.DefenceLevelMedium
	case levelInt >= 6:
		level = typedef.DefenceLevelLow
	default:
		level = typedef.DefenceLevelVeryLow
	}

	// HQ gets +1 tier bonus
	if territory.HQ && level < typedef.DefenceLevelVeryHigh {
		level++
	}

	levelString, _ := getLevelString(level)
	return level, fmt.Sprintf("%s (%d)", levelString, levelInt)
}

func getLevelString(level typedef.DefenceLevel) (string, color.RGBA) {
	switch level {
	case typedef.DefenceLevelVeryHigh:
		return "Very High", color.RGBA{255, 0, 0, 255}
	case typedef.DefenceLevelHigh:
		return "High", color.RGBA{255, 128, 0, 255}
	case typedef.DefenceLevelMedium:
		return "Medium", color.RGBA{255, 255, 0, 255}
	case typedef.DefenceLevelLow:
		return "Low", color.RGBA{128, 255, 0, 255}
	case typedef.DefenceLevelVeryLow:
		return "Very Low", color.RGBA{0, 255, 0, 255}
	default:
		return "Unknown", color.RGBA{128, 128, 128, 255}
	}
}
