package main

import (
	"math"
	"math/rand"
	"time"
)

const trigTableSize = 1024

var trigSin [trigTableSize]float32
var trigCos [trigTableSize]float32

func init() {
	for i := 0; i < trigTableSize; i++ {
		angle := float64(i) * 2.0 * math.Pi / float64(trigTableSize)
		trigSin[i] = float32(math.Sin(angle))
		trigCos[i] = float32(math.Cos(angle))
	}
}

type chunkGenJob struct {
	key      chunkKey
	instance uint64
}

type chunkGenResult struct {
	key      chunkKey
	chunk    *Chunk
	instance uint64
}

func (w *World) queueChunkGen(key chunkKey) {
	if w.IsClient {
		return
	}
	select {
	case <-w.done:
		return
	default:
	}
	w.chunksMu.Lock()
	chunk := w.chunks[key]
	if chunk == nil || chunk.generated || w.pending[key] {
		w.chunksMu.Unlock()
		return
	}
	job := chunkGenJob{key: key, instance: chunk.instance}
	select {
	case w.genQueue <- job:
		w.pending[key] = true
	default:
	}
	w.chunksMu.Unlock()
}

func (w *World) ProcessGenResults() {
	deadline := time.Now().Add(3 * time.Millisecond)
	for count := 0; count < 4; count++ {
		if count > 0 && time.Now().After(deadline) {
			return
		}
		select {
		case res := <-w.genResults:
			w.chunksMu.Lock()
			old := w.chunks[res.key]
			if old == nil || old.instance != res.instance || old.generated {
				w.chunksMu.Unlock()
				w.chunkPool.Put(res.chunk)
				continue
			}
			res.chunk.instance = res.instance
			w.chunks[res.key] = res.chunk
			delete(w.pending, res.key)
			w.chunksMu.Unlock()
			w.freeChunk(old)
			w.stitchChunkLighting(res.key.X, res.key.Z)
			w.markChunkAllSectionsDirty(res.key.X, res.key.Z)
			w.markNeighborsDirty(res.key.X, res.key.Z)
			w.applyPendingEdits(res.key)
			perfMon.IncrementChunkLoad()
		default:
			return
		}
	}
}

func (w *World) genWorker() {
	defer w.workers.Done()
	for {
		var job chunkGenJob
		select {
		case <-w.done:
			return
		case job = <-w.genQueue:
		}
		select {
		case <-w.done:
			return
		default:
		}
		chunk := w.chunkPool.Get()
		if w.SavePath == "" || !TryLoadChunk(w.SavePath, chunk, job.key.X, job.key.Z) {
			generateChunkData(w.seed, job.key.X, job.key.Z, chunk)
		}
		chunk.rebuildHeightMap()
		chunk.rebuildTorchCount()
		ensureChunkSections(chunk)
		initializeChunkLighting(chunk)
		chunk.generated = true
		select {
		case w.genResults <- chunkGenResult{key: job.key, chunk: chunk, instance: job.instance}:
		case <-w.done:
			w.chunkPool.Put(chunk)
			return
		}
	}
}

