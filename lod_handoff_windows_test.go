//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

// Renders real generated chunk meshes and seed-derived LOD together. This is
// opt-in because it needs a native WebGPU device and builds several chunks.
func TestWebGPUNearDistantHandoffPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_LOD_HANDOFF_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_LOD_HANDOFF_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.HorizonDistance = 64
	const seed = uint32(1234511)
	w := NewClientWorld()
	w.seed, w.TimeTicks = seed, 6000
	w.StartMeshWorkers(assets, 8)
	defer w.Close()
	for cz := 3; cz <= 8; cz++ {
		for cx := -10; cx <= 3; cx++ {
			chunk := w.ensureChunk(cx, cz)
			generateChunkData(seed, cx, cz, chunk)
			chunk.skyLight.Fill(15)
			chunk.generated = true
			for x := 0; x < chunkWidth; x++ {
				for z := 0; z < chunkWidth; z++ {
					for y := 0; y < int(chunk.heightMap[x][z]); y++ {
						if chunk.blocks.Get(x, y, z) != blockAir {
							chunk.sectionBlocks[y/sectionHeight]++
						}
					}
				}
			}
			ensureChunkSections(chunk)
		}
	}
	frame := webGPUFrameContext{
		Camera: webGPUCamera{Position: webGPUVec3{0, 180, 0}, Target: webGPUVec3{0, 60, 230}, Up: webGPUVec3{0, 1, 0}, Fovy: 70},
		Width:  1280, Height: 720,
	}
	state := &InputState{}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		w.ProcessMeshResults(assets, 32)
		if err := r.DrawGameplay(w, frame, state); err != nil {
			t.Fatal(err)
		}
		if r.lod != nil && len(r.lod.tiles) >= 48 && w.render.readySections > 0 && w.render.readySections == w.render.visibleSections {
			break
		}
	}
	if r.lod == nil || w.render.readySections == 0 {
		t.Fatal("full chunks or distant tiles did not become ready")
	}
	if len(r.lod.waterTiles) == 0 {
		t.Fatal("no distant water became ready for the handoff preview")
	}
	if err := r.updateSceneCameraAtTime(frame.Camera, w.TimeTicks); err != nil {
		t.Fatal(err)
	}
	r.collectVisibleCamera(w, frame.Camera)
	if err := r.updateLODChunkMask(w, frame.Camera); err != nil {
		t.Fatal(err)
	}
	cache := &w.render
	if len(cache.translucent) == 0 {
		t.Fatal("no full-detail water is visible at the handoff")
	}
	img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
		if err := r.lod.draw(pass, r, frame.Camera, w); err != nil {
			return err
		}
		pass.SetPipeline(r.solidPipeline)
		pass.SetBindGroup(0, r.bindGroup, nil)
		for _, section := range cache.visible {
			if err := r.drawMeshMap(pass, section.chunk.opaqueMeshes[section.sec], cache); err != nil {
				return err
			}
		}
		pass.SetPipeline(r.opaquePipeline)
		for _, section := range cache.visible {
			if err := r.drawMeshMap(pass, section.chunk.cutoutMeshes[section.sec], cache); err != nil {
				return err
			}
		}
		if err := r.lod.drawWater(pass, r, frame.Camera, cache); err != nil {
			return err
		}
		pass.SetPipeline(r.waterPipeline)
		for _, item := range cache.translucent {
			if item.glass {
				continue
			}
			if err := r.drawMeshMap(pass, item.section.chunk.waterMeshes[item.section.sec], cache); err != nil {
				return err
			}
		}
		return nil
	})
	path := filepath.Join("work", "lod-handoff.png")
	saveNativePreview(t, img, path)
	t.Logf("full-chunk/LOD handoff: %s", path)
	fullOnly := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
		pass.SetPipeline(r.solidPipeline)
		pass.SetBindGroup(0, r.bindGroup, nil)
		for _, section := range cache.visible {
			if err := r.drawMeshMap(pass, section.chunk.opaqueMeshes[section.sec], cache); err != nil {
				return err
			}
		}
		pass.SetPipeline(r.opaquePipeline)
		for _, section := range cache.visible {
			if err := r.drawMeshMap(pass, section.chunk.cutoutMeshes[section.sec], cache); err != nil {
				return err
			}
		}
		return nil
	})
	saveNativePreview(t, fullOnly, filepath.Join("work", "lod-handoff-full-only.png"))
	lodOnly := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
		return r.lod.draw(pass, r, frame.Camera, w)
	})
	saveNativePreview(t, lodOnly, filepath.Join("work", "lod-handoff-lod-only.png"))
}
