package eruntime

import "RueaES/typedef"

func SetRuntimeOptions(options typedef.RuntimeOptions) {
	st.runtimeOptions = options
	if options.TreasuryEnabled {
		// Ensure treasury levels are updated based on new options
		for _, territory := range st.territories {
			updateTreasuryLevel(territory)
		}
	}
}

func GetRuntimeOptions() typedef.RuntimeOptions {
	return st.runtimeOptions
}
