package main

const (
	defaultGameFrameTime = float32(1.0 / 60.0)
	maxGameFrameTime     = float32(0.05)
)

var gameFrameDelta = defaultGameFrameTime

// setGameFrameTime records the frame delta used by gameplay simulation. The
// OpenGL path feeds Raylib's measured frame time; the WebGPU path feeds its own
// wall-clock delta because Raylib EndDrawing is intentionally skipped there.
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
