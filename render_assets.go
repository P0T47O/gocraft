package main

import (
	"encoding/json"
	"os"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type RenderAssets struct {
	textures        map[string]rl.Texture2D
	faceMeshes      map[string]faceMesh
	faceModels      map[string]rl.Model
	iconRenders     map[byte]rl.RenderTexture2D
	iconOffsets     map[byte]rl.Vector2
	animated        map[string]*AnimatedTexture
	materials       map[string]rl.Material
	crossItemModels map[byte]rl.Model // Pre-generated extruded meshes for cross-type items
	hotbarTex       rl.Texture2D
	hotbarSel       rl.Texture2D
	slotTex         rl.Texture2D
	slotSelect      rl.Texture2D
	inventoryTex    rl.Texture2D
	iconCamera      rl.Camera3D
	cutoutShader    rl.Shader
	fogShader       rl.Shader
	atlas           *TextureAtlas
}

type AnimatedTexture struct {
	Frames       []rl.Texture2D
	FrameSeconds float32
	Time         float32
	Index        int
}

type TextureAtlas struct {
	Texture rl.Texture2D
	UVs     map[string]rl.Rectangle
}

func loadRenderAssets() *RenderAssets {
	assets := &RenderAssets{
		textures:        map[string]rl.Texture2D{},
		faceMeshes:      map[string]faceMesh{},
		faceModels:      map[string]rl.Model{},
		iconRenders:     map[byte]rl.RenderTexture2D{},
		iconOffsets:     map[byte]rl.Vector2{},
		animated:        map[string]*AnimatedTexture{},
		materials:       map[string]rl.Material{},
		crossItemModels: map[byte]rl.Model{},
		iconCamera: rl.Camera3D{
			Position:   rl.NewVector3(2.3, 2.3, 2.3),
			Target:     rl.NewVector3(0, 0, 0),
			Up:         rl.NewVector3(0, 1, 0),
			Fovy:       2.4,
			Projection: rl.CameraOrthographic,
		},
	}

	assets.hotbarTex = rl.LoadTexture("textures/gui/sprites/hud/hotbar.png")
	assets.hotbarSel = rl.LoadTexture("textures/gui/sprites/hud/hotbar_selection.png")
	assets.slotTex = rl.LoadTexture("textures/gui/sprites/container/slot.png")
	assets.slotSelect = rl.LoadTexture("textures/gui/sprites/container/slot_highlight_front.png")
	assets.inventoryTex = rl.LoadTexture("textures/gui/container/inventory.png")
	if assets.hotbarTex.ID != 0 {
		rl.SetTextureFilter(assets.hotbarTex, rl.FilterPoint)
	}
	if assets.hotbarSel.ID != 0 {
		rl.SetTextureFilter(assets.hotbarSel, rl.FilterPoint)
	}
	if assets.slotTex.ID != 0 {
		rl.SetTextureFilter(assets.slotTex, rl.FilterPoint)
	}
	if assets.slotSelect.ID != 0 {
		rl.SetTextureFilter(assets.slotSelect, rl.FilterPoint)
	}
	if assets.inventoryTex.ID != 0 {
		rl.SetTextureFilter(assets.inventoryTex, rl.FilterPoint)
	}

	assets.loadBlockTextures()
	// Explicitly load grass side overlay for multi-pass rendering
	assets.loadTexture("textures/block/grass_block_side_overlay.png")

	assets.generateAtlas()
	assets.cutoutShader = loadCutoutShader()
	assets.fogShader = assets.loadFogShader()
	assets.initFaceMeshes()
	assets.initFaceModels()
	assets.initIcons()
	assets.initCrossItemModels()

	return assets
}

func (a *RenderAssets) unload() {
	animatedIDs := map[uint32]bool{}
	for _, rt := range a.iconRenders {
		if rt.ID != 0 {
			rl.UnloadRenderTexture(rt)
		}
	}
	for _, anim := range a.animated {
		for _, tex := range anim.Frames {
			if tex.ID != 0 {
				animatedIDs[tex.ID] = true
				rl.UnloadTexture(tex)
			}
		}
	}
	// Note: We skip unloading a.faceModels and a.faceMeshes because they share meshes
	// and calling UnloadModel on multiple models sharing a mesh causes double-free crashes (0xc0000374).
	// Since these are only unloaded on exit, the OS will reclaim the memory.

	for _, tex := range a.textures {
		if tex.ID != 0 && !animatedIDs[tex.ID] {
			rl.UnloadTexture(tex)
		}
	}
	if a.hotbarTex.ID != 0 {
		rl.UnloadTexture(a.hotbarTex)
	}
	if a.hotbarSel.ID != 0 {
		rl.UnloadTexture(a.hotbarSel)
	}
	if a.slotTex.ID != 0 {
		rl.UnloadTexture(a.slotTex)
	}
	if a.slotSelect.ID != 0 {
		rl.UnloadTexture(a.slotSelect)
	}
	if a.inventoryTex.ID != 0 {
		rl.UnloadTexture(a.inventoryTex)
	}

	// Do NOT unload materials as they share textures/shaders managed by RenderAssets.
	// UnloadMaterial would try to free these shared resources causing double-free crashes.
	// Since LoadMaterialDefault returns a struct by value and we only attached shared pointers,
	// we don't need to explicitly unload them if we unload transparency resources separately.
	a.materials = nil

	if a.cutoutShader.ID != 0 {
		rl.UnloadShader(a.cutoutShader)
	}
	if a.fogShader.ID != 0 {
		rl.UnloadShader(a.fogShader)
	}
}

func (a *RenderAssets) loadTexture(path string) rl.Texture2D {
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	var tex rl.Texture2D
	if isAnimatedTexture(path) {
		tex = a.loadAnimatedTexture(path)
	} else if path == "textures/block/torch.png" {
		img := rl.LoadImage(path)
		if img != nil && img.Data != nil {
			if img.Width == 2 && img.Height == 16 {
				canvas := rl.GenImageColor(16, 16, rl.Blank)
				src := rl.NewRectangle(0, 0, float32(img.Width), float32(img.Height))
				dst := rl.NewRectangle(7, 0, float32(img.Width), float32(img.Height))
				rl.ImageDraw(canvas, img, src, dst, rl.White)
				rl.UnloadImage(img)
				img = canvas
			}
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	} else if path == "textures/block/oak_leaves.png" || path == "textures/block/grass_block_side_overlay.png" {
		img := rl.LoadImage(path)
		if img != nil && img.Data != nil {
			colors := rl.LoadImageColors(img)
			hasAlpha := false
			if len(colors) > 0 {
				for _, c := range colors {
					if c.A < 255 {
						hasAlpha = true
						break
					}
				}
				if !hasAlpha {
					key := pickEdgeColorKey(colors, int(img.Width), int(img.Height))
					rl.ImageAlphaClear(img, key, 0.05)
				}
				rl.UnloadImageColors(colors)
			}
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	} else if path == "textures/block/glass.png" {
		img := rl.LoadImage(path)
		if img != nil && img.Data != nil {
			colors := rl.LoadImageColors(img)
			if len(colors) > 0 {
				w := int(img.Width)
				h := int(img.Height)
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						idx := y*w + x
						c := colors[idx]
						if c.A > 0 && c.A < 255 {
							c.A = 255
							rl.ImageDrawPixel(img, int32(x), int32(y), c)
						}
					}
				}
				rl.UnloadImageColors(colors)
			}
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		}
	}
	if tex.ID == 0 {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File missing: Generate Magenta/Black Checkerboard
			img := rl.GenImageChecked(16, 16, 8, 8, rl.Magenta, rl.Black)
			tex = rl.LoadTextureFromImage(img)
			rl.UnloadImage(img)
		} else {
			// File exists: Load it
			img := rl.LoadImage(path)
			if img != nil && img.Data != nil {
				w := float32(img.Width)
				h := float32(img.Height)
				if h > w {
					rl.ImageCrop(img, rl.NewRectangle(0, 0, w, w))
				}
				tex = rl.LoadTextureFromImage(img)
				rl.UnloadImage(img)
			} else {
				tex = rl.LoadTexture(path)
			}
		}
	}

	if tex.ID != 0 {
		rl.SetTextureFilter(tex, rl.FilterPoint)
	}
	a.textures[path] = tex
	return tex
}

