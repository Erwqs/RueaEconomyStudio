package app

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"etools/eruntime"
	"etools/typedef"
)

// populateTransitResourceMenu populates the transit resource menu with current transit information
func (m *MapView) populateTransitResourceMenu() {
	if m.transitResourceMenu == nil {
		return
	}

	// Save current collapsed states before clearing
	collapsedStates := m.transitResourceMenu.SaveCollapsedStates()

	// Clear existing elements
	m.transitResourceMenu.ClearElements()

	// Clear current territory since this is a global menu
	m.transitResourceMenu.ClearCurrentTerritory()

	// Set title
	m.transitResourceMenu.SetTitle("In Transit Resource Inspector")

	// Get all territories
	territories := eruntime.GetAllTerritories()
	if len(territories) == 0 {
		m.transitResourceMenu.Text("No territory data available", DefaultTextOptions())
		return
	}

	// Group territories by those with and without transit resources
	territoriesWithTransit := make([]*typedef.Territory, 0)
	totalTransits := 0

	for _, territory := range territories {
		transitResources := eruntime.GetTransitResourcesForTerritory(territory)
		if len(transitResources) > 0 {
			territoriesWithTransit = append(territoriesWithTransit, territory)
			totalTransits += len(transitResources)
		}
	}

	// Add summary information
	summaryOptions := DefaultTextOptions()
	summaryOptions.Color = color.RGBA{255, 255, 0, 255} // Yellow for summary

	textInputOptions := DefaultTextInputOptions()
	textInputOptions.Width = 300 // Set a reasonable width for the input
	textInputOptions.Height = 20
	textInputOptions.Placeholder = "Type to filter transits..."

	// Create the text input and auto-focus it for better UX
	m.transitResourceMenu.TextInput("Filter", m.transitResourceFilter, textInputOptions, func(input string) {
		// Store the filter string and refresh the menu
		m.transitResourceFilter = input
		// Refresh the menu with the new filter
		m.populateTransitResourceMenu()
	}).SetFocusOnLastTextInput()

	// Add filter help text
	helpOptions := DefaultTextOptions()
	helpOptions.Color = color.RGBA{180, 180, 180, 255} // Gray for help text
	helpOptions.FontSize = 12
	// m.transitResourceMenu.Text("Filter examples: origin:pirate dest:detlas at:maltic emerald>1000 ore:500", helpOptions)

	if len(territoriesWithTransit) == 0 {
		m.transitResourceMenu.Text("No resources in transit", DefaultTextOptions())
		m.transitResourceMenu.RestoreCollapsedStates(collapsedStates)
		return
	}

	// Sort territories by name for consistent display
	sort.Slice(territoriesWithTransit, func(i, j int) bool {
		return territoriesWithTransit[i].Name < territoriesWithTransit[j].Name
	})

	// Parse the current filter criteria
	filterCriteria := parseTransitFilter(m.transitResourceFilter)

	// Create a horizontal scrolling container for cards
	container := m.transitResourceMenu.Container()

	// Track filtered results for summary
	filteredTransitCount := 0

	// Create cards for each transit resource
	for _, territory := range territoriesWithTransit {
		transitResources := eruntime.GetTransitResourcesForTerritory(territory)
		if len(transitResources) == 0 {
			continue
		}

		// Add each transit as a separate card to the container
		for _, transit := range transitResources {
			// Apply filter
			if !matchesTransitFilter(&transit, territory, filterCriteria) {
				continue
			}

			filteredTransitCount++
			// Create card for this transit
			card := NewCard()

			// Make sure card is visible
			card.SetVisible(true)

			// Add card to the horizontal container
			container.Add(card)

			// Origin and destination info
			originName := "Unknown"
			if transit.Origin != nil {
				originName = transit.Origin.Name
			}

			// Card title with route info - make it ultra compact
			titleOptions := DefaultTextOptions()
			titleOptions.Color = color.RGBA{255, 255, 255, 255} // White for title
			titleOptions.FontSize = 10                          // Ultra small font
			card.Text(fmt.Sprintf("From: %s", originName), titleOptions)
			card.Text(fmt.Sprintf("To: %s", originName), titleOptions)

			// Current location - ultra compact
			locationOptions := DefaultTextOptions()
			locationOptions.Color = color.RGBA{255, 165, 0, 255} // Orange
			locationOptions.FontSize = 9
			card.Text(fmt.Sprintf("At %s", territory.Name), locationOptions)

			// Show only the main resources being transported - ultra compact
			resources := transit.BasicResources

			emeraldOptions := DefaultTextOptions()
			emeraldOptions.Color = color.RGBA{0, 255, 0, 255} // Green
			emeraldOptions.FontSize = 9
			card.Text(fmt.Sprintf("E %.0f", resources.Emeralds), emeraldOptions)

			oreOptions := DefaultTextOptions()
			oreOptions.Color = color.RGBA{180, 180, 180, 255} // Light grey
			oreOptions.FontSize = 9
			card.Text(fmt.Sprintf("O %.0f", resources.Ores), oreOptions)

			woodOptions := DefaultTextOptions()
			woodOptions.Color = color.RGBA{139, 69, 19, 255} // Brown
			woodOptions.FontSize = 9
			card.Text(fmt.Sprintf("W %.0f", resources.Wood), woodOptions)

			fishOptions := DefaultTextOptions()
			fishOptions.Color = color.RGBA{0, 150, 255, 255} // Blue
			fishOptions.FontSize = 9
			card.Text(fmt.Sprintf("F %.0f", resources.Fish), fishOptions)

			cropOptions := DefaultTextOptions()
			cropOptions.Color = color.RGBA{255, 255, 0, 255} // Yellow
			cropOptions.FontSize = 9
			card.Text(fmt.Sprintf("C %.0f", resources.Crops), cropOptions)

			// Compact status
			if transit.Next != nil {
				nextOptions := DefaultTextOptions()
				nextOptions.Color = color.RGBA{255, 165, 0, 255} // Orange
				nextOptions.FontSize = 8
				card.Text(fmt.Sprintf("→ %s", transit.Next.Name), nextOptions)
			} else {
				arrivalOptions := DefaultTextOptions()
				arrivalOptions.Color = color.RGBA{0, 255, 255, 255} // Cyan
				arrivalOptions.FontSize = 8
				card.Text("Arriving", arrivalOptions)
			}

			// Single action button - ultra compact
			buttonOptions := DefaultButtonOptions()
			buttonOptions.Height = 18 // Ultra small button
			buttonOptions.FontSize = 8
			// Red
			buttonOptions.BackgroundColor = color.RGBA{200, 50, 50, 255} // Red
			buttonOptions.BorderColor = color.RGBA{255, 100, 100, 255}   // Light red border
			buttonOptions.HoverColor = color.RGBA{255, 150, 150, 255}    // Lighter red on hover
			buttonOptions.PressedColor = color.RGBA{255, 200, 200, 255}  // Even lighter red when pressed
			card.Button("Void", buttonOptions, func() {
				// Void resource from transit system

			})

		}
	}

	// Add summary information showing filtered results
	// summaryText := fmt.Sprintf("Showing %d/%d transits", filteredTransitCount, totalTransits)
	// if m.transitResourceFilter != "" {
	// 	summaryText += fmt.Sprintf(" (filtered by: %s)", m.transitResourceFilter)
	// }
	// m.transitResourceMenu.Text(summaryText, summaryOptions)

	// Show message if no transits match the filter
	if filteredTransitCount == 0 && totalTransits > 0 && m.transitResourceFilter != "" {
		noMatchOptions := DefaultTextOptions()
		noMatchOptions.Color = color.RGBA{255, 200, 100, 255} // Orange for no match message
		m.transitResourceMenu.Text("No transits match the current filter criteria", noMatchOptions)
	}

	// Restore collapsed states
	m.transitResourceMenu.RestoreCollapsedStates(collapsedStates)
}

