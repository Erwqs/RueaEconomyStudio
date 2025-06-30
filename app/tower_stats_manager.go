package app

import (
	"etools/eruntime"
	"etools/typedef"
	"fmt"
	"image/color"
)

// UpdateTowerStats updates only the Tower Stats section with new territory data
func (m *EdgeMenu) UpdateTowerStats(territoryName string) {
	// Get fresh territory data
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return
	}

	// Find the "Tower Stats" collapsible menu and update its text elements
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			if collapsible.title == "Tower Stats" {
				// Clear existing tower stats text elements
				collapsible.elements = collapsible.elements[:0]

				// Re-add updated tower stats text elements
				collapsible.Text(fmt.Sprintf("Damage: %.0f - %.0f",
					territory.TowerStats.Damage.Low,
					territory.TowerStats.Damage.High),
					DefaultTextOptions())

				collapsible.Text(fmt.Sprintf("Attack: %.1f/s", territory.TowerStats.Attack), DefaultTextOptions())

				collapsible.Text(fmt.Sprintf("Health: %.0f", territory.TowerStats.Health), DefaultTextOptions())

				collapsible.Text(fmt.Sprintf("Defence: %.1f%%", territory.TowerStats.Defence*100), DefaultTextOptions())

				collapsible.Spacer(DefaultSpacerOptions())

				// Calculate and display average DPS
				averageDPS := territory.TowerStats.Attack * ((float64(territory.TowerStats.Damage.High)) + (float64(territory.TowerStats.Damage.Low))) / 2
				collapsible.Text(fmt.Sprintf("Average DPS: %.0f", averageDPS), DefaultTextOptions())

				// Calculate and display effective HP
				ehp := territory.TowerStats.Health / (1.0 - territory.TowerStats.Defence)
				collapsible.Text(fmt.Sprintf("Effective HP: %.0f", ehp), DefaultTextOptions())

				// Level and LevelInt
				levelString, colour := getLevelString(territory.Level)
				textOptions := DefaultTextOptions()
				textOptions.Color = colour
				collapsible.Text(fmt.Sprintf("%s (%d)", levelString, territory.LevelInt), textOptions)

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
