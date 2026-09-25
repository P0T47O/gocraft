//go:build windows

package main

import (
	"github.com/gogpu/wgpu"
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
	// Reconfigure even when dimensions have not changed (e.g. restore or a
	// recoverable Present error). Menu and gameplay share this recovery path.
	depthBefore := r.depthView
	if err = r.recoverOutdatedSurface(wgpu.ErrSurfaceOutdated); err != nil {
		t.Fatal(err)
	}
	if err = r.DrawMenu(p); err != nil {
		t.Fatal(err)
	}
	if r.depthView == depthBefore {
		t.Fatal("same-size stale surface was not rebuilt")
	}
	// Simulate an external resize after the frame snapshot (not a settings request).
	w.Resize(1024, 768)
	w.applyPendingResize()
	if err = r.DrawMenu(p); err != nil {
		t.Fatalf("menu resize race: %v", err)
	}
	w.Poll()
	p.reset(uint32(w.width), uint32(w.height))
	drawMenu()
	if err = r.DrawMenu(p); err != nil {
		t.Fatal(err)
	}
	if r.width != uint32(w.width) || r.height != uint32(w.height) {
		t.Fatal("surface dimensions did not catch up")
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
			if session == 0 {
				// Exercise the real TCP -> client state -> gameplay renderer path
				// for all hostile models, without touching a personal save.
				for i, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
					mob := newMob(kind, "native-hostile-"+kind, float64(camera.Position.X)+float64(i*2-3), float64(camera.Position.Y-playerEyeY), float64(camera.Position.Z)+6)
					server.SpawnEntity(mob)
				}
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					w.Poll()
					updateGame()
					seen := 0
					for _, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
						if e := remoteEntities["native-hostile-"+kind]; e != nil && e.MobKind == kind {
							seen++
						}
					}
					if seen == 4 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				for _, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
					if e := remoteEntities["native-hostile-"+kind]; e == nil || e.MobKind != kind {
						t.Fatalf("%s was not synchronized into the native game session", kind)
					}
				}
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
			// Settings are applied after input capture, before rendering this frame.
			// Previously this changed the HWND immediately, leaving the frame stale.
			for _, size := range [][2]int{{1024, 768}, {1280, 720}, {1600, 900}, {1280, 720}} {
				w.Poll()
				nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
				drawPauseMenu()
				settings.ResolutionWidth, settings.ResolutionHeight = size[0], size[1]
				ApplySettings() // No SaveSettings: preserve the user's preferences.
				if err = drawExperimentalWebGPUFrame(); err != nil {
					t.Fatalf("same-frame resolution change: %v", err)
				}
				w.Poll()
				nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
				drawPauseMenu()
				if err = drawExperimentalWebGPUFrame(); err != nil {
					t.Fatalf("resized frame: %v", err)
				}
				if client == nil || server == nil || world == nil {
					t.Fatal("resize terminated the session")
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
