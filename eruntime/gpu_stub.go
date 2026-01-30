package eruntime

import (
	"RueaES/typedef"
	"fmt"
)

var gpuComputeAvailable bool

// ensureGPUCompute validates GPU availability. This build uses CPU-only fallback.
func ensureGPUCompute() (bool, error) {
	gpuComputeAvailable = false
	return false, fmt.Errorf("GPU compute is not available in this build")
}

// handleGPUComputeFailure logs GPU initialization issues without crashing.
func handleGPUComputeFailure(stage string, err error) {
	debugf("GPU compute disabled (%s): %v\n", stage, err)
}

// UseGPUCompute reports whether GPU compute is enabled and available.
func UseGPUCompute() bool {
	return gpuComputeAvailable && st.runtimeOptions.ComputationSource == typedef.ComputationGPU
}

// computeNetResourcesBatch is a CPU-only stub; return false so callers fall back.
func computeNetResourcesBatch(_ []*typedef.Territory) (map[*typedef.Territory]typedef.BasicResources, bool) {
	return nil, false
}
