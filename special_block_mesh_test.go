package main

import (
	"math"
	"testing"
)

func almostMesh(a, b float32) bool { return math.Abs(float64(a-b)) < 0.001 }

func TestTorchMeshesUseAtlasAndTouchSupport(t *testing.T) {
	initBlockRegistry()
	const path = "textures/block/torch.png"
	rect := AtlasRect{X: 0.25, Y: 0.125, Width: 0.0625, Height: 0.0625}
	a := &RenderAssets{atlas: &TextureAtlas{UVs: map[string]AtlasRect{path: rect}}}
	var heights [chunkWidth][chunkWidth]int16
	for meta := byte(0); meta <= 4; meta++ {
		t.Run(string(rune('0'+meta)), func(t *testing.T) {
			data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
				func(x, y, z int) byte {
					if x == 8 && y == 8 && z == 8 {
						return blockTorch
					}
					return blockAir
				},
				func(x, y, z int) byte { return 15 },
				func(x, y, z int) byte { return meta }, 12345)
			defer releaseMeshResults(data)
			if len(data["cutout"]) != 1 || len(data["cutout"]["atlas"]) != 1 {
				t.Fatalf("meta %d not drawn through atlas: %v", meta, data["cutout"])
			}
			mesh := data["cutout"]["atlas"][0]
			if mesh.vertCount != 44 || len(mesh.indices) != 66 {
				t.Fatalf("meta %d torch faces missing: %d vertices, %d indices", meta, mesh.vertCount, len(mesh.indices))
			}
			for i := 0; i < len(mesh.texcoords); i += 2 {
				u, v := mesh.texcoords[i], mesh.texcoords[i+1]
				if u < rect.X-1e-6 || u > rect.X+rect.Width+1e-6 || v < rect.Y-1e-6 || v > rect.Y+rect.Height+1e-6 {
					t.Fatalf("meta %d UV escaped torch atlas cell: %v,%v", meta, u, v)
				}
			}
			for face := 0; face < mesh.vertCount/4; face++ {
				v := face * 12
				first := meshVec3(mesh.vertices[v+3]-mesh.vertices[v], mesh.vertices[v+4]-mesh.vertices[v+1], mesh.vertices[v+5]-mesh.vertices[v+2])
				second := meshVec3(mesh.vertices[v+6]-mesh.vertices[v], mesh.vertices[v+7]-mesh.vertices[v+1], mesh.vertices[v+8]-mesh.vertices[v+2])
				n := meshCross(first, second)
				if n.X*mesh.normals[v]+n.Y*mesh.normals[v+1]+n.Z*mesh.normals[v+2] <= 0 {
					t.Fatalf("meta %d torch face %d has inward normal", meta, face)
				}
			}
			for i := 0; i < len(mesh.vertices); i += 3 {
				for axis := 0; axis < 3; axis++ {
					if mesh.vertices[i+axis] < 7.499 || mesh.vertices[i+axis] > 8.501 {
						t.Fatalf("meta %d torch vertex escaped block: %v", meta, mesh.vertices[i:i+3])
					}
				}
			}
			// Bottom stem face is the sixth quad. Its center must touch the
			// floor for meta 0 or the corresponding wall face for meta 1..4.
			var base [3]float32
			baseMin := [3]float32{100, 100, 100}
			baseMax := [3]float32{-100, -100, -100}
			for i := 20; i < 24; i++ {
				for axis := 0; axis < 3; axis++ {
					value := mesh.vertices[i*3+axis]
					base[axis] += value / 4
					baseMin[axis] = min(baseMin[axis], value)
					baseMax[axis] = max(baseMax[axis], value)
				}
			}
			switch meta {
			case 0:
				if !almostMesh(base[1], 7.5) {
					t.Fatalf("floor torch floats: base=%v", base)
				}
			case 1:
				if base[1] < 7.6 || base[1] > 7.8 {
					t.Fatalf("wall torch attaches too low or high: %v", base)
				}
				if !almostMesh(baseMax[2], 8.5) {
					t.Fatalf("north-wall base=%v", baseMax)
				}
			case 2:
				if !almostMesh(baseMin[2], 7.5) {
					t.Fatalf("south-wall base=%v", baseMin)
				}
			case 3:
				if !almostMesh(baseMax[0], 8.5) {
					t.Fatalf("west-wall base=%v", baseMax)
				}
			case 4:
				if !almostMesh(baseMin[0], 7.5) {
					t.Fatalf("east-wall base=%v", baseMin)
				}
			}
		})
	}
}

