package eruntime

import "etools/typedef"

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

	sethqUnsafe(territory)
}

// sethqUnsafe is the internal version that doesn't acquire locks
// Caller must ensure proper locking (st.mu write lock)
func sethqUnsafe(territory *typedef.Territory) {
	// Find old HQ for this guild and unset it
	for _, t := range st.territories {
		if t != nil && t.Guild.Name == territory.Guild.Name && t.HQ && t != territory {
			// Lock the old HQ territory to unset it safely
			t.Mu.Lock()
			t.HQ = false
			t.Mu.Unlock()
		}
	}

	// Set this territory as the new HQ
	territory.Mu.Lock()
	territory.HQ = true
	territory.Mu.Unlock()

	// Update trading routes for all territories of this guild to reflect the new HQ
	// Note: updateRoute is now called within the st.mu.Lock, so it's protected
	st.updateRoute()
}
