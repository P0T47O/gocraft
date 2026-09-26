package main

import "gocraft/platform"

// A tile spans eight chunks. Sixteen cells per side keep distant geometry
// bounded independently of the number of full-detail chunks in the horizon.
const lodTileSize = 128
const lodCellSize = 8
const lodCellsPerTile = lodTileSize / lodCellSize

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
	case blockStone:
		return [4]uint8{118, 120, 118, 255}
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
	const side = lodCellsPerTile + 1
	data := lodTileData{
		vertices: make([]platform.Vertex, 0, side*side+lodCellsPerTile*lodCellsPerTile*4),
		indices:  make([]uint32, 0, lodCellsPerTile*lodCellsPerTile*12),
	}
	var heights [side][side]int
	var tops [side][side]byte
	baseX, baseZ := key.X*lodTileSize, key.Z*lodTileSize
	for z := 0; z < side; z++ {
		for x := 0; x < side; x++ {
			wx, wz := baseX+x*lodCellSize, baseZ+z*lodCellSize
			column := sampleTerrainColumn(seed, wx, wz)
			heights[z][x], tops[z][x] = column.height, column.top
			data.vertices = append(data.vertices, platform.Vertex{
				Position: [3]float32{float32(wx), float32(column.height) - .65, float32(wz)},
				Color:    lodSurfaceColor(column.top),
			})
		}
	}
	for z := 0; z < lodCellsPerTile; z++ {
		for x := 0; x < lodCellsPerTile; x++ {
			a := uint32(z*side + x)
			data.indices = append(data.indices, a, a+uint32(side), a+1, a+1, a+uint32(side), a+uint32(side)+1)
			// Water is a separate horizontal layer. The terrain beneath remains
			// available for shoreline interpolation, not a sloping blue quad.
			if heights[z][x] >= seaLevel && heights[z][x+1] >= seaLevel &&
				heights[z+1][x] >= seaLevel && heights[z+1][x+1] >= seaLevel {
				continue
			}
			color := [4]uint8{65, 104, 157, 255}
			if tops[z][x] == blockSnow {
				color = lodSurfaceColor(blockIce)
			}
			base := uint32(len(data.vertices))
			wx, wz := float32(baseX+x*lodCellSize), float32(baseZ+z*lodCellSize)
			y := float32(seaLevel) - .6
			for _, pos := range [4][3]float32{{wx, y, wz}, {wx + lodCellSize, y, wz}, {wx, y, wz + lodCellSize}, {wx + lodCellSize, y, wz + lodCellSize}} {
				data.vertices = append(data.vertices, platform.Vertex{Position: pos, Color: color})
			}
			data.indices = append(data.indices, base, base+2, base+1, base+1, base+2, base+3)
		}
	}
	return data
}
