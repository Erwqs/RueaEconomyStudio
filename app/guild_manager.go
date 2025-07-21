package app

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strings"

	"etools/eruntime"
	"etools/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// GuildData represents a guild entry
type GuildData struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

// GuildManager handles the guild management UI and functionality
type GuildManager struct {
	visible         bool
	modal           *UIModalExtended
	nameInput       *UITextInputExtended
	tagInput        *UITextInputExtended
	guilds          []GuildData
	filteredGuilds  []GuildData
	scrollOffset    int
	hoveredIndex    int
	selectedIndex   int
	guildFilePath   string
	framesAfterOpen int // Frames since the manager was opened (to prevent immediate ESC closing)
}

// NewGuildManager creates a new guild manager
func NewGuildManager() *GuildManager {
	screenW, screenH := ebiten.WindowSize()
	modalWidth := 500
	modalHeight := 400
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	modal := NewUIModalExtended("Guild Management", modalX, modalY, modalWidth, modalHeight)

	nameInput := NewUITextInputExtended("Search or add guild name...", modalX+20, modalY+60, 300, 50)
	tagInput := NewUITextInputExtended("Tag...", modalX+330, modalY+60, 150, 10)

	gm := &GuildManager{
		visible:        false,
		modal:          modal,
		nameInput:      nameInput,
		tagInput:       tagInput,
		guilds:         []GuildData{},
		filteredGuilds: []GuildData{},
		scrollOffset:   0,
		hoveredIndex:   -1,
		selectedIndex:  -1,
		guildFilePath:  "guilds.json",
	}

	// Load guilds from file
	gm.loadGuildsFromFile()

	return gm
}

// Show makes the guild manager visible
func (gm *GuildManager) Show() {
	gm.visible = true
	gm.framesAfterOpen = 0 // Reset frame counter when opening
}

// Hide makes the guild manager invisible
func (gm *GuildManager) Hide() {
	gm.visible = false
	gm.nameInput.ClearFocus()
	gm.tagInput.ClearFocus()
}

// IsVisible returns true if the guild manager is visible
func (gm *GuildManager) IsVisible() bool {
	return gm.visible
}

