package main

import (
	"gocraft/platform"
	"sync"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

type ChunkMesh struct {
	glMesh   *platform.Mesh
	material rl.Material
	// Cached Uniforms (initialized to -2 to verify, or we can use lazy init)
	// Actually, we can just look it up once if we store it.
	// But simpler: just execute the lookup if it's -1? No, 0 is valid.
	// Let's use -1 as "not found/not initialized" but 0 is a valid loc.
	// We'll init with -2.
	locMVP int32
	locCol int32
	init   bool
}

func (m *ChunkMesh) Draw(shader uint32, viewProj mgl32.Mat4, overrideTextureID uint32) {
	if m.glMesh == nil {
		return
	}

	platform.UseProgram(shader)

	if !m.init {
		m.locMVP = platform.GetUniformLocation(shader, "mvp")
		if m.locMVP == -1 {
			m.locMVP = platform.GetUniformLocation(shader, "matModelViewProjection")
		}
		m.locCol = platform.GetUniformLocation(shader, "colDiffuse")
		m.init = true
	}

	// Upload MVP (using pre-calculated Matrix)
	platform.UniformMatrix4fv(m.locMVP, 1, false, &viewProj[0])

	// Set colDiffuse
	if m.locCol != -1 {
		platform.Uniform4f(m.locCol, 1.0, 1.0, 1.0, 1.0)
	}

	// Bind Texture
	platform.ActiveTexture(platform.GL_TEXTURE0)
	var texID uint32
	if overrideTextureID != 0 {
		texID = overrideTextureID
	} else {
		texID = uint32(m.material.Maps.Texture.ID)
	}
	platform.BindTexture(platform.GL_TEXTURE_2D, texID)

	m.glMesh.Draw()
}

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

func (m *ChunkMesh) unload() {
	if m.glMesh != nil {
		m.glMesh.Unload()
		m.glMesh = nil
	}
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

func (a *RenderAssets) buildAllMeshData(heightMap *[chunkWidth][chunkWidth]int16, baseX, baseZ int, yMin, yMax int, getBlock BlockGetter, getLight LightGetter, seed uint32) map[string]map[string][]*MeshBuildData {
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
				if def.RenderType == RenderTypeTorch {
					continue
				}

				pass := "opaque"
				switch def.RenderType {
				case RenderTypeCutout:
					pass = "cutout"
				case RenderTypeCross:
					pass = "cutout"
				case RenderTypeGlass:
					pass = "glass"
				case RenderTypeLiquid:
					if block == blockLava {
						pass = "opaque" // Lava is opaque enough to write depth and block water behind it
					} else {
						pass = "water"
					}
				}

				textures := def.Textures
				px, py, pz := float32(wx), float32(y), float32(wz)

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
					sx0, sx1 := isOccluding(wx-1, y-1, wz), isOccluding(wx+1, y-1, wz)
					sz0, sz1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y-1, wz+1)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y-1, wz+1)
					c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y-1, wz+1)
					ao := float32(cornerAO(sx0, sz0, c00)+cornerAO(sx0, sz1, c01)+cornerAO(sx1, sz0, c10)+cornerAO(sx1, sz1, c11)) / 12.0
					// Bottom: No tint for Grass Block
					bottomTintCol := tintColor
					if block == blockGrass {
						bottomTintCol = rl.NewColor(0, 0, 0, 0)
					}
					col := a.applyAO(block, bottomTint, ao, false, getLight(wx, y-1, wz), bottomTintCol)

					usePath := textures.Bottom
					uvRect, inAtlas := a.getAtlasUV(textures.Bottom)
					uvs := []float32{0, 0, 0, 1, 1, 1, 1, 0}

					if inAtlas {
						usePath = "atlas"
						// Transform UVs: u' = OriginX + u*Width, v' = OriginY + v*Height
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFace(
						[]float32{px - 0.5, py - 0.5, pz + 0.5, px - 0.5, py - 0.5, pz - 0.5, px + 0.5, py - 0.5, pz - 0.5, px + 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(0, -1, 0),
						uvs, // SW(BL), NW(TL), NE(TR), SE(BR)
						col,
					)
				}
				// NORTH (Z-)
				if a.shouldDrawFace(block, getBlock(wx, y, wz-1)) {
					sx0, sx1 := isOccluding(wx-1, y, wz-1), isOccluding(wx+1, y, wz-1)
					sy0, sy1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y+1, wz-1)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
					c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
					ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0
					// North: No tint for Grass Block sides
					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					col := a.applyAO(block, northTint, ao, false, getLight(wx, y, wz-1), sideTintCol)

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

					getBuilder(pass, usePath).addFace(
						[]float32{px - 0.5, py + 0.5, pz - 0.5, px + 0.5, py + 0.5, pz - 0.5, px + 0.5, py - 0.5, pz - 0.5, px - 0.5, py - 0.5, pz - 0.5},
						rl.NewVector3(0, 0, -1),
						uvs,
						col,
					)

					if block == blockGrass {
						oDisp := float32(0.5) // Exact overlay (Polygon Offset handles depth)
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

						// Smooth Overlay Logic
						// Redefine getCornerColor locally (simplest)
						getCornerColor := func(cx, cz int) rl.Color {
							rs, gs, bs := float32(0), float32(0), float32(0)
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

						// Verts: TL(-1,-1), TR(1,-1), BR(1,-1), BL(-1,-1)
						tLeft := getCornerColor(-1, -1)
						tRight := getCornerColor(1, -1)
						tints := []rl.Color{tLeft, tRight, tRight, tLeft}

						aos := []float32{ao, ao, ao, ao}
						colors := a.applyAOSmooth(block, white, aos, false, getLight(wx, y, wz-1), tints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - 0.5, py + 0.5, pz - oDisp, px + 0.5, py + 0.5, pz - oDisp, px + 0.5, py - 0.5, pz - oDisp, px - 0.5, py - 0.5, pz - oDisp},
							rl.NewVector3(0, 0, -1),
							oUVs,
							colors,
						)
					}
				}
				// SOUTH (Z+)
				if a.shouldDrawFace(block, getBlock(wx, y, wz+1)) {
					sx0, sx1 := isOccluding(wx-1, y, wz+1), isOccluding(wx+1, y, wz+1)
					sy0, sy1 := isOccluding(wx, y-1, wz+1), isOccluding(wx, y+1, wz+1)
					c00, c01 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
					c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
					ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0
					// South: No tint for Grass Block sides
					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					col := a.applyAO(block, southTint, ao, false, getLight(wx, y, wz+1), sideTintCol)

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

					getBuilder(pass, usePath).addFace(
						[]float32{px + 0.5, py + 0.5, pz + 0.5, px - 0.5, py + 0.5, pz + 0.5, px - 0.5, py - 0.5, pz + 0.5, px + 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(0, 0, 1),
						uvs,
						col,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{1, 0, 0, 0, 0, 1, 1, 1} // South UVs usually flipped H
						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						// Smooth Overlay Logic
						getCornerColor := func(cx, cz int) rl.Color {
							rs, gs, bs := float32(0), float32(0), float32(0)
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

						// Verts: TR(1,1), TL(-1,1), BL(-1,1), BR(1,1)
						tRight := getCornerColor(1, 1)
						tLeft := getCornerColor(-1, 1)
						tints := []rl.Color{tRight, tLeft, tLeft, tRight}

						aos := []float32{ao, ao, ao, ao}
						colors := a.applyAOSmooth(block, white, aos, false, getLight(wx, y, wz+1), tints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + 0.5, py + 0.5, pz + oDisp, px - 0.5, py + 0.5, pz + oDisp, px - 0.5, py - 0.5, pz + oDisp, px + 0.5, py - 0.5, pz + oDisp},
							rl.NewVector3(0, 0, 1),
							oUVs,
							colors,
						)
					}
				}
				// EAST (X+)
				if a.shouldDrawFace(block, getBlock(wx+1, y, wz)) {
					sz0, sz1 := isOccluding(wx+1, y, wz-1), isOccluding(wx+1, y, wz+1)
					sy0, sy1 := isOccluding(wx+1, y-1, wz), isOccluding(wx+1, y+1, wz)
					c00, c01 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
					c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
					ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0
					// East: No tint for Grass Block sides
					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					col := a.applyAO(block, eastTint, ao, false, getLight(wx+1, y, wz), sideTintCol)

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

					getBuilder(pass, usePath).addFace(
						[]float32{px + 0.5, py + 0.5, pz - 0.5, px + 0.5, py + 0.5, pz + 0.5, px + 0.5, py - 0.5, pz + 0.5, px + 0.5, py - 0.5, pz - 0.5},
						rl.NewVector3(1, 0, 0),
						uvs,
						col,
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

						// Smooth Overlay Logic
						getCornerColor := func(cx, cz int) rl.Color {
							rs, gs, bs := float32(0), float32(0), float32(0)
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

						// Verts: TL(1,-1), TR(1,1), BR(1,1), BL(1,-1)
						tLeft := getCornerColor(1, -1)
						tRight := getCornerColor(1, 1)
						tints := []rl.Color{tLeft, tRight, tRight, tLeft}

						aos := []float32{ao, ao, ao, ao}
						colors := a.applyAOSmooth(block, white, aos, false, getLight(wx+1, y, wz), tints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + oDisp, py + 0.5, pz - 0.5, px + oDisp, py + 0.5, pz + 0.5, px + oDisp, py - 0.5, pz + 0.5, px + oDisp, py - 0.5, pz - 0.5},
							rl.NewVector3(1, 0, 0),
							oUVs,
							colors,
						)
					}
				}
				// WEST (X-)
				if a.shouldDrawFace(block, getBlock(wx-1, y, wz)) {
					sz0, sz1 := isOccluding(wx-1, y, wz-1), isOccluding(wx-1, y, wz+1)
					sy0, sy1 := isOccluding(wx-1, y-1, wz), isOccluding(wx-1, y+1, wz)
					c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
					c10, c11 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
					ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0
					// West: No tint for Grass Block sides
					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = rl.NewColor(0, 0, 0, 0)
					}
					col := a.applyAO(block, westTint, ao, false, getLight(wx-1, y, wz), sideTintCol)

					usePath := textures.West
					uvRect, inAtlas := a.getAtlasUV(textures.West)
					uvs := []float32{1, 0, 0, 0, 0, 1, 1, 1}

					if inAtlas {
						usePath = "atlas"
						for k := 0; k < 8; k += 2 {
							uvs[k] = uvRect.X + uvs[k]*uvRect.Width
							uvs[k+1] = uvRect.Y + uvs[k+1]*uvRect.Height
						}
					}

					getBuilder(pass, usePath).addFace(
						[]float32{px - 0.5, py + 0.5, pz + 0.5, px - 0.5, py + 0.5, pz - 0.5, px - 0.5, py - 0.5, pz - 0.5, px - 0.5, py - 0.5, pz + 0.5},
						rl.NewVector3(-1, 0, 0),
						uvs,
						col,
					)

					if block == blockGrass {
						oDisp := float32(0.5)
						oPath := "textures/block/grass_block_side_overlay.png"
						oPass := "cutout"

						oUVs := []float32{1, 0, 0, 0, 0, 1, 1, 1} // South UVs usually flipped H? West/East sometimes swap uvs depending on implementation
						// Standard Cube West: (0,0,1,0...) ?
						// West face in my code used {1,0, 0,0, 0,1, 1,1} (flipped H from standard 0,0)
						// I'll copy the UVs from the West block logic above to match orientation.
						oUVs = []float32{1, 0, 0, 0, 0, 1, 1, 1}

						oUVRect, oInAtlas := a.getAtlasUV(oPath)
						if oInAtlas {
							oPath = "atlas"
							for k := 0; k < 8; k += 2 {
								oUVs[k] = oUVRect.X + oUVs[k]*oUVRect.Width
								oUVs[k+1] = oUVRect.Y + oUVs[k+1]*oUVRect.Height
							}
						}

						// Smooth Overlay Logic
						getCornerColor := func(cx, cz int) rl.Color {
							rs, gs, bs := float32(0), float32(0), float32(0)
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

						// Verts: TL(-1,1), TR(-1,-1), BR(-1,-1), BL(-1,1)
						tLeft := getCornerColor(-1, 1)
						tRight := getCornerColor(-1, -1)
						tints := []rl.Color{tLeft, tRight, tRight, tLeft}

						aos := []float32{ao, ao, ao, ao}
						colors := a.applyAOSmooth(block, white, aos, false, getLight(wx-1, y, wz), tints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - oDisp, py + 0.5, pz + 0.5, px - oDisp, py + 0.5, pz - 0.5, px - oDisp, py - 0.5, pz - 0.5, px - oDisp, py - 0.5, pz + 0.5},
							rl.NewVector3(-1, 0, 0),
							oUVs,
							colors,
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
