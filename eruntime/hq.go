package eruntime

import (
	"etools/typedef"
	"fmt"
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
		fmt.Printf("[ERUNTIME] SetHQ blocked during state loading for territory: %s\n", territory.Name)
		return
	}

	sethqUnsafe(territory)
}

// sethqUnsafe is the internal version that doesn't acquire locks
// Caller must ensure proper locking (st.mu write lock)
func sethqUnsafe(territory *typedef.Territory) {
	fmt.Printf("[HQ_DEBUG] Setting HQ for territory %s (guild: %s)\n", territory.Name, territory.Guild.Name)

	// Find old HQ for this guild and unset it
	for _, t := range st.territories {
		if t != nil && t.Guild.Name == territory.Guild.Name && t.HQ && t != territory {
			// Lock the old HQ territory to unset it safely
			t.Mu.Lock()
			fmt.Printf("[HQ_DEBUG] Unsetting old HQ: %s (guild: %s)\n", t.Name, t.Guild.Name)
			t.HQ = false
			t.Mu.Unlock()
		}
	}

	// Set this territory as the new HQ
	territory.Mu.Lock()
	territory.HQ = true
	fmt.Printf("[HQ_DEBUG] HQ set successfully for territory %s (guild: %s)\n", territory.Name, territory.Guild.Name)
	territory.Mu.Unlock()

	// Update trading routes for all territories of this guild to reflect the new HQ
	// Note: updateRoute is now called within the st.mu.Lock, so it's protected
	st.updateRoute()
}