// FilterCriteria represents parsed filter criteria
type FilterCriteria struct {
	Origin      string
	Destination string
	Location    string
	Emeralds    *ResourceFilter
	Ores        *ResourceFilter
	Wood        *ResourceFilter
	Fish        *ResourceFilter
	Crops       *ResourceFilter
}

// ResourceFilter represents a resource amount filter
type ResourceFilter struct {
	Operator string // "=", ">", "<", ">=", "<="
	Value    float64
}

// parseTransitFilter parses a filter string into structured criteria
func parseTransitFilter(filterStr string) *FilterCriteria {
	if filterStr == "" {
		return nil
	}

	criteria := &FilterCriteria{}

	// Split by spaces and process each token
	tokens := strings.Fields(strings.ToLower(filterStr))

	for _, token := range tokens {
		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "origin", "from":
				criteria.Origin = value
			case "dest", "destination", "to":
				criteria.Destination = value
			case "at", "location", "loc":
				criteria.Location = value
			case "emerald", "emeralds", "e":
				criteria.Emeralds = parseResourceFilter(value)
			case "ore", "ores", "o":
				criteria.Ores = parseResourceFilter(value)
			case "wood", "w":
				criteria.Wood = parseResourceFilter(value)
			case "fish", "f":
				criteria.Fish = parseResourceFilter(value)
			case "crop", "crops", "c":
				criteria.Crops = parseResourceFilter(value)
			}
		} else {
			// If no tag is specified, default to location filter (at:)
			if criteria.Location == "" {
				criteria.Location = token
			} else {
				// If location is already set, append with space for multiple terms
				criteria.Location = criteria.Location + " " + token
			}
		}
	}

	return criteria
}

