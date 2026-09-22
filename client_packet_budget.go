package main

import (
	"math"
	"time"
)

// Share elapsed frame time with packet application instead of providing a
// fixed 2ms each frame. Backlog earns a larger share, but never more than 6ms;
// no unused credit is carried across pauses. One packet remains indivisible.
func clientPacketBudget(frameSeconds float32, queued int) time.Duration {
	dt := float64(frameSeconds)
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		dt = 1.0 / 60
	}
	share := 0.12
	if queued >= 64 {
		share = 0.20
	}
	seconds := min(0.006, max(0.00025, min(dt, 0.1)*share))
	return time.Duration(seconds * float64(time.Second))
}
