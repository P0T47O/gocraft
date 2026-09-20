package main

import (
	"fmt"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type RenderAssets struct {
	// CPU atlas metadata only in WebGPU sessions; legacy GPU resources stay lazy.
	webGPU          bool
	mu              sync.RWMutex
	textures        map[string]rl.Texture2D
	faceMeshes      map[string]faceMesh
	faceModels      map[string]rl.Model
	iconRenders     map[byte]rl.RenderTexture2D
	iconOffsets     map[byte]rl.Vector2
	animated        map[string]*AnimatedTexture
	materials       map[string]rl.Material
	crossItemModels map[byte]rl.Model // Pre-generated extruded meshes for cross-type items
	iconCamera      rl.Camera3D
	cutoutShader    rl.Shader
	fogShader       rl.Shader
	atlas           *TextureAtlas
	CrackTextures   [10]rl.Texture2D
	crackModel      rl.Model
}

type AnimatedTexture struct {
	Frames       []rl.Texture2D
	FrameSeconds float32
	Time         float32
	Index        int
}

type AtlasRect struct {
	X, Y, Width, Height float32
}

type TextureAtlas struct {
	Texture rl.Texture2D
	UVs     map[string]AtlasRect
}

// WebGPU atlas construction installs CPU UV metadata before workers start.
// No legacy textures, models, icon render targets or shaders are created.
func newWebGPUCPUAssets() *RenderAssets {
	return &RenderAssets{webGPU: true, atlas: &TextureAtlas{}}
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

	assets.loadBlockTextures()
	// Explicitly load grass side overlay for multi-pass rendering
	assets.loadTexture("textures/block/grass_block_side_overlay.png")

	// Load crack animation textures
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("textures/block/destroy_stage_%d.png", i)
		assets.CrackTextures[i] = assets.loadTexture(path)
	}

	assets.generateAtlas()
	assets.cutoutShader = loadCutoutShader()
	assets.fogShader = assets.loadFogShader()
	assets.initFaceMeshes()
	assets.initFaceModels()
	assets.initIcons()
	assets.initCrossItemModels()

	// Initialize crack model (unit cube)
	cubeMesh := rl.GenMeshCube(1.0, 1.0, 1.0)
	assets.crackModel = rl.LoadModelFromMesh(cubeMesh)

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
