package main

import "math"

type Collider struct{ Width, Depth, Height, StepHeight float32 }

func moveColliderCore(world *World, pos, delta gameVec3, shape Collider) gameVec3 {
	// Sweep short segments, then bisect a blocked axis to land flush with surfaces.
	steps := int(math.Ceil(float64(max(abs32(delta.X), abs32(delta.Y), abs32(delta.Z))) / 0.2))
	if steps < 1 {
		return pos
	}
	step := gameVec3Scale(delta, 1/float32(steps))
	for n := 0; n < steps; n++ {
		for _, axis := range []int{0, 2, 1} {
			d := step.X
			if axis == 1 {
				d = step.Y
			}
			if axis == 2 {
				d = step.Z
			}
			if d == 0 {
				continue
			}
			target := offsetAxis(pos, axis, d)
			if !colliderHitsCore(world, target, shape) {
				pos = target
				continue
			}
			low, high := float32(0), float32(1)
			for j := 0; j < 10; j++ {
				mid := (low + high) / 2
				if colliderHitsCore(world, offsetAxis(pos, axis, d*mid), shape) {
					high = mid
				} else {
					low = mid
				}
			}
			pos = offsetAxis(pos, axis, d*low)
		}
	}
	return pos
}

func colliderHitsCore(world *World, pos gameVec3, shape Collider) bool {
	feetY := pos.Y
	minX := pos.X - shape.Width/2 - 0.001
	maxX := pos.X + shape.Width/2 + 0.001
	minZ := pos.Z - shape.Depth/2 - 0.001
	maxZ := pos.Z + shape.Depth/2 + 0.001
	minY := feetY
	maxY := feetY + shape.Height

	minBX := blockIndexFromCoord(minX)
	maxBX := blockIndexFromCoord(maxX)
	minBZ := blockIndexFromCoord(minZ)
	maxBZ := blockIndexFromCoord(maxZ)
	minBY := blockIndexFromCoord(minY)
	maxBY := blockIndexFromCoord(maxY)

	for x := minBX; x <= maxBX; x++ {
		for y := minBY; y <= maxBY; y++ {
			for z := minBZ; z <= maxBZ; z++ {
				if isSolidBlock(world.BlockAt(x, y, z)) {
					// Precise AABB check for centered blocks
					bx, by, bz := float32(x), float32(y), float32(z)
					if maxX > bx-0.5 && minX < bx+0.5 &&
						maxY > by-0.5 && minY < by+0.5 &&
						maxZ > bz-0.5 && minZ < bz+0.5 {
						return true
					}
				}
			}
		}
	}
	return false
}
