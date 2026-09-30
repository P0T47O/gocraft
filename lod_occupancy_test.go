package main

import "testing"

func TestLODBuildingSurvivesChunkUnloadAndReload(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	const seed = uint32(1234511)
	w := NewClientWorld()
	defer w.Close()
	w.seed = seed
	var source Chunk
	generateChunkData(seed, 0, 0, &source)
	if !w.applyChunkPacket(chunkPacket(chunkKey{}, &source)) {
		t.Fatal("packet rejected")
	}
	// A wooden roof entirely between the former 8-block sample points.
	y := max(int(source.heightMap[3][5])+10, 180)
	for x := 3; x <= 6; x++ {
		w.SetBlockAt(x, y, 5, blockLog)
	}
	key := lodTileKey{}
	snapshot := w.snapshotLODTile(key)
	for x := 3; x <= 6; x++ {
		c := snapshot[lodPoint{x, 5}].occupancy
		if c == nil || !lodOccupiedAt(c, y) || lodOccupiedAt(c, y-1) {
			t.Fatalf("roof at %d was missed or filled to ground", x)
		}
	}
	w.UnloadChunks(100, 100, 8, nil)
	if w.getChunkIfGenerated(0, 0) != nil {
		t.Fatal("fixture chunk not unloaded")
	}
	for _, step := range []int{2, 4, 8, 16} {
		tile := buildLODTileAtStep(seed, key, w.snapshotLODTile(key), step)
		foundTop, foundBottom := false, false
		for _, v := range tile.vertices {
			if v.Position == ([3]float32{2.5, float32(y) + .5, 4.5}) && v.Color == lodStructureColor(blockLog, 1) {
				foundTop = true
			}
			if v.Position == ([3]float32{2.5, float32(y) - .5, 4.5}) && v.Color == lodStructureColor(blockLog, .65) {
				foundBottom = true
			}
		}
		if !foundTop || !foundBottom {
			t.Fatalf("step %d lost roof or underside after unloading", step)
		}
	}
	// Server snapshots also recover saved buildings, without local placement
	// events. Then removing them clears the cached geometry after unloading.
	for x := 3; x <= 6; x++ {
		source.blocks.Set(x, y, 5, blockLog)
		source.heightMap[x][5] = int16(y + 1)
	}
	if !w.applyChunkPacket(chunkPacket(chunkKey{}, &source)) {
		t.Fatal("reload rejected")
	}
	if !lodOccupiedAt(w.snapshotLODTile(key)[lodPoint{3, 5}].occupancy, y) {
		t.Fatal("server snapshot lost saved off-grid wooden roof")
	}
	before := w.lodVersions[key]
	for x := 3; x <= 6; x++ {
		w.SetBlockAt(x, y, 5, blockAir)
	}
	w.UnloadChunks(100, 100, 8, nil)
	if w.lodVersions[key] <= before {
		t.Fatal("demolition did not invalidate LOD")
	}
	for x := 3; x <= 6; x++ {
		if w.snapshotLODTile(key)[lodPoint{x, 5}].occupancy != nil {
			t.Fatal("demolished roof persisted")
		}
		if !lodOccupiedAt(snapshot[lodPoint{x, 5}].occupancy, y) {
			t.Fatal("worker snapshot changed after demolition")
		}
	}
}

func TestLODOccupancyNaturalChunksRemainSparse(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	w := &World{seed: 1234511, IsClient: true}
	for _, key := range []chunkKey{{0, 0}, {-12, -12}, {14, -7}, {30, 22}, {100, 100}} {
		var chunk Chunk
		generateChunkData(w.seed, key.X, key.Z, &chunk)
		w.recordChunkLODColumns(&chunk, key.X, key.Z)
	}
	if len(w.lodOccupancyTiles) != 0 {
		t.Fatalf("natural terrain retained %d structure tiles", len(w.lodOccupancyTiles))
	}
}

func TestLODOccupancyCrossTileSidesAndVerticalGaps(t *testing.T) {
	w := &World{seed: 1234511}
	var chunk Chunk
	for _, x := range []int{-1, 0} {
		lx := modFloor(x, chunkWidth)
		for _, y := range []int{180, 181, 185} {
			chunk.blocks.Set(lx, y, 5, blockIronBlock)
		}
		chunk.heightMap[lx][5] = 186
		w.recordLODOccupancy(&chunk, x, 5)
	}
	if w.lodVersions[lodTileKey{-1, 0}] < 2 || w.lodVersions[lodTileKey{0, 0}] < 2 {
		t.Fatal("cross-tile edits did not invalidate both meshes")
	}
	snapshot := w.snapshotLODTile(lodTileKey{})
	c := snapshot[lodPoint{0, 5}].occupancy
	if c == nil || len(c.spans) != 2 || lodOccupiedAt(c, 183) {
		t.Fatal("vertical air gap collapsed")
	}
	var tile lodTileData
	appendLODOccupancy(&tile, lodTileKey{}, snapshot)
	// Two isolated runs would emit 12 quads; their west faces are occluded
	// by the other tile's columns, leaving 10 quads.
	if len(tile.vertices) != 40 {
		t.Fatalf("unexpected exposed faces: %d vertices", len(tile.vertices))
	}
}

func BenchmarkLODCaptureLoadedChunk(b *testing.B) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	var chunk Chunk
	generateChunkData(1234511, 0, 0, &chunk)
	w := &World{seed: 1234511, IsClient: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.recordChunkLODColumns(&chunk, 0, 0)
	}
}
