package eruntime

import (
	"runtime"
	"sync"
)

// update calculates the next state of the simulation
// update() handles resource generation, territorial defences
// should be called every second
func (s *state) update() {
	// Choose processing method based on configuration
	if s.useParallelProcessing {
		s.updateParallel()
	} else {
		s.updateSequential()
	}
}

// updateParallel processes territories in parallel for better performance
func (s *state) updateParallel() {
	numWorkers := min(
		// Use all available CPU cores
		runtime.NumCPU(),
		// Cap at 8 to avoid too much overhead
		8)

	territoryCount := len(s.territories)
	if territoryCount == 0 {
		return
	}

	// Calculate chunk size for each worker
	chunkSize := territoryCount / numWorkers
	if chunkSize == 0 {
		chunkSize = 1
	}

	var wg sync.WaitGroup

	// Process territories in parallel chunks
	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			start := workerID * chunkSize
			end := start + chunkSize
			if workerID == numWorkers-1 {
				end = territoryCount // Last worker handles remaining territories
			}

			// Process this worker's chunk of territories
			for j := start; j < end && j < territoryCount; j++ {
				territory := s.territories[j]
				if territory == nil {
					continue
				}

				// All territory processing functions already use individual territory locks
				// so parallel processing is safe

				// Calculate generation potential for this territory (this handles affordability checks internally)
				_, _, _, _ = calculateGeneration(territory)

				// Actually perform the resource generation/consumption
				doGenerate(territory)

				// Calculate tower stats based on At upgrade levels (step 5)
				territory.TowerStats = calculateTowerStats(territory)
				updateTreasuryLevel(territory)
				updateGenerationBonus(territory)
			}
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
}

// updateSequential is the original sequential implementation for comparison/fallback
func (s *state) updateSequential() {
	// Original sequential implementation
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
// update2() handles resource traversal, routing, storage rounding, and tribute processing
// should be called every minute before update()
func (s *state) update2() {
	ResourceTraversalAndTaxV2() // Using new decoupled transit system
	s.processTributes()         // Process tribute transfers
	s.ClampResource()
}
