//go:build windows

package main

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// DrawGameplay is the live WebGPU gameplay presentation path. World geometry,
// entities/effects, HUD, text, inventory and container screens are encoded into
// one render pass so the Raylib/OpenGL presenter remains completely idle while
// StatePlaying owns the HWND through WebGPU.
func (r *webGPUWorldRenderer) DrawGameplay(world *World, frame webGPUFrameContext, state *InputState) error {
	if world == nil {
		return nil
	}
	width := max(uint32(1), frame.Width)
	height := max(uint32(1), frame.Height)
	if width != r.width || height != r.height {
		r.width, r.height = width, height
		if err := r.configureSurface(); err != nil {
			return err
		}
	}
	if err := updateWebGPUAtlasAnimations(r, frame.Time); err != nil {
		return fmt.Errorf("update WebGPU atlas animations: %w", err)
	}
	if err := r.updateSceneCamera(frame.Camera); err != nil {
		return fmt.Errorf("update WebGPU gameplay scene: %w", err)
	}
	r.collectVisibleCamera(world, frame.Camera)
	cache := &world.render
	defer func() {
		clear(cache.visible)
		cache.visible = cache.visible[:0]
		clear(cache.translucent)
		cache.translucent = cache.translucent[:0]
	}()

	surfaceTexture, _, err := r.surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	if surfaceTexture == nil {
		return fmt.Errorf("surface returned no texture")
	}
	view, err := surfaceTexture.CreateView(nil)
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	defer view.Release()
	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft WebGPU gameplay encoder"})
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft WebGPU gameplay pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 180.0 / 255.0, G: 210.0 / 255.0, B: 1.0, A: 1.0},
		}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View: r.depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore,
			DepthClearValue: 1, DepthReadOnly: false,
		},
	})
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}

	pass.SetPipeline(r.opaquePipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	for _, section := range cache.visible {
		if err := r.drawMeshMap(pass, section.chunk.opaqueMeshes[section.sec], cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
		if err := r.drawMeshMap(pass, section.chunk.cutoutMeshes[section.sec], cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
	}

	entities, err := ensureWebGPUEntityRenderer(r)
	if err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	if err := entities.Draw(pass, r, state, frame.Time); err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}

	pass.SetPipeline(r.translucentPipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	for _, item := range cache.translucent {
		section := item.section
		meshes := section.chunk.waterMeshes[section.sec]
		if item.glass {
			meshes = section.chunk.glassMeshes[section.sec]
		}
		if err := r.drawMeshMap(pass, meshes, cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
	}

	hud, err := ensureWebGPUHUDRenderer(r)
	if err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	if err := hud.Draw(pass, r.width, r.height, state); err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	text, err := ensureWebGPUTextRenderer(r)
	if err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	if err := text.Draw(pass, r.width, r.height, state); err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	if err := drawWebGPUInventoryOverlay(pass, r, state); err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}
	if err := drawWebGPUContainerOverlay(pass, r, state); err != nil {
		_ = pass.End()
		r.surface.DiscardTexture()
		return err
	}

	if err := pass.End(); err != nil {
		r.surface.DiscardTexture()
		return err
	}
	commandBuffer, err := encoder.Finish()
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	defer commandBuffer.Release()
	if _, err := r.queue.Submit(commandBuffer); err != nil {
		r.surface.DiscardTexture()
		return err
	}
	return r.surface.Present(surfaceTexture)
}
