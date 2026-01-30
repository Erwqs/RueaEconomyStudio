package app

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"RueaES/eruntime"
	"RueaES/pluginhost"
	"RueaES/typedef"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

// PluginManager controls native extension loading/unloading and persistence.
type PluginManager struct {
	visible         bool
	modal           *EnhancedModal
	plugins         []typedef.PluginState
	selectedIndex   int
	lastSelected    int
	scrollOffset    int
	controlScroll   int
	controlContent  int
	framesAfterOpen int

	addButton                *EnhancedButton
	toggleButton             *EnhancedButton
	removeButton             *EnhancedButton
	host                     *pluginhost.Host
	uiControls               map[string][]pluginhost.UIControl
	uiButtons                map[string]*EnhancedButton
	uiCheckboxes             map[string]*Checkbox
	uiSliders                map[string]*MenuSlider
	uiTextInputs             map[string]*MenuTextInput
	uiSelects                map[string]*Dropdown
	uiLayout                 map[string]layoutSlot
	fileDialogue             *FileSystemDialogue
	modalOverlay             *pluginModalOverlay
	pluginFileDialogue       *FileSystemDialogue
	pluginFileOwner          string
	pluginColorPicker        *ColorPicker
	pluginColorOwner         string
	territorySelectorOwner   string
	territorySelectorOverlay *TerritorySelectorOverlay
	dropPrompt               *pluginDropPrompt
}

type pluginDropPrompt struct {
	modal       *EnhancedModal
	continueBtn *EnhancedButton
	cancelBtn   *EnhancedButton
	path        string
	onContinue  func()
	onCancel    func()
}

func newPluginDropPrompt() *pluginDropPrompt {
	modal := NewEnhancedModal("Load Plugin", 520, 240)
	prompt := &pluginDropPrompt{
		modal:       modal,
		continueBtn: NewEnhancedButton("Continue", 0, 0, 130, 36, nil),
		cancelBtn:   NewEnhancedButton("Cancel", 0, 0, 130, 36, nil),
	}

	// Warn the user with a yellow primary action and a neutral cancel.
	prompt.continueBtn.SetYellowButtonStyle()
	prompt.cancelBtn.SetGrayButtonStyle()

	return prompt
}

func (p *pluginDropPrompt) layoutButtons() {
	cx := p.modal.X + p.modal.Width/2
	y := p.modal.Y + p.modal.Height - 70
	p.cancelBtn.SetPosition(cx-150, y)
	p.continueBtn.SetPosition(cx+20, y)
}

func (p *pluginDropPrompt) Show(path string, onContinue, onCancel func()) {
	p.path = path
	p.onContinue = onContinue
	p.onCancel = onCancel
	p.modal.Title = fmt.Sprintf("Load %s?", filepath.Base(path))
	p.layoutButtons()

	p.continueBtn.OnClick = func() {
		if p.onContinue != nil {
			p.onContinue()
		}
		p.Hide()
	}

	p.cancelBtn.OnClick = func() {
		if p.onCancel != nil {
			p.onCancel()
		}
		p.Hide()
	}

	p.modal.Show()
}

func (p *pluginDropPrompt) Hide() {
	if p.modal != nil {
		p.modal.Hide()
	}
}

func (p *pluginDropPrompt) IsVisible() bool {
	return p != nil && p.modal != nil && p.modal.IsVisible()
}

func (p *pluginDropPrompt) Update() bool {
	if !p.IsVisible() {
		return false
	}

	p.modal.Update()
	p.layoutButtons()
	mx, my := ebiten.CursorPosition()
	p.continueBtn.Update(mx, my)
	p.cancelBtn.Update(mx, my)

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if p.onCancel != nil {
			p.onCancel()
		}
		p.Hide()
		return true
	}

	return true
}

func (p *pluginDropPrompt) Draw(screen *ebiten.Image) {
	if !p.IsVisible() {
		return
	}

	p.modal.Draw(screen)

	font := loadWynncraftFont(16)
	contentX, contentY, contentW, _ := p.modal.GetContentArea()

	primary := fmt.Sprintf("Load %s?", filepath.Base(p.path))
	warning := "Loading arbitrary plugin can be dangerous, continue?"

	text.Draw(screen, primary, font, contentX, contentY+20, EnhancedUIColors.Text)

	wrapped := wrapTextFace(font, warning, contentW)
	lineY := contentY + 50
	for _, line := range wrapped {
		text.Draw(screen, line, font, contentX, lineY, EnhancedUIColors.TextSecondary)
		lineY += 20
	}

	p.continueBtn.Draw(screen)
	p.cancelBtn.Draw(screen)
}

// hideAllUIExceptTerritorySelector closes visible host UI surfaces so the territory selector can take focus.
func (pm *PluginManager) hideAllUIExceptTerritorySelector() {
	// Close plugin-driven modals/dialogs first.
	if pm.modalOverlay != nil && pm.modalOverlay.IsVisible() {
		pm.modalOverlay.Hide()
	}
	if pm.pluginFileDialogue != nil && pm.pluginFileDialogue.IsVisible() {
		if pm.pluginFileOwner != "" {
			pluginhost.CompleteFileDialogCancel(pm.pluginFileOwner)
			pm.pluginFileOwner = ""
		}
		pm.pluginFileDialogue.Hide()
	}
	if pm.pluginColorPicker != nil && pm.pluginColorPicker.IsVisible() {
		if pm.pluginColorOwner != "" {
			pluginhost.CompleteColorPickerCancel(pm.pluginColorOwner)
			pm.pluginColorOwner = ""
		}
		pm.pluginColorPicker.Hide()
	}

	// Hide the plugin manager chrome itself.
	if pm.modal != nil && pm.modal.IsVisible() {
		pm.modal.Hide()
	}
	pm.visible = false

	// Close global UI managed elsewhere.
	if lm := GetLoadoutManager(); lm != nil {
		lm.CancelLoadoutApplication()
		lm.Hide()
	}
	if gm := GetEnhancedGuildManager(); gm != nil && gm.IsVisible() {
		gm.Hide()
	}
	if mv := GetMapView(); mv != nil {
		if mv.edgeMenu != nil && mv.edgeMenu.IsVisible() {
			mv.edgeMenu.Hide()
		}
		if mv.territoriesManager != nil && mv.territoriesManager.IsSideMenuOpen() {
			mv.territoriesManager.CloseSideMenu()
		}
		if mv.stateManagementMenu != nil && mv.stateManagementMenu.menu.IsVisible() {
			mv.stateManagementMenu.menu.Hide()
			mv.stateManagementMenuVisible = false
		}
		if mv.tributeMenu != nil && mv.tributeMenu.IsVisible() {
			mv.tributeMenu.Hide()
		}
		if mv.analysisModal != nil && mv.analysisModal.IsVisible() {
			mv.analysisModal.Hide()
		}
	}
}

type layoutSlot struct {
	x      int
	y      int
	width  int
	height int
}

