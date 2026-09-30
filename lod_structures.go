package main

import "gocraft/platform"

// Structures stay separate from the sampled landscape. Live received chunks
// supply voxel occupancy runs; older top-only summaries retain the 8×8 prism
// fallback used by diagnostic fixtures.
func appendLODRaisedColumns(data *lodTileData, seed uint32, key lodTileKey, overrides map[lodPoint]lodColumn) {
	if len(overrides) == 0 {
		return
	}
	appendLODOccupancy(data, key, overrides)
	// Adjacent prisms repeatedly inspect the same ground and neighbor columns.
	// Cache just those touched by this sparse tile instead of resampling the
	// deterministic terrain for every one of the four wall decisions.
	type prismColumn struct{ ground, height int }
	cache := make(map[lodPoint]prismColumn, len(overrides))
	columnAt := func(x, z int) prismColumn {
		point := lodPoint{x, z}
		if column, ok := cache[point]; ok {
			return column
		}
		ground := sampleTerrainColumn(seed, x, z).height
		height := ground
		if changed, ok := overrides[point]; ok {
			height = changed.height
		}
		column := prismColumn{ground, height}
		cache[point] = column
		return column
	}
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for z := baseZ; z < baseZ+lodTileSize; z += lodCellSize {
		for x := baseX; x < baseX+lodTileSize; x += lodCellSize {
			column, ok := overrides[lodPoint{x, z}]
			if !ok || column.occupancy != nil {
				continue
			}
			base := columnAt(x, z).ground
			if column.height <= base {
				continue
			}
			top := float32(column.height) - .65
			x0, x1 := float32(x), float32(x+lodCellSize)
			z0, z1 := float32(z), float32(z+lodCellSize)
			color := lodStructureColor(column.top, 1)
			appendLODStructureQuad(data, [4][3]float32{{x0, top, z0}, {x0, top, z1}, {x1, top, z0}, {x1, top, z1}}, color)

			wall := func(nx, nz int, points [2][2]float32, shade float32) {
				bottom := max(base, columnAt(nx, nz).height)
				if bottom >= column.height {
					return
				}
				low := float32(bottom) - .65
				appendLODStructureQuad(data, [4][3]float32{
					{points[0][0], top, points[0][1]},
					{points[1][0], top, points[1][1]},
					{points[0][0], low, points[0][1]},
					{points[1][0], low, points[1][1]},
				}, lodStructureColor(column.top, shade))
			}
			// Endpoint order gives outward-facing CCW triangles on all four sides.
			wall(x-lodCellSize, z, [2][2]float32{{x0, z1}, {x0, z0}}, .78)
			wall(x+lodCellSize, z, [2][2]float32{{x1, z0}, {x1, z1}}, .78)
			wall(x, z-lodCellSize, [2][2]float32{{x0, z0}, {x1, z0}}, .88)
			wall(x, z+lodCellSize, [2][2]float32{{x1, z1}, {x0, z1}}, .88)
		}
	}
}

func lodStructureColor(block byte, shade float32) [4]uint8 {
	color := lodSurfaceColor(block)
	for i := 0; i < 3; i++ {
		color[i] = uint8(float32(color[i]) * shade)
	}
	color[3] = 253 // Distinct from opaque terrain (255) and tree silhouettes (254).
	return color
}

func appendLODStructureQuad(data *lodTileData, points [4][3]float32, color [4]uint8) {
	base := uint32(len(data.vertices))
	for _, point := range points {
		data.vertices = append(data.vertices, platform.Vertex{Position: point, Color: color})
	}
	data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
}