func generateChunkData(seed uint32, cx, cz int, chunk *Chunk) {
	blocks := &chunk.blocks
	heightMap := &chunk.heightMap
	// Pool reuse must not retain air-space blocks from previous coordinates.
	*blocks = [chunkWidth][chunkHeight][chunkWidth]byte{}
	var columns [chunkWidth][chunkWidth]terrainColumn
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			wx, wz := cx*chunkWidth+x, cz*chunkWidth+z
			col := sampleTerrainColumn(seed, wx, wz)
			columns[x][z] = col
			for y := 0; y < col.topY(); y++ {
				blocks[x][y][z] = col.blockAt(seed, wx, y, wz)
			}
			heightMap[x][z] = int16(col.topY())
		}
	}
	randForChunk := func(salt int64) *rand.Rand {
		return rand.New(rand.NewSource(int64(generationHash(seed, cx, 0, cz, uint64(salt)))))
	}
	placeOreVeins := func(rng *rand.Rand, ore byte, tries, size, minY, maxY int, triangular bool) {
		if tries <= 0 || size <= 0 {
			return
		}
		if minY < 1 {
			minY = 1
		}
		if maxY > chunkHeight-1 {
			maxY = chunkHeight - 1
		}
		if minY >= maxY {
			return
		}
		span := maxY - minY
		for t := 0; t < tries; t++ {
			x := rng.Intn(chunkWidth)
			z := rng.Intn(chunkWidth)
			y := minY + rng.Intn(span)
			if triangular {
				half := span / 2
				if half > 0 {
					y = minY + rng.Intn(half) + rng.Intn(half)
					if y >= maxY {
						y = maxY - 1
					}
				}
			}
			idx := rng.Intn(trigTableSize)
			sinA := trigSin[idx]
			cosA := trigCos[idx]
			x1 := float32(x) + sinA*float32(size)/8.0
			x2 := float32(x) - sinA*float32(size)/8.0
			z1 := float32(z) + cosA*float32(size)/8.0
			z2 := float32(z) - cosA*float32(size)/8.0
			y1 := float32(y) + rng.Float32()*float32(size)/16.0
			y2 := float32(y) - rng.Float32()*float32(size)/16.0

			for i := 0; i < size; i++ {
				t := float32(i) / float32(size)
				cx := lerp(x1, x2, t)
				cy := lerp(y1, y2, t)
				cz := lerp(z1, z2, t)
				sinT := float32(math.Sin(float64(t) * math.Pi))
				r := (sinT + 1.0) * rng.Float32() * float32(size) / 16.0
				radius := r + 1.0
				rx := radius / 2.0
				ry := radius / 2.0
				rz := radius / 2.0
				minX := fastFloor(cx - rx)
				maxX := fastFloor(cx + rx)
				minY := fastFloor(cy - ry)
				maxY := fastFloor(cy + ry)
				minZ := fastFloor(cz - rz)
				maxZ := fastFloor(cz + rz)

				for xi := minX; xi <= maxX; xi++ {
					if xi < 0 || xi >= chunkWidth {
						continue
					}
					dx := (float32(xi) + 0.5 - cx) / rx
					dx2 := dx * dx
					if dx2 >= 1.0 {
						continue
					}
					for yi := minY; yi <= maxY; yi++ {
						if yi < 1 || yi >= chunkHeight-1 {
							continue
						}
						dy := (float32(yi) + 0.5 - cy) / ry
						dy2 := dy * dy
						if dx2+dy2 >= 1.0 {
							continue
						}
						for zi := minZ; zi <= maxZ; zi++ {
							if zi < 0 || zi >= chunkWidth {
								continue
							}
							dz := (float32(zi) + 0.5 - cz) / rz
							if dx2+dy2+dz*dz >= 1.0 {
								continue
							}
							if blocks[xi][yi][zi] == blockStone {
								blocks[xi][yi][zi] = ore
							}
						}
					}
				}
			}
		}
	}

	placeOreVeins(randForChunk(0x1001), blockCoalOre, 20, 16, 1, 128, false)
	placeOreVeins(randForChunk(0x1002), blockIronOre, 20, 8, 1, 64, false)
	placeOreVeins(randForChunk(0x1003), blockGoldOre, 2, 8, 1, 32, false)
	placeOreVeins(randForChunk(0x1004), blockDiamondOre, 1, 7, 1, 16, false)
	placeOreVeins(randForChunk(0x1005), blockLapisOre, 1, 6, 1, 32, true)

	placeGeneratedTrees(seed, cx, cz, chunk)
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			col := columns[x][z]
			y := col.height
			if y >= chunkHeight-2 || col.top != blockGrass && col.top != blockSand || blocks[x][y][z] != blockAir || y < seaLevel {
				continue
			}
			wx, wz := cx*chunkWidth+x, cz*chunkWidth+z
			r := (hash2(seed+4, wx, wz) + 1) * 0.5
			b := vegetationBlock(col.biomeID, col.top, r)
			if b != blockAir {
				blocks[x][y][z] = b
				heightMap[x][z] = max(heightMap[x][z], int16(y+1))
				if b == blockCactus && hash2(seed+5, wx, wz) > 0.5 && blocks[x][y+1][z] == blockAir {
					blocks[x][y+1][z] = b
					heightMap[x][z] = max(heightMap[x][z], int16(y+2))
				}
			}
		}
	}
}

func terrainTopY(seed uint32, x, z int) int {
	return sampleTerrainColumn(seed, x, z).topY()
}

