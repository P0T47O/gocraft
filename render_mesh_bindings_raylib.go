package main

import rl "github.com/gen2brain/raylib-go/raylib"

// legacyMeshBindings is never allowed to allocate textures on the native path.
func (a *RenderAssets) legacyMeshBindings(path string) (uint32, uint32) {
	if a.webGPU {
		return 0, 0
	}
	var texture rl.Texture2D
	if path == "atlas" && a.atlas != nil {
		texture = a.atlas.Texture
	} else {
		texture = a.loadTexture(path)
	}
	material := a.getMaterial(path, texture)
	return texture.ID, material.Shader.ID
}
