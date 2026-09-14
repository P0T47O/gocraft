package main

import "math"

const BiomeExtremeHills = 19
const BiomeRiver = 20
const BiomeFrozenRiver = 21
const (
	regionPlains = iota
	regionForest
	regionDesert
	regionTaiga
	regionSnow
	regionMountain
	regionCount
)

var regionBiomes = [regionCount]int{BiomePlains, BiomeForest, BiomeDesert, BiomeTaiga, BiomeSnowyTundra, BiomeExtremeHills}

type environmentSample struct {
	height, biome                          int
	elevation, land, temperature, humidity float32
	weights                                [regionCount]float32
	riverBank                              float32
}

// Gradient noise avoids interpolated random-height plateaus. Independent salts
// and world coordinates keep results independent of chunk order.
func terrainNoise(seed uint32, x, z float64, salt uint64) float32 {
	ix, iz := int(math.Floor(x)), int(math.Floor(z))
	tx, tz := x-float64(ix), z-float64(iz)
	fade := func(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }
	dot := func(dx, dz int) float64 {
		index := generationHash(seed, ix+dx, 0, iz+dz, salt) % trigTableSize
		return float64(trigCos[index])*(tx-float64(dx)) + float64(trigSin[index])*(tz-float64(dz))
	}
	u, v := fade(tx), fade(tz)
	a, b, c, d := dot(0, 0), dot(1, 0), dot(0, 1), dot(1, 1)
	return float32(((a+(b-a)*u)*(1-v) + (c+(d-c)*u)*v) * 1.4)
}

// Continuous weights, not dominant biome IDs, drive the generation pipeline.
func sampleEnvironment(seed uint32, x, z int) environmentSample {
	xf, zf := float64(x), float64(z)
	wx := xf + float64(terrainNoise(seed, xf/600, zf/600, 1))*65
	wz := zf + float64(terrainNoise(seed, xf/600, zf/600, 2))*65
	t := terrainNoise(seed, wx/1000, wz/1000, 10)
	h := terrainNoise(seed, wx/900, wz/900, 11)
	continental := terrainNoise(seed, wx/1600, wz/1600, 12)
	land := smoothstep(-.35, .20, continental)
	cold := 1 - smoothstep(-.35, -.10, t)
	dry := smoothstep(.08, .35, t) * (1 - smoothstep(-.25, .05, h))
	wood := smoothstep(-.22, .22, h)
	mountains := smoothstep(-.12, .65, terrainNoise(seed, wx/850, wz/850, 13))
	w := [regionCount]float32{}
	w[regionTaiga], w[regionSnow] = cold*wood, cold*(1-wood)
	w[regionDesert] = (1 - cold) * dry
	rest := (1 - cold) * (1 - dry)
	w[regionMountain] = rest * mountains
	w[regionForest] = rest * (1 - mountains) * wood
	w[regionPlains] = rest * (1 - mountains) * (1 - wood)
	rolling := terrainNoise(seed, wx/180, wz/180, 20)
	detail := terrainNoise(seed, wx/43, wz/43, 21)
	ridgeField := terrainNoise(seed, wx/260, wz/260, 22)
	ridge := 1 - float32(math.Sqrt(float64(ridgeField*ridgeField+.025)))
	valleys := terrainNoise(seed, wx/115, wz/115, 23)
	shapes := [regionCount]float32{
		72 + rolling*5 + detail*.6,
		75 + rolling*9 + detail*1.2,
		73 + terrainNoise(seed, wx/95, wz/170, 24)*7 + detail*.5,
		76 + rolling*10 + detail,
		73 + rolling*6 + detail*.5,
		82 + ridge*ridge*64 + valleys*13 + detail*3,
	}
	landHeight := float32(0)
	best := 0
	for i := range w {
		landHeight += w[i] * shapes[i]
		if w[i] > w[best] {
			best = i
		}
	}
	elevation := lerp(35+rolling*6, landHeight, land)
	biome := regionBiomes[best]
	if elevation < seaLevel-2 {
		biome = BiomeOcean
		if cold > .5 {
			biome = BiomeFrozenOcean
		}
	} else if elevation < seaLevel+2 {
		biome = BiomeBeach
		if cold > .5 {
			biome = BiomeSnowyBeach
		}
	}
	// Carve after the regional height blend, never switch entire biome profiles.
	// Sea-level water is deliberate: this is not a flow/erosion simulation.
	baseElevation := elevation
	signedDistance := riverSignedDistance(seed, wx, wz)
	distance := abs(signedDistance)
	channelWidth := 4 + terrainNoise(seed, wx/170, wz/170, 61)*2
	side := float32(1)
	if signedDistance < 0 {
		side = -1
	}
	asymmetry := side * terrainNoise(seed, wx/230, wz/230, 62)
	terraceWidth := channelWidth + 9 + 7*(1+asymmetry)
	width := (terraceWidth + 18 + max(float32(0), elevation-seaLevel)*1.4) * (1 + .3*asymmetry)
	bank := 1 - smoothstep(terraceWidth, width, distance)
	bank *= bank
	// A low irregular shelf interrupts the old single bowl-shaped bank.
	shelf := float32(seaLevel+1.8) + terrainNoise(seed, wx/35, wz/35, 63)*1.2
	bed := float32(seaLevel-4) + terrainNoise(seed, wx/55, wz/55, 64)*1.2
	channel := lerp(bed, shelf, smoothstep(channelWidth*.4, channelWidth+5, distance))
	elevation = min(elevation, lerp(channel, elevation, 1-bank))
	if baseElevation >= seaLevel-2 && bank > 0 && elevation < seaLevel {
		biome = BiomeRiver
		if cold > .5 {
			biome = BiomeFrozenRiver
		}
	}
	return environmentSample{height: int(elevation), elevation: elevation, biome: biome, land: land, temperature: t, humidity: h, weights: w, riverBank: bank}
}

