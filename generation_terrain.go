package main

import "math"

// height is the first air/water voxel, never the last solid voxel.
type terrainColumn struct {
	height, biomeID int
	top, filler     byte
	ocean           bool
	environment     environmentSample
	slope           float32
}

func sampleTerrainColumn(seed uint32, x, z int) terrainColumn {
	e := sampleEnvironment(seed, x, z)
	// Central differences of unquantized height avoid voxel stair-step artifacts.
	dx := (sampleEnvironment(seed, x+2, z).elevation - sampleEnvironment(seed, x-2, z).elevation) / 4
	dz := (sampleEnvironment(seed, x, z+2).elevation - sampleEnvironment(seed, x, z-2).elevation) / 4
	return terrainColumnFromSamples(seed, x, z, e, dx, dz)
}

func terrainColumnFromSamples(seed uint32, x, z int, e environmentSample, dx, dz float32) terrainColumn {
	slope := float32(math.Sqrt(float64(dx*dx + dz*dz)))
	top, filler := surfaceFromEnvironment(seed, x, z, e, slope)
	return terrainColumn{height: e.height, biomeID: e.biome, top: top, filler: filler, ocean: isOceanBiome(e.biome), environment: e, slope: slope}
}

func (c terrainColumn) topY() int { return max(c.height, int(seaLevel)) }

func (c terrainColumn) blockAt(seed uint32, x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return blockAir
	}
	if y == 0 {
		return blockBedrock
	}
	if y >= c.height {
		if y >= seaLevel {
			return blockAir
		}
		if y == seaLevel-1 && (c.biomeID == BiomeFrozenOcean || c.biomeID == BiomeFrozenRiver || c.biomeID == BiomeTaiga || c.biomeID == BiomeSnowyBeach || c.biomeID == BiomeSnowyTundra) {
			return blockIce
		}
		return blockWater
	}
	if y == c.height-1 {
		return c.top
	}
	if y >= c.height-4 {
		return c.filler
	}
	if caveAt(seed, x, y, z, c) {
		return blockAir
	}
	return blockStone
}

// SplitMix64 avalanche with ordered full-width signed coordinates. Unlike
// shifted XOR packing, negative Z cannot erase the seed or X channel.
func generationHash(seed uint32, x, y, z int, salt uint64) uint64 {
	mix := func(v uint64) uint64 {
		v = (v ^ (v >> 30)) * 0xbf58476d1ce4e5b9
		v = (v ^ (v >> 27)) * 0x94d049bb133111eb
		return v ^ (v >> 31)
	}
	h := mix(uint64(seed) + salt + 0x9e3779b97f4a7c15)
	h = mix(h + uint64(int64(x)) + 0x9e3779b97f4a7c15)
	h = mix(h + uint64(int64(y)) + 0x9e3779b97f4a7c15)
	return mix(h + uint64(int64(z)) + 0x9e3779b97f4a7c15)
}

// Smooth world-coordinate value noise, including across negative lattice cells.
func caveNoise(seed uint32, x, y, z float64, salt uint64) float64 {
	ix, iy, iz := int(math.Floor(x)), int(math.Floor(y)), int(math.Floor(z))
	smooth := func(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }
	u, v, w := smooth(x-float64(ix)), smooth(y-float64(iy)), smooth(z-float64(iz))
	value := 0.0
	for dx := 0; dx <= 1; dx++ {
		for dy := 0; dy <= 1; dy++ {
			for dz := 0; dz <= 1; dz++ {
				a, b, c := 1-u, 1-v, 1-w
				if dx == 1 {
					a = u
				}
				if dy == 1 {
					b = v
				}
				if dz == 1 {
					c = w
				}
				n := float64(generationHash(seed, ix+dx, iy+dy, iz+dz, salt)>>11)/9007199254740992.0*2 - 1
				value += a * b * c * n
			}
		}
	}
	return value
}

func caveAt(seed uint32, x, y, z int, c terrainColumn) bool {
	// Keep four bottom layers, eight surface layers, and all ocean/submerged
	// columns intact. No chunk-local random walk or clipping at chunk edges.
	if y < 4 || y >= c.height-8 || c.ocean || c.height < seaLevel {
		return false
	}
	a := caveNoise(seed, float64(x)/28, float64(y)/20, float64(z)/28, 0xcafe01)
	b := caveNoise(seed, float64(x)/35, float64(y)/25, float64(z)/35, 0xcafe02)
	return math.Abs(a) < 0.09 && math.Abs(b) < 0.13
}

func vegetationBlock(biome int, ground byte, r float32) byte {
	if ground == blockSand && biome == BiomeDesert {
		if r > 0.995 {
			return blockCactus
		}
		if r < 0.01 {
			return blockDeadBush
		}
	}
	if ground != blockGrass {
		return blockAir
	}
	switch biome {
	case BiomePlains, BiomeForest, BiomeBirchForest:
		if r > 0.99 {
			return blockRose
		}
		if r > 0.98 {
			return blockDandelion
		}
		if r > 0.9 {
			return blockTallGrass
		}
	case BiomeTaiga:
		if r > 0.95 {
			return blockTallGrass
		}
	}
	return blockAir
}
