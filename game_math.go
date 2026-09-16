package main

import "math"

// gameVec3 is the renderer-neutral vector used by gameplay, collision and
// server simulation. Presentation/input layers may adapt renderer-native
// vectors at their boundary, but shared gameplay must not depend on Raylib.
type gameVec3 struct {
	X, Y, Z float32
}

func gameVec3Scale(v gameVec3, scale float32) gameVec3 {
	return gameVec3{X: v.X * scale, Y: v.Y * scale, Z: v.Z * scale}
}

func gameVec3Length(v gameVec3) float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

func gameVec3Normalize(v gameVec3) gameVec3 {
	length := gameVec3Length(v)
	if length == 0 {
		return gameVec3{}
	}
	return gameVec3Scale(v, 1/length)
}
