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
		return
	}

	sethqUnsafe(territory)
}

// sethqUnsafe is the internal version that doesn't acquire locks
// Caller must ensure proper locking (st.mu write lock)
func sethqUnsafe(territory *typedef.Territory) {
	// Find old HQ for this guild and unset it
	guildTag := territory.Guild.Tag
	if oldHQ := getHQFromMap(guildTag); oldHQ != nil && oldHQ != territory {
		// Lock the old HQ territory to unset it safely
		oldHQ.Mu.Lock()
		oldHQ.HQ = false
		oldHQ.Mu.Unlock()

		// Remove old HQ from map
		setHQInMap(oldHQ, false)
	}

	// Set this territory as the new HQ
	territory.Mu.Lock()
	territory.HQ = true
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
	}()
}
