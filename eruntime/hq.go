package eruntime

import (
	"RueaES/typedef"
	"time"
)

func gethq(name string) *typedef.Territory {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Find the first territory that is marked as HQ for the given guild
	for _, t := range st.territories {
		if t != nil && t.Guild.Name == name && t.HQ {
			return t
		}
	}
	return nil // No HQ found for this guild
}

func sethq(territory *typedef.Territory) {
	// Protect the entire HQ setting operation with a write lock
	st.mu.Lock()
	defer st.mu.Unlock()

	// Don't allow HQ modifications during state loading
	if st.stateLoading {
		// fmt.Printf("[ERUNTIME] SetHQ blocked during state loading for territory: %s\n", territory.Name)
		return
	}

	sethqUnsafe(territory)
}

// sethqUnsafe is the internal version that doesn't acquire locks
// Caller must ensure proper locking (st.mu write lock)
func sethqUnsafe(territory *typedef.Territory) {
	// fmt.Printf("[HQ_DEBUG] Setting HQ for territory %s (guild: %s)\n", territory.Name, territory.Guild.Name)

	// Find old HQ for this guild and unset it
	guildTag := territory.Guild.Tag
	if oldHQ := getHQFromMap(guildTag); oldHQ != nil && oldHQ != territory {
		// Lock the old HQ territory to unset it safely
		oldHQ.Mu.Lock()
		// fmt.Printf("[HQ_DEBUG] Unsetting old HQ: %s (guild: %s)\n", oldHQ.Name, oldHQ.Guild.Name)
		oldHQ.HQ = false
		oldHQ.Mu.Unlock()

		// Remove old HQ from map
		setHQInMap(oldHQ, false)
	}

	// Set this territory as the new HQ
	territory.Mu.Lock()
	territory.HQ = true
	// fmt.Printf("[HQ_DEBUG] HQ set successfully for territory %s (guild: %s)\n", territory.Name, territory.Guild.Name)
	territory.Mu.Unlock()

	// Add new HQ to map
	setHQInMap(territory, true)

	// Update trading routes for all territories of this guild to reflect the new HQ
	// Note: updateRoute is now called within the st.mu.Lock, so it's protected
	st.updateRoute()

	// Notify UI components to update after HQ change
	// This ensures territory colors and HQ icons are refreshed
	go func() {
		// Add a small delay to ensure state is fully settled
		time.Sleep(50 * time.Millisecond)

		// Notify territory manager to update colors and visual state
		NotifyTerritoryColorsUpdate()

		// Notify only the specific guild that had its HQ changed for efficiency
		NotifyGuildSpecificUpdate(territory.Guild.Name)

		// fmt.Printf("[HQ_DEBUG] HQ change notifications sent to UI components for guild: %s\n", territory.Guild.Name)
	}()
}
