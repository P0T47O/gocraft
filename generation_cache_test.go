package main

import "testing"

func TestTerrainCacheMatchesReference(t *testing.T) {
	for _, seed := range []uint32{0, 12345, 0xffffffff} {
		for _, pos := range [][2]int{{0, 0}, {-1, -1}, {21, -39}} {
			var cache terrainSampleCache
			cache.init(seed, pos[0], pos[1])
			for x := -treeRadius; x < chunkWidth+treeRadius; x++ {
				for z := -treeRadius; z < chunkWidth+treeRadius; z++ {
					wx, wz := pos[0]*chunkWidth+x, pos[1]*chunkWidth+z
					if cache.column(seed, wx, wz) != sampleTerrainColumn(seed, wx, wz) {
						t.Fatalf("column differs: seed %d (%d,%d)", seed, wx, wz)
					}
				}
			}
			cached, reference := new(Chunk), new(Chunk)
			generateChunkData(seed, pos[0], pos[1], cached)
			generateChunkDataSampled(seed, pos[0], pos[1], reference, sampleTerrainColumn)
			if !cached.blocks.Equal(&reference.blocks) || cached.heightMap != reference.heightMap {
				t.Fatalf("generated chunk differs: seed %d at %v", seed, pos)
			}
		}
	}
}

func BenchmarkGenerationSampling(b *testing.B) {
	for _, cached := range []bool{false, true} {
		name := "reference"
		if cached {
			name = "cached"
		}
		b.Run(name, func(b *testing.B) {
			c := new(Chunk)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// Multiple locations include negative coordinates, inland and ocean.
				cx, cz := i%8-4, i%5-2
				if cached {
					generateChunkData(12345, cx, cz, c)
				} else {
					generateChunkDataSampled(12345, cx, cz, c, sampleTerrainColumn)
				}
			}
		})
	}
}
