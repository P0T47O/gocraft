package main

// Beds occupy a foot and a head block at the same height. Bits 0-1 point
// from foot to head; shapeUpper identifies the head (not a vertical half).
func bedFacingFromYaw(yaw float32) byte { return (facingFromYaw(yaw) + 2) & 3 }

func bedOtherPos(x, y, z int, meta byte) (int, int, int) {
	dx, dz := faceOffset(meta & 3)
	if meta&shapeUpper != 0 {
		dx, dz = -dx, -dz
	}
	return x + dx, y, z + dz
}

func bedPartsMatch(w *World, x, y, z int, meta byte) bool {
	ox, oy, oz := bedOtherPos(x, y, z, meta)
	if w.getChunkIfGenerated(divFloor(ox, chunkWidth), divFloor(oz, chunkWidth)) == nil || w.BlockAt(ox, oy, oz) != blockBed {
		return false
	}
	otherMeta := w.MetaAt(ox, oy, oz)
	return otherMeta&3 == meta&3 && otherMeta&shapeUpper != meta&shapeUpper
}

func (w *World) canPlaceBed(x, y, z int, facing byte) bool {
	if y <= 0 || y >= chunkHeight || facing > 3 {
		return false
	}
	dx, dz := faceOffset(facing)
	for _, pos := range [][2]int{{x, z}, {x + dx, z + dz}} {
		bx, bz := pos[0], pos[1]
		if w.getChunkIfGenerated(divFloor(bx, chunkWidth), divFloor(bz, chunkWidth)) == nil {
			return false
		}
		block := w.BlockAt(bx, y, bz)
		if block != blockAir && block != blockWater || !GetBlock(w.BlockAt(bx, y-1, bz)).IsCollidable {
			return false
		}
	}
	return true
}

func (s *Server) removeOtherBedHalf(x, y, z int, meta byte) {
	if !bedPartsMatch(s.World, x, y, z, meta) {
		return
	}
	ox, oy, oz := bedOtherPos(x, y, z, meta)
	s.World.SetBlockAt(ox, oy, oz, blockAir)
	s.scheduleFluidAround(BlockPos{int32(ox), int32(oy), int32(oz)})
	s.Broadcast(&PacketBlockChange{X: int32(ox), Y: int32(oy), Z: int32(oz), BlockID: blockAir})
}

// Restore the client's predicted second half if placement or mining was
// rejected. The ordinary correction packet only names the clicked block.
func (s *Server) sendBedNeighbor(name string, x, y, z int, meta byte) {
	ox, oy, oz := bedOtherPos(x, y, z, meta)
	if s.World.getChunkIfGenerated(divFloor(ox, chunkWidth), divFloor(oz, chunkWidth)) == nil {
		return
	}
	s.SendTo(name, &PacketBlockChange{X: int32(ox), Y: int32(oy), Z: int32(oz), BlockID: s.World.BlockAt(ox, oy, oz), Meta: s.World.MetaAt(ox, oy, oz)})
}

func (s *Server) removeUnsupportedBedAbove(x, y, z int, survival bool) {
	by := y + 1
	if s.World.BlockAt(x, by, z) != blockBed || GetBlock(s.World.BlockAt(x, y, z)).IsCollidable {
		return
	}
	meta := s.World.MetaAt(x, by, z)
	s.World.SetBlockAt(x, by, z, blockAir)
	s.scheduleFluidAround(BlockPos{int32(x), int32(by), int32(z)})
	s.Broadcast(&PacketBlockChange{X: int32(x), Y: int32(by), Z: int32(z), BlockID: blockAir})
	s.removeOtherBedHalf(x, by, z, meta)
	if survival {
		s.dropFarmItem(x, by, z, blockBed, 1)
	}
}