// Update handles input and updates the guild manager state
func (gm *GuildManager) Update() {
	if !gm.visible {
		return
	}

	// Increment frame counter
	gm.framesAfterOpen++

	// Get mouse position
	mx, my := ebiten.CursorPosition()

	// Check if clicked outside the modal to close it
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !gm.modal.Contains(mx, my) {
			gm.Hide()
			return
		}
	}

	// Close on ESC key, but only after a few frames to prevent immediate closing
	// from queued ESC key presses
	if gm.framesAfterOpen > 5 && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		gm.Hide()
		return
	}

	// Update input fields
	nameChanged := gm.nameInput.Update()
	tagChanged := gm.tagInput.Update()

	// If any of the search fields changed, update filtered guilds
	if nameChanged || tagChanged {
		gm.filterGuilds()
	}

	// Handle Tab key for switching between inputs
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if gm.nameInput.IsFocused() {
			gm.nameInput.ClearFocus()
			gm.tagInput.SetFocus()
		} else if gm.tagInput.IsFocused() {
			gm.tagInput.ClearFocus()
			gm.nameInput.SetFocus()
		} else {
			gm.nameInput.SetFocus()
		}
	}

	// Handle Enter key to add guild
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if gm.nameInput.IsFocused() || gm.tagInput.IsFocused() {
			gm.addGuild(gm.nameInput.GetText(), gm.tagInput.GetText())
		}
	}

	// Reset hovered index
	gm.hoveredIndex = -1

	// Check for hovering/clicking on guild items
	if len(gm.filteredGuilds) > 0 {
		itemHeight := 30
		listStartY := gm.modal.Y + 100

		// Calculate which items are visible based on scroll offset
		maxVisibleItems := (gm.modal.Height - 130) / itemHeight

		for i := 0; i < len(gm.filteredGuilds) && i < maxVisibleItems; i++ {
			itemIndex := i + gm.scrollOffset
			if itemIndex >= len(gm.filteredGuilds) {
				break
			}

			itemY := listStartY + (i * itemHeight)

			// Guild item area
			itemRect := Rect{
				X:      gm.modal.X + 20,
				Y:      itemY,
				Width:  gm.modal.Width - 70,
				Height: itemHeight,
			}

			// Remove button area
			removeRect := Rect{
				X:      gm.modal.X + gm.modal.Width - 40,
				Y:      itemY,
				Width:  20,
				Height: itemHeight,
			}

			// Check if hovering over item
			if mx >= itemRect.X && mx < itemRect.X+itemRect.Width &&
				my >= itemRect.Y && my < itemRect.Y+itemRect.Height {
				gm.hoveredIndex = itemIndex

				// Handle click on item
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					gm.selectedIndex = itemIndex
				}
			}

			// Check if hovering over remove button
			if mx >= removeRect.X && mx < removeRect.X+removeRect.Width &&
				my >= removeRect.Y && my < removeRect.Y+removeRect.Height {

				// Handle click on remove button
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					gm.removeGuild(itemIndex)
				}
			}
		}
	}

	// Handle mouse wheel for scrolling
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		gm.scrollOffset -= int(wheelY * 3)
		if gm.scrollOffset < 0 {
			gm.scrollOffset = 0
		}
		maxOffset := len(gm.filteredGuilds) - ((gm.modal.Height - 130) / 30)
		if maxOffset < 0 {
			maxOffset = 0
		}
		if gm.scrollOffset > maxOffset {
			gm.scrollOffset = maxOffset
		}
	}

	// Handle add button click
	addButtonRect := Rect{
		X:      gm.modal.X + gm.modal.Width - 60,
		Y:      gm.modal.Y + 60,
		Width:  40,
		Height: 30,
	}

	if mx >= addButtonRect.X && mx < addButtonRect.X+addButtonRect.Width &&
		my >= addButtonRect.Y && my < addButtonRect.Y+addButtonRect.Height {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			gm.addGuild(gm.nameInput.GetText(), gm.tagInput.GetText())
		}
	}
}

// filterGuilds filters the guild list based on name and tag inputs
func (gm *GuildManager) filterGuilds() {
	nameFilter := strings.ToLower(gm.nameInput.GetText())
	tagFilter := strings.ToLower(gm.tagInput.GetText())

	// If both filters are empty, show all guilds
	if nameFilter == "" && tagFilter == "" {
		gm.filteredGuilds = gm.guilds
		return
	}

	// Filter guilds based on name and tag
	gm.filteredGuilds = []GuildData{}
	for _, guild := range gm.guilds {
		nameMatch := nameFilter == "" || strings.Contains(strings.ToLower(guild.Name), nameFilter)
		tagMatch := tagFilter == "" || strings.Contains(strings.ToLower(guild.Tag), tagFilter)

		if nameMatch && tagMatch {
			gm.filteredGuilds = append(gm.filteredGuilds, guild)
		}
	}
}

// addGuild adds a new guild to the list
func (gm *GuildManager) addGuild(name, tag string) {
	name = strings.TrimSpace(name)
	tag = strings.TrimSpace(tag)

	// Validate inputs
	if name == "" || tag == "" {
		return
	}

	// Check for duplicates
	for _, guild := range gm.guilds {
		if strings.EqualFold(guild.Name, name) && strings.EqualFold(guild.Tag, tag) {
			return // Already exists
		}
	}

	// Add new guild
	newGuild := GuildData{
		Name: name,
		Tag:  tag,
	}

	gm.guilds = append(gm.guilds, newGuild)
	gm.filterGuilds() // Update filtered list

	// Clear inputs
	gm.nameInput.SetText("")
	gm.tagInput.SetText("")
	gm.nameInput.SetFocus()

	// Save to file
	gm.saveGuildsToFile()
}

