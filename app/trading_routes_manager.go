package app

import (
	"fmt"
	"image/color"

	"RueaES/eruntime"
)

// UpdateTradingRoutes updates only the Trading Routes section with new territory data
func (m *EdgeMenu) UpdateTradingRoutes(territoryName string) {
	// fmt.Printf("DEBUG: UpdateTradingRoutes called for territory: %s\n", territoryName)
	// Get fresh territory data from eruntime
	territory := eruntime.GetTerritory(territoryName)
	if territory == nil {
		return
	}

	// Find the "Trading Routes" collapsible menu and update its contents
	for _, element := range m.elements {
		if collapsible, ok := element.(*CollapsibleMenu); ok {
			if collapsible.title == "Trading Routes" {
				// Save the expanded/collapsed state of existing route submenus
				routeStates := make(map[string]bool)
				for _, existingElement := range collapsible.elements {
					if existingSubmenu, ok := existingElement.(*CollapsibleMenu); ok {
						// Store the expanded state using the route title as key
						isExpanded := !existingSubmenu.collapsed
						routeStates[existingSubmenu.title] = isExpanded
						// Also save to the persistent map for when menu is closed and reopened
						m.tradingRouteStates[existingSubmenu.title] = isExpanded
					}
				}

				// Clear existing elements and rebuild with fresh data
				collapsible.elements = collapsible.elements[:0]

				if len(territory.TradingRoutes) > 0 {
					// Get current guild information for route coloring
					currentGuild := territory.Guild
					allies := eruntime.GetAllies()

					for _, route := range territory.TradingRoutes {
						if len(route) == 0 {
							continue
						}

						// Get destination (last territory in route)
						destination := route[len(route)-1].Name

						// Format route title with route tax percentage
						routeTitle := ""
						if territory.RouteTax >= 0 {
							routeTaxPercentage := int(territory.RouteTax * 100)
							routeTitle = fmt.Sprintf("Route to %s (%d%%)", destination, routeTaxPercentage)
						} else {
							// If route tax is invalid (-1.0), don't show percentage
							routeTitle = fmt.Sprintf("Route to %s", destination)
						}

						// Create nested collapsible menu for this route
						routeOptions := DefaultCollapsibleMenuOptions()
						// Restore the previous expanded/collapsed state
						// First check current session states, then persistent states, default to collapsed
						wasExpanded := false
						if expanded, exists := routeStates[routeTitle]; exists {
							wasExpanded = expanded
						} else if expanded, exists := m.tradingRouteStates[routeTitle]; exists {
							wasExpanded = expanded
						}
						routeOptions.Collapsed = !wasExpanded
						routeSubmenu := collapsible.CollapsibleMenu(routeTitle, routeOptions)

						for j, routeTerritory := range route {
							routeTerritoryName := routeTerritory.Name

							// Determine arrow color based on ownership
							var arrowColor color.RGBA
							var taxText string

							if routeTerritory.Guild.Name == currentGuild.Name {
								// We own this territory - bright blue
								arrowColor = color.RGBA{0, 150, 255, 255} // Bright blue
							} else {
								// Check if it's an ally
								isAlly := false
								if currentGuildAllies, exists := allies[&currentGuild]; exists {
									for _, ally := range currentGuildAllies {
										if ally.Name == routeTerritory.Guild.Name {
											isAlly = true
											break
										}
									}
								}

								if isAlly {
									// Allied territory - green
									arrowColor = color.RGBA{0, 255, 0, 255} // Green
								} else {
									// Not allied - red
									arrowColor = color.RGBA{255, 0, 0, 255} // Red
								}
							}

							// Add tax percentage only if not owned by us and has tax
							if routeTerritory.Guild.Name != currentGuild.Name {
								taxPercentage := int(routeTerritory.Tax.Tax * 100)
								if taxPercentage > 0 {
									taxText = fmt.Sprintf(" (%d%%)", taxPercentage)
								}
							}

							// Create the text with arrow
							var displayText string
							if j == 0 {
								displayText = fmt.Sprintf("→ %s%s", routeTerritoryName, taxText)
							} else {
								displayText = fmt.Sprintf("→ %s%s", routeTerritoryName, taxText)
							}

							// Create clickable text with the arrow color
							clickableOptions := DefaultTextOptions()
							clickableOptions.Color = arrowColor

							// Capture territory name for closure
							capturedTerritoryName := routeTerritoryName
							routeSubmenu.ClickableText(displayText, clickableOptions, func() {
								// Use the territory navigation callback if available
								if m.territoryNavCallback != nil {
									m.territoryNavCallback(capturedTerritoryName)
								} else {
									// fmt.Printf("DEBUG: Route territory clicked (no nav callback): %s\n", capturedTerritoryName)
								}
							})
						}
					}
				} else {
					// No trading routes available
					noRoutesText := NewMenuText("No trading routes", DefaultTextOptions())
					collapsible.elements = append(collapsible.elements, noRoutesText)
				}

				// fmt.Printf("DEBUG: Trading routes updated in place for %s (preserved %d route states)\n", territoryName, len(routeStates))
				return // Found and updated, exit
			}
		}
	}
}
