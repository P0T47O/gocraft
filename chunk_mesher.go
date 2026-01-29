package main

import (
	"fmt"
	"gocraft/platform"
	"sync"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type MeshBuildData struct {
	vertices  []float32
	normals   []float32
	texcoords []float32
	colors    []uint8
	indices   []uint32
	vertCount int
}

type meshBuilder = MeshBuildData

func (mb *meshBuilder) addFace(v []float32, n rl.Vector3, t []float32, c rl.Color) {
	mb.vertices = append(mb.vertices, v...)
	for i := 0; i < 4; i++ {
		mb.normals = append(mb.normals, n.X, n.Y, n.Z)
		mb.colors = append(mb.colors, c.R, c.G, c.B, c.A)
	}
	mb.texcoords = append(mb.texcoords, t...)
	base := uint32(mb.vertCount)
	mb.indices = append(mb.indices, base, base+1, base+2, base, base+2, base+3)
	mb.vertCount += 4
}

func (mb *meshBuilder) addFaceSmooth(v []float32, n rl.Vector3, t []float32, colors []rl.Color) {
	mb.vertices = append(mb.vertices, v...)
	for i := 0; i < 4; i++ {
		mb.normals = append(mb.normals, n.X, n.Y, n.Z)
		mb.colors = append(mb.colors, colors[i].R, colors[i].G, colors[i].B, colors[i].A)
	}
	mb.texcoords = append(mb.texcoords, t...)
	base := uint32(mb.vertCount)
	mb.indices = append(mb.indices, base, base+1, base+2, base, base+2, base+3)
	mb.vertCount += 4
}

var meshBuilderPool = sync.Pool{
	New: func() interface{} {
		return &MeshBuildData{
			vertices:  make([]float32, 0, 4096),
			normals:   make([]float32, 0, 4096),
			texcoords: make([]float32, 0, 4096),
			colors:    make([]uint8, 0, 4096),
			indices:   make([]uint32, 0, 4096),
		}
	},
}

var interleaveBufferPool = sync.Pool{
	New: func() interface{} {
		return make([]float32, 0, 16384)
	},
}

func (mb *MeshBuildData) Reset() {
	mb.vertices = mb.vertices[:0]
	mb.normals = mb.normals[:0]
	mb.texcoords = mb.texcoords[:0]
	mb.colors = mb.colors[:0]
	mb.indices = mb.indices[:0]
	mb.vertCount = 0
}

func (a *RenderAssets) applyMeshData(data map[string][]*MeshBuildData) map[string][]*ChunkMesh {
	meshes := map[string][]*ChunkMesh{}
	for path, list := range data {
		var tex rl.Texture2D
		if path == "atlas" && a.atlas != nil {
			tex = a.atlas.Texture
		} else {
			tex = a.loadTexture(path)
		}

		for _, d := range list {
			if d.vertCount == 0 {
				d.Reset()
				meshBuilderPool.Put(d)
				continue
			}

			// Indices are already uint32
			indices := d.indices

			// Interleave data: Pos(3), Tex(2), Color(4), Normal(3) -> 12 floats per vertex
			totalFloats := d.vertCount * 12

			// Get buffer from pool
			buffer := interleaveBufferPool.Get().([]float32)
			if cap(buffer) < totalFloats {
				buffer = make([]float32, 0, totalFloats)
			} else {
				buffer = buffer[:0]
			}

			for i := 0; i < d.vertCount; i++ {
				// Pos
				buffer = append(buffer, d.vertices[i*3], d.vertices[i*3+1], d.vertices[i*3+2])
				// Tex
				buffer = append(buffer, d.texcoords[i*2], d.texcoords[i*2+1])
				// Color (uint8 -> float32)
				buffer = append(buffer, float32(d.colors[i*4])/255.0, float32(d.colors[i*4+1])/255.0, float32(d.colors[i*4+2])/255.0, float32(d.colors[i*4+3])/255.0)
				// Normal
				buffer = append(buffer, d.normals[i*3], d.normals[i*3+1], d.normals[i*3+2])
			}

			// Upload to PureGL (Manual Memory Management)
			// Note: UploadMesh likely copies the data, so we can reuse `buffer` immediately
			glMesh := platform.UploadMesh(buffer, indices)

			// Return buffer to pool
			interleaveBufferPool.Put(buffer)

			var material rl.Material
			if path == "atlas" && a.atlas != nil {
				material = a.getMaterial("atlas", tex)
			} else {
				material = a.getMaterial(path, tex)
			}

			meshes[path] = append(meshes[path], &ChunkMesh{
				glMesh:   glMesh,
				material: material,
			})

			// Return builder to pool
			d.Reset()
			meshBuilderPool.Put(d)
		}
	}
	return meshes
}

func (a *RenderAssets) isTransparent(b byte) bool {

	return GetBlock(b).IsTransparent
}

// getBiomeBaseColor returns the raw color for a specific biome.
func (a *RenderAssets) getBiomeBaseColor(biomeID int, isWater bool) (float32, float32, float32) {
	// Water Colors
	if isWater {
		switch biomeID {
		case BiomeFrozenOcean, BiomeSnowyTundra, BiomeIceSpikes:
			return 60, 100, 190 // Frozen/Cold
		case BiomeDesert:
			return 60, 200, 230 // Desert (Cyan-ish)
		case BiomeDeepForest:
			// User requested "Treat as Temperate" but standard Jungle is lighter.
			// Let's stick to Standard Blue for now to match user request "Treat as others".
			return 64, 120, 255
		default:
			return 64, 120, 255 // Standard Water
		}
	}

	// Foliage Colors (Grass/Leaves)
	switch biomeID {
	case BiomeDesert, BiomeSavanna:
		return 191, 183, 85 // Brownish (Dry)
	case BiomeTaiga, BiomeSnowyTundra, BiomeIceSpikes, BiomeFrozenOcean:
		return 128, 180, 151 // Cold Blue-Green
	case BiomeDeepForest:
		// User requested to treat as Temperate (Forest)
		return 121, 192, 90
	case BiomeBirchForest:
		return 136, 187, 103 // Pale Green (for Grass only; Leaves are fixed)
	case BiomeForest, BiomePlains:
		return 121, 192, 90 // Standard Green
	default:
		return 121, 192, 90
	}
}

func (a *RenderAssets) getClimateColor(seed uint32, x, z int) rl.Color {
	temp, hum := getClimate(seed, x, z)

	// Normalize roughly -1..1 to 0..1
	t := (temp + 1.0) * 0.5
	h := (hum + 1.0) * 0.5
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	if h < 0 {
		h = 0
	} else if h > 1 {
		h = 1
	}

	// Simple Bilinear Interpolation of 4 corners of the "Color Map"
	// Cold/Dry (T=0,H=0): Taiga/Tundra (Aqua Grey)
	c00 := rl.NewColor(130, 180, 150, 255)
	// Hot/Dry (T=1,H=0): Desert (Olive Yellow)
	c10 := rl.NewColor(190, 180, 80, 255)
	// Cold/Wet (T=0,H=1): Swamp/ColdForest (Dark Green)
	c01 := rl.NewColor(80, 120, 80, 255) // Dull
	// Hot/Wet (T=1,H=1): Jungle (Vibrant Neon Green)
	c11 := rl.NewColor(60, 230, 40, 255)

	// Lerp H first
	lerpColor := func(a, b rl.Color, fw float32) rl.Color {
		return rl.NewColor(
			uint8(float32(a.R)+float32(int(b.R)-int(a.R))*fw),
			uint8(float32(a.G)+float32(int(b.G)-int(a.G))*fw),
			uint8(float32(a.B)+float32(int(b.B)-int(a.B))*fw),
			255,
		)
	}

	cLow := lerpColor(c00, c10, t)
	cHigh := lerpColor(c01, c11, t)
	final := lerpColor(cLow, cHigh, h)

	return final
}
func (a *RenderAssets) shouldDrawFace(block byte, neighbor byte) bool {
	if neighbor == blockAir {
		return true
	}
	if GetBlock(neighbor).IsTransparent {
		// Ice Logic: Seamless with itself and Water
		if block == blockIce {
			if neighbor == blockIce || neighbor == blockWater {
				return false
			}
			return true
		}
		// Water Logic: Seamless with itself and Ice (and Lava)
		if block == blockWater {
			if neighbor == blockWater || neighbor == blockIce || neighbor == blockLava {
				return false
			}
			return true
		}
		// Glass and Leaves: User requested NO culling, even between identical blocks
		return true
	}
	return false
}

func (a *RenderAssets) applyAO(block byte, col rl.Color, ao float32, useGrass bool, light byte, tintColor rl.Color) rl.Color {
	// Apply Tint if valid (alpha > 0)
	if tintColor.A > 0 {
		col = rl.NewColor(
			uint8(float32(col.R)*float32(tintColor.R)/255.0),
			uint8(float32(col.G)*float32(tintColor.G)/255.0),
			uint8(float32(col.B)*float32(tintColor.B)/255.0),
			col.A, // Keep original alpha (texture) or maybe map's alpha?
		)
		// For water, we usually hardset alpha?
		if block == blockWater {
			col.A = 200
		}
	}

	if block == blockGlass || block == blockIce {
		ao = 0
	}
	f := 1.0 - ao*0.6
	if f < 0.4 {
		f = 0.4
	}
	lightF := 0.1 + (float32(light)/15.0)*0.9
	f *= lightF
	return rl.NewColor(
		uint8(float32(col.R)*f),
		uint8(float32(col.G)*f),
		uint8(float32(col.B)*f),
		col.A,
	)
}

func (a *RenderAssets) applyAOSmooth(block byte, col rl.Color, aos []float32, useGrass bool, light byte, tints []rl.Color) []rl.Color {
	if len(aos) < 4 || len(tints) < 4 {
		// Defensive return + log (or panic with info)
		fmt.Printf("applyAOSmooth: Invalid slice length! aos=%d, tints=%d\n", len(aos), len(tints))
		return []rl.Color{col, col, col, col}
	}
	res := make([]rl.Color, 4)
	lightF := 0.1 + (float32(light)/15.0)*0.9

	for i := 0; i < 4; i++ {
		c := col
		// Apply Tint
		if tints[i].A > 0 {
			c = rl.NewColor(
				uint8(float32(c.R)*float32(tints[i].R)/255.0),
				uint8(float32(c.G)*float32(tints[i].G)/255.0),
				uint8(float32(c.B)*float32(tints[i].B)/255.0),
				c.A,
			)
			if block == blockWater {
				c.A = 200
			}
		}

		// Apply AO
		ao := aos[i]
		if block == blockGlass || block == blockIce {
			ao = 0
		}
		f := 1.0 - ao*0.6
		if f < 0.4 {
			f = 0.4
		}
		f *= lightF

		res[i] = rl.NewColor(
			uint8(float32(c.R)*f),
			uint8(float32(c.G)*f),
			uint8(float32(c.B)*f),
			c.A,
		)
	}
	return res
}

func (a *RenderAssets) buildAllMeshData(heightMap *[chunkWidth][chunkWidth]int16, baseX, baseZ int, yMin, yMax int, getBlock BlockGetter, getLight LightGetter, getMeta MetaGetter, seed uint32) map[string]map[string][]*MeshBuildData {
	white := rl.NewColor(255, 255, 255, 255)
	northTint := rl.NewColor(210, 210, 210, 255)
	southTint := rl.NewColor(225, 225, 225, 255)
	westTint := rl.NewColor(200, 200, 200, 255)
	eastTint := rl.NewColor(190, 190, 190, 255)
	bottomTint := rl.NewColor(140, 140, 140, 255)

	results := map[string]map[string][]*MeshBuildData{
		"opaque": {},
		"cutout": {},
		"glass":  {},
		"water":  {},
	}

	getBuilder := func(pass string, path string) *meshBuilder {
		passMap := results[pass]
		list := passMap[path]
		if len(list) > 0 {
			builder := list[len(list)-1]
			// Conservative batching: Stop well before uint32 limit (practically unlimited for chunks)
			// 100k vertices per mesh is plenty safe.
			if builder.vertCount+4 <= 100000 {
				return builder
			}
		}
		builder := meshBuilderPool.Get().(*MeshBuildData)
		// Ensure it's empty (though Put calls Reset, good to be safe or rely on Reset being called before Put)
		passMap[path] = append(passMap[path], builder)
		return builder
	}

	isOccluding := func(wx, wy, wz int) bool {
		b := getBlock(wx, wy, wz)
		return b != blockAir && !a.isTransparent(b)
	}

	cornerAO := func(side1, side2, corner bool) int {
		if side1 && side2 {
			return 3
		}
		oc := 0
		if side1 {
			oc++
		}
		if side2 {
			oc++
		}
		if corner {
			oc++
		}
		return oc
	}

	// Biome Cache (18x18) to support 3x3 smoothing
	// Map -1..16 -> 0..17
	var biomeCache [18][18]int
	for cx := -1; cx <= chunkWidth; cx++ {
		for cz := -1; cz <= chunkWidth; cz++ {
			biomeCache[cx+1][cz+1] = getBiome(seed, baseX+cx, baseZ+cz)
		}
	}

	// OPTIMIZE: This is the hottest loop in the game. Changes here have massive impact.
	// Consider SIMD optimization or moving to Compute Shaders in the future.
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			// Pre-calculate Smoothed Tint for this column
			// Sample 3x3
			var sumR, sumG, sumB float32
			count := float32(0)

			// Foliage
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					bID := biomeCache[x+1+dx][z+1+dz]
					r, g, b := a.getBiomeBaseColor(bID, false)
					sumR += r
					sumG += g
					sumB += b
					count++
				}
			}
			smoothFoliage := rl.NewColor(uint8(sumR/count), uint8(sumG/count), uint8(sumB/count), 255)

			// Water
			sumR, sumG, sumB = 0, 0, 0
			count = 0
			for dx := -1; dx <= 1; dx++ {
				for dz := -1; dz <= 1; dz++ {
					bID := biomeCache[x+1+dx][z+1+dz]
					r, g, b := a.getBiomeBaseColor(bID, true)
					sumR += r
					sumG += g
					sumB += b
					count++
				}
			}
			smoothWater := rl.NewColor(uint8(sumR/count), uint8(sumG/count), uint8(sumB/count), 255)

			// Fixed Tints
			birchColor := rl.NewColor(128, 167, 85, 255)
			spruceColor := rl.NewColor(97, 153, 97, 255)

			for y := yMin; y < yMax; y++ {
				wx, wz := baseX+x, baseZ+z
				// biomeID variable removed as we use cache/tintColor now

				block := getBlock(wx, y, wz)
				if block == blockAir {
					continue
				}

				// Determine Tint for this block
				tintColor := rl.NewColor(0, 0, 0, 0) // No tint

				if block == blockWater {
					tintColor = smoothWater
				} else if block == blockLeavesBirch {
					tintColor = birchColor // Fixed
				} else if block == blockLeavesSpruce {
					tintColor = spruceColor // Fixed
				} else if block == blockLeaves { // Oak
					tintColor = smoothFoliage
				} else if block == blockGrass || block == blockTallGrass {
					tintColor = smoothFoliage
				}

				def := GetBlock(block)
				textures := def.Textures
				px, py, pz := float32(wx), float32(y), float32(wz)

				// Torch Rendering (History Version)
				if def.RenderType == RenderTypeTorch {
					// Logic restored from render_block.go (DrawModelEx -> addFace)
					light := getLight(wx, y, wz)
					col := a.applyAO(block, white, 0.0, false, light, tintColor)

					meta := getMeta(wx, y, wz)

					// Stem Dimensions (1x1 unit base = 1 pixel scale? No, base mesh is 1 unit)
					// stemScale := rl.NewVector3(0.125, 0.625, 0.125)
					// This means 2x10x2 pixels

					// Rotation Logic
					// Default (Up)
					var rotationAxis rl.Vector3 = rl.NewVector3(0, 1, 0)
					var rotationAngle float32 = 0

					var offset rl.Vector3 = rl.NewVector3(0, -0.04, 0) // Base Y offset

					wallOffset := float32(0.4375)                // 7/16
					tiltAngle := float32(22.5 * 3.14159 / 180.0) // Radians

					switch meta {
					case 1: // North (Z-)
						dir := rl.NewVector3(0, 0, -1)
						offset.Z += wallOffset
						rotationAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
						rotationAngle = tiltAngle
					case 2: // South (Z+)
						dir := rl.NewVector3(0, 0, 1)
						offset.Z -= wallOffset
						rotationAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
						rotationAngle = tiltAngle
					case 3: // West (X-)
						dir := rl.NewVector3(-1, 0, 0)
						offset.X += wallOffset
						rotationAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
						rotationAngle = tiltAngle
					case 4: // East (X+)
						dir := rl.NewVector3(1, 0, 0)
						offset.X -= wallOffset
						rotationAxis = rl.Vector3CrossProduct(rl.NewVector3(0, 1, 0), dir)
						rotationAngle = tiltAngle
					}

					// We need a custom AddFace that transforms vertices
					addTransformedFace := func(builder *meshBuilder, v []float32, vScale rl.Vector3, uvs []float32) {
						// v usually 12 floats (4 vertices * 3 coords)
						transformedV := make([]float32, 12)
						// Create rotation matrix
						mat := rl.MatrixRotate(rotationAxis, rotationAngle)

						for i := 0; i < 4; i++ {
							vx := v[i*3] * vScale.X
							vy := v[i*3+1] * vScale.Y
							vz := v[i*3+2] * vScale.Z

							vec := rl.NewVector3(vx, vy, vz)
							vec = rl.Vector3Transform(vec, mat)

							transformedV[i*3] = px + offset.X + vec.X
							transformedV[i*3+1] = py + offset.Y + vec.Y
							transformedV[i*3+2] = pz + offset.Z + vec.Z
						}

						// Calculate Normal (approximate or rotated)
						// For now simple Up/Side but rotated
						n := rl.NewVector3(0, 1, 0) // Placeholder
						n = rl.Vector3Transform(n, mat)

						builder.addFace(transformedV, n, uvs, col)
					}

					// Textures from RenderAssets meshes
					// We can't access faceMeshes directly here easily unless we export them or copy logic.
					// I'll copy the UV logic from render_assets.go initFaceMeshes.

					// STEM
					stemS := rl.NewVector3(0.125, 0.625, 0.125)

					builder := getBuilder("cutout", textures.North) // Use same texture for all

					// Stem Faces
					// North (Z-)
					addTransformedFace(builder, []float32{-0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5}, stemS, []float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 1.0, 0.4375, 1.0})
					// South (Z+)
					addTransformedFace(builder, []float32{0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5}, stemS, []float32{0.5625, 0.375, 0.4375, 0.375, 0.4375, 1.0, 0.5625, 1.0})
					// East (X+)
					addTransformedFace(builder, []float32{0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5}, stemS, []float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 1.0, 0.4375, 1.0})
					// West (X-)
					addTransformedFace(builder, []float32{-0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5, -0.5, 0.5}, stemS, []float32{0.5625, 0.375, 0.4375, 0.375, 0.4375, 1.0, 0.5625, 1.0})
					// Top
					addTransformedFace(builder, []float32{-0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, -0.5, -0.5, 0.5, -0.5}, stemS, []float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 0.5, 0.4375, 0.5})
					// Bottom
					addTransformedFace(builder, []float32{-0.5, -0.5, -0.5, 0.5, -0.5, -0.5, 0.5, -0.5, 0.5, -0.5, -0.5, 0.5}, stemS, []float32{0.4375, 0.5, 0.5625, 0.5, 0.5625, 0.625, 0.4375, 0.625})

					// FLAME
					// flamePos.Y += stemScale.Y*0.5 + 0.125
					// In local space (relative to stemPos), flame is offset by (0, 0.625*0.5 + 0.125, 0)
					// = 0.3125 + 0.125 = 0.4375

					// But wait, our `offset` variable is the Stem Center.
					// We need to shift the flame relative to that.
					// flameCenterLocal = (0, 0.4375, 0)

					flameOffset := rl.NewVector3(0, 0.4375, 0)

					// Modify addTransformedFace to accept local shift
					addTransformedFaceShifted := func(builder *meshBuilder, v []float32, vScale rl.Vector3, uvs []float32, shift rl.Vector3) {
						transformedV := make([]float32, 12)
						mat := rl.MatrixRotate(rotationAxis, rotationAngle)

						for i := 0; i < 4; i++ {
							// Apply Scale
							vx := v[i*3] * vScale.X
							vy := v[i*3+1] * vScale.Y
							vz := v[i*3+2] * vScale.Z

							// Apply Local Shift (Flame relative to Stem)
							vx += shift.X
							vy += shift.Y
							vz += shift.Z

							// Apply Rotation
							vec := rl.NewVector3(vx, vy, vz)
							vec = rl.Vector3Transform(vec, mat)

							// Translate to World
							transformedV[i*3] = px + offset.X + vec.X
							transformedV[i*3+1] = py + offset.Y + vec.Y
							transformedV[i*3+2] = pz + offset.Z + vec.Z
						}
						// Normal
						n := rl.NewVector3(0, 1, 0)
						n = rl.Vector3Transform(n, mat)
						builder.addFace(transformedV, n, uvs, col)
					}

					flameS := rl.NewVector3(0.5, 0.5, 0.5)

					// Flame Faces
					addTransformedFaceShifted(builder, []float32{-0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5}, flameS, []float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.375, 0.4375, 0.375}, flameOffset)
					addTransformedFaceShifted(builder, []float32{0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5}, flameS, []float32{0.5625, 0.0, 0.4375, 0.0, 0.4375, 0.375, 0.5625, 0.375}, flameOffset)
					addTransformedFaceShifted(builder, []float32{0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5}, flameS, []float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.375, 0.4375, 0.375}, flameOffset)
					addTransformedFaceShifted(builder, []float32{-0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5, -0.5, 0.5}, flameS, []float32{0.5625, 0.0, 0.4375, 0.0, 0.4375, 0.375, 0.5625, 0.375}, flameOffset)
					addTransformedFaceShifted(builder, []float32{-0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, -0.5, -0.5, 0.5, -0.5}, flameS, []float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.125, 0.4375, 0.125}, flameOffset)

					continue
				}

				pass := "opaque"
				switch def.RenderType {
				case RenderTypeCutout, RenderTypeTorch:
					pass = "cutout"
				case RenderTypeCross:
					pass = "cutout"
				case RenderTypeGlass:
					pass = "glass"
				case RenderTypeLiquid:
					if block == blockLava {
						pass = "opaque"
					} else {
						pass = "water"
					}
				}

				if def.RenderType == RenderTypeCross {
					light := getLight(wx, y, wz)
					col := a.applyAO(block, white, 0, false, light, tintColor)

					uvRect, inAtlas := a.getAtlasUV(textures.North)

					// Front UVs
					uvsF := []float32{0, 0, 1, 0, 1, 1, 0, 1}
					// Back UVs (Mirrored)
					uvsB := []float32{1, 0, 0, 0, 0, 1, 1, 1}

					if inAtlas {
						for k := 0; k < 8; k += 2 {
							uvsF[k] = uvRect.X + uvsF[k]*uvRect.Width
							uvsF[k+1] = uvRect.Y + uvsF[k+1]*uvRect.Height
							uvsB[k] = uvRect.X + uvsB[k]*uvRect.Width
							uvsB[k+1] = uvRect.Y + uvsB[k+1]*uvRect.Height
						}
					}

					builder := getBuilder("cutout", "atlas")
					if !inAtlas {
						builder = getBuilder("cutout", textures.North)
					}

					// Plane 1: -0.4,-0.4 to 0.4,0.4
					// Front
					builder.addFace(
						[]float32{
							px - 0.4, py + 0.5, pz - 0.4,
							px + 0.4, py + 0.5, pz + 0.4,
							px + 0.4, py - 0.5, pz + 0.4,
							px - 0.4, py - 0.5, pz - 0.4,
						},
						rl.NewVector3(1, 0, -1),
						uvsF,
						col,
					)
					// Back
					builder.addFace(
						[]float32{
							px + 0.4, py + 0.5, pz + 0.4,
							px - 0.4, py + 0.5, pz - 0.4,
							px - 0.4, py - 0.5, pz - 0.4,
							px + 0.4, py - 0.5, pz + 0.4,
						},
						rl.NewVector3(-1, 0, 1),
						uvsB,
						col,
					)

					// Plane 2: -0.4,0.4 to 0.4,-0.4
					// Front
					builder.addFace(
						[]float32{
							px - 0.4, py + 0.5, pz + 0.4,
							px + 0.4, py + 0.5, pz - 0.4,
							px + 0.4, py - 0.5, pz - 0.4,
							px - 0.4, py - 0.5, pz + 0.4,
						},
						rl.NewVector3(1, 0, 1),
						uvsF,
						col,
					)
					// Back
					builder.addFace(
						[]float32{
							px + 0.4, py + 0.5, pz - 0.4,
							px - 0.4, py + 0.5, pz + 0.4,
							px - 0.4, py - 0.5, pz + 0.4,
							px + 0.4, py - 0.5, pz - 0.4,
						},
						rl.NewVector3(-1, 0, -1),
						uvsB,
						col,
					)
					continue
				}

				// Cactus checks
				if block == blockCactus {
					cactusExt := float32(0.4375)

					// TOP (Y+)
					if a.shouldDrawFace(block, getBlock(wx, y+1, wz)) {
						sx0, sx1 := isOccluding(wx-1, y+1, wz), isOccluding(wx+1, y+1, wz)
						sz0, sz1 := isOccluding(wx, y+1, wz-1), isOccluding(wx, y+1, wz+1)
						c00, c01 := isOccluding(wx-1, y+1, wz-1), isOccluding(wx-1, y+1, wz+1)
						c10, c11 := isOccluding(wx+1, y+1, wz-1), isOccluding(wx+1, y+1, wz+1)
						ao := float32(cornerAO(sx0, sz0, c00)+cornerAO(sx0, sz1, c01)+cornerAO(sx1, sz0, c10)+cornerAO(sx1, sz1, c11)) / 12.0

						col := a.applyAO(block, white, ao, false, getLight(wx, y+1, wz), tintColor)
						// Vanilla: Top is cropped 1..15 (0.0625 .. 0.9375)
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMin, uMin, uMax, uMin, uMax, uMax, uMin, uMax}

						usePath := textures.Top
						uvRect, inAtlas := a.getAtlasUV(textures.Top)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px - cactusExt, py + 0.5, pz + cactusExt, px + cactusExt, py + 0.5, pz + cactusExt, px + cactusExt, py + 0.5, pz - cactusExt, px - cactusExt, py + 0.5, pz - cactusExt},
							rl.NewVector3(0, 1, 0),
							uvs, col,
						)
					}
					// BOTTOM (Y-)
					if a.shouldDrawFace(block, getBlock(wx, y-1, wz)) {
						sx0, sx1 := isOccluding(wx-1, y-1, wz), isOccluding(wx+1, y-1, wz)
						sz0, sz1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y-1, wz+1)
						c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y-1, wz+1)
						c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y-1, wz+1)
						ao := float32(cornerAO(sx0, sz0, c00)+cornerAO(sx0, sz1, c01)+cornerAO(sx1, sz0, c10)+cornerAO(sx1, sz1, c11)) / 12.0

						col := a.applyAO(block, bottomTint, ao, false, getLight(wx, y-1, wz), tintColor)
						// Vanilla: Bottom is cropped 1..15 (0.0625 .. 0.9375)
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMin, uMin, uMax, uMin, uMax, uMax, uMin, uMax}

						usePath := textures.Bottom
						uvRect, inAtlas := a.getAtlasUV(textures.Bottom)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px - cactusExt, py - 0.5, pz - cactusExt, px + cactusExt, py - 0.5, pz - cactusExt, px + cactusExt, py - 0.5, pz + cactusExt, px - cactusExt, py - 0.5, pz + cactusExt},
							rl.NewVector3(0, -1, 0),
							uvs, col,
						)
					}
					// NORTH (Z-)
					if a.shouldDrawFace(block, getBlock(wx, y, wz-1)) {
						sx0, sx1 := isOccluding(wx-1, y, wz-1), isOccluding(wx+1, y, wz-1)
						sy0, sy1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y+1, wz-1)
						c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
						c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
						ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0

						col := a.applyAO(block, northTint, ao, false, getLight(wx, y, wz-1), tintColor)
						uvs := []float32{0, 0, 1, 0, 1, 1, 0, 1}

						usePath := textures.North
						uvRect, inAtlas := a.getAtlasUV(textures.North)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px - cactusExt, py + 0.5, pz - cactusExt, px + cactusExt, py + 0.5, pz - cactusExt, px + cactusExt, py - 0.5, pz - cactusExt, px - cactusExt, py - 0.5, pz - cactusExt},
							rl.NewVector3(0, 0, -1),
							uvs, col,
						)
					}
					// SOUTH (Z+)
					if a.shouldDrawFace(block, getBlock(wx, y, wz+1)) {
						sx0, sx1 := isOccluding(wx-1, y, wz+1), isOccluding(wx+1, y, wz+1)
						sy0, sy1 := isOccluding(wx, y-1, wz+1), isOccluding(wx, y+1, wz+1)
						c00, c01 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
						c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
						ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0

						col := a.applyAO(block, southTint, ao, false, getLight(wx, y, wz+1), tintColor)
						// Sides: Cropped Horizontally (1..15), Full Vertically (0..16)
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMax, 0, uMin, 0, uMin, 1, uMax, 1}

						usePath := textures.South
						uvRect, inAtlas := a.getAtlasUV(textures.South)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px + cactusExt, py + 0.5, pz + cactusExt, px - cactusExt, py + 0.5, pz + cactusExt, px - cactusExt, py - 0.5, pz + cactusExt, px + cactusExt, py - 0.5, pz + cactusExt},
							rl.NewVector3(0, 0, 1),
							uvs, col,
						)
					}
					// EAST (X+)
					if a.shouldDrawFace(block, getBlock(wx+1, y, wz)) {
						sz0, sz1 := isOccluding(wx+1, y, wz-1), isOccluding(wx+1, y, wz+1)
						sy0, sy1 := isOccluding(wx+1, y-1, wz), isOccluding(wx+1, y+1, wz)
						c00, c01 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
						c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
						ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0

						col := a.applyAO(block, eastTint, ao, false, getLight(wx+1, y, wz), tintColor)
						// Sides: Cropped Horizontally (1..15), Full Vertically (0..16)
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMin, 0, uMax, 0, uMax, 1, uMin, 1}

						usePath := textures.East
						uvRect, inAtlas := a.getAtlasUV(textures.East)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px + cactusExt, py + 0.5, pz - cactusExt, px + cactusExt, py + 0.5, pz + cactusExt, px + cactusExt, py - 0.5, pz + cactusExt, px + cactusExt, py - 0.5, pz - cactusExt},
							rl.NewVector3(1, 0, 0),
							uvs, col,
						)
					}
					// WEST (X-)
					if a.shouldDrawFace(block, getBlock(wx-1, y, wz)) {
						sz0, sz1 := isOccluding(wx-1, y, wz-1), isOccluding(wx-1, y, wz+1)
						sy0, sy1 := isOccluding(wx-1, y-1, wz), isOccluding(wx-1, y+1, wz)
						c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
						c10, c11 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
						ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0

						col := a.applyAO(block, westTint, ao, false, getLight(wx-1, y, wz), tintColor)
						// Sides: Cropped Horizontally (1..15), Full Vertically (0..16)
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMax, 0, uMin, 0, uMin, 1, uMax, 1}

						usePath := textures.West
						uvRect, inAtlas := a.getAtlasUV(textures.West)
						if inAtlas {
							usePath = "atlas"
							for k := 0; k < 8; k += 2 {
								uvs[k] = uvRect.X + uvs[k]*uvRect.Width
								uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
							}
						}

						getBuilder(pass, usePath).addFace(
							[]float32{px - cactusExt, py + 0.5, pz + cactusExt, px - cactusExt, py + 0.5, pz - cactusExt, px - cactusExt, py - 0.5, pz - cactusExt, px - cactusExt, py - 0.5, pz + cactusExt},
							rl.NewVector3(-1, 0, 0),
							uvs, col,
						)
					}
					continue
				}

				// TOP (Y+)
				if a.shouldDrawFace(block, getBlock(wx, y+1, wz)) {
					sx0, sx1 := isOccluding(wx-1, y+1, wz), isOccluding(wx+1, y+1, wz)
					sz0, sz1 := isOccluding(wx, y+1, wz-1), isOccluding(wx, y+1, wz+1)
					c00, c01 := isOccluding(wx-1, y+1, wz-1), isOccluding(wx-1, y+1, wz+1)
					c10, c11 := isOccluding(wx+1, y+1, wz-1), isOccluding(wx+1, y+1, wz+1)
					ao0 := float32(cornerAO(sx0, sz1, c01)) / 3.0
					ao1 := float32(cornerAO(sx1, sz1, c11)) / 3.0
					ao2 := float32(cornerAO(sx1, sz0, c10)) / 3.0
					ao3 := float32(cornerAO(sx0, sz0, c00)) / 3.0
					aos := []float32{ao0, ao1, ao2, ao3}

					// Top of Grass Block
					// Only use Tint if it's Grass Block Top
					var colors []rl.Color
					if block == blockGrass {
						// Smooth Biome Blending
						// Calculate 4 vertex colors

						// Helper to get average color of 4 blocks touching a corner
						getCornerColor := func(cx, cz int) rl.Color {
							// cx, cz are directions relative to wx, wz. e.g. -1, -1 for TL
							rs, gs, bs := float32(0), float32(0), float32(0)
							// Helper to add
							add := func(dx, dz int) {
								c := a.getClimateColor(seed, wx+dx, wz+dz)
								rs += float32(c.R)
								gs += float32(c.G)
								bs += float32(c.B)
							}
							add(0, 0)
							add(cx, 0)
							add(0, cz)
							add(cx, cz)
							return rl.NewColor(uint8(rs/4), uint8(gs/4), uint8(bs/4), 255)
						}

						// Vertices match indices:
						// 0: px-0.5, pz+0.5 (Left-Bottom) -> X-1, Z+1
						t0 := getCornerColor(-1, 1)
						// 1: px+0.5, pz+0.5 (Right-Bottom) -> X+1, Z+1
						t1 := getCornerColor(1, 1)
						// 2: px+0.5, pz-0.5 (Right-Top) -> X+1, Z-1
						t2 := getCornerColor(1, -1)
						// 3: px-0.5, pz-0.5 (Left-Top) -> X-1, Z-1
						t3 := getCornerColor(-1, -1)

						tints := []rl.Color{t0, t1, t2, t3}
						colors = a.applyAOSmooth(block, white, aos, true, getLight(wx, y+1, wz), tints)
					} else {
						// Standard Block (Flat Tint)
						// Make 4 copies of tint
						tints := []rl.Color{tintColor, tintColor, tintColor, tintColor}
						colors = a.applyAOSmooth(block, white, aos, true, getLight(wx, y+1, wz), tints)
					}

					usePath := textures.Top
					uvRect, inAtlas := a.getAtlasUV(textures.Top)
					uvs := []float32{0, 0, 1, 0, 1, 1, 0, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px - 0.5, py + 0.5, pz + 0.5, px + 0.5, py + 0.5, pz + 0.5, px + 0.5, py + 0.5, pz - 0.5, px - 0.5, py + 0.5, pz - 0.5},
						rl.NewVector3(0, 1, 0),
						uvs,
						colors,
					)
				}
				// BOTTOM (Y-)
				if a.shouldDrawFace(block, getBlock(wx, y-1, wz)) {
					// Neighbors at Y-1
					sx0, sx1 := isOccluding(wx-1, y-1, wz), isOccluding(wx+1, y-1, wz)
					sz0, sz1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y-1, wz+1)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y-1, wz+1)
					c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y-1, wz+1)

					// 4 Corners: 00(BL), 01(TL), 10(BR), 11(TR) relative to the face plane?
					// Verts are: SW, NW, NE, SE (standard quad winding)
					// SW (X-1, Z+1) -> sx0, sz1, c01
					ao0 := float32(cornerAO(sx0, sz1, c01)) / 3.0
					// NW (X-1, Z-1) -> sx0, sz0, c00
					ao1 := float32(cornerAO(sx0, sz0, c00)) / 3.0
					// NE (X+1, Z-1) -> sx1, sz0, c10
					ao2 := float32(cornerAO(sx1, sz0, c10)) / 3.0
					// SE (X+1, Z+1) -> sx1, sz1, c11
					ao3 := float32(cornerAO(sx1, sz1, c11)) / 3.0

					aos := []float32{ao0, ao1, ao2, ao3}

					bottomTintCol := tintColor
					if block == blockGrass {
						bottomTintCol = rl.NewColor(0, 0, 0, 0)
					}
					// Replicate tint 4 times
					tints := []rl.Color{bottomTintCol, bottomTintCol, bottomTintCol, bottomTintCol}

					colors := a.applyAOSmooth(block, bottomTint, aos, false, getLight(wx, y-1, wz), tints)

					usePath := textures.Bottom
					uvRect, inAtlas := a.getAtlasUV(textures.Bottom)
					uvs := []float32{0, 0, 0, 1, 1, 1, 1, 0}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px - 0.5, py - 0.5, pz + 0.5, px - 0.5, py - 0.5, pz - 0.5, px + 0.5, py - 0.5, pz - 0.5, px + 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(0, -1, 0),
						uvs,
						colors,
					)
				}
				// NORTH (Z-)
				if a.shouldDrawFace(block, getBlock(wx, y, wz-1)) {
					// Neighbors at Z-1
					sx0, sx1 := isOccluding(wx-1, y, wz-1), isOccluding(wx+1, y, wz-1)
					sy0, sy1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y+1, wz-1)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
					c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)

					// Verts: TL(X-1, Y+1), TR(X+1, Y+1), BR(X+1, Y-1), BL(X-1, Y-1)
					// TL: sx0, sy1, c01
					ao0 := float32(cornerAO(sx0, sy1, c01)) / 3.0
					// TR: sx1, sy1, c11
					ao1 := float32(cornerAO(sx1, sy1, c11)) / 3.0
					// BR: sx1, sy0, c10
					ao2 := float32(cornerAO(sx1, sy0, c10)) / 3.0
					// BL: sx0, sy0, c00
					ao3 := float32(cornerAO(sx0, sy0, c00)) / 3.0

					aos := []float32{ao0, ao1, ao2, ao3}

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					tints := []rl.Color{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, northTint, aos, false, getLight(wx, y, wz-1), tints)

					usePath := textures.North
					uvRect, inAtlas := a.getAtlasUV(textures.North)
					uvs := []float32{0, 0, 1, 0, 1, 1, 0, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px - 0.5, py + 0.5, pz - 0.5, px + 0.5, py + 0.5, pz - 0.5, px + 0.5, py - 0.5, pz - 0.5, px - 0.5, py - 0.5, pz - 0.5},
						rl.NewVector3(0, 0, -1),
						uvs,
						colors,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{0, 0, 1, 0, 1, 1, 0, 1}
						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						// Overlay uses Biome Tint
						// Overlay Verts match block verts: TL, TR, BR, BL
						// We need 4 separate cached colors for 4 corners of the FACE?
						// Yes, getCornerColor(dx, dy) relative to face center?
						// Wait, getCornerColor in Top/Bottom used X,Z neighbor average.
						// For Side, we want gradient Y?
						// Actually, Minecraft usually gradients sides based on Y+1 color vs Y-1 color?
						// Or just applies the biome color of (X,Z) to the whole column?
						// In Beta 1.8, sides use block color (X,Z).
						// But with AO, we darken corners.
						// For Overlay Tint: The overlay is "Grass Side".
						// We can use the SAME calculate (X,Z) biome color for all 4 verts, or try to smooth it.
						// Smoothing vertical biome color is overkill.
						// Let's us Flat Tint for Overlay (using X,Z color) + Vertex AO.
						// But previously we used:
						// tLeft := getCornerColor(-1, -1) (on top loop)
						// For side, we are at (X, Z).
						// Let's just use smoothFoliage (calculated at column start) for all 4 overlay verts.
						// It's close enough.

						ovTints := []rl.Color{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, false, getLight(wx, y, wz-1), ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - 0.5, py + 0.5, pz - oDisp, px + 0.5, py + 0.5, pz - oDisp, px + 0.5, py - 0.5, pz - oDisp, px - 0.5, py - 0.5, pz - oDisp},
							rl.NewVector3(0, 0, -1),
							oUVs,
							ovColors,
						)
					}
				}
				// SOUTH (Z+)
				if a.shouldDrawFace(block, getBlock(wx, y, wz+1)) {
					// Neighbors at Z+1
					sx0, sx1 := isOccluding(wx-1, y, wz+1), isOccluding(wx+1, y, wz+1)
					sy0, sy1 := isOccluding(wx, y-1, wz+1), isOccluding(wx, y+1, wz+1)
					c00, c01 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
					c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)

					// Verts: TR(X+1, Y+1), TL(X-1, Y+1), BL(X-1, Y-1), BR(X+1, Y-1)
					// But we define quads:
					// []float32{px + 0.5, py + 0.5, pz + 0.5, (TR)
					//           px - 0.5, py + 0.5, pz + 0.5, (TL)
					//           px - 0.5, py - 0.5, pz + 0.5, (BL)
					//           px + 0.5, py - 0.5, pz + 0.5} (BR)

					// TR: sx1, sy1, c11
					ao0 := float32(cornerAO(sx1, sy1, c11)) / 3.0
					// TL: sx0, sy1, c01
					ao1 := float32(cornerAO(sx0, sy1, c01)) / 3.0
					// BL: sx0, sy0, c00
					ao2 := float32(cornerAO(sx0, sy0, c00)) / 3.0
					// BR: sx1, sy0, c10
					ao3 := float32(cornerAO(sx1, sy0, c10)) / 3.0

					aos := []float32{ao0, ao1, ao2, ao3}

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					tints := []rl.Color{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, southTint, aos, false, getLight(wx, y, wz+1), tints)

					usePath := textures.South
					uvRect, inAtlas := a.getAtlasUV(textures.South)
					uvs := []float32{1, 0, 0, 0, 0, 1, 1, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px + 0.5, py + 0.5, pz + 0.5, px - 0.5, py + 0.5, pz + 0.5, px - 0.5, py - 0.5, pz + 0.5, px + 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(0, 0, 1),
						uvs,
						colors,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{1, 0, 0, 0, 0, 1, 1, 1}
						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						ovTints := []rl.Color{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, false, getLight(wx, y, wz+1), ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + 0.5, py + 0.5, pz + oDisp, px - 0.5, py + 0.5, pz + oDisp, px - 0.5, py - 0.5, pz + oDisp, px + 0.5, py - 0.5, pz + oDisp},
							rl.NewVector3(0, 0, 1),
							oUVs,
							ovColors,
						)
					}
				}
				// EAST (X+)
				if a.shouldDrawFace(block, getBlock(wx+1, y, wz)) {
					// Neighbors at X+1
					sz0, sz1 := isOccluding(wx+1, y, wz-1), isOccluding(wx+1, y, wz+1)
					sy0, sy1 := isOccluding(wx+1, y-1, wz), isOccluding(wx+1, y+1, wz)
					c00, c01 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
					c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)

					// Verts:
					// px + 0.5, py + 0.5, pz - 0.5 (Top Left / near Z-) -> sz0, sy1, c01
					ao0 := float32(cornerAO(sz0, sy1, c01)) / 3.0
					// px + 0.5, py + 0.5, pz + 0.5 (Top Right / near Z+) -> sz1, sy1, c11
					ao1 := float32(cornerAO(sz1, sy1, c11)) / 3.0
					// px + 0.5, py - 0.5, pz + 0.5 (Bottom Right / near Z+) -> sz1, sy0, c10
					ao2 := float32(cornerAO(sz1, sy0, c10)) / 3.0
					// px + 0.5, py - 0.5, pz - 0.5 (Bottom Left / near Z-) -> sz0, sy0, c00
					ao3 := float32(cornerAO(sz0, sy0, c00)) / 3.0

					aos := []float32{ao0, ao1, ao2, ao3}

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					tints := []rl.Color{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, eastTint, aos, false, getLight(wx+1, y, wz), tints)

					usePath := textures.East
					uvRect, inAtlas := a.getAtlasUV(textures.East)
					uvs := []float32{0, 0, 1, 0, 1, 1, 0, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px + 0.5, py + 0.5, pz - 0.5, px + 0.5, py + 0.5, pz + 0.5, px + 0.5, py - 0.5, pz + 0.5, px + 0.5, py - 0.5, pz - 0.5},
						rl.NewVector3(1, 0, 0),
						uvs,
						colors,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{0, 0, 1, 0, 1, 1, 0, 1}
						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						ovTints := []rl.Color{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, false, getLight(wx+1, y, wz), ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + 0.5, py + 0.5, pz - oDisp, px + 0.5, py + 0.5, pz + oDisp, px + 0.5, py - 0.5, pz + oDisp, px + 0.5, py - 0.5, pz - oDisp},
							rl.NewVector3(1, 0, 0),
							oUVs,
							ovColors,
						)
					}
				}
				// WEST (X-)
				if a.shouldDrawFace(block, getBlock(wx-1, y, wz)) {
					// Neighbors at X-1
					sz0, sz1 := isOccluding(wx-1, y, wz-1), isOccluding(wx-1, y, wz+1)
					sy0, sy1 := isOccluding(wx-1, y-1, wz), isOccluding(wx-1, y+1, wz)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
					c10, c11 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)

					// Verts:
					// px - 0.5, py + 0.5, pz + 0.5 (TL / near Z+) -> sz1, sy1, c11
					ao0 := float32(cornerAO(sz1, sy1, c11)) / 3.0
					// px - 0.5, py + 0.5, pz - 0.5 (TR / near Z-) -> sz0, sy1, c01
					ao1 := float32(cornerAO(sz0, sy1, c01)) / 3.0
					// px - 0.5, py - 0.5, pz - 0.5 (BR / near Z-) -> sz0, sy0, c00
					ao2 := float32(cornerAO(sz0, sy0, c00)) / 3.0
					// px - 0.5, py - 0.5, pz + 0.5 (BL / near Z+) -> sz1, sy0, c10
					ao3 := float32(cornerAO(sz1, sy0, c10)) / 3.0

					aos := []float32{ao0, ao1, ao2, ao3}

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					tints := []rl.Color{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, westTint, aos, false, getLight(wx-1, y, wz), tints)

					usePath := textures.West
					uvRect, inAtlas := a.getAtlasUV(textures.West)
					uvs := []float32{0, 0, 1, 0, 1, 1, 0, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFaceSmooth(
						[]float32{px - 0.5, py + 0.5, pz + 0.5, px - 0.5, py + 0.5, pz - 0.5, px - 0.5, py - 0.5, pz - 0.5, px - 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(-1, 0, 0),
						uvs,
						colors,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{0, 0, 1, 0, 1, 1, 0, 1}
						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						ovTints := []rl.Color{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, false, getLight(wx-1, y, wz), ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - 0.5, py + 0.5, pz + oDisp, px - 0.5, py + 0.5, pz - oDisp, px - 0.5, py - 0.5, pz - oDisp, px - 0.5, py - 0.5, pz + oDisp},
							rl.NewVector3(-1, 0, 0),
							oUVs,
							ovColors,
						)
					}
				}
			}
		}
	}
	return results
}

func allocFloat32(data []float32) *float32 {
	if len(data) == 0 {
		return nil
	}
	return (*float32)(unsafe.Pointer(&data[0]))
}

func allocUint8(data []uint8) *uint8 {
	if len(data) == 0 {
		return nil
	}
	return (*uint8)(unsafe.Pointer(&data[0]))
}

func allocUint16(data []uint16) *uint16 {
	if len(data) == 0 {
		return nil
	}
	return (*uint16)(unsafe.Pointer(&data[0]))
}
