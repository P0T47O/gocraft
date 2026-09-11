package main

import (
	"math"
	"testing"
)

func TestGenerationDeterminismAndReuse(t *testing.T) {
	for _, p := range [][2]int{{0, 0}, {-1, -1}, {3, -7}, {-65, 64}} {
		var a, b Chunk
		generateChunkData(12345, p[0], p[1], &a)
		generateChunkData(98765, 10, -10, &b)
		generateChunkData(12345, p[0], p[1], &b)
		if a.blocks != b.blocks || a.heightMap != b.heightMap {
			t.Fatalf("reuse changed chunk %v", p)
		}
		generateChunkData(12346, p[0], p[1], &b)
		if a.blocks == b.blocks {
			t.Fatalf("seed had no effect at %v", p)
		}
	}
}

func TestGenerationProceduralConsistency(t *testing.T) {
	caves, water, land := 0, 0, 0
	for _, p := range [][2]int{{0, 0}, {-1, -1}, {16, 16}, {-32, 8}, {64, -64}, {-64, 64}} {
		var chunk Chunk
		generateChunkData(12345, p[0], p[1], &chunk)
		for x := 0; x < chunkWidth; x++ {
			for z := 0; z < chunkWidth; z++ {
				wx, wz := p[0]*chunkWidth+x, p[1]*chunkWidth+z
				c := sampleTerrainColumn(12345, wx, wz)
				if c.height < 2 || c.height >= chunkHeight || terrainHeight(12345, wx, wz) != c.height || terrainTopY(12345, wx, wz) != c.topY() {
					t.Fatal("invalid height contract")
				}
				if c.height < seaLevel {
					water++
				} else {
					land++
				}
				if blockAtProcedural(12345, wx, -1, wz) != blockAir || blockAtProcedural(12345, wx, chunkHeight, wz) != blockAir {
					t.Fatal("out of range fallback")
				}
				for y := 0; y < chunkHeight; y++ {
					got := chunk.blocks[x][y][z]
					want := blockAtProcedural(12345, wx, y, wz)
					if y >= c.topY() { // Decorations are excluded from fallback.
						if want != blockAir {
							t.Fatal("air begins at topY")
						}
						continue
					}
					if got >= blockCoalOre && got <= blockLapisOre {
						got = blockStone
					}
					if got != want {
						t.Fatalf("procedural mismatch (%d,%d,%d): %d != %d", wx, y, wz, got, want)
					}
					if y > 0 && y < c.height-4 && got == blockAir {
						caves++
					}
				}
				top := 0
				for y := 0; y < chunkHeight; y++ {
					if chunk.blocks[x][y][z] != blockAir {
						top = y + 1
					}
				}
				if int(chunk.heightMap[x][z]) != top {
					t.Fatal("height map disagrees with blocks")
				}
			}
		}
	}
	if caves == 0 || water == 0 || land == 0 {
		t.Fatalf("coverage caves=%d water=%d land=%d", caves, water, land)
	}
}

func TestGenerationFlowers(t *testing.T) {
	for _, biome := range []int{BiomePlains, BiomeForest, BiomeBirchForest} {
		for _, tc := range []struct {
			r float32
			b byte
		}{{0.90, blockAir}, {0.95, blockTallGrass}, {0.985, blockDandelion}, {0.995, blockRose}} {
			if got := vegetationBlock(biome, blockGrass, tc.r); got != tc.b {
				t.Fatalf("biome %d r=%f: %d", biome, tc.r, got)
			}
		}
	}
	// Exercise coordinate rolls as well as threshold dispatch.
	count := map[byte]int{}
	for x := -100; x < 100; x++ {
		for z := -100; z < 100; z++ {
			count[vegetationBlock(BiomePlains, blockGrass, (hash2(12349, x, z)+1)*0.5)]++
		}
	}
	if count[blockRose] == 0 || count[blockDandelion] == 0 {
		t.Fatal("flower rolls unreachable")
	}
}

func TestGenerationCavesProtectedAndContinuous(t *testing.T) {
	c := terrainColumn{height: 90, top: blockGrass, filler: blockDirt}
	carved, crossings := 0, 0
	for x := -32; x < 32; x++ {
		for z := -32; z < 32; z++ {
			for y := 0; y < 90; y++ {
				hole := caveAt(42, x, y, z, c)
				if hole {
					carved++
					if y < 4 || y >= 82 {
						t.Fatal("carved protected layer")
					}
					if x%chunkWidth == 0 && caveAt(42, x-1, y, z, c) {
						crossings++
					}
				}
				ocean := c
				ocean.ocean = true
				if caveAt(42, x, y, z, ocean) {
					t.Fatal("ocean carved")
				}
				submerged := c
				submerged.height = 61
				if caveAt(42, x, y, z, submerged) {
					t.Fatal("submerged column carved")
				}
			}
		}
	}
	if carved == 0 || crossings == 0 {
		t.Fatalf("missing caves/crossings: %d/%d", carved, crossings)
	}
	if float64(carved)/(64*64*90) > 0.10 {
		t.Fatal("caves are not conservative")
	}
	for _, x := range []float64{-32, -16, 0, 16, 32} {
		a := caveNoise(42, x-1e-7, 1.25, -0.4, 99)
		b := caveNoise(42, x+1e-7, 1.25, -0.4, 99)
		if math.Abs(a-b) > 1e-5 {
			t.Fatal("noise discontinuity at lattice boundary")
		}
	}
}

