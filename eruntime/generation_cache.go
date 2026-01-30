package eruntime

import (
	"RueaES/typedef"
	"sync"
)

type generationCacheEntry struct {
	tick      uint64
	staticGen typedef.BasicResources
	costNow   typedef.BasicResourcesSecond
}

type generationCacheState struct {
	mu   sync.RWMutex
	tick uint64
	data map[*typedef.Territory]generationCacheEntry
}

var generationCache generationCacheState

// prepareGenerationCache resets the cache for the current tick.
func prepareGenerationCache(tick uint64) {
	generationCache.mu.Lock()
	generationCache.tick = tick
	if generationCache.data == nil {
		generationCache.data = make(map[*typedef.Territory]generationCacheEntry, len(st.territories))
	} else {
		for k := range generationCache.data {
			delete(generationCache.data, k)
		}
	}
	generationCache.mu.Unlock()
}

// getCachedGeneration returns cached values if available for the given tick.
func getCachedGeneration(territory *typedef.Territory, tick uint64) (typedef.BasicResources, typedef.BasicResourcesSecond, bool) {
	generationCache.mu.RLock()
	entry, ok := generationCache.data[territory]
	cacheTick := generationCache.tick
	generationCache.mu.RUnlock()
	if !ok || cacheTick != tick || entry.tick != tick {
		return typedef.BasicResources{}, typedef.BasicResourcesSecond{}, false
	}
	return entry.staticGen, entry.costNow, true
}

// storeCachedGeneration records generation results for reuse within the same tick.
func storeCachedGeneration(territory *typedef.Territory, tick uint64, staticGen typedef.BasicResources, costNow typedef.BasicResourcesSecond) {
	generationCache.mu.Lock()
	if generationCache.data == nil {
		generationCache.data = make(map[*typedef.Territory]generationCacheEntry, len(st.territories))
	}
	if generationCache.tick != tick {
		generationCache.tick = tick
		for k := range generationCache.data {
			delete(generationCache.data, k)
		}
	}
	generationCache.data[territory] = generationCacheEntry{
		tick:      tick,
		staticGen: staticGen,
		costNow:   costNow,
	}
	generationCache.mu.Unlock()
}
