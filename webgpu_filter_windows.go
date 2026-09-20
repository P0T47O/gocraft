//go:build windows

package main

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func webGPUFilterSettings() (bool, int) {
	if currentSettings == nil {
		return true, 8
	}
	return currentSettings.Mipmaps, normalizeAnisotropy(currentSettings.Anisotropy)
}

func webGPUAtlasSamplerDescriptor(mipmaps bool, af int) *wgpu.SamplerDescriptor {
	d := &wgpu.SamplerDescriptor{
		Label: "GoCraft world filtering", AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest, MinFilter: gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest, Anisotropy: uint16(normalizeAnisotropy(af)),
	}
	if mipmaps {
		d.LodMaxClamp = float32(safeAtlasMipLevel(af))
		d.MipmapFilter = gputypes.FilterModeLinear
	}
	// WebGPU requires all three filters linear when anisotropy is enabled.
	if d.Anisotropy > 1 {
		d.MagFilter, d.MinFilter, d.MipmapFilter = gputypes.FilterModeLinear, gputypes.FilterModeLinear, gputypes.FilterModeLinear
	}
	return d
}

func (r *webGPUWorldRenderer) updateFiltering() error {
	mips, af := webGPUFilterSettings()
	if r.filterMipmaps == mips && r.filterAF == af {
		return nil
	}
	sampler, err := r.device.CreateSampler(webGPUAtlasSamplerDescriptor(mips, af))
	if err != nil {
		return err
	}
	group, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label: "GoCraft world filtering", Layout: r.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: r.sceneBuffer, Size: webGPUWorldSceneBytes},
			{Binding: 1, TextureView: r.filteredAtlasView(mips)}, {Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		sampler.Release()
		return err
	}
	r.bindGroup.Release()
	r.atlasSampler.Release()
	r.bindGroup, r.atlasSampler = group, sampler
	r.filterMipmaps, r.filterAF = mips, af
	return nil
}

// The pinned Vulkan HAL treats LodMaxClamp==0 as unlimited. Restrict the
// accessible subresources instead of relying on that sampler clamp.
func (r *webGPUWorldRenderer) filteredAtlasView(mipmaps bool) *wgpu.TextureView {
	if !mipmaps {
		return r.atlasBaseView
	}
	return r.atlasView
}