// removeGuild removes a guild from the list
func (gm *GuildManager) removeGuild(index int) {
	if index < 0 || index >= len(gm.filteredGuilds) {
		return
	}

	// Find the guild in the main list
	guildToRemove := gm.filteredGuilds[index]
	for i, guild := range gm.guilds {
		if guild.Name == guildToRemove.Name && guild.Tag == guildToRemove.Tag {
			// Remove from main list
			gm.guilds = append(gm.guilds[:i], gm.guilds[i+1:]...)
			break
		}
	}

	// Update filtered list
	gm.filterGuilds()

	// Save to file
	gm.saveGuildsToFile()
}

// loadGuildsFromFile loads guilds from a JSON file
func (gm *GuildManager) loadGuildsFromFile() {
	data, err := os.ReadFile(gm.guildFilePath)
	if err != nil {
		// File doesn't exist, start with empty guilds list
		return
	}

	err = json.Unmarshal(data, &gm.guilds)
	if err != nil {
		// Invalid JSON, start fresh
		gm.guilds = []GuildData{}
	}

	// Initialize filtered guilds
	gm.filteredGuilds = gm.guilds
}

// saveGuildsToFile saves guilds to a JSON file
func (gm *GuildManager) saveGuildsToFile() {
	data, err := json.MarshalIndent(gm.guilds, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling guilds:", err)
		return
	}

	err = os.WriteFile(gm.guildFilePath, data, 0644)
	if err != nil {
		fmt.Println("Error writing guilds file:", err)
	}
}

// GuildClaim represents a territory claimed by a guild
type GuildClaim struct {
	TerritoryName string `json:"territory"`
	GuildName     string `json:"guild_name"`
	GuildTag      string `json:"guild_tag"`
}

// GuildClaimManager handles persistent guild territory claims
type GuildClaimManager struct {
	// Map of territory name to guild claim
	Claims map[string]GuildClaim `json:"claims"`
	// Path to the file where claims are saved
	ClaimsFilePath string `json:"-"`
	// Flag to suspend automatic redraws during batch operations
	suspendRedraws bool `json:"-"`
}

// NewGuildClaimManager creates a new guild claim manager
func NewGuildClaimManager() *GuildClaimManager {
	manager := &GuildClaimManager{
		Claims:         make(map[string]GuildClaim),
		ClaimsFilePath: "territory_claims.json",
	}

	// Load existing claims
	manager.LoadClaimsFromFile()

	return manager
}

// AddClaim adds or updates a territory claim
func (gcm *GuildClaimManager) AddClaim(territoryName, guildName, guildTag string) {
	gcm.Claims[territoryName] = GuildClaim{
		TerritoryName: territoryName,
		GuildName:     guildName,
		GuildTag:      guildTag,
	}

	// Update the eruntime system with the new guild ownership
	guild := typedef.Guild{
		Name:   guildName,
		Tag:    guildTag,
		Allies: nil, // Initialize as empty - allies will be handled separately
	}

	// Notify the eruntime that this territory now belongs to this guild
	updatedTerritory := eruntime.SetGuild(territoryName, guild)
	if updatedTerritory != nil {
		fmt.Printf("[GUILD_MANAGER] Successfully updated eruntime for territory %s -> guild %s [%s]\n",
			territoryName, guildName, guildTag)
	} else {
		fmt.Printf("[GUILD_MANAGER] Warning: Failed to update eruntime for territory %s\n", territoryName)
	}

	// Save changes
	gcm.SaveClaimsToFile()

	// Trigger a redraw to show the claim immediately, unless suspended
	if !gcm.suspendRedraws {
		gcm.TriggerRedraw()
	}
}

// RemoveClaim removes a territory claim
func (gcm *GuildClaimManager) RemoveClaim(territoryName string) {
	delete(gcm.Claims, territoryName)

	// Update the eruntime system to remove guild ownership (set to empty guild)
	emptyGuild := typedef.Guild{
		Name:   "",
		Tag:    "",
		Allies: nil,
	}

	// Notify the eruntime that this territory no longer belongs to any guild
	updatedTerritory := eruntime.SetGuild(territoryName, emptyGuild)
	if updatedTerritory != nil {
		fmt.Printf("[GUILD_MANAGER] Successfully removed guild ownership from territory %s in eruntime\n", territoryName)
	} else {
		fmt.Printf("[GUILD_MANAGER] Warning: Failed to update eruntime when removing claim for territory %s\n", territoryName)
	}

	// Save changes
	gcm.SaveClaimsToFile()

	// Trigger a redraw to show the change immediately, unless suspended
	if !gcm.suspendRedraws {
		gcm.TriggerRedraw()
	}
}