func (a *RenderAssets) getMaterial(path string, tex rl.Texture2D) rl.Material {
	if mat, ok := a.materials[path]; ok {
		return mat
	}

	material := rl.LoadMaterialDefault()
	if tex.ID != 0 {
		rl.SetMaterialTexture(&material, rl.MapDiffuse, tex)
	}

	if a.fogShader.ID != 0 {
		material.Shader = a.fogShader
	} else if (path == "textures/block/oak_leaves.png" || path == "textures/block/torch.png") && a.cutoutShader.ID != 0 {
		material.Shader = a.cutoutShader
	}

	a.materials[path] = material
	return material
}

func pickEdgeColorKey(colors []rl.Color, w, h int) rl.Color {
	if w <= 0 || h <= 0 || len(colors) < w*h {
		return rl.White
	}
	counts := map[uint32]int{}
	encode := func(c rl.Color) uint32 {
		return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
	}
	track := func(x, y int) {
		c := colors[y*w+x]
		counts[encode(c)]++
	}
	for x := 0; x < w; x++ {
		track(x, 0)
		track(x, h-1)
	}
	for y := 1; y < h-1; y++ {
		track(0, y)
		track(w-1, y)
	}
	var best rl.Color
	bestCount := -1
	for k, v := range counts {
		if v > bestCount {
			bestCount = v
			best = rl.NewColor(uint8(k>>24), uint8(k>>16), uint8(k>>8), uint8(k))
		}
	}
	if bestCount < 0 {
		return rl.White
	}
	return best
}

