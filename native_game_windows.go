//go:build windows

package main

import (
	"runtime"
	"strings"
	"time"
)

var nativeGameWindow *nativeWindow
var nativeMenuCanvas gpuMenuPainter

func runNativeWebGPU(settings *GameSettings) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w, err := createNativeWindow(settings.ResolutionWidth, settings.ResolutionHeight, true)
	if err != nil {
		return err
	}
	nativeGameWindow = w
	nativeWindowActive = true
	defer func() { w.Close(); nativeGameWindow = nil; nativeWindowActive = false }()
	w.Poll()
	assets = newWebGPUCPUAssets()
	if err = ensureExperimentalWebGPURenderer(); err != nil {
		return err
	}
	defer func() {
		if world != nil {
			exitGame()
		}
		closeExperimentalWebGPURenderer()
		assets = nil
	}()
	InitRecipes()
	ui = NewUIComponents()
	perfMon = NewPerformanceMonitor()
	defer perfMon.Close()
	oldCanvas := menuCanvas
	menuCanvas = &nativeMenuCanvas
	defer func() { menuCanvas = oldCanvas; windowCursor = nil }()
	if settings.PlayerName != "" && (*username == "" || strings.HasPrefix(*username, "Player")) {
		*username = settings.PlayerName
	}
	last := time.Now()
	for !quitRequested && !w.closed {
		started := time.Now()
		w.Poll()
		if w.closed {
			break
		}
		if w.minimized || w.width <= 0 || w.height <= 0 {
			time.Sleep(20 * time.Millisecond)
			last = started
			continue
		}
		dt := float32(started.Sub(last).Seconds())
		last = started
		setGameFrameTime(dt)
		nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
		if currentState == StateMenu {
			if inputKeyPressed(keyEscape) && menuPage != MenuMain {
				if menuPage == MenuSettings {
					SaveSettings()
				}
				if menuPage == MenuCreateWorld {
					menuNavigate(MenuSingleplayer)
				} else {
					menuNavigate(MenuMain)
				}
			}
			drawMenu()
			// A click may have started a session; don't mix its first frame with menu commands.
			if currentState == StateMenu {
				if err = activeWebGPUWorldRenderer.DrawMenu(&nativeMenuCanvas); err != nil {
					return err
				}
			}
		} else {
			updateGame()
			if currentState == StatePlaying && input.isDead() {
				drawDeathScreen(input)
			}
			if currentState == StatePlaying && isPaused {
				drawPauseMenu()
			}
			if currentState == StatePlaying {
				if err = drawExperimentalWebGPUFrame(); err != nil {
					return err
				}
			}
		}
		perfMon.UpdateFrame(dt)
		if rest := time.Second/60 - time.Since(started); rest > 0 {
			time.Sleep(rest)
		}
	}
	return nil
}
