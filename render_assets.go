package main

// RenderAssets contains immutable CPU atlas metadata shared with mesh workers.
// GPU textures, samplers, pipelines and buffers belong to the WebGPU renderer.
type RenderAssets struct{ atlas *TextureAtlas }

type AtlasRect struct{ X, Y, Width, Height float32 }
type TextureAtlas struct{ UVs map[string]AtlasRect }

func newWebGPUCPUAssets() *RenderAssets { return &RenderAssets{atlas: &TextureAtlas{}} }
func (a *RenderAssets) unload()         { a.atlas = nil }
func (a *RenderAssets) getAtlasUV(path string) (AtlasRect, bool) {
	if a.atlas == nil {
		return AtlasRect{Width: 1, Height: 1}, false
	}
	uv, ok := a.atlas.UVs[path]
	return uv, ok
}
