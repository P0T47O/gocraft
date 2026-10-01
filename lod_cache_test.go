package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitLODCache(t *testing.T, w *World, key chunkKey) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w.requestLODCacheChunk(key)
		w.processLODCache()
		if w.lodObservedChunks[key] != nil {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("cache record did not arrive")
}

func TestClientLODCacheRestartGroundFoliageAndFreshSnapshot(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	root := t.TempDir()
	const seed = uint32(1234511)
	key := chunkKey{}
	var source Chunk
	generateChunkData(seed, 0, 0, &source)
	w := NewClientWorld()
	w.seed = seed
	w.openLODCache(root, "save:one")
	if !w.applyChunkPacket(chunkPacket(key, &source)) {
		t.Fatal("packet rejected")
	}
	x, z := 3, 5
	ground := sampleTerrainColumn(seed, x, z).height
	for y := ground - 3; y < ground; y++ {
		w.SetBlockAt(x, y, z, blockAir)
	}
	w.SetBlockAt(4, 180, 5, blockIronBlock)
	w.SetBlockAt(4, 181, 5, blockLeavesBirch)
	snapshot := w.snapshotLODTile(lodTileKey{})
	if got := snapshot[lodPoint{x, z}]; got.height != ground-3 || !got.changed {
		t.Fatalf("off-grid digging not captured: %+v", got)
	}
	w.UnloadChunks(100, 100, 8, nil)
	w.Close() // Flushes dirty chunks even if no frame pump ran.
	reopened := NewClientWorld()
	defer reopened.Close()
	reopened.seed = seed
	reopened.openLODCache(root, "save:one")
	waitLODCache(t, reopened, key)
	if reopened.getChunkIfGenerated(0, 0) != nil {
		t.Fatal("cache loaded a complete gameplay chunk")
	}
	columns := reopened.snapshotLODTile(lodTileKey{})
	if columns[lodPoint{x, z}].height != ground-3 {
		t.Fatal("digging lost after restart")
	}
	if !lodOccupiedAt(columns[lodPoint{4, 5}].occupancy, 181) {
		t.Fatal("building/planted foliage lost after restart")
	}
	for _, step := range []int{2, 4, 8, 16} {
		tile := buildLODTileAtStep(seed, lodTileKey{}, columns, step)
		found := false
		for _, v := range tile.vertices {
			if v.Position == ([3]float32{2.5, float32(ground-3) - .5, 4.5}) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%d-block terrain grid ignored off-grid digging", step)
		}
	}
	// Real server state overwrites cached excavation and planted geometry.
	if !reopened.applyChunkPacket(chunkPacket(key, &source)) {
		t.Fatal("fresh packet rejected")
	}
	updated := reopened.snapshotLODTile(lodTileKey{})
	if updated[lodPoint{x, z}].height != ground || lodOccupiedAt(updated[lodPoint{4, 5}].occupancy, 180) {
		t.Fatal("cache overrode fresh server data")
	}
	if snapshot[lodPoint{x, z}].height != ground-3 {
		t.Fatal("old worker snapshot mutated")
	}
	reopened.Close()
	third := NewClientWorld()
	defer third.Close()
	third.seed = seed
	third.openLODCache(root, "save:one")
	waitLODCache(t, third, key)
	if third.snapshotLODTile(lodTileKey{})[lodPoint{x, z}].height != ground {
		t.Fatal("refresh not persisted")
	}
	for _, tc := range []struct {
		identity string
		seed     uint32
	}{{"save:other", seed}, {"save:one", seed + 1}} {
		other := NewClientWorld()
		other.seed = tc.seed
		other.openLODCache(root, tc.identity)
		if other.lodCache.dir == third.lodCache.dir {
			t.Fatal("world/seed cache namespaces collided")
		}
		other.Close()
	}
}

func TestLODCacheLateLoadCannotUndoLocalEdit(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	w := NewClientWorld()
	defer w.Close()
	w.seed = 1234511
	w.openLODCache(t.TempDir(), "race")
	var source Chunk
	generateChunkData(w.seed, 0, 0, &source)
	w.applyChunkPacket(chunkPacket(chunkKey{}, &source))
	old := w.lodObservedChunks[chunkKey{}]
	w.SetBlockAt(3, 180, 5, blockGoldBlock)
	w.lodCache.loaded <- lodCacheJob{key: chunkKey{}, record: old, epoch: 0}
	w.processLODCache()
	if !lodOccupiedAt(w.snapshotLODTile(lodTileKey{})[lodPoint{3, 5}].occupancy, 180) {
		t.Fatal("late disk snapshot undid placement")
	}
}

func TestLODCacheCorruptionAndMemoryReload(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 8, ExperimentalLOD: true, HorizonDistance: 64}
	defer func() { currentSettings = previous }()
	root := t.TempDir()
	w := NewClientWorld()
	w.seed = 1234511
	w.openLODCache(root, "test")
	var source Chunk
	generateChunkData(w.seed, 0, 0, &source)
	w.applyChunkPacket(chunkPacket(chunkKey{}, &source))
	w.SetBlockAt(3, 180, 5, blockGoldBlock)
	dir := w.lodCache.dir
	w.Close()
	path := filepath.Join(dir, "0_0.lod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &lodDiskCache{seed: 1234511}
	for _, n := range []int{0, 4, 25, len(data) - 1} {
		bad := filepath.Join(t.TempDir(), "0_0.lod")
		os.WriteFile(bad, data[:n], 0600)
		if _, err := c.read(bad); err == nil {
			t.Fatalf("accepted truncated cache at %d bytes", n)
		}
	}
	reopened := NewClientWorld()
	defer reopened.Close()
	reopened.seed = 1234511
	reopened.openLODCache(root, "test")
	waitLODCache(t, reopened, chunkKey{})
	reopened.trimLODCacheMemory(0)
	if len(reopened.lodObservedChunks) != 0 {
		t.Fatal("cold cache not evicted")
	}
	reopened.requestLODCacheTile(lodTileKey{}) // Runs even when the tile mesh needs no rebuild.
	waitLODCache(t, reopened, chunkKey{})
	if !lodOccupiedAt(reopened.snapshotLODTile(lodTileKey{})[lodPoint{3, 5}].occupancy, 180) {
		t.Fatal("cold disk record not restored")
	}
}

func TestLODCacheIgnoresMalformedFiles(t *testing.T) {
	w := NewClientWorld()
	defer w.Close()
	w.seed = 42
	w.openLODCache(t.TempDir(), "empty")
	dir := w.lodCache.dir
	os.WriteFile(filepath.Join(dir, "0_0.lod"), []byte("broken"), 0600)
	w.closeLODCache()
	w.openLODCache(filepath.Dir(dir), "empty")
	// Closing must cancel a loader even if its result queue is full.
	w.closeLODCache()
}
