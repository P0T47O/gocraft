package main

import "math"

// Horizontal metadata points at the visible front: north, east, south, west.
// Bit 2 raises slabs or turns stairs upside down. Other blocks ignore it.
const (
	faceNorth byte = iota
	faceEast
	faceSouth
	faceWest
	shapeUpper  byte = 4
	shapeDouble byte = 8
)

type shapeBox struct{ minX, minY, minZ, maxX, maxY, maxZ float32 }

func isSlab(id byte) bool        { return id == blockOakSlab || id == blockStoneSlab }
func isStair(id byte) bool       { return id == blockOakStairs || id == blockCobbleStairs }
func isShapedBlock(id byte) bool { return isSlab(id) || isStair(id) || id == blockWoodDoor }

func facingFromYaw(yaw float32) byte {
	sx, sz := math.Sin(float64(yaw)), math.Cos(float64(yaw))
	if math.Abs(sx) > math.Abs(sz) {
		if sx > 0 {
			return faceWest
		}
		return faceEast
	}
	if sz > 0 {
		return faceNorth
	}
	return faceSouth
}

func placementMeta(id byte, yaw float32, upper bool) byte {
	if isSlab(id) {
		if upper {
			return shapeUpper
		}
		return 0
	}
	if isStair(id) {
		meta := facingFromYaw(yaw)
		if upper {
			meta |= shapeUpper
		}
		return meta
	}
	if id == blockChest || id == blockFurnace || id == blockWoodDoor {
		return facingFromYaw(yaw)
	}
	return 0
}

func validPlacementMeta(id, meta byte) bool {
	switch {
	case isSlab(id):
		return meta == 0 || meta == shapeUpper
	case isStair(id):
		return meta <= 7
	case id == blockChest || id == blockFurnace || id == blockWoodDoor:
		return meta <= 3
	case id == blockTorch:
		return meta <= 4
	default:
		return meta == 0
	}
}

func shapeBoxes(id, meta byte) ([3]shapeBox, int) {
	var boxes [3]shapeBox
	if !isShapedBlock(id) {
		return boxes, 0
	}
	if id == blockWoodDoor {
		const thickness = float32(3.0 / 16.0)
		switch meta & 3 {
		case faceNorth:
			if meta&shapeDouble == 0 {
				boxes[0] = shapeBox{-.5, -.5, -.5, .5, .5, -.5 + thickness}
			} else {
				boxes[0] = shapeBox{.5 - thickness, -.5, -.5, .5, .5, .5}
			}
		case faceEast:
			if meta&shapeDouble == 0 {
				boxes[0] = shapeBox{.5 - thickness, -.5, -.5, .5, .5, .5}
			} else {
				boxes[0] = shapeBox{-.5, -.5, .5 - thickness, .5, .5, .5}
			}
		case faceSouth:
			if meta&shapeDouble == 0 {
				boxes[0] = shapeBox{-.5, -.5, .5 - thickness, .5, .5, .5}
			} else {
				boxes[0] = shapeBox{-.5, -.5, -.5, -.5 + thickness, .5, .5}
			}
		case faceWest:
			if meta&shapeDouble == 0 {
				boxes[0] = shapeBox{-.5, -.5, -.5, -.5 + thickness, .5, .5}
			} else {
				boxes[0] = shapeBox{-.5, -.5, -.5, .5, .5, -.5 + thickness}
			}
		}
		return boxes, 1
	}
	if isSlab(id) && meta&shapeDouble != 0 {
		boxes[0] = shapeBox{-.5, -.5, -.5, .5, .5, .5}
		return boxes, 1
	}
	base := shapeBox{-.5, -.5, -.5, .5, 0, .5}
	if meta&shapeUpper != 0 {
		base.minY, base.maxY = 0, .5
	}
	boxes[0] = base
	if isSlab(id) {
		return boxes, 1
	}
	riser := shapeBox{-.5, 0, -.5, .5, .5, .5}
	if meta&shapeUpper != 0 {
		riser.minY, riser.maxY = -.5, 0
	}
	// Riser occupies the half away from the visible/front face.
	switch meta & 3 {
	case faceNorth:
		riser.minZ = 0
	case faceSouth:
		riser.maxZ = 0
	case faceEast:
		riser.maxX = 0
	case faceWest:
		riser.minX = 0
	}
	boxes[1] = riser
	return boxes, 2
}

