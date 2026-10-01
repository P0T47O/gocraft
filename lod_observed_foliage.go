package main

// Keep the cache's exact leaf runs, but bound the rendered crown complexity.
// Each occupied 4x4x8 cell emits one box fitted to actual leaves; empty cells
// emit nothing, so felled trees do not return as procedural silhouettes.
func appendLODObservedFoliage(data *lodTileData, tile lodTileKey, columns map[lodPoint]lodColumn) {
	type cell struct {
		x, y, z int
		block   byte
	}
	type bounds struct{ x0, x1, y0, y1, z0, z1 int }
	crowns := make(map[cell]bounds)
	baseX, baseZ := tile.X*lodTileSize, tile.Z*lodTileSize
	for p, c := range columns {
		if c.occupancy == nil || p.X < baseX || p.X >= baseX+lodTileSize || p.Z < baseZ || p.Z >= baseZ+lodTileSize {
			continue
		}
		for _, run := range c.occupancy.spans {
			if !generationIsLeaf(run.block) {
				continue
			}
			for y := run.lo; y < run.hi; {
				end := min(run.hi, (divFloor(y, 8)+1)*8)
				key := cell{divFloor(p.X, 4), divFloor(y, 8), divFloor(p.Z, 4), run.block}
				b, exists := crowns[key]
				if !exists {
					b = bounds{p.X, p.X + 1, y, end, p.Z, p.Z + 1}
				} else {
					b.x0, b.x1 = min(b.x0, p.X), max(b.x1, p.X+1)
					b.y0, b.y1 = min(b.y0, y), max(b.y1, end)
					b.z0, b.z1 = min(b.z0, p.Z), max(b.z1, p.Z+1)
				}
				crowns[key] = b
				y = end
			}
		}
	}
	for key, b := range crowns {
		x0, x1 := float32(b.x0)-.5, float32(b.x1)-.5
		y0, y1 := float32(b.y0)-.5, float32(b.y1)-.5
		z0, z1 := float32(b.z0)-.5, float32(b.z1)-.5
		appendLODStructureQuad(data, [4][3]float32{{x0, y1, z0}, {x0, y1, z1}, {x1, y1, z0}, {x1, y1, z1}}, lodStructureColor(key.block, 1))
		appendLODStructureQuad(data, [4][3]float32{{x0, y0, z1}, {x0, y0, z0}, {x1, y0, z1}, {x1, y0, z0}}, lodStructureColor(key.block, .65))
		appendLODStructureQuad(data, [4][3]float32{{x0, y1, z1}, {x0, y1, z0}, {x0, y0, z1}, {x0, y0, z0}}, lodStructureColor(key.block, .78))
		appendLODStructureQuad(data, [4][3]float32{{x1, y1, z0}, {x1, y1, z1}, {x1, y0, z0}, {x1, y0, z1}}, lodStructureColor(key.block, .78))
		appendLODStructureQuad(data, [4][3]float32{{x0, y1, z0}, {x1, y1, z0}, {x0, y0, z0}, {x1, y0, z0}}, lodStructureColor(key.block, .88))
		appendLODStructureQuad(data, [4][3]float32{{x1, y1, z1}, {x0, y1, z1}, {x1, y0, z1}, {x0, y0, z1}}, lodStructureColor(key.block, .88))
	}
}
