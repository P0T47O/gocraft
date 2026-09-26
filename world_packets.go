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
	if fresh {
		c.heightMap = [chunkWidth][chunkWidth]int16{}
		c.sectionBlocks = [sectionCount]uint16{}
		c.torches = c.torches[:0]
		c.torchCount = 0
		c.blocks.FromWire(p.Data, 0, 255)
		c.meta.FromWire(p.MetaData, 0, 255)
		c.skyLight.FromWire(p.LightData, 4, 15)
		c.blockLight.FromWire(p.LightData, 0, 15)
	}
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
				// Derive metadata directly from the packet already being scanned,
				// avoiding two extra passes through sparse storage on first receipt.
				if fresh && p.Data[idx] != blockAir {
					c.heightMap[x][z] = int16(y + 1)
					c.sectionBlocks[y/sectionHeight]++
					if p.Data[idx] == blockTorch {
						c.torches = append(c.torches, blockPos{x, y, z})
						c.torchCount++
					}
				}
				geometry := fresh || c.blocks.Get(x, y, z) != p.Data[idx] || c.meta.Get(x, y, z) != meta
				if fresh || geometry || c.skyLight.Get(x, y, z) != light>>4 || c.blockLight.Get(x, y, z) != light&15 {
					changed[y/sectionHeight] = true
					// Missing neighbors were sampled as air with full sky light.
					// An empty, sunlit new voxel does not invalidate their meshes.
					if !fresh || p.Data[idx] != blockAir || meta != 0 || light != 0xf0 {
						meshChanges.mark(x, y, z)
					}
				}
				geometryChanged = geometryChanged || geometry
				if !fresh {
					c.blocks.Set(x, y, z, p.Data[idx])
					c.meta.Set(x, y, z, meta)
					c.skyLight.Set(x, y, z, light>>4)
					c.blockLight.Set(x, y, z, light&15)
				}
				idx++
			}
		}
	}
	if geometryChanged && !fresh {
		c.rebuildHeightMap()
		c.rebuildTorchCount()
	}
	c.generated = true
	if !fresh {
		c.blocks.Compact()
		c.meta.Compact()
		c.skyLight.Compact()
		c.blockLight.Compact()
	}
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
	if geometryChanged || !c.lodCaptured {
		w.recordChunkLODColumns(c, cx, cz)
	}
	return true
}
