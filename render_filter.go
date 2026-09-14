package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"gocraft/platform"
)

const atlasTileSize = 16
const atlasCellSize = 64
const atlasPadding = 16
const atlasMaxMipLevel = 4

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
	if texture.Mipmaps > 1 {
		rl.TextureParameters(texture.ID, rl.TextureMinFilter, rl.TextureFilterNearestMipLinear)
	}
	if atlas {
		platform.SetTextureMaxLevel(texture.ID, atlasMaxMipLevel)
	}
}

func atlasSourceCoordinate(pixel int) int {
	return max(0, min(atlasTileSize-1, pixel-atlasPadding))
}
