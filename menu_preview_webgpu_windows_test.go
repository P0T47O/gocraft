//go:build windows

package main

import (
	"context"
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Optional diagnostic: render the exact native menu command stream offscreen.
func captureNativePreview(t *testing.T, r *webGPUWorldRenderer, width, height uint32, draw func(*wgpu.RenderPassEncoder) error) *image.RGBA {
	t.Helper()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(r.prepareSurface(width, height))
	if activeWebGPUHUDRenderer != nil {
		activeWebGPUHUDRenderer.uploads.begin()
	}
	if activeWebGPUTextRenderer != nil {
		activeWebGPUTextRenderer.uploads.begin()
	}
	texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{Size: wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: r.format, Usage: gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc})
	check(err)
	defer texture.Release()
	view, err := r.device.CreateTextureView(texture, nil)
	check(err)
	defer view.Release()
	stride := (width*4 + 255) &^ uint32(255)
	size := uint64(stride) * uint64(height)
	buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{Size: size, Usage: gputypes.BufferUsageMapRead | gputypes.BufferUsageCopyDst})
	check(err)
	defer buffer.Release()
	encoder, err := r.device.CreateCommandEncoder(nil)
	check(err)
	sky := r.sceneSky
	if sky == ([3]float32{}) {
		sky = worldDaylight(6000).Sky
	}
	// Gameplay uses reverse-Z depth (Greater), so an offscreen world preview
	// must clear depth to zero just like the live gameplay pass.
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore, ClearValue: gputypes.Color{R: float64(sky[0]), G: float64(sky[1]), B: float64(sky[2]), A: 1}}}, DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: r.depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 0}})
	check(err)
	check(draw(pass))
	check(pass.End())
	commands, err := encoder.Finish()
	check(err)
	_, err = r.queue.Submit(commands)
	commands.Release()
	check(err)
	encoder, err = r.device.CreateCommandEncoder(nil)
	check(err)
	encoder.CopyTextureToBuffer(texture, buffer, []wgpu.BufferTextureCopy{{BufferLayout: wgpu.ImageDataLayout{BytesPerRow: stride, RowsPerImage: height}, TextureBase: wgpu.ImageCopyTexture{Texture: texture}, Size: wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1}}})
	commands, err = encoder.Finish()
	check(err)
	_, err = r.queue.Submit(commands)
	commands.Release()
	check(err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	check(buffer.Map(ctx, wgpu.MapModeRead, 0, size))
	mapped, err := buffer.MappedRange(0, size)
	check(err)
	pixels := mapped.Bytes()
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := uint32(0); y < height; y++ {
		for x := uint32(0); x < width; x++ {
			i, j := y*stride+x*4, (y*width+x)*4
			copy(img.Pix[j:j+4], pixels[i:i+4])
			if r.format == gputypes.TextureFormatBGRA8Unorm || r.format == gputypes.TextureFormatBGRA8UnormSrgb {
				img.Pix[j], img.Pix[j+2] = img.Pix[j+2], img.Pix[j]
			}
		}
	}
	check(buffer.Unmap())
	return img
}

func saveNativePreview(t *testing.T, img *image.RGBA, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func exportNativeMenuPreview(t *testing.T, r *webGPUWorldRenderer, p *gpuMenuPainter, path string) {
	t.Helper()
	saveNativePreview(t, captureNativePreview(t, r, p.width, p.height, func(pass *wgpu.RenderPassEncoder) error { return p.draw(pass, r) }), path)
}
