package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Keep mesh data alive to prevent GC
var crossItemMeshData = make(map[byte]*meshDataHolder)

type meshDataHolder struct {
	vertices []float32
	colors   []uint8
}

func (a *RenderAssets) DrawItem(e *RemoteEntity) {
	if e.Metadata == 0 {
		return
	}
	itemID := byte(e.Metadata & 0xFF)
	count := int((e.Metadata >> 8) & 0xFF)
	if count <= 0 {
		count = 1
	}

	def := GetBlock(itemID)
	if def == nil {
		return
	}

	// Animation
	time := float32(rl.GetTime())
	bob := float32(math.Sin(float64(time*2.5))) * 0.1
	rot := time * 90.0

	pos := rl.NewVector3(float32(e.X), float32(e.Y)+bob+0.25, float32(e.Z))

	// Determine how many items to draw (1-3 based on count)
	numItems := 1
	if count >= 16 {
		numItems = 2
	}
	if count >= 32 {
		numItems = 3
	}

	offsets := []rl.Vector3{
		{X: 0, Y: 0, Z: 0},
		{X: 0.08, Y: 0.04, Z: -0.08},
		{X: -0.08, Y: 0.08, Z: 0.08},
	}

	// Calculate Rotation Matrix
	matRot := rl.MatrixRotate(rl.NewVector3(0, 1, 0), rot*math.Pi/180.0)

	rl.DisableBackfaceCulling() // Keep this just in case

	for i := 0; i < numItems; i++ {
		off := offsets[i]

		// Rotate offset
		offVec := rl.NewVector3(off.X, off.Y, off.Z)
		rotatedOff := rl.Vector3Transform(offVec, matRot)

		// Final Position
		finalPos := rl.Vector3Add(pos, rotatedOff)

		if def.RenderType == RenderTypeCross {
			// Use pre-generated model
			if model, ok := a.crossItemModels[itemID]; ok {
				rl.DrawModel(model, finalPos, 1.0, rl.White)
			}
		} else {
			a.drawBlockItem(itemID, finalPos, 0.25, rot)
		}
	}
	rl.EnableBackfaceCulling()
}

func (a *RenderAssets) drawBlockItem(block byte, position rl.Vector3, scale float32, rotation float32) {
	def := GetBlock(block)
	if def == nil {
		return
	}
	faces := def.Textures

	topTint := rl.NewColor(255, 255, 255, 255)
	northTint := rl.NewColor(210, 210, 210, 255)
	southTint := rl.NewColor(225, 225, 225, 255)
	westTint := rl.NewColor(200, 200, 200, 255)
	eastTint := rl.NewColor(190, 190, 190, 255)
	bottomTint := rl.NewColor(140, 140, 140, 255)
	grassTint := rl.NewColor(112, 185, 70, 255)
	leafTint := rl.NewColor(96, 165, 60, 255)

	applyBiomeTint := func(col rl.Color, useGrass bool) rl.Color {
		if block == blockLeaves {
			return rl.NewColor(
				uint8(float32(col.R)*float32(leafTint.R)/255.0),
				uint8(float32(col.G)*float32(leafTint.G)/255.0),
				uint8(float32(col.B)*float32(leafTint.B)/255.0),
				col.A,
			)
		}
		if block == blockGrass && useGrass {
			return rl.NewColor(
				uint8(float32(col.R)*float32(grassTint.R)/255.0),
				uint8(float32(col.G)*float32(grassTint.G)/255.0),
				uint8(float32(col.B)*float32(grassTint.B)/255.0),
				col.A,
			)
		}
		return col
	}

	blockPos := position // Already absolute
	scaleVec := rl.NewVector3(scale, scale, scale)
	axis := rl.NewVector3(0, 1, 0)

	rl.DrawModelEx(a.getFaceModel("top", faces.Top), blockPos, axis, rotation, scaleVec, applyBiomeTint(topTint, true))
	rl.DrawModelEx(a.getFaceModel("bottom", faces.Bottom), blockPos, axis, rotation, scaleVec, applyBiomeTint(bottomTint, false))
	rl.DrawModelEx(a.getFaceModel("north", faces.North), blockPos, axis, rotation, scaleVec, applyBiomeTint(northTint, false))
	rl.DrawModelEx(a.getFaceModel("south", faces.South), blockPos, axis, rotation, scaleVec, applyBiomeTint(southTint, false))
	rl.DrawModelEx(a.getFaceModel("west", faces.West), blockPos, axis, rotation, scaleVec, applyBiomeTint(westTint, false))
	rl.DrawModelEx(a.getFaceModel("east", faces.East), blockPos, axis, rotation, scaleVec, applyBiomeTint(eastTint, false))
}

// initCrossItemModels pre-generates meshes for cross-type items (flowers, etc.)
func (a *RenderAssets) initCrossItemModels() {
	for _, block := range allBlocks {
		def := GetBlock(block)
		if def == nil || def.RenderType != RenderTypeCross {
			continue
		}

		model := a.generateCrossItemMesh(block)
		if model.Meshes != nil {
			a.crossItemModels[block] = model
		}
	}
}

