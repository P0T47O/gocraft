package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) makeFace(vertices []float32, normal rl.Vector3, texcoords []float32) faceMesh {
	normals := make([]float32, 0, 12)
	for i := 0; i < 4; i++ {
		normals = append(normals, normal.X, normal.Y, normal.Z)
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	mesh := rl.Mesh{
		VertexCount:   4,
		TriangleCount: 2,
		Vertices:      &vertices[0],
		Texcoords:     &texcoords[0],
		Normals:       &normals[0],
		Indices:       &indices[0],
	}
	rl.UploadMesh(&mesh, false)
	return faceMesh{
		mesh:      mesh,
		vertices:  vertices,
		normals:   normals,
		texcoords: texcoords,
		indices:   indices,
	}
}

func (a *RenderAssets) initFaceMeshes() {
	a.faceMeshes["top"] = a.makeFace(
		[]float32{
			-0.5, 0.5, 0.5,
			0.5, 0.5, 0.5,
			0.5, 0.5, -0.5,
			-0.5, 0.5, -0.5,
		},
		rl.NewVector3(0, 1, 0),
		[]float32{
			0, 0,
			1, 0,
			1, 1,
			0, 1,
		},
	)
	a.faceMeshes["bottom"] = a.makeFace(
		[]float32{
			-0.5, -0.5, 0.5,
			0.5, -0.5, 0.5,
			0.5, -0.5, -0.5,
			-0.5, -0.5, -0.5,
		},
		rl.NewVector3(0, -1, 0),
		[]float32{
			0, 0,
			1, 0,
			1, 1,
			0, 1,
		},
	)
	a.faceMeshes["north"] = a.makeFace(
		[]float32{
			-0.5, 0.5, -0.5,
			0.5, 0.5, -0.5,
			0.5, -0.5, -0.5,
			-0.5, -0.5, -0.5,
		},
		rl.NewVector3(0, 0, -1),
		[]float32{
			0, 0,
			1, 0,
			1, 1,
			0, 1,
		},
	)
	a.faceMeshes["south"] = a.makeFace(
		[]float32{
			0.5, 0.5, 0.5,
			-0.5, 0.5, 0.5,
			-0.5, -0.5, 0.5,
			0.5, -0.5, 0.5,
		},
		rl.NewVector3(0, 0, 1),
		[]float32{
			1, 0,
			0, 0,
			0, 1,
			1, 1,
		},
	)
	a.faceMeshes["east"] = a.makeFace(
		[]float32{
			0.5, 0.5, -0.5,
			0.5, 0.5, 0.5,
			0.5, -0.5, 0.5,
			0.5, -0.5, -0.5,
		},
		rl.NewVector3(1, 0, 0),
		[]float32{
			0, 0,
			1, 0,
			1, 1,
			0, 1,
		},
	)
	a.faceMeshes["west"] = a.makeFace(
		[]float32{
			-0.5, 0.5, 0.5,
			-0.5, 0.5, -0.5,
			-0.5, -0.5, -0.5,
			-0.5, -0.5, 0.5,
		},
		rl.NewVector3(-1, 0, 0),
		[]float32{
			1, 0,
			0, 0,
			0, 1,
			1, 1,
		},
	)

	// Torch meshes (Box style, 2x2 pixels)
	// Vertices are at +/- 0.5 in local space. With scale 0.125, this becomes +/- 0.0625 world units (2 pixels).

	// NORTH face (Z = -0.5)
	a.faceMeshes["torch_side_north"] = a.makeFace(
		[]float32{-0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5},
		rl.NewVector3(0, 0, -1),
		[]float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 1.0, 0.4375, 1.0},
	)
	// SOUTH face (Z = 0.5)
	a.faceMeshes["torch_side_south"] = a.makeFace(
		[]float32{0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5},
		rl.NewVector3(0, 0, 1),
		[]float32{0.5625, 0.375, 0.4375, 0.375, 0.4375, 1.0, 0.5625, 1.0},
	)
	// EAST face (X = 0.5)
	a.faceMeshes["torch_side_east"] = a.makeFace(
		[]float32{0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5},
		rl.NewVector3(1, 0, 0),
		[]float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 1.0, 0.4375, 1.0},
	)
	// WEST face (X = -0.5)
	a.faceMeshes["torch_side_west"] = a.makeFace(
		[]float32{-0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5, -0.5, 0.5},
		rl.NewVector3(-1, 0, 0),
		[]float32{0.5625, 0.375, 0.4375, 0.375, 0.4375, 1.0, 0.5625, 1.0},
	)
	// TOP face (Y = 0.5)
	a.faceMeshes["torch_top"] = a.makeFace(
		[]float32{-0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, -0.5, -0.5, 0.5, -0.5},
		rl.NewVector3(0, 1, 0),
		[]float32{0.4375, 0.375, 0.5625, 0.375, 0.5625, 0.5, 0.4375, 0.5},
	)
	// BOTTOM face (Y = -0.5)
	a.faceMeshes["torch_bottom"] = a.makeFace(
		[]float32{-0.5, -0.5, -0.5, 0.5, -0.5, -0.5, 0.5, -0.5, 0.5, -0.5, -0.5, 0.5},
		rl.NewVector3(0, -1, 0),
		[]float32{0.4375, 0.5, 0.5625, 0.5, 0.5625, 0.625, 0.4375, 0.625},
	)

	// Flame meshes (Box style, slightly higher)
	a.faceMeshes["torch_flame_north"] = a.makeFace(
		[]float32{-0.5, 0.5, -0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5},
		rl.NewVector3(0, 0, -1),
		[]float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.375, 0.4375, 0.375},
	)
	a.faceMeshes["torch_flame_south"] = a.makeFace(
		[]float32{0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5, 0.5, 0.5, -0.5, 0.5},
		rl.NewVector3(0, 0, 1),
		[]float32{0.5625, 0.0, 0.4375, 0.0, 0.4375, 0.375, 0.5625, 0.375},
	)
	a.faceMeshes["torch_flame_east"] = a.makeFace(
		[]float32{0.5, 0.5, -0.5, 0.5, 0.5, 0.5, 0.5, -0.5, 0.5, 0.5, -0.5, -0.5},
		rl.NewVector3(1, 0, 0),
		[]float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.375, 0.4375, 0.375},
	)
	a.faceMeshes["torch_flame_west"] = a.makeFace(
		[]float32{-0.5, 0.5, 0.5, -0.5, 0.5, -0.5, -0.5, -0.5, -0.5, -0.5, -0.5, 0.5},
		rl.NewVector3(-1, 0, 0),
		[]float32{0.5625, 0.0, 0.4375, 0.0, 0.4375, 0.375, 0.5625, 0.375},
	)
	a.faceMeshes["torch_flame_top"] = a.makeFace(
		[]float32{-0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, -0.5, -0.5, 0.5, -0.5},
		rl.NewVector3(0, 1, 0),
		[]float32{0.4375, 0.0, 0.5625, 0.0, 0.5625, 0.125, 0.4375, 0.125},
	)

	// Cross meshes for plants (X shape)
	// Plane 1: -0.4 to 0.4 diagonally
	a.faceMeshes["cross_1"] = a.makeFace(
		[]float32{
			-0.4, 0.5, -0.4,
			0.4, 0.5, 0.4,
			0.4, -0.5, 0.4,
			-0.4, -0.5, -0.4,
		},
		rl.NewVector3(1, 0, -1),
		[]float32{0, 0, 1, 0, 1, 1, 0, 1},
	)
	// Plane 2: -0.4 to 0.4 diagonally
	a.faceMeshes["cross_2"] = a.makeFace(
		[]float32{
			-0.4, 0.5, 0.4,
			0.4, 0.5, -0.4,
			0.4, -0.5, -0.4,
			-0.4, -0.5, 0.4,
		},
		rl.NewVector3(1, 0, 1),
		[]float32{0, 0, 1, 0, 1, 1, 0, 1},
	)

	// Cactus Meshes (Inset by 1 pixel = 1/16 = 0.0625)
	// Width = 14/16 = 0.875
	// Half Width = 0.4375
	cactusExt := float32(0.4375)

	// Top (Y = 0.5)
	a.faceMeshes["cactus_top"] = a.makeFace(
		[]float32{-cactusExt, 0.5, cactusExt, cactusExt, 0.5, cactusExt, cactusExt, 0.5, -cactusExt, -cactusExt, 0.5, -cactusExt},
		rl.NewVector3(0, 1, 0),
		[]float32{0.0625, 0.0625, 0.9375, 0.0625, 0.9375, 0.9375, 0.0625, 0.9375},
	)
	// Bottom (Y = -0.5)
	a.faceMeshes["cactus_bottom"] = a.makeFace(
		[]float32{-cactusExt, -0.5, cactusExt, cactusExt, -0.5, cactusExt, cactusExt, -0.5, -cactusExt, -cactusExt, -0.5, -cactusExt},
		rl.NewVector3(0, -1, 0),
		[]float32{0.0625, 0.0625, 0.9375, 0.0625, 0.9375, 0.9375, 0.0625, 0.9375},
	)
	// North (-Z)
	a.faceMeshes["cactus_north"] = a.makeFace(
		[]float32{-cactusExt, 0.5, -cactusExt, cactusExt, 0.5, -cactusExt, cactusExt, -0.5, -cactusExt, -cactusExt, -0.5, -cactusExt},
		rl.NewVector3(0, 0, -1),
		[]float32{0.0625, 0, 0.9375, 0, 0.9375, 1, 0.0625, 1},
	)
	// South (+Z)
	a.faceMeshes["cactus_south"] = a.makeFace(
		[]float32{cactusExt, 0.5, cactusExt, -cactusExt, 0.5, cactusExt, -cactusExt, -0.5, cactusExt, cactusExt, -0.5, cactusExt},
		rl.NewVector3(0, 0, 1),
		[]float32{0.0625, 0, 0.9375, 0, 0.9375, 1, 0.0625, 1},
	)
	// East (+X)
	a.faceMeshes["cactus_east"] = a.makeFace(
		[]float32{cactusExt, 0.5, -cactusExt, cactusExt, 0.5, cactusExt, cactusExt, -0.5, cactusExt, cactusExt, -0.5, -cactusExt},
		rl.NewVector3(1, 0, 0),
		[]float32{0.0625, 0, 0.9375, 0, 0.9375, 1, 0.0625, 1},
	)
	// West (-X)
	a.faceMeshes["cactus_west"] = a.makeFace(
		[]float32{-cactusExt, 0.5, cactusExt, -cactusExt, 0.5, -cactusExt, -cactusExt, -0.5, -cactusExt, -cactusExt, -0.5, cactusExt},
		rl.NewVector3(-1, 0, 0),
		[]float32{0.0625, 0, 0.9375, 0, 0.9375, 1, 0.0625, 1},
	)
}

func (a *RenderAssets) getFaceModel(face string, path string) rl.Model {
	key := face + "|" + path
	if model, ok := a.faceModels[key]; ok {
		return model
	}
	tex := a.textures[path]
	mesh := a.faceMeshes[face].mesh
	model := rl.LoadModelFromMesh(mesh)
	materials := model.GetMaterials()
	if len(materials) > 0 && tex.ID != 0 {
		rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
		if (path == "textures/block/oak_leaves.png" || path == "textures/block/torch.png") && a.cutoutShader.ID != 0 {
			materials[0].Shader = a.cutoutShader
		}
	}
	a.faceModels[key] = model
	return model
}

func (a *RenderAssets) initFaceModels() {
	for i := 0; i < 256; i++ {
		def := GetItemVisual(byte(i))
		if def == nil || def.ID == blockAir {
			continue
		}
		a.getFaceModel("top", def.Textures.Top)
		a.getFaceModel("bottom", def.Textures.Bottom)
		a.getFaceModel("north", def.Textures.North)
		a.getFaceModel("south", def.Textures.South)
		a.getFaceModel("east", def.Textures.East)
		a.getFaceModel("west", def.Textures.West)
	}
}
