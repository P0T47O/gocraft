package main

// Only samples on the LOD vertex grid are retained. The client world owns
// these maps; render jobs receive private copies and never read live chunks.
type lodPoint struct{ X, Z int }
type lodColumn struct {
	height int
	top    byte
}

func lodSurfaceBlock(block byte) bool {
	switch block {
	case blockAir, blockWater, blockLava, blockIce, blockTorch,
		blockLeaves, blockLeavesBirch, blockLeavesSpruce,
		blockLog, blockLogBirch, blockLogSpruce,
		blockTallGrass, blockRose, blockDandelion, blockDeadBush,
		blockWheatCrop, blockPotatoCrop, blockCarrotCrop, blockCactus:
		return false
	default:
		return true
	}
}

func snapshotLODColumn(chunk *Chunk, x, z int) lodColumn {
	lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
	chunk.mu.RLock()
	defer chunk.mu.RUnlock()
	for y := int(chunk.heightMap[lx][lz]) - 1; y >= 0; y-- {
		block := chunk.blocks.Get(lx, y, lz)
		if lodSurfaceBlock(block) {
			return lodColumn{y + 1, block}
		}
	}
	return lodColumn{}
}

func (w *World) recordLODColumn(chunk *Chunk, x, z int) {
	point := lodPoint{x, z}
	actual := snapshotLODColumn(chunk, x, z)
	baseline := sampleTerrainColumn(w.seed, x, z)
	want := lodColumn{baseline.height, baseline.top}
	old, had := w.lodColumns[point]
	if actual == want {
		if !had {
			return
		}
		delete(w.lodColumns, point)
	} else {
		if had && old == actual {
			return
		}
		if w.lodColumns == nil {
			w.lodColumns = make(map[lodPoint]lodColumn)
		}
		w.lodColumns[point] = actual
	}
	if w.lodVersions == nil {
		w.lodVersions = make(map[lodTileKey]uint64)
	}
	// Grid samples at tile edges belong to both adjacent meshes.
	baseX, baseZ := divFloor(x, lodTileSize), divFloor(z, lodTileSize)
	for dz := 0; dz <= 1; dz++ {
		if dz == 1 && z%lodTileSize != 0 {
			continue
		}
		for dx := 0; dx <= 1; dx++ {
			if dx == 1 && x%lodTileSize != 0 {
				continue
			}
			w.lodVersions[lodTileKey{baseX - dx, baseZ - dz}]++
		}
	}
}

func (w *World) recordChunkLODColumns(chunk *Chunk, cx, cz int) {
	if !w.IsClient || (horizonDistance() <= renderDistance() && !chunk.lodCaptured) {
		return
	}
	for z := 0; z < chunkWidth; z += lodCellSize {
		for x := 0; x < chunkWidth; x += lodCellSize {
			w.recordLODColumn(chunk, cx*chunkWidth+x, cz*chunkWidth+z)
		}
	}
	chunk.lodCaptured = true
}

func (w *World) resetLODState() {
	w.lodColumns = nil
	w.lodVersions = nil
	w.chunksMu.RLock()
	for _, chunk := range w.chunks {
		chunk.lodCaptured = false
	}
	w.chunksMu.RUnlock()
}

func (w *World) snapshotLODTile(key lodTileKey) map[lodPoint]lodColumn {
	var result map[lodPoint]lodColumn
	for z := 0; z <= lodCellsPerTile; z++ {
		for x := 0; x <= lodCellsPerTile; x++ {
			point := lodPoint{key.X*lodTileSize + x*lodCellSize, key.Z*lodTileSize + z*lodCellSize}
			if column, ok := w.lodColumns[point]; ok {
				if result == nil {
					result = make(map[lodPoint]lodColumn)
				}
				result[point] = column
			}
		}
	}
	return result
}
