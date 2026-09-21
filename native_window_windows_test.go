//go:build windows

package main

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestNativeKeyMapping(t *testing.T) {
	for i := uintptr(0); i < 9; i++ {
		if nativeKey('1'+i, 0) != keyOne+int32(i) {
			t.Fatal("hotbar key mapping")
		}
	}
	if nativeKey(0x10, 0x36<<16) != keyRightShift || nativeKey(0x11, 1<<24) != keyRightControl || nativeKey(0x11, 0) != keyLeftControl || nativeKey(0xffff, 0) != -1 {
		t.Fatal("modifier mapping")
	}
}

// This integration test owns an invisible native HWND, never a Raylib window.
func TestNativeWebGPUWindow(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_REGRESSION") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_REGRESSION=1 for native window/GPU validation")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	preserveWindowInput(t)
	w, err := createNativeWindow(1280, 720, false)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.Poll()
	if windowWidth() != 1280 || windowHeight() != 720 {
		t.Fatalf("client dimensions %dx%d", windowWidth(), windowHeight())
	}
	// Exercise transitions through the same message handler as the native pump.
	nativeWindowProc(w.hwnd, 0x100, 'W', 0)
	nativeWindowProc(w.hwnd, 0x102, 0xd83d, 0)
	nativeWindowProc(w.hwnd, 0x102, 0xde42, 0)
	if !w.frame.Down[keyW] || !w.frame.Pressed[keyW] || len(w.frame.Text) != 1 || w.frame.Text[0] != '🙂' {
		t.Fatal("native key/text input")
	}
	nativeWindowProc(w.hwnd, 0x8, 0, 0)
	if w.frame.Down[keyW] {
		t.Fatal("focus loss left a movement key held")
	}
	w.Resize(1024, 768)
	w.Poll()
	if w.width != 1024 || w.height != 768 {
		t.Fatalf("resize %dx%d", w.width, w.height)
	}
	initBlockRegistry()
	InitRecipes()
	a := newWebGPUCPUAssets()
	r, err := newWebGPUWorldRenderer(w.hwnd, a, uint32(w.width), uint32(w.height))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeWebGPUEntityRenderer()
		closeWebGPUHUDRenderer()
		closeWebGPUTextRenderer()
		r.Close()
		resetWebGPUAtlasAnimations()
	}()
	oldUI, oldCanvas, oldPage, oldSettings := ui, menuCanvas, menuPage, currentSettings
	oldNative := nativeWindowActive
	defer func() {
		ui, menuCanvas, menuPage, currentSettings = oldUI, oldCanvas, oldPage, oldSettings
		nativeWindowActive = oldNative
	}()
	settings := *LoadSettings()
	currentSettings = &settings // Never save local preferences.
	nativeWindowActive = true
	ui = NewUIComponents()
	p := &gpuMenuPainter{}
	menuCanvas = p
	windowFrame = windowInputFrame{Width: w.width, Height: w.height}
	for _, page := range []MenuPage{MenuMain, MenuSingleplayer, MenuMultiplayer, MenuCreateWorld, MenuSettings, MenuMain} {
		menuPage = page
		p.reset(uint32(w.width), uint32(w.height))
		drawMenu()
		if len(p.batches) == 0 {
			t.Fatal("empty native menu")
		}
		if err = r.DrawMenu(p); err != nil {
			t.Fatalf("menu %d: %v", page, err)
		}
	}
	w.Resize(1280, 720)
	w.Poll()
	p.reset(uint32(w.width), uint32(w.height))
	drawMenu()
	if err = r.DrawMenu(p); err != nil {
		t.Fatalf("resized menu: %v", err)
	}
	if path := os.Getenv("GOCRAFT_NATIVE_MENU_PREVIEW"); path != "" {
		exportNativeMenuPreview(t, r, p, path)
	}
	// Exercise real local networking/worker shutdown using only a temporary save.
	oldAssets, oldWorld, oldClient, oldServer, oldInput := assets, world, client, server, input
	oldRenderer, oldWindow, oldGPU := activeWebGPUWorldRenderer, nativeGameWindow, *useWebGPU
	oldState, oldPaused, oldPauseSettings, oldCamera := currentState, isPaused, pauseSettings, camera
	oldMenu := nativeMenuCanvas
	defer func() {
		assets, world, client, server, input = oldAssets, oldWorld, oldClient, oldServer, oldInput
		activeWebGPUWorldRenderer, nativeGameWindow, *useWebGPU = oldRenderer, oldWindow, oldGPU
		currentState, isPaused, pauseSettings, camera = oldState, oldPaused, oldPauseSettings, oldCamera
		nativeMenuCanvas = oldMenu
	}()
	assets, activeWebGPUWorldRenderer, nativeGameWindow, *useWebGPU = a, r, w, true
	settings.RenderDistance = minRenderDistance
	menuCanvas = &nativeMenuCanvas
	save := t.TempDir()
	if err = SaveLevelData(save, 42); err != nil {
		t.Fatal(err)
	}
	for session := 0; session < 2; session++ {
		startGame(save, "", false)
		if currentState != StatePlaying {
			t.Fatalf("session did not start: %s", menuError)
		}
		func() {
			defer exitGame()
			deadline := time.Now().Add(8 * time.Second)
			for !input.VitalsReady && time.Now().Before(deadline) {
				w.Poll()
				updateGame()
				time.Sleep(10 * time.Millisecond)
			}
			if !input.VitalsReady {
				t.Fatal("local server did not deliver player state")
			}
			for frame := 0; frame < 3; frame++ {
				w.Poll()
				updateGame()
				if err = drawExperimentalWebGPUFrame(); err != nil {
					t.Fatal(err)
				}
			}
			isPaused = true
			server.Paused.Store(true)
			for _, settingsOpen := range []bool{false, true} {
				pauseSettings = settingsOpen
				nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
				drawPauseMenu()
				if err = drawExperimentalWebGPUFrame(); err != nil {
					t.Fatal(err)
				}
			}
		}()
		if world != nil || client != nil || server != nil || assets != a || activeWebGPUWorldRenderer != r {
			t.Fatal("session shutdown released window assets or leaked gameplay owners")
		}
		nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
		drawMenu()
		if err = r.DrawMenu(&nativeMenuCanvas); err != nil {
			t.Fatal(err)
		}
	}
	nativeWindowProc(w.hwnd, 0x10, 0, 0)
	if !w.closed {
		t.Fatal("close request not recorded")
	}
}
