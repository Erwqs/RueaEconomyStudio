package eruntime

// Enhanced resource computation shader - performs complex calculations to justify GPU usage
const resourceComputeShaderSrc = `
//kage:unit pixels

package main

var TerritoryCount float
var TextureWidth float
var Time float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    // Get territory index from source position
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    // Skip if beyond territory count
    if territoryIdx >= TerritoryCount {
        return vec4(0, 0, 0, 0)
    }
    
    // Sample territory data from image 0
    territoryTexCoord := vec2(mod(territoryIdx, TextureWidth) / TextureWidth, floor(territoryIdx / TextureWidth) / TextureWidth)
    
    territoryData := imageSrc0At(territoryTexCoord)
    costData := imageSrc1At(territoryTexCoord)
    
    // Unpack territory properties
    baseGeneration := territoryData.r * 255.0 / 100.0  // Denormalize base generation
    storageLevel := territoryData.g * 255.0 / 10000.0  // Denormalize storage
    upgradeLevels := territoryData.b * 255.0           // Upgrade levels
    bonusLevels := territoryData.a * 255.0             // Bonus levels
    
    // Unpack cost data multipliers
    efficiencyMultiplier := costData.g / 255.0
    storageMultiplier := costData.b / 255.0
    rateMultiplier := costData.a / 255.0
    
    // Complex mathematical calculations using Kage-supported functions
    // Simulate market dynamics with trigonometric functions
    demandFactor := sin(Time * 0.1 + territoryIdx * 0.01) * 0.1 + 1.0
    
    // Growth calculations using available math functions
    decayFactor := 1.0 / (1.0 + storageLevel * 0.01) // Approximate exp decay
    growthFactor := decayFactor * (1.0 + bonusLevels / 255.0)
    
    // Multi-resource interdependency using sqrt (available in Kage)
    resourceSynergy := sqrt(upgradeLevels / 255.0) * 1.5
    
    // Seasonal effects with cosine
    seasonalBonus := cos(Time * 0.05 + territoryIdx * 0.02) * 0.2 + 1.0
    
    // Time-based fluctuation
    timeFluctuation := sin(Time * 0.03 + territoryIdx * 0.05) * 0.15 + 1.0
    
    // Calculate final generation with complex interactions
    finalGeneration := baseGeneration * demandFactor * growthFactor * resourceSynergy * seasonalBonus * timeFluctuation
    
    // Multi-step cost efficiency calculation (unrolled loop)
    costEfficiency1 := sqrt(upgradeLevels / 255.0) * 0.1
    costEfficiency2 := (upgradeLevels / 255.0) * 0.1
    costEfficiency3 := sqrt(upgradeLevels / 255.0) * (upgradeLevels / 255.0) * 0.1
    costEfficiency4 := (upgradeLevels / 255.0) * (upgradeLevels / 255.0) * 0.1
    costEfficiency5 := sqrt(upgradeLevels / 255.0) * (upgradeLevels / 255.0) * (upgradeLevels / 255.0) * 0.1
    
    costEfficiency := costEfficiency1 + costEfficiency2 + costEfficiency3 + costEfficiency4 + costEfficiency5
    
    // Storage bonus with mathematical approximation of logarithm
    // Approximate log using Taylor series: log(1+x) ≈ x - x²/2 + x³/3 for small x
    logApprox := storageLevel * 10.0 - (storageLevel * 10.0) * (storageLevel * 10.0) * 0.5
    storageBonus := 1.0 + logApprox * 0.1
    storageBonus *= (1.0 + baseGeneration * 0.1)
    
    // Rate bonus with time-based fluctuation
    rateBonus := efficiencyMultiplier * storageMultiplier * rateMultiplier
    rateBonus *= (1.0 + sin(Time * 0.01 + territoryIdx * 0.007) * 0.1)
    
    // Pack comprehensive results into RGBA
    return vec4(finalGeneration / 100.0, costEfficiency, storageBonus, rateBonus)
}
`

// Cost computation shader - calculates resource costs and affordability
const costComputeShaderSrc = `
//kage:unit pixels

package main

var TerritoryCount float
var TextureWidth float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    if territoryIdx >= TerritoryCount {
        return vec4(0, 0, 0, 0)
    }
    
    territoryTexCoord := vec2(mod(territoryIdx, TextureWidth) / TextureWidth, floor(territoryIdx / TextureWidth) / TextureWidth)
    
    territoryData := imageSrc0At(territoryTexCoord)
    
    // Calculate affordability for each resource type
    storageLevel := territoryData.g * 255.0 / 10000.0
    upgradeLevels := territoryData.b * 255.0
    bonusLevels := territoryData.a * 255.0
    
    // Cost calculations based on upgrade and bonus levels
    oreCost := upgradeLevels * 0.1 + bonusLevels * 0.05
    woodCost := upgradeLevels * 0.08 + bonusLevels * 0.06
    fishCost := upgradeLevels * 0.09 + bonusLevels * 0.04
    cropCost := upgradeLevels * 0.07 + bonusLevels * 0.07
    
    // Affordability check (simplified) - using step function instead of ternary
    oreAffordable := step(oreCost, storageLevel)
    woodAffordable := step(woodCost, storageLevel)
    fishAffordable := step(fishCost, storageLevel)
    cropAffordable := step(cropCost, storageLevel)
    
    return vec4(oreAffordable, woodAffordable, fishAffordable, cropAffordable)
}
`