// Unloaded-column fallback includes terrain, water/ice and caves. Ores and
// decorations are intentionally materialized only when a chunk is generated.
func blockAtProcedural(seed uint32, x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return blockAir
	}
	return sampleTerrainColumn(seed, x, z).blockAt(seed, x, y, z)
}

// Terrain Parameters
const (
	seaLevel            = 62.0
	continentalBaseFreq = 1.0 / 1200.0
	erosionBaseFreq     = 1.0 / 600.0
	weirdnessBaseFreq   = 1.0 / 400.0 // Peaks & Valleys
	warpFreq            = 1.0 / 400.0
	warpAmp             = 25.0
	detailFreq          = 1.0 / 50.0
)

// Spline function to calculate target height based on noise parameters
func calculateSplineHeight(c, e, pv float32) float32 {
	// 1. Continentalness Base (Ocean vs Land)
	// Deep Ocean -> Shelf -> Coast -> Inland
	offshore := lerp(seaLevel-45.0, seaLevel-10.0, smoothstep(-1.1, -0.4, c))
	inland := lerp(seaLevel+2.0, seaLevel+30.0, smoothstep(0.0, 1.0, c))

	base := lerp(offshore, inland, smoothstep(-0.15, 0.15, c))

	// 2. Erosion (Roughness)
	// -1.0 (Rough/Mountain) -> 1.0 (Flat/Plains)
	// Invert for calculation: 0.0 (Mountain) .. 1.0 (Flat)
	erosionFactor := (e + 1.0) * 0.5

	// 3. Weirdness / Peaks & Valleys (PV)
	// User Logic:
	// - Negative PV: Jagged, sharp peaks.
	// - Positive PV: Shattered, steep terrain (Plateaus/Badlands style).
	// - Near Zero: Rivers / Valleys.

	var terrainShape float32

	if pv < 0 {
		// NEGATIVE WEIRDNESS: Jagged Peaks
		// Logic: Deep valleys at 0, Sharp peaks at -1
		// Use squared/cubed magnitude to sharpen
		val := abs(pv)
		// Sharp peak shape: Rise slowly then spike
		peak := val * val * val    // 0.1->0.001, 0.9->0.73
		terrainShape = peak * 70.0 // Very high peaks
	} else {
		// POSITIVE WEIRDNESS: Shattered / Plateau
		// Logic: Rise quickly to a plateau/terrace
		// Reduce plateau height to 35.0 (was 50.0) to make them less dominant
		val := pv
		// Smoothstep to create a "cliff" effect: 0..0.2 low, 0.2..0.8 steep rise, 0.8..1.0 high flat
		plateau := smoothstep(0.1, 0.4, val)*0.7 + smoothstep(0.6, 0.9, val)*0.3
		terrainShape = plateau * 35.0
	}

	// River / Valley carving (PV near 0)
	// If erosion is high (Flat areas), PV~0 creates rivers
	riverDepth := float32(0.0)
	// Ease the river condition slightly to ensure they appear
	if abs(pv) < 0.1 && erosionFactor > 0.3 {
		// Carve down to slightly below sea level
		t := 1.0 - (abs(pv) / 0.1) // 1.0 at center, 0.0 at edge
		riverDepth = -15.0 * t
	}

	// 4. Final Blend based on Erosion
	// Dampen terrainShape based on Erosion.
	// Previous: smoothstep(0.2, 0.8, erosionFactor)
	// New: smoothstep(0.0, 0.5, erosionFactor)
	// This means anything with Erosion Factor > 0.5 (e > 0.0) is effectively FULL Plains.
	// Erosion Factor < 0.0 (e < -1.0) is full Mountain.
	dampener := 1.0 - smoothstep(0.0, 0.5, erosionFactor)

	// Even in plains, allow slight rolling (pv * 5)
	plainsRolling := pv * 5.0

	finalOffset := lerp(plainsRolling, terrainShape, dampener)

	return base + finalOffset + riverDepth
}

func terrainHeight(seed uint32, x, z int) int {
	return sampleTerrainColumn(seed, x, z).height
}

func rawTerrainHeight(seed uint32, x, z int) int {
	height, _ := terrainShapeSample(seed, x, z)
	return height
}

