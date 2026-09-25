package main

const (
	dayLengthTicks    = 24000 // 20 minutes at the server's 20 TPS.
	initialWorldTime  = 1000  // Start new worlds in the morning.
	timeSyncInterval  = 20    // One authoritative snapshot per second.
	serverTicksPerSec = 20
)

type daylightState struct {
	Brightness float32
	Sky        [3]float32
}

// worldDaylight is continuous at midnight and uses the same clock for the
// world shader and sky clear color. Time 0 is sunrise, 6000 is noon.
func worldDaylight(ticks float64) daylightState {
	t := ticks - float64(dayLengthTicks)*float64(int64(ticks)/dayLengthTicks)
	if t < 0 {
		t += dayLengthTicks
	}
	keys := [...]struct {
		tick  float64
		state daylightState
	}{
		{0, daylightState{0.72, [3]float32{0.52, 0.66, 0.90}}},
		{2000, daylightState{1, [3]float32{180.0 / 255, 210.0 / 255, 1}}},
		{10000, daylightState{1, [3]float32{180.0 / 255, 210.0 / 255, 1}}},
		{12000, daylightState{0.70, [3]float32{0.87, 0.60, 0.52}}},
		{14000, daylightState{0.28, [3]float32{0.07, 0.11, 0.21}}},
		{22000, daylightState{0.28, [3]float32{0.07, 0.11, 0.21}}},
		{23000, daylightState{0.42, [3]float32{0.29, 0.33, 0.52}}},
		{dayLengthTicks, daylightState{0.72, [3]float32{0.52, 0.66, 0.90}}},
	}
	for i := 1; i < len(keys); i++ {
		if t > keys[i].tick {
			continue
		}
		f := float32((t - keys[i-1].tick) / (keys[i].tick - keys[i-1].tick))
		f = f * f * (3 - 2*f)
		a, b := keys[i-1].state, keys[i].state
		state := daylightState{Brightness: a.Brightness + (b.Brightness-a.Brightness)*f}
		for channel := range state.Sky {
			state.Sky[channel] = a.Sky[channel] + (b.Sky[channel]-a.Sky[channel])*f
		}
		return state
	}
	return keys[0].state
}