func loadCutoutShader() rl.Shader {
	vertex := "#version 330\n" +
		"in vec3 vertexPosition;\n" +
		"in vec2 vertexTexCoord;\n" +
		"in vec4 vertexColor;\n" +
		"out vec2 fragTexCoord;\n" +
		"out vec4 fragColor;\n" +
		"uniform mat4 mvp;\n" +
		"void main()\n" +
		"{\n" +
		"    fragTexCoord = vertexTexCoord;\n" +
		"    fragColor = vertexColor;\n" +
		"    gl_Position = mvp*vec4(vertexPosition, 1.0);\n" +
		"}\n"
	fragment := "#version 330\n" +
		"in vec2 fragTexCoord;\n" +
		"in vec4 fragColor;\n" +
		"out vec4 finalColor;\n" +
		"uniform sampler2D texture0;\n" +
		"uniform vec4 colDiffuse;\n" +
		"void main()\n" +
		"{\n" +
		"    vec4 texelColor = texture(texture0, fragTexCoord);\n" +
		"    vec4 color = texelColor*colDiffuse*fragColor;\n" +
		"    if (color.a < 0.5) discard;\n" +
		"    finalColor = color;\n" +
		"}\n"
	return rl.LoadShaderFromMemory(vertex, fragment)
}

func isAnimatedTexture(path string) bool {
	switch path {
	case "textures/block/water_still.png", "textures/block/water_flow.png", "textures/block/lava_still.png", "textures/block/lava_flow.png":
		return true
	default:
		return false
	}
}

func (a *RenderAssets) loadAnimatedTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[anim.Index]
	}
	img := rl.LoadImage(path)
	if img == nil || img.Data == nil {
		if img != nil {
			rl.UnloadImage(img)
		}
		return rl.Texture2D{}
	}
	w := int(img.Width)
	h := int(img.Height)
	frameSize := w
	if h < w {
		frameSize = h
	}
	frames := 0
	if frameSize > 0 {
		frames = h / frameSize
	}
	if frames <= 0 {
		frames = 1
	}
	frameList := make([]rl.Texture2D, 0, frames)
	for i := 0; i < frames; i++ {
		frame := rl.ImageCopy(img)
		rec := rl.NewRectangle(0, float32(i*frameSize), float32(frameSize), float32(frameSize))
		rl.ImageCrop(frame, rec)
		tex := rl.LoadTextureFromImage(frame)
		if tex.ID != 0 {
			rl.SetTextureFilter(tex, rl.FilterPoint)
			frameList = append(frameList, tex)
		}
		rl.UnloadImage(frame)
	}
	rl.UnloadImage(img)

	frameSeconds := float32(0.1)
	metaPath := path + ".mcmeta"
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta struct {
			Animation struct {
				FrameTime int `json:"frametime"`
			} `json:"animation"`
		}
		if json.Unmarshal(data, &meta) == nil && meta.Animation.FrameTime > 0 {
			frameSeconds = float32(meta.Animation.FrameTime) / 20.0
		}
	}
	anim := &AnimatedTexture{
		Frames:       frameList,
		FrameSeconds: frameSeconds,
		Time:         0,
		Index:        0,
	}
	a.animated[path] = anim
	if len(frameList) == 0 {
		return rl.Texture2D{}
	}
	return frameList[0]
}