// TriggerRedraw forces a redraw of all territory renderers
func (gcm *GuildClaimManager) TriggerRedraw() {
	// Find all territory managers that might need updating
	// For now, just invalidate any visible territory cache
	// fmt.Println("[DEBUG] TriggerRedraw called for GuildClaimManager")
	if app := GetCurrentApp(); app != nil {
		// fmt.Println("[DEBUG] Got app instance")
		if gameplayModule := app.GetGameplayModule(); gameplayModule != nil {
			// fmt.Println("[DEBUG] Got gameplay module")
			if mapView := gameplayModule.GetMapView(); mapView != nil {
				// fmt.Println("[DEBUG] Got map view")
				if tm := mapView.GetTerritoriesManager(); tm != nil {
					// fmt.Println("[DEBUG] Got territories manager")
					// First reload the persistent claims from file
					if err := tm.ReloadClaims(); err != nil {
						// fmt.Printf("[DEBUG] Error reloading claims: %v\n", err)
					} else {
						// fmt.Println("[DEBUG] Successfully reloaded claims")
					}
					// Then invalidate the cache to force a redraw
					if renderer := tm.GetRenderer(); renderer != nil {
						// fmt.Println("[DEBUG] Got renderer")
						if cache := renderer.GetTerritoryCache(); cache != nil {
							// fmt.Println("[DEBUG] Got cache, calling ForceRedraw")
							cache.ForceRedraw()
						} else {
							// fmt.Println("[DEBUG] Cache is nil")
						}
					} else {
						// fmt.Println("[DEBUG] Renderer is nil")
					}
				} else {
					// fmt.Println("[DEBUG] Territories manager is nil")
				}
			} else {
				// fmt.Println("[DEBUG] Map view is nil")
			}
		} else {
			// fmt.Println("[DEBUG] Gameplay module is nil")
		}
	} else {
		// fmt.Println("[DEBUG] App instance is nil")
	}
}

// GetClaim gets a territory claim
func (gcm *GuildClaimManager) GetClaim(territoryName string) (GuildClaim, bool) {
	claim, exists := gcm.Claims[territoryName]
	return claim, exists
}

// HasClaim checks if a territory is claimed
func (gcm *GuildClaimManager) HasClaim(territoryName string) bool {
	_, exists := gcm.Claims[territoryName]
	return exists
}

// GetClaimsForGuild returns a map of territory names that are claimed by the specified guild
func (gcm *GuildClaimManager) GetClaimsForGuild(guildName, guildTag string) map[string]bool {
	guildClaims := make(map[string]bool)

	for territoryName, claim := range gcm.Claims {
		if claim.GuildName == guildName && claim.GuildTag == guildTag {
			guildClaims[territoryName] = true
		}
	}

	return guildClaims
}

// SaveClaimsToFile saves the claims to a JSON file
func (gcm *GuildClaimManager) SaveClaimsToFile() error {
	// Convert map to slice for better JSON formatting
	claimsList := make([]GuildClaim, 0, len(gcm.Claims))
	for _, claim := range gcm.Claims {
		claimsList = append(claimsList, claim)
	}

	// Marshal the claims to JSON
	data, err := json.MarshalIndent(claimsList, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling claims: %v\n", err)
		return err
	}

	// Write to file
	err = os.WriteFile(gcm.ClaimsFilePath, data, 0644)
	if err != nil {
		fmt.Printf("Error writing claims to file: %v\n", err)
		return err
	}

	return nil
}

