package main

const (
	defaultGameFrameTime = float32(1.0 / 60.0)
	maxGameFrameTime     = float32(0.05)
)

var gameFrameDelta = defaultGameFrameTime

// setGameFrameTime records the native main loop's wall-clock delta and bounds
// simulation steps so an interrupted frame cannot advance physics too far.
func setGameFrameTime(dt float32) {
	if dt <= 0 {
		return
	}
	if dt > maxGameFrameTime {
		dt = maxGameFrameTime
	}
	gameFrameDelta = dt
}

func gameFrameTime() float32 {
	if gameFrameDelta <= 0 {
		return defaultGameFrameTime
	}
	return gameFrameDelta
}
