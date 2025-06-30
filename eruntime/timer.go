package eruntime

import (
	"runtime"
	"time"
)

// Start creates a timer ticker that calls update() every tick
func (s *state) start() {
	// Don't start if already running
	if s.timerChan != nil {
		return
	}

	s.timerChan = time.NewTicker(1 * time.Second)
	go func() {
		for range s.timerChan.C {
			if s.halted {
				continue
			}

			s.nexttick() // Advance the simulation state by 1 tick
		}
	}()
}

// Stop stops the timer ticker and destroy the timer
// This should only be called when the application is shutting down to prevent state corruption
func (s *state) stop() {
	s.halt()
	if s.timerChan != nil {
		s.timerChan.Stop()
		s.timerChan = nil
	}
}

// halt stops the ticker from calling update()
func (s *state) halt() {
	s.halted = true
}

func (s *state) resume() {
	s.halted = false
	if s.timerChan == nil {
		s.start() // Restart the ticker if it was stopped
	}
}

func (s *state) isHalted() bool {
	return s.halted
}

// nexttick is called by the timer ticker to advance the simulation state by 1 tick
// It should be called every second or called by the user manually
func (s *state) nexttick() {
	// Process resource deliveries BEFORE consumption on minute boundaries
	// This ensures territories receive HQ shipments before consuming resources
	if s.tick%60 == 0 {
		// Enqueue resource transversal
		s.update2()
	}
	s.update()
	s.tick++

	// Force garbage collection every 5 minutes to help with memory management
	if s.tick%300 == 0 {
		runtime.GC()
	}
}

// setTickRate changes the tick rate by stopping and restarting the timer
func (s *state) setTickRate(ticksPerSecond int) {
	// Stop current timer if running
	if s.timerChan != nil {
		s.timerChan.Stop()
		s.timerChan = nil
	}

	// Calculate interval based on ticks per second
	var interval time.Duration
	if ticksPerSecond <= 0 {
		// If 0 or negative, stop the timer completely
		return
	} else if ticksPerSecond == 1 {
		interval = 1 * time.Second
	} else {
		interval = time.Second / time.Duration(ticksPerSecond)
	}

	// Start new timer with new interval
	s.timerChan = time.NewTicker(interval)
	go func() {
		for range s.timerChan.C {
			if s.halted {
				continue
			}
			s.nexttick() // Advance the simulation state by 1 tick
		}
	}()
}
