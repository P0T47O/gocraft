package main

import "math"

// gameCamera belongs to gameplay; render backends consume a snapshot of it.
type gameCamera struct {
	Position, Target, Up gameVec3
	Fovy                 float32
}

func newGameVec3(x, y, z float32) gameVec3 { return gameVec3{X: x, Y: y, Z: z} }

func gameVec3Add(a, b gameVec3) gameVec3 {
	return newGameVec3(a.X+b.X, a.Y+b.Y, a.Z+b.Z)
}

func gameVec3Subtract(a, b gameVec3) gameVec3 {
	return newGameVec3(a.X-b.X, a.Y-b.Y, a.Z-b.Z)
}

func (s *InputState) InitFromCamera(camera gameCamera) {
	dir := gameVec3Subtract(camera.Target, camera.Position)
	dir = gameVec3Normalize(dir)
	s.Yaw = float32(math.Atan2(float64(dir.X), float64(dir.Z)))
	s.Pitch = float32(math.Asin(float64(dir.Y)))
}

func (s *InputState) RayFromCenter(camera gameCamera) (gameVec3, gameVec3) {
	origin := camera.Position
	direction := gameVec3Normalize(gameVec3{
		X: camera.Target.X - camera.Position.X,
		Y: camera.Target.Y - camera.Position.Y,
		Z: camera.Target.Z - camera.Position.Z,
	})
	return origin, direction
}
