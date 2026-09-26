package main

import "testing"

func TestLODSparseChunkSnapshotAndEdits(t *testing.T) {
	const seed = uint32(1234511)
	var chunk Chunk
	generateChunkData(seed, 0, 0, &chunk)
	w := &World{seed: seed, IsClient: true}
	for z := 0; z < chunkWidth; z += lodCellSize {
		for x := 0; x < chunkWidth; x += lodCellSize {
			w.recordLODColumn(&chunk, x, z)
		}
	}
	if len(w.lodColumns) != 0 {
		t.Fatalf("untouched generated chunk produced %d LOD overrides", len(w.lodColumns))
	}
	point := lodPoint{0, 0}
	base := sampleTerrainColumn(seed, point.X, point.Z)
	y := int(chunk.heightMap[0][0]) + 5
	if y >= chunkHeight {
		t.Skip("terrain too high for tower fixture")
	}
	old := chunk.blocks.Get(0, y, 0)
	oldHeight := chunk.heightMap[0][0]
	chunk.blocks.Set(0, y, 0, blockCobblestone)
	chunk.heightMap[0][0] = int16(y + 1)
	w.recordLODColumn(&chunk, point.X, point.Z)
	got, ok := w.lodColumns[point]
	if !ok || got != (lodColumn{y + 1, blockCobblestone}) {
		t.Fatalf("placed tower not captured: %+v, present=%t", got, ok)
	}
	if got.height <= base.height {
		t.Fatal("fixture did not raise the terrain")
	}
	for _, key := range []lodTileKey{{0, 0}, {-1, 0}, {0, -1}, {-1, -1}} {
		if w.lodVersions[key] != 1 {
			t.Fatalf("shared tile edge %v not invalidated", key)
		}
	}
	snapshot := w.snapshotLODTile(lodTileKey{0, 0})
	tile := buildLODTileWithColumns(seed, lodTileKey{0, 0}, snapshot)
	if tile.vertices[0].Position[1] != float32(y+1)-.65 || tile.vertices[0].Color != lodSurfaceColor(blockCobblestone) {
		t.Fatalf("mesh did not use authoritative surface sample: %+v", tile.vertices[0])
	}
	chunk.blocks.Set(0, y, 0, old)
	chunk.heightMap[0][0] = oldHeight
	w.recordLODColumn(&chunk, point.X, point.Z)
	if _, ok := w.lodColumns[point]; ok {
		t.Fatal("restored procedural column retained an override")
	}
	if snapshot[point] != (lodColumn{y + 1, blockCobblestone}) {
		t.Fatal("worker snapshot was mutated by a later world edit")
	}
	if w.lodVersions[lodTileKey{-1, -1}] != 2 {
		t.Fatal("restoring a column did not invalidate distant tiles")
	}
}

func TestLODSnapshotSkipsTreeAndDecoration(t *testing.T) {
	var chunk Chunk
	chunk.blocks.Set(0, 70, 0, blockGrass)
	chunk.blocks.Set(0, 71, 0, blockLog)
	chunk.blocks.Set(0, 72, 0, blockLeaves)
	chunk.blocks.Set(0, 73, 0, blockRose)
	chunk.heightMap[0][0] = 74
	if got := snapshotLODColumn(&chunk, 0, 0); got != (lodColumn{71, blockGrass}) {
		t.Fatalf("tree/decorations changed terrain surface: %+v", got)
	}
}

func TestLODUnmodifiedBiomesRemainSparse(t *testing.T) {
	const seed = uint32(1234511)
	w := &World{seed: seed, IsClient: true}
	for _, key := range []chunkKey{{0, 0}, {-12, -12}, {14, -7}, {30, 22}, {100, 100}} {
		var chunk Chunk
		generateChunkData(seed, key.X, key.Z, &chunk)
		for z := 0; z < chunkWidth; z += lodCellSize {
			for x := 0; x < chunkWidth; x += lodCellSize {
				w.recordLODColumn(&chunk, key.X*chunkWidth+x, key.Z*chunkWidth+z)
			}
		}
	}
	if len(w.lodColumns) != 0 {
		t.Fatalf("unaltered natural terrain stored %d unnecessary LOD overrides", len(w.lodColumns))
	}
}

func TestLODChunkPacketAndBlockChange(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 24, HorizonDistance: 96}
	defer func() { currentSettings = previous }()
	const seed = uint32(1234511)
	var source Chunk
	generateChunkData(seed, 0, 0, &source)
	y := int(source.heightMap[8][8]) + 5
	if y >= chunkHeight {
		t.Skip("terrain too high for tower fixture")
	}
	source.blocks.Set(8, y, 8, blockCobblestone)
	source.heightMap[8][8] = int16(y + 1)
	w := NewClientWorld()
	defer w.Close()
	w.seed = seed
	if !w.applyChunkPacket(chunkPacket(chunkKey{}, &source)) {
		t.Fatal("server chunk snapshot rejected")
	}
	point := lodPoint{8, 8}
	if w.lodColumns[point] != (lodColumn{y + 1, blockCobblestone}) || !w.chunks[chunkKey{}].lodCaptured {
		t.Fatalf("server-edited column not captured: %+v", w.lodColumns[point])
	}
	before := w.lodVersions[lodTileKey{}]
	// Previously captured chunks must keep their sparse record current even
	// while the user temporarily disables the horizon.
	currentSettings.HorizonDistance = 0
	w.SetBlockAt(8, y, 8, blockAir)
	if _, ok := w.lodColumns[point]; ok {
		t.Fatal("block change did not restore procedural LOD surface")
	}
	if w.lodVersions[lodTileKey{}] <= before {
		t.Fatal("block change did not invalidate the LOD tile")
	}
}
