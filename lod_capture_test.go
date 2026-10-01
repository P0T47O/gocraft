package main

import "testing"

func TestLODAsyncCapturePrivateSnapshotAndEdit(t *testing.T) {
	old := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = old }()
	w := NewClientWorld()
	defer w.Close()
	w.seed = 1234511
	w.startLODObservedCapture()
	var source Chunk
	generateChunkData(w.seed, 0, 0, &source)
	source.blocks.Set(3, 220, 5, blockIronBlock)
	source.rebuildHeightMap()
	w.applyChunkPacket(chunkPacket(chunkKey{}, &source))
	// Deliberately mutate the live storage without recapturing it: the worker
	// must retain iron in its private copy even if it has not started yet.
	w.getChunkIfGenerated(0, 0).blocks.Set(3, 220, 5, blockGoldBlock)
	w.finishLODObservedCapture()
	c := w.lodObservedChunks[chunkKey{}].columns[5*16+3]
	if got := c.occupancy.spans[len(c.occupancy.spans)-1].block; got != blockIronBlock {
		t.Fatalf("worker read live storage: %d", got)
	}
	w.startLODObservedCapture()
	source.blocks.Set(3, 220, 5, blockGoldBlock)
	// Force a new snapshot which races with a subsequent edit.
	source.blocks.Set(3, 219, 5, blockGoldBlock)
	w.applyChunkPacket(chunkPacket(chunkKey{}, &source))
	w.SetBlockAt(3, 221, 5, blockDiamondBlock)
	w.finishLODObservedCapture()
	c = w.lodObservedChunks[chunkKey{}].columns[5*16+3]
	last := c.occupancy.spans[len(c.occupancy.spans)-1]
	if last.block != blockDiamondBlock || last.hi != 222 {
		t.Fatal("late capture overwrote edit")
	}
}

func TestLODAsyncCaptureFlushesBeforeDiskShutdown(t *testing.T) {
	old := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = old }()
	root := t.TempDir()
	w := NewClientWorld()
	w.seed = 1234511
	w.openLODCache(root, "capture")
	w.startLODObservedCapture()
	var source Chunk
	generateChunkData(w.seed, 0, 0, &source)
	source.blocks.Set(3, 220, 5, blockIronBlock)
	source.rebuildHeightMap()
	w.applyChunkPacket(chunkPacket(chunkKey{}, &source))
	w.Close() // No render/update loop has consumed the capture result.
	next := NewClientWorld()
	defer next.Close()
	next.seed = 1234511
	next.openLODCache(root, "capture")
	waitLODCache(t, next, chunkKey{})
	if !lodOccupiedAt(next.snapshotLODTile(lodTileKey{})[lodPoint{3, 5}].occupancy, 220) {
		t.Fatal("shutdown dropped pending capture")
	}
}