// LoadClaimsFromFile loads the claims from a JSON file
func (gcm *GuildClaimManager) LoadClaimsFromFile() error {
	// Check if file exists
	if _, err := os.Stat(gcm.ClaimsFilePath); os.IsNotExist(err) {
		// File doesn't exist, just return without error
		return nil
	}

	// Read the file
	data, err := os.ReadFile(gcm.ClaimsFilePath)
	if err != nil {
		fmt.Printf("Error reading claims file: %v\n", err)
		return err
	}

	// Unmarshal the claims
	var claimsList []GuildClaim
	err = json.Unmarshal(data, &claimsList)
	if err != nil {
		fmt.Printf("Error unmarshaling claims: %v\n", err)
		return err
	}

	// Use batch function for all claims
	gcm.AddClaimsBatch(claimsList)

	fmt.Printf("[GUILD_MANAGER] Loaded %d claims and batch synchronized with eruntime\n", len(claimsList))
	return nil
}

// Global instance for singleton access
var guildClaimManagerInstance *GuildClaimManager

// GetGuildClaimManager returns the singleton instance
func GetGuildClaimManager() *GuildClaimManager {
	if guildClaimManagerInstance == nil {
		guildClaimManagerInstance = NewGuildClaimManager()
	}
	return guildClaimManagerInstance
}

// PrintClaims prints all current territory claims for debugging
func (gcm *GuildClaimManager) PrintClaims() {
	fmt.Printf("===== Territory Claims =====\n")
	if len(gcm.Claims) == 0 {
		fmt.Printf("No territories claimed.\n")
	} else {
		for territoryName, claim := range gcm.Claims {
			fmt.Printf("Territory: %s, Guild: %s [%s]\n",
				territoryName, claim.GuildName, claim.GuildTag)
		}
	}
	fmt.Printf("===========================\n")
}

// AddClaimsBatch sets multiple claims at once and synchronizes with eruntime efficiently.
func (gcm *GuildClaimManager) AddClaimsBatch(claims []GuildClaim) {
	gcm.suspendRedraws = true

	guildUpdates := make(map[string]*typedef.Guild)
	for _, claim := range claims {
		gcm.Claims[claim.TerritoryName] = claim
		guild := &typedef.Guild{
			Name:   claim.GuildName,
			Tag:    claim.GuildTag,
			Allies: nil,
		}
		guildUpdates[claim.TerritoryName] = guild
	}

	if len(guildUpdates) > 0 {
		updatedTerritories := eruntime.SetGuildBatch(guildUpdates)
		successCount := len(updatedTerritories)
		fmt.Printf("[GUILD_MANAGER] Batch synchronized %d/%d claims with eruntime\n",
			successCount, len(guildUpdates))
	}

	gcm.SaveClaimsToFile()
	gcm.suspendRedraws = false
	gcm.TriggerRedraw()
}

