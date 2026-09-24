package main

import (
	"sync"
	"unsafe"

	"image/color"
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

func (mb *meshBuilder) addFace(v []float32, n gameVec3, t []float32, c color.RGBA) {
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

func (mb *meshBuilder) addFaceSmooth(v []float32, n gameVec3, t []float32, colors []color.RGBA) {
	mb.vertices = append(mb.vertices, v...)
	for i := 0; i < 4; i++ {
		mb.normals = append(mb.normals, n.X, n.Y, n.Z)
		mb.colors = append(mb.colors, colors[i].R, colors[i].G, colors[i].B, colors[i].A)
	}
	mb.texcoords = append(mb.texcoords, t...)
	base := uint32(mb.vertCount)
	if flipLightDiagonal(colors) {
		mb.indices = append(mb.indices, base, base+1, base+3, base+1, base+2, base+3)
	} else {
		mb.indices = append(mb.indices, base, base+1, base+2, base, base+2, base+3)
	}
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

func (mb *MeshBuildData) Reset() {
	mb.vertices = mb.vertices[:0]
	mb.normals = mb.normals[:0]
	mb.texcoords = mb.texcoords[:0]
	mb.colors = mb.colors[:0]
	mb.indices = mb.indices[:0]
	mb.vertCount = 0
}

func (a *RenderAssets) isTransparent(b byte) bool {

	return GetBlock(b).IsTransparent
}

// getBiomeBaseColor returns the raw color for a specific biome.
func (a *RenderAssets) getBiomeBaseColor(biomeID int, isWater bool) (float32, float32, float32) {
	// Water Colors
	if isWater {
		switch biomeID {
		case BiomeFrozenOcean, BiomeSnowyTundra, BiomeIceSpikes, BiomeTaiga, BiomeSnowyBeach:
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
	case BiomeTaiga, BiomeSnowyTundra, BiomeIceSpikes, BiomeFrozenOcean, BiomeSnowyBeach:
		return 128, 180, 151 // Cold Blue-Green
	case BiomeExtremeHills:
		return 112, 158, 112 // Muted mountain foliage
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

func (a *RenderAssets) getClimateColor(seed uint32, x, z int) color.RGBA {
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
	c00 := meshColor(130, 180, 150, 255)
	// Hot/Dry (T=1,H=0): Desert (Olive Yellow)
	c10 := meshColor(190, 180, 80, 255)
	// Cold/Wet (T=0,H=1): Swamp/ColdForest (Dark Green)
	c01 := meshColor(80, 120, 80, 255) // Dull
	// Hot/Wet (T=1,H=1): Jungle (Vibrant Neon Green)
	c11 := meshColor(60, 230, 40, 255)

	// Lerp H first
	lerpColor := func(a, b color.RGBA, fw float32) color.RGBA {
		return meshColor(
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

func (a *RenderAssets) applyAO(block byte, col color.RGBA, ao float32, useGrass bool, light byte, tintColor color.RGBA) color.RGBA {
	// Apply Tint if valid (alpha > 0)
	if tintColor.A > 0 {
		col = meshColor(
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
	return meshColor(
		uint8(float32(col.R)*f),
		uint8(float32(col.G)*f),
		uint8(float32(col.B)*f),
		col.A,
	)
}

func (a *RenderAssets) buildAllMeshData(heightMap *[chunkWidth][chunkWidth]int16, baseX, baseZ int, yMin, yMax int, getBlock BlockGetter, getLight LightGetter, getMeta MetaGetter, seed uint32, cachedTints ...*meshTintCache) map[string]map[string][]*MeshBuildData {
	white := meshColor(255, 255, 255, 255)
	northTint := meshColor(210, 210, 210, 255)
	southTint := meshColor(225, 225, 225, 255)
	westTint := meshColor(200, 200, 200, 255)
	eastTint := meshColor(190, 190, 190, 255)
	bottomTint := meshColor(140, 140, 140, 255)

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

	var tintCache *meshTintCache
	if len(cachedTints) > 0 {
		tintCache = cachedTints[0]
	}
	if tintCache == nil {
		tintCache = a.buildMeshTintCache(seed, baseX, baseZ)
	}

	// OPTIMIZE: This is the hottest loop in the game. Changes here have massive impact.
	// Consider SIMD optimization or moving to Compute Shaders in the future.
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			smoothFoliage := tintCache.foliage[x][z]
			smoothWater := tintCache.water[x][z]

			// Fixed Tints
			birchColor := meshColor(128, 167, 85, 255)
			spruceColor := meshColor(97, 153, 97, 255)

			for y := yMin; y < yMax; y++ {
				wx, wz := baseX+x, baseZ+z
				// biomeID variable removed as we use cache/tintColor now

				block := getBlock(wx, y, wz)
				if block == blockAir {
					continue
				}

				// Determine Tint for this block
				tintColor := meshColor(0, 0, 0, 0) // No tint

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

				// Torch geometry is centered inside its block and submitted through
				// the same atlas batch as the rest of the cutout world mesh.
				if def.RenderType == RenderTypeTorch {
					light := getLight(wx, y, wz)
					col := a.applyAO(block, white, 0.0, false, light, tintColor)

					meta := getMeta(wx, y, wz)
					geometry := geometryForTorch(meta)
					offset, stemS, flameS, flameOffset := geometry.offset, geometry.stemSize, geometry.flameSize, geometry.flameOffset
					rotation := meshRotation(geometry.axis, geometry.angle)

					uvRect, inAtlas := a.getAtlasUV(textures.North)
					path := textures.North
					if inAtlas {
						path = "atlas"
					}
					torchUV := func(uvs []float32) []float32 {
						if inAtlas {
							for i := 0; i < len(uvs); i += 2 {
								uvs[i] = uvRect.X + uvs[i]*uvRect.Width
								uvs[i+1] = uvRect.Y + uvs[i+1]*uvRect.Height
							}
						}
						return uvs
					}
					torchNormal := func(vertices []float32) gameVec3 {
						edgeA := meshVec3(vertices[3]-vertices[0], vertices[4]-vertices[1], vertices[5]-vertices[2])
						edgeB := meshVec3(vertices[6]-vertices[0], vertices[7]-vertices[1], vertices[8]-vertices[2])
						return gameVec3Normalize(meshCross(edgeA, edgeB))
					}
					addTransformedFace := func(builder *meshBuilder, v []float32, vScale gameVec3, uvs []float32) {
						// v usually 12 floats (4 vertices * 3 coords)
						transformedV := make([]float32, 12)
						// Create rotation matrix

						for i := 0; i < 4; i++ {
							vx := v[i*3] * vScale.X
							vy := v[i*3+1] * vScale.Y
							vz := v[i*3+2] * vScale.Z

							vec := meshVec3(vx, vy, vz)
							vec = meshTransform(vec, rotation)

							transformedV[i*3] = px + offset.X + vec.X
							transformedV[i*3+1] = py + offset.Y + vec.Y
							transformedV[i*3+2] = pz + offset.Z + vec.Z
						}

						builder.addFace(transformedV, torchNormal(transformedV), torchUV(uvs), col)
					}

					// STEM
					builder := getBuilder("cutout", path)

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

					// The flame begins where the stem ends and fits the block.
					addTransformedFaceShifted := func(builder *meshBuilder, v []float32, vScale gameVec3, uvs []float32, shift gameVec3) {
						transformedV := make([]float32, 12)

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
							vec := meshVec3(vx, vy, vz)
							vec = meshTransform(vec, rotation)

							// Translate to World
							transformedV[i*3] = px + offset.X + vec.X
							transformedV[i*3+1] = py + offset.Y + vec.Y
							transformedV[i*3+2] = pz + offset.Z + vec.Z
						}
						builder.addFace(transformedV, torchNormal(transformedV), torchUV(uvs), col)
					}

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
						meshVec3(1, 0, -1),
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
						meshVec3(-1, 0, 1),
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
						meshVec3(1, 0, 1),
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
						meshVec3(-1, 0, -1),
						uvsB,
						col,
					)
					continue
				}

				// Cactus checks
				if block == blockCactus {
					cactusExt := float32(0.4375)
					// Side planes lie inside this voxel. A solid neighbor must
					// not force their lighting to that neighbor's dark interior.
					cactusLight := getLight(wx, y, wz)

					// TOP (Y+)
					if above := getBlock(wx, y+1, wz); above != blockCactus && a.shouldDrawFace(block, above) {
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
							meshVec3(0, 1, 0),
							uvs, col,
						)
					}
					// BOTTOM (Y-)
					if below := getBlock(wx, y-1, wz); below != blockCactus && a.shouldDrawFace(block, below) {
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
							meshVec3(0, -1, 0),
							uvs, col,
						)
					}
					// The side planes are inset by 1/16. Even a full opaque
					// neighbor cannot occlude them, so emit all four sides.
					// NORTH (Z-)
					{
						sx0, sx1 := isOccluding(wx-1, y, wz-1), isOccluding(wx+1, y, wz-1)
						sy0, sy1 := isOccluding(wx, y-1, wz-1), isOccluding(wx, y+1, wz-1)
						c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
						c10, c11 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
						ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0

						col := a.applyAO(block, northTint, ao, false, max(cactusLight, getLight(wx, y, wz-1)), tintColor)
						// Match the same 1..15 texel crop as the other sides.
						uMin, uMax := float32(0.0625), float32(0.9375)
						uvs := []float32{uMin, 0, uMax, 0, uMax, 1, uMin, 1}

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
							meshVec3(0, 0, -1),
							uvs, col,
						)
					}
					// SOUTH (Z+)
					{
						sx0, sx1 := isOccluding(wx-1, y, wz+1), isOccluding(wx+1, y, wz+1)
						sy0, sy1 := isOccluding(wx, y-1, wz+1), isOccluding(wx, y+1, wz+1)
						c00, c01 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
						c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
						ao := float32(cornerAO(sx0, sy0, c00)+cornerAO(sx0, sy1, c01)+cornerAO(sx1, sy0, c10)+cornerAO(sx1, sy1, c11)) / 12.0

						col := a.applyAO(block, southTint, ao, false, max(cactusLight, getLight(wx, y, wz+1)), tintColor)
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
							meshVec3(0, 0, 1),
							uvs, col,
						)
					}
					// EAST (X+)
					{
						sz0, sz1 := isOccluding(wx+1, y, wz-1), isOccluding(wx+1, y, wz+1)
						sy0, sy1 := isOccluding(wx+1, y-1, wz), isOccluding(wx+1, y+1, wz)
						c00, c01 := isOccluding(wx+1, y-1, wz-1), isOccluding(wx+1, y+1, wz-1)
						c10, c11 := isOccluding(wx+1, y-1, wz+1), isOccluding(wx+1, y+1, wz+1)
						ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0

						col := a.applyAO(block, eastTint, ao, false, max(cactusLight, getLight(wx+1, y, wz)), tintColor)
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
							meshVec3(1, 0, 0),
							uvs, col,
						)
					}
					// WEST (X-)
					{
						sz0, sz1 := isOccluding(wx-1, y, wz-1), isOccluding(wx-1, y, wz+1)
						sy0, sy1 := isOccluding(wx-1, y-1, wz), isOccluding(wx-1, y+1, wz)
						c00, c01 := isOccluding(wx-1, y-1, wz-1), isOccluding(wx-1, y+1, wz-1)
						c10, c11 := isOccluding(wx-1, y-1, wz+1), isOccluding(wx-1, y+1, wz+1)
						ao := float32(cornerAO(sz0, sy0, c00)+cornerAO(sz0, sy1, c01)+cornerAO(sz1, sy0, c10)+cornerAO(sz1, sy1, c11)) / 12.0

						col := a.applyAO(block, westTint, ao, false, max(cactusLight, getLight(wx-1, y, wz)), tintColor)
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
							meshVec3(-1, 0, 0),
							uvs, col,
						)
					}
					continue
				}

				// TOP (Y+)
				if a.shouldDrawFace(block, getBlock(wx, y+1, wz)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightTop, getBlock, getLight)

					// Top of Grass Block
					// Only use Tint if it's Grass Block Top
					var colors []color.RGBA
					if block == blockGrass {
						// Smooth Biome Blending
						// Calculate 4 vertex colors

						// Helper to get average color of 4 blocks touching a corner
						getCornerColor := func(cx, cz int) color.RGBA {
							// cx, cz are directions relative to wx, wz. e.g. -1, -1 for TL
							rs, gs, bs := float32(0), float32(0), float32(0)
							// Helper to add
							add := func(dx, dz int) {
								c := tintCache.climate[x+dx+1][z+dz+1]
								rs += float32(c.R)
								gs += float32(c.G)
								bs += float32(c.B)
							}
							add(0, 0)
							add(cx, 0)
							add(0, cz)
							add(cx, cz)
							return meshColor(uint8(rs/4), uint8(gs/4), uint8(bs/4), 255)
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

						tints := []color.RGBA{t0, t1, t2, t3}
						colors = a.applyAOSmooth(block, white, aos, lights, tints)
					} else {
						// Standard Block (Flat Tint)
						// Make 4 copies of tint
						tints := []color.RGBA{tintColor, tintColor, tintColor, tintColor}
						colors = a.applyAOSmooth(block, white, aos, lights, tints)
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
						meshVec3(0, 1, 0),
						uvs,
						colors,
					)
				}
				// BOTTOM (Y-)
				if a.shouldDrawFace(block, getBlock(wx, y-1, wz)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightBottom, getBlock, getLight)

					bottomTintCol := tintColor
					if block == blockGrass {
						bottomTintCol = meshColor(0, 0, 0, 0)
					}
					// Replicate tint 4 times
					tints := []color.RGBA{bottomTintCol, bottomTintCol, bottomTintCol, bottomTintCol}

					colors := a.applyAOSmooth(block, bottomTint, aos, lights, tints)

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
						meshVec3(0, -1, 0),
						uvs,
						colors,
					)
				}
				// NORTH (Z-)
				if a.shouldDrawFace(block, getBlock(wx, y, wz-1)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightNorth, getBlock, getLight)

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = meshColor(0, 0, 0, 0)
					}
					tints := []color.RGBA{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, northTint, aos, lights, tints)

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
						meshVec3(0, 0, -1),
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

						ovTints := []color.RGBA{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, lights, ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - 0.5, py + 0.5, pz - oDisp, px + 0.5, py + 0.5, pz - oDisp, px + 0.5, py - 0.5, pz - oDisp, px - 0.5, py - 0.5, pz - oDisp},
							meshVec3(0, 0, -1),
							oUVs,
							ovColors,
						)
					}
				}
				// SOUTH (Z+)
				if a.shouldDrawFace(block, getBlock(wx, y, wz+1)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightSouth, getBlock, getLight)

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = meshColor(0, 0, 0, 0)
					}
					tints := []color.RGBA{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, southTint, aos, lights, tints)

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
						meshVec3(0, 0, 1),
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

						ovTints := []color.RGBA{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, lights, ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + 0.5, py + 0.5, pz + oDisp, px - 0.5, py + 0.5, pz + oDisp, px - 0.5, py - 0.5, pz + oDisp, px + 0.5, py - 0.5, pz + oDisp},
							meshVec3(0, 0, 1),
							oUVs,
							ovColors,
						)
					}
				}
				// EAST (X+)
				if a.shouldDrawFace(block, getBlock(wx+1, y, wz)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightEast, getBlock, getLight)

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = meshColor(0, 0, 0, 0)
					}
					tints := []color.RGBA{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, eastTint, aos, lights, tints)

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
						meshVec3(1, 0, 0),
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

						ovTints := []color.RGBA{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, lights, ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px + 0.5, py + 0.5, pz - oDisp, px + 0.5, py + 0.5, pz + oDisp, px + 0.5, py - 0.5, pz + oDisp, px + 0.5, py - 0.5, pz - oDisp},
							meshVec3(1, 0, 0),
							oUVs,
							ovColors,
						)
					}
				}
				// WEST (X-)
				if a.shouldDrawFace(block, getBlock(wx-1, y, wz)) {
					aos, lights := sampleFaceLighting(wx, y, wz, lightWest, getBlock, getLight)

					sideTintCol := tintColor
					if block == blockGrass {
						sideTintCol = meshColor(0, 0, 0, 0)
					}
					tints := []color.RGBA{sideTintCol, sideTintCol, sideTintCol, sideTintCol}
					colors := a.applyAOSmooth(block, westTint, aos, lights, tints)

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
						meshVec3(-1, 0, 0),
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

						ovTints := []color.RGBA{smoothFoliage, smoothFoliage, smoothFoliage, smoothFoliage}
						ovColors := a.applyAOSmooth(block, white, aos, lights, ovTints)

						getBuilder(oPass, oPath).addFaceSmooth(
							[]float32{px - 0.5, py + 0.5, pz + oDisp, px - 0.5, py + 0.5, pz - oDisp, px - 0.5, py - 0.5, pz - oDisp, px - 0.5, py - 0.5, pz + oDisp},
							meshVec3(-1, 0, 0),
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
