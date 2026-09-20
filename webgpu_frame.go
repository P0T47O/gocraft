package main

// Backend-neutral snapshots; the window adapter fills these on the owner thread.
type webGPUPoint struct {
	X, Y float32
}

type webGPUVec3 struct {
	X, Y, Z float32
}

type webGPUCamera struct {
	Position webGPUVec3
	Target   webGPUVec3
	Up       webGPUVec3
	Fovy     float32
}

type webGPUFrameContext struct {
	Camera        webGPUCamera
	Width, Height uint32
	Time          float32
}