func (a *RenderAssets) Update(dt float32) {
	for path, anim := range a.animated {
		if len(anim.Frames) <= 1 || anim.FrameSeconds <= 0 {
			continue
		}
		index := int(rl.GetTime()/float64(anim.FrameSeconds)) % len(anim.Frames)
		if index != anim.Index {
			anim.Index = index
			a.setTextureForPath(path, anim.Frames[anim.Index])
		}
	}
}

func (a *RenderAssets) setTextureForPath(path string, tex rl.Texture2D) {
	a.textures[path] = tex
	for key, model := range a.faceModels {
		if strings.HasSuffix(key, "|"+path) {
			materials := model.GetMaterials()
			if len(materials) > 0 {
				rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
			}
			a.faceModels[key] = model
		}
	}
}

func (a *RenderAssets) loadBlockTextures() {
	for i := 0; i < 256; i++ {
		def := Blocks[i]
		if def == nil || def.ID == blockAir {
			continue
		}
		a.loadTexture(def.Textures.Top)
		a.loadTexture(def.Textures.Bottom)
		a.loadTexture(def.Textures.North)
		a.loadTexture(def.Textures.South)
		a.loadTexture(def.Textures.East)
		a.loadTexture(def.Textures.West)
	}
}

func (a *RenderAssets) getFaceModelAnimated(face string, path string) rl.Model {
	model := a.getFaceModel(face, path)
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		tex := anim.Frames[anim.Index]
		materials := model.GetMaterials()
		if len(materials) > 0 {
			rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
		}
	}
	return model
}

func (a *RenderAssets) isAnimated(path string) bool {
	anim, ok := a.animated[path]
	return ok && len(anim.Frames) > 1
}

func (a *RenderAssets) currentTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[anim.Index]
	}
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	return rl.Texture2D{}
}

func (a *RenderAssets) baseTexture(path string) rl.Texture2D {
	if anim, ok := a.animated[path]; ok && len(anim.Frames) > 0 {
		return anim.Frames[0]
	}
	if tex, ok := a.textures[path]; ok {
		return tex
	}
	return rl.Texture2D{}
}

func (a *RenderAssets) applyTextureToModel(model rl.Model, tex rl.Texture2D) rl.Model {
	if tex.ID == 0 {
		return model
	}
	materials := model.GetMaterials()
	if len(materials) > 0 {
		rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
	}
	return model
}

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
		def := Blocks[i]
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

func (a *RenderAssets) initIcons() {
	for _, block := range allBlocks {
		a.makeIcon(block)
	}
}

func (a *RenderAssets) makeIcon(block byte) rl.RenderTexture2D {
	if rt, ok := a.iconRenders[block]; ok {
		return rt
	}
	rt := rl.LoadRenderTexture(64, 64)
	if rt.ID != 0 {
		rl.BeginTextureMode(rt)
		rl.ClearBackground(rl.Blank)
		rl.EnableDepthTest()
		def := GetBlock(block)
		if def.RenderType == RenderTypeCross {
			// Render flat 2D sprite for cross type
			texPath := def.Textures.North
			tex := a.textures[texPath]
			if tex.ID != 0 {
				// Draw texture scaled to fit 64x64
				// Keep aspect ratio if needed, but usually 1:1 for icons
				scale := float32(64) / float32(tex.Width)
				if float32(tex.Height)*scale > 64 {
					scale = 64 / float32(tex.Height)
				}
				destW := float32(tex.Width) * scale
				destH := float32(tex.Height) * scale
				destX := (64 - destW) / 2
				destY := (64 - destH) / 2

				tint := rl.White
				if block == blockTallGrass {
					tint = rl.NewColor(145, 189, 89, 255)
				}
				rl.DrawTextureEx(tex, rl.NewVector2(destX, destY), 0, scale, tint)
			}
		} else {
			rl.BeginMode3D(a.iconCamera)

			// Shading for icon: Top is bright, Front/Side have different shades
			getShadedLight := func(ix, iy, iz int) byte {
				if iy > 0 {
					return 15 // Top
				}
				if iz < 0 {
					return 12 // North
				}
				if iz > 0 {
					return 14 // South
				}
				if ix < 0 {
					return 10 // West
				}
				if ix > 0 {
					return 11 // East
				}
				return 13
			}

			if block == blockLeaves {
				rl.DisableBackfaceCulling()
			}
			a.drawBlock(block, rl.NewVector3(0, 0, 0), func(nx, ny, nz int) byte {
				return blockAir
			}, getShadedLight, func(ix, iy, iz int) byte {
				return 0
			}, 0, 0, 0)
			if block == blockLeaves {
				rl.EnableBackfaceCulling()
			}

			rl.EndMode3D()
		}
		rl.DisableDepthTest()
		rl.EndTextureMode()
		rl.SetTextureFilter(rt.Texture, rl.FilterPoint)
		a.iconRenders[block] = rt
		a.iconOffsets[block] = measureIconOffset(rt.Texture)
	}
	return rt
}

