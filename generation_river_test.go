package main

import (
	"math"
	"testing"
)

func TestGenerationRiversAndFrozenWater(t *testing.T) {
	counts := map[int]int{}
	boundary := false
	for x := -2048; x <= 2048; x += 8 {
		for z := -2048; z <= 2048; z += 8 {
			c := sampleTerrainColumn(42, x, z)
			if c.biomeID != BiomeRiver && c.biomeID != BiomeFrozenRiver {
				continue
			}
			counts[c.biomeID]++
			if c.ocean || c.height >= seaLevel || c.topY() != seaLevel {
				t.Fatal("river classified as ocean or dry")
			}
			want := blockWater
			if c.biomeID == BiomeFrozenRiver {
				want = blockIce
			}
			if got := c.blockAt(42, x, seaLevel-1, z); got != want {
				t.Fatalf("river surface %d want %d", got, want)
			}
			if _, ok := sampleTreeAnchor(42, x, z); ok {
				t.Fatal("tree in river")
			}
			if x%chunkWidth == 0 && !boundary {
				var chunk Chunk
				cx, cz := int(math.Floor(float64(x)/chunkWidth)), int(math.Floor(float64(z)/chunkWidth))
				generateChunkData(42, cx, cz, &chunk)
				if chunk.blocks.Get(0, seaLevel-1, z-cz*chunkWidth) != want {
					t.Fatal("chunk/fallback river mismatch")
				}
				boundary = true
			}
		}
	}
	if counts[BiomeRiver] < 100 || counts[BiomeFrozenRiver] < 100 || !boundary {
		t.Fatalf("missing river coverage %v", counts)
	}
	t.Logf("river samples: %v", counts)
}
