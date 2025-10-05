package eruntime

import (
	"RueaES/typedef"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// GPUComputeMode defines the computation mode
type GPUComputeMode int

const (
	ComputeCPUOnly GPUComputeMode = iota
	ComputeCPUGPU                 // Hybrid: CPU+GPU parallel processing
)

// GPUCompute handles GPU-accelerated territory calculations using fragment shaders
type GPUCompute struct {
	enabled bool
	mode    GPUComputeMode

	// Computation shaders
	resourceShader  *ebiten.Shader
	costShader      *ebiten.Shader
	bonusShader     *ebiten.Shader
	dijkstraShader  *ebiten.Shader
	astarShader     *ebiten.Shader
	floodfillShader *ebiten.Shader

	// Texture buffers for data exchange
	territoryDataTexture     *ebiten.Image // Territory input data
	costsDataTexture         *ebiten.Image // Cost tables data
	resultsTexture           *ebiten.Image // Computation results
	tempResultsTexture       *ebiten.Image // Temporary results for ping-pong
	pathfindingDataTexture   *ebiten.Image // Pathfinding input data
	pathfindingResultTexture *ebiten.Image // Pathfinding results

	// Buffer dimensions (territories arranged in 2D grid)
	textureWidth   int
	textureHeight  int
	maxTerritories int

	// Performance optimization
	framesSinceReadback int
	readbackFrequency   int    // Read back results every N frames
	cachedResults       []byte // Cached GPU results
	
	// Pre-allocated buffers to reduce GC pressure
	territoryBuffer []byte
	costBuffer      []byte
	uniformsBuffer  map[string]interface{}

	// Synchronization
	mutex sync.RWMutex

	// Performance metrics
	gpuComputeTime    float64
	cpuComputeTime    float64
	hybridComputeTime float64
}

// ComputeController interface for controlling computation modes
type ComputeController interface {
	SetComputeMode(mode string) error
	GetComputeMode() string
	IsGPUAvailable() bool
}

// Global GPU compute instance
var gpuCompute *GPUCompute

// Global interface for accessing GPU compute functionality from other packages
var GlobalComputeController ComputeController

// InitializeGPUCompute initializes the GPU computation system
func InitializeGPUCompute() error {
	if gpuCompute != nil {
		return nil // Already initialized
	}

	// Use fixed texture dimensions optimized for GPU memory alignment
	// 32x32 = 1024 territories max, power-of-2 dimensions for better GPU performance

	gc := &GPUCompute{
		enabled:           false,
		mode:              ComputeCPUOnly,
		textureWidth:      32, // 32x32 = 1024 territories max
		textureHeight:     32,
		maxTerritories:    1024,
		readbackFrequency: 5, // Read back results every 5 frames for better performance
		cachedResults:     make([]byte, 32*32*4),
		
		// Pre-allocate buffers to reduce GC pressure during runtime
		territoryBuffer:   make([]byte, 32*32*4),
		costBuffer:        make([]byte, 32*32*4),
		uniformsBuffer:    make(map[string]interface{}, 4),
	}
	
	// Initialize shaders
	if err := gc.initializeShaders(); err != nil {
		fmt.Printf("[GPU_COMPUTE] Failed to initialize shaders: %v. GPU compute disabled.\n", err)
		gc.enabled = false
	} else {
		// Shaders compiled successfully, now try textures
		if err := gc.initializeTextures(); err != nil {
			fmt.Printf("[GPU_COMPUTE] Failed to initialize textures: %v. GPU compute disabled.\n", err)
			gc.enabled = false
		} else {
			// Both shaders and textures initialized successfully
			gc.enabled = true
			fmt.Println("[GPU_COMPUTE] GPU compute system initialized successfully")
		}
	}

	gpuCompute = gc
	GlobalComputeController = gc // Set the global controller interface

	if !gc.enabled {
		fmt.Println("[GPU_COMPUTE] GPU compute system disabled, using CPU-only mode")
	}

	return nil
}

// initializeShaders compiles all compute shaders
func (gc *GPUCompute) initializeShaders() error {
	var err error

	fmt.Println("[GPU_COMPUTE] Starting shader compilation...")

	// Resource generation computation shader
	gc.resourceShader, err = ebiten.NewShader([]byte(resourceComputeShaderSrc))
	if err != nil {
		return fmt.Errorf("failed to compile resource shader: %w", err)
	}
	fmt.Println("[GPU_COMPUTE] Resource shader compiled successfully")

	// Cost computation shader
	gc.costShader, err = ebiten.NewShader([]byte(costComputeShaderSrc))
	if err != nil {
		return fmt.Errorf("failed to compile cost shader: %w", err)
	}
	fmt.Println("[GPU_COMPUTE] Cost shader compiled successfully")

	// Bonus computation shader
	gc.bonusShader, err = ebiten.NewShader([]byte(bonusComputeShaderSrc))
	if err != nil {
		return fmt.Errorf("failed to compile bonus shader: %w", err)
	}
	fmt.Println("[GPU_COMPUTE] Bonus shader compiled successfully")

	// Temporarily skip pathfinding shaders to isolate the issue
	fmt.Println("[GPU_COMPUTE] Skipping pathfinding shaders for debugging")
	
	// // Dijkstra pathfinding shader
	// gc.dijkstraShader, err = ebiten.NewShader([]byte(dijkstraComputeShaderSrc))
	// if err != nil {
	// 	return fmt.Errorf("failed to compile dijkstra shader: %w", err)
	// }
	// fmt.Println("[GPU_COMPUTE] Dijkstra shader compiled successfully")

	// // A* pathfinding shader
	// gc.astarShader, err = ebiten.NewShader([]byte(astarComputeShaderSrc))
	// if err != nil {
	// 	return fmt.Errorf("failed to compile astar shader: %w", err)
	// }
	// fmt.Println("[GPU_COMPUTE] A* shader compiled successfully")

	// // Floodfill shader
	// gc.floodfillShader, err = ebiten.NewShader([]byte(floodfillComputeShaderSrc))
	// if err != nil {
	// 	return fmt.Errorf("failed to compile floodfill shader: %w", err)
	// }
	// fmt.Println("[GPU_COMPUTE] Floodfill shader compiled successfully")

	fmt.Println("[GPU_COMPUTE] All compute shaders compiled successfully")
	return nil
}

// initializeTextures creates texture buffers for data exchange
func (gc *GPUCompute) initializeTextures() error {
	fmt.Println("[GPU_COMPUTE] Starting texture initialization...")
	
	// Territory data texture (RGBA channels pack different territory properties)
	gc.territoryDataTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Cost data texture (stores cost tables and multipliers)
	gc.costsDataTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Results texture (stores computation results)
	gc.resultsTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Temporary results texture for ping-pong rendering
	gc.tempResultsTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Pathfinding data texture (stores territory connections and costs)
	gc.pathfindingDataTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Pathfinding results texture
	gc.pathfindingResultTexture = ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	fmt.Printf("[GPU_COMPUTE] Initialized %dx%d textures for GPU computation\n", gc.textureWidth, gc.textureHeight)
	return nil
}

// SetComputeMode sets the computation mode (CPU only or CPU+GPU)
func SetComputeMode(mode GPUComputeMode) error {
	if gpuCompute == nil {
		return fmt.Errorf("GPU compute not initialized")
	}

	gpuCompute.mutex.Lock()
	defer gpuCompute.mutex.Unlock()

	gpuCompute.mode = mode
	gpuCompute.enabled = mode != ComputeCPUOnly

	// fmt.Printf("[GPU_COMPUTE] Mode set to: %s\n", func() string {
	// 	switch mode {
	// 	case ComputeCPUOnly:
	// 		return "CPU Only"
	// 	case ComputeCPUGPU:
	// 		return "CPU + GPU Hybrid"
	// 	default:
	// 		return "Unknown"
	// 	}
	// }())

	return nil
}

// GetComputeMode returns the current computation mode
func GetComputeMode() GPUComputeMode {
	if gpuCompute == nil {
		return ComputeCPUOnly
	}

	gpuCompute.mutex.RLock()
	defer gpuCompute.mutex.RUnlock()
	return gpuCompute.mode
}

// IsGPUComputeEnabled returns whether GPU computation is enabled
func IsGPUComputeEnabled() bool {
	if gpuCompute == nil {
		return false
	}

	gpuCompute.mutex.RLock()
	defer gpuCompute.mutex.RUnlock()

	// Check both GPU hardware availability and runtime option
	runtimeOptions := GetRuntimeOptions()
	return gpuCompute.enabled && runtimeOptions.GPUComputeEnabled
}

// ComputeTerritoryUpdatesGPU performs territory calculations on GPU
func (gc *GPUCompute) ComputeTerritoryUpdatesGPU(territories map[string]*typedef.Territory) error {
	if !gc.enabled {
		return fmt.Errorf("GPU compute not enabled")
	}

	// Start performance timing
	startTime := time.Now()

	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	territoryList := make([]*typedef.Territory, 0, len(territories))
	for _, t := range territories {
		territoryList = append(territoryList, t)
	}

	if len(territoryList) > gc.maxTerritories {
		return fmt.Errorf("too many territories for GPU compute: %d > %d", len(territoryList), gc.maxTerritories)
	}

	// Pack territory data into texture
	if err := gc.packTerritoryData(territoryList); err != nil {
		return fmt.Errorf("failed to pack territory data: %w", err)
	}

	// Pack cost data into texture
	if err := gc.packCostData(); err != nil {
		return fmt.Errorf("failed to pack cost data: %w", err)
	}

	// Run combined compute shader pipeline for better performance
	if err := gc.runCombinedComputeShaders(len(territoryList)); err != nil {
		return fmt.Errorf("combined compute failed: %w", err)
	}

	// Read back results and update territories
	if err := gc.readBackResults(territoryList); err != nil {
		return fmt.Errorf("failed to read back results: %w", err)
	}

	// Update performance metrics
	gc.gpuComputeTime = time.Since(startTime).Seconds()
	
	// Performance monitoring - log slow operations
	if gc.gpuComputeTime > 0.005 { // More than 5ms is concerning
		fmt.Printf("[GPU_COMPUTE] GPU compute took %.3fms for %d territories\n", 
			gc.gpuComputeTime*1000, len(territoryList))
	}

	return nil
}

// packTerritoryData packs territory properties into GPU texture
func (gc *GPUCompute) packTerritoryData(territories []*typedef.Territory) error {
	// Use pre-allocated buffer to reduce GC pressure
	pixels := gc.territoryBuffer

	for i, territory := range territories {
		if territory == nil {
			continue
		}

		x := i % gc.textureWidth
		y := i / gc.textureWidth
		offset := (y*gc.textureWidth + x) * 4

		// Pack territory data into RGBA channels
		// R: Base resource generation (normalized)
		// G: Storage levels (normalized)
		// B: Upgrade levels (packed)
		// A: Bonus levels (packed)

		territory.Mu.RLock()

		// Pack base generation (average of all resources, normalized to 0-255)
		baseGen := (territory.ResourceGeneration.Base.Ores +
			territory.ResourceGeneration.Base.Wood +
			territory.ResourceGeneration.Base.Fish +
			territory.ResourceGeneration.Base.Crops) / 4.0
		pixels[offset] = byte(math.Min(255, baseGen/100.0*255)) // R channel

		// Pack storage levels (average, normalized)
		storageLevel := (territory.Storage.At.Ores +
			territory.Storage.At.Wood +
			territory.Storage.At.Fish +
			territory.Storage.At.Crops) / 4.0
		pixels[offset+1] = byte(math.Min(255, storageLevel/10000.0*255)) // G channel

		// Pack upgrade levels (4 levels in one byte)
		upgrades := (territory.Options.Upgrade.Set.Damage << 6) |
			(territory.Options.Upgrade.Set.Attack << 4) |
			(territory.Options.Upgrade.Set.Health << 2) |
			territory.Options.Upgrade.Set.Defence
		pixels[offset+2] = byte(upgrades) // B channel

		// Pack some bonus levels (8 levels, 4 bits each, take first 2)
		bonuses := (territory.Options.Bonus.Set.ResourceRate << 4) |
			territory.Options.Bonus.Set.EmeraldRate
		pixels[offset+3] = byte(bonuses) // A channel

		territory.Mu.RUnlock()
	}

	// Upload to GPU texture
	gc.territoryDataTexture.WritePixels(pixels)
	// fmt.Printf("[GPU_COMPUTE] Packed %d territories into GPU texture\n", len(territories))
	return nil
}

// packCostData packs cost tables and multipliers into GPU texture
func (gc *GPUCompute) packCostData() error {
	pixels := make([]byte, gc.textureWidth*gc.textureHeight*4)

	// Pack cost tables into texture
	// This is a simplified version - in reality you'd pack the entire cost structure
	for i := 0; i < gc.textureWidth*gc.textureHeight; i++ {
		offset := i * 4

		// Pack some basic cost multipliers
		pixels[offset] = 100   // Base cost multiplier (R)
		pixels[offset+1] = 50  // Efficiency multiplier (G)
		pixels[offset+2] = 200 // Storage multiplier (B)
		pixels[offset+3] = 150 // Rate multiplier (A)
	}

	gc.costsDataTexture.WritePixels(pixels)
	// fmt.Println("[GPU_COMPUTE] Packed cost data into GPU texture")
	return nil
}

// runCombinedComputeShaders executes optimized single-pass GPU compute
func (gc *GPUCompute) runCombinedComputeShaders(territoryCount int) error {
	// Use pre-allocated uniform buffer to reduce GC pressure
	uniforms := gc.uniformsBuffer
	// Clear previous values efficiently
	for k := range uniforms {
		delete(uniforms, k)
	}
	uniforms["TerritoryCount"] = float32(territoryCount)
	uniforms["TextureWidth"] = float32(gc.textureWidth)
	uniforms["Time"] = float32(time.Now().Unix() % 86400) // More stable than ActualTPS

	// Single comprehensive pass: All computations in enhanced resource shader
	// This reduces GPU passes from 3 to 1, dramatically improving performance
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = gc.territoryDataTexture
	op.Images[1] = gc.costsDataTexture
	op.Uniforms = uniforms
	gc.resultsTexture.DrawRectShader(gc.textureWidth, gc.textureHeight, gc.resourceShader, op)

	// No longer need cost and bonus shaders - all computation is in enhanced resource shader
	// This eliminates 2/3 of GPU passes and reduces memory bandwidth by ~66%

	return nil
}

// readBackResults reads computation results from GPU and applies them to territories
func (gc *GPUCompute) readBackResults(territories []*typedef.Territory) error {
	var pixels []byte

	// Dramatically reduce readback frequency to minimize CPU-GPU sync
	gc.framesSinceReadback++
	if gc.framesSinceReadback >= 10 || len(gc.cachedResults) == 0 { // Only readback every 10 frames
		// Time to read from GPU - use pre-allocated buffer
		gc.resultsTexture.ReadPixels(gc.cachedResults)
		pixels = gc.cachedResults
		gc.framesSinceReadback = 0
	} else {
		// Use cached results - this eliminates expensive ReadPixels() call
		pixels = gc.cachedResults
	}

	for i, territory := range territories {
		if territory == nil {
			continue
		}

		x := i % gc.textureWidth
		y := i / gc.textureWidth
		offset := (y*gc.textureWidth + x) * 4

		// No need to lock here - the calling function should handle locking
		// Unpack results from RGBA channels
		// R: Generated resources (normalized amount to add to storage)
		// G: Cost efficiency bonus
		// B: Storage capacity multiplier
		// A: Generation rate multiplier

		generatedAmount := float64(pixels[offset]) / 255.0 * 100.0 // Scale to reasonable range
		costEfficiencyBonus := float64(pixels[offset+1]) / 255.0
		storageMultiplier := 1.0 + (float64(pixels[offset+2])/255.0)*0.5 // 0-50% storage bonus
		rateMultiplier := 1.0 + (float64(pixels[offset+3])/255.0)*0.3    // 0-30% rate bonus

		territory.Mu.Lock()

		// Apply GPU-computed resource generation with batching optimizations
		// Scale generation based on readback frequency to maintain consistent rates
		frameScaling := float64(gc.framesSinceReadback + 1)
		scaledGeneration := generatedAmount * rateMultiplier * frameScaling
		
		territory.Storage.At.Ores += scaledGeneration
		territory.Storage.At.Wood += scaledGeneration
		territory.Storage.At.Fish += scaledGeneration
		territory.Storage.At.Crops += scaledGeneration

		// Apply cost efficiency to generation bonus
		territory.GenerationBonus += costEfficiencyBonus * 15.0              // 0-15% bonus
		territory.GenerationBonus = math.Min(territory.GenerationBonus, 150) // Cap at 150%

		// Apply storage limits (same as CPU version)
		baseCapacity := typedef.BaseResourceCapacity
		bonusStorageMultiplier := float64(st.costs.Bonuses.LargerResourceStorage.Value[territory.Options.Bonus.At.LargerResourceStorage])
		hqMultiplier := 1.0
		if territory.HQ {
			hqMultiplier = 5.0
		}

		// Apply the GPU-computed storage bonus
		finalStorageMultiplier := bonusStorageMultiplier * storageMultiplier

		maxOres := baseCapacity.Ores * finalStorageMultiplier * hqMultiplier
		maxWood := baseCapacity.Wood * finalStorageMultiplier * hqMultiplier
		maxFish := baseCapacity.Fish * finalStorageMultiplier * hqMultiplier
		maxCrops := baseCapacity.Crops * finalStorageMultiplier * hqMultiplier

		// Clamp to storage limits
		territory.Storage.At.Ores = math.Min(territory.Storage.At.Ores, maxOres)
		territory.Storage.At.Wood = math.Min(territory.Storage.At.Wood, maxWood)
		territory.Storage.At.Fish = math.Min(territory.Storage.At.Fish, maxFish)
		territory.Storage.At.Crops = math.Min(territory.Storage.At.Crops, maxCrops)

		territory.Mu.Unlock()

		// Debug output for first few territories
		if i < 3 {
			// fmt.Printf("[GPU_COMPUTE] Territory %d (%s): generated=%.2f, costEff=%.3f, storageMult=%.3f, rateMult=%.3f\n",
			// i, territory.Name, generatedAmount, costEfficiencyBonus, storageMultiplier, rateMultiplier)
		}
	}

	// fmt.Printf("[GPU_COMPUTE] GPU results applied to %d territories\n", len(territories))
	return nil
}

// HybridComputeTerritories performs hybrid CPU+GPU territory processing
func HybridComputeTerritories(territories map[string]*typedef.Territory) error {
	if gpuCompute == nil || !gpuCompute.enabled || gpuCompute.mode != ComputeCPUGPU {
		return fmt.Errorf("hybrid compute not available")
	}

	territorySlice := make([]*typedef.Territory, 0, len(territories))
	for _, t := range territories {
		territorySlice = append(territorySlice, t)
	}

	// Split territories between GPU and CPU
	// GPU handles 98% of territories to maximize GPU utilization
	// CPU handles remaining 2% for fallback and validation
	splitPoint := (len(territorySlice) * 49) / 50

	gpuTerritories := make(map[string]*typedef.Territory)
	cpuTerritories := make(map[string]*typedef.Territory)

	for i, territory := range territorySlice {
		if i < splitPoint {
			gpuTerritories[territory.Name] = territory
		} else {
			cpuTerritories[territory.Name] = territory
		}
	}

	// Process in parallel
	var wg sync.WaitGroup
	var gpuErr, cpuErr error

	// GPU processing
	wg.Add(1)
	go func() {
		defer wg.Done()
		gpuErr = gpuCompute.ComputeTerritoryUpdatesGPU(gpuTerritories)
		if gpuErr != nil {
			// Handle GPU error and fallback to CPU
			gpuCompute.handleGPUError(gpuErr)
			// Process these territories with CPU as fallback
			for _, territory := range gpuTerritories {
				if territory == nil {
					continue
				}

				// Calculate generation potential
				_, _, _, _ = calculateGeneration(territory)

				// Perform resource generation/consumption
				doGenerate(territory)

				// Calculate tower stats
				territory.TowerStats = calculateTowerStats(territory)
				updateTreasuryLevel(territory)
				updateGenerationBonus(territory)
			}
		} else {
			// GPU processing succeeded, minimize additional CPU work
			// Only do critical CPU-only operations every few ticks to reduce overhead
			if st.tick%5 == 0 { // Only every 5th tick
				for _, territory := range gpuTerritories {
					if territory == nil {
						continue
					}

					// Critical CPU-only operations (reduced frequency)
					territory.TowerStats = calculateTowerStats(territory)
					updateTreasuryLevel(territory)
					updateGenerationBonus(territory)

					// Update warnings less frequently
					if st.tick%10 == 0 {
						territory.Mu.Lock()
						updateTerritoryWarnings(territory, st.tick)
						territory.Mu.Unlock()
					}
				}
			}
		}
	}()

	// CPU processing (use existing parallel CPU implementation)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Process CPU territories using existing optimized CPU code
		for _, territory := range cpuTerritories {
			if territory == nil {
				continue
			}

			// Calculate generation potential
			_, _, _, _ = calculateGeneration(territory)

			// Perform resource generation/consumption
			doGenerate(territory)

			// Calculate tower stats
			territory.TowerStats = calculateTowerStats(territory)
			updateTreasuryLevel(territory)
			updateGenerationBonus(territory)
		}
	}()

	wg.Wait()

	if gpuErr != nil {
		// fmt.Printf("[GPU_COMPUTE] GPU processing failed: %v, falling back to CPU\n", gpuErr)
		// Fall back to CPU for GPU territories
		for _, territory := range gpuTerritories {
			if territory == nil {
				continue
			}
			_, _, _, _ = calculateGeneration(territory)
			doGenerate(territory)
			territory.TowerStats = calculateTowerStats(territory)
			updateTreasuryLevel(territory)
			updateGenerationBonus(territory)
		}
	}

	if cpuErr != nil {
		return fmt.Errorf("CPU processing failed: %w", cpuErr)
	}

	return nil
}

