package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"gocraft/platform"
)

var textureAnisotropyLimit int

func supportedAnisotropy() int {
	if textureAnisotropyLimit == 0 {
		textureAnisotropyLimit = normalizeAnisotropy(platform.MaxTextureAnisotropy())
	}
	return textureAnisotropyLimit
}

// Keep magnification pixel-sharp; blend mip levels during minification.
// UI render targets retain their original point sampling.
func configureWorldTexture(texture *rl.Texture2D, atlas bool) {
	if texture.ID == 0 {
		return
	}
	if texture.Mipmaps <= 1 {
		rl.GenTextureMipmaps(texture)
	}
	rl.SetTextureFilter(*texture, rl.FilterPoint)
	// FilterPoint selects NEAREST_MIPMAP_NEAREST when mip storage exists.
	// Explicitly disable mip sampling before applying the selected mode.
	rl.TextureParameters(texture.ID, rl.TextureMinFilter, rl.TextureFilterNearest)
	mipmaps, anisotropy := true, 8
	if currentSettings != nil {
		mipmaps, anisotropy = currentSettings.Mipmaps, currentSettings.Anisotropy
	}
	anisotropy = min(normalizeAnisotropy(anisotropy), supportedAnisotropy())
	if mipmaps && texture.Mipmaps > 1 {
		rl.TextureParameters(texture.ID, rl.TextureMinFilter, rl.TextureFilterNearestMipLinear)
		if anisotropy > 1 {
			rl.TextureParameters(texture.ID, rl.TextureMinFilter, rl.TextureFilterMipLinear)
		}
	} else if anisotropy > 1 {
		rl.TextureParameters(texture.ID, rl.TextureMinFilter, rl.TextureFilterLinear)
	}
	if atlas {
		platform.SetTextureMaxLevel(texture.ID, safeAtlasMipLevel(anisotropy))
	}
	// rl.TextureParameters resets anisotropy on other parameter changes: apply last.
	if supportedAnisotropy() > 1 {
		rl.TextureParameters(texture.ID, rl.TextureFilterAnisotropic, int32(anisotropy))
	}
}

// Called from settings UI on the render thread, between batches. Retain mip
// storage while disabled so toggling never reallocates the world's textures.
func applyWorldTextureFiltering() {
	if assets == nil || assets.webGPU {
		return
	}
	rl.DrawRenderBatchActive()
	for path, texture := range assets.textures {
		configureWorldTexture(&texture, false)
		assets.textures[path] = texture
	}
	for _, animation := range assets.animated {
		for i := range animation.Frames {
			configureWorldTexture(&animation.Frames[i], false)
		}
	}
	if assets.atlas != nil {
		configureWorldTexture(&assets.atlas.Texture, true)
	}
}

func atlasSourceCoordinate(pixel int) int {
	return max(0, min(atlasTileSize-1, pixel-atlasPadding))
}
