//go:build windows

package main

import (
	"fmt"
	"image/color"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

type gpuMenuBatch struct {
	clip   uiRect
	shapes *webGPUHUDBuilder
	text   *webGPUTextBatch
}
type gpuMenuPainter struct {
	width, height uint32
	clip          uiRect
	batches       []gpuMenuBatch
}

func (p *gpuMenuPainter) reset(width, height uint32) {
	p.width, p.height = width, height
	p.clip = newUIRect(0, 0, float32(width), float32(height))
	clear(p.batches)
	p.batches = p.batches[:0]
}
func (p *gpuMenuPainter) Size() (int, int) { return int(p.width), int(p.height) }
func gpuMenuColor(c color.RGBA) [4]float32 {
	return [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
}
func (p *gpuMenuPainter) batch(text bool) *gpuMenuBatch {
	if n := len(p.batches); n > 0 {
		b := &p.batches[n-1]
		if b.clip == p.clip && (b.text != nil) == text {
			return b
		}
	}
	b := gpuMenuBatch{clip: p.clip}
	if text {
		b.text = newWebGPUTextBatch(p.width, p.height)
	} else {
		b.shapes = newWebGPUHUDBuilder(p.width, p.height)
	}
	p.batches = append(p.batches, b)
	return &p.batches[len(p.batches)-1]
}
func (p *gpuMenuPainter) Rect(r uiRect, c color.RGBA) {
	if r.Width > 0 && r.Height > 0 {
		p.batch(false).shapes.rect(r.X, r.Y, r.Width, r.Height, gpuMenuColor(c))
	}
}
func (p *gpuMenuPainter) Text(s string, x, y, size float32, c color.RGBA) {
	p.batch(true).text.text(x, y, size, s, gpuMenuColor(c))
}
func (p *gpuMenuPainter) Measure(s string, size float32) float32 { return webGPUTextWidth(size, s) }
func (p *gpuMenuPainter) Clip(r uiRect) {
	x, y := max(float32(0), r.X), max(float32(0), r.Y)
	p.clip = newUIRect(x, y, max(float32(0), min(float32(p.width), r.X+r.Width)-x), max(float32(0), min(float32(p.height), r.Y+r.Height)-y))
}
func (p *gpuMenuPainter) Unclip() { p.clip = newUIRect(0, 0, float32(p.width), float32(p.height)) }
func (p *gpuMenuPainter) draw(pass *wgpu.RenderPassEncoder, r *webGPUWorldRenderer) error {
	hud, err := ensureWebGPUHUDRenderer(r)
	if err != nil {
		return err
	}
	text, err := ensureWebGPUTextRenderer(r)
	if err != nil {
		return err
	}
	for _, b := range p.batches {
		if b.clip.Width < 1 || b.clip.Height < 1 {
			continue
		}
		pass.SetScissorRect(gputypes.ScissorRect{X: uint32(b.clip.X), Y: uint32(b.clip.Y), Width: uint32(b.clip.Width), Height: uint32(b.clip.Height)})
		if b.shapes != nil {
			err = hud.drawPrepared(pass, b.shapes)
		} else {
			err = text.drawPrepared(pass, b.text)
		}
		if err != nil {
			return err
		}
	}
	pass.SetScissorRect(gputypes.ScissorRect{Width: r.width, Height: r.height})
	return nil
}
func (r *webGPUWorldRenderer) DrawMenu(p *gpuMenuPainter) error {
	if p.width != r.width || p.height != r.height {
		r.width, r.height = p.width, p.height
		if err := r.configureSurface(); err != nil {
			return err
		}
	}
	if activeWebGPUHUDRenderer != nil {
		activeWebGPUHUDRenderer.uploads.begin()
	}
	if activeWebGPUTextRenderer != nil {
		activeWebGPUTextRenderer.uploads.begin()
	}
	texture, _, err := r.surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	if texture == nil {
		return fmt.Errorf("menu surface returned no texture")
	}
	presented := false
	defer func() {
		if !presented {
			r.surface.DiscardTexture()
		}
	}()
	view, err := texture.CreateView(nil)
	if err != nil {
		return err
	}
	defer view.Release()
	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft menu"})
	if err != nil {
		return err
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments:       []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore, ClearValue: gputypes.Color{A: 1}}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{View: r.depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore, DepthClearValue: 1},
	})
	if err != nil {
		return err
	}
	if err = p.draw(pass, r); err != nil {
		_ = pass.End()
		return err
	}
	if err = pass.End(); err != nil {
		return err
	}
	commands, err := encoder.Finish()
	if err != nil {
		return err
	}
	defer commands.Release()
	if _, err = r.queue.Submit(commands); err != nil {
		return err
	}
	if err = r.surface.Present(texture); err != nil {
		return err
	}
	presented = true
	return nil
}