func terrainShapeSample(seed uint32, x, z int) (int, float32) {
	xf := float32(x)
	zf := float32(z)

	// 1. Domain Warping
	qX := fbm2(seed, xf*warpFreq, zf*warpFreq)
	qZ := fbm2(seed+1, xf*warpFreq, zf*warpFreq)
	warpX := xf + qX*warpAmp
	warpZ := zf + qZ*warpAmp

	// 2. Sample Noise Channels
	// Continentalness (Large Scale)
	cont := fbm2(seed+2, warpX*continentalBaseFreq, warpZ*continentalBaseFreq)

	// Erosion (Medium Scale)
	erosion := fbm2(seed+3, warpX*erosionBaseFreq, warpZ*erosionBaseFreq)

	// Weirdness / Peaks&Valleys (Small Scale)
	pv := fbm2(seed+4, warpX*weirdnessBaseFreq, warpZ*weirdnessBaseFreq)

	// 3. Compute Target Height via Spline
	targetHeight := calculateSplineHeight(cont, erosion, pv)

	// 4. Add Micro-Detail
	detail := fbm2(seed+5, xf*0.03, zf*0.03) * 3.0

	finalHeight := targetHeight + detail

	// 5. Clamping Strictness
	// Only clamp deeply inland areas to ensure buildable flat lands
	// Allow coast and near-coast to slope naturally
	if cont > 0.3 {
		if finalHeight < seaLevel {
			finalHeight = seaLevel
		}
	}

	return int(finalHeight), cont
}

const (
	// Biome IDs
	BiomeOcean       = 0
	BiomeDeepOcean   = 1
	BiomeFrozenOcean = 2
	BiomeBeach       = 3
	BiomeStoneBeach  = 4
	BiomeSnowyBeach  = 5
	BiomeForest      = 10
	BiomeDeepForest  = 11
	BiomeBirchForest = 12
	BiomePlains      = 13
	BiomeSavanna     = 14
	BiomeDesert      = 15
	BiomeTaiga       = 16
	BiomeSnowyTundra = 17
	BiomeIceSpikes   = 18
)

type BiomeParams struct {
	ID          int
	Temperature float32 // -1.0 (Cold) to 1.0 (Hot)
	Humidity    float32 // -1.0 (Dry) to 1.0 (Wet)
	Scale       float32
	Effect      string
}

// Noise Frequencies for Biomes (Low frequency for large zones)
const (
	tempFreq = 1.0 / 800.0
	humFreq  = 1.0 / 800.0
)

func getClimate(seed uint32, x, z int) (float32, float32) {
	xf := float32(x)
	zf := float32(z)
	temp := fbm2(seed+10, xf*tempFreq, zf*tempFreq)
	hum := fbm2(seed+11, xf*humFreq, zf*humFreq)
	// Clamp roughly to -1..1 or just return raw?
	// Raw is fine, typical range -1.2 to 1.2
	return temp, hum
}

func getBiome(seed uint32, x, z int) int {
	xf := float32(x)
	zf := float32(z)

	// 1. Continentalness (Controls Ocean/Land)
	cont := fbm2(seed+2, xf*continentalBaseFreq, zf*continentalBaseFreq)
	// Cont: -1.0 (Deep Ocean) .. 1.0 (Inland)

	// 2. Temperature (Controls Cold/Hot)
	temp := fbm2(seed+10, xf*tempFreq, zf*tempFreq)

	// 3. Humidity (Controls Dry/Wet)
	hum := fbm2(seed+11, xf*humFreq, zf*humFreq)

	// --- Ocean Logic ---
	if cont < -0.25 {
		if cont < -0.6 {
			// Deep Ocean
			return BiomeDeepOcean
		}
		if temp < -0.5 {
			return BiomeFrozenOcean
		}
		return BiomeOcean
	}

	// --- Land Logic (Temperature/Humidity Grid) ---

	// Normalize Temp/Hum roughly to -1..1 range if noise is standard
	// Our fbm2 returns roughly -1..1 or slightly more.

	if temp < -0.4 {
		// COLD BIOMES
		if hum < -0.4 {
			return BiomeIceSpikes
		} else if hum > 0.4 {
			return BiomeTaiga
		}
		return BiomeSnowyTundra
	} else if temp > 0.4 {
		// HOT BIOMES
		if hum < -0.4 {
			return BiomeDesert
		} else if hum > 0.4 {
			return BiomeDeepForest // Jungle
		}
		return BiomeSavanna // or Plains
	} else {
		// TEMPERATE BIOMES
		if hum < -0.3 {
			return BiomePlains
		} else if hum > 0.6 {
			// Prefer Birch in slightly wetter temperate, Oak in mid
			return BiomeBirchForest
		}
		return BiomeForest
	}
}