func TestInsetCactusSidesRemainVisibleBesideSolidBlocks(t *testing.T) {
	initBlockRegistry()
	const side = "textures/block/cactus_side.png"
	const top = "textures/block/cactus_top.png"
	rect := AtlasRect{X: 0.25, Y: 0.125, Width: 0.0625, Height: 0.0625}
	a := &RenderAssets{atlas: &TextureAtlas{UVs: map[string]AtlasRect{side: rect, top: rect}}}
	var heights [chunkWidth][chunkWidth]int16
	for _, tc := range []struct {
		name         string
		x, z, dx, dz int
	}{
		{"north", 8, 0, 0, -1}, {"south", 8, 15, 0, 1},
		{"west", 0, 8, -1, 0}, {"east", 15, 8, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
				func(x, y, z int) byte {
					if x == tc.x && y == 8 && z == tc.z {
						return blockCactus
					}
					if x == tc.x+tc.dx && y == 8 && z == tc.z+tc.dz {
						return blockStone
					}
					if x == tc.x && y == 7 && z == tc.z {
						return blockSand
					}
					return blockAir
				},
				func(x, y, z int) byte {
					if x == tc.x+tc.dx && y == 8 && z == tc.z+tc.dz {
						return 0 // opaque neighbor interior
					}
					return 15
				},
				func(x, y, z int) byte { return 0 }, 12345)
			defer releaseMeshResults(data)
			meshes := data["opaque"]["atlas"]
			if len(meshes) != 1 || meshes[0].vertCount != 20 {
				t.Fatalf("%s-side cactus beside stone needs top plus four sides; got %v", tc.name, data["opaque"])
			}
			mesh := meshes[0]
			// Top is the first face; north is the second. It must use the
			// same 1..15 horizontal texel crop as south/east/west.
			u0, u1 := mesh.texcoords[8], mesh.texcoords[10]
			if !almostMesh(u0, rect.X+rect.Width/16) || !almostMesh(u1, rect.X+rect.Width*15/16) {
				t.Fatalf("north UV differs from other cactus sides: %v,%v", u0, u1)
			}
			// At least the side beside the opaque neighbor must use light
			// from the cactus cell, rather than becoming nearly black.
			for face := 1; face <= 4; face++ {
				if mesh.colors[face*16] < 30 {
					t.Fatalf("cactus side %d was darkened by a solid neighbor", face)
				}
			}
		})
	}
}

func TestStackedCactusHasNoHiddenInternalCaps(t *testing.T) {
	initBlockRegistry()
	rect := AtlasRect{X: 0.25, Y: 0.125, Width: 0.0625, Height: 0.0625}
	a := &RenderAssets{atlas: &TextureAtlas{UVs: map[string]AtlasRect{
		"textures/block/cactus_side.png":   rect,
		"textures/block/cactus_top.png":    rect,
		"textures/block/cactus_bottom.png": rect,
	}}}
	var heights [chunkWidth][chunkWidth]int16
	data := a.buildAllMeshData(&heights, 0, 0, 8, 10,
		func(x, y, z int) byte {
			if x == 8 && z == 8 {
				switch y {
				case 7:
					return blockSand
				case 8, 9:
					return blockCactus
				}
			}
			return blockAir
		},
		func(x, y, z int) byte { return 15 },
		func(x, y, z int) byte { return 0 }, 12345)
	defer releaseMeshResults(data)
	meshes := data["opaque"]["atlas"]
	if len(meshes) != 1 || meshes[0].vertCount != 36 { // 8 sides + one exposed top
		t.Fatalf("stacked cactus emitted hidden top/bottom caps: %v", data["opaque"])
	}
}
