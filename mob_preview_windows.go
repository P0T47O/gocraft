//go:build windows

package main

import (
	"fmt"
	"runtime"
	"time"
)

// The workshop uses the same atlas, entity pipelines, input and surface recovery
// as gameplay. It has no network connection and never opens a world save.
func runMobPreview() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w, err := createNativeWindow(1100, 760, true)
	if err != nil {
		return err
	}
	defer w.Close()
	w.Poll()
	nativeGameWindow = w
	nativeWindowActive = true
	mobWorkshopActive, mobWorkshopBounds = true, true
	defer func() {
		nativeGameWindow = nil
		nativeWindowActive = false
		mobWorkshopActive = false
		windowCursor = nil
	}()
	assets = newWebGPUCPUAssets()
	if err = ensureExperimentalWebGPURenderer(); err != nil {
		return err
	}
	defer func() { closeExperimentalWebGPURenderer(); assets = nil }()
	scene := NewClientWorld()
	defer scene.Close()
	e := &RemoteEntity{Type: EntityPig, MobKind: "pig", MobHealth: 10}
	remoteEntities = map[string]*RemoteEntity{"workshop-pig": e}
	defer clear(remoteEntities)
	camera = gameCamera{Position: gameVec3{2.4, 1.8, 3}, Target: gameVec3{0, .5, 0}, Up: gameVec3{0, 1, 0}, Fovy: 45}
	state := NewInputState()
	mode := "idle"
	status := "1 Idle  2 Walk  3 Flee  4 Hurt  5 Death  R Reload  B Bounds  A/D Rotate"
	last := time.Now()
	for !w.closed {
		started := time.Now()
		w.Poll()
		if w.closed || inputKeyPressed(keyEscape) {
			break
		}
		if w.minimized || w.width <= 0 || w.height <= 0 {
			time.Sleep(20 * time.Millisecond)
			last = started
			continue
		}
		dt := min(float32(started.Sub(last).Seconds()), float32(.1))
		last = started
		for i, name := range []string{"idle", "walk", "flee", "hurt", "dead"} {
			if inputKeyPressed(keyOne + int32(i)) {
				mode = name
				if mode == "dead" {
					e.DeathTime = 0
				}
			}
		}
		if inputKeyDown(keyA) {
			e.Yaw -= dt
		}
		if inputKeyDown(keyD) {
			e.Yaw += dt
		}
		if inputKeyPressed(keyB) {
			mobWorkshopBounds = !mobWorkshopBounds
		}
		if inputKeyPressed(keyR) {
			next, loadErr := reloadMobPreview()
			if loadErr != nil {
				status = loadErr.Error()
			} else {
				closeExperimentalWebGPURenderer()
				mobContent = next
				if err = ensureExperimentalWebGPURenderer(); err != nil {
					return err
				}
				status = "Reloaded definitions, model and animations"
			}
		}
		advanceMobPreview(e, mobContent, mode, dt)
		nativeMenuCanvas.reset(uint32(w.width), uint32(w.height))
		nativeMenuCanvas.Text("MOB WORKSHOP", 28, 24, 28, invAccent)
		nativeMenuCanvas.Text(fmt.Sprintf("pig / %s", mode), 28, 65, 20, invText)
		nativeMenuCanvas.Text(status, 28, float32(w.height)-40, 14, invMuted)
		frame := webGPUFrameFromWindow()
		frame.HideHUD = true
		if err = activeWebGPUWorldRenderer.DrawGameplay(scene, frame, state); err != nil {
			return err
		}
		if rest := time.Second/60 - time.Since(started); rest > 0 {
			time.Sleep(rest)
		}
	}
	return nil
}
