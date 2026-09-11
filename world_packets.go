package main

// Apply a complete server snapshot on the render thread. Duplicate retry replies
// do not invalidate meshes; lighting refreshes only rebuild affected sections.
func (w *World) applyChunkPacket(p *PacketChunkData) bool {
	const size = chunkWidth * chunkHeight * chunkWidth
	if len(p.Data) != size || len(p.LightData) != size || (p.MetaData != nil && len(p.MetaData) != size) {
		return false
	}
	cx, cz := int(p.CX), int(p.CZ)
	c := w.requestChunk(cx, cz)
	fresh := !c.generated
	var changed [sectionCount]bool
	geometryChanged := fresh
	idx := 0
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				meta := byte(0)
				if p.MetaData != nil {
					meta = p.MetaData[idx]
				}
				light := p.LightData[idx]
				geometry := c.blocks[x][y][z] != p.Data[idx] || c.meta[x][y][z] != meta
				if fresh || geometry || c.skyLight[x][y][z] != light>>4 || c.blockLight[x][y][z] != light&15 {
					changed[y/sectionHeight] = true
				}
				geometryChanged = geometryChanged || geometry
				c.blocks[x][y][z] = p.Data[idx]
				c.meta[x][y][z] = meta
				c.skyLight[x][y][z] = light >> 4
				c.blockLight[x][y][z] = light & 15
				idx++
			}
		}
	}
	if geometryChanged {
		c.rebuildHeightMap()
		c.rebuildTorchCount()
	}
	c.generated = true
	ensureChunkSections(c)
	for sec, dirty := range changed {
		if !dirty {
			continue
		}
		// The one-voxel sampling halo can cross X/Z and vertical section edges.
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				for s := max(0, sec-1); s <= min(sectionCount-1, sec+1); s++ {
					w.markChunkSectionDirty(cx+dx, cz+dz, s)
				}
			}
		}
		if c.sectionBlocks[sec] == 0 {
			clearSectionMeshes(c, sec)
			c.sectionDirty[sec] = false
		}
	}
	if fresh {
		perfMon.IncrementChunkLoad()
	}
	w.applyPendingEdits(chunkKey{cx, cz})
	return true
}
