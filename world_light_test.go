package main

import (
	"math/rand"
	"testing"
)

// Keep registry changes scoped to each test, without requiring graphics setup.
func lightingTestRegistry(t testing.TB) {
	oldStone, oldTorch, oldLava := Blocks[blockStone], Blocks[blockTorch], Blocks[blockLava]
	Blocks[blockStone] = &BlockDef{IsOpaque: true}
	Blocks[blockTorch] = &BlockDef{LightLevel: 14}
	Blocks[blockLava] = &BlockDef{IsOpaque: true, LightLevel: 15}
	t.Cleanup(func() { Blocks[blockStone], Blocks[blockTorch], Blocks[blockLava] = oldStone, oldTorch, oldLava })
}

func lightingTestWorld(chunks map[chunkKey]*Chunk) *World {
	w := &World{chunks: chunks, lightChanged: make(map[chunkKey]bool)}
	for _, c := range chunks {
		c.generated = true
		initializeChunkLighting(c)
	}
	for key := range chunks {
		w.stitchChunkLighting(key.X, key.Z)
	}
	return w
}

// Independent rebuild oracle: clear everything, enqueue ALL direct sky and
// emitter sources, then flood across the fixture. Never calls the production
// initializer, stitcher, cached solver, or dirty-section machinery.
func referenceLighting(w *World) map[chunkKey]*Chunk {
	result := make(map[chunkKey]*Chunk)
	for key, c := range w.chunks {
		result[key] = &Chunk{blocks: c.blocks}
	}
	type point struct{ x, y, z int }
	directions := [...]point{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	for _, sky := range []bool{true, false} {
		queue := make([]point, 0)
		for key, c := range result {
			for x := 0; x < chunkWidth; x++ {
				for z := 0; z < chunkWidth; z++ {
					open := true
					for y := chunkHeight - 1; y >= 0; y-- {
						b := GetBlock(c.blocks[x][y][z])
						if b.IsOpaque {
							open = false
						}
						level := b.LightLevel
						if sky {
							level = 0
							if open {
								level = 15
							}
							c.skyLight[x][y][z] = level
						} else {
							c.blockLight[x][y][z] = level
						}
						if level > 0 {
							queue = append(queue, point{key.X*chunkWidth + x, y, key.Z*chunkWidth + z})
						}
					}
				}
			}
		}
		for head := 0; head < len(queue); head++ {
			p := queue[head]
			c := result[chunkKey{divFloor(p.x, chunkWidth), divFloor(p.z, chunkWidth)}]
			level := c.blockLight[modFloor(p.x, chunkWidth)][p.y][modFloor(p.z, chunkWidth)]
			if sky {
				level = c.skyLight[modFloor(p.x, chunkWidth)][p.y][modFloor(p.z, chunkWidth)]
			}
			if level <= 1 {
				continue
			}
			for _, d := range directions {
				n := point{p.x + d.x, p.y + d.y, p.z + d.z}
				if n.y < 0 || n.y >= chunkHeight {
					continue
				}
				nc := result[chunkKey{divFloor(n.x, chunkWidth), divFloor(n.z, chunkWidth)}]
				if nc == nil {
					continue
				}
				x, z := modFloor(n.x, chunkWidth), modFloor(n.z, chunkWidth)
				if GetBlock(nc.blocks[x][n.y][z]).IsOpaque {
					continue
				}
				target := &nc.blockLight[x][n.y][z]
				if sky {
					target = &nc.skyLight[x][n.y][z]
				}
				if *target < level-1 {
					*target = level - 1
					queue = append(queue, n)
				}
			}
		}
	}
	return result
}

func assertLightingRebuilt(t *testing.T, w *World) {
	t.Helper()
	want := referenceLighting(w)
	for key, c := range w.chunks {
		for x := 0; x < chunkWidth; x++ {
			for y := 0; y < chunkHeight; y++ {
				for z := 0; z < chunkWidth; z++ {
					r := want[key]
					if c.skyLight[x][y][z] != r.skyLight[x][y][z] || c.blockLight[x][y][z] != r.blockLight[x][y][z] {
						t.Fatalf("chunk %v local (%d,%d,%d): sky/block %d/%d, rebuilt %d/%d", key, x, y, z, c.skyLight[x][y][z], c.blockLight[x][y][z], r.skyLight[x][y][z], r.blockLight[x][y][z])
					}
				}
			}
		}
	}
}

func editLighting(w *World, x, y, z int, block byte) {
	c := w.chunks[chunkKey{divFloor(x, chunkWidth), divFloor(z, chunkWidth)}]
	lx, lz := modFloor(x, chunkWidth), modFloor(z, chunkWidth)
	old := c.blocks[lx][y][lz]
	c.blocks[lx][y][lz] = block
	w.updateBlockLight(x, y, z, old, block)
	w.updateSkyLight(x, y, z)
}

func TestLightingLocalDirectSkyAndRoof(t *testing.T) {
	lightingTestRegistry(t)
	c := &Chunk{}
	initializeChunkLighting(c)
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if c.skyLight[x][y][z] != 15 || c.blockLight[x][y][z] != 0 {
					t.Fatal("empty chunk must have direct sky everywhere")
				}
			}
		}
	}
	if c.generated || c.sectionDirty != nil {
		t.Fatal("offline initialization changed publication/mesh state")
	}
	for x := 4; x <= 10; x++ {
		for z := 4; z <= 10; z++ {
			c.blocks[x][20][z] = blockStone
		}
	}
	c.blocks[0][8][0] = blockTorch
	initializeChunkLighting(c)
	if c.skyLight[7][20][7] != 0 || c.skyLight[7][21][7] != 15 {
		t.Fatal("roof must be opaque with direct sky above")
	}
	if c.skyLight[4][19][7] != 14 || c.skyLight[7][19][7] != 11 {
		t.Fatal("lateral skylight under roof must attenuate")
	}
	assertLightingRebuilt(t, &World{chunks: map[chunkKey]*Chunk{{0, 0}: c}})
	// Full roof: missing chunks must never act as artificial sky sources.
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			c.blocks[x][20][z] = blockStone
		}
	}
	initializeChunkLighting(c)
	if c.skyLight[0][19][0] != 0 {
		t.Fatal("missing neighbors leaked skylight")
	}
}

