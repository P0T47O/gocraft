package main

import (
	"math"
	"testing"
	"time"
)

func TestClientPacketBudget(t *testing.T) {
	for _, tc := range []struct {
		dt    float32
		queue int
		want  time.Duration
	}{
		{1.0 / 60, 0, 2 * time.Millisecond}, {1.0 / 20, 0, 6 * time.Millisecond},
		{1.0 / 60, 128, time.Second / 300}, {1.0 / 20, 128, 6 * time.Millisecond},
		{10, 128, 6 * time.Millisecond}, {0, 0, 2 * time.Millisecond},
		{float32(math.NaN()), 0, 2 * time.Millisecond}, {float32(math.Inf(1)), 0, 2 * time.Millisecond},
	} {
		got := clientPacketBudget(tc.dt, tc.queue)
		if diff := got - tc.want; diff < -time.Microsecond || diff > time.Microsecond {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
}
