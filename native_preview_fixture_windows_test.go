//go:build windows

package main

import (
	"github.com/gogpu/wgpu"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Preview resources and state are restored on the owning OS thread. No personal
// settings or saves are loaded, and input edges stay empty during capture.
func nativePreviewFixture(t *testing.T) (*webGPUWorldRenderer, func()) {
	t.Helper()
	runtime.LockOSThread()
	oldFrame, oldCursor := windowFrame, windowCursor
	oldAssets, oldSettings, oldUI := assets, currentSettings, ui
	oldCanvas, oldNative := menuCanvas, nativeWindowActive
	oldInventory, oldMode, oldPaused := localInventory, currentGameMode, isPaused
	oldEntities := remoteEntities
	initBlockRegistry()
	InitRecipes()
	w, err := createNativeWindow(1280, 720, false)
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	currentSettings = &GameSettings{PlayerName: "Preview", ResolutionWidth: 1280, ResolutionHeight: 720, Mipmaps: true, Anisotropy: 8, RenderDistance: defaultRenderDistance}
	assets = newWebGPUCPUAssets()
	restore := func() {
		w.Close()
		windowFrame, windowCursor = oldFrame, oldCursor
		assets, currentSettings, ui = oldAssets, oldSettings, oldUI
		menuCanvas, nativeWindowActive = oldCanvas, oldNative
		localInventory, currentGameMode, isPaused = oldInventory, oldMode, oldPaused
		remoteEntities = oldEntities
		runtime.UnlockOSThread()
	}
	r, err := newWebGPUWorldRenderer(w.hwnd, assets, 1280, 720)
	if err != nil {
		restore()
		t.Fatal(err)
	}
	nativeWindowActive = true
	windowFrame = windowInputFrame{Width: 1280, Height: 720}
	ui = NewUIComponents()
	currentGameMode, isPaused = ModeSurvival, false
	remoteEntities = map[string]*RemoteEntity{}
	if err := os.MkdirAll("work", 0755); err != nil {
		r.Close()
		restore()
		t.Fatal(err)
	}
	return r, func() {
		closeWebGPUEntityRenderer()
		closeWebGPUHUDRenderer()
		closeWebGPUTextRenderer()
		r.Close()
		restore()
	}
}

func captureNativeUI(t *testing.T, r *webGPUWorldRenderer, name string, width, height int, state *InputState, menu func()) {
	t.Helper()
	windowFrame = windowInputFrame{Width: width, Height: height}
	p := &gpuMenuPainter{}
	p.reset(uint32(width), uint32(height))
	menuCanvas = p
	if menu != nil {
		menu()
	}
	image := captureNativePreview(t, r, uint32(width), uint32(height), func(pass *wgpu.RenderPassEncoder) error {
		if state != nil {
			hud, err := ensureWebGPUHUDRenderer(r)
			if err != nil {
				return err
			}
			if err = hud.Draw(pass, uint32(width), uint32(height), state); err != nil {
				return err
			}
			text, err := ensureWebGPUTextRenderer(r)
			if err != nil {
				return err
			}
			if err = text.Draw(pass, uint32(width), uint32(height), state); err != nil {
				return err
			}
			if err = drawWebGPUInventoryOverlay(pass, r, state); err != nil {
				return err
			}
			if err = drawWebGPUContainerOverlay(pass, r, state); err != nil {
				return err
			}
		}
		return p.draw(pass, r)
	})
	varied := false
	for i := 4; i < len(image.Pix); i += 4 {
		if image.Pix[i] != image.Pix[0] || image.Pix[i+1] != image.Pix[1] || image.Pix[i+2] != image.Pix[2] {
			varied = true
			break
		}
	}
	if !varied {
		t.Fatal("empty UI capture", name)
	}
	saveNativePreview(t, image, filepath.Join("work", name+".png"))
}
