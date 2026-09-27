package main

import (
	"strconv"
	"testing"
)

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
			if v.Position[1] >= float32(sample.height)-.65 || v.Color == lodStructureColor(sample.block, 1) {
				t.Fatalf("%s at cell size %d distorted the landscape: %+v", sample.name, step, v)
			}
			found := false
			for _, cap := range tile.vertices {
				if cap.Position == ([3]float32{float32(sample.x), float32(sample.height) - .65, float32(sample.z)}) && cap.Color == lodStructureColor(sample.block, 1) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s at cell size %d has no separate top face", sample.name, step)
			}
		}
		// The empty center is essential to a horizontal ring. A separate
		// structure mesh can give it an inner wall, although a vertical ring's
		// suspended opening still needs richer occupancy data.
		if _, filled := w.lodColumns[lodPoint{128, 1024}]; filled {
			t.Fatalf("ring center filled at cell size %d", step)
		}
	}
	ringKey := lodTileKey{0, 8}
	ring := buildLODTileAtStep(seed, ringKey, w.snapshotLODTile(ringKey), lodCellSize)
	innerWall := false
	for _, v := range ring.vertices {
		if v.Position[0] == 120 && v.Position[2] == 1024 && v.Position[1] < 179 && v.Color == lodStructureColor(blockObsidian, .78) {
			innerWall = true
			break
		}
	}
	if !innerWall {
		t.Fatal("horizontal ring lost its inner vertical wall")
	}
	cuboidKey := lodTileKey{-1, 8}
	cuboid := buildLODTileAtStep(seed, cuboidKey, w.snapshotLODTile(cuboidKey), lodCellSize)
	for _, v := range cuboid.vertices {
		if v.Position[0] == -128 && v.Position[2] > 992 && v.Position[2] < 1056 && v.Color == lodStructureColor(blockIronBlock, .78) {
			t.Fatal("tile boundary created an internal cuboid wall")
		}
	}
}

func BenchmarkLODStructureTile(b *testing.B) {
	const seed = uint32(1234511)
	w := lodStructureFixture(seed)
	key := lodTileKey{0, 8} // Pyramid and half the ring.
	overrides := w.snapshotLODTile(key)
	for _, step := range []int{lodNearCellSize, lodCellSize, lodFarCellSize} {
		b.Run(strconv.Itoa(step), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				buildLODTileAtStep(seed, key, overrides, step)
			}
		})
	}
}
