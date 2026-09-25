//go:build windows

package main

import (
	"os"
	"testing"
)

func TestNativeMobWorkshop(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("opt-in GPU workshop")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	oldActive, oldBounds, oldCanvas := mobWorkshopActive, mobWorkshopBounds, nativeMenuCanvas
	defer func() { mobWorkshopActive, mobWorkshopBounds, nativeMenuCanvas = oldActive, oldBounds, oldCanvas }()
	mobWorkshopActive, mobWorkshopBounds = true, true
	scene := NewClientWorld()
	defer scene.Close()
	e := &RemoteEntity{Type: EntityPig, MobKind: "pig", MobHealth: 10}
	remoteEntities = map[string]*RemoteEntity{"workshop-test": e}
	frame := webGPUFrameContext{Width: 1280, Height: 720, HideHUD: true, Camera: webGPUCamera{Position: webGPUVec3{X: 2.4, Y: 1.8, Z: 3}, Target: webGPUVec3{Y: .5}, Up: webGPUVec3{Y: 1}, Fovy: 45}}
	state := NewInputState()
	for _, kind := range []string{"pig", "zombie", "skeleton", "spider", "creeper"} {
		e.MobKind = kind
		for _, mode := range []string{"idle", "walk", "flee", "hurt", "dead"} {
			advanceMobPreview(e, mobContent, mode, .1)
			nativeMenuCanvas.reset(frame.Width, frame.Height)
			nativeMenuCanvas.Text(kind+" "+mode, 28, 24, 28, invText)
			if err := r.DrawGameplay(scene, frame, state); err != nil {
				t.Fatal(kind, mode, err)
			}
		}
	}
	batch := newWebGPUEntityBatch()
	batch.addWorkshopGuides()
	withBounds := len(batch.vertices)
	mobWorkshopBounds = false
	batch.vertices = nil
	batch.indices = nil
	batch.addWorkshopGuides()
	if len(batch.vertices) == 0 || len(batch.vertices) >= withBounds {
		t.Fatal("workshop grid/bounds toggle failed")
	}
}