// classifyBiomeFromNoise determines biome from pre-computed noise values,
// avoiding redundant fbm2 calls when noise is already available.
func classifyBiomeFromNoise(cont, temp, hum float32) int {
	if cont < -0.25 {
		if cont < -0.6 {
			return BiomeDeepOcean
		}
		if temp < -0.5 {
			return BiomeFrozenOcean
		}
		return BiomeOcean
	}
	if temp < -0.4 {
		if hum < -0.4 {
			return BiomeIceSpikes
		} else if hum > 0.4 {
			return BiomeTaiga
		}
		return BiomeSnowyTundra
	} else if temp > 0.4 {
		if hum < -0.4 {
			return BiomeDesert
		} else if hum > 0.4 {
			return BiomeDeepForest
		}
		return BiomeSavanna
	} else {
		if hum < -0.3 {
			return BiomePlains
		} else if hum > 0.6 {
			return BiomeBirchForest
		}
		return BiomeForest
	}
}

// Helper to determine if a biome is generally water/ocean
func isOceanBiome(biome int) bool {
	return biome == BiomeOcean || biome == BiomeDeepOcean || biome == BiomeFrozenOcean
}

// Keep oceanWeight for terrain spline (using Continentalness directly there instead of biome ID)
// But biomeValue was used.
// We should update biomeValue to return just continentalness for terrain shape calculation
// OR update terrainHeight to use continentalness directly.
// Let's repurpose biomeValue to actually return Continentalness for now to minimize diffs in terrainHeight,
// but `getBiome` will be used for block placement.
func getContinentalness(seed uint32, x, z int) float32 {
	xf := float32(x)
	zf := float32(z)
	cont := fbm2(seed+2, xf*continentalBaseFreq, zf*continentalBaseFreq)
	return cont
}

func fbm2(seed uint32, x, z float32) float32 {
	value := float32(0.0)
	amp := float32(1.0)
	freq := float32(1.0)
	for i := 0; i < 4; i++ {
		value += noise2(seed, x*freq, z*freq) * amp
		amp *= 0.5
		freq *= 2.0
	}
	// The terrain spline and climate thresholds consume [-1,1]. Four raw
	// octaves sum to 1.875 and could otherwise flatten entire regions at Y=255.
	return value / 1.875
}

func noise2(seed uint32, x, z float32) float32 {
	x0 := fastFloor(x)
	z0 := fastFloor(z)
	x1 := x0 + 1
	z1 := z0 + 1
	tx := x - float32(x0)
	tz := z - float32(z0)
	u := fade(tx)
	v := fade(tz)
	n00 := hash2(seed, x0, z0)
	n10 := hash2(seed, x1, z0)
	n01 := hash2(seed, x0, z1)
	n11 := hash2(seed, x1, z1)
	nx0 := lerp(n00, n10, u)
	nx1 := lerp(n01, n11, u)
	return lerp(nx0, nx1, v)
}

func hash2(seed uint32, x, z int) float32 {
	n := uint32(x)*73856093 ^ uint32(z)*19349663 ^ seed ^ 0x9e3779b9
	n ^= n >> 15
	n *= 0x27d4eb2d
	n ^= n >> 15
	return float32(n)/2147483647.5 - 1.0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func fade(t float32) float32 {
	return t * t * (3 - 2*t)
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

func smoothstep(edge0, edge1, x float32) float32 {
	if edge0 == edge1 {
		return 0
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func ridge(v float32) float32 {
	av := v
	if av < 0 {
		av = -av
	}
	return 1.0 - av
}

func smoothCurve(h, base, scale float32) float32 {
	d := (h - base) / scale
	return base + d*scale*0.75 + float32(math.Tanh(float64(d)))*scale*0.25
}

const (
	biomeOcean = iota
	biomePlains
	biomeMountains
)

func classifyBiome(v float64) int {
	switch {
	case v < 0.35:
		return biomeOcean
	case v < 0.65:
		return biomePlains
	default:
		return biomeMountains
	}
}

func fastFloor(x float32) int {
	i := int(x)
	if float32(i) > x {
		return i - 1
	}
	return i
}
