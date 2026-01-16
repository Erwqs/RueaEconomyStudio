package eruntime

import "RueaES/typedef"

func SetRuntimeOptions(options typedef.RuntimeOptions) {
	normalizeRuntimeOptions(&options)
	oldOptions := st.runtimeOptions
	st.runtimeOptions = options

	if options.TreasuryEnabled {
		// Ensure treasury levels are updated based on new options
		for _, territory := range st.territories {
			updateTreasuryLevel(territory)
		}
	}

	// If pathfinding algorithm changed, recalculate all routes
	if oldOptions.PathfindingAlgorithm != options.PathfindingAlgorithm {
		st.updateRoute()
	}
}

func GetRuntimeOptions() typedef.RuntimeOptions {
	return st.runtimeOptions
}
