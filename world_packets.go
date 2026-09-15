package main

import "time"

// Apply a complete server snapshot on the render thread. Duplicate retry replies
// do not invalidate meshes; lighting refreshes only rebuild affected sections.
func (w *World) applyChunkPacket(p *PacketChunkData) bool {
	start := time.Now()
	defer func() { perfMon.recordLoading(phaseClientApply, start) }()
	const size = chunkWidth * chunkHeight * chunkWidth
	if len(p.Data) != size || len(p.LightData) != size || (p.MetaData != nil && len(p.MetaData) != size) {
		return false
	}
	cx, cz := int(p.CX), int(p.CZ)
	c := w.requestChunk(cx, cz)
	fresh := !c.generated
	var changed [sectionCount]bool
	var meshChanges chunkMeshChanges
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
					// Missing neighbors were sampled as air with full sky light.
					// An empty, sunlit new voxel does not invalidate their meshes.
					if !fresh || p.Data[idx] != blockAir || meta != 0 || light != 0xf0 {
						meshChanges.mark(x, y, z)
					}
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
	if fresh {
		meshChanges[1][1] = 0xffff
	}
	meshChanges.apply(w, cx, cz)
	for sec, dirty := range changed {
		if !dirty {
			continue
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
