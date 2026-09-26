//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

func TestLODTileRangeIncludesNearUnderlay(t *testing.T) {
	center := lodTileKey{0, 0}
	if !lodTileInRange(center, center, 4) || !lodTileInRange(lodTileKey{4, 0}, center, 4) {
		t.Fatal("LOD must cover the near field while full chunks are streaming")
	}
	if lodTileInRange(lodTileKey{7, 0}, center, 4) {
		t.Fatal("LOD tile outside horizon retained")
	}
}

func TestWebGPUDistantTerrainPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_LOD_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_LOD_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.HorizonDistance = 64
	world := &World{seed: 1234511, TimeTicks: 6000}
	frame := webGPUFrameContext{
		Camera: webGPUCamera{Position: webGPUVec3{0, 130, 0}, Target: webGPUVec3{600, 70, 600}, Up: webGPUVec3{0, 1, 0}, Fovy: 65},
		Width:  1280, Height: 720,
	}
	state := &InputState{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := r.DrawGameplay(world, frame, state); err != nil {
			t.Fatal(err)
		}
		if r.lod != nil && len(r.lod.tiles) >= 64 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if r.lod == nil || len(r.lod.tiles) == 0 || world.render.drawCalls == 0 {
		t.Fatalf("no distant terrain drawn: renderer=%v drawCalls=%d", r.lod != nil, world.render.drawCalls)
	}
	img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
		return r.lod.draw(pass, r, frame.Camera, &world.render)
	})
	path := filepath.Join("work", "lod-terrain.png")
	saveNativePreview(t, img, path)
	t.Logf("distant terrain preview: %s; ready tiles=%d", path, len(r.lod.tiles))
	world.seed = 42
	if err := r.DrawGameplay(world, frame, state); err != nil {
		t.Fatal(err)
	}
	if r.lod == nil || r.lod.seed != 42 {
		t.Fatal("old-seed LOD cache survived a world change")
	}
	currentSettings.HorizonDistance = 0
	if err := r.DrawGameplay(world, frame, state); err != nil {
		t.Fatal(err)
	}
	if r.lod != nil {
		t.Fatal("disabling the horizon retained GPU meshes or workers")
	}
}
