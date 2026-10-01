package main

import "slices"

// A chunk-sized client record contains no gameplay/light/mesh state. It is
// replaced on edits; background disk and mesh workers only see immutable data.
type lodObservedChunk struct {
	columns [chunkWidth * chunkWidth]lodColumn
}

func snapshotLODObservedColumn(chunk *Chunk, x, z int, base terrainColumn) lodColumn {
	lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
	chunk.mu.RLock()
	defer chunk.mu.RUnlock()
	ground := 0
	top := byte(blockAir)
	// Raised construction/fill remains explicit geometry rather than pulling
	// roofs into the interpolated ground. Digging/material changes below the
	// original surface are authoritative terrain.
	for y := min(int(chunk.heightMap[lx][lz]), base.height) - 1; y >= 0; y-- {
		b := chunk.blocks.Get(lx, y, lz)
		if lodSurfaceBlock(b) {
			ground, top = y+1, b
			break
		}
	}
	occupancy := &lodOccupancy{ground: ground, surface: top}
	for y := ground; y < int(chunk.heightMap[lx][lz]); y++ {
		b := chunk.blocks.Get(lx, y, lz)
		if !lodSurfaceBlock(b) && !generationIsLog(b) && !generationIsLeaf(b) {
			continue
		}
		spans := occupancy.spans
		if n := len(spans); n > 0 && spans[n-1].hi == y && spans[n-1].block == b {
			occupancy.spans[n-1].hi++
		} else {
			occupancy.spans = append(spans, lodSpan{y, y + 1, b})
		}
	}
	return lodColumn{height: ground, top: top, occupancy: occupancy, observed: true, changed: ground != base.height || top != base.top}
}

func (w *World) recordLODObservedChunk(chunk *Chunk, cx, cz int) {
	if w.lodCapture != nil {
		delete(w.lodCapture.pending, chunkKey{cx, cz})
	}
	w.installLODObservedChunk(chunkKey{cx, cz}, buildLODObservedChunk(chunk, w.seed, cx, cz), true)
}

func buildLODObservedChunk(chunk *Chunk, seed uint32, cx, cz int) *lodObservedChunk {
	var samples terrainSampleCache
	samples.init(seed, cx, cz)
	record := &lodObservedChunk{}
	for z := 0; z < chunkWidth; z++ {
		for x := 0; x < chunkWidth; x++ {
			wx, wz := cx*chunkWidth+x, cz*chunkWidth+z
			record.columns[z*chunkWidth+x] = snapshotLODObservedColumn(chunk, wx, wz, samples.column(seed, wx, wz))
		}
	}
	return record
}

func (w *World) recordLODObservedColumn(chunk *Chunk, x, z int) {
	key := chunkKey{divFloor(x, chunkWidth), divFloor(z, chunkWidth)}
	// An edit must fold in the latest real chunk, including changes not yet
	// published by a pending capture, rather than modifying an older disk record.
	if w.lodCapture != nil && w.lodCapture.pending[key] != 0 {
		w.recordLODObservedChunk(chunk, key.X, key.Z)
		return
	}
	old := w.lodObservedChunks[key]
	if old == nil {
		w.recordLODObservedChunk(chunk, key.X, key.Z)
		return
	}
	index := modFloor(z, chunkWidth)*chunkWidth + modFloor(x, chunkWidth)
	column := snapshotLODObservedColumn(chunk, x, z, sampleTerrainColumn(w.seed, x, z))
	previous := old.columns[index]
	if previous.height == column.height && previous.top == column.top && slices.Equal(previous.occupancy.spans, column.occupancy.spans) {
		return
	}
	record := *old
	record.columns[index] = column
	w.installLODObservedChunk(key, &record, true)
}

func (w *World) installLODObservedChunk(key chunkKey, record *lodObservedChunk, dirty bool) {
	if w.lodObservedChunks == nil {
		w.lodObservedChunks = make(map[chunkKey]*lodObservedChunk)
	}
	w.lodObservedChunks[key] = record
	if w.lodCache != nil {
		w.touchLODCache(key)
	}
	if w.lodVersions == nil {
		w.lodVersions = make(map[lodTileKey]uint64)
	}
	// Fine terrain patches and tree crowns read the adjacent chunk halo.
	for z := divFloor(key.Z*16-16, lodTileSize); z <= divFloor(key.Z*16+31, lodTileSize); z++ {
		for x := divFloor(key.X*16-16, lodTileSize); x <= divFloor(key.X*16+31, lodTileSize); x++ {
			w.lodVersions[lodTileKey{x, z}]++
		}
	}
	if dirty && w.lodCache != nil {
		if w.lodCacheEpoch == nil {
			w.lodCacheEpoch = make(map[chunkKey]uint64)
		}
		w.lodCacheEpoch[key]++
		if w.lodCacheDirty == nil {
			w.lodCacheDirty = make(map[chunkKey]bool)
		}
		w.lodCacheDirty[key] = true
	}
}