func TestLightingTorchChunkArrivalAndReplacement(t *testing.T) {
	lightingTestRegistry(t)
	for _, reverse := range []bool{false, true} {
		a, b := &Chunk{}, &Chunk{}
		a.blocks[chunkWidth-1][16][7] = blockTorch
		keys := []chunkKey{{-1, 0}, {0, 0}}
		chunks := []*Chunk{a, b}
		if reverse {
			keys[0], keys[1] = keys[1], keys[0]
			chunks[0], chunks[1] = chunks[1], chunks[0]
		}
		w := lightingTestWorld(map[chunkKey]*Chunk{keys[0]: chunks[0]})
		initializeChunkLighting(chunks[1])
		chunks[1].generated = true
		w.chunks[keys[1]] = chunks[1]
		w.lightChanged = make(map[chunkKey]bool)
		w.stitchChunkLighting(keys[1].X, keys[1].Z)
		if b.blockLight[0][16][7] != 13 {
			t.Fatal("torch light did not cross arriving chunk border")
		}
		if !w.lightChanged[chunkKey{0, 0}] {
			t.Fatal("changed receiving chunk not marked for network refresh")
		}
		assertLightingRebuilt(t, w)
		// Replacing the source chunk must remove light already resident in b.
		replacement := &Chunk{generated: true}
		initializeChunkLighting(replacement)
		w.chunks[chunkKey{-1, 0}] = replacement
		w.lightChanged = make(map[chunkKey]bool)
		w.stitchChunkLighting(-1, 0)
		if b.blockLight[0][16][7] != 0 || !w.lightChanged[chunkKey{0, 0}] {
			t.Fatal("replacement left stale neighbor lighting")
		}
		assertLightingRebuilt(t, w)
	}
}

func TestLightingSkyChunkArrival(t *testing.T) {
	lightingTestRegistry(t)
	c := &Chunk{}
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			c.blocks[x][20][z] = blockStone
		}
	}
	w := lightingTestWorld(map[chunkKey]*Chunk{{0, 0}: c})
	neighbor := &Chunk{generated: true}
	initializeChunkLighting(neighbor)
	w.chunks[chunkKey{1, 0}] = neighbor
	w.stitchChunkLighting(1, 0)
	if c.skyLight[chunkWidth-1][19][8] != 14 || !w.lightChanged[chunkKey{0, 0}] {
		t.Fatal("arrival did not relight existing roof border")
	}
	assertLightingRebuilt(t, w)
	// The same publish API must reconcile a replacement that removes that sky.
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			neighbor.blocks[x][20][z] = blockStone
		}
	}
	w.rebuildLightingForChunk(1, 0)
	assertLightingRebuilt(t, w)
}

