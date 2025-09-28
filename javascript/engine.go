package javascript

import (
	"RueaES/eruntime"
	"RueaES/typedef"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dop251/goja"
)

func init() {
	// Create scripts directory if it doesn't exist
	_, err := os.ReadDir("scripts")
	if err != nil {
		os.Mkdir("scripts", 0755)
	}
}

type Eruntime struct{}
type Utils struct{}

func (e *Eruntime) GetAllies() map[*typedef.Guild][]*typedef.Guild {
	return eruntime.GetAllies()
}

func (e *Eruntime) GetTerritoryStats(name string) *eruntime.TerritoryStats {
	return eruntime.GetTerritoryStats(name)
}

func (e *Eruntime) GetAllTerritoryStats() map[string]*eruntime.TerritoryStats {
	return eruntime.GetAllTerritoryStats()
}

func (e *Eruntime) GetSystemStats() *eruntime.SystemStats {
	return eruntime.GetSystemStats()
}

func (e *Eruntime) GetResourceMovementTimer() int {
	return eruntime.GetResourceMovementTimer()
}

func (e *Eruntime) GetTerritory(name string) *typedef.Territory {
	return eruntime.GetTerritory(name)
}

func (e *Eruntime) GetTerritories() []*typedef.Territory {
	return eruntime.GetTerritories()
}

func (e *Eruntime) SetGuild(territory string, guild typedef.Guild) *typedef.Territory {
	return eruntime.SetGuild(territory, guild)
}

func (e *Eruntime) SetGuildBatch(opts map[string]*typedef.Guild) []*typedef.Territory {
	return eruntime.SetGuildBatch(opts)
}

func (e *Eruntime) Set(territory string, opts typedef.TerritoryOptions) *typedef.Territory {
	return eruntime.Set(territory, opts)
}

func (e *Eruntime) ModifyStorageState(territory string, newState typedef.BasicResourcesInterface) *typedef.Territory {
	return eruntime.ModifyStorageState(territory, newState)
}

func (e *Eruntime) Halt() {
	eruntime.Halt()
}

func (e *Eruntime) Resume() {
	eruntime.Resume()
}

func (e *Eruntime) NextTick() {
	eruntime.NextTick()
}

func (e *Eruntime) StartTimer() {
	eruntime.StartTimer()
}

func (e *Eruntime) IsHalted() bool {
	return eruntime.IsHalted()
}

func (e *Eruntime) SetTickRate(tps int) {
	eruntime.SetTickRate(tps)
}

func (e *Eruntime) Reset() {
	eruntime.Reset()
}

func (e *Eruntime) GetTradingRoutes() map[string][]string {
	return eruntime.GetTradingRoutes()
}

func (e *Eruntime) GetTradingRoutesForTerritory(territory string) []string {
	return eruntime.GetTradingRoutesForTerritory(territory)
}

func (e *Eruntime) GetAllGuilds() []string {
	return eruntime.GetAllGuilds()
}

func (e *Eruntime) GetUpgradeCost(upgradeType string, level int) (int, string) {
	return eruntime.GetUpgradeCost(upgradeType, level)
}

func (e *Eruntime) SetTerritoryUpgrade(territory string, upgradeType string, level int) *typedef.Territory {
	return eruntime.SetTerritoryUpgrade(territory, upgradeType, level)
}

func (e *Eruntime) SetTerritoryBonus(territory string, bonusType string, level int) *typedef.Territory {
	return eruntime.SetTerritoryBonus(territory, bonusType, level)
}

func (e *Eruntime) GetBonusCost(bonusType string, level int) (int, string) {
	return eruntime.GetBonusCost(bonusType, level)
}

func (e *Eruntime) GetCost() *typedef.Costs {
	return eruntime.GetCost()
}

func (e *Eruntime) SetTreasuryOverride(t *typedef.Territory, level typedef.TreasuryOverride) {
	eruntime.SetTreasuryOverride(t, level)
}

func (u *Utils) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

func (u *Utils) Post(url string, ct string, body []byte) (*http.Response, error) {
	return http.Post(url, ct, bytes.NewBuffer(body))
}

func (e *Eruntime) NewTerritoryOptions() typedef.TerritoryOptions {
	return typedef.TerritoryOptions{}
}

func (e *Eruntime) NewBasicResources() typedef.BasicResources {
	return typedef.BasicResources{}
}

func (e *Eruntime) NewGuild() typedef.Guild {
	return typedef.Guild{}
}

