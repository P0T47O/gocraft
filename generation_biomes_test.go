package main

import "testing"

func TestGenerationClassicBiomeCoverage(t *testing.T) {
	counts := map[int]int{}
	heights := map[int]int{}
	for _, seed := range []uint32{0, 1, 42, 12345, 98765} {
		for x := -4096; x <= 4096; x += 64 {
			for z := -4096; z <= 4096; z += 64 {
				c := sampleTerrainColumn(seed, x, z)
				counts[c.biomeID]++
				heights[c.biomeID] += c.height
				if getBiome(seed, x, z) != c.biomeID || c.ocean != isOceanBiome(c.biomeID) {
					t.Fatal("environment consumers disagree")
				}
				switch c.biomeID {
				case BiomeDesert:
					if c.environment.weights[regionDesert] > .8 && (c.top != blockSand || c.filler != blockSandstone) {
						t.Fatal("desert surface")
					}
				case BiomeTaiga, BiomeSnowyTundra, BiomeSnowyBeach:
					if c.environment.weights[regionTaiga]+c.environment.weights[regionSnow] > .8 && c.top != blockSnow {
						t.Fatal("missing snow")
					}
				case BiomeFrozenOcean:
					if c.blockAt(seed, x, int(seaLevel)-1, z) != blockIce {
						t.Fatal("missing ice")
					}
				case BiomeOcean, BiomeBeach, BiomeForest, BiomePlains, BiomeExtremeHills:
				default:
					t.Fatalf("unexpected later-era biome %d", c.biomeID)
				}
				if d := absInt(c.height - sampleTerrainColumn(seed, x+1, z).height); d > 4 {
					t.Fatalf("height seam at %d,%d seed %d: %d", x, z, seed, d)
				}
			}
		}
	}
	for _, id := range []int{BiomePlains, BiomeForest, BiomeDesert, BiomeTaiga, BiomeSnowyTundra, BiomeExtremeHills, BiomeOcean, BiomeFrozenOcean, BiomeBeach, BiomeSnowyBeach} {
		if counts[id] < 50 {
			t.Fatalf("biome %d too scarce in fixture: %d", id, counts[id])
		}
		t.Logf("biome %d: samples=%d meanHeight=%.1f", id, counts[id], float64(heights[id])/float64(counts[id]))
	}
	if float64(heights[BiomeExtremeHills])/float64(counts[BiomeExtremeHills]) < float64(heights[BiomePlains])/float64(counts[BiomePlains])+15 {
		t.Fatal("mountains lack relief")
	}
}

func TestGenerationClassicVegetation(t *testing.T) {
	for _, id := range []int{BiomePlains, BiomeForest, BiomeDesert, BiomeTaiga, BiomeSnowyTundra, BiomeExtremeHills} {
		found := false
		for x := -2048; x <= 2048 && !found; x += 16 {
			for z := -2048; z <= 2048; z += 16 {
				c := sampleTerrainColumn(42, x, z)
				if c.biomeID != id {
					continue
				}
				var chunk Chunk
				generateChunkData(42, x/chunkWidth, z/chunkWidth, &chunk)
				for lx := 0; lx < chunkWidth; lx++ {
					for lz := 0; lz < chunkWidth; lz++ {
						col := sampleTerrainColumn(42, x+lx, z+lz)
						if col.biomeID != id {
							continue
						}
						top := chunk.blocks[lx][col.height-1][lz]
						if top >= blockCoalOre && top <= blockLapisOre {
							top = blockStone
						}
						if top != col.top {
							t.Fatal("decoration damaged surface")
						}
						if col.environment.weights[regionDesert] > .99 || col.environment.weights[regionSnow] > .99 {
							if _, ok := sampleTreeAnchor(42, x+lx, z+lz); ok {
								t.Fatal("tree in treeless biome")
							}
						}
					}
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no fixture for biome %d", id)
		}
	}
}
