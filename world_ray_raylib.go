package main

import rl "github.com/gen2brain/raylib-go/raylib"

func (w *World) HitTest(ray rl.Ray, maxDist float32) hitInfo {
	return w.rayCast(
		ray.Position.X,
		ray.Position.Y,
		ray.Position.Z,
		ray.Direction.X,
		ray.Direction.Y,
		ray.Direction.Z,
		maxDist,
	)
}