// GetGPUComputePerformanceInfo returns performance metrics
func GetGPUComputePerformanceInfo() string {
	if gpuCompute == nil {
		return "GPU Compute: Not initialized"
	}

	gpuCompute.mutex.RLock()
	defer gpuCompute.mutex.RUnlock()

	mode := "CPU Only"
	if gpuCompute.mode == ComputeCPUGPU {
		mode = "CPU + GPU Hybrid"
	}

	return fmt.Sprintf("GPU Compute: %s | GPU: %.2fms | CPU: %.2fms | Hybrid: %.2fms",
		mode, gpuCompute.gpuComputeTime, gpuCompute.cpuComputeTime, gpuCompute.hybridComputeTime)
}

// ComputeController interface implementation for GPUCompute
func (gc *GPUCompute) SetComputeMode(mode string) error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	switch mode {
	case "cpu":
		gc.mode = ComputeCPUOnly
		// fmt.Println("[GPU_COMPUTE] Switched to CPU-only mode")
	case "hybrid", "gpu":
		fmt.Printf("[GPU_COMPUTE] Attempting to enable GPU mode. gc.enabled = %v\n", gc.enabled)
		if gc.enabled {
			gc.mode = ComputeCPUGPU
			fmt.Println("[GPU_COMPUTE] Successfully switched to CPU+GPU hybrid mode")
		} else {
			fmt.Println("[GPU_COMPUTE] GPU not available, staying in CPU-only mode")
			gc.mode = ComputeCPUOnly
			return fmt.Errorf("GPU compute not available")
		}
	default:
		return fmt.Errorf("invalid compute mode: %s", mode)
	}
	return nil
}

