package main

// HitTest keeps the input-facing query small while the voxel traversal lives in
// rayCast. The input layer now supplies renderer-neutral origin/direction
// vectors; no Raylib ray type crosses into World anymore.
func (w *World) HitTest(origin, direction gameVec3, maxDist float32) hitInfo {
	return w.rayCast(
		origin.X,
		origin.Y,
		origin.Z,
		direction.X,
		direction.Y,
		direction.Z,
		maxDist,
	)
}
