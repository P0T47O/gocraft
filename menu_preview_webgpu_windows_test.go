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
func exportNativeMenuPreview(t *testing.T, r *webGPUWorldRenderer, p *gpuMenuPainter, path string) {
	t.Helper()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{Size: wgpu.Extent3D{Width: p.width, Height: p.height, DepthOrArrayLayers: 1}, MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D, Format: r.format, Usage: gputypes.TextureUsageRenderAttachment | gputypes.TextureUsageCopySrc})
	check(err)
	defer texture.Release()
	view, err := r.device.CreateTextureView(texture, nil)
	check(err)
	defer view.Release()
	stride := (p.width*4 + 255) &^ uint32(255)
	size := uint64(stride) * uint64(p.height)
	buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{Size: size, Usage: gputypes.BufferUsageMapRead | gputypes.BufferUsageCopyDst})
	check(err)
	defer buffer.Release()
	encoder, err := r.device.CreateCommandEncoder(nil)
	check(err)
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore}}, DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: r.depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 1}})
	check(err)
	check(p.draw(pass, r))
	check(pass.End())
	commands, err := encoder.Finish()
	check(err)
	_, err = r.queue.Submit(commands)
	commands.Release()
	check(err)
	encoder, err = r.device.CreateCommandEncoder(nil)
	check(err)
	encoder.CopyTextureToBuffer(texture, buffer, []wgpu.BufferTextureCopy{{BufferLayout: wgpu.ImageDataLayout{BytesPerRow: stride, RowsPerImage: p.height}, TextureBase: wgpu.ImageCopyTexture{Texture: texture}, Size: wgpu.Extent3D{Width: p.width, Height: p.height, DepthOrArrayLayers: 1}}})
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
	img := image.NewRGBA(image.Rect(0, 0, int(p.width), int(p.height)))
	for y := uint32(0); y < p.height; y++ {
		for x := uint32(0); x < p.width; x++ {
			i, j := y*stride+x*4, (y*p.width+x)*4
			copy(img.Pix[j:j+4], pixels[i:i+4])
			if r.format == gputypes.TextureFormatBGRA8Unorm || r.format == gputypes.TextureFormatBGRA8UnormSrgb {
				img.Pix[j], img.Pix[j+2] = img.Pix[j+2], img.Pix[j]
			}
		}
	}
	check(buffer.Unmap())
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	check(png.Encode(f, img))
}
