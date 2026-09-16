//go:build windows

package main

import (
	"fmt"

	"github.com/gogpu/gputypes"
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
		if err := r.queue.WriteTexture(
			&wgpu.ImageCopyTexture{
				Texture:  r.atlasTexture,
				MipLevel: 0,
				Origin: wgpu.Origin3D{
					X: animation.atlasX,
					Y: animation.atlasY,
					Z: 0,
				},
			},
			animation.frames[index],
			&wgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  animation.bytesPerRow,
				RowsPerImage: animation.height,
			},
			&wgpu.Extent3D{
				Width:              animation.width,
				Height:             animation.height,
				DepthOrArrayLayers: 1,
			},
		); err != nil {
			return fmt.Errorf("upload animated atlas frame: %w", err)
		}
		animation.index = index
	}
	return nil
}

func resetWebGPUAtlasAnimations() {
	activeWebGPUAtlasAnimations = nil
}