// Timeout after 60 seconds
func Execute(src, scriptName string) (goja.Value, error) {
	vm := goja.New()
	e := &Eruntime{}
	u := &Utils{}

	// Utility functions
	vm.Set("sprintf", fmt.Sprintf)
	vm.Set("printf", fmt.Printf)
	vm.Set("println", fmt.Println)
	vm.Set("eruntime", e)
	vm.Set("utils", u)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(60*time.Second))
	defer cancel()

	// Channel to receive result or error
	resultCh := make(chan struct {
		val goja.Value
		err error
	})

	// Run script in a goroutine
	go func() {
		val, err := vm.RunString(src)
		resultCh <- struct {
			val goja.Value
			err error
		}{val, err}
	}()

	select {
	case <-ctx.Done():
		vm.Interrupt("timeout")
		return nil, fmt.Errorf("script %s timed out: %w", scriptName, ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, fmt.Errorf("failed to run script %s: %w", scriptName, res.err)
		}
		if res.val == nil || res.val == goja.Undefined() {
			return nil, fmt.Errorf("script %s returned no value", scriptName)
		}
		return res.val, nil
	}
}

// ScriptRuntime holds the runtime information for a running script
type ScriptRuntime struct {
	Cancel chan struct{}
	Error  chan error
	Done   chan struct{} // Signals when the script has completely finished
}

// Run executes init() fuinction in the provided JavaScript source code.
// And tick() every state tick
func Run(src, scriptName string) *ScriptRuntime {
	runtime := &ScriptRuntime{
		Cancel: make(chan struct{}),
		Error:  make(chan error, 1), // Buffered to prevent blocking
		Done:   make(chan struct{}), // Signals when script is completely done
	}

	vm := goja.New()
	e := &Eruntime{}
	u := &Utils{}

	// Utility functions
	vm.Set("sprintf", fmt.Sprintf)
	vm.Set("printf", fmt.Printf)
	vm.Set("println", fmt.Println)
	vm.Set("eruntime", e)
	vm.Set("utils", u)

	go func() { // Run init() function
		defer func() {
			// Always close the Done channel to signal script completion
			close(runtime.Done)
			if r := recover(); r != nil {
				runtime.Error <- fmt.Errorf("script panic: %v", r)
			}
		}()

		if _, err := vm.RunString(src); err != nil {
			fmt.Printf("[JS_ENGINE] Failed to parse script %s: %v\n", scriptName, err)
			runtime.Error <- fmt.Errorf("parse error: %w", err)
			return
		}

		// Check if tick() and init() functions are defined
		tickFunc := vm.Get("tick")
		if tickFunc == nil || goja.IsUndefined(tickFunc) || goja.IsNull(tickFunc) {
			fmt.Printf("[JS_ENGINE] Script %s: tick() function not found\n", scriptName)
			runtime.Error <- fmt.Errorf("tick() function not found")
			return
		}

		initFunc := vm.Get("init")
		if initFunc == nil || goja.IsUndefined(initFunc) || goja.IsNull(initFunc) {
			fmt.Printf("[JS_ENGINE] Script %s: init() function not found\n", scriptName)
			runtime.Error <- fmt.Errorf("init() function not found")
			return
		}

		// Call init() function
		if _, err := vm.RunString("init()"); err != nil {
			fmt.Printf("[JS_ENGINE] Failed to run init() in script %s: %v\n", scriptName, err)
			runtime.Error <- fmt.Errorf("init() error: %w", err)
			return
		}
		fmt.Printf("[JS_ENGINE] Script %s initialized successfully\n", scriptName)

		for {
			select {
			case <-eruntime.GetStateTick():
				if _, err := vm.RunString("tick()"); err != nil {
					fmt.Printf("[JS_ENGINE] Error in tick() for script %s: %v\n", scriptName, err)
					runtime.Error <- fmt.Errorf("tick() error: %w", err)
					return
				}
			case <-runtime.Cancel:
				fmt.Printf("[JS_ENGINE] Script %s terminated\n", scriptName)
				return
			}
		}

	}()

	return runtime
}

func GetScripts() ([]string, error) {
	files, err := os.ReadDir("scripts")
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory: %w", err)
	}

	scripts := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && (file.Name()[len(file.Name())-3:] == ".js") {
			scripts = append(scripts, file.Name())
		}
	}

	return scripts, nil
}

// JavaScript-friendly wrapper types with getter/setter methods
type JSEruntime struct{}
type JSTerritoryOptions struct {
	opts typedef.TerritoryOptions
}

type JSBasicResources struct {
	res typedef.BasicResources
}

type JSGuild struct {
	guild typedef.Guild
}

// TerritoryOptions wrapper methods - Upgrades
func (jsto *JSTerritoryOptions) SetDamageUpgrade(value int) {
	jsto.opts.Upgrades.Damage = value
}

func (jsto *JSTerritoryOptions) GetDamageUpgrade() int {
	return jsto.opts.Upgrades.Damage
}

