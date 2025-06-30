package eruntime

import "etools/typedef"

func calculateTreasury(territory *typedef.Territory) float64 {
	bonusMultiplier := 1.0
	if !st.runtimeOptions.TreasuryEnabled {
		return bonusMultiplier
	}

	// Calculate distance from HQ based on trading routes
	if len(territory.TradingRoutes) == 0 || (len(territory.TradingRoutes) == 0 && !territory.HQ){
		return bonusMultiplier // No trading routes means no treasury bonus
	}

	distance := len(territory.TradingRoutes[0])

	if territory.HQ {
		distance = 0
	}

	// Clamp distance
	if distance > 6 {
		distance = 6 // Maximum distance
	}

	// Get the territory's displayed level
	level := territory.Treasury

	// Calculate treasury bonus based on distance and level
	var treasuryBonus float64

	switch distance {
	case 3:
		switch level {
		case typedef.TreasuryLevelVeryLow:
			treasuryBonus = 0.0 // 0%
		case typedef.TreasuryLevelLow:
			treasuryBonus = 0.085 // 8.5%
		case typedef.TreasuryLevelMedium:
			treasuryBonus = 0.17 // 17%
		case typedef.TreasuryLevelHigh:
			treasuryBonus = 0.2125 // 21.25%
		case typedef.TreasuryLevelVeryHigh:
			treasuryBonus = 0.255 // 25.5%
		}
	case 4:
		switch level {
		case typedef.TreasuryLevelVeryLow:
			treasuryBonus = 0.0 // 0%
		case typedef.TreasuryLevelLow:
			treasuryBonus = 0.07 // 7%
		case typedef.TreasuryLevelMedium:
			treasuryBonus = 0.14 // 14%
		case typedef.TreasuryLevelHigh:
			treasuryBonus = 0.175 // 17.5%
		case typedef.TreasuryLevelVeryHigh:
			treasuryBonus = 0.21 // 21%
		}
	case 5:
		switch level {
		case typedef.TreasuryLevelVeryLow:
			treasuryBonus = 0.0 // 0%
		case typedef.TreasuryLevelLow:
			treasuryBonus = 0.055 // 5.5%
		case typedef.TreasuryLevelMedium:
			treasuryBonus = 0.11 // 11%
		case typedef.TreasuryLevelHigh:
			treasuryBonus = 0.1375 // 13.75%
		case typedef.TreasuryLevelVeryHigh:
			treasuryBonus = 0.165 // 16.5%
		}
	case 6:
		switch level {
		case typedef.TreasuryLevelVeryLow:
			treasuryBonus = 0.0 // 0%
		case typedef.TreasuryLevelLow:
			treasuryBonus = 0.04 // 4%
		case typedef.TreasuryLevelMedium:
			treasuryBonus = 0.08 // 8%
		case typedef.TreasuryLevelHigh:
			treasuryBonus = 0.10 // 10%
		case typedef.TreasuryLevelVeryHigh:
			treasuryBonus = 0.12 // 12%
		}
	default:
		// For HQ + 1 & 2 distance
		switch level {
		case typedef.TreasuryLevelVeryLow:
			treasuryBonus = 0.0 // 0%
		case typedef.TreasuryLevelLow:
			treasuryBonus = 0.10 // 10%
		case typedef.TreasuryLevelMedium:
			treasuryBonus = 0.20 // 20%
		case typedef.TreasuryLevelHigh:
			treasuryBonus = 0.25 // 25%
		case typedef.TreasuryLevelVeryHigh:
			treasuryBonus = 0.30 // 30%
		}
	}

	return 1.0 + treasuryBonus
}

// calculateTreasuryLevel calculates the treasury level based on time since captured
func calculateTreasuryLevel(capturedAt uint64) typedef.TreasuryLevel {
	// Calculate time difference in ticks
	timeSinceCaptured := st.tick - capturedAt

	// Convert ticks to seconds (assuming st.tick is incremented every second)
	secondsSinceCaptured := timeSinceCaptured

	// Calculate time thresholds:
	// Very Low: 0 to 59 minutes 59 seconds (3599 seconds)
	// Low: 1 hour to 23 hours 59 minutes 59 seconds (3600 to 86399 seconds)
	// Medium: 1 day to 4 days 23 hours 59 minutes 59 seconds (86400 to 431999 seconds)
	// High: 5 days to 11 days 23 hours 59 minutes 59 seconds (432000 to 1036799 seconds)
	// Very High: 12 days+ (1036800+ seconds)

	switch {
	case secondsSinceCaptured < 3600: // Less than 1 hour
		return typedef.TreasuryLevelVeryLow
	case secondsSinceCaptured < 86400: // Less than 1 day
		return typedef.TreasuryLevelLow
	case secondsSinceCaptured < 432000: // Less than 5 days
		return typedef.TreasuryLevelMedium
	case secondsSinceCaptured < 1036800: // Less than 12 days
		return typedef.TreasuryLevelHigh
	default: // 12 days or more
		return typedef.TreasuryLevelVeryHigh
	}
}

// updateTreasuryLevel updates a territory's treasury level based on time since captured
func updateTreasuryLevel(territory *typedef.Territory) {
	if territory == nil {
		return
	}

	territory.Mu.Lock()
	defer territory.Mu.Unlock()

	// Don't update treasury for territories with no guild or "No Guild"
	if territory.Guild.Name == "" || territory.Guild.Name == "No Guild" {
		territory.Treasury = typedef.TreasuryLevelVeryLow
		territory.CapturedAt = 0
		return
	}

	// Calculate and update treasury level
	territory.Treasury = calculateTreasuryLevel(territory.CapturedAt)
}

// updateGenerationBonus calculates and updates the territory's GenerationBonus based on treasury
func updateGenerationBonus(territory *typedef.Territory) {
	treasuryMultiplier := calculateTreasury(territory)
	// Convert from multiplier (1.0 + bonus) to percentage bonus
	territory.GenerationBonus = (treasuryMultiplier - 1.0) * 100.0
}
