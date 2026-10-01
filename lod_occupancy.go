package main

import "slices"

// Runs are half-open Y intervals of one material in a one-block X/Z column.
// Records are replaced, never mutated: worker snapshots can safely retain them.
type lodSpan struct {
	lo, hi int
	block  byte
}

type lodOccupancy struct {
	ground  int
	surface byte
	spans   []lodSpan
}

func (w *World) recordLODOccupancy(chunk *Chunk, x, z int) {
	base := sampleTerrainColumn(w.seed, x, z)
	point := lodPoint{x, z}
	key := lodTileKey{divFloor(x, lodTileSize), divFloor(z, lodTileSize)}
	old := w.lodOccupancyTiles[key][point]
	lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
	chunk.mu.RLock()
	height := int(chunk.heightMap[lx][lz])
	var spans []lodSpan
	// Natural trunks are identified by the shared deterministic tree anchor,
	// rather than excluding every log and thereby losing wooden buildings.
	var anchor treeAnchor
	var naturalTree bool
	for y := base.height; y < height; y++ {
		block := chunk.blocks.Get(lx, y, lz)
		if generationIsLog(block) {
			if anchor.height == 0 {
				anchor, naturalTree = treeAnchorFromColumn(w.seed, x, z, base)
				if anchor.height == 0 {
					anchor.height = -1
				}
			}
			if naturalTree && block == anchor.log && y >= anchor.y && y < anchor.y+anchor.height {
				continue
			}
		} else if !lodSurfaceBlock(block) {
			continue
		}
		if n := len(spans); n > 0 && spans[n-1].hi == y && spans[n-1].block == block {
			spans[n-1].hi++
		} else {
			spans = append(spans, lodSpan{y, y + 1, block})
		}
	}
	chunk.mu.RUnlock()
	if old == nil && len(spans) == 0 || old != nil && slices.Equal(old.spans, spans) {
		return
	}
	if len(spans) == 0 {
		delete(w.lodOccupancyTiles[key], point)
		if len(w.lodOccupancyTiles[key]) == 0 {
			delete(w.lodOccupancyTiles, key)
		}
	} else {
		if w.lodOccupancyTiles == nil {
			w.lodOccupancyTiles = make(map[lodTileKey]map[lodPoint]*lodOccupancy)
		}
		if w.lodOccupancyTiles[key] == nil {
			w.lodOccupancyTiles[key] = make(map[lodPoint]*lodOccupancy)
		}
		w.lodOccupancyTiles[key][point] = &lodOccupancy{base.height, base.top, spans}
	}
	if w.lodVersions == nil {
		w.lodVersions = make(map[lodTileKey]uint64)
	}
	w.lodVersions[key]++
	// A neighbor's exposed side changes at a tile edge too.
	if modFloor(x, lodTileSize) == 0 {
		w.lodVersions[lodTileKey{key.X - 1, key.Z}]++
	}
	if modFloor(x, lodTileSize) == lodTileSize-1 {
		w.lodVersions[lodTileKey{key.X + 1, key.Z}]++
	}
	if modFloor(z, lodTileSize) == 0 {
		w.lodVersions[lodTileKey{key.X, key.Z - 1}]++
	}
	if modFloor(z, lodTileSize) == lodTileSize-1 {
		w.lodVersions[lodTileKey{key.X, key.Z + 1}]++
	}
}

func appendLODOccupancy(data *lodTileData, key lodTileKey, columns map[lodPoint]lodColumn) {
	appendLODObservedFoliage(data, key, columns)
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for point, column := range columns {
		if column.occupancy == nil || point.X < baseX || point.X >= baseX+lodTileSize || point.Z < baseZ || point.Z >= baseZ+lodTileSize {
			continue
		}
		x0, x1 := float32(point.X)-.5, float32(point.X)+.5
		z0, z1 := float32(point.Z)-.5, float32(point.Z)+.5
		for _, span := range column.occupancy.spans {
			if generationIsLeaf(span.block) {
				continue
			}
			lo, hi := float32(span.lo)-.5, float32(span.hi)-.5
			// Only true air gaps receive caps, including undersides of bridges.
			if !lodOccupiedAt(column.occupancy, span.hi) {
				appendLODStructureQuad(data, [4][3]float32{{x0, hi, z0}, {x0, hi, z1}, {x1, hi, z0}, {x1, hi, z1}}, lodStructureColor(span.block, 1))
			}
			if span.lo > column.occupancy.ground && !lodOccupiedAt(column.occupancy, span.lo-1) {
				appendLODStructureQuad(data, [4][3]float32{{x0, lo, z1}, {x0, lo, z0}, {x1, lo, z1}, {x1, lo, z0}}, lodStructureColor(span.block, .65))
			}
			wall := func(nx, nz int, endpoints [2][2]float32, shade float32) {
				neighbor := columns[lodPoint{nx, nz}].occupancy
				start := -1
				for y := span.lo; y <= span.hi; y++ {
					exposed := y < span.hi && !lodOccupiedAt(neighbor, y)
					if exposed && start < 0 {
						start = y
					}
					if !exposed && start >= 0 {
						bottom, top := float32(start)-.5, float32(y)-.5
						appendLODStructureQuad(data, [4][3]float32{{endpoints[0][0], top, endpoints[0][1]}, {endpoints[1][0], top, endpoints[1][1]}, {endpoints[0][0], bottom, endpoints[0][1]}, {endpoints[1][0], bottom, endpoints[1][1]}}, lodStructureColor(span.block, shade))
						start = -1
					}
				}
			}
			wall(point.X-1, point.Z, [2][2]float32{{x0, z1}, {x0, z0}}, .78)
			wall(point.X+1, point.Z, [2][2]float32{{x1, z0}, {x1, z1}}, .78)
			wall(point.X, point.Z-1, [2][2]float32{{x0, z0}, {x1, z0}}, .88)
			wall(point.X, point.Z+1, [2][2]float32{{x1, z1}, {x0, z1}}, .88)
		}
	}
}

func lodOccupiedAt(occupancy *lodOccupancy, y int) bool {
	if occupancy == nil {
		return false
	}
	if y < occupancy.ground {
		return true
	}
	for _, span := range occupancy.spans {
		if y >= span.lo && y < span.hi {
			return true
		}
	}
	return false
}