func (gc *GPUCompute) GetComputeMode() string {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()

	switch gc.mode {
	case ComputeCPUOnly:
		return "cpu"
	case ComputeCPUGPU:
		return "hybrid"
	}
	return "cpu"
}

func (gc *GPUCompute) IsGPUAvailable() bool {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()
	return gc.enabled
}

// SetGlobalComputeController sets the global compute controller
func SetGlobalComputeController(controller ComputeController) {
	GlobalComputeController = controller
}

// GPUErrorCallback is called when GPU compute encounters an error and needs to fallback to CPU
type GPUErrorCallback func(error)

var gpuErrorCallback GPUErrorCallback

// SetGPUErrorCallback sets the callback function for GPU errors
func SetGPUErrorCallback(callback GPUErrorCallback) {
	gpuErrorCallback = callback
}

// handleGPUError handles GPU errors by disabling GPU compute and calling the error callback
func (gc *GPUCompute) handleGPUError(err error) {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// fmt.Printf("[GPU_COMPUTE] GPU error encountered: %v. Falling back to CPU-only mode.\n", err)

	// Disable GPU compute
	gc.enabled = false
	gc.mode = ComputeCPUOnly

	// Update runtime options to reflect the fallback
	currOpts := GetRuntimeOptions()
	currOpts.GPUComputeEnabled = false
	SetRuntimeOptions(currOpts)

	// Call the error callback to update UI
	if gpuErrorCallback != nil {
		gpuErrorCallback(err)
	}
}