// Bonus computation shader - calculates bonus levels and effects
const bonusComputeShaderSrc = `
//kage:unit pixels

package main

var TerritoryCount float
var TextureWidth float

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    if territoryIdx >= TerritoryCount {
        return vec4(0, 0, 0, 0)
    }
    
    territoryTexCoord := vec2(mod(territoryIdx, TextureWidth) / TextureWidth, floor(territoryIdx / TextureWidth) / TextureWidth)
    
    territoryData := imageSrc0At(territoryTexCoord)
    previousResults := imageSrc1At(territoryTexCoord)
    
    // Get affordability from cost computation
    oreAffordable := previousResults.r
    woodAffordable := previousResults.g
    fishAffordable := previousResults.b
    cropAffordable := previousResults.a
    
    // Calculate bonus effects based on affordability
    bonusLevels := territoryData.a * 255.0
    
    // Calculate final bonuses
    resourceBonus := (oreAffordable + woodAffordable + fishAffordable + cropAffordable) * 0.25
    efficiencyBonus := bonusLevels / 255.0 * resourceBonus
    storageBonus := resourceBonus * 1.5
    rateBonus := resourceBonus * 0.8
    
    return vec4(resourceBonus, efficiencyBonus, storageBonus, rateBonus)
}
`

// Dijkstra pathfinding shader - single iteration pass
const dijkstraComputeShaderSrc = `//go:build ignore

//kage:unit pixels

package main

const (
    TextureWidth = 512.0
    TerritoryCount = 1000.0
    MaxDistance = 999999.0
)

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    if territoryIdx >= TerritoryCount {
        return vec4(MaxDistance / 1000000.0, 0, 0, 0)
    }
    
    territoryTexCoord := vec2(mod(territoryIdx, TextureWidth) / TextureWidth, floor(territoryIdx / TextureWidth) / TextureWidth)
    
    // imageSrc0: current distances (R: distance, G: visited, B: parent_x, A: parent_y)
    // imageSrc1: territory data (R: terrain_cost, G: connection_mask, B: x_coord, A: y_coord)
    currentState := imageSrc0At(territoryTexCoord)
    territoryData := imageSrc1At(territoryTexCoord)
    
    currentDistance := currentState.r * 1000000.0
    visited := currentState.g
    terrainCost := territoryData.r * 255.0
    
    // Skip if already visited
    if visited > 0.5 {
        return currentState
    }
    
    minDistance := currentDistance
    newParentX := currentState.b
    newParentY := currentState.a
    
    // Check 4-directional neighbors
    for dy := -1.0; dy <= 1.0; dy += 1.0 {
        for dx := -1.0; dx <= 1.0; dx += 1.0 {
            if abs(dx) + abs(dy) != 1.0 {
                continue
            }
            
            neighborX := mod(territoryIdx, TextureWidth) + dx
            neighborY := floor(territoryIdx / TextureWidth) + dy
            
            if neighborX < 0.0 || neighborX >= TextureWidth || neighborY < 0.0 || neighborY >= TextureWidth {
                continue
            }
            
            neighborTexCoord := vec2(neighborX / TextureWidth, neighborY / TextureWidth)
            neighborState := imageSrc0At(neighborTexCoord)
            neighborDistance := neighborState.r * 1000000.0
            neighborVisited := neighborState.g
            
            if neighborVisited > 0.5 {
                newDistance := neighborDistance + terrainCost + 1.0
                if newDistance < minDistance {
                    minDistance = newDistance
                    newParentX = neighborX / TextureWidth
                    newParentY = neighborY / TextureWidth
                }
            }
        }
    }
    
    return vec4(minDistance / 1000000.0, 0.0, newParentX, newParentY)
}
`

