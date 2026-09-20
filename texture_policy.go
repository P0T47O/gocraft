package main

const atlasTileSize = 16
const atlasCellSize = 256
const atlasPadding = 112
const atlasMaxMipLevel = 4

func normalizeAnisotropy(value int) int {
	level := 1
	for level < 16 && level*2 <= value {
		level *= 2
	}
	return level
}

func safeAtlasMipLevel(anisotropy int) int {
	level := atlasMaxMipLevel
	padding := min(atlasPadding, atlasCellSize-atlasPadding-atlasTileSize)
	for level > 0 && padding/(1<<level) < anisotropy/2+1 {
		level--
	}
	return level
}

func isAnimatedTexture(path string) bool {
	switch path {
	case "textures/block/water_still.png", "textures/block/water_flow.png", "textures/block/lava_still.png", "textures/block/lava_flow.png":
		return true
	}
	return false
}
