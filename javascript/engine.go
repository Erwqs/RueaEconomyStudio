package javascript

import (
	"context"
	"etools/eruntime"
	"etools/typedef"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

type Eruntime struct{}

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

// Timeout after 60 seconds
func Execute(src, scriptName string) (goja.Value, error) {
	vm := goja.New()
	e := &Eruntime{}

	// Utility functions
	vm.Set("sprintf", fmt.Sprintf)
	vm.Set("printf", fmt.Printf)
	vm.Set("println", fmt.Println)
	vm.Set("eruntime", e)

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

// Run executes init() fuinction in the provided JavaScript source code.
// And tick() every state tick
func Run(src, scriptName string) (cancel chan struct{}) {
	vm := goja.New()
	e := &Eruntime{}

	// Utility functions
	vm.Set("sprintf", fmt.Sprintf)
	vm.Set("printf", fmt.Printf)
	vm.Set("println", fmt.Println)
	vm.Set("eruntime", e)

	go func() { // Run init() function
		if _, err := vm.RunString(src); err != nil {
			return
		}

		// Check if tick() and init() functions are defined
		if _, ok := vm.Get("tick").Export().(func()); !ok {
			return
		}

		if _, ok := vm.Get("init").Export().(func()); !ok {
			return
		}

		// Run init() function
		if _, err := vm.RunString("init()"); err != nil {
			return
		}

		for {
			select {
				case <-eruntime.GetStateTick():
					if _, err := vm.RunString("tick()"); err != nil {
						return
					}
				case <-cancel:
					return
				
			}
		}

	}()

	return
}