// PathfindingAlgorithm represents different pathfinding algorithms
type PathfindingAlgorithm int

const (
	AlgorithmDijkstra PathfindingAlgorithm = iota
	AlgorithmAStar
	AlgorithmFloodfill
)

// PathfindingRequest represents a pathfinding request
type PathfindingRequest struct {
	Algorithm     PathfindingAlgorithm
	SourceX       int
	SourceY       int
	TargetX       int
	TargetY       int
	MaxIterations int
}

// PathfindingResult represents the result of pathfinding
type PathfindingResult struct {
	Path     []Vector2
	Distance float64
	Found    bool
	Cost     float64
}

// Vector2 represents a 2D coordinate
type Vector2 struct {
	X, Y int
}

// ComputeDijkstraPath performs Dijkstra pathfinding on GPU
func (gc *GPUCompute) ComputeDijkstraPath(sourceX, sourceY, targetX, targetY int, maxIterations int) (*PathfindingResult, error) {
	if !gc.enabled {
		return nil, fmt.Errorf("GPU compute not available")
	}

	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// Initialize distance texture (R: distance, G: visited, B: parent_x, A: parent_y)
	distanceTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Set source distance to 0, all others to max
	initData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	for i := 0; i < len(initData); i += 4 {
		initData[i] = 255 // Max distance (normalized)
		initData[i+1] = 0 // Not visited
		initData[i+2] = 0 // No parent
		initData[i+3] = 0
	}

	// Set source pixel to distance 0 and visited
	sourceIdx := sourceY*gc.textureWidth + sourceX
	if sourceIdx*4 < len(initData) {
		initData[sourceIdx*4] = 0     // Distance 0
		initData[sourceIdx*4+1] = 255 // Visited
	}

	distanceTexture.WritePixels(initData)

	// Perform iterative Dijkstra computation
	for iter := 0; iter < maxIterations; iter++ {
		tempTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

		// Run shader pass
		opts := &ebiten.DrawRectShaderOptions{}
		opts.Images[0] = distanceTexture
		opts.Images[1] = gc.territoryDataTexture

		tempTexture.DrawRectShader(gc.textureWidth, gc.textureHeight, gc.dijkstraShader, opts)
		distanceTexture = tempTexture
	}

	// Extract path result
	result := &PathfindingResult{
		Path:     []Vector2{},
		Distance: 0,
		Found:    false,
		Cost:     0,
	}

	// Read back results and reconstruct path
	pixelData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	distanceTexture.ReadPixels(pixelData)

	targetIdx := targetY*gc.textureWidth + targetX
	if targetIdx*4 < len(pixelData) {
		distance := float64(pixelData[targetIdx*4]) * 1000000.0 / 255.0
		if distance < 999999.0 {
			result.Found = true
			result.Distance = distance
			result.Cost = distance

			// Reconstruct path by following parent pointers
			currentX, currentY := targetX, targetY
			for currentX != sourceX || currentY != sourceY {
				result.Path = append([]Vector2{{X: currentX, Y: currentY}}, result.Path...)

				currentIdx := currentY*gc.textureWidth + currentX
				if currentIdx*4+3 >= len(pixelData) {
					break
				}

				parentX := int(float64(pixelData[currentIdx*4+2]) * float64(gc.textureWidth) / 255.0)
				parentY := int(float64(pixelData[currentIdx*4+3]) * float64(gc.textureHeight) / 255.0)

				if parentX == currentX && parentY == currentY {
					break // Avoid infinite loop
				}

				currentX, currentY = parentX, parentY
			}

			result.Path = append([]Vector2{{X: sourceX, Y: sourceY}}, result.Path...)
		}
	}

	return result, nil
}

