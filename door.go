package main

// A door is two blocks: facing in bits 0-1, upper in bit 2, open in bit 3.
// Only the lower half is placed or dropped as an inventory item.
func doorOtherY(y int, meta byte) int {
	if meta&shapeUpper != 0 {
		return y - 1
	}
	return y + 1
}

func (w *World) canPlaceDoor(x, y, z int) bool {
	return y > 0 && y < chunkHeight-1 &&
		isOpaqueBlock(w.BlockAt(x, y-1, z)) &&
		w.BlockAt(x, y+1, z) == blockAir
}

func (s *Server) toggleDoor(pos BlockPos) bool {
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	if s.World.BlockAt(x, y, z) != blockWoodDoor {
		return false
	}
	meta := s.World.MetaAt(x, y, z)
	otherY := doorOtherY(y, meta)
	if s.World.BlockAt(x, otherY, z) != blockWoodDoor {
		return true
	}
	otherMeta := s.World.MetaAt(x, otherY, z)
	if otherMeta&3 != meta&3 || otherMeta&shapeUpper == meta&shapeUpper {
		return true
	}
	if meta&shapeDouble == 0 {
		meta |= shapeDouble
		otherMeta |= shapeDouble
	} else {
		meta &^= shapeDouble
		otherMeta &^= shapeDouble
	}
	s.World.SetMetaAt(x, y, z, meta)
	s.World.SetMetaAt(x, otherY, z, otherMeta)
	s.Broadcast(&PacketBlockChange{X: pos.X, Y: pos.Y, Z: pos.Z, BlockID: blockWoodDoor, Meta: meta})
	s.Broadcast(&PacketBlockChange{X: pos.X, Y: int32(otherY), Z: pos.Z, BlockID: blockWoodDoor, Meta: otherMeta})
	return true
}

func (s *Server) removeOtherDoorHalf(x, y, z int, meta byte) {
	otherY := doorOtherY(y, meta)
	if s.World.BlockAt(x, otherY, z) != blockWoodDoor {
		return
	}
	otherMeta := s.World.MetaAt(x, otherY, z)
	if otherMeta&shapeUpper == meta&shapeUpper || otherMeta&3 != meta&3 {
		return
	}
	s.World.SetBlockAt(x, otherY, z, blockAir)
	s.Broadcast(&PacketBlockChange{X: int32(x), Y: int32(otherY), Z: int32(z), BlockID: blockAir})
}

func (s *Server) removeUnsupportedDoorAbove(x, y, z int, survival bool) {
	bottomY := y + 1
	if s.World.BlockAt(x, bottomY, z) != blockWoodDoor || s.World.MetaAt(x, bottomY, z)&shapeUpper != 0 {
		return
	}
	meta := s.World.MetaAt(x, bottomY, z)
	s.World.SetBlockAt(x, bottomY, z, blockAir)
	s.Broadcast(&PacketBlockChange{X: int32(x), Y: int32(bottomY), Z: int32(z), BlockID: blockAir})
	s.removeOtherDoorHalf(x, bottomY, z, meta)
	if survival {
		s.dropFarmItem(x, bottomY, z, blockWoodDoor, 1)
	}
}
