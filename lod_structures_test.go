package main

import "testing"

// These are deliberately large, contrasting player-built shapes, not new
// procedural terrain. Feed their top blocks through the same sparse column
// capture used when a modified chunk arrives from the server.
func lodStructureFixture(seed uint32) *World {
	w := &World{seed: seed, IsClient: true}
	chunks := make(map[chunkKey]*Chunk)
	absInt := func(v int) int {
		if v < 0 {
			return -v
		}
		return v
	}
	place := func(x, z, height int, block byte) {
		key := chunkKey{divFloor(x, chunkWidth), divFloor(z, chunkWidth)}
		chunk := chunks[key]
		if chunk == nil {
			chunk = &Chunk{}
			chunks[key] = chunk
		}
		lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
		chunk.blocks.Set(lx, height-1, lz, block)
		chunk.heightMap[lx][lz] = int16(height)
		w.recordLODColumn(chunk, x, z)
	}
	for z := 984; z <= 1064; z += lodCellSize {
		for x := -168; x <= 168; x += lodCellSize {
			dz := absInt(z - 1024)
			switch {
			case absInt(x+128) <= 32 && dz <= 32:
				place(x, z, 190, blockIronBlock) // 64×64×roughly 120 cuboid.
			case absInt(x) <= 32 && dz <= 32:
				level := max(absInt(x), dz)
				place(x, z, 130+60*(32-level)/32, blockGoldBlock)
			case (x-128)*(x-128)+dz*dz >= 16*16 && (x-128)*(x-128)+dz*dz <= 36*36:
				place(x, z, 180, blockObsidian) // Horizontal hollow ring.
			}
		}
	}
	return w
}

func TestLODArtificialStructuresAtAllLevels(t *testing.T) {
	const seed = uint32(1234511)
	w := lodStructureFixture(seed)
	if len(w.lodColumns) < 100 {
		t.Fatalf("only %d authoritative structure samples were captured", len(w.lodColumns))
	}
	for _, step := range []int{lodNearCellSize, lodCellSize, lodFarCellSize} {
		unseenKey := lodTileKey{-1, 8}
		unseen := buildLODTileAtStep(seed, unseenKey, nil, step)
		if v := unseen.vertices[0]; v.Position[1] != float32(sampleTerrainColumn(seed, -128, 1024).height)-.65 {
			t.Fatalf("unvisited structure unexpectedly appeared at cell size %d", step)
		}
		for _, sample := range []struct {
			name   string
			x, z   int
			height int
			block  byte
		}{
			{"cuboid", -128, 1024, 190, blockIronBlock},
			{"pyramid", 0, 1024, 190, blockGoldBlock},
			{"ring rim", 160, 1024, 180, blockObsidian},
		} {
			key := lodTileKey{divFloor(sample.x, lodTileSize), divFloor(sample.z, lodTileSize)}
			tile := buildLODTileAtStep(seed, key, w.snapshotLODTile(key), step)
			side := lodTileSize/step + 1
			lx, lz := modFloor(sample.x, lodTileSize)/step, modFloor(sample.z, lodTileSize)/step
			v := tile.vertices[lz*side+lx]
			if v.Position[1] != float32(sample.height)-.65 || v.Color != lodSurfaceColor(sample.block) {
				t.Fatalf("%s at cell size %d: %+v", sample.name, step, v)
			}
		}
		// The empty center is essential to a ring; filling it would turn the
		// test fixture into a disk. A heightfield can keep this ground hole,
		// but it cannot represent a vertical ring's through-hole.
		if _, filled := w.lodColumns[lodPoint{128, 1024}]; filled {
			t.Fatalf("ring center filled at cell size %d", step)
		}
	}
}
