//go:build windows

package main

import (
	"image/color"
	"os"
	"testing"
)

// Exercises the real surface path: multisampled terrain resolve, single-sample
// HUD composition, then a single-sample menu frame on the same renderer.
func TestWebGPUMSAAFrame(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_MSAA_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_MSAA_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	if r.worldSamples != 4 || r.msaaView == nil {
		t.Fatal("multisampled world attachments not initialized")
	}
	frame := webGPUFrameContext{
		Camera: webGPUCamera{Position: webGPUVec3{8, 6, 20}, Target: webGPUVec3{8, 1, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55},
		Width:  1280, Height: 720,
	}
	if err := r.DrawGameplay(&World{TimeTicks: 6000}, frame, &InputState{}); err != nil {
		t.Fatalf("multisampled gameplay: %v", err)
	}
	menu := &gpuMenuPainter{}
	menu.reset(1280, 720)
	menu.Rect(newUIRect(100, 100, 200, 80), color.RGBA{R: 80, G: 120, B: 160, A: 255})
	if err := r.DrawMenu(menu); err != nil {
		t.Fatalf("single-sample menu after gameplay: %v", err)
	}
	if err := r.prepareSurface(1024, 768); err != nil {
		t.Fatalf("resize multisampled attachments: %v", err)
	}
	if r.msaaView == nil || r.depthView == nil {
		t.Fatal("resized multisampled attachments missing")
	}
}

func TestWebGPUAASettingsPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_MSAA_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_MSAA_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t)
	defer cleanup()
	previousPage := menuPage
	defer func() { menuPage = previousPage }()
	menuPage = MenuSettings
	captureNativeUI(t, r, "aa-settings", 1280, 720, nil, drawMenu)
}