func faceOffset(face byte) (int, int) {
	switch face & 3 {
	case faceNorth:
		return 0, -1
	case faceEast:
		return 1, 0
	case faceSouth:
		return 0, 1
	default:
		return -1, 0
	}
}

// Corners depend on adjacent stairs, not stored metadata. The same derived
// shape is used by meshing, collision and picking, including across chunks.
func shapeBoxesAt(id, meta byte, x, y, z int, getBlock BlockGetter, getMeta MetaGetter) ([3]shapeBox, int) {
	boxes, count := shapeBoxes(id, meta)
	if !isStair(id) {
		return boxes, count
	}
	front := meta & 3
	perpendicular := func(nx, nz int) (byte, bool) {
		if getBlock(nx, y, nz) != id {
			return 0, false
		}
		nm := getMeta(nx, y, nz)
		return nm & 3, nm&shapeUpper == meta&shapeUpper && (nm&1) != (front&1)
	}
	quarter := func(box *shapeBox, neighboringFront byte) {
		// Keep the lateral half on the neighbor's high side.
		switch neighboringFront {
		case faceNorth:
			box.minZ = 0
		case faceSouth:
			box.maxZ = 0
		case faceEast:
			box.maxX = 0
		case faceWest:
			box.minX = 0
		}
	}
	backX, backZ := faceOffset((front + 2) & 3)
	if nf, ok := perpendicular(x+backX, z+backZ); ok {
		quarter(&boxes[1], nf)
		return boxes, 2 // outer corner
	}
	frontX, frontZ := faceOffset(front)
	if nf, ok := perpendicular(x+frontX, z+frontZ); ok {
		extra := boxes[1]
		switch front {
		case faceNorth:
			extra.minZ, extra.maxZ = -.5, 0
		case faceSouth:
			extra.minZ, extra.maxZ = 0, .5
		case faceEast:
			extra.minX, extra.maxX = 0, .5
		case faceWest:
			extra.minX, extra.maxX = -.5, 0
		}
		quarter(&extra, nf)
		boxes[2] = extra
		return boxes, 3 // inner corner
	}
	return boxes, count
}

func slabDropCount(id, meta byte) int {
	if isSlab(id) && meta&shapeDouble != 0 {
		return 2
	}
	return 1
}

func orientedTextures(id, meta byte, faces blockFaces) blockFaces {
	if id != blockChest && id != blockFurnace {
		return faces
	}
	front := faces.North
	side := faces.South
	faces.North, faces.South, faces.East, faces.West = side, side, side, side
	switch meta & 3 {
	case faceNorth:
		faces.North = front
	case faceEast:
		faces.East = front
	case faceSouth:
		faces.South = front
	case faceWest:
		faces.West = front
	}
	return faces
}

func shapedRayHit(id, meta byte, x, y, z int, origin, direction gameVec3, entry, exit float32, getBlock BlockGetter, getMeta MetaGetter) (float32, gameVec3, bool) {
	boxes, count := shapeBoxesAt(id, meta, x, y, z, getBlock, getMeta)
	best := exit
	var normal gameVec3
	found := false
	for _, box := range boxes[:count] {
		minV := [3]float32{float32(x) + box.minX, float32(y) + box.minY, float32(z) + box.minZ}
		maxV := [3]float32{float32(x) + box.maxX, float32(y) + box.maxY, float32(z) + box.maxZ}
		orig := [3]float32{origin.X, origin.Y, origin.Z}
		dir := [3]float32{direction.X, direction.Y, direction.Z}
		near, far := entry, best
		var face gameVec3
		valid := true
		for axis := 0; axis < 3; axis++ {
			if math.Abs(float64(dir[axis])) < 1e-7 {
				if orig[axis] < minV[axis] || orig[axis] > maxV[axis] {
					valid = false
					break
				}
				continue
			}
			a, b := (minV[axis]-orig[axis])/dir[axis], (maxV[axis]-orig[axis])/dir[axis]
			sign := float32(-1)
			if a > b {
				a, b = b, a
				sign = 1
			}
			if a >= near {
				near = a
				face = gameVec3{}
				switch axis {
				case 0:
					face.X = sign
				case 1:
					face.Y = sign
				case 2:
					face.Z = sign
				}
			}
			far = min(far, b)
			if near > far {
				valid = false
				break
			}
		}
		if valid && near >= entry && near <= best {
			best, normal, found = near, face, true
		}
	}
	return best, normal, found
}