// parseResourceFilter parses a resource filter value (e.g., ">1000", "500", "<=200")
func parseResourceFilter(valueStr string) *ResourceFilter {
	if valueStr == "" {
		return nil
	}

	filter := &ResourceFilter{Operator: "="}

	// Check for operators
	if strings.HasPrefix(valueStr, ">=") {
		filter.Operator = ">="
		valueStr = strings.TrimPrefix(valueStr, ">=")
	} else if strings.HasPrefix(valueStr, "<=") {
		filter.Operator = "<="
		valueStr = strings.TrimPrefix(valueStr, "<=")
	} else if strings.HasPrefix(valueStr, ">") {
		filter.Operator = ">"
		valueStr = strings.TrimPrefix(valueStr, ">")
	} else if strings.HasPrefix(valueStr, "<") {
		filter.Operator = "<"
		valueStr = strings.TrimPrefix(valueStr, "<")
	}

	// Parse the numeric value
	if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
		filter.Value = value
		return filter
	}

	return nil
}

// matchesFilter checks if a transit resource matches the filter criteria
func matchesTransitFilter(transit *typedef.InTransitResources, territory *typedef.Territory, criteria *FilterCriteria) bool {
	if criteria == nil {
		return true
	}

	// Check origin filter
	if criteria.Origin != "" {
		originName := "unknown"
		if transit.Origin != nil {
			originName = strings.ToLower(transit.Origin.Name)
		}
		if !strings.Contains(originName, criteria.Origin) {
			return false
		}
	}

	// Check destination filter
	if criteria.Destination != "" {
		destName := "unknown"
		if transit.Destination != nil {
			destName = strings.ToLower(transit.Destination.Name)
		}
		if !strings.Contains(destName, criteria.Destination) {
			return false
		}
	}

	// Check location filter (current territory)
	if criteria.Location != "" {
		locationName := strings.ToLower(territory.Name)
		if !strings.Contains(locationName, criteria.Location) {
			return false
		}
	}

	// Check resource amount filters
	resources := transit.BasicResources

	if criteria.Emeralds != nil && !matchesResourceAmount(resources.Emeralds, criteria.Emeralds) {
		return false
	}
	if criteria.Ores != nil && !matchesResourceAmount(resources.Ores, criteria.Ores) {
		return false
	}
	if criteria.Wood != nil && !matchesResourceAmount(resources.Wood, criteria.Wood) {
		return false
	}
	if criteria.Fish != nil && !matchesResourceAmount(resources.Fish, criteria.Fish) {
		return false
	}
	if criteria.Crops != nil && !matchesResourceAmount(resources.Crops, criteria.Crops) {
		return false
	}

	return true
}

// matchesResourceAmount checks if a resource amount matches the filter
func matchesResourceAmount(amount float64, filter *ResourceFilter) bool {
	switch filter.Operator {
	case "=":
		return amount == filter.Value
	case ">":
		return amount > filter.Value
	case "<":
		return amount < filter.Value
	case ">=":
		return amount >= filter.Value
	case "<=":
		return amount <= filter.Value
	default:
		return false
	}
}