func TestGenerationFlowersInChunks(t *testing.T) {
	roses, dandelions := 0, 0
	for cx := -16; cx <= 16; cx += 2 {
		for cz := -16; cz <= 16; cz += 2 {
			c := sampleTerrainColumn(12345, cx*chunkWidth, cz*chunkWidth)
			if c.top != blockGrass || c.height < seaLevel {
				continue
			}
			var chunk Chunk
			generateChunkData(12345, cx, cz, &chunk)
			for x := 0; x < chunkWidth; x++ {
				for z := 0; z < chunkWidth; z++ {
					col := sampleTerrainColumn(12345, cx*chunkWidth+x, cz*chunkWidth+z)
					switch chunk.blocks[x][col.height][z] {
					case blockRose:
						roses++
					case blockDandelion:
						dandelions++
					}
				}
			}
			if roses > 0 && dandelions > 0 {
				return
			}
		}
	}
	t.Fatalf("flowers missing from generated chunks: roses=%d dandelions=%d", roses, dandelions)
}

func TestGenerationHashCoordinates(t *testing.T) {
	seen := map[uint64]bool{}
	for _, x := range []int{-65536, -1, 0, 1, 65536} {
		for _, z := range []int{-65536, -1, 0, 1, 65536} {
			for _, seed := range []uint32{0, 1, 0xffffffff} {
				h := generationHash(seed, x, 0, z, 0x1001)
				if seen[h] {
					t.Fatalf("hash alias at seed=%d x=%d z=%d", seed, x, z)
				}
				seen[h] = true
			}
		}
	}
	// These collided under the former (cx << 16) XOR cz packing.
	if generationHash(1, 1, 0, 0, 0x1001) == generationHash(1, 0, 0, 65536, 0x1001) {
		t.Fatal("packed coordinate alias")
	}
}

// Compare chunk-clipped output with a single larger world-coordinate reference.
// Scan for an actual boundary anchor so this cannot pass with no trees.
func TestGenerationTreeBoundariesAndOverlap(t *testing.T) {
	seed := uint32(12345)
	var anchor treeAnchor
	found := false
	for x := -256; x < 256 && !found; x++ {
		if x%chunkWidth != 0 {
			continue
		}
		for z := -256; z < 256; z++ {
			if a, ok := sampleTreeAnchor(seed, x, z); ok {
				anchor, found = a, true
				break
			}
		}
	}
	if !found {
		t.Fatal("no boundary anchor found")
	}
	bx := anchor.x - chunkWidth
	bz := int(math.Floor(float64(anchor.z)/chunkWidth))*chunkWidth - chunkWidth
	type pos struct{ x, y, z int }
	ref := map[pos]byte{}
	overlaps := 0
	for x := bx; x < bx+2*chunkWidth; x++ {
		for z := bz; z < bz+3*chunkWidth; z++ {
			c := sampleTerrainColumn(seed, x, z)
			for y := 0; y < c.topY(); y++ {
				ref[pos{x, y, z}] = c.blockAt(seed, x, y, z)
			}
		}
	}
	for x := bx - treeRadius; x < bx+2*chunkWidth+treeRadius; x++ {
		for z := bz - treeRadius; z < bz+3*chunkWidth+treeRadius; z++ {
			a, ok := sampleTreeAnchor(seed, x, z)
			if !ok {
				continue
			}
			a.emit(func(x, y, z int, b byte) {
				if x < bx || x >= bx+2*chunkWidth || z < bz || z >= bz+3*chunkWidth {
					return
				}
				p := pos{x, y, z}
				old := ref[p]
				if generationIsLeaf(old) && (generationIsLeaf(b) || generationIsLog(b)) {
					overlaps++
				}
				if old == blockAir || generationIsLog(b) && generationIsLeaf(old) {
					ref[p] = b
				}
			})
		}
	}
	leavesAcross := 0
	for dx := 1; dx >= 0; dx-- {
		for dz := 2; dz >= 0; dz-- {
			cx, cz := bx/chunkWidth+dx, bz/chunkWidth+dz
			var chunk Chunk
			generateChunkData(seed, cx, cz, &chunk)
			for x := 0; x < chunkWidth; x++ {
				for z := 0; z < chunkWidth; z++ {
					for y := 0; y < chunkHeight; y++ {
						wx, wz := cx*chunkWidth+x, cz*chunkWidth+z
						want, got := ref[pos{wx, y, wz}], chunk.blocks[x][y][z]
						if generationIsLog(want) || generationIsLeaf(want) || generationIsLog(got) || generationIsLeaf(got) {
							if got != want {
								t.Fatalf("tree seam (%d,%d,%d): %d != %d", wx, y, wz, got, want)
							}
							if wx == anchor.x-1 && generationIsLeaf(got) {
								leavesAcross++
							}
						}
					}
				}
			}
		}
	}
	if leavesAcross == 0 {
		t.Fatal("canopy did not cross chunk boundary")
	}
	if overlaps == 0 {
		t.Fatal("reference region did not exercise overlapping trees")
	}
}
