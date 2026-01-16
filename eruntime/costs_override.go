package eruntime

import (
	"encoding/json"
	"errors"
	"fmt"

	"RueaES/assets"
	"RueaES/typedef"
)

// ReloadDefaultCosts loads the built-in costs from assets/upgrades.json.
func ReloadDefaultCosts() error {
	data, err := assets.AssetFiles.ReadFile("upgrades.json")
	if err != nil {
		return err
	}
	return applyCostsJSON(data)
}

// SetCostsFromMap replaces the in-memory costs using a map payload (same shape as upgrades.json).
func SetCostsFromMap(payload map[string]any) error {
	if payload == nil {
		return errors.New("nil cost payload")
	}

	// Support envelope {"json": "..."} used by plugins to pass raw JSON string.
	if raw, ok := payload["json"]; ok {
		switch v := raw.(type) {
		case string:
			return applyCostsJSON([]byte(v))
		case []byte:
			return applyCostsJSON(v)
		default:
			return fmt.Errorf("json field has unsupported type %T", v)
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return applyCostsJSON(data)
}

func applyCostsJSON(data []byte) error {
	var costs typedef.Costs
	if err := json.Unmarshal(data, &costs); err != nil {
		return err
	}

	// Basic validation to avoid empty slices causing panics.
	ensureInt := func(slice []int, fallback int) []int {
		if len(slice) == 0 {
			return []int{fallback}
		}
		return slice
	}
	ensureFloat := func(slice []float64, fallback float64) []float64 {
		if len(slice) == 0 {
			return []float64{fallback}
		}
		return slice
	}
	// Clamp upgrade multipliers to at least one entry.
	costs.UpgradeMultiplier.Damage = ensureFloat(costs.UpgradeMultiplier.Damage, 1)
	costs.UpgradeMultiplier.Attack = ensureFloat(costs.UpgradeMultiplier.Attack, 1)
	costs.UpgradeMultiplier.Health = ensureFloat(costs.UpgradeMultiplier.Health, 1)
	costs.UpgradeMultiplier.Defence = ensureFloat(costs.UpgradeMultiplier.Defence, 1)

	// Clamp upgrade costs to at least one zero entry.
	costs.UpgradesCost.Damage.Value = ensureInt(costs.UpgradesCost.Damage.Value, 0)
	costs.UpgradesCost.Attack.Value = ensureInt(costs.UpgradesCost.Attack.Value, 0)
	costs.UpgradesCost.Health.Value = ensureInt(costs.UpgradesCost.Health.Value, 0)
	costs.UpgradesCost.Defence.Value = ensureInt(costs.UpgradesCost.Defence.Value, 0)

	// Clamp bonus costs/value slices to at least one zero entry to prevent indexing crashes.
	padBonus := func(b *typedef.BonusCosts) {
		b.Cost = ensureInt(b.Cost, 0)
		b.Value = ensureFloat(b.Value, 0)
	}
	padBonus(&costs.Bonuses.StrongerMinions)
	padBonus(&costs.Bonuses.TowerMultiAttack)
	padBonus(&costs.Bonuses.TowerAura)
	padBonus(&costs.Bonuses.TowerVolley)
	padBonus(&costs.Bonuses.GatheringExperience)
	padBonus(&costs.Bonuses.MobExperience)
	padBonus(&costs.Bonuses.MobDamage)
	padBonus(&costs.Bonuses.PvPDamage)
	padBonus(&costs.Bonuses.XPSeeking)
	padBonus(&costs.Bonuses.TomeSeeking)
	padBonus(&costs.Bonuses.EmeraldsSeeking)
	padBonus(&costs.Bonuses.LargerResourceStorage)
	padBonus(&costs.Bonuses.LargerEmeraldsStorage)
	padBonus(&costs.Bonuses.EfficientResource)
	padBonus(&costs.Bonuses.EfficientEmeralds)
	padBonus(&costs.Bonuses.ResourceRate)
	padBonus(&costs.Bonuses.EmeraldsRate)

	st.costs = costs
	return nil
}
