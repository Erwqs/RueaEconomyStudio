package app

import (
	"fmt"
	"image/color"

	"RueaES/eruntime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// TerritoryModal handles modal overlay for territory menus
type TerritoryModal struct {
	visible          bool             // Whether the modal is currently visible
	alpha            float64          // Current alpha value (0.0 to 1.0)
	targetAlpha      float64          // Target alpha value for animation
	screenWidth      int              // Current screen width
	screenHeight     int              // Current screen height
	edgeMenuPosition EdgeMenuPosition // Position of the associated edge menu
	currentTerritory string           // Name of the currently opened territory
	hovered          bool             // Whether the mouse is currently over the modal
	hiddenByHover    bool             // Whether the modal is hidden due to mouse hover
}

// NewTerritoryModal creates a new territory modal
func NewTerritoryModal() *TerritoryModal {
	return &TerritoryModal{
		visible:          false,
		alpha:            0.0,
		targetAlpha:      0.0,
		edgeMenuPosition: EdgeMenuRight,
	}
}

// Show makes the modal instantly visible
func (tm *TerritoryModal) Show() {
	tm.visible = true
	tm.alpha = 1.0
	tm.targetAlpha = 1.0
	tm.hiddenByHover = false
}

// Hide makes the modal instantly invisible
func (tm *TerritoryModal) Hide() {
	tm.visible = false
	tm.alpha = 0.0
	tm.targetAlpha = 0.0
}

// HideByHover hides the modal and marks it as hidden by hover
func (tm *TerritoryModal) HideByHover() {
	tm.Hide()
	tm.hiddenByHover = true
}

// IsVisible returns whether the modal is currently visible
func (tm *TerritoryModal) IsVisible() bool {
	return tm.visible
}

// SetScreenDimensions updates the screen dimensions
func (tm *TerritoryModal) SetScreenDimensions(width, height int) {
	tm.screenWidth = width
	tm.screenHeight = height
}

// SetEdgeMenuPosition sets the position of the associated edge menu
func (tm *TerritoryModal) SetEdgeMenuPosition(position EdgeMenuPosition) {
	tm.edgeMenuPosition = position
}

// SetCurrentTerritory sets the territory for guild-wide calculations
func (tm *TerritoryModal) SetCurrentTerritory(territoryName string) {
	tm.currentTerritory = territoryName
}

// Update handles the modal logic (no animation)
func (tm *TerritoryModal) Update(deltaTime float64) {
	// Calculate modal bounds (same as in Draw method, but dynamic height)
	// modalWidth := 350
	// rowCount := 1 + 5 // header + 5 resources
	// rowHeight := 18
	// padding := 40 // header, spacing, and bottom margin
	// modalHeight := rowCount*rowHeight + padding

	//var modalX, modalY int
	// switch tm.edgeMenuPosition {
	// case EdgeMenuRight:
	// 	modalX = tm.screenWidth - 450 - modalWidth
	// 	modalY = 50
	// case EdgeMenuLeft:
	// 	modalX = 450
	// 	modalY = 50
	// case EdgeMenuTop:
	// 	modalX = tm.screenWidth - modalWidth - 50
	// 	modalY = 150
	// case EdgeMenuBottom:
	// 	modalX = tm.screenWidth - modalWidth - 50
	// 	modalY = tm.screenHeight - 200 - modalHeight
	// default:
	// 	modalX = tm.screenWidth - 450 - modalWidth
	// 	modalY = 50
	// }

	// mx, my := ebiten.CursorPosition()
	// mouseOver := mx >= modalX && mx < modalX+modalWidth && my >= modalY && my < modalY+modalHeight

	// if mouseOver {
	// 	tm.hovered = true
	// 	if tm.visible {
	// 		tm.HideByHover()
	// 	}
	// } else {
	// 	if tm.hovered || tm.hiddenByHover {
	// 		tm.hovered = false
	// 		if tm.hiddenByHover {
	// 			tm.Show()
	// 		}
	// 	}
	// }
}

// Draw renders the modal overlay
func (tm *TerritoryModal) Draw(screen *ebiten.Image) {
	if !tm.visible || tm.alpha <= 0.01 {
		return
	}

	// Dynamic modal height
	modalWidth := float32(380)
	rowCount := 1 + 5 // header + 5 resources
	rowHeight := float32(18)
	padding := float32(40)
	modalHeight := float32(rowCount)*rowHeight + padding

	baseOpacity := tm.alpha * 0.8 // 80% max opacity
	modalColor := color.RGBA{20, 30, 40, uint8(255.0 * baseOpacity)}
	borderColor := color.RGBA{120, 120, 160, uint8(255.0 * baseOpacity)}

	switch tm.edgeMenuPosition {
	case EdgeMenuRight:
		edgeMenuWidth := float32(400)
		modalX := float32(tm.screenWidth) - edgeMenuWidth - modalWidth - 20
		modalY := float32(10)
		if modalX < 0 {
			modalX = 0
			modalWidth = float32(tm.screenWidth) - edgeMenuWidth - 20
		}
		vector.DrawFilledRect(screen, modalX, modalY, modalWidth, modalHeight, modalColor, false)
		vector.StrokeRect(screen, modalX, modalY, modalWidth, modalHeight, 2, borderColor, false)
		tm.drawResourceTable(screen, int(modalX), int(modalY), int(modalWidth), int(modalHeight))
	case EdgeMenuLeft:
		edgeMenuWidth := float32(400)
		modalX := edgeMenuWidth + 20
		modalY := float32(10)
		if modalX+modalWidth > float32(tm.screenWidth) {
			modalWidth = float32(tm.screenWidth) - modalX
		}
		vector.DrawFilledRect(screen, modalX, modalY, modalWidth, modalHeight, modalColor, false)
		vector.StrokeRect(screen, modalX, modalY, modalWidth, modalHeight, 2, borderColor, false)
		tm.drawResourceTable(screen, int(modalX), int(modalY), int(modalWidth), int(modalHeight))
	case EdgeMenuTop:
		modalWidth := float32(tm.screenWidth)
		modalHeight := float32(rowCount)*rowHeight + padding
		vector.DrawFilledRect(screen, 0, 0, modalWidth, modalHeight, modalColor, false)
		vector.StrokeRect(screen, 0, 0, modalWidth, modalHeight, 2, borderColor, false)
	case EdgeMenuBottom:
		modalWidth := float32(tm.screenWidth)
		modalHeight := float32(rowCount)*rowHeight + padding
		modalY := float32(tm.screenHeight) - modalHeight
		vector.DrawFilledRect(screen, 0, modalY, modalWidth, modalHeight, modalColor, false)
		vector.StrokeRect(screen, 0, modalY, modalWidth, modalHeight, 2, borderColor, false)
	}
}

// drawResourceTable renders the resource usage table inside the modal
func (tm *TerritoryModal) drawResourceTable(screen *ebiten.Image, x, y, width, height int) {
	if tm.currentTerritory == "" {
		return
	}

	// Get the guild of the current territory
	currentTerritory := eruntime.GetTerritory(tm.currentTerritory)
	if currentTerritory == nil {
		return
	}

	guildName := currentTerritory.Guild.Name
	if guildName == "" || guildName == "No Guild" {
		return
	}

	// Get all territories to calculate guild totals
	territories := eruntime.GetTerritories()
	if territories == nil {
		return
	}

	// Calculate totals for each resource (guild-wide)
	var totalProd, totalUsage struct {
		Emerald, Ore, Crop, Wood, Fish float64
	}

	for _, territory := range territories {
		if territory == nil || territory.Guild.Name != guildName {
			continue // Skip territories not owned by this guild
		}

		// Get territory stats
		stats := eruntime.GetTerritoryStats(territory.Name)
		if stats == nil {
			continue
		}

		// Add production
		totalProd.Emerald += stats.CurrentGeneration.Emeralds
		totalProd.Ore += stats.CurrentGeneration.Ores
		totalProd.Crop += stats.CurrentGeneration.Crops
		totalProd.Wood += stats.CurrentGeneration.Wood
		totalProd.Fish += stats.CurrentGeneration.Fish

		// Add usage (costs)
		totalUsage.Emerald += stats.TotalCosts.Emeralds
		totalUsage.Ore += stats.TotalCosts.Ores
		totalUsage.Crop += stats.TotalCosts.Crops
		totalUsage.Wood += stats.TotalCosts.Wood
		totalUsage.Fish += stats.TotalCosts.Fish
	}

	// Add tribute data from guild totals
	// Note: TributeIn and TributeOut are stored as per-minute values, but we need per-hour for display
	// so we multiply by 60 to convert to per-hour

	// Get guild directly from guild manager to ensure we have the latest tribute data
	guild := eruntime.GetGuildByName(guildName)
	if guild != nil {
		totalProd.Emerald += guild.TributeIn.Emeralds * 60
		totalProd.Ore += guild.TributeIn.Ores * 60
		totalProd.Crop += guild.TributeIn.Crops * 60
		totalProd.Wood += guild.TributeIn.Wood * 60
		totalProd.Fish += guild.TributeIn.Fish * 60

		totalUsage.Emerald += guild.TributeOut.Emeralds * 60
		totalUsage.Ore += guild.TributeOut.Ores * 60
		totalUsage.Crop += guild.TributeOut.Crops * 60
		totalUsage.Wood += guild.TributeOut.Wood * 60
		totalUsage.Fish += guild.TributeOut.Fish * 60
	} else {
		fmt.Printf("DEBUG: Guild %s not found in guild manager\n", guildName)
	}

	// Load font
	font := loadWynncraftFont(16)
	if font == nil {
		return
	}

	// Text color with alpha
	textAlpha := tm.alpha
	textColor := color.RGBA{255, 255, 255, uint8(255.0 * textAlpha)}
	headerColor := color.RGBA{255, 215, 0, uint8(255.0 * textAlpha)} // Gold

	// Starting positions
	startX := x + 10
	startY := y + 15
	lineHeight := 18

	// Draw guild header first
	guildHeaderColor := color.RGBA{255, 215, 0, uint8(255.0 * textAlpha)} // Gold
	guildTitle := fmt.Sprintf("Guild: %s", guildName)
	text.Draw(screen, guildTitle, font, startX, startY, guildHeaderColor)
	startY += lineHeight + 5

	// Column positions
	resourceCol := startX
	prodCol := startX + 80
	usageCol := startX + 150
	netCol := startX + 220
	percentCol := startX + 290

	// Draw header
	text.Draw(screen, "Resource", font, resourceCol, startY, headerColor)
	text.Draw(screen, "Prod", font, prodCol, startY, headerColor)
	text.Draw(screen, "Usage", font, usageCol, startY, headerColor)
	text.Draw(screen, "Net", font, netCol, startY, headerColor)
	text.Draw(screen, "%", font, percentCol, startY, headerColor)

	// Draw separator line
	currentY := startY + 5
	separatorColor := color.RGBA{120, 120, 160, uint8(255.0 * textAlpha)}
	vector.DrawFilledRect(screen, float32(startX), float32(currentY), float32(width-20), 1, separatorColor, false)

	currentY += lineHeight

	// Resource data
	resources := []struct {
		name  string
		prod  float64
		usage float64
		color color.RGBA
	}{
		{"Emerald", totalProd.Emerald, totalUsage.Emerald, color.RGBA{144, 238, 144, uint8(255.0 * textAlpha)}},
		{"Ore", totalProd.Ore, totalUsage.Ore, color.RGBA{220, 220, 220, uint8(255.0 * textAlpha)}},
		{"Crop", totalProd.Crop, totalUsage.Crop, color.RGBA{255, 255, 0, uint8(255.0 * textAlpha)}},
		{"Wood", totalProd.Wood, totalUsage.Wood, color.RGBA{139, 69, 19, uint8(255.0 * textAlpha)}},
		{"Fish", totalProd.Fish, totalUsage.Fish, color.RGBA{127, 216, 230, uint8(255.0 * textAlpha)}},
	}

	// Draw each resource row
	for _, resource := range resources {
		net := resource.prod - resource.usage
		var percentage float64
		if resource.prod > 0 {
			percentage = (resource.usage / resource.prod) * 100
		}

		// Format numbers
		prodStr := fmt.Sprintf("%.0f", resource.prod)
		usageStr := fmt.Sprintf("%.0f", resource.usage)
		netStr := fmt.Sprintf("%.0f", net)
		percentStr := fmt.Sprintf("%.1f", percentage)

		// Draw resource name in its color
		text.Draw(screen, resource.name, font, resourceCol, currentY, resource.color)

		// Draw other values in white
		text.Draw(screen, prodStr, font, prodCol, currentY, textColor)
		text.Draw(screen, usageStr, font, usageCol, currentY, textColor)

		// Color code net: green if positive, red if negative
		netColor := textColor
		if net > 0 {
			netColor = color.RGBA{0, 255, 0, uint8(255.0 * textAlpha)} // Green
		} else if net < 0 {
			netColor = color.RGBA{255, 0, 0, uint8(255.0 * textAlpha)} // Red
		}
		text.Draw(screen, netStr, font, netCol, currentY, netColor)

		// Color code percentage: green if low, red if high
		percentColor := textColor
		if percentage > 100 {
			percentColor = color.RGBA{255, 0, 0, uint8(255.0 * textAlpha)} // Red
		} else if percentage > 80 {
			percentColor = color.RGBA{255, 165, 0, uint8(255.0 * textAlpha)} // Orange
		} else if percentage < 50 {
			percentColor = color.RGBA{0, 255, 0, uint8(255.0 * textAlpha)} // Green
		}
		text.Draw(screen, percentStr, font, percentCol, currentY, percentColor)

		currentY += lineHeight
	}
}

// Helper function for absolute value (already exists in other files but keeping it local)
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
