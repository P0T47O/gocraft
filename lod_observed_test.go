package main

import "testing"

func TestLODObservedTreeRemovalAndFill(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	const seed = uint32(1234511)
	var anchor treeAnchor
	found := false
	for z := 0; z < 128 && !found; z++ {
		for x := 0; x < 128; x++ {
			if modFloor(x, 16) < 3 || modFloor(x, 16) > 12 || modFloor(z, 16) < 3 || modFloor(z, 16) > 12 {
				continue
			}
			if (hash2(seed+1, x, z)+1)*.5 >= maxTreeAnchorChance {
				continue
			}
			if a, ok := sampleTreeAnchor(seed, x, z); ok {
				anchor, found = a, true
				break
			}
		}
	}
	if !found {
		t.Fatal("no deterministic tree fixture")
	}
	key := chunkKey{divFloor(anchor.x, 16), divFloor(anchor.z, 16)}
	var source Chunk
	generateChunkData(seed, key.X, key.Z, &source)
	w := NewClientWorld()
	defer w.Close()
	w.seed = seed
	w.applyChunkPacket(chunkPacket(key, &source))
	tileKey := lodTileKey{divFloor(anchor.x, 128), divFloor(anchor.z, 128)}
	before := w.snapshotLODTile(tileKey)
	if !before[lodPoint{anchor.x, anchor.z}].observed || !lodOccupiedAt(before[lodPoint{anchor.x, anchor.z}].occupancy, anchor.y) {
		t.Fatal("actual natural trunk not captured")
	}
	// Remove foliage in this real chunk, including the chosen generated tree.
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			for y := 0; y < chunkHeight; y++ {
				b := source.blocks.Get(x, y, z)
				if generationIsLog(b) || generationIsLeaf(b) {
					w.SetBlockAt(key.X*16+x, y, key.Z*16+z, blockAir)
				}
			}
		}
	}
	w.UnloadChunks(100, 100, 8, nil)
	after := w.snapshotLODTile(tileKey)
	if lodOccupiedAt(after[lodPoint{anchor.x, anchor.z}].occupancy, anchor.y) {
		t.Fatal("removed tree survived in actual geometry")
	}
	for _, step := range []int{2, 4, 8, 16} {
		mesh := buildLODTileAtStep(seed, tileKey, after, step)
		trunkPoint := [3]float32{float32(anchor.x) - .35, float32(anchor.y) - .5, float32(anchor.z) - .35}
		for _, v := range mesh.vertices {
			if v.Position == trunkPoint && v.Color[3] == 254 {
				t.Fatalf("procedural tree reappeared at step %d", step)
			}
		}
	}
	// Filling an off-grid column is preserved as explicit geometry, not a
	// large interpolated hill that would bury neighboring buildings.
	w.applyChunkPacket(chunkPacket(key, &source))
	x, z := key.X*16+3, key.Z*16+5
	ground := sampleTerrainColumn(seed, x, z).height
	for y := ground; y < ground+3; y++ {
		w.SetBlockAt(x, y, z, blockDirt)
	}
	c := w.snapshotLODTile(tileKey)[lodPoint{x, z}]
	if c.height != ground || !lodOccupiedAt(c.occupancy, ground+2) {
		t.Fatal("off-grid fill was lost or pulled terrain into a hill")
	}
}
