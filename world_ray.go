package main

import "math"

func (w *World) rayCast(originX, originY, originZ, dirX, dirY, dirZ, maxDist float32) hitInfo {
	if dirX == 0 && dirY == 0 && dirZ == 0 {
		return hitInfo{}
	}

	// Centered coordinates: Block 0 is [-0.5, 0.5]
	x := int(math.Floor(float64(originX + 0.5)))
	y := int(math.Floor(float64(originY + 0.5)))
	z := int(math.Floor(float64(originZ + 0.5)))

	stepX := sign(dirX)
	stepY := sign(dirY)
	stepZ := sign(dirZ)

	tDeltaX := axisDelta(dirX)
	tDeltaY := axisDelta(dirY)
	tDeltaZ := axisDelta(dirZ)

	tMaxX := axisMax(originX, float32(x), dirX, stepX)
	tMaxY := axisMax(originY, float32(y), dirY, stepY)
	tMaxZ := axisMax(originZ, float32(z), dirZ, stepZ)

	var dist float32
	var normal struct {
		X, Y, Z float32
	}
	origin := meshVec3(originX, originY, originZ)
	direction := meshVec3(dirX, dirY, dirZ)
	checkVoxel := func(entry float32) hitInfo {
		block := w.BlockAt(x, y, z)
		if block == blockAir || block == blockWater {
			return hitInfo{}
		}
		if block == blockTorch {
			exit := min(maxDist, tMaxX, tMaxY, tMaxZ)
			if hitDist, hitNormal, ok := torchRayHit(w.MetaAt(x, y, z), x, y, z, origin, direction, entry, exit); ok {
				return hitInfo{x: x, y: y, z: z, normal: struct{ X, Y, Z float32 }{hitNormal.X, hitNormal.Y, hitNormal.Z}, distance: hitDist, hit: true}
			}
			return hitInfo{}
		}
		return hitInfo{x: x, y: y, z: z, normal: normal, distance: entry, hit: true}
	}
	if hit := checkVoxel(0); hit.hit {
		return hit
	}

	for dist <= maxDist {
		if tMaxX < tMaxY {
			if tMaxX < tMaxZ {
				x += stepX
				dist = tMaxX
				tMaxX += tDeltaX
				normal = struct {
					X, Y, Z float32
				}{X: float32(-stepX)}
			} else {
				z += stepZ
				dist = tMaxZ
				tMaxZ += tDeltaZ
				normal = struct {
					X, Y, Z float32
				}{Z: float32(-stepZ)}
			}
		} else {
			if tMaxY < tMaxZ {
				y += stepY
				dist = tMaxY
				tMaxY += tDeltaY
				normal = struct {
					X, Y, Z float32
				}{Y: float32(-stepY)}
			} else {
				z += stepZ
				dist = tMaxZ
				tMaxZ += tDeltaZ
				normal = struct {
					X, Y, Z float32
				}{Z: float32(-stepZ)}
			}
		}
		if dist > maxDist {
			break
		}
		if hit := checkVoxel(dist); hit.hit {
			return hit
		}
	}
	return hitInfo{}
}

func sign(v float32) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

func axisDelta(v float32) float32 {
	if v == 0 {
		return float32(math.Inf(1))
	}
	return float32(math.Abs(float64(1.0 / v)))
}

func axisMax(origin float32, voxel float32, dir float32, step int) float32 {
	if dir == 0 || step == 0 {
		return float32(math.Inf(1))
	}
	var boundary float32
	if step > 0 {
		boundary = voxel + 0.5
	} else {
		boundary = voxel - 0.5
	}
	return (boundary - origin) / dir
}