func (pm *PluginManager) handleCommandHighlightTerritory(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	mv := GetMapView()
	if mv == nil || mv.territoriesManager == nil {
		return pluginhost.HostErrInternal
	}
	if _, exists := mv.territoriesManager.Territories[name]; !exists {
		return pluginhost.HostErrBadArgument
	}
	mv.territoriesManager.SetSelectedTerritory(name)
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandClearHighlight() int {
	mv := GetMapView()
	if mv == nil || mv.territoriesManager == nil {
		return pluginhost.HostErrInternal
	}
	mv.territoriesManager.DeselectTerritory()
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetOverlay(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	rVal, rok := args["r"].(float64)
	gVal, gok := args["g"].(float64)
	bVal, bok := args["b"].(float64)
	aVal, aok := args["a"].(float64)
	if !rok || !gok || !bok {
		return pluginhost.HostErrBadArgument
	}
	// alpha optional, default 255
	if !aok {
		aVal = 255
	}
	c := color.RGBA{R: clampU8(rVal), G: clampU8(gVal), B: clampU8(bVal), A: clampU8(aVal)}
	pluginhost.SetOverlayColor(name, c)
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandClearOverlays() int {
	pluginhost.ClearOverlays()
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandGetOverlays() (int, map[string]any) {
	cache := pluginhost.CopyOverlayCache()
	resp := make(map[string]any, len(cache))
	for k, v := range cache {
		resp[k] = map[string]any{"r": v.R, "g": v.G, "b": v.B, "a": v.A}
	}
	return pluginhost.HostOK, map[string]any{"overlays": resp}
}

func (pm *PluginManager) handleCommandGetOverlay(args map[string]any) (int, map[string]any) {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument, nil
	}
	if c, found := pluginhost.GetOverlayColor(name); found {
		return pluginhost.HostOK, map[string]any{"overlay": map[string]any{"r": c.R, "g": c.G, "b": c.B, "a": c.A}}
	}
	return pluginhost.HostOK, map[string]any{"overlay": nil}
}

func (pm *PluginManager) handleCommandClearOverlay(args map[string]any) (int, map[string]any) {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument, nil
	}
	pluginhost.ClearOverlay(name)
	return pluginhost.HostOK, nil
}

func (pm *PluginManager) handleCommandSetRoutingMode(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	modeVal, ok := args["routing_mode"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	modeFloat, ok := toFloat(modeVal)
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if err := eruntime.SetTerritoryRoutingMode(name, typedef.Routing(int(modeFloat))); err != nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetBorder(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	borderVal, ok := args["border"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	borderFloat, ok := toFloat(borderVal)
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if err := eruntime.SetTerritoryBorder(name, typedef.Border(int(borderFloat))); err != nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetTax(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	normal, ok := toFloat(args["tax"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	ally, ok := toFloat(args["ally_tax"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if err := eruntime.SetTerritoryTax(name, normal, ally); err != nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetTreasury(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	overrideVal, ok := toFloat(args["treasury_override"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	override := int(overrideVal)
	if override < int(typedef.TreasuryOverrideNone) || override > int(typedef.TreasuryOverrideVeryHigh) {
		return pluginhost.HostErrBadArgument
	}
	territory := eruntime.GetTerritory(name)
	if territory == nil {
		return pluginhost.HostErrBadArgument
	}
	eruntime.SetTreasuryOverride(territory, typedef.TreasuryOverride(override))
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetHQ(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	flag, ok := toBool(args["hq"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if err := eruntime.SetTerritoryHQ(name, flag); err != nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetUpgrade(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	kind, ok := args["upgrade"].(string)
	if !ok || kind == "" {
		return pluginhost.HostErrBadArgument
	}
	levelF, ok := toFloat(args["level"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if t := eruntime.SetTerritoryUpgrade(name, kind, int(levelF)); t == nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetBonus(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	kind, ok := args["bonus"].(string)
	if !ok || kind == "" {
		return pluginhost.HostErrBadArgument
	}
	levelF, ok := toFloat(args["level"])
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if t := eruntime.SetTerritoryBonus(name, kind, int(levelF)); t == nil {
		return pluginhost.HostErrBadArgument
	}
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandSetStorage(args map[string]any) int {
	nameVal, ok := args["territory"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	name, ok := nameVal.(string)
	if !ok || name == "" {
		return pluginhost.HostErrBadArgument
	}
	stateVal, ok := args["storage"]
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	resources, ok := toBasicResources(stateVal)
	if !ok {
		return pluginhost.HostErrBadArgument
	}
	if eruntime.GetTerritory(name) == nil {
		return pluginhost.HostErrBadArgument
	}
	eruntime.ModifyStorageState(name, &resources)
	return pluginhost.HostOK
}

func (pm *PluginManager) handleCommandUpdateTribute(args map[string]any) (int, map[string]any) {
	idVal, ok := args["id"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	id, ok := idVal.(string)
	if !ok || id == "" {
		return pluginhost.HostErrBadArgument, nil
	}
	amount := (*typedef.BasicResources)(nil)
	if amountVal, ok := args["amount"]; ok {
		if converted, ok := toBasicResources(amountVal); ok {
			amt := converted
			amount = &amt
		} else {
			return pluginhost.HostErrBadArgument, nil
		}
	}
	interval := (*uint32)(nil)
	if intervalVal, ok := args["interval_minutes"]; ok {
		if intervalF, ok := toFloat(intervalVal); ok && intervalF > 0 {
			ival := uint32(intervalF)
			interval = &ival
		} else {
			return pluginhost.HostErrBadArgument, nil
		}
	}
	if amount == nil && interval == nil {
		return pluginhost.HostErrBadArgument, nil
	}
	if err := eruntime.UpdateTribute(id, amount, interval); err != nil {
		return pluginhost.HostErrInternal, map[string]any{"error": err.Error()}
	}
	return pluginhost.HostOK, nil
}

func (pm *PluginManager) handleCommandGetTerritories() (int, map[string]any) {
	mv := GetMapView()
	if mv == nil || mv.territoriesManager == nil {
		return pluginhost.HostErrInternal, nil
	}
	gm := GetEnhancedGuildManager()
	terr := mv.territoriesManager.Territories
	list := make([]map[string]any, 0, len(terr))
	for name, t := range terr {
		var ownerColor map[string]any
		if gm != nil {
			if c, ok := gm.GetGuildColor(t.Guild.Name, t.Guild.Prefix); ok {
				ownerColor = map[string]any{"r": c.R, "g": c.G, "b": c.B, "a": c.A}
			}
		}
		loc := map[string]any{"start": t.Location.Start, "end": t.Location.End}
		resources := map[string]any{
			"emeralds": t.Resources.Emeralds,
			"ore":      t.Resources.Ore,
			"crops":    t.Resources.Crops,
			"fish":     t.Resources.Fish,
			"wood":     t.Resources.Wood,
		}
		entry := map[string]any{
			"name":       name,
			"guild_name": t.Guild.Name,
			"guild_tag":  t.Guild.Prefix,
			"hq":         t.isHQ,
			"location":   loc,
			"resources":  resources,
			"color":      ownerColor,
		}
		list = append(list, entry)
	}
	return pluginhost.HostOK, map[string]any{"territories": list}
}

func (pm *PluginManager) handleCommandGetGuilds() (int, map[string]any) {
	gm := GetEnhancedGuildManager()
	if gm == nil {
		return pluginhost.HostErrInternal, nil
	}
	list := make([]map[string]any, 0, len(gm.guilds))
	for _, g := range gm.guilds {
		list = append(list, map[string]any{
			"name":  g.Name,
			"tag":   g.Tag,
			"color": g.Color,
			"show":  g.Show,
		})
	}
	return pluginhost.HostOK, map[string]any{"guilds": list}
}

func (pm *PluginManager) handleCommandGetLoadouts() (int, map[string]any) {
	lm := GetLoadoutManager()
	if lm == nil {
		return pluginhost.HostErrInternal, nil
	}
	list := make([]map[string]any, 0, len(lm.loadouts))
	for _, l := range lm.loadouts {
		list = append(list, map[string]any{"name": l.Name})
	}
	return pluginhost.HostOK, map[string]any{"loadouts": list}
}

func (pm *PluginManager) handleCommandGetState() (int, map[string]any) {
	mv := GetMapView()
	selected := ""
	if mv != nil && mv.territoriesManager != nil {
		selected = mv.territoriesManager.GetSelectedTerritoryName()
	}
	overlayCount := len(pluginhost.CopyOverlayCache())
	gm := GetEnhancedGuildManager()
	loadouts := GetLoadoutManager()
	tribs := eruntime.GetAllActiveTributes()
	territoryCount := 0
	if mv != nil && mv.territoriesManager != nil {
		territoryCount = len(mv.territoriesManager.Territories)
	}
	tick := eruntime.GetCurrentTick()

	resp := map[string]any{
		"selected_territory": selected,
		"overlay_count":      overlayCount,
		"guilds":             0,
		"loadouts":           0,
		"active_tributes":    len(tribs),
		"territories":        territoryCount,
		"tick":               tick,
	}
	if gm != nil {
		resp["guilds"] = len(gm.guilds)
	}
	if loadouts != nil {
		resp["loadouts"] = len(loadouts.loadouts)
	}
	return pluginhost.HostOK, resp
}

func (pm *PluginManager) handleCommandGetTribute() (int, map[string]any) {
	active := eruntime.GetAllActiveTributes()
	list := make([]map[string]any, 0, len(active))
	for _, t := range active {
		if t == nil {
			continue
		}
		nextMinutes, nextTicks := tributeETA(t)
		list = append(list, map[string]any{
			"id":                    t.ID,
			"from_guild":            t.FromGuildName,
			"to_guild":              t.ToGuildName,
			"interval_minutes":      t.IntervalMinutes,
			"active":                t.IsActive,
			"last_transfer":         t.LastTransfer,
			"next_transfer_minutes": nextMinutes,
			"next_transfer_ticks":   nextTicks,
			"amount_per_hour": map[string]any{
				"emeralds": t.AmountPerHour.Emeralds,
				"ores":     t.AmountPerHour.Ores,
				"wood":     t.AmountPerHour.Wood,
				"fish":     t.AmountPerHour.Fish,
				"crops":    t.AmountPerHour.Crops,
			},
		})
	}
	return pluginhost.HostOK, map[string]any{"tributes": list}
}

func (pm *PluginManager) handleCommandCreateTribute(args map[string]any) (int, map[string]any) {
	amountVal, ok := args["amount"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	amount, ok := toBasicResources(amountVal)
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	intervalVal, ok := args["interval_minutes"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	intervalF, ok := toFloat(intervalVal)
	if !ok || intervalF <= 0 {
		return pluginhost.HostErrBadArgument, nil
	}
	from := ""
	to := ""
	if v, ok := args["from_guild"]; ok {
		if s, ok := v.(string); ok {
			from = s
		}
	}
	if v, ok := args["to_guild"]; ok {
		if s, ok := v.(string); ok {
			to = s
		}
	}
	tribute, err := eruntime.CreateTribute(from, to, amount, uint32(intervalF))
	if err != nil {
		return pluginhost.HostErrBadArgument, map[string]any{"error": err.Error()}
	}
	if err := eruntime.AddTribute(tribute); err != nil {
		return pluginhost.HostErrInternal, map[string]any{"error": err.Error()}
	}
	return pluginhost.HostOK, map[string]any{"id": tribute.ID}
}

func (pm *PluginManager) handleCommandDeleteTribute(args map[string]any) (int, map[string]any) {
	idVal, ok := args["id"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	id, ok := idVal.(string)
	if !ok || id == "" {
		return pluginhost.HostErrBadArgument, nil
	}
	if err := eruntime.RemoveTribute(id); err != nil {
		return pluginhost.HostErrInternal, map[string]any{"error": err.Error()}
	}
	return pluginhost.HostOK, nil
}

func (pm *PluginManager) handleCommandSetTributeActive(args map[string]any) (int, map[string]any) {
	idVal, ok := args["id"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	id, ok := idVal.(string)
	if !ok || id == "" {
		return pluginhost.HostErrBadArgument, nil
	}
	activeVal, ok := args["active"]
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	active, ok := toBool(activeVal)
	if !ok {
		return pluginhost.HostErrBadArgument, nil
	}
	var err error
	if active {
		err = eruntime.EnableTributeByID(id)
	} else {
		err = eruntime.DisableTributeByID(id)
	}
	if err != nil {
		return pluginhost.HostErrInternal, map[string]any{"error": err.Error()}
	}
	return pluginhost.HostOK, nil
}

// NewPluginManager creates the plugin manager UI and state tracker.
func NewPluginManager() *PluginManager {
	screenW, screenH := ebiten.WindowSize()
	modalWidth := 1180
	modalHeight := 600
	modalX := (screenW - modalWidth) / 2
	modalY := (screenH - modalHeight) / 2

	modal := NewEnhancedModal("Extensions", modalWidth, modalHeight)

	buttonWidth := 140
	buttonHeight := 40
	buttonY := modalY + modalHeight - 60

	addButton := NewEnhancedButton("Add Plugin", modalX+20, buttonY, buttonWidth, buttonHeight, nil)
	toggleButton := NewEnhancedButton("Enable", modalX+180, buttonY, buttonWidth, buttonHeight, nil)
	removeButton := NewEnhancedButton("Unload", modalX+340, buttonY, buttonWidth, buttonHeight, nil)

	pm := &PluginManager{
		visible:         false,
		modal:           modal,
		plugins:         []typedef.PluginState{},
		selectedIndex:   -1,
		lastSelected:    -1,
		scrollOffset:    0,
		framesAfterOpen: 0,
		controlScroll:   0,
		controlContent:  0,
		addButton:       addButton,
		toggleButton:    toggleButton,
		removeButton:    removeButton,
		host:            pluginhost.NewHost(),
		uiControls:      make(map[string][]pluginhost.UIControl),
		uiButtons:       make(map[string]*EnhancedButton),
		uiCheckboxes:    make(map[string]*Checkbox),
		uiSliders:       make(map[string]*MenuSlider),
		uiTextInputs:    make(map[string]*MenuTextInput),
		uiSelects:       make(map[string]*Dropdown),
		uiLayout:        make(map[string]layoutSlot),
		dropPrompt:      newPluginDropPrompt(),
	}

	pm.modalOverlay = newPluginModalOverlay(pm.host)

	pluginhost.RegisterToastHandler(func(msg string, c color.RGBA) {
		NewToast().AutoClose(3*time.Second).Text(msg, ToastOption{Colour: c}).Show()
	})
	pluginhost.RegisterCommandHandler(func(verb string, args map[string]any) (int, map[string]any) {
		switch verb {
		case "ping":
			return pluginhost.HostOK, nil
		case "highlight_territory":
			return pm.handleCommandHighlightTerritory(args), nil
		case "clear_highlight":
			return pm.handleCommandClearHighlight(), nil
		case "set_overlay_color":
			return pm.handleCommandSetOverlay(args), nil
		case "clear_overlays":
			return pm.handleCommandClearOverlays(), nil
		case "clear_overlay":
			return pm.handleCommandClearOverlay(args)
		case "set_territory_routing_mode":
			return pm.handleCommandSetRoutingMode(args), nil
		case "set_territory_border":
			return pm.handleCommandSetBorder(args), nil
		case "set_territory_tax":
			return pm.handleCommandSetTax(args), nil
		case "set_territory_treasury":
			return pm.handleCommandSetTreasury(args), nil
		case "set_territory_hq":
			return pm.handleCommandSetHQ(args), nil
		case "set_territory_upgrade":
			return pm.handleCommandSetUpgrade(args), nil
		case "set_territory_bonus":
			return pm.handleCommandSetBonus(args), nil
		case "set_territory_storage":
			return pm.handleCommandSetStorage(args), nil
		case "get_overlays":
			return pm.handleCommandGetOverlays()
		case "get_overlay":
			return pm.handleCommandGetOverlay(args)
		case "get_territories":
			return pm.handleCommandGetTerritories()
		case "get_guilds":
			return pm.handleCommandGetGuilds()
		case "get_loadouts":
			return pm.handleCommandGetLoadouts()
		case "get_state":
			return pm.handleCommandGetState()
		case "get_tribute":
			return pm.handleCommandGetTribute()
		case "create_tribute":
			return pm.handleCommandCreateTribute(args)
		case "update_tribute":
			return pm.handleCommandUpdateTribute(args)
		case "delete_tribute":
			return pm.handleCommandDeleteTribute(args)
		case "set_tribute_active":
			return pm.handleCommandSetTributeActive(args)
		default:
			return pluginhost.HostErrUnsupported, nil
		}
	})

	pluginhost.RegisterModalHandlers(func(spec pluginhost.ModalSpec) {
		if pm.modalOverlay != nil {
			pm.modalOverlay.Show(spec)
		}
	}, func(pluginID, modalID string) {
		if pm.modalOverlay != nil && pm.modalOverlay.IsVisible() && pm.modalOverlay.pluginID == pluginID && pm.modalOverlay.modalID == modalID {
			pm.modalOverlay.Hide()
		}
	})

	pluginhost.RegisterFileDialogHandler(func(spec pluginhost.FileDialogSpec) {
		pm.showPluginFileDialog(spec)
	})

	pluginhost.RegisterColorPickerHandler(func(spec pluginhost.ColorPickerSpec) {
		pm.showPluginColorPicker(spec)
	})

	pluginhost.RegisterTerritorySelectorHandler(func(spec pluginhost.TerritorySelectorSpec) {
		pm.showPluginTerritorySelector(spec)
	})

	addButton.OnClick = func() {
		pm.showFileDialogue()
	}

	toggleButton.OnClick = func() {
		pm.toggleSelectedPlugin()
	}

	removeButton.OnClick = func() {
		pm.removeSelectedPlugin()
	}

	return pm
}

// Show displays the plugin manager.
func (pm *PluginManager) Show() {
	pm.visible = true
	pm.framesAfterOpen = 0
	pm.modal.Show()
}

// Hide hides the plugin manager and any active file dialogue.
func (pm *PluginManager) Hide() {
	pm.visible = false
	pm.modal.Hide()
	if pm.fileDialogue != nil {
		pm.fileDialogue.Hide()
	}
	if pm.pluginFileDialogue != nil && pm.pluginFileDialogue.IsVisible() {
		if pm.pluginFileOwner != "" {
			pluginhost.CompleteFileDialogCancel(pm.pluginFileOwner)
			pm.pluginFileOwner = ""
		}
		pm.pluginFileDialogue.Hide()
	}
	if pm.pluginColorPicker != nil && pm.pluginColorPicker.IsVisible() {
		if pm.pluginColorOwner != "" {
			pluginhost.CompleteColorPickerCancel(pm.pluginColorOwner)
			pm.pluginColorOwner = ""
		}
		pm.pluginColorPicker.Hide()
	}
	if pm.territorySelectorOverlay != nil && pm.territorySelectorOverlay.IsVisible() {
		if pm.territorySelectorOwner != "" {
			pluginhost.CompleteTerritorySelectorCancel(pm.territorySelectorOwner)
			pm.territorySelectorOwner = ""
		}
		pm.territorySelectorOverlay.Hide()
	}
	pm.syncRuntimeState()
}

// IsVisible reports visibility of the plugin manager.
func (pm *PluginManager) IsVisible() bool {
	if pm.dropPrompt != nil && pm.dropPrompt.IsVisible() {
		return true
	}
	if pm.pluginFileDialogue != nil && pm.pluginFileDialogue.IsVisible() {
		return true
	}
	if pm.pluginColorPicker != nil && pm.pluginColorPicker.IsVisible() {
		return true
	}
	if pm.fileDialogue != nil && pm.fileDialogue.IsVisible() {
		return true
	}
	return pm.visible
}

// Tick advances all enabled plugins via the host.
func (pm *PluginManager) Tick() {
	if pm.host != nil {
		pm.host.Tick()
	}
}

// Update processes input and layout.
func (pm *PluginManager) Update() bool {
	inputConsumed := false
	if pm.dropPrompt != nil && pm.dropPrompt.IsVisible() {
		if pm.dropPrompt.Update() {
			return true
		}
	}
	if pm.modalOverlay != nil && pm.modalOverlay.IsVisible() {
		if pm.modalOverlay.Update() {
			return true
		}
	}
	if pm.pluginFileDialogue != nil && pm.pluginFileDialogue.IsVisible() {
		if pm.pluginFileDialogue.Update() {
			inputConsumed = true
		}
	}
	if pm.pluginColorPicker != nil && pm.pluginColorPicker.IsVisible() {
		if pm.pluginColorPicker.Update() {
			inputConsumed = true
		}
	}
	if pm.territorySelectorOverlay != nil && pm.territorySelectorOverlay.IsVisible() {
		if pm.territorySelectorOverlay.Update() {
			return true
		}
	}
	if pm.fileDialogue != nil && pm.fileDialogue.IsVisible() {
		if pm.fileDialogue.Update() {
			inputConsumed = true
		}
	}

	if !pm.visible {
		return inputConsumed
	}

	pm.framesAfterOpen++
	pm.updateLayout()
	pm.modal.Update()

	mx, my := ebiten.CursorPosition()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if !pm.modal.Contains(mx, my) {
			pm.Hide()
			return true
		}
	}

	if pm.framesAfterOpen > 5 && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		pm.Hide()
		return true
	}

	pm.addButton.Update(mx, my)
	pm.toggleButton.Update(mx, my)
	pm.removeButton.Update(mx, my)
	pm.updateDynamicUI(mx, my)

	pm.updateListInteraction(mx, my)

	return true
}

// Draw renders the plugin manager and any open dialogue.
func (pm *PluginManager) Draw(screen *ebiten.Image) {
	if pm.visible {
		pm.modal.Draw(screen)
		pm.drawList(screen)
		pm.drawDetails(screen)
		pm.drawControlSurface(screen)
		pm.addButton.Draw(screen)
		pm.toggleButton.Draw(screen)
		pm.removeButton.Draw(screen)
		pm.drawDynamicUI(screen)
	}

	if pm.modalOverlay != nil && pm.modalOverlay.IsVisible() {
		pm.modalOverlay.Draw(screen)
	}

	if pm.pluginFileDialogue != nil && pm.pluginFileDialogue.IsVisible() {
		pm.pluginFileDialogue.Draw(screen)
	}

	if pm.pluginColorPicker != nil && pm.pluginColorPicker.IsVisible() {
		pm.pluginColorPicker.Draw(screen)
	}
	if pm.territorySelectorOverlay != nil && pm.territorySelectorOverlay.IsVisible() {
		pm.territorySelectorOverlay.Draw(screen)
	}

	if pm.fileDialogue != nil && pm.fileDialogue.IsVisible() {
		pm.fileDialogue.Draw(screen)
	}

	if pm.dropPrompt != nil && pm.dropPrompt.IsVisible() {
		pm.dropPrompt.Draw(screen)
	}
}

// GetPluginsForSave returns a copy of plugin states for persistence.
func (pm *PluginManager) GetPluginsForSave() []typedef.PluginState {
	pm.syncRuntimeState()
	plugins := make([]typedef.PluginState, 0, len(pm.plugins))
	for _, p := range pm.plugins {
		copyPlugin := p
		// Persist the entry but require explicit re-enable on next load
		copyPlugin.Enabled = false
		plugins = append(plugins, copyPlugin)
	}
	return plugins
}

// ApplySavedPlugins restores plugins from state, marking missing binaries.
func (pm *PluginManager) ApplySavedPlugins(saved []typedef.PluginState) {
	pm.plugins = make([]typedef.PluginState, 0, len(saved))
	pm.selectedIndex = -1
	pm.scrollOffset = 0

	for _, plugin := range saved {
		copyPlugin := plugin
		copyPlugin.Enabled = false
		copyPlugin.Missing = false
		copyPlugin.LastError = ""
		if copyPlugin.Path == "" || !pluginFileExists(copyPlugin.Path) {
			copyPlugin.Missing = true
			copyPlugin.LastError = "Binary not found"
		}
		pm.plugins = append(pm.plugins, copyPlugin)
	}
}

func (pm *PluginManager) updateLayout() {
	screenW, screenH := ebiten.WindowSize()
	pm.modal.Width = 1180
	pm.modal.Height = 600
	pm.modal.X = (screenW - pm.modal.Width) / 2
	pm.modal.Y = (screenH - pm.modal.Height) / 2

	buttonY := pm.modal.Y + pm.modal.Height - 60
	pm.addButton.X = pm.modal.X + 20
	pm.addButton.Y = buttonY
	pm.toggleButton.X = pm.modal.X + 180
	pm.toggleButton.Y = buttonY
	pm.removeButton.X = pm.modal.X + 340
	pm.removeButton.Y = buttonY

}

// controlView computes the scrollable region within the control surface panel.
func (pm *PluginManager) controlView() (viewTop, viewHeight int, controlRect image.Rectangle) {
	_, _, controlRect = pm.layoutRects()
	viewTop = controlRect.Min.Y + 50
	viewHeight = controlRect.Dy() - 60
	if viewHeight < 0 {
		viewHeight = 0
	}
	return
}

// layoutRects returns the rectangles for the list, details, and control surface panels.
func (pm *PluginManager) layoutRects() (image.Rectangle, image.Rectangle, image.Rectangle) {
	margin := 20
	gap := 10
	listWidth := 300
	detailWidth := 340
	available := pm.modal.Width - (margin * 2) - (gap * 2)
	controlWidth := available - listWidth - detailWidth
	if controlWidth < 240 {
		controlWidth = 240
	}
	startY := pm.modal.Y + 60
	height := pm.modal.Height - 140
	listRect := image.Rect(pm.modal.X+margin, startY, pm.modal.X+margin+listWidth, startY+height)
	detailRect := image.Rect(listRect.Max.X+gap, startY, listRect.Max.X+gap+detailWidth, startY+height)
	controlRect := image.Rect(detailRect.Max.X+gap, startY, detailRect.Max.X+gap+controlWidth, startY+height)
	return listRect, detailRect, controlRect
}

func (pm *PluginManager) updateListInteraction(mx, my int) {
	listRect, _, _ := pm.layoutRects()
	itemHeight := 32
	maxVisibleItems := listRect.Dy() / itemHeight

	pm.toggleButton.enabled = pm.selectedIndex >= 0 && pm.selectedIndex < len(pm.plugins)
	pm.removeButton.enabled = pm.toggleButton.enabled

	pm.updateToggleButtonText()

	if len(pm.plugins) == 0 {
		return
	}

	pm.hoverSelection(mx, my, listRect, itemHeight, maxVisibleItems)
	pm.handleScroll(mx, my, listRect, itemHeight, maxVisibleItems)
	pm.handleSelectionChanged()
}

func (pm *PluginManager) hoverSelection(mx, my int, listRect image.Rectangle, itemHeight, maxVisibleItems int) {
	for i := 0; i < len(pm.plugins) && i < maxVisibleItems; i++ {
		idx := i + pm.scrollOffset
		if idx >= len(pm.plugins) {
			break
		}
		itemY := listRect.Min.Y + (i * itemHeight)
		itemRect := Rect{X: listRect.Min.X, Y: itemY, Width: listRect.Dx(), Height: itemHeight}

		if mx >= itemRect.X && mx < itemRect.X+itemRect.Width && my >= itemRect.Y && my < itemRect.Y+itemRect.Height {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				pm.selectedIndex = idx
			}
		}
	}
}

func (pm *PluginManager) handleScroll(mx, my int, listRect image.Rectangle, itemHeight, maxVisibleItems int) {
	_, wy := ebiten.Wheel()
	if wy != 0 && mx >= listRect.Min.X && mx < listRect.Max.X && my >= listRect.Min.Y && my < listRect.Max.Y {
		pm.scrollOffset -= int(wy * 3)
		maxScroll := len(pm.plugins) - maxVisibleItems
		if maxScroll < 0 {
			maxScroll = 0
		}
		if pm.scrollOffset < 0 {
			pm.scrollOffset = 0
		} else if pm.scrollOffset > maxScroll {
			pm.scrollOffset = maxScroll
		}
	}
}

func (pm *PluginManager) drawList(screen *ebiten.Image) {
	font := loadWynncraftFont(14)
	listRect, _, _ := pm.layoutRects()
	listStartY := listRect.Min.Y
	itemHeight := 32
	maxVisibleItems := listRect.Dy() / itemHeight
	vector.DrawFilledRect(screen, float32(listRect.Min.X), float32(listRect.Min.Y), float32(listRect.Dx()), float32(listRect.Dy()), EnhancedUIColors.Surface, false)
	vector.StrokeRect(screen, float32(listRect.Min.X), float32(listRect.Min.Y), float32(listRect.Dx()), float32(listRect.Dy()), 1, EnhancedUIColors.Border, false)

	if len(pm.plugins) == 0 {
		text.Draw(screen, "No plugins loaded", font, listRect.Min.X+10, listRect.Min.Y+24, EnhancedUIColors.TextSecondary)
		return
	}

	for i := 0; i < maxVisibleItems && i+pm.scrollOffset < len(pm.plugins); i++ {
		idx := i + pm.scrollOffset
		plugin := pm.plugins[idx]
		itemY := listStartY + (i * itemHeight)
		isSelected := idx == pm.selectedIndex

		if isSelected {
			vector.DrawFilledRect(screen, float32(listRect.Min.X), float32(itemY), float32(listRect.Dx()), float32(itemHeight), EnhancedUIColors.ItemSelected, false)
		}

		status := ""
		statusColor := EnhancedUIColors.TextSecondary
		if plugin.Missing {
			status = "(missing)"
			statusColor = colorRGBA(255, 120, 120)
		} else if plugin.Enabled {
			status = "(enabled)"
			statusColor = colorRGBA(120, 255, 120)
		} else {
			status = "(disabled)"
		}

		text.Draw(screen, plugin.Name, font, listRect.Min.X+10, itemY+22, EnhancedUIColors.Text)
		text.Draw(screen, status, font, listRect.Min.X+220, itemY+22, statusColor)
	}

	// Scrollbar for long lists
	contentItems := len(pm.plugins)
	if contentItems > maxVisibleItems {
		trackX := listRect.Max.X - 8
		trackW := 6
		trackY := listRect.Min.Y
		trackH := listRect.Dy()
		vector.DrawFilledRect(screen, float32(trackX), float32(trackY), float32(trackW), float32(trackH), colorRGBA(40, 40, 40), false)
		vector.StrokeRect(screen, float32(trackX), float32(trackY), float32(trackW), float32(trackH), 1, EnhancedUIColors.Border, false)
		thumbH := trackH * maxVisibleItems / contentItems
		if thumbH < 16 {
			thumbH = 16
		}
		if thumbH > trackH {
			thumbH = trackH
		}
		maxScrollItems := contentItems - maxVisibleItems
		thumbY := trackY
		if maxScrollItems > 0 {
			thumbY += (trackH - thumbH) * pm.scrollOffset / maxScrollItems
		}
		vector.DrawFilledRect(screen, float32(trackX+1), float32(thumbY), float32(trackW-2), float32(thumbH), EnhancedUIColors.ItemSelected, false)
	}
}

func (pm *PluginManager) drawDetails(screen *ebiten.Image) {
	font := loadWynncraftFont(14)
	headerFont := loadWynncraftFont(16)
	_, detailRect, _ := pm.layoutRects()
	detailX := detailRect.Min.X
	detailY := detailRect.Min.Y
	detailWidth := detailRect.Dx()
	detailHeight := detailRect.Dy()

	vector.DrawFilledRect(screen, float32(detailX), float32(detailY), float32(detailWidth), float32(detailHeight), EnhancedUIColors.Surface, false)
	vector.StrokeRect(screen, float32(detailX), float32(detailY), float32(detailWidth), float32(detailHeight), 1, EnhancedUIColors.Border, false)

	text.Draw(screen, "Plugin Details", headerFont, detailX+10, detailY+24, EnhancedUIColors.Text)

	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		text.Draw(screen, "Select a plugin to view details", font, detailX+10, detailY+50, EnhancedUIColors.TextSecondary)
		return
	}

	plugin := pm.plugins[pm.selectedIndex]
	text.Draw(screen, fmt.Sprintf("Name: %s", plugin.Name), font, detailX+10, detailY+50, EnhancedUIColors.Text)
	text.Draw(screen, fmt.Sprintf("Path: %s", displayPluginPath(plugin.Path)), font, detailX+10, detailY+70, EnhancedUIColors.TextSecondary)
	text.Draw(screen, fmt.Sprintf("Version: %s", plugin.Version), font, detailX+10, detailY+90, EnhancedUIColors.TextSecondary)
	stateText := "Disabled"
	stateColor := EnhancedUIColors.TextSecondary
	if plugin.Enabled {
		stateText = "Enabled"
		stateColor = colorRGBA(120, 255, 120)
	}
	if plugin.Missing {
		stateText = "Missing"
		stateColor = colorRGBA(255, 120, 120)
	}
	text.Draw(screen, fmt.Sprintf("Status: %s", stateText), font, detailX+10, detailY+110, stateColor)

	if plugin.LastError != "" {
		text.Draw(screen, fmt.Sprintf("Last Error: %s", plugin.LastError), font, detailX+10, detailY+130, colorRGBA(255, 180, 120))
	}

	author := plugin.Author
	if author == "" {
		author = "Unknown"
	}
	text.Draw(screen, fmt.Sprintf("Author: %s", author), font, detailX+10, detailY+170, EnhancedUIColors.TextSecondary)

	desc := plugin.Description
	if desc == "" {
		desc = "No description provided"
	}
	maxDescWidth := detailWidth - 20
	lines := wrapTextFace(font, desc, maxDescWidth)
	descY := detailY + 190
	text.Draw(screen, "Description:", font, detailX+10, descY, EnhancedUIColors.Text)
	lineY := descY + 20
	for _, line := range lines {
		text.Draw(screen, line, font, detailX+10, lineY, EnhancedUIColors.TextSecondary)
		lineY += 18
	}
}

func (pm *PluginManager) drawControlSurface(screen *ebiten.Image) {
	font := loadWynncraftFont(14)
	headerFont := loadWynncraftFont(16)
	viewTop, viewHeight, controlRect := pm.controlView()

	vector.DrawFilledRect(screen, float32(controlRect.Min.X), float32(controlRect.Min.Y), float32(controlRect.Dx()), float32(controlRect.Dy()), EnhancedUIColors.Surface, false)
	vector.StrokeRect(screen, float32(controlRect.Min.X), float32(controlRect.Min.Y), float32(controlRect.Dx()), float32(controlRect.Dy()), 1, EnhancedUIColors.Border, false)

	text.Draw(screen, "Control Surface", headerFont, controlRect.Min.X+10, controlRect.Min.Y+24, EnhancedUIColors.Text)

	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		text.Draw(screen, "Select a plugin to view controls", font, controlRect.Min.X+10, controlRect.Min.Y+50, EnhancedUIColors.TextSecondary)
		return
	}

	plugin := pm.plugins[pm.selectedIndex]
	ctrls := pm.uiControls[plugin.Path]
	if len(ctrls) == 0 {
		text.Draw(screen, "No UI controls provided by this plugin", font, controlRect.Min.X+10, controlRect.Min.Y+50, EnhancedUIColors.TextSecondary)
	}

	maxScroll := pm.controlContent - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if maxScroll > 0 && pm.controlContent > 0 && viewHeight > 0 {
		trackX := controlRect.Max.X - 8
		trackW := 6
		trackY := viewTop
		trackH := viewHeight
		vector.DrawFilledRect(screen, float32(trackX), float32(trackY), float32(trackW), float32(trackH), colorRGBA(40, 40, 40), false)
		vector.StrokeRect(screen, float32(trackX), float32(trackY), float32(trackW), float32(trackH), 1, EnhancedUIColors.Border, false)
		thumbH := trackH * trackH / pm.controlContent
		if thumbH < 24 {
			thumbH = 24
		}
		if thumbH > trackH {
			thumbH = trackH
		}
		thumbY := trackY
		if maxScroll > 0 {
			thumbY += (trackH - thumbH) * pm.controlScroll / maxScroll
		}
		vector.DrawFilledRect(screen, float32(trackX+1), float32(thumbY), float32(trackW-2), float32(thumbH), EnhancedUIColors.ItemSelected, false)
	}
}

func (pm *PluginManager) showFileDialogue() {
	pm.fileDialogue = NewFileSystemDialogue(FileDialogueOpen, "Select Plugin", []string{".so", ".dll", ".dylib"})
	pm.fileDialogue.SetOnFileSelected(func(path string) {
		pm.addPluginFromPath(path)
	})
	pm.fileDialogue.SetOnCancelled(func() {})
	pm.fileDialogue.Show()
}

func (pm *PluginManager) promptPluginDrop(path string) {
	if pm.dropPrompt == nil {
		pm.dropPrompt = newPluginDropPrompt()
	}

	pm.dropPrompt.Show(path, func() {
		pm.addPluginFromPathWithEnable(path, false)
	}, func() {})
}

func (pm *PluginManager) showPluginFileDialog(spec pluginhost.FileDialogSpec) {
	mode := FileDialogueOpen
	if spec.Mode == pluginhost.FileDialogSave {
		mode = FileDialogueSave
	}
	pm.pluginFileOwner = spec.PluginID
	pm.pluginFileDialogue = NewFileSystemDialogue(mode, "Select File", spec.Filters)
	if spec.Title != "" {
		pm.pluginFileDialogue.Title = spec.Title
	}
	if spec.DefaultPath != "" {
		pm.pluginFileDialogue.SetCurrentPath(spec.DefaultPath)
	}
	pm.pluginFileDialogue.SetOnFileSelected(func(path string) {
		if pm.pluginFileOwner != "" {
			pluginhost.CompleteFileDialogAccept(pm.pluginFileOwner, path)
			pm.pluginFileOwner = ""
		}
		pm.pluginFileDialogue.Hide()
	})
	pm.pluginFileDialogue.SetOnCancelled(func() {
		if pm.pluginFileOwner != "" {
			pluginhost.CompleteFileDialogCancel(pm.pluginFileOwner)
			pm.pluginFileOwner = ""
		}
		pm.pluginFileDialogue.Hide()
	})
	pm.pluginFileDialogue.Show()
}

func (pm *PluginManager) showPluginColorPicker(spec pluginhost.ColorPickerSpec) {
	screenW, screenH := ebiten.WindowSize()
	width := 520
	height := 360
	x := (screenW - width) / 2
	y := (screenH - (height + 120)) / 2
	pm.pluginColorOwner = spec.PluginID
	pm.pluginColorPicker = NewColorPicker(x, y, width, height)
	hex := fmt.Sprintf("#%02X%02X%02X", spec.Initial.R, spec.Initial.G, spec.Initial.B)
	pm.pluginColorPicker.Show(-1, hex)
	pm.pluginColorPicker.onConfirm = func(c color.RGBA) {
		if pm.pluginColorOwner != "" {
			pluginhost.CompleteColorPickerAccept(pm.pluginColorOwner, c)
			pm.pluginColorOwner = ""
		}
	}
	pm.pluginColorPicker.onCancel = func() {
		if pm.pluginColorOwner != "" {
			pluginhost.CompleteColorPickerCancel(pm.pluginColorOwner)
			pm.pluginColorOwner = ""
		}
	}
}

// showPluginTerritorySelector presents a full-screen map-based selector (click/drag like loadout apply mode).
func (pm *PluginManager) showPluginTerritorySelector(spec pluginhost.TerritorySelectorSpec) {
	// Close other UI surfaces before opening the selector so it owns the screen.
	pm.hideAllUIExceptTerritorySelector()

	pm.territorySelectorOwner = spec.PluginID
	mv := GetMapView()
	if mv == nil || mv.territoriesManager == nil {
		pluginhost.CompleteTerritorySelectorCancel(pm.territorySelectorOwner)
		pm.territorySelectorOwner = ""
		return
	}

	// Preselect only existing territories.
	pre := make([]string, 0, len(spec.Preselect))
	for _, name := range spec.Preselect {
		if name == "" {
			continue
		}
		if _, ok := mv.territoriesManager.Territories[name]; ok {
			pre = append(pre, name)
		}
	}

	pm.territorySelectorOverlay = NewTerritorySelectorOverlay(spec.Title, spec.MultiSelect, pre)
	pm.territorySelectorOverlay.OnAccept = func(selected []string) {
		if pm.territorySelectorOwner != "" {
			pluginhost.CompleteTerritorySelectorAccept(pm.territorySelectorOwner, selected)
			pm.territorySelectorOwner = ""
		}
	}
	pm.territorySelectorOverlay.OnCancel = func() {
		if pm.territorySelectorOwner != "" {
			pluginhost.CompleteTerritorySelectorCancel(pm.territorySelectorOwner)
			pm.territorySelectorOwner = ""
		}
	}
	pm.territorySelectorOverlay.Show()
}

func (pm *PluginManager) addPluginFromPath(path string) {
	pm.addPluginFromPathWithEnable(path, true)
}

func (pm *PluginManager) addPluginFromPathWithEnable(path string, enable bool) {
	cleaned := filepath.Clean(path)
	if cleaned == "" {
		return
	}

	stat, err := os.Stat(cleaned)
	if err != nil || stat.IsDir() {
		return
	}

	base := filepath.Base(cleaned)
	plugin := typedef.PluginState{
		ID:      cleaned,
		Name:    base,
		Path:    cleaned,
		Enabled: enable,
		Config: typedef.PluginConfig{
			AllowFileSystem:  false,
			AllowNetwork:     false,
			AllowCPU:         false,
			AllowTime:        false,
			AllowStateAccess: false,
			UserSettings:     make(map[string]any),
		},
	}

	for i := range pm.plugins {
		if pm.plugins[i].Path == cleaned {
			if !enable {
				plugin.Enabled = pm.plugins[i].Enabled
			}
			pm.plugins[i] = plugin
			pm.selectedIndex = i
			pm.updateToggleButtonText()
			if enable {
				pm.tryEnable(i)
			}
			return
		}
	}

	pm.plugins = append(pm.plugins, plugin)
	pm.selectedIndex = len(pm.plugins) - 1
	pm.updateToggleButtonText()
	if enable {
		pm.tryEnable(pm.selectedIndex)
	}
}

func (pm *PluginManager) toggleSelectedPlugin() {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	plugin := &pm.plugins[pm.selectedIndex]
	if plugin.Missing {
		plugin.Enabled = false
		plugin.LastError = "Binary missing"
	} else {
		if plugin.Enabled {
			plugin.Enabled = false
			plugin.LastError = ""
			if pm.host != nil {
				pm.host.Disable(plugin.Path)
			}
		} else {
			pm.tryEnable(pm.selectedIndex)
		}
	}
	pm.updateToggleButtonText()
}

func (pm *PluginManager) removeSelectedPlugin() {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	if pm.host != nil {
		pm.host.Disable(pm.plugins[pm.selectedIndex].Path)
	}
	pm.plugins = append(pm.plugins[:pm.selectedIndex], pm.plugins[pm.selectedIndex+1:]...)
	pm.selectedIndex = -1
	pm.updateToggleButtonText()
}

func (pm *PluginManager) updateToggleButtonText() {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		pm.toggleButton.Text = "Enable"
		pm.toggleButton.enabled = false
		pm.removeButton.enabled = false
		return
	}
	plugin := pm.plugins[pm.selectedIndex]
	if plugin.Enabled {
		pm.toggleButton.Text = "Disable"
	} else {
		pm.toggleButton.Text = "Enable"
	}
	pm.toggleButton.enabled = !plugin.Missing
	pm.removeButton.enabled = true
}

func (pm *PluginManager) tryEnable(index int) {
	if index < 0 || index >= len(pm.plugins) {
		return
	}
	state := pm.plugins[index]
	inst, err := pm.ensureLoaded(state)
	if err != nil {
		pm.plugins[index].Enabled = false
		pm.plugins[index].LastError = err.Error()
		return
	}

	pm.plugins[index].Enabled = true
	pm.plugins[index].Missing = false
	pm.plugins[index].LastError = ""

	if inst != nil && pm.host != nil {
		if snap, ok := pm.host.Snapshot(pm.plugins[index].Path); ok {
			snap.Enabled = true
			snap.Missing = false
			pm.plugins[index] = snap
		}
	}
}

func (pm *PluginManager) ensureLoaded(state typedef.PluginState) (*pluginhost.Instance, error) {
	if pm.host == nil {
		return nil, fmt.Errorf("plugin host unavailable")
	}
	inst, err := pm.host.Load(&state)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func (pm *PluginManager) syncRuntimeState() {
	if pm.host == nil {
		return
	}
	for i := range pm.plugins {
		if snap, ok := pm.host.Snapshot(pm.plugins[i].Path); ok {
			pm.plugins[i].StateBlob = snap.StateBlob
			pm.plugins[i].Config.UserSettings = snap.Config.UserSettings
			if snap.LastError != "" {
				pm.plugins[i].LastError = snap.LastError
			}
		}
	}
}

func (pm *PluginManager) handleSelectionChanged() {
	if pm.selectedIndex == pm.lastSelected {
		return
	}
	pm.lastSelected = pm.selectedIndex
	pm.controlScroll = 0
	pm.rebuildDynamicUI()
}

func pluginFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isPluginBinary(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".so", ".dll", ".dylib":
		return true
	default:
		return false
	}
}

func colorRGBA(r, g, b uint8) color.RGBA {
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func boolToI64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func toBool(v any) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case int:
		return val != 0, true
	case int64:
		return val != 0, true
	case float64:
		return val != 0, true
	default:
		return false, false
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

func toBasicResources(v any) (typedef.BasicResources, bool) {
	res := typedef.BasicResources{}
	m, ok := v.(map[string]any)
	if !ok {
		return res, false
	}
	if f, ok := toFloat(m["emeralds"]); ok {
		res.Emeralds = f
	}
	if f, ok := toFloat(m["ores"]); ok {
		res.Ores = f
	}
	if f, ok := toFloat(m["wood"]); ok {
		res.Wood = f
	}
	if f, ok := toFloat(m["fish"]); ok {
		res.Fish = f
	}
	if f, ok := toFloat(m["crops"]); ok {
		res.Crops = f
	}
	return res, true
}

func wrapTextFace(face font.Face, content string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{content}
	}
	words := strings.Fields(content)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0)
	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		if text.BoundString(face, candidate).Dx() <= maxWidth {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = word
	}
	lines = append(lines, line)
	return lines
}

func tributeETA(t *typedef.ActiveTribute) (uint64, uint64) {
	if t == nil {
		return 0, 0
	}
	interval := uint64(t.IntervalMinutes)
	if interval == 0 {
		interval = 1
	}
	tick := eruntime.GetCurrentTick()
	currentMinute := tick / 60
	lastMinute := t.CreatedAt / 60
	if t.LastTransfer > 0 {
		lastMinute = t.LastTransfer / 60
	}
	elapsed := currentMinute - lastMinute
	nextMinutes := interval - (elapsed % interval)
	if nextMinutes == 0 {
		nextMinutes = interval
	}
	remTicksThisMinute := tick % 60
	nextTicks := nextMinutes*60 - remTicksThisMinute
	return nextMinutes, nextTicks
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

var cwdOnce sync.Once
var cwd string

// displayPluginPath returns a stable relative path (when possible) for UI display while keeping absolute paths for storage.
func displayPluginPath(p string) string {
	if p == "" {
		return p
	}
	cwdOnce.Do(func() {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	})
	if cwd != "" {
		if rel, err := filepath.Rel(cwd, p); err == nil {
			return rel
		}
	}
	return p
}

func (pm *PluginManager) setUserSetting(id string, val any) {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	idx := pm.selectedIndex
	cfg := pm.plugins[idx].Config.UserSettings
	if cfg == nil {
		cfg = make(map[string]any)
		pm.plugins[idx].Config.UserSettings = cfg
	}
	cfg[id] = val
	pm.plugins[idx].Config.UserSettings = cfg

	if pm.host != nil {
		copyMap := make(map[string]any, len(cfg))
		for k, v := range cfg {
			copyMap[k] = v
		}
		if err := pm.host.UpdateSettings(pm.plugins[idx].Path, copyMap); err != nil {
		}
	}
}

func indexOf(options []string, target string) int {
	for i, opt := range options {
		if opt == target {
			return i
		}
	}
	return -1
}

type uiButtonEntry struct {
	btn     *EnhancedButton
	control pluginhost.UIControl
}

// rebuildDynamicUI creates clickable controls (currently only buttons) for the selected plugin's UI description.
func (pm *PluginManager) rebuildDynamicUI() {
	pm.uiButtons = make(map[string]*EnhancedButton)
	pm.uiCheckboxes = make(map[string]*Checkbox)
	pm.uiSliders = make(map[string]*MenuSlider)
	pm.uiTextInputs = make(map[string]*MenuTextInput)
	pm.uiSelects = make(map[string]*Dropdown)
	pm.uiLayout = make(map[string]layoutSlot)
	pm.controlContent = 0
	pm.controlScroll = 0
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	plugin := pm.plugins[pm.selectedIndex]
	settings := plugin.Config.UserSettings
	path := plugin.Path
	if pm.host == nil {
		return
	}
	if ctrls, ok := pm.host.UIControls(path); ok {
		pm.uiControls[path] = ctrls
	}

	ctrls := pm.uiControls[path]
	if len(ctrls) == 0 {
		return
	}

	// Layout controls vertically in the dedicated control surface panel.
	_, _, controlRect := pm.layoutRects()
	startX := controlRect.Min.X + 10
	startY := controlRect.Min.Y + 50
	btnW := controlRect.Dx() - 20
	btnH := 30
	spacing := 10
	curY := startY

	for _, c := range ctrls {
		key := path + "::" + c.ID
		switch c.Kind {
		case pluginhost.UIControlButton:
			btn := NewEnhancedButton(c.Label, startX, curY, btnW, btnH, nil)
			btn.OnClick = func(control pluginhost.UIControl) func() {
				return func() {
					_ = pm.host.SendUIEvent(path, pluginhost.UIEvent{ControlID: control.ID, Kind: pluginhost.UIControlButton})
				}
			}(c)
			pm.uiButtons[key] = btn
			pm.uiLayout[key] = layoutSlot{x: startX, y: curY, width: btnW, height: btnH}
			curY += btnH + spacing
		case pluginhost.UIControlCheckbox:
			cb := NewCheckbox(startX, curY, 18, c.Label, loadWynncraftFont(14))
			if val, ok := settings[c.ID]; ok {
				if b, bok := toBool(val); bok {
					cb.SetChecked(b)
				}
			}
			cb.SetOnClick(func(checked bool) {
				pm.setUserSetting(c.ID, checked)
				_ = pm.host.SendUIEvent(path, pluginhost.UIEvent{ControlID: c.ID, Kind: pluginhost.UIControlCheckbox, I64: boolToI64(checked)})
			})
			pm.uiCheckboxes[key] = cb
			pm.uiLayout[key] = layoutSlot{x: startX, y: curY, width: cb.Size, height: cb.Size}
			curY += cb.Size + spacing
		case pluginhost.UIControlSlider:
			sOpts := DefaultSliderOptions()
			sOpts.MinValue = c.MinValue
			sOpts.MaxValue = c.MaxValue
			if c.Step > 0 {
				sOpts.Step = c.Step
			}
			sOpts.ShowValue = true
			sOpts.ValueFormat = "%.2f"
			initial := c.MinValue
			if val, ok := settings[c.ID]; ok {
				if f, fok := toFloat(val); fok {
					initial = f
				}
			}
			slider := NewMenuSlider(c.Label, initial, sOpts, func(val float64) {
				pm.setUserSetting(c.ID, val)
				_ = pm.host.SendUIEvent(path, pluginhost.UIEvent{ControlID: c.ID, Kind: pluginhost.UIControlSlider, F64: val})
			})
			pm.uiSliders[key] = slider
			pm.uiLayout[key] = layoutSlot{x: startX, y: curY, width: btnW, height: sOpts.Height}
			curY += sOpts.Height + spacing
		case pluginhost.UIControlText:
			tOpts := DefaultTextInputOptions()
			tOpts.Width = btnW
			tOpts.Height = 28
			initial := ""
			if val, ok := settings[c.ID]; ok {
				if s, ok := val.(string); ok {
					initial = s
				}
			}
			textInput := NewMenuTextInput(c.Label, initial, tOpts, func(val string) {
				pm.setUserSetting(c.ID, val)
				_ = pm.host.SendUIEvent(path, pluginhost.UIEvent{ControlID: c.ID, Kind: pluginhost.UIControlText, Str: val})
			})
			pm.uiTextInputs[key] = textInput
			pm.uiLayout[key] = layoutSlot{x: startX, y: curY, width: btnW, height: tOpts.Height}
			curY += tOpts.Height + 12
		case pluginhost.UIControlSelect:
			if len(c.Options) == 0 {
				continue
			}
			dd := NewDropdown(startX, curY, btnW, btnH, c.Options, func(selected string) {
				idx := indexOf(c.Options, selected)
				pm.setUserSetting(c.ID, selected)
				_ = pm.host.SendUIEvent(path, pluginhost.UIEvent{ControlID: c.ID, Kind: pluginhost.UIControlSelect, I64: int64(idx)})
			})
			if val, ok := settings[c.ID]; ok {
				switch v := val.(type) {
				case string:
					if idx := indexOf(c.Options, v); idx >= 0 {
						dd.selectedIndex = idx
					}
				case int:
					if v >= 0 && v < len(c.Options) {
						dd.selectedIndex = v
					}
				case int64:
					if v >= 0 && int(v) < len(c.Options) {
						dd.selectedIndex = int(v)
					}
				}
			}
			pm.uiSelects[key] = dd
			pm.uiLayout[key] = layoutSlot{x: startX, y: curY, width: btnW, height: btnH}
			curY += btnH + spacing
		default:
			// unsupported type
		}
	}

	pm.controlContent = curY - startY
}

func (pm *PluginManager) updateDynamicUI(mx, my int) {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	viewTop, viewHeight, controlRect := pm.controlView()
	viewBottom := viewTop + viewHeight
	maxScroll := pm.controlContent - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		if mx >= controlRect.Min.X && mx < controlRect.Max.X && my >= controlRect.Min.Y && my < controlRect.Max.Y {
			pm.controlScroll -= int(wy * 20)
			if pm.controlScroll < 0 {
				pm.controlScroll = 0
			} else if pm.controlScroll > maxScroll {
				pm.controlScroll = maxScroll
			}
		}
	}
	for key, btn := range pm.uiButtons {
		if slot, ok := pm.uiLayout[key]; ok {
			btn.X = slot.x
			btn.Y = slot.y - pm.controlScroll
		}
		if btn.Y+btn.Height >= viewTop && btn.Y <= viewBottom {
			btn.Update(mx, my)
		}
	}
	mPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	for key, cb := range pm.uiCheckboxes {
		if slot, ok := pm.uiLayout[key]; ok {
			cb.X = slot.x
			cb.Y = slot.y - pm.controlScroll
		}
		if cb.Y+cb.Size >= viewTop && cb.Y <= viewBottom {
			cb.Update(mx, my, mPressed)
		}
	}
	delta := 1.0 / 60.0
	for key, sl := range pm.uiSliders {
		slot, ok := pm.uiLayout[key]
		if !ok {
			continue
		}
		y := slot.y - pm.controlScroll
		sl.sliderRect = image.Rect(slot.x+5, y+14, slot.x+slot.width-5, y+34)
		if y+slot.height >= viewTop && y <= viewBottom {
			sl.Update(mx, my, delta)
		}
	}
	for key, ti := range pm.uiTextInputs {
		slot, ok := pm.uiLayout[key]
		if !ok {
			continue
		}
		y := slot.y - pm.controlScroll
		ti.rect = image.Rect(slot.x, y, slot.x+slot.width, y+slot.height)
		if ti.rect.Min.Y+ti.rect.Dy() >= viewTop && ti.rect.Min.Y <= viewBottom {
			ti.Update(mx, my, delta)
		}
	}
	for key, dd := range pm.uiSelects {
		slot, ok := pm.uiLayout[key]
		if !ok {
			continue
		}
		dd.X = slot.x
		dd.Y = slot.y - pm.controlScroll
		dd.Width = slot.width
		if dd.Y+dd.Height >= viewTop && dd.Y <= viewBottom {
			dd.Update(mx, my)
		}
	}
}

func (pm *PluginManager) drawDynamicUI(screen *ebiten.Image) {
	if pm.selectedIndex < 0 || pm.selectedIndex >= len(pm.plugins) {
		return
	}
	viewTop, viewHeight, _ := pm.controlView()
	viewBottom := viewTop + viewHeight
	for key, btn := range pm.uiButtons {
		if slot, ok := pm.uiLayout[key]; ok {
			btn.X = slot.x
			btn.Y = slot.y - pm.controlScroll
		}
		if btn.Y+btn.Height < viewTop || btn.Y > viewBottom {
			continue
		}
		btn.Draw(screen)
	}
	for key, cb := range pm.uiCheckboxes {
		if slot, ok := pm.uiLayout[key]; ok {
			cb.X = slot.x
			cb.Y = slot.y - pm.controlScroll
		}
		if cb.Y+cb.Size < viewTop || cb.Y > viewBottom {
			continue
		}
		cb.Draw(screen)
	}
	font := loadWynncraftFont(16)
	for key, sl := range pm.uiSliders {
		slot, ok := pm.uiLayout[key]
		if !ok {
			continue
		}
		y := slot.y - pm.controlScroll
		if y+slot.height < viewTop || y > viewBottom {
			continue
		}
		sl.Draw(screen, slot.x, y, slot.width, font)
	}
	for key, ti := range pm.uiTextInputs {
		slot, ok := pm.uiLayout[key]
		if !ok {
			continue
		}
		y := slot.y - pm.controlScroll
		if y+slot.height < viewTop || y > viewBottom {
			continue
		}
		ti.Draw(screen, slot.x, y, slot.width, font)
	}
	for key, dd := range pm.uiSelects {
		if slot, ok := pm.uiLayout[key]; ok {
			dd.X = slot.x
			dd.Y = slot.y - pm.controlScroll
			dd.Width = slot.width
		}
		if dd.Y+dd.Height < viewTop || dd.Y > viewBottom {
			continue
		}
		dd.Draw(screen)
	}
}
