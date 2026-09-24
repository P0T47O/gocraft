package main

import "math"

// torchRayHit intersects the two rendered cuboids, not the enclosing voxel.
// entry and exit bound the portion of the ray inside this voxel.
func torchRayHit(meta byte, x, y, z int, origin, direction gameVec3, entry, exit float32) (float32, gameVec3, bool) {
	g := geometryForTorch(meta)
	localOrigin := meshVec3(origin.X-float32(x)-g.offset.X, origin.Y-float32(y)-g.offset.Y, origin.Z-float32(z)-g.offset.Z)
	toLocal := meshRotation(g.axis, -g.angle)
	localOrigin = meshTransform(localOrigin, toLocal)
	localDirection := meshTransform(direction, toLocal)
	toWorld := meshRotation(g.axis, g.angle)

	best := float32(math.Inf(1))
	var bestNormal gameVec3
	for _, box := range [2]struct{ center, size gameVec3 }{
		{meshVec3(0, 0, 0), g.stemSize},
		{g.flameOffset, g.flameSize},
	} {
		if distance, normal, ok := rayCuboid(localOrigin, localDirection, box.center, box.size); ok && distance >= entry-0.00001 && distance <= exit+0.00001 && distance < best {
			best = distance
			bestNormal = meshTransform(normal, toWorld)
		}
	}
	return best, bestNormal, !math.IsInf(float64(best), 1)
}

func rayCuboid(origin, direction, center, size gameVec3) (float32, gameVec3, bool) {
	near, far := float32(math.Inf(-1)), float32(math.Inf(1))
	var normal gameVec3
	for axis := 0; axis < 3; axis++ {
		var o, d, c, half float32
		switch axis {
		case 0:
			o, d, c, half = origin.X, direction.X, center.X, size.X*0.5
		case 1:
			o, d, c, half = origin.Y, direction.Y, center.Y, size.Y*0.5
		default:
			o, d, c, half = origin.Z, direction.Z, center.Z, size.Z*0.5
		}
		if d == 0 {
			if o < c-half || o > c+half {
				return 0, gameVec3{}, false
			}
			continue
		}
		a, b := (c-half-o)/d, (c+half-o)/d
		face := float32(-1)
		if a > b {
			a, b = b, a
			face = 1
		}
		if a > near {
			near = a
			normal = gameVec3{}
			switch axis {
			case 0:
				normal.X = face
			case 1:
				normal.Y = face
			default:
				normal.Z = face
			}
		}
		if b < far {
			far = b
		}
		if near > far {
			return 0, gameVec3{}, false
		}
	}
	if far < 0 {
		return 0, gameVec3{}, false
	}
	if near < 0 { // Camera starts inside the model.
		return 0, gameVec3{}, true
	}
	return near, normal, true
}