// Draw renders the guild manager
func (gm *GuildManager) Draw(screen *ebiten.Image) {
	if !gm.visible {
		return
	}

	// Draw modal background
	gm.modal.Draw(screen)

	// Draw text inputs
	font := GetDefaultFont(16)
	text.Draw(screen, "Name:", font, gm.nameInput.X-60, gm.nameInput.Y+20, color.White)
	text.Draw(screen, "Tag:", font, gm.tagInput.X-40, gm.tagInput.Y+20, color.White)

	gm.nameInput.Draw(screen)
	gm.tagInput.Draw(screen)

	// Draw add button
	addButtonRect := Rect{
		X:      gm.modal.X + gm.modal.Width - 60,
		Y:      gm.modal.Y + 60,
		Width:  40,
		Height: 30,
	}

	addButtonColor := color.RGBA{100, 200, 100, 255}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if mx >= addButtonRect.X && mx < addButtonRect.X+addButtonRect.Width &&
			my >= addButtonRect.Y && my < addButtonRect.Y+addButtonRect.Height {
			addButtonColor = color.RGBA{80, 180, 80, 255}
		}
	}

	vector.DrawFilledRect(screen, float32(addButtonRect.X), float32(addButtonRect.Y),
		float32(addButtonRect.Width), float32(addButtonRect.Height), addButtonColor, false)

	addText := "Add"
	addTextBounds := text.BoundString(font, addText)
	text.Draw(screen, addText,
		font,
		addButtonRect.X+(addButtonRect.Width-addTextBounds.Dx())/2,
		addButtonRect.Y+(addButtonRect.Height+addTextBounds.Dy())/2-2,
		color.White)

	// Draw guild list heading
	listHeading := fmt.Sprintf("Guilds (%d)", len(gm.filteredGuilds))
	text.Draw(screen, listHeading, font, gm.modal.X+20, gm.modal.Y+95, color.White)

	// Draw guild list
	if len(gm.filteredGuilds) > 0 {
		itemHeight := 30
		listStartY := gm.modal.Y + 100

		// Calculate which items are visible based on scroll offset
		maxVisibleItems := (gm.modal.Height - 130) / itemHeight

		for i := 0; i < len(gm.filteredGuilds) && i < maxVisibleItems; i++ {
			itemIndex := i + gm.scrollOffset
			if itemIndex >= len(gm.filteredGuilds) {
				break
			}

			guild := gm.filteredGuilds[itemIndex]
			itemY := listStartY + (i * itemHeight)

			// Determine item background color
			bgColor := color.RGBA{40, 40, 40, 255}
			if itemIndex == gm.hoveredIndex {
				bgColor = color.RGBA{60, 60, 60, 255}
			}
			if itemIndex == gm.selectedIndex {
				bgColor = color.RGBA{80, 80, 120, 255}
			}

			// Draw item background
			vector.DrawFilledRect(screen,
				float32(gm.modal.X+20),
				float32(itemY),
				float32(gm.modal.Width-40),
				float32(itemHeight),
				bgColor, false)

			// Draw guild info
			guildText := fmt.Sprintf("%s, %s", guild.Name, guild.Tag)
			text.Draw(screen, guildText, font, gm.modal.X+30, itemY+20, color.White)

			// Draw remove button
			removeRect := Rect{
				X:      gm.modal.X + gm.modal.Width - 40,
				Y:      itemY + 5,
				Width:  20,
				Height: 20,
			}

			removeColor := color.RGBA{200, 100, 100, 255}
			mx, my := ebiten.CursorPosition()
			if mx >= removeRect.X && mx < removeRect.X+removeRect.Width &&
				my >= removeRect.Y && my < removeRect.Y+removeRect.Height {
				removeColor = color.RGBA{255, 120, 120, 255}
			}

			vector.DrawFilledRect(screen,
				float32(removeRect.X),
				float32(removeRect.Y),
				float32(removeRect.Width),
				float32(removeRect.Height),
				removeColor, false)

			// Draw X in remove button
			text.Draw(screen, "✕", font, removeRect.X+6, removeRect.Y+16, color.White)
		}

		// Draw scrollbar if needed
		if len(gm.filteredGuilds) > maxVisibleItems {
			scrollbarHeight := gm.modal.Height - 120
			thumbHeight := scrollbarHeight * maxVisibleItems / len(gm.filteredGuilds)
			thumbY := gm.modal.Y + 100 + (scrollbarHeight-thumbHeight)*gm.scrollOffset/(len(gm.filteredGuilds)-maxVisibleItems)

			// Draw scrollbar track
			vector.DrawFilledRect(screen,
				float32(gm.modal.X+gm.modal.Width-15),
				float32(gm.modal.Y+100),
				float32(5),
				float32(scrollbarHeight),
				color.RGBA{60, 60, 60, 255}, false)

			// Draw scrollbar thumb
			vector.DrawFilledRect(screen,
				float32(gm.modal.X+gm.modal.Width-15),
				float32(thumbY),
				float32(5),
				float32(thumbHeight),
				color.RGBA{120, 120, 120, 255}, false)
		}
	} else {
		// Draw empty message
		emptyText := "No guilds found. Add one using the fields above."
		emptyBounds := text.BoundString(font, emptyText)
		text.Draw(screen, emptyText,
			font,
			gm.modal.X+(gm.modal.Width-emptyBounds.Dx())/2,
			gm.modal.Y+150,
			color.RGBA{180, 180, 180, 255})
	}
}
