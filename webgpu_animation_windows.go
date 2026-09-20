//go:build windows

package main

import (
	"fmt"

	"github.com/gogpu/wgpu"
)

// updateWebGPUAtlasAnimations advances atlas-backed animated block textures
// using the same frame timing as the reference renderer. Only the changed tile
// is uploaded; chunk meshes and UVs remain untouched.
func updateWebGPUAtlasAnimations(r *webGPUWorldRenderer, now float32) error {
	if r == nil || r.queue == nil || r.atlasTexture == nil {
		return nil
	}
	for i := range activeWebGPUAtlasAnimations {
		animation := &activeWebGPUAtlasAnimations[i]
		if len(animation.frames) <= 1 || animation.frameSeconds <= 0 {
			continue
		}
		index := int(now/animation.frameSeconds) % len(animation.frames)
		if index < 0 {
			index += len(animation.frames)
		}
		if index == animation.index {
			continue
		}
		for level, mip := range animation.mipFrame(index) {
			if err := r.queue.WriteTexture(
				&wgpu.ImageCopyTexture{Texture: r.atlasTexture, MipLevel: uint32(level), Origin: wgpu.Origin3D{
					X: (animation.atlasX - atlasPadding) >> uint(level), Y: (animation.atlasY - atlasPadding) >> uint(level),
				}}, mip.pixels,
				&wgpu.ImageDataLayout{BytesPerRow: uint32(mip.width * 4), RowsPerImage: uint32(mip.height)},
				&wgpu.Extent3D{Width: uint32(mip.width), Height: uint32(mip.height), DepthOrArrayLayers: 1},
			); err != nil {
				return fmt.Errorf("upload animated atlas mip: %w", err)
			}
		}
		animation.index = index
	}
	return nil
}

func resetWebGPUAtlasAnimations() {
	activeWebGPUAtlasAnimations = nil
}
