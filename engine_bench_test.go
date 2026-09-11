package main

import "testing"

// Identical geometry and lighting in both cases; only column-tint caching differs.
func BenchmarkMeshTintCache(b *testing.B) {
	if Blocks[blockStone] == nil {
		initBlockRegistry()
	}
	a := &RenderAssets{}
	var heights [chunkWidth][chunkWidth]int16
	for x := range heights {
		for z := range heights[x] {
			heights[x][z] = 16
		}
	}
	getBlock := func(x, y, z int) byte {
		if y == 15 {
			return blockGrass
		}
		if y >= 0 && y < 15 {
			return blockStone
		}
		return blockAir
	}
	getLight := func(x, y, z int) byte { return 15 }
	getMeta := func(x, y, z int) byte { return 0 }
	cached := a.buildMeshTintCache(12345, 0, 0)
	for _, reuse := range []bool{false, true} {
		name := "recompute"
		if reuse {
			name = "cached"
		}
		b.Run(name, func(b *testing.B) {
			var tint *meshTintCache
			if reuse {
				tint = cached
			}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				data := a.buildAllMeshData(&heights, 0, 0, 0, 16, getBlock, getLight, getMeta, 12345, tint)
				releaseMeshResults(data)
			}
		})
	}
}

func BenchmarkChunkGeneration(b *testing.B) {
	if Blocks[blockStone] == nil {
		initBlockRegistry()
	}
	c := &Chunk{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		generateChunkData(12345, 0, 0, c)
	}
}

func TestFramePercentiles(t *testing.T) {
	if a, b := framePercentiles(nil); a != 0 || b != 0 {
		t.Fatal("empty sample")
	}
	samples := make([]float64, 100)
	for i := range samples {
		samples[i] = float64(100 - i)
	}
	if a, b := framePercentiles(samples); a != 95 || b != 99 {
		t.Fatalf("percentiles %.1f %.1f", a, b)
	}
}