func measureIconOffset(tex rl.Texture2D) rl.Vector2 {
	img := rl.LoadImageFromTexture(tex)
	if img == nil || img.Data == nil {
		if img != nil {
			rl.UnloadImage(img)
		}
		return rl.NewVector2(0, 0)
	}
	colors := rl.LoadImageColors(img)
	w := int(img.Width)
	h := int(img.Height)
	minX := w
	minY := h
	maxX := -1
	maxY := -1
	if len(colors) >= w*h {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := colors[y*w+x]
				if c.A > 5 {
					if x < minX {
						minX = x
					}
					if y < minY {
						minY = y
					}
					if x > maxX {
						maxX = x
					}
					if y > maxY {
						maxY = y
					}
				}
			}
		}
	}
	rl.UnloadImageColors(colors)
	rl.UnloadImage(img)
	if maxX < 0 || maxY < 0 {
		return rl.NewVector2(0, 0)
	}
	centerX := float32(minX+maxX+1) * 0.5
	centerY := float32(minY+maxY+1) * 0.5
	texCenterX := float32(w) * 0.5
	texCenterY := float32(h) * 0.5
	return rl.NewVector2(centerX-texCenterX, centerY-texCenterY)
}

func (a *RenderAssets) generateAtlas() {
	// Identify all unique block textures needed
	paths := []string{
		"textures/block/stone.png",
		"textures/block/dirt.png",
		"textures/block/grass_block_top.png",
		"textures/block/grass_block_side.png",
		"textures/block/cobblestone.png",
		"textures/block/oak_planks.png",
		"textures/block/bedrock.png",
		"textures/block/sand.png",
		"textures/block/gravel.png",
		"textures/block/gold_ore.png",
		"textures/block/iron_ore.png",
		"textures/block/coal_ore.png",
		"textures/block/oak_log.png",
		"textures/block/oak_log_top.png",
		"textures/block/oak_leaves.png", // cutout
		"textures/block/glass.png",      // glass
		"textures/block/diamond_ore.png",
		"textures/block/lapis_ore.png",
		"textures/block/torch.png", // cutout
		"textures/block/crafting_table_top.png",
		"textures/block/crafting_table_side.png",
		"textures/block/crafting_table_front.png",
		"textures/block/furnace_front.png",
		"textures/block/furnace_side.png",
		"textures/block/furnace_top.png",
		"textures/block/tnt_side.png",
		"textures/block/tnt_top.png",
		"textures/block/tnt_bottom.png",
	}

	// Dynamic sizing
	count := len(paths)
	cols := 0
	rows := 0
	size := 16 // Standard block size

	// Simple sqrt approximation for grid
	side := 1
	for side*side < count {
		side++
	}
	cols = side
	rows = side

	atlasWidth := int32(cols * size)
	atlasHeight := int32(rows * size)

	atlasImg := rl.GenImageColor(int(atlasWidth), int(atlasHeight), rl.Blank)
	uvs := map[string]rl.Rectangle{}

	for i, path := range paths {
		var img *rl.Image

		// Special handling for torch (resize canvas/center)
		if path == "textures/block/torch.png" {
			raw := rl.LoadImage(path)
			if raw != nil {
				img = rl.GenImageColor(16, 16, rl.Blank)
				src := rl.NewRectangle(0, 0, float32(raw.Width), float32(raw.Height))
				dst := rl.NewRectangle(7, 0, float32(raw.Width), float32(raw.Height)) // Center torch
				rl.ImageDraw(img, raw, src, dst, rl.White)
				rl.UnloadImage(raw)
			}
		} else if path == "textures/block/oak_leaves.png" || path == "textures/block/grass_block_side_overlay.png" {
			img = rl.LoadImage(path)
			// Apply alpha fix similar to loadTexture
			if img != nil && img.Data != nil {
				colors := rl.LoadImageColors(img)
				hasAlpha := false
				for _, c := range colors {
					if c.A < 255 {
						hasAlpha = true
						break
					}
				}
				if !hasAlpha {
					key := pickEdgeColorKey(colors, int(img.Width), int(img.Height))
					rl.ImageAlphaClear(img, key, 0.05)
				}
				rl.UnloadImageColors(colors)
			}
		} else if path == "textures/block/glass.png" {
			img = rl.LoadImage(path)
			// Apply glass alpha fix
			if img != nil && img.Data != nil {
				colors := rl.LoadImageColors(img)
				if len(colors) > 0 {
					w := int(img.Width)
					h := int(img.Height)
					for y := 0; y < h; y++ {
						for x := 0; x < w; x++ {
							idx := y*w + x
							c := colors[idx]
							if c.A > 0 && c.A < 255 {
								c.A = 255
								rl.ImageDrawPixel(img, int32(x), int32(y), c)
							}
						}
					}
					rl.UnloadImageColors(colors)
				}
			}
		} else {
			img = rl.LoadImage(path)
		}

		if img == nil || img.Data == nil {
			if img != nil {
				rl.UnloadImage(img)
			}
			continue
		}

		if img.Width != 16 || img.Height != 16 {
			rl.ImageCrop(img, rl.NewRectangle(0, 0, 16, 16))
		}

		// Calculate pos
		col := i % cols
		row := i / cols
		x := int32(col * size)
		y := int32(row * size)

		// Draw to atlas
		src := rl.NewRectangle(0, 0, float32(img.Width), float32(img.Height))
		dst := rl.NewRectangle(float32(x), float32(y), float32(img.Width), float32(img.Height))
		rl.ImageDraw(atlasImg, img, src, dst, rl.White)

		// Calculate UV (normalized 0-1)
		uvX := float32(x) / float32(atlasWidth)
		uvY := float32(y) / float32(atlasHeight)
		uvW := float32(img.Width) / float32(atlasWidth)
		uvH := float32(img.Height) / float32(atlasHeight)

		uvs[path] = rl.NewRectangle(uvX, uvY, uvW, uvH)

		rl.UnloadImage(img)
	}

	tex := rl.LoadTextureFromImage(atlasImg)
	rl.SetTextureFilter(tex, rl.FilterPoint)
	rl.UnloadImage(atlasImg)

	a.atlas = &TextureAtlas{
		Texture: tex,
		UVs:     uvs,
	}
}

