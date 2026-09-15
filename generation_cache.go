package main

// One generation job owns this cache. A tree halo plus the slope stencil keeps
// all samples in world coordinates, including at negative chunk boundaries.
const generationSampleHalo = treeRadius + 2
const generationSampleSide = chunkWidth + 2*generationSampleHalo

type terrainSampleCache struct {
	seed    uint32
	x, z    int
	samples [generationSampleSide][generationSampleSide]environmentSample
}

func (c *terrainSampleCache) init(seed uint32, cx, cz int) {
	c.seed, c.x, c.z = seed, cx*chunkWidth-generationSampleHalo, cz*chunkWidth-generationSampleHalo
	for x := range c.samples {
		for z := range c.samples[x] {
			c.samples[x][z] = sampleEnvironment(seed, c.x+x, c.z+z)
		}
	}
}

func (c *terrainSampleCache) column(seed uint32, x, z int) terrainColumn {
	ix, iz := x-c.x, z-c.z
	if seed != c.seed || ix < 2 || iz < 2 || ix >= generationSampleSide-2 || iz >= generationSampleSide-2 {
		return sampleTerrainColumn(seed, x, z)
	}
	dx := (c.samples[ix+2][iz].elevation - c.samples[ix-2][iz].elevation) / 4
	dz := (c.samples[ix][iz+2].elevation - c.samples[ix][iz-2].elevation) / 4
	return terrainColumnFromSamples(seed, x, z, c.samples[ix][iz], dx, dz)
}
