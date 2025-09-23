package app

import (
	"etools/eruntime"
	"etools/typedef"
	"fmt"
	"image/color"
)

// UpdateTowerStats updates only the Tower Stats section with new territory data
func (m *EdgeMenu) UpdateTowerStats(territoryName string) {
	// fmt.Printf("DEBUG: UpdateTowerStats called for territory: %s\n", territoryName)
	// Get fresh territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return
	}

	// Find the "Tower Stats" collapsible menu and update its text elements in place
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			if collapsible.title == "Tower Stats" {
				// Update existing text elements instead of replacing them
				elementIndex := 0

				// Update damage text (element 0)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						textElement.text = fmt.Sprintf("Damage: %.0f - %.0f",
							territory.TowerStats.Damage.Low,
							territory.TowerStats.Damage.High)
					}
				}
				elementIndex++

				// Update Wynn's Math damage text (element 1)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						textElement.text = fmt.Sprintf("Wynn's Math Damage: %.0f - %.0f",
							territory.TowerStats.Damage.Low*2,
							territory.TowerStats.Damage.High*2)
					}
				}
				elementIndex++

				// Update attack text (element 2)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						textElement.text = fmt.Sprintf("Attack: %.1f/s", territory.TowerStats.Attack)
					}
				}
				elementIndex++

				// Update health text (element 3)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						textElement.text = fmt.Sprintf("Health: %.0f", territory.TowerStats.Health)
					}
				}
				elementIndex++

				// Update defence text (element 4)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						textElement.text = fmt.Sprintf("Defence: %.1f%%", territory.TowerStats.Defence*100)
					}
				}
				elementIndex++

				// Skip spacer (element 5)
				elementIndex++

				// Update average DPS text (element 6)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						averageDPS := territory.TowerStats.Attack * ((float64(territory.TowerStats.Damage.High)) + (float64(territory.TowerStats.Damage.Low))) / 2
						textElement.text = fmt.Sprintf("Average DPS: %.0f", averageDPS)
					}
				}
				elementIndex++

				// Update Wynn's math average DPS text (element 7)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						averageDPS2 := territory.TowerStats.Attack * ((float64(territory.TowerStats.Damage.High * 2)) + (float64(territory.TowerStats.Damage.Low * 2))) / 2
						textElement.text = fmt.Sprintf("Wynn's Math Average DPS: %.0f", averageDPS2)
					}
				}
				elementIndex++

				// Update effective HP text (element 8)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						ehp := territory.TowerStats.Health / (1.0 - territory.TowerStats.Defence)
						textElement.text = fmt.Sprintf("Effective HP: %.0f", ehp)
					}
				}
				elementIndex++

				// Update level text (element 9)
				if elementIndex < len(collapsible.elements) {
					if textElement, ok := collapsible.elements[elementIndex].(*MenuText); ok {
						levelString, colour := getLevelString(territory.Level)
						textElement.text = fmt.Sprintf("%s (%d)", levelString, territory.LevelInt)
						textElement.options.Color = colour
					}
				}

				// fmt.Printf("DEBUG: Tower stats updated in place for %s\n", territoryName)
				return // Found and updated, exit
			}
		}
	}
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
