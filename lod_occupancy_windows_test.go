//go:build windows

package main

import (
	"os"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

func TestWebGPULODOccupancyPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_OCCUPANCY_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_OCCUPANCY_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.ExperimentalLOD = true
	currentSettings.HorizonDistance = 64
	w, chunks := lodVerticalStructureFixture(1234511)
	w.lodColumns = nil // Capture actual voxel occupancy, without legacy prisms.
	for key, chunk := range chunks {
		for x := 0; x < chunkWidth; x++ {
			for z := 0; z < chunkWidth; z++ {
				w.recordLODOccupancy(chunk, key.X*chunkWidth+x, key.Z*chunkWidth+z)
			}
		}
	}
	w.TimeTicks = 6000
	frame := webGPUFrameContext{Camera: webGPUCamera{Position: webGPUVec3{0, 205, 624}, Target: webGPUVec3{0, 145, 1024}, Up: webGPUVec3{0, 1, 0}, Fovy: 70}, Width: 1280, Height: 720}
	deadline := time.Now().Add(12 * time.Second)
	ready := 0
	for time.Now().Before(deadline) {
		if err := r.DrawGameplay(w, frame, &InputState{}); err != nil {
			t.Fatal(err)
		}
		ready = 0
		if r.lod != nil {
			for z := 7; z <= 8; z++ {
				for x := -2; x <= 1; x++ {
					if r.lod.tiles[lodTileKey{x, z}] != nil {
						ready++
					}
				}
			}
		}
		if ready == 8 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ready != 8 {
		t.Fatalf("only %d of 8 structure tiles ready", ready)
	}
	img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error { return r.lod.drawPreview(pass, r, frame.Camera, w) })
	saveNativePreview(t, img, "work/lod-occupancy.png")
}
