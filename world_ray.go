package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (w *World) HitTest(ray rl.Ray, maxDist float32) hitInfo {
	return w.rayCast(ray, maxDist)
}

func (w *World) rayCast(ray rl.Ray, maxDist float32) hitInfo {
	dir := ray.Direction
	if dir.X == 0 && dir.Y == 0 && dir.Z == 0 {
		return hitInfo{}
	}
	ox := ray.Position.X
	oy := ray.Position.Y
	oz := ray.Position.Z

	// Centered coordinates: Block 0 is [-0.5, 0.5]
	x := int(math.Floor(float64(ox + 0.5)))
	y := int(math.Floor(float64(oy + 0.5)))
	z := int(math.Floor(float64(oz + 0.5)))

	if w.BlockAt(x, y, z) != blockAir && w.BlockAt(x, y, z) != blockWater {
		return hitInfo{x: x, y: y, z: z, hit: true}
	}

	stepX := sign(dir.X)
	stepY := sign(dir.Y)
	stepZ := sign(dir.Z)

	tDeltaX := axisDelta(dir.X)
	tDeltaY := axisDelta(dir.Y)
	tDeltaZ := axisDelta(dir.Z)

	tMaxX := axisMax(ox, float32(x), dir.X, stepX)
	tMaxY := axisMax(oy, float32(y), dir.Y, stepY)
	tMaxZ := axisMax(oz, float32(z), dir.Z, stepZ)

	var dist float32
	var normal rl.Vector3

	for dist <= maxDist {
		if tMaxX < tMaxY {
			if tMaxX < tMaxZ {
				x += stepX
				dist = tMaxX
				tMaxX += tDeltaX
				normal = rl.NewVector3(float32(-stepX), 0, 0)
			} else {
				z += stepZ
				dist = tMaxZ
				tMaxZ += tDeltaZ
				normal = rl.NewVector3(0, 0, float32(-stepZ))
			}
		} else {
			if tMaxY < tMaxZ {
				y += stepY
				dist = tMaxY
				tMaxY += tDeltaY
				normal = rl.NewVector3(0, float32(-stepY), 0)
			} else {
				z += stepZ
				dist = tMaxZ
				tMaxZ += tDeltaZ
				normal = rl.NewVector3(0, 0, float32(-stepZ))
			}
		}
		if dist > maxDist {
			break
		}
		if w.BlockAt(x, y, z) != blockAir && w.BlockAt(x, y, z) != blockWater {
			return hitInfo{
				x:        x,
				y:        y,
				z:        z,
				normal:   normal,
				distance: dist,
				hit:      true,
			}
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
