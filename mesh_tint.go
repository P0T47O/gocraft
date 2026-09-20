package main

import "image/color"

// Immutable, shared by all sections and rebuilds of one chunk instance.
type meshTintCache struct {
	foliage, water [chunkWidth][chunkWidth]color.RGBA
	climate        [chunkWidth + 2][chunkWidth + 2]color.RGBA
}

func (a *RenderAssets) buildMeshTintCache(seed uint32, baseX, baseZ int) *meshTintCache {
	c := &meshTintCache{}
	var colors [chunkWidth + 2][chunkWidth + 2][2]color.RGBA
	for x := -1; x <= chunkWidth; x++ {
		for z := -1; z <= chunkWidth; z++ {
			e := sampleEnvironment(seed, baseX+x, baseZ+z)
			for pass := 0; pass < 2; pass++ {
				colors[x+1][z+1][pass] = a.environmentColor(e, pass == 1)
			}
			c.climate[x+1][z+1] = a.getClimateColor(seed, baseX+x, baseZ+z)
		}
	}
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			for pass := 0; pass < 2; pass++ {
				var r, g, b float32
				for dx := -1; dx <= 1; dx++ {
					for dz := -1; dz <= 1; dz++ {
						col := colors[x+dx+1][z+dz+1][pass]
						r += float32(col.R)
						g += float32(col.G)
						b += float32(col.B)
					}
				}
				color := meshColor(uint8(r/9), uint8(g/9), uint8(b/9), 255)
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

func (a *RenderAssets) environmentColor(e environmentSample, water bool) color.RGBA {
	var r, g, b float32
	for i, w := range e.weights {
		cr, cg, cb := a.getBiomeBaseColor(regionBiomes[i], water)
		r += cr * w
		g += cg * w
		b += cb * w
	}
	return meshColor(uint8(r), uint8(g), uint8(b), 255)
}