func TestLightingIncrementalMatchesRebuild(t *testing.T) {
	lightingTestRegistry(t)
	w := lightingTestWorld(map[chunkKey]*Chunk{{-1, 0}: {}, {0, 0}: {}})
	edits := []struct {
		x, y, z int
		block   byte
	}{
		{-1, 16, 7, blockTorch}, {2, 16, 7, blockTorch},
		{0, 16, 7, blockStone}, {0, 16, 7, blockAir},
		{-1, 16, 7, blockAir}, {2, 16, 7, blockStone},
		{0, 30, 7, blockStone}, {0, 18, 7, blockStone}, {0, 30, 7, blockAir},
		{0, 18, 7, blockAir}, {-1, 16, 0, blockLava}, {-1, 16, 0, blockAir},
		{0, chunkHeight - 1, 7, blockStone}, {0, chunkHeight - 1, 7, blockAir},
		{0, 0, 7, blockStone}, {0, 0, 7, blockAir},
	}
	for i, e := range edits {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			editLighting(w, e.x, e.y, e.z, e.block)
			assertLightingRebuilt(t, w)
		})
	}
}

func TestLightingRandomEditsUnderRoof(t *testing.T) {
	lightingTestRegistry(t)
	w := lightingTestWorld(map[chunkKey]*Chunk{{0, -1}: {}, {0, 0}: {}})
	for _, c := range w.chunks {
		for x := 0; x < chunkWidth; x++ {
			for z := 0; z < chunkWidth; z++ {
				c.blocks[x][24][z] = blockStone
			}
		}
		initializeChunkLighting(c)
	}
	for key := range w.chunks {
		w.stitchChunkLighting(key.X, key.Z)
	}
	rng := rand.New(rand.NewSource(7))
	blocks := []byte{blockAir, blockStone, blockTorch, blockLava}
	for i := 0; i < 24; i++ {
		x, y, z := rng.Intn(5)+5, rng.Intn(5)+20, rng.Intn(5)-2
		editLighting(w, x, y, z, blocks[rng.Intn(len(blocks))])
		assertLightingRebuilt(t, w)
	}
	for key, c := range w.chunks {
		for x := 0; x < chunkWidth; x++ {
			for y := 20; y <= 24; y++ {
				for z := 0; z < chunkWidth; z++ {
					if emitsLight(c.blocks[x][y][z]) {
						editLighting(w, key.X*chunkWidth+x, y, key.Z*chunkWidth+z, blockAir)
					}
				}
			}
		}
	}
	assertLightingRebuilt(t, w)
}

func TestLightingDirtyHaloBatchedAndNoop(t *testing.T) {
	lightingTestRegistry(t)
	w := lightingTestWorld(map[chunkKey]*Chunk{{0, 0}: {}, {1, 0}: {}, {0, 1}: {}, {1, 1}: {}})
	for _, c := range w.chunks {
		ensureChunkSections(c)
		for sec := range c.sectionDirty {
			c.sectionDirty[sec] = false
			c.meshVersion[sec] = 0
		}
	}
	w.lightChanged = make(map[chunkKey]bool)
	u := newLightUpdate(w)
	p := lightPos{chunkWidth - 1, sectionHeight - 1, chunkWidth - 1}
	u.set(p, false, 3)
	u.set(p, false, 4)
	u.flush()
	for key, c := range w.chunks {
		for sec := 0; sec < 2; sec++ {
			if !c.sectionDirty[sec] || c.meshVersion[sec] != 1 {
				t.Fatalf("halo chunk %v section %d was not invalidated once", key, sec)
			}
		}
		if c.meshVersion[2] != 0 {
			t.Fatal("unaffected section dirtied")
		}
	}
	if len(w.lightChanged) != 1 || !w.lightChanged[chunkKey{0, 0}] {
		t.Fatal("mesh halo confused with changed light data")
	}
	w.setBlockLightAtInternal(p.x, p.y, p.z, 4)
	if w.chunks[chunkKey{0, 0}].meshVersion[0] != 1 {
		t.Fatal("no-op write advanced mesh version")
	}
}

func TestLightingZeroQueueGuard(t *testing.T) {
	lightingTestRegistry(t)
	c := &Chunk{}
	spreadLocalLight(c, &c.blockLight, []lightPos{{1, 1, 1}})
	if c.blockLight[2][1][1] != 0 {
		t.Fatal("zero light underflowed")
	}
}

func BenchmarkLightingInitializeEmpty(b *testing.B) {
	lightingTestRegistry(b)
	c := &Chunk{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		initializeChunkLighting(c)
	}
}

func BenchmarkLightingTorchRemoval(b *testing.B) {
	lightingTestRegistry(b)
	w := lightingTestWorld(map[chunkKey]*Chunk{{0, 0}: {}, {1, 0}: {}})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		editLighting(w, chunkWidth-1, 16, 7, blockTorch)
		editLighting(w, chunkWidth-1, 16, 7, blockAir)
	}
}

func BenchmarkLightingStitchUnchanged(b *testing.B) {
	lightingTestRegistry(b)
	w := lightingTestWorld(map[chunkKey]*Chunk{{0, 0}: {}, {1, 0}: {}})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.stitchChunkLighting(0, 0)
	}
}
