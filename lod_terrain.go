package main

import "gocraft/platform"

// A tile spans eight chunks. Its grid has 8, 16, or 32 cells per side;
// distant geometry stays bounded independently of the full-chunk horizon.
const lodTileSize = 128
const lodCellSize = 8
const lodCellsPerTile = lodTileSize / lodCellSize
const lodNearCellSize = 4
const lodFarCellSize = 16
const lodBoundaryCellSize = lodFarCellSize

type lodTileKey struct{ X, Z int }

type lodTileData struct {
	vertices []platform.Vertex
	indices  []uint32
}

func lodSurfaceColor(block byte) [4]uint8 {
	switch block {
	case blockGrass:
		return [4]uint8{83, 135, 63, 255}
	case blockSand, blockSandstone:
		return [4]uint8{211, 198, 142, 255}
	case blockSnow:
		return [4]uint8{226, 232, 228, 255}
	case blockStone, blockCobblestone, blockStoneSlab, blockCobbleStairs:
		return [4]uint8{118, 120, 118, 255}
	case blockPlank, blockPlankOak, blockPlankBirch, blockPlankSpruce, blockOakSlab, blockOakStairs:
		return [4]uint8{150, 116, 75, 255}
	case blockGlass, blockIronBlock, blockWhiteWool:
		return [4]uint8{209, 215, 212, 255}
	case blockObsidian:
		return [4]uint8{43, 37, 57, 255}
	case blockGoldBlock, blockGlowstone:
		return [4]uint8{225, 183, 74, 255}
	case blockDiamondBlock:
		return [4]uint8{80, 205, 206, 255}
	case blockGravel:
		return [4]uint8{135, 130, 126, 255}
	case blockIce:
		return [4]uint8{166, 197, 213, 255}
	default:
		return [4]uint8{116, 104, 81, 255}
	}
}

// buildLODTile reads only deterministic terrain columns. It never allocates
// full chunks, samples caves, or touches GPU/world state, so workers can build
// these meshes without competing with the client chunk owner.
func buildLODTile(seed uint32, key lodTileKey) lodTileData {
	return buildLODTileWithColumns(seed, key, nil)
}

func buildLODTileWithColumns(seed uint32, key lodTileKey, overrides map[lodPoint]lodColumn) lodTileData {
	return buildLODTileAtStep(seed, key, overrides, lodCellSize)
}

func lodSampleColumn(seed uint32, x, z int, overrides map[lodPoint]lodColumn) lodColumn {
	if changed, ok := overrides[lodPoint{x, z}]; ok {
		return changed
	}
	column := sampleTerrainColumn(seed, x, z)
	return lodColumn{column.height, column.top}
}

// All detail levels follow the same 16-block line at tile borders. Extra
// fine-grid edge vertices lie on that line, so adjacent 4/8/16-block tiles
// cannot open a crack even when their interior triangulations differ.
func lodGridColumn(seed uint32, x, z int, edge bool, overrides map[lodPoint]lodColumn) (float32, byte, [4]uint8) {
	if !edge || (x%lodBoundaryCellSize == 0 && z%lodBoundaryCellSize == 0) {
		c := lodSampleColumn(seed, x, z, overrides)
		return float32(c.height), c.top, lodSurfaceColor(c.top)
	}
	if x%lodTileSize == 0 {
		z0 := divFloor(z, lodBoundaryCellSize) * lodBoundaryCellSize
		a := lodSampleColumn(seed, x, z0, overrides)
		b := lodSampleColumn(seed, x, z0+lodBoundaryCellSize, overrides)
		t := float32(z-z0) / lodBoundaryCellSize
		return lerp(float32(a.height), float32(b.height), t), lodNearestTop(a.top, b.top, t), lodBlendColor(lodSurfaceColor(a.top), lodSurfaceColor(b.top), t)
	}
	x0 := divFloor(x, lodBoundaryCellSize) * lodBoundaryCellSize
	a := lodSampleColumn(seed, x0, z, overrides)
	b := lodSampleColumn(seed, x0+lodBoundaryCellSize, z, overrides)
	t := float32(x-x0) / lodBoundaryCellSize
	return lerp(float32(a.height), float32(b.height), t), lodNearestTop(a.top, b.top, t), lodBlendColor(lodSurfaceColor(a.top), lodSurfaceColor(b.top), t)
}

func lodNearestTop(a, b byte, t float32) byte {
	if t < .5 {
		return a
	}
	return b
}

func lodBlendColor(a, b [4]uint8, t float32) [4]uint8 {
	return [4]uint8{
		uint8(lerp(float32(a[0]), float32(b[0]), t)),
		uint8(lerp(float32(a[1]), float32(b[1]), t)),
		uint8(lerp(float32(a[2]), float32(b[2]), t)), 255,
	}
}