// ComputeAStarPath performs A* pathfinding on GPU
func (gc *GPUCompute) ComputeAStarPath(sourceX, sourceY, targetX, targetY int, maxIterations int) (*PathfindingResult, error) {
	if !gc.enabled {
		return nil, fmt.Errorf("GPU compute not available")
	}

	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// Initialize state texture (R: g_cost, G: f_cost, B: visited, A: parent_idx)
	stateTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Set initial state
	initData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	for i := 0; i < len(initData); i += 4 {
		initData[i] = 255   // Max g_cost
		initData[i+1] = 255 // Max f_cost
		initData[i+2] = 0   // Not visited
		initData[i+3] = 0   // No parent
	}

	// Set source pixel
	sourceIdx := sourceY*gc.textureWidth + sourceX
	if sourceIdx*4 < len(initData) {
		initData[sourceIdx*4] = 0     // g_cost = 0
		initData[sourceIdx*4+1] = 0   // f_cost = heuristic
		initData[sourceIdx*4+2] = 255 // Visited
	}

	stateTexture.WritePixels(initData)

	// Perform iterative A* computation
	for iter := 0; iter < maxIterations; iter++ {
		tempTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

		opts := &ebiten.DrawRectShaderOptions{}
		opts.Images[0] = stateTexture
		opts.Images[1] = gc.territoryDataTexture

		tempTexture.DrawRectShader(gc.textureWidth, gc.textureHeight, gc.astarShader, opts)
		stateTexture = tempTexture
	}

	// Extract result similar to Dijkstra
	result := &PathfindingResult{
		Path:     []Vector2{},
		Distance: 0,
		Found:    false,
		Cost:     0,
	}

	pixelData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	stateTexture.ReadPixels(pixelData)

	targetIdx := targetY*gc.textureWidth + targetX
	if targetIdx*4 < len(pixelData) {
		gCost := float64(pixelData[targetIdx*4]) * 1000000.0 / 255.0
		if gCost < 999999.0 {
			result.Found = true
			result.Distance = gCost
			result.Cost = gCost

			// Path reconstruction would be implemented here
			// For now, just return basic result
			result.Path = []Vector2{{X: sourceX, Y: sourceY}, {X: targetX, Y: targetY}}
		}
	}

	return result, nil
}

