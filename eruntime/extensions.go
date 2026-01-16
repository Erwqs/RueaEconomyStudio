package eruntime

// SetExternalCalculatorActive toggles external calculation mode.
// When enabled, the runtime will run updates sequentially to avoid races with plugin-driven work.
func SetExternalCalculatorActive(active bool) {
	st.mu.Lock()
	st.externalCalculatorActive = active
	st.mu.Unlock()
}
