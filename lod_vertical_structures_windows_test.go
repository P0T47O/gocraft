//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

// The input shapes are actual voxels in temporary chunks. Only their captured
// 8-block columns reach the current distant renderer, which is exactly the
// representation limitation this diagnostic is meant to expose.
func TestWebGPUDistantVerticalAndFloatingStructures(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_STRUCTURE_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_STRUCTURE_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.ExperimentalLOD = true
	currentSettings.HorizonDistance = 64
	w, _ := lodVerticalStructureFixture(1234511)
	w.TimeTicks = 6000
	state := &InputState{}
	for _, view := range []struct {
		name string
		z    float32
		step int
	}{
		{"near", 920, lodNearCellSize},
		{"middle", 624, lodCellSize},
		{"far", 224, lodFarCellSize},
	} {
		frame := webGPUFrameContext{
			Camera: webGPUCamera{
				Position: webGPUVec3{0, 160, view.z},
				Target:   webGPUVec3{0, 150, 1024},
				Up:       webGPUVec3{0, 1, 0},
				Fovy:     80,
			},
			Width: 1280, Height: 720,
		}
		deadline := time.Now().Add(12 * time.Second)
		ready := 0
		for time.Now().Before(deadline) {
			if err := r.DrawGameplay(w, frame, state); err != nil {
				t.Fatal(err)
			}
			ready = 0
			for x := -2; x <= 1; x++ {
				key := lodTileKey{x, 8}
				if r.lod != nil && r.lod.tiles[key] != nil && r.lod.steps[key] == view.step {
					ready++
				}
			}
			if ready == 4 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if ready != 4 {
			t.Fatalf("%s: only %d of 4 structure tiles reached %d-block detail", view.name, ready, view.step)
		}
		img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
			return r.lod.drawPreview(pass, r, frame.Camera, w)
		})
		path := filepath.Join("work", "lod-vertical-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("%s: %d-block cells, %s", view.name, view.step, path)
	}
}