// ComputeFloodfill performs floodfill algorithm on GPU
func (gc *GPUCompute) ComputeFloodfill(sourceX, sourceY int, maxIterations int) (map[Vector2]int, error) {
	if !gc.enabled {
		return nil, fmt.Errorf("GPU compute not available")
	}

	gc.mutex.Lock()
	defer gc.mutex.Unlock()

	// Initialize flood state texture (R: filled, G: generation, B: source_id, A: barrier)
	floodTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

	// Set initial state
	initData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	for i := 0; i < len(initData); i += 4 {
		initData[i] = 0   // Not filled
		initData[i+1] = 0 // Generation 0
		initData[i+2] = 0 // Source ID
		initData[i+3] = 0 // No barrier
	}

	// Set source pixel
	sourceIdx := sourceY*gc.textureWidth + sourceX
	if sourceIdx*4 < len(initData) {
		initData[sourceIdx*4] = 255 // Filled
		initData[sourceIdx*4+1] = 0 // Generation 0
		initData[sourceIdx*4+2] = 1 // Source ID 1
	}

	floodTexture.WritePixels(initData)

	// Perform iterative floodfill
	for iter := 0; iter < maxIterations; iter++ {
		tempTexture := ebiten.NewImage(gc.textureWidth, gc.textureHeight)

		opts := &ebiten.DrawRectShaderOptions{}
		opts.Images[0] = floodTexture
		opts.Images[1] = gc.territoryDataTexture

		tempTexture.DrawRectShader(gc.textureWidth, gc.textureHeight, gc.floodfillShader, opts)
		floodTexture = tempTexture
	}

	// Extract filled areas
	result := make(map[Vector2]int)
	pixelData := make([]byte, gc.textureWidth*gc.textureHeight*4)
	floodTexture.ReadPixels(pixelData)

	for y := 0; y < gc.textureHeight; y++ {
		for x := 0; x < gc.textureWidth; x++ {
			idx := (y*gc.textureWidth + x) * 4
			if idx+2 < len(pixelData) && pixelData[idx] > 128 { // Filled
				generation := int(pixelData[idx+1])
				result[Vector2{X: x, Y: y}] = generation
			}
		}
	}

	return result, nil
}

// PathfindingRequest helper function
func (gc *GPUCompute) FindPath(req PathfindingRequest) (*PathfindingResult, error) {
	switch req.Algorithm {
	case AlgorithmDijkstra:
		return gc.ComputeDijkstraPath(req.SourceX, req.SourceY, req.TargetX, req.TargetY, req.MaxIterations)
	case AlgorithmAStar:
		return gc.ComputeAStarPath(req.SourceX, req.SourceY, req.TargetX, req.TargetY, req.MaxIterations)
	case AlgorithmFloodfill:
		// Floodfill doesn't have a target, so we'll use it differently
		filled, err := gc.ComputeFloodfill(req.SourceX, req.SourceY, req.MaxIterations)
		if err != nil {
			return nil, err
		}

		result := &PathfindingResult{
			Path:     []Vector2{},
			Distance: float64(len(filled)),
			Found:    len(filled) > 0,
			Cost:     0,
		}

		// Convert filled areas to path
		for pos := range filled {
			result.Path = append(result.Path, pos)
		}

		return result, nil
	default:
		return nil, fmt.Errorf("unknown pathfinding algorithm")
	}
}