// generateCrossItemMesh creates a single mesh with all pixel cubes for a cross item
func (a *RenderAssets) generateCrossItemMesh(block byte) rl.Model {
	def := GetBlock(block)
	if def == nil {
		return rl.Model{}
	}

	texPath := def.Textures.North
	tex := a.textures[texPath]
	if tex.ID == 0 {
		return rl.Model{}
	}

	img := rl.LoadImageFromTexture(tex)
	if img == nil || img.Data == nil {
		return rl.Model{}
	}
	defer rl.UnloadImage(img)

	colors := rl.LoadImageColors(img)
	if len(colors) == 0 {
		return rl.Model{}
	}
	defer rl.UnloadImageColors(colors)

	w := int(img.Width)
	h := int(img.Height)

	// Tint for tall grass
	tint := rl.White
	if block == blockTallGrass {
		tint = rl.NewColor(145, 189, 89, 255)
	}

	scale := float32(0.3)
	pixelSize := scale / float32(w)
	thickness := pixelSize * 1.2

	// Collect all vertices and colors
	holder := &meshDataHolder{
		vertices: make([]float32, 0, 256*36*3),
		colors:   make([]uint8, 0, 256*36*4),
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if idx >= len(colors) {
				continue
			}
			c := colors[idx]
			if c.A < 10 {
				continue
			}

			// Apply tint
			r := uint8(float32(c.R) * float32(tint.R) / 255.0)
			g := uint8(float32(c.G) * float32(tint.G) / 255.0)
			b := uint8(float32(c.B) * float32(tint.B) / 255.0)

			// Calculate world position for this pixel
			pixelX := -scale/2 + (float32(x)+0.5)*pixelSize
			pixelY := scale/2 - (float32(y)+0.5)*pixelSize
			pixelZ := float32(0)

			// Add cube vertices
			addCubeVertices(&holder.vertices, &holder.colors, pixelX, pixelY, pixelZ, pixelSize, pixelSize, thickness, r, g, b)
		}
	}

	if len(holder.vertices) == 0 {
		return rl.Model{}
	}

	// Keep the data alive
	crossItemMeshData[block] = holder

	// Create new mesh manually
	mesh := rl.Mesh{
		VertexCount:   int32(len(holder.vertices) / 3),
		TriangleCount: int32(len(holder.vertices) / 9),
	}

	// Point to the slice data
	mesh.Vertices = &holder.vertices[0]
	mesh.Colors = &holder.colors[0]

	rl.UploadMesh(&mesh, false)
	model := rl.LoadModelFromMesh(mesh)

	return model
}

// addCubeVertices adds 36 vertices (6 faces) for a cube at the given position
func addCubeVertices(vertices *[]float32, colors *[]uint8, cx, cy, cz, sx, sy, sz float32, r, g, b uint8) {
	hx, hy, hz := sx/2, sy/2, sz/2

	// Define 6 faces (each face has 2 triangles = 6 vertices)
	faces := [][6][3]float32{
		// Front (Z+)
		{
			{cx - hx, cy + hy, cz + hz}, {cx + hx, cy + hy, cz + hz}, {cx + hx, cy - hy, cz + hz},
			{cx - hx, cy + hy, cz + hz}, {cx + hx, cy - hy, cz + hz}, {cx - hx, cy - hy, cz + hz},
		},
		// Back (Z-)
		{
			{cx + hx, cy + hy, cz - hz}, {cx - hx, cy + hy, cz - hz}, {cx - hx, cy - hy, cz - hz},
			{cx + hx, cy + hy, cz - hz}, {cx - hx, cy - hy, cz - hz}, {cx + hx, cy - hy, cz - hz},
		},
		// Top (Y+)
		{
			{cx - hx, cy + hy, cz - hz}, {cx + hx, cy + hy, cz - hz}, {cx + hx, cy + hy, cz + hz},
			{cx - hx, cy + hy, cz - hz}, {cx + hx, cy + hy, cz + hz}, {cx - hx, cy + hy, cz + hz},
		},
		// Bottom (Y-)
		{
			{cx - hx, cy - hy, cz + hz}, {cx + hx, cy - hy, cz + hz}, {cx + hx, cy - hy, cz - hz},
			{cx - hx, cy - hy, cz + hz}, {cx + hx, cy - hy, cz - hz}, {cx - hx, cy - hy, cz - hz},
		},
		// Right (X+)
		{
			{cx + hx, cy + hy, cz + hz}, {cx + hx, cy + hy, cz - hz}, {cx + hx, cy - hy, cz - hz},
			{cx + hx, cy + hy, cz + hz}, {cx + hx, cy - hy, cz - hz}, {cx + hx, cy - hy, cz + hz},
		},
		// Left (X-)
		{
			{cx - hx, cy + hy, cz - hz}, {cx - hx, cy + hy, cz + hz}, {cx - hx, cy - hy, cz + hz},
			{cx - hx, cy + hy, cz - hz}, {cx - hx, cy - hy, cz + hz}, {cx - hx, cy - hy, cz - hz},
		},
	}

	// Shade each face differently
	faceShades := []float32{1.0, 0.85, 1.0, 0.6, 0.9, 0.8}

	for fi, face := range faces {
		shade := faceShades[fi]
		for _, v := range face {
			*vertices = append(*vertices, v[0], v[1], v[2])
			*colors = append(*colors,
				uint8(float32(r)*shade),
				uint8(float32(g)*shade),
				uint8(float32(b)*shade),
				255,
			)
		}
	}
}
