package eruntime

import (
	"runtime"
	"time"
)

var StateTick = make(chan uint64)

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

	// Watchdog to detect stalls in tick processing
	// go func() {
	// 	lastTick := uint64(0)
	// 	lastProgress := time.Now()
	// 	buf := make([]byte, 1<<20)

	// 	for range time.Tick(2 * time.Second) {
	// 		current := Tick()
	// 		if current != lastTick {
	// 			lastTick = current
	// 			lastProgress = time.Now()
	// 			continue
	// 		}

	// 		if time.Since(lastProgress) > 2*time.Second {
	// 			if err := pprof.Lookup("goroutine").WriteTo(os.Stdout, 2); err != nil {
	// 				if m := runtime.Stack(buf, true); m > 0 {
	// 					// _, _ = os.Stdout.Write(buf[:m])
	// 				}
	// 			}
	// 			lastProgress = time.Now()
	// 		}
	// 	}
	// }()
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
	// Use a lightweight approach that doesn't block the timer
	// For very high tick rates, we queue the tick request instead of blocking
	select {
	case s.tickQueue <- struct{}{}:
		SendStateTick()
		// Successfully queued the tick
	default:
		// Queue is full, skip this tick to prevent blocking
		// This happens when processing can't keep up with tick rate
	}
}

// Internal tick processing function
func (s *state) processQueuedTicks() {
	for range s.tickQueue {
		tickStart := time.Now()
		s.mu.Lock()

		s.tick++

		// Process resource deliveries BEFORE consumption on minute boundaries
		// This ensures territories receive HQ shipments before consuming resources
		if s.tick%60 == 0 {
			s.update2()
		}
		s.update()

		// Trigger auto-save every minute (60 ticks)
		if s.tick%60 == 0 {
			go TriggerAutoSave()
		}

		// Force garbage collection every 5 minutes to help with memory management
		if s.tick%300 == 0 {
			runtime.GC()
		}

		// Update performance metrics
		tickEnd := time.Now()
		s.tickProcessTime = tickEnd.Sub(tickStart)

		// Calculate actual TPS (every 100 ticks for reasonable accuracy)
		if s.tick%100 == 0 {
			if !s.lastTickTime.IsZero() {
				timeDiff := tickEnd.Sub(s.lastTickTime)
				if timeDiff > 0 {
					s.actualTPS = 100.0 / timeDiff.Seconds()
				}
			}
			s.lastTickTime = tickEnd
		}

		s.mu.Unlock()

		// For very high tick rates, yield occasionally to prevent CPU monopolization
		if s.tick%1000 == 0 {
			runtime.Gosched()
		}
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
	}

	if ticksPerSecond == 1 {
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

func SendStateTick() {
	// Try sending without blocking
	select {
	case StateTick <- uint64(st.tick):
		// Successfully sent tick notification
	default:
		// Channel is full, skip sending to avoid blocking
	}
}

func GetStateTick() <-chan uint64 {
	return StateTick
}