// A* pathfinding shader - single iteration pass
const astarComputeShaderSrc = `//go:build ignore

//kage:unit pixels

package main

const (
    TextureWidth = 512.0
    TerritoryCount = 1000.0
    MaxDistance = 999999.0
    TargetX = 31.0
    TargetY = 31.0
)

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    if territoryIdx >= TerritoryCount {
        return vec4(MaxDistance / 1000000.0, MaxDistance / 1000000.0, 0, 0)
    }
    
    currentX := mod(territoryIdx, TextureWidth)
    currentY := floor(territoryIdx / TextureWidth)
    territoryTexCoord := vec2(currentX / TextureWidth, currentY / TextureWidth)
    
    // imageSrc0: current state (R: g_cost, G: f_cost, B: visited, A: parent_idx)
    // imageSrc1: territory data (R: terrain_cost, G: h_cost, B: connection_mask, A: unused)
    currentState := imageSrc0At(territoryTexCoord)
    territoryData := imageSrc1At(territoryTexCoord)
    
    gCost := currentState.r * 1000000.0
    fCost := currentState.g * 1000000.0
    visited := currentState.b
    terrainCost := territoryData.r * 255.0
    
    // Calculate heuristic (Manhattan distance to target)
    hCost := abs(currentX - TargetX) + abs(currentY - TargetY)
    
    // Skip if already visited
    if visited > 0.5 {
        return currentState
    }
    
    minGCost := gCost
    minFCost := fCost
    newParentIdx := currentState.a
    
    // Check 4-directional neighbors
    for dy := -1.0; dy <= 1.0; dy += 1.0 {
        for dx := -1.0; dx <= 1.0; dx += 1.0 {
            if abs(dx) + abs(dy) != 1.0 {
                continue
            }
            
            neighborX := currentX + dx
            neighborY := currentY + dy
            
            if neighborX < 0.0 || neighborX >= TextureWidth || neighborY < 0.0 || neighborY >= TextureWidth {
                continue
            }
            
            neighborTexCoord := vec2(neighborX / TextureWidth, neighborY / TextureWidth)
            neighborState := imageSrc0At(neighborTexCoord)
            neighborGCost := neighborState.r * 1000000.0
            neighborVisited := neighborState.b
            
            if neighborVisited > 0.5 {
                tentativeGCost := neighborGCost + terrainCost + 1.0
                tentativeFCost := tentativeGCost + hCost
                
                if tentativeGCost < minGCost {
                    minGCost = tentativeGCost
                    minFCost = tentativeFCost
                    neighborIdx := neighborY * TextureWidth + neighborX
                    newParentIdx = neighborIdx / TerritoryCount
                }
            }
        }
    }
    
    return vec4(minGCost / 1000000.0, minFCost / 1000000.0, 0.0, newParentIdx)
}
`

// Floodfill shader - single iteration pass
const floodfillComputeShaderSrc = `//go:build ignore

//kage:unit pixels

package main

const (
    TextureWidth = 512.0
    TerritoryCount = 1000.0
    SourceTerritoryIdx = 0.0
)

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
    territoryIdx := floor(srcPos.y * TextureWidth) * TextureWidth + floor(srcPos.x * TextureWidth)
    
    if territoryIdx >= TerritoryCount {
        return vec4(0, 0, 0, 0)
    }
    
    currentX := mod(territoryIdx, TextureWidth)
    currentY := floor(territoryIdx / TextureWidth)
    territoryTexCoord := vec2(currentX / TextureWidth, currentY / TextureWidth)
    
    // imageSrc0: flood state (R: filled, G: generation, B: source_id, A: barrier)
    // imageSrc1: territory data (R: terrain_type, G: passable, B: cost, A: owner)
    currentState := imageSrc0At(territoryTexCoord)
    territoryData := imageSrc1At(territoryTexCoord)
    
    filled := currentState.r
    generation := currentState.g * 255.0
    sourceId := currentState.b * 255.0
    barrier := currentState.a
    passable := territoryData.g
    
    // Skip if already filled or blocked by barrier
    if filled > 0.5 || barrier > 0.5 || passable < 0.5 {
        return currentState
    }
    
    newFilled := 0.0
    newGeneration := generation
    newSourceId := sourceId
    
    // Check if any neighbor is filled with lower generation
    for dy := -1.0; dy <= 1.0; dy += 1.0 {
        for dx := -1.0; dx <= 1.0; dx += 1.0 {
            if abs(dx) + abs(dy) != 1.0 {
                continue
            }
            
            neighborX := currentX + dx
            neighborY := currentY + dy
            
            if neighborX < 0.0 || neighborX >= TextureWidth || neighborY < 0.0 || neighborY >= TextureWidth {
                continue
            }
            
            neighborTexCoord := vec2(neighborX / TextureWidth, neighborY / TextureWidth)
            neighborState := imageSrc0At(neighborTexCoord)
            neighborFilled := neighborState.r
            neighborGeneration := neighborState.g * 255.0
            neighborSourceId := neighborState.b * 255.0
            
            if neighborFilled > 0.5 && neighborGeneration + 1.0 < 255.0 {
                newFilled = 1.0
                newGeneration = neighborGeneration + 1.0
                newSourceId = neighborSourceId
                break
            }
        }
        if newFilled > 0.5 {
            break
        }
    }
    
    return vec4(newFilled, newGeneration / 255.0, newSourceId / 255.0, barrier)
}
`
