//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

// Opt-in native WebGPU diagnostic. The structures are supplied as captured
// authoritative columns, exactly what the distant renderer can consume after
// client chunks have been seen. No personal save or settings are changed.
func TestWebGPUDistantStructuresAtThreeDistances(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_STRUCTURE_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_STRUCTURE_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.HorizonDistance = 64
	w := lodStructureFixture(1234511)
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
				Position: webGPUVec3{0, 230, view.z},
				Target:   webGPUVec3{0, 125, 1024},
				Up:       webGPUVec3{0, 1, 0},
				Fovy:     95,
			},
			Width: 1280, Height: 720,
		}
		// Eight tiles cover all three structures, including their boundaries.
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if err := r.DrawGameplay(w, frame, state); err != nil {
				t.Fatal(err)
			}
			ready := 0
			if r.lod != nil {
				for _, z := range []int{7, 8} {
					for x := -2; x <= 1; x++ {
						key := lodTileKey{x, z}
						if r.lod.tiles[key] != nil && r.lod.steps[key] == view.step {
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
		for _, z := range []int{7, 8} {
			for x := -2; x <= 1; x++ {
				key := lodTileKey{x, z}
				if r.lod == nil || r.lod.tiles[key] == nil || r.lod.steps[key] != view.step {
					t.Fatalf("%s: structure tile %v did not reach %d-block detail", view.name, key, view.step)
				}
			}
		}
		img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
			return r.lod.drawPreview(pass, r, frame.Camera, w)
		})
		path := filepath.Join("work", "lod-structures-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("%s: %d-block cells, %s", view.name, view.step, path)
	}
	// A high angle checks the ring's ground opening; its front wall hides the
	// hole in the three distance views even when the geometry is correct.
	overhead := webGPUFrameContext{
		Camera: webGPUCamera{
			Position: webGPUVec3{128, 330, 960},
			Target:   webGPUVec3{128, 90, 1024},
			Up:       webGPUVec3{0, 1, 0},
			Fovy:     70,
		},
		Width: 1280, Height: 720,
	}
	deadline := time.Now().Add(8 * time.Second)
	ready := 0
	for time.Now().Before(deadline) {
		if err := r.DrawGameplay(w, overhead, state); err != nil {
			t.Fatal(err)
		}
		ready = 0
		for _, z := range []int{7, 8} {
			for _, x := range []int{0, 1} {
				key := lodTileKey{x, z}
				if r.lod.tiles[key] != nil && r.lod.steps[key] == lodNearCellSize {
					ready++
				}
			}
		}
		if ready == 4 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ready != 4 {
		t.Fatalf("ring overhead: only %d of 4 tiles ready", ready)
	}
	img := captureNativePreview(t, r, overhead.Width, overhead.Height, func(pass *wgpu.RenderPassEncoder) error {
		return r.lod.drawPreview(pass, r, overhead.Camera, w)
	})
	path := filepath.Join("work", "lod-structures-ring-overhead.png")
	saveNativePreview(t, img, path)
	t.Logf("ring overhead: %s", path)
}