func buildLODTileAtStep(seed uint32, key lodTileKey, overrides map[lodPoint]lodColumn, step int) lodTileData {
	if step != lodNearCellSize && step != lodCellSize && step != lodFarCellSize {
		panic("invalid distant terrain cell size")
	}
	cells := lodTileSize / step
	side := cells + 1
	data := lodTileData{
		vertices: make([]platform.Vertex, 0, side*side+cells*cells*4),
		indices:  make([]uint32, 0, cells*cells*12),
	}
	heights := make([]float32, side*side)
	tops := make([]byte, side*side)
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for z := 0; z < side; z++ {
		for x := 0; x < side; x++ {
			wx, wz := baseX+x*step, baseZ+z*step
			height, top, color := lodGridColumn(seed, wx, wz, x == 0 || x == cells || z == 0 || z == cells, overrides)
			heights[z*side+x], tops[z*side+x] = height, top
			data.vertices = append(data.vertices, platform.Vertex{
				Position: [3]float32{float32(wx), height - .65, float32(wz)},
				Color:    color,
			})
		}
	}
	for z := 0; z < cells; z++ {
		for x := 0; x < cells; x++ {
			a := uint32(z*side + x)
			data.indices = append(data.indices, a, a+uint32(side), a+1, a+1, a+uint32(side), a+uint32(side)+1)
			// Water is a separate horizontal layer. The terrain beneath remains
			// available for shoreline interpolation, not a sloping blue quad.
			if heights[z*side+x] >= seaLevel && heights[z*side+x+1] >= seaLevel &&
				heights[(z+1)*side+x] >= seaLevel && heights[(z+1)*side+x+1] >= seaLevel {
				continue
			}
			color := [4]uint8{65, 104, 157, 255}
			if tops[z*side+x] == blockSnow {
				color = lodSurfaceColor(blockIce)
			}
			base := uint32(len(data.vertices))
			wx, wz := float32(baseX+x*step), float32(baseZ+z*step)
			y := float32(seaLevel) - .6
			for _, pos := range [4][3]float32{{wx, y, wz}, {wx + float32(step), y, wz}, {wx, y, wz + float32(step)}, {wx + float32(step), y, wz + float32(step)}} {
				data.vertices = append(data.vertices, platform.Vertex{Position: pos, Color: color})
			}
			data.indices = append(data.indices, base, base+2, base+1, base+1, base+2, base+3)
		}
	}
	// The actual generator's random draw must be below its maximum chance.
	// Hash-filter first; only rare candidate coordinates pay for a terrain
	// sample. Every emitted silhouette now has a matching full-detail tree.
	for z := baseZ; z < baseZ+lodTileSize; z++ {
		for x := baseX; x < baseX+lodTileSize; x++ {
			if (hash2(seed+1, x, z)+1)*.5 >= maxTreeAnchorChance {
				continue
			}
			if anchor, ok := sampleTreeAnchor(seed, x, z); ok {
				appendLODTree(&data, anchor)
			}
		}
	}
	return data
}

func appendLODTree(data *lodTileData, anchor treeAnchor) {
	height := float32(anchor.height)
	// Compact solid silhouettes remain visible after a tree has shrunk below
	// a texel. Deliberately omit individual leaves, branches and atlas reads.
	centerX, centerZ := float32(anchor.x), float32(anchor.z)
	ground := float32(anchor.y) - .5
	trunkBase := uint32(len(data.vertices))
	trunk := [4]uint8{94, 72, 48, 254}
	if anchor.log == blockLogBirch {
		trunk = [4]uint8{192, 188, 174, 254}
	}
	for _, y := range [2]float32{ground, ground + height*.52} {
		for _, p := range [4][2]float32{{-.35, -.35}, {.35, -.35}, {-.35, .35}, {.35, .35}} {
			data.vertices = append(data.vertices, platform.Vertex{Position: [3]float32{centerX + p[0], y, centerZ + p[1]}, Color: trunk})
		}
	}
	for _, face := range [][3]uint32{{0, 4, 2}, {2, 4, 6}, {1, 3, 5}, {3, 7, 5}, {0, 1, 4}, {1, 5, 4}, {2, 6, 3}, {3, 6, 7}} {
		data.indices = append(data.indices, trunkBase+face[0], trunkBase+face[1], trunkBase+face[2])
	}
	base := uint32(len(data.vertices))
	radius := float32(2.4)
	if anchor.spruce {
		radius = 2
	}
	// Alpha 254 marks LOD-only vegetation for the near-field shader mask.
	leaf := [4]uint8{48, 91, 45, 254}
	if anchor.spruce {
		leaf = [4]uint8{40, 72, 57, 254}
	} else if anchor.leaves == blockLeavesBirch {
		leaf = [4]uint8{66, 105, 45, 254}
	}
	// Two stacked square rings plus an apex form a low-cost, tapered crown.
	for ring, y := range []float32{ground + height*.45, ground + height*.83} {
		r := radius
		if ring == 1 {
			r *= .7
		}
		for _, p := range [4][2]float32{{-r, -r}, {r, -r}, {-r, r}, {r, r}} {
			data.vertices = append(data.vertices, platform.Vertex{Position: [3]float32{centerX + p[0], y, centerZ + p[1]}, Color: leaf})
		}
	}
	data.vertices = append(data.vertices, platform.Vertex{Position: [3]float32{centerX, ground + height + 1, centerZ}, Color: leaf})
	// Both sides are visible from above, including the crown's underside.
	for _, face := range [][3]uint32{{0, 2, 4}, {2, 6, 4}, {1, 5, 3}, {3, 5, 7}, {0, 4, 1}, {1, 4, 5}, {2, 3, 6}, {3, 7, 6}, {4, 6, 8}, {6, 7, 8}, {7, 5, 8}, {5, 4, 8}} {
		data.indices = append(data.indices, base+face[0], base+face[1], base+face[2])
	}
}
