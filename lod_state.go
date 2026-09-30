package main

// Terrain deviations retain grid samples; structures retain every occupied
// above-ground column. The client owns both maps; jobs receive private maps
// and immutable occupancy records, never live chunks.
type lodPoint struct{ X, Z int }
type lodColumn struct {
	height    int
	top       byte
	occupancy *lodOccupancy // Immutable authoritative above-ground voxel runs.
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
			return lodColumn{height: y + 1, top: block}
		}
	}
	return lodColumn{}
}

func (w *World) recordLODColumn(chunk *Chunk, x, z int) {
	point := lodPoint{x, z}
	actual := snapshotLODColumn(chunk, x, z)
	baseline := sampleTerrainColumn(w.seed, x, z)
	want := lodColumn{height: baseline.height, top: baseline.top}
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
	// A raised prism at the last sample of a tile also controls the first
	// side wall in the next tile, whose worker snapshot includes a one-cell halo.
	if x%lodTileSize == lodTileSize-lodCellSize {
		w.lodVersions[lodTileKey{baseX + 1, baseZ}]++
	}
	if z%lodTileSize == lodTileSize-lodCellSize {
		w.lodVersions[lodTileKey{baseX, baseZ + 1}]++
	}
	if x%lodTileSize == lodTileSize-lodCellSize && z%lodTileSize == lodTileSize-lodCellSize {
		w.lodVersions[lodTileKey{baseX + 1, baseZ + 1}]++
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
	for z := 0; z < chunkWidth; z++ {
		for x := 0; x < chunkWidth; x++ {
			w.recordLODOccupancy(chunk, cx*chunkWidth+x, cz*chunkWidth+z)
		}
	}
	chunk.lodCaptured = true
}

func (w *World) resetLODState() {
	w.lodColumns = nil
	w.lodVersions = nil
	w.lodOccupancyTiles = nil
	w.chunksMu.RLock()
	for _, chunk := range w.chunks {
		chunk.lodCaptured = false
	}
	w.chunksMu.RUnlock()
}

func (w *World) snapshotLODTile(key lodTileKey) map[lodPoint]lodColumn {
	var result map[lodPoint]lodColumn
	for z := -1; z <= lodCellsPerTile; z++ {
		for x := -1; x <= lodCellsPerTile; x++ {
			point := lodPoint{key.X*lodTileSize + x*lodCellSize, key.Z*lodTileSize + z*lodCellSize}
			if column, ok := w.lodColumns[point]; ok {
				if result == nil {
					result = make(map[lodPoint]lodColumn)
				}
				result[point] = column
			}
		}
	}
	// Tile buckets keep this lookup proportional to nearby structures, rather
	// than scanning every building visited during the session.
	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			for point, occupancy := range w.lodOccupancyTiles[lodTileKey{key.X + dx, key.Z + dz}] {
				if point.X < key.X*lodTileSize-1 || point.X > (key.X+1)*lodTileSize || point.Z < key.Z*lodTileSize-1 || point.Z > (key.Z+1)*lodTileSize {
					continue
				}
				if result == nil {
					result = make(map[lodPoint]lodColumn)
				}
				column, ok := result[point]
				if !ok {
					column = lodColumn{height: occupancy.ground, top: occupancy.surface}
				}
				column.occupancy = occupancy
				result[point] = column
			}
		}
	}
	return result
}
