//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"gocraft/platform"
	"math"
	"os"
	"testing"
	"time"
)

// Exercise real textured draws over several submissions and animation updates:
// flat-color tests cannot detect incorrect mip selection or texture corruption.
func TestWebGPUMipStabilityGPU(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_REGRESSION=1")
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	instance, err := wgpu.CreateInstance(nil)
	check(err)
	defer instance.Release()
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{PowerPreference: gputypes.PowerPreferenceHighPerformance})
	check(err)
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	check(err)
	defer device.Release()
	r := &webGPUWorldRenderer{device: device, queue: device.Queue(), format: gputypes.TextureFormatRGBA8Unorm}
	defer func() { r.device = nil; r.closeResources(false) }()
	settings, animations := currentSettings, activeWebGPUAtlasAnimations
	defer func() { currentSettings = settings; activeWebGPUAtlasAnimations = animations }()
	currentSettings = &GameSettings{Mipmaps: true, Anisotropy: 8}
	atlas := &webGPUBlockAtlas{width: 512, height: 256, pixels: make([]byte, 512*256*4)}
	for y := 0; y < 256; y++ {
		for x := 0; x < 512; x++ {
			value := byte(0)
			if (webGPUAtlasSourceCoordinate(x%256, 16)+webGPUAtlasSourceCoordinate(y, 16))%2 == 0 {
				value = 255
			}
			i := (y*512 + x) * 4
			copy(atlas.pixels[i:i+4], []byte{value, value, value, 255})
		}
	}
	check(r.createPipelineResources(atlas))
	frameA, frameB := make([]byte, 16*256), make([]byte, 16*256)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			i := y*256 + x*4
			copy(frameA[i:i+4], []byte{255, 0, 0, 255})
			copy(frameB[i:i+4], []byte{0, 255, 0, 255})
		}
	}
	activeWebGPUAtlasAnimations = []webGPUAtlasAnimation{{frames: [][]byte{frameA, frameB}, width: 16, height: 16, bytesPerRow: 256, atlasX: 256 + atlasPadding, atlasY: atlasPadding, frameSeconds: .1}}
	const size = 64
	target, err := device.CreateTexture(&wgpu.TextureDescriptor{Size: wgpu.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: r.format, Usage: gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc})
	check(err)
	defer target.Release()
	view, err := device.CreateTextureView(target, nil)
	check(err)
	defer view.Release()
	depth, dv, err := createWebGPUChunkPreviewDepth(device, size, size)
	check(err)
	defer depth.Release()
	defer dv.Release()
	readback, err := device.CreateBuffer(&wgpu.BufferDescriptor{Size: size * size * 4, Usage: gputypes.BufferUsageMapRead | gputypes.BufferUsageCopyDst})
	check(err)
	defer readback.Release()
	backend := platform.EnableExperimentalWebGPU(device)
	defer platform.SetMeshBackend(nil)
	mesh, err := backend.UploadChecked([]platform.Vertex{
		{Position: [3]float32{-1, -1, .5}, Texcoord: [2]float32{112. / 512, 128. / 256}, Color: [4]byte{255, 255, 255, 255}},
		{Position: [3]float32{1, -1, .5}, Texcoord: [2]float32{128. / 512, 128. / 256}, Color: [4]byte{255, 255, 255, 255}},
		{Position: [3]float32{1, 1, .5}, Texcoord: [2]float32{128. / 512, 112. / 256}, Color: [4]byte{255, 255, 255, 255}},
		{Position: [3]float32{-1, 1, .5}, Texcoord: [2]float32{112. / 512, 112. / 256}, Color: [4]byte{255, 255, 255, 255}},
	}, []uint32{0, 1, 2, 0, 2, 3})
	check(err)
	defer mesh.Unload()
	for frame := 0; frame < 11; frame++ {
		if frame == 8 {
			// Color-code levels: minification must choose blue mip levels when
			// enabled, but red base level when disabled, then blue on re-enable.
			for level := 0; level <= atlasMaxMipLevel; level++ {
				w, h := 512>>level, 256>>level
				pixels := make([]byte, w*h*4)
				for i := 0; i < len(pixels); i += 4 {
					pixels[i+3] = 255
					if level == 0 {
						pixels[i] = 255
					} else {
						pixels[i+2] = 255
					}
				}
				check(r.queue.WriteTexture(&wgpu.ImageCopyTexture{Texture: r.atlasTexture, MipLevel: uint32(level)}, pixels, &wgpu.ImageDataLayout{BytesPerRow: uint32(w * 4), RowsPerImage: uint32(h)}, &wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}))
			}
		}
		if frame >= 8 {
			currentSettings = &GameSettings{Mipmaps: frame != 9, Anisotropy: 1}
			check(r.updateFiltering())
		}
		scene := make([]byte, webGPUWorldSceneBytes)
		for _, offset := range []int{0, 20, 40, 60} {
			binary.LittleEndian.PutUint32(scene[offset:], math.Float32bits(1))
		}
		if frame < 8 {
			binary.LittleEndian.PutUint32(scene[48:], math.Float32bits(float32(frame)*.004))
		} else {
			binary.LittleEndian.PutUint32(scene[0:], math.Float32bits(.0625))
			binary.LittleEndian.PutUint32(scene[20:], math.Float32bits(.0625))
		}
		binary.LittleEndian.PutUint32(scene[80:], math.Float32bits(100))
		binary.LittleEndian.PutUint32(scene[84:], math.Float32bits(200))
		check(r.queue.WriteBuffer(r.sceneBuffer, 0, scene))
		if frame > 0 && frame < 8 {
			check(updateWebGPUAtlasAnimations(r, float32(frame)*.11))
		}
		encoder, err := device.CreateCommandEncoder(nil)
		check(err)
		pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}}, DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: dv, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 1}})
		check(err)
		pass.SetPipeline(r.solidPipeline)
		pass.SetBindGroup(0, r.bindGroup, nil)
		check(backend.DrawPass(pass, mesh))
		check(pass.End())
		cb, err := encoder.Finish()
		check(err)
		_, err = r.queue.Submit(cb)
		cb.Release()
		check(err)
		encoder, err = device.CreateCommandEncoder(nil)
		check(err)
		encoder.CopyTextureToBuffer(target, readback, []wgpu.BufferTextureCopy{{BufferLayout: wgpu.ImageDataLayout{BytesPerRow: 256, RowsPerImage: size}, TextureBase: wgpu.ImageCopyTexture{Texture: target}, Size: wgpu.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1}}})
		cb, err = encoder.Finish()
		check(err)
		_, err = r.queue.Submit(cb)
		cb.Release()
		check(err)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = readback.Map(ctx, wgpu.MapModeRead, 0, size*size*4)
		cancel()
		check(err)
		mapped, err := readback.MappedRange(0, size*size*4)
		check(err)
		pixels := mapped.Bytes()
		if frame >= 8 {
			center := append([]byte(nil), pixels[(32*size+32)*4:][:4]...)
			check(readback.Unmap())
			wantChannel := 2
			if frame == 9 {
				wantChannel = 0
			}
			if center[wantChannel] < 240 {
				t.Fatalf("mipmap toggle frame %d sampled wrong LOD: %v", frame, center)
			}
			t.Logf("mipmap toggle frame %d pixel %v", frame, center)
			continue
		}
		lo, hi := 255, 0
		for y := 8; y < 56; y++ {
			for x := 8; x < 56; x++ {
				v := int(pixels[(y*size+x)*4])
				lo = min(lo, v)
				hi = max(hi, v)
			}
		}
		check(readback.Unmap())
		t.Logf("frame %d checker range %d..%d", frame, lo, hi)
		if hi-lo < 240 {
			t.Fatalf("magnified pixel art lost detail after frame %d", frame)
		}
	}
}
