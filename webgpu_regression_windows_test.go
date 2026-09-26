//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
	"math"
	"os"
	"testing"
	"time"
)

func TestWebGPUFilterDescriptor(t *testing.T) {
	for _, mips := range []bool{false, true} {
		for _, af := range []int{1, 2, 4, 8, 16} {
			d := webGPUAtlasSamplerDescriptor(mips, af)
			if !mips && d.LodMaxClamp != 0 {
				t.Fatal("mip disable ignored")
			}
			if mips && d.LodMaxClamp != float32(safeAtlasMipLevel(af)) {
				t.Fatal("unsafe mip clamp")
			}
			if af > 1 && (d.MinFilter != gputypes.FilterModeLinear || d.MagFilter != gputypes.FilterModeLinear || d.MipmapFilter != gputypes.FilterModeLinear) {
				t.Fatal("invalid anisotropy filters")
			}
		}
	}
}

// Real offscreen GPU readback, no window, save or settings file is touched.
func TestWebGPUUIUploadIsolationGPU(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_REGRESSION=1")
	}
	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Release()
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{PowerPreference: gputypes.PowerPreferenceHighPerformance})
	if err != nil {
		t.Fatal(err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Release()
	t.Logf("GPU: %+v", adapter.Info())
	r := &webGPUWorldRenderer{device: device, queue: device.Queue(), format: gputypes.TextureFormatRGBA8Unorm}
	defer func() { r.device = nil; r.closeResources(false) }()
	atlas := &webGPUBlockAtlas{width: 256, height: 256, pixels: make([]byte, 256*256*4)}
	for i := range atlas.pixels {
		atlas.pixels[i] = 255
	}
	if err := r.createPipelineResources(atlas); err != nil {
		t.Fatal(err)
	}
	defer closeWebGPUHUDRenderer()
	hud, err := ensureWebGPUHUDRenderer(r)
	if err != nil {
		t.Fatal(err)
	}
	oldSettings := currentSettings
	defer func() { currentSettings = oldSettings }()
	for _, mips := range []bool{true, false} {
		for _, af := range []int{1, 2, 4, 8, 16} {
			currentSettings = &GameSettings{Mipmaps: mips, Anisotropy: af}
			if err := r.updateFiltering(); err != nil {
				t.Fatalf("mips %v AF %d: %v", mips, af, err)
			}
		}
	}
	const width, height = 64, 32
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{Size: wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: r.format, Usage: gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc})
	if err != nil {
		t.Fatal(err)
	}
	defer texture.Release()
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Release()
	readback, err := device.CreateBuffer(&wgpu.BufferDescriptor{Size: width * height * 4, Usage: gputypes.BufferUsageMapRead | gputypes.BufferUsageCopyDst})
	if err != nil {
		t.Fatal(err)
	}
	defer readback.Release()
	for frame := 0; frame < 2; frame++ {
		hud.uploads.begin()
		enc, err := device.CreateCommandEncoder(nil)
		if err != nil {
			t.Fatal(err)
		}
		pass, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}}})
		if err != nil {
			t.Fatal(err)
		}
		left := newWebGPUHUDBuilder(width, height)
		left.rect(0, 0, 32, 32, [4]float32{1, 0, 0, 1})
		right := newWebGPUHUDBuilder(width, height)
		right.rect(32, 0, 32, 32, [4]float32{0, 1, 0, 1})
		if err := hud.drawPrepared(pass, left); err != nil {
			t.Fatal(err)
		}
		if err := hud.drawPrepared(pass, right); err != nil {
			t.Fatal(err)
		}
		if err := pass.End(); err != nil {
			t.Fatal(err)
		}
		renderCommands, err := enc.Finish()
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.queue.Submit(renderCommands)
		renderCommands.Release()
		if err != nil {
			t.Fatal(err)
		}
		enc, err = device.CreateCommandEncoder(nil)
		if err != nil {
			t.Fatal(err)
		}
		enc.CopyTextureToBuffer(texture, readback, []wgpu.BufferTextureCopy{{BufferLayout: wgpu.ImageDataLayout{BytesPerRow: 256, RowsPerImage: height}, TextureBase: wgpu.ImageCopyTexture{Texture: texture}, Size: wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1}}})
		cb, err := enc.Finish()
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.queue.Submit(cb)
		cb.Release()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = readback.Map(ctx, wgpu.MapModeRead, 0, width*height*4)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		range_, err := readback.MappedRange(0, width*height*4)
		if err != nil {
			t.Fatal(err)
		}
		pixels := range_.Bytes()
		li, ri := (16*width+16)*4, (16*width+48)*4
		if pixels[li] < 250 || pixels[li+1] > 5 || pixels[ri] > 5 || pixels[ri+1] < 250 {
			t.Fatalf("frame %d overwritten draws: left=%v right=%v", frame, pixels[li:li+4], pixels[ri:ri+4])
		}
		if err := readback.Unmap(); err != nil {
			t.Fatal(err)
		}
	}
	if len(hud.uploads.slots) != 2 {
		t.Fatal("frame buffers were not reused")
	}
	// Validate the actual 24-byte world pipeline and outward-facing triangle
	// with backface culling, not just the CPU layout declaration.
	backend := platform.EnableExperimentalWebGPU(device)
	defer platform.SetMeshBackend(nil)
	mesh, err := backend.UploadChecked([]platform.Vertex{
		{Position: [3]float32{-.8, -.8, .5}, Color: [4]byte{0, 0, 255, 255}},
		{Position: [3]float32{.8, -.8, .5}, Color: [4]byte{0, 0, 255, 255}},
		{Position: [3]float32{0, .8, .5}, Color: [4]byte{0, 0, 255, 255}},
	}, []uint32{0, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	defer mesh.Unload()
	scene := make([]byte, webGPUWorldSceneBytes)
	for _, offset := range []int{0, 20, 40, 60} {
		binary.LittleEndian.PutUint32(scene[offset:], math.Float32bits(1))
	}
	binary.LittleEndian.PutUint32(scene[80:], math.Float32bits(100))
	binary.LittleEndian.PutUint32(scene[84:], math.Float32bits(200))
	if err := r.queue.WriteBuffer(r.sceneBuffer, 0, scene); err != nil {
		t.Fatal(err)
	}
	depth, depthView, err := createWebGPUChunkPreviewDepth(device, width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer depth.Release()
	defer depthView.Release()
	enc, err := device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatal(err)
	}
	pass, err := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments:       []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	pass.SetPipeline(r.solidPipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	if err := backend.DrawPass(pass, mesh); err != nil {
		t.Fatal(err)
	}
	if err := pass.End(); err != nil {
		t.Fatal(err)
	}
	cb, err := enc.Finish()
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.queue.Submit(cb)
	cb.Release()
	if err != nil {
		t.Fatal(err)
	}
	enc, err = device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatal(err)
	}
	enc.CopyTextureToBuffer(texture, readback, []wgpu.BufferTextureCopy{{BufferLayout: wgpu.ImageDataLayout{BytesPerRow: 256, RowsPerImage: height}, TextureBase: wgpu.ImageCopyTexture{Texture: texture}, Size: wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1}}})
	cb, err = enc.Finish()
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.queue.Submit(cb)
	cb.Release()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := readback.Map(ctx, wgpu.MapModeRead, 0, width*height*4); err != nil {
		t.Fatal(err)
	}
	result, err := readback.MappedRange(0, width*height*4)
	if err != nil {
		t.Fatal(err)
	}
	pixel := result.Bytes()[(16*width+32)*4:][:4]
	if pixel[0] > 5 || pixel[1] > 5 || pixel[2] < 250 {
		t.Fatalf("compact/culling triangle missing: %v", pixel)
	}
	if err := readback.Unmap(); err != nil {
		t.Fatal(err)
	}
	// More than the former entity budget must be split into distinct buffers,
	// never overwrite the earlier batch or fail the game frame.
	previousAssets, previousEntities := assets, remoteEntities
	defer func() { assets = previousAssets; remoteEntities = previousEntities }()
	assets = newWebGPUCPUAssets()
	initBlockRegistry()
	assets.atlas.UVs = map[string]AtlasRect{GetBlock(blockIronOre).Textures.Top: {Width: 1, Height: 1}}
	remoteEntities = make(map[string]*RemoteEntity)
	for i := 0; i < 1500; i++ {
		remoteEntities[fmt.Sprint(i)] = &RemoteEntity{Type: EntityPlayer}
	}
	entities, err := ensureWebGPUEntityRenderer(r)
	if err != nil {
		t.Fatal(err)
	}
	defer closeWebGPUEntityRenderer()
	enc, err = device.CreateCommandEncoder(nil)
	if err != nil {
		t.Fatal(err)
	}
	pass, err = enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments:       []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := entities.Draw(pass, r, nil, 0); err != nil {
		t.Fatal(err)
	}
	if len(entities.vertices.slots) < 2 {
		t.Fatal("entity stress did not exercise multiple batches")
	}
	if err := pass.End(); err != nil {
		t.Fatal(err)
	}
	cb, err = enc.Finish()
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.queue.Submit(cb)
	cb.Release()
	if err != nil {
		t.Fatal(err)
	}
	device.Poll(wgpu.PollWait)
}
