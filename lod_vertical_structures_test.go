package main

import "testing"

// Build actual voxel shapes in temporary chunk objects, then pass their
// captured columns through the same sparse LOD path as received client chunks.
// No world save is touched. The vertical ring lies in the X/Y plane; the
// platform has no supports and empty air beneath its entire footprint.
func lodVerticalStructureFixture(seed uint32) (*World, map[chunkKey]*Chunk) {
	w := &World{seed: seed, IsClient: true}
	chunks := make(map[chunkKey]*Chunk)
	samples := make(map[lodPoint]bool)
	place := func(x, y, z int, block byte) {
		key := chunkKey{divFloor(x, chunkWidth), divFloor(z, chunkWidth)}
		chunk := chunks[key]
		if chunk == nil {
			chunk = &Chunk{}
			chunks[key] = chunk
		}
		lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
		chunk.blocks.Set(lx, y, lz, block)
		if int(chunk.heightMap[lx][lz]) <= y {
			chunk.heightMap[lx][lz] = int16(y + 1)
		}
		if x%lodCellSize == 0 && z%lodCellSize == 0 {
			samples[lodPoint{x, z}] = true
		}
	}
	for x := -136; x <= -56; x++ {
		for y := 110; y <= 190; y++ {
			dx, dy := x+96, y-150
			radiusSquared := dx*dx + dy*dy
			if radiusSquared < 22*22 || radiusSquared > 40*40 {
				continue
			}
			for z := 1020; z < 1028; z++ {
				place(x, y, z, blockObsidian)
			}
		}
	}
	for x := 64; x <= 128; x++ {
		for z := 992; z <= 1056; z++ {
			for y := 178; y <= 181; y++ {
				place(x, y, z, blockIronBlock)
			}
		}
	}
	for point := range samples {
		key := chunkKey{divFloor(point.X, chunkWidth), divFloor(point.Z, chunkWidth)}
		w.recordLODColumn(chunks[key], point.X, point.Z)
	}
	return w, chunks
}

func TestLODVerticalRingAndFloatingPlatformLimits(t *testing.T) {
	const seed = uint32(1234511)
	w, chunks := lodVerticalStructureFixture(seed)
	blockAt := func(x, y, z int) byte {
		key := chunkKey{divFloor(x, chunkWidth), divFloor(z, chunkWidth)}
		return chunks[key].blocks.Get(modFloor(x, chunkWidth), y, modFloor(z, chunkWidth))
	}
	if blockAt(-96, 150, 1024) != blockAir || blockAt(-96, 190, 1024) != blockObsidian {
		t.Fatal("vertical ring fixture has no genuine empty center and top arch")
	}
	if blockAt(96, 150, 1024) != blockAir || blockAt(96, 180, 1024) != blockIronBlock {
		t.Fatal("floating-platform fixture is not suspended over empty air")
	}
	for _, tc := range []struct {
		name   string
		x, z   int
		block  byte
		height int
	}{
		{"ring center", -96, 1024, blockObsidian, 191},
		{"platform center", 96, 1024, blockIronBlock, 182},
	} {
		if got := w.lodColumns[lodPoint{tc.x, tc.z}]; got != (lodColumn{height: tc.height, top: tc.block}) {
			t.Fatalf("%s: top-only capture=%+v", tc.name, got)
		}
		for _, step := range []int{lodNearCellSize, lodCellSize, lodFarCellSize} {
			key := lodTileKey{divFloor(tc.x, lodTileSize), divFloor(tc.z, lodTileSize)}
			tile := buildLODTileAtStep(seed, key, w.snapshotLODTile(key), step)
			cap := [3]float32{float32(tc.x), float32(tc.height) - .65, float32(tc.z)}
			found := false
			for _, vertex := range tile.vertices {
				if vertex.Position == cap && vertex.Color == lodStructureColor(tc.block, 1) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s: %d-block LOD lost the captured top", tc.name, step)
			}
		}
	}
	// The test intentionally records the current limitation: both different
	// occupancy profiles collapse to one top height and therefore become
	// ground-connected prisms. It is not an assertion of visual correctness.
}
