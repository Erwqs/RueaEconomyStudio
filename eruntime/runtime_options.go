package eruntime

import "RueaES/typedef"

func SetRuntimeOptions(options typedef.RuntimeOptions) {
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

	// If GPU compute setting changed, update the compute controller
	if oldOptions.GPUComputeEnabled != options.GPUComputeEnabled {
		if GlobalComputeController != nil {
			if options.GPUComputeEnabled {
				// Try to enable GPU mode
				if err := GlobalComputeController.SetComputeMode("hybrid"); err != nil {
					// If failed, revert the option to false
					st.runtimeOptions.GPUComputeEnabled = false
				}
			} else {
				// Disable GPU mode
				GlobalComputeController.SetComputeMode("cpu")
			}
		}
	}
}

func GetRuntimeOptions() typedef.RuntimeOptions {
	return st.runtimeOptions
}
