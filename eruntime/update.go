package eruntime

// update calculates the next state of the simulation
// update() handles resource generation, territorial defences
// should be called every second
func (s *state) update() {
	// 1: Calculate upgrades and bonus upgrade, apply to Net and Resource
	// 2: Calculate resource generation from generation bonus, use multiplier level from .At
	// 3: Add/subtract net to/from final resource generation
	// 4.1: If subtracting results in negative resources, then we cannot afford any upgrades/bonuses that use that resource, we will need to set the .At of that upgrade/bonuses to 0, else set it to .Set . Do not consume resource if we cannot afford the upgrade/bonus if it results in negative resources.
	//      If it does not result to negative resources, we will set the .At of that upgrade/bonuses to .Set
	// 4.2: If adding results in more resource than storage allows, we need to cap the resource to the storage limit
	// 5: Calculate territory damage and defence based on .At field

	// Negative net generation doesn't mean that we cannot afford the upgrade, but it means that HQ needs to send extra resources to this territory (if HQ exists, if not then the .At will oscillate between Set and 0 due to negative net generation and generation is not enough to cover the cost)
	for _, territory := range s.territories {
		if territory == nil {
			continue
		}

		// Calculate generation potential for this territory (this handles affordability checks internally)
		_, _, _, _ = calculateGeneration(territory)

		// Actually perform the resource generation/consumption
		doGenerate(territory)

		// Calculate tower stats based on At upgrade levels (step 5)
		territory.TowerStats = calculateTowerStats(territory)
		updateTreasuryLevel(territory)
		updateGenerationBonus(territory)
	}
}

// update2 calculates the next state of the simulation
// update2() handles resource traversal, routing, storage rounding
// should be called every minute before update()
func (s *state) update2() {
	ResourceTraversalAndTaxV2() // Using new decoupled transit system
	s.ClampResource()
}
