package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Immutable, shared by all sections and rebuilds of one chunk instance.
type meshTintCache struct {
	foliage, water [chunkWidth][chunkWidth]rl.Color
	climate        [chunkWidth + 2][chunkWidth + 2]rl.Color
}

func (a *RenderAssets) buildMeshTintCache(seed uint32, baseX, baseZ int) *meshTintCache {
	c := &meshTintCache{}
	var biomes [chunkWidth + 2][chunkWidth + 2]int
	for x := -1; x <= chunkWidth; x++ {
		for z := -1; z <= chunkWidth; z++ {
			biomes[x+1][z+1] = getBiome(seed, baseX+x, baseZ+z)
			c.climate[x+1][z+1] = a.getClimateColor(seed, baseX+x, baseZ+z)
		}
	}
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			for pass := 0; pass < 2; pass++ {
				var r, g, b float32
				for dx := -1; dx <= 1; dx++ {
					for dz := -1; dz <= 1; dz++ {
						cr, cg, cb := a.getBiomeBaseColor(biomes[x+dx+1][z+dz+1], pass == 1)
						r += cr
						g += cg
						b += cb
					}
				}
				color := rl.NewColor(uint8(r/9), uint8(g/9), uint8(b/9), 255)
				if pass == 0 {
					c.foliage[x][z] = color
				} else {
					c.water[x][z] = color
				}
			}
		}
	}
	return c
}