func (a *RenderAssets) getAtlasUV(path string) (rl.Rectangle, bool) {
	if a.atlas == nil {
		return rl.NewRectangle(0, 0, 1, 1), false
	}
	uv, ok := a.atlas.UVs[path]
	return uv, ok
}

func (a *RenderAssets) loadFogShader() rl.Shader {
	vertex := `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec4 vertexColor;
out vec2 fragTexCoord;
out vec4 fragColor;
out float fragDist;
uniform mat4 mvp;
uniform mat4 matModel;
uniform vec3 viewPos;
void main()
{
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    vec4 worldPos = matModel * vec4(vertexPosition, 1.0);
    fragDist = length(worldPos.xyz - viewPos);
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`
	fragment := `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
in float fragDist;
out vec4 finalColor;
uniform sampler2D texture0;
uniform vec4 colDiffuse;
uniform vec4 fogColor;
uniform float fogDensity;
uniform int fogMode; // 0: linear, 1: exp, 2: exp2

void main()
{
    vec4 texelColor = texture(texture0, fragTexCoord);
    vec4 color = texelColor * colDiffuse * fragColor;
    if (color.a < 0.5) discard;

    float fogFactor = 0.0;
    if (fogMode == 0) {
        float fogStart = 160.0; // Increased from 48.0
        float fogEnd = 240.0;   // Increased from 160.0 to match 16 chunks * 16 blocks = 256
        fogFactor = (fogEnd - fragDist) / (fogEnd - fogStart);
    } else if (fogMode == 1) {
        fogFactor = exp(-fragDist * fogDensity);
    } else if (fogMode == 2) {
        fogFactor = exp(-pow(fragDist * fogDensity, 2.0));
    }
    fogFactor = clamp(fogFactor, 0.0, 1.0);
    
    finalColor = mix(fogColor, color, fogFactor);
}`
	return rl.LoadShaderFromMemory(vertex, fragment)
}
