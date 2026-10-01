package main

import "gocraft/platform"

func lodObservedPatchCells(key lodTileKey, step int, columns map[lodPoint]lodColumn) []bool {
	cells := lodTileSize / step
	result := make([]bool, cells*cells)
	// Mark cells from edits once, including the shared-vertex margin. Unedited
	// terrain no longer searches every column in every coarse cell.
	for p, c := range columns {
		if !c.observed || !c.changed {
			continue
		}
		x, z := p.X-key.X*lodTileSize, p.Z-key.Z*lodTileSize
		for cz := max(0, divFloor(z-2, step)); cz <= min(cells-1, divFloor(z+1, step)); cz++ {
			for cx := max(0, divFloor(x-2, step)); cx <= min(cells-1, divFloor(x+1, step)); cx++ {
				result[cz*cells+cx] = true
			}
		}
	}
	return result
}

func appendLODObservedPatch(data *lodTileData, seed uint32, key lodTileKey, x, z, step, side int, columns map[lodPoint]lodColumn) {
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for dz := 0; dz < step; dz++ {
		for dx := 0; dx < step; dx++ {
			base := uint32(len(data.vertices))
			for _, offset := range [4][2]int{{dx, dz}, {dx, dz + 1}, {dx + 1, dz}, {dx + 1, dz + 1}} {
				gx, gz := x+offset[0], z+offset[1]
				wx, wz := baseX+gx, baseZ+gz
				v := lodGridVertexAt(data, side, step, gx, gz)
				if c := columns[lodPoint{wx, wz}]; c.observed && c.changed {
					v.Position[1] = float32(c.height) - .5
					v.Color = lodSurfaceColorAt(seed, wx, wz, c.top)
					v.Texcoord = [2]float32{} // Real edits must not morph back to seed.
				} else {
					// The patch's unedited perimeter follows the ordinary grid and
					// its parent morph, preserving the shared outer edge.
					v.Texcoord = lodPatchMorph(data, side, step, gx, gz)
				}
				data.vertices = append(data.vertices, v)
			}
			data.indices = append(data.indices, base, base+1, base+2, base+2, base+1, base+3)
			wx, wz := baseX+x+dx, baseZ+z+dz
			c := columns[lodPoint{wx, wz}]
			// Preserve the procedural sea layer for excavated ocean beds. Empty
			// inland holes do not manufacture sea-level water.
			natural := sampleTerrainColumn(seed, wx, wz)
			if natural.height < seaLevel && (!c.observed || c.height < seaLevel) {
				water := uint32(len(data.waterVertices))
				ice := natural.blockAt(seed, wx, seaLevel-1, wz) == blockIce
				if ice {
					water = uint32(len(data.vertices))
				}
				y := float32(seaLevel) - .5
				for i, p := range [4][3]float32{{float32(wx) - .5, y, float32(wz) - .5}, {float32(wx) + .5, y, float32(wz) - .5}, {float32(wx) - .5, y, float32(wz) + .5}, {float32(wx) + .5, y, float32(wz) + .5}} {
					if ice {
						data.vertices = append(data.vertices, platform.Vertex{Position: p, Color: lodSurfaceColor(blockIce)})
					} else {
						data.waterVertices = append(data.waterVertices, platform.Vertex{Position: p, Texcoord: [2]float32{float32(i % 2), float32(i / 2)}, Color: lodWaterColor(seed, wx, wz)})
					}
				}
				if ice {
					data.indices = append(data.indices, water, water+2, water+1, water+1, water+2, water+3)
				} else {
					data.waterIndices = append(data.waterIndices, water, water+2, water+1, water+1, water+2, water+3)
				}
			}
		}
	}
}

func lodPatchMorph(data *lodTileData, side, step, x, z int) [2]float32 {
	x0, z0 := x/step, z/step
	x1, z1 := min(x0+1, side-1), min(z0+1, side-1)
	tx, tz := float32(x%step)/float32(step), float32(z%step)/float32(step)
	a, b := data.vertices[z0*side+x0], data.vertices[z0*side+x1]
	c, d := data.vertices[z1*side+x0], data.vertices[z1*side+x1]
	return [2]float32{lerp(lerp(a.Texcoord[0], b.Texcoord[0], tx), lerp(c.Texcoord[0], d.Texcoord[0], tx), tz), a.Texcoord[1]}
}