// Distance to a continuous, warped noise contour, approximately in blocks.
// Gradient normalization keeps channel width less sensitive to noise steepness.
func riverSignedDistance(seed uint32, x, z float64) float32 {
	field := func(x, z float64) float32 { return terrainNoise(seed, x/900, z/900, 60) + .12 }
	v := field(x, z)
	dx := (field(x+2, z) - field(x-2, z)) / 4
	dz := (field(x, z+2) - field(x, z-2)) / 4
	gradient := max(float32(.00015), float32(math.Sqrt(float64(dx*dx+dz*dz))))
	return v / gradient
}

// Correlated surface patches rather than independent per-voxel dithering.
// Rock exposure follows slope, never a fixed altitude contour.
func surfaceFromEnvironment(seed uint32, x, z int, e environmentSample, slope float32) (byte, byte) {
	patch := terrainNoise(seed, float64(x)/19, float64(z)/19, 40) * .16
	snow := e.weights[regionTaiga] + e.weights[regionSnow]
	if (e.biome == BiomeRiver || e.biome == BiomeFrozenRiver) && e.height < seaLevel {
		if e.weights[regionDesert] > .5+patch {
			return blockSand, blockSandstone
		}
		return blockGravel, blockDirt
	}
	if e.height < seaLevel-2 {
		if e.height < seaLevel-8 {
			return blockGravel, blockGravel
		}
		return blockSand, blockSand
	}
	coast := (1 - smoothstep(0, 5, abs(e.elevation-seaLevel))) * (1 - e.riverBank)
	sand := max(e.weights[regionDesert], coast*(1-snow))
	rock := smoothstep(.38, .95, slope) * smoothstep(.1, .55, e.weights[regionMountain])
	if rock > .5+patch {
		return blockStone, blockStone
	}
	if snow > .5+patch {
		return blockSnow, blockDirt
	}
	if sand > .5+patch {
		return blockSand, blockSandstone
	}
	if e.height < seaLevel {
		return blockDirt, blockDirt
	}
	return blockGrass, blockDirt
}

func vegetationAt(seed uint32, x, z int, c terrainColumn) byte {
	r := (hash2(seed+4, x, z) + 1) * .5
	if c.top == blockSand && c.environment.weights[regionDesert] > .65 {
		return vegetationBlock(BiomeDesert, c.top, r)
	}
	if c.top != blockGrass {
		return blockAir
	}
	patch := (terrainNoise(seed, float64(x)/28, float64(z)/28, 41) + 1) * .5
	fertility := (1 - c.environment.weights[regionDesert]) * (1 - smoothstep(.3, .8, c.slope))
	density := patch * fertility * (.18 - .08*c.environment.weights[regionForest])
	if r > 1-density {
		if patch > .68 && r > .97 {
			if terrainNoise(seed, float64(x)/60, float64(z)/60, 42) > 0 {
				return blockRose
			}
			return blockDandelion
		}
		return blockTallGrass
	}
	return blockAir
}
