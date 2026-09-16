package main

import rl "github.com/gen2brain/raylib-go/raylib"

// These adapters keep Raylib vectors at the current input/camera boundary while
// shared movement and collision simulation use gameVec3 internally.
func gameVec3FromRaylib(v rl.Vector3) gameVec3 {
	return gameVec3{X: v.X, Y: v.Y, Z: v.Z}
}

func raylibVec3FromGame(v gameVec3) rl.Vector3 {
	return rl.NewVector3(v.X, v.Y, v.Z)
}

func moveCollider(world *World, pos, delta rl.Vector3, shape Collider) rl.Vector3 {
	return raylibVec3FromGame(moveColliderCore(world, gameVec3FromRaylib(pos), gameVec3FromRaylib(delta), shape))
}

func colliderHits(world *World, pos rl.Vector3, shape Collider) bool {
	return colliderHitsCore(world, gameVec3FromRaylib(pos), shape)
}

func feetSupported(w *World, p rl.Vector3) bool {
	return feetSupportedCore(w, gameVec3FromRaylib(p))
}

func touchesLiquid(w *World, p rl.Vector3, id byte) bool {
	return touchesLiquidCore(w, gameVec3FromRaylib(p), id)
}

func (s *InputState) StepMovement(w *World, p rl.Vector3, dt float32, c MovementControls, creative bool) rl.Vector3 {
	return raylibVec3FromGame(s.stepMovementCore(w, gameVec3FromRaylib(p), dt, c, creative))
}
