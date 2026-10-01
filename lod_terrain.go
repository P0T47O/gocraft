package main

import "gocraft/platform"

// A tile spans eight chunks. Its grid has 8, 16, or 32 cells per side;
// distant geometry stays bounded independently of the full-chunk horizon.
const lodTileSize = 128
const lodCellSize = 8
const lodCellsPerTile = lodTileSize / lodCellSize
const lodNearCellSize = 2
const lodTransitionCellSize = 4
const lodFarCellSize = 16
const lodBoundaryCellSize = lodFarCellSize

type lodTileKey struct{ X, Z int }

type lodTileData struct {
	vertices      []platform.Vertex
	indices       []uint32
	waterVertices []platform.Vertex
	waterIndices  []uint32
}

func lodSurfaceColor(block byte) [4]uint8 {
	if block != blockGrass {
		if palette := activeLODMaterialPalette.Load(); palette != nil && palette[block][3] != 0 {
			return palette[block]
		}
	}
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

// Match the full-detail grass top's climate tint and vanilla texture average.
// The block renderer multiplies the grass texture by climateColor at each top
// vertex; using a fixed green here made the full-chunk boundary a color ring.
func lodSurfaceColorAt(seed uint32, x, z int, block byte) [4]uint8 {
	if block != blockGrass {
		return lodSurfaceColor(block)
	}
	c := climateColor(seed, x, z)
	const grassTopAverage = float32(147.0 / 255.0)
	return [4]uint8{uint8(float32(c.R) * grassTopAverage), uint8(float32(c.G) * grassTopAverage), uint8(float32(c.B) * grassTopAverage), 255}
}

func lodWaterColor(seed uint32, x, z int) [4]uint8 {
	e := sampleEnvironment(seed, x, z)
	var r, g, b float32
	for i, weight := range e.weights {
		cr, cg, cb := biomeBaseColor(regionBiomes[i], true)
		r += cr * weight
		g += cg * weight
		b += cb * weight
	}
	// Full-detail water uses this biome tint with vertex alpha 200. Let the
	// translucent pass reveal the same terrain bed instead of baking a second,
	// opaque bed color into the LOD water.
	return [4]uint8{uint8(r), uint8(g), uint8(b), 200}
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

// Raised player edits are geometry, not terrain-height samples: interpolating
// them into the ground turns buildings into sharp hills. Digging and material
// changes at the existing surface still belong to the terrain grid.
func lodSampleColumn(seed uint32, x, z int, overrides map[lodPoint]lodColumn) lodColumn {
	// Observed ground was already clamped to the seed surface at capture time;
	// buildings are separate occupancy. Do not regenerate a known terrain column.
	if changed, ok := overrides[lodPoint{x, z}]; ok && changed.observed {
		return changed
	}
	column := sampleTerrainColumn(seed, x, z)
	if changed, ok := overrides[lodPoint{x, z}]; ok && changed.height <= column.height {
		return changed
	}
	return lodColumn{height: column.height, top: column.top}
}

// All detail levels follow the same 16-block line at tile borders. Extra
// fine-grid edge vertices lie on that line, so adjacent 4/8/16-block tiles
// cannot open a crack even when their interior triangulations differ.
func lodGridColumn(seed uint32, x, z int, edge bool, overrides map[lodPoint]lodColumn) (float32, byte, [4]uint8) {
	if !edge || (x%lodBoundaryCellSize == 0 && z%lodBoundaryCellSize == 0) {
		c := lodSampleColumn(seed, x, z, overrides)
		return float32(c.height), c.top, lodSurfaceColorAt(seed, x, z, c.top)
	}
	if x%lodTileSize == 0 {
		z0 := divFloor(z, lodBoundaryCellSize) * lodBoundaryCellSize
		a := lodSampleColumn(seed, x, z0, overrides)
		b := lodSampleColumn(seed, x, z0+lodBoundaryCellSize, overrides)
		t := float32(z-z0) / lodBoundaryCellSize
		return lerp(float32(a.height), float32(b.height), t), lodNearestTop(a.top, b.top, t), lodBlendColor(lodSurfaceColorAt(seed, x, z0, a.top), lodSurfaceColorAt(seed, x, z0+lodBoundaryCellSize, b.top), t)
	}
	x0 := divFloor(x, lodBoundaryCellSize) * lodBoundaryCellSize
	a := lodSampleColumn(seed, x0, z, overrides)
	b := lodSampleColumn(seed, x0+lodBoundaryCellSize, z, overrides)
	t := float32(x-x0) / lodBoundaryCellSize
	return lerp(float32(a.height), float32(b.height), t), lodNearestTop(a.top, b.top, t), lodBlendColor(lodSurfaceColorAt(seed, x0, z, a.top), lodSurfaceColorAt(seed, x0+lodBoundaryCellSize, z, b.top), t)
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

// The two triangles in each parent cell share a bottom-left to top-right
// diagonal. Match that plane exactly so a fine tile can morph into its parent
// level before its mesh is replaced. Parent vertices are already in heights.
func lodParentHeight(heights []float32, side, x, z int) float32 {
	px, pz := x/2*2, z/2*2
	if px+2 >= side || pz+2 >= side {
		return heights[z*side+x]
	}
	a := heights[pz*side+px]
	if x%2 == 0 && z%2 == 0 {
		return a
	}
	b := heights[pz*side+px+2]
	c := heights[(pz+2)*side+px]
	if x%2 == 0 {
		return (a + c) * .5
	}
	if z%2 == 0 {
		return (a + b) * .5
	}
	return (b + c) * .5
}

func buildLODTileAtStep(seed uint32, key lodTileKey, overrides map[lodPoint]lodColumn, step int) lodTileData {
	if step != lodNearCellSize && step != lodTransitionCellSize && step != lodCellSize && step != lodFarCellSize {
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
	waterColors := make([][4]uint8, side*side)
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for z := 0; z < side; z++ {
		for x := 0; x < side; x++ {
			wx, wz := baseX+x*step, baseZ+z*step
			height, top, color := lodGridColumn(seed, wx, wz, x == 0 || x == cells || z == 0 || z == cells, overrides)
			heights[z*side+x], tops[z*side+x] = height, top
			data.vertices = append(data.vertices, platform.Vertex{
				// Voxel blocks are centered on integer X/Z. Their footprint
				// begins half a block before the sampled column, so the LOD
				// grid must use the same edge at a real-chunk handoff.
				Position: [3]float32{float32(wx) - .5, height - .5, float32(wz) - .5},
				Color:    color,
			})
		}
	}
	if step < lodFarCellSize {
		for z := 0; z < side; z++ {
			for x := 0; x < side; x++ {
				v := &data.vertices[z*side+x]
				v.Texcoord = [2]float32{lodParentHeight(heights, side, x, z) - .5, float32(step)}
			}
		}
	}
	patches := lodObservedPatchCells(key, step, overrides)
	for z := 0; z < cells; z++ {
		for x := 0; x < cells; x++ {
			if patches[z*cells+x] {
				appendLODObservedPatch(&data, seed, key, x*step, z*step, step, side, overrides)
				continue
			}
			a := uint32(z*side + x)
			data.indices = append(data.indices, a, a+uint32(side), a+1, a+1, a+uint32(side), a+uint32(side)+1)
			// Water is a separate horizontal layer. The terrain beneath remains
			// available for shoreline interpolation, not a sloping blue quad.
			if heights[z*side+x] >= seaLevel && heights[z*side+x+1] >= seaLevel &&
				heights[(z+1)*side+x] >= seaLevel && heights[(z+1)*side+x+1] >= seaLevel {
				continue
			}
			ice := tops[z*side+x] == blockSnow
			base := uint32(len(data.waterVertices))
			wx, wz := float32(baseX+x*step)-.5, float32(baseZ+z*step)-.5
			y := float32(seaLevel) - .5
			for i, pos := range [4][3]float32{{wx, y, wz}, {wx + float32(step), y, wz}, {wx, y, wz + float32(step)}, {wx + float32(step), y, wz + float32(step)}} {
				if ice {
					data.vertices = append(data.vertices, platform.Vertex{Position: pos, Color: lodSurfaceColor(blockIce)})
				} else {
					index := (z+i/2)*side + x + i%2
					if waterColors[index][3] == 0 {
						waterColors[index] = lodWaterColor(seed, baseX+(x+i%2)*step, baseZ+(z+i/2)*step)
					}
					data.waterVertices = append(data.waterVertices, platform.Vertex{Position: pos, Texcoord: [2]float32{float32(i % 2), float32(i / 2)}, Color: waterColors[index]})
				}
			}
			if ice {
				base = uint32(len(data.vertices) - 4)
				data.indices = append(data.indices, base, base+2, base+1, base+1, base+2, base+3)
			} else {
				data.waterIndices = append(data.waterIndices, base, base+2, base+1, base+1, base+2, base+3)
			}
		}
	}
	appendLODChunkBoundaryStrips(&data, seed, key, overrides, step, side)
	appendLODRaisedColumns(&data, seed, key, overrides)
	// The actual generator's random draw must be below its maximum chance.
	// Hash-filter first; only rare candidate coordinates pay for a terrain
	// sample. Every emitted silhouette now has a matching full-detail tree.
	for z := baseZ; z < baseZ+lodTileSize; z++ {
		for x := baseX; x < baseX+lodTileSize; x++ {
			if (hash2(seed+1, x, z)+1)*.5 >= maxTreeAnchorChance {
				continue
			}
			if overrides[lodPoint{x, z}].observed {
				continue
			}
			if anchor, ok := sampleTreeAnchor(seed, x, z); ok {
				appendLODTree(&data, anchor)
			}
		}
	}
	return data
}

// Interpolate the ordinary grid along a real-chunk edge at voxel spacing.
// The handoff may use 2/4-block cells, but the real silhouette changes at
// every block; coarse edge vertices can leave sky-colored pinholes.
func lodGridVertexAt(data *lodTileData, side, step, x, z int) platform.Vertex {
	x0, z0 := x/step, z/step
	x1, z1 := min(x0+1, side-1), min(z0+1, side-1)
	tx, tz := float32(x%step)/float32(step), float32(z%step)/float32(step)
	a, b := data.vertices[z0*side+x0], data.vertices[z0*side+x1]
	c, d := data.vertices[z1*side+x0], data.vertices[z1*side+x1]
	out := a
	out.Position[0] = lerp(a.Position[0], b.Position[0], tx)
	out.Position[2] = lerp(a.Position[2], c.Position[2], tz)
	out.Position[1] = lerp(lerp(a.Position[1], b.Position[1], tx), lerp(c.Position[1], d.Position[1], tx), tz)
	out.Color = lodBlendColor(lodBlendColor(a.Color, b.Color, tx), lodBlendColor(c.Color, d.Color, tx), tz)
	return out
}

// An eight-block transition joins the last real voxel column to the ordinary
// LOD heightfield. The shader swaps it in only at a live handoff.
func appendLODChunkBoundaryStrips(data *lodTileData, seed uint32, key lodTileKey, overrides map[lodPoint]lodColumn, step, side int) {
	// Coarser tiles never touch a complete chunk at the normal full/LOD
	// radius. Avoid carrying unused seam geometry all the way to the horizon.
	if step > lodTransitionCellSize {
		return
	}
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	width := max(8, step)
	for x := 0; x+width <= lodTileSize; x += chunkWidth {
		for z := 0; z < lodTileSize; z++ {
			wx, wz := baseX+x, baseZ+z
			if !overrides[lodPoint{wx - 1, wz}].observed {
				continue
			}
			inner0 := lodSampleColumn(seed, wx-1, wz, overrides)
			inner1 := lodSampleColumn(seed, wx-1, wz+1, overrides)
			diff0 := float32(inner0.height) - (lodGridVertexAt(data, side, step, x, z).Position[1] + .5)
			diff1 := float32(inner1.height) - (lodGridVertexAt(data, side, step, x, z+1).Position[1] + .5)
			near0 := lodSurfaceColorAt(seed, wx-1, wz, inner0.top)
			near1 := lodSurfaceColorAt(seed, wx-1, wz+1, inner1.top)
			for offset := 0; offset < width; offset += step {
				t0, t1 := float32(offset)/float32(width), float32(offset+step)/float32(width)
				base := uint32(len(data.vertices))
				for i, grid := range [4][2]int{{x + offset, z}, {x + offset, z + 1}, {x + offset + step, z}, {x + offset + step, z + 1}} {
					t, diff, near := t0, diff0, near0
					if i >= 2 {
						t = t1
					}
					if i%2 == 1 {
						diff, near = diff1, near1
					}
					original := lodGridVertexAt(data, side, step, grid[0], grid[1])
					original.Position[1] += diff * (1 - t)
					original.Color = lodBlendColor(near, original.Color, t)
					if offset == 0 && i < 2 {
						original.Position[0] -= .35
						original.Position[1] -= .02
					}
					original.Texcoord = [2]float32{0, -1}
					data.vertices = append(data.vertices, original)
				}
				data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
			}
		}
	}
	for z := 0; z+width <= lodTileSize; z += chunkWidth {
		for x := 0; x < lodTileSize; x++ {
			wx, wz := baseX+x, baseZ+z
			if !overrides[lodPoint{wx, wz - 1}].observed {
				continue
			}
			inner0 := lodSampleColumn(seed, wx, wz-1, overrides)
			inner1 := lodSampleColumn(seed, wx+1, wz-1, overrides)
			diff0 := float32(inner0.height) - (lodGridVertexAt(data, side, step, x, z).Position[1] + .5)
			diff1 := float32(inner1.height) - (lodGridVertexAt(data, side, step, x+1, z).Position[1] + .5)
			near0 := lodSurfaceColorAt(seed, wx, wz-1, inner0.top)
			near1 := lodSurfaceColorAt(seed, wx+1, wz-1, inner1.top)
			for offset := 0; offset < width; offset += step {
				t0, t1 := float32(offset)/float32(width), float32(offset+step)/float32(width)
				base := uint32(len(data.vertices))
				for i, grid := range [4][2]int{{x, z + offset}, {x, z + offset + step}, {x + 1, z + offset}, {x + 1, z + offset + step}} {
					t, diff, near := t0, diff0, near0
					if i%2 == 1 {
						t = t1
					}
					if i >= 2 {
						diff, near = diff1, near1
					}
					original := lodGridVertexAt(data, side, step, grid[0], grid[1])
					original.Position[1] += diff * (1 - t)
					original.Color = lodBlendColor(near, original.Color, t)
					if offset == 0 && i%2 == 0 {
						original.Position[2] -= .35
						original.Position[1] -= .02
					}
					original.Texcoord = [2]float32{0, -2}
					data.vertices = append(data.vertices, original)
				}
				data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
			}
		}
	}
	// The first two loops join an outside cell east/south of a real chunk.
	// Mirror them for a real chunk east/south of the outside cell. The inner
	// edge samples the first real column, which is on the retained 8-block grid.
	for boundary := chunkWidth; boundary <= lodTileSize; boundary += chunkWidth {
		wx := baseX + boundary
		for z := 0; z < lodTileSize; z++ {
			wz := baseZ + z
			if !overrides[lodPoint{wx, wz}].observed {
				continue
			}
			inner0 := lodSampleColumn(seed, wx, wz, overrides)
			inner1 := lodSampleColumn(seed, wx, wz+1, overrides)
			diff0 := float32(inner0.height) - (lodGridVertexAt(data, side, step, boundary, z).Position[1] + .5)
			diff1 := float32(inner1.height) - (lodGridVertexAt(data, side, step, boundary, z+1).Position[1] + .5)
			near0 := lodSurfaceColorAt(seed, wx, wz, inner0.top)
			near1 := lodSurfaceColorAt(seed, wx, wz+1, inner1.top)
			for offset := 0; offset < width; offset += step {
				col := boundary - width + offset
				t0, t1 := float32(width-offset)/float32(width), float32(width-offset-step)/float32(width)
				base := uint32(len(data.vertices))
				for i, grid := range [4][2]int{{col, z}, {col, z + 1}, {col + step, z}, {col + step, z + 1}} {
					t, diff, near := t0, diff0, near0
					if i >= 2 {
						t = t1
					}
					if i%2 == 1 {
						diff, near = diff1, near1
					}
					v := lodGridVertexAt(data, side, step, grid[0], grid[1])
					v.Position[1] += diff * (1 - t)
					v.Color = lodBlendColor(near, v.Color, t)
					if offset+step == width && i >= 2 {
						v.Position[0] += .35
						v.Position[1] -= .02
					}
					v.Texcoord = [2]float32{0, -3}
					data.vertices = append(data.vertices, v)
				}
				data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
			}
		}
		wz := baseZ + boundary
		for x := 0; x < lodTileSize; x++ {
			wx := baseX + x
			if !overrides[lodPoint{wx, wz}].observed {
				continue
			}
			inner0 := lodSampleColumn(seed, wx, wz, overrides)
			inner1 := lodSampleColumn(seed, wx+1, wz, overrides)
			diff0 := float32(inner0.height) - (lodGridVertexAt(data, side, step, x, boundary).Position[1] + .5)
			diff1 := float32(inner1.height) - (lodGridVertexAt(data, side, step, x+1, boundary).Position[1] + .5)
			near0 := lodSurfaceColorAt(seed, wx, wz, inner0.top)
			near1 := lodSurfaceColorAt(seed, wx+1, wz, inner1.top)
			for offset := 0; offset < width; offset += step {
				row := boundary - width + offset
				t0, t1 := float32(width-offset)/float32(width), float32(width-offset-step)/float32(width)
				base := uint32(len(data.vertices))
				for i, grid := range [4][2]int{{x, row}, {x, row + step}, {x + 1, row}, {x + 1, row + step}} {
					t, diff, near := t0, diff0, near0
					if i%2 == 1 {
						t = t1
					}
					if i >= 2 {
						diff, near = diff1, near1
					}
					v := lodGridVertexAt(data, side, step, grid[0], grid[1])
					v.Position[1] += diff * (1 - t)
					v.Color = lodBlendColor(near, v.Color, t)
					if offset+step == width && i%2 == 1 {
						v.Position[2] += .35
						v.Position[1] -= .02
					}
					v.Texcoord = [2]float32{0, -4}
					data.vertices = append(data.vertices, v)
				}
				data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
			}
		}
	}
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