func (jsto *JSTerritoryOptions) SetAttackUpgrade(value int) {
	jsto.opts.Upgrades.Attack = value
}

func (jsto *JSTerritoryOptions) GetAttackUpgrade() int {
	return jsto.opts.Upgrades.Attack
}

func (jsto *JSTerritoryOptions) SetHealthUpgrade(value int) {
	jsto.opts.Upgrades.Health = value
}

func (jsto *JSTerritoryOptions) GetHealthUpgrade() int {
	return jsto.opts.Upgrades.Health
}

func (jsto *JSTerritoryOptions) SetDefenceUpgrade(value int) {
	jsto.opts.Upgrades.Defence = value
}

func (jsto *JSTerritoryOptions) GetDefenceUpgrade() int {
	return jsto.opts.Upgrades.Defence
}

// TerritoryOptions wrapper methods - Bonuses
func (jsto *JSTerritoryOptions) SetStrongerMinionsBonus(value int) {
	jsto.opts.Bonuses.StrongerMinions = value
}

func (jsto *JSTerritoryOptions) GetStrongerMinionsBonus() int {
	return jsto.opts.Bonuses.StrongerMinions
}

func (jsto *JSTerritoryOptions) SetTowerMultiAttackBonus(value int) {
	jsto.opts.Bonuses.TowerMultiAttack = value
}

func (jsto *JSTerritoryOptions) GetTowerMultiAttackBonus() int {
	return jsto.opts.Bonuses.TowerMultiAttack
}

func (jsto *JSTerritoryOptions) SetResourceRateBonus(value int) {
	jsto.opts.Bonuses.ResourceRate = value
}

func (jsto *JSTerritoryOptions) GetResourceRateBonus() int {
	return jsto.opts.Bonuses.ResourceRate
}

func (jsto *JSTerritoryOptions) SetEmeraldRateBonus(value int) {
	jsto.opts.Bonuses.EmeraldRate = value
}

func (jsto *JSTerritoryOptions) GetEmeraldRateBonus() int {
	return jsto.opts.Bonuses.EmeraldRate
}

// BasicResources wrapper methods
func (jsbr *JSBasicResources) SetEmeralds(value float64) {
	jsbr.res.Emeralds = value
}

func (jsbr *JSBasicResources) GetEmeralds() float64 {
	return jsbr.res.Emeralds
}

func (jsbr *JSBasicResources) SetOres(value float64) {
	jsbr.res.Ores = value
}

func (jsbr *JSBasicResources) GetOres() float64 {
	return jsbr.res.Ores
}

func (jsbr *JSBasicResources) SetWood(value float64) {
	jsbr.res.Wood = value
}

func (jsbr *JSBasicResources) GetWood() float64 {
	return jsbr.res.Wood
}

func (jsbr *JSBasicResources) SetFish(value float64) {
	jsbr.res.Fish = value
}

func (jsbr *JSBasicResources) GetFish() float64 {
	return jsbr.res.Fish
}

func (jsbr *JSBasicResources) SetCrops(value float64) {
	jsbr.res.Crops = value
}

func (jsbr *JSBasicResources) GetCrops() float64 {
	return jsbr.res.Crops
}

// Guild wrapper methods
func (jsg *JSGuild) SetName(value string) {
	jsg.guild.Name = value
}

func (jsg *JSGuild) GetName() string {
	return jsg.guild.Name
}

func (jsg *JSGuild) SetTag(value string) {
	jsg.guild.Tag = value
}

func (jsg *JSGuild) GetTag() string {
	return jsg.guild.Tag
}

// Enhanced Eruntime methods that work with JavaScript-friendly wrappers
func (e *Eruntime) NewJSTerritoryOptions() *JSTerritoryOptions {
	return &JSTerritoryOptions{opts: typedef.TerritoryOptions{}}
}

func (e *Eruntime) NewJSBasicResources() *JSBasicResources {
	return &JSBasicResources{res: typedef.BasicResources{}}
}

func (e *Eruntime) NewJSGuild() *JSGuild {
	return &JSGuild{guild: typedef.Guild{}}
}

// Modified Set method that accepts JS wrapper
func (e *Eruntime) SetWithJS(territory string, options *JSTerritoryOptions) *typedef.Territory {
	return eruntime.Set(territory, options.opts)
}

// Modified ModifyStorageState that accepts JS wrapper
func (e *Eruntime) ModifyStorageStateWithJS(territory string, newState *JSBasicResources) *typedef.Territory {
	return eruntime.ModifyStorageState(territory, &newState.res)
}

// Modified SetGuild that accepts JS wrapper
func (e *Eruntime) SetGuildWithJS(territory string, guild *JSGuild) *typedef.Territory {
	return eruntime.SetGuild(territory, guild.guild)
}
