//go:build windows

package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type webGPUPoint struct {
	X, Y float32
}

type webGPUVec3 struct {
	X, Y, Z float32
}

type webGPUCamera struct {
	Position webGPUVec3
	Target   webGPUVec3
	Up       webGPUVec3
	Fovy     float32
}

type webGPUFrameContext struct {
	Camera        webGPUCamera
	Width, Height uint32
	Time          float32
}

var activeWebGPUWorldRenderer *webGPUWorldRenderer

func webGPUMousePosition() webGPUPoint {
	p := rl.GetMousePosition()
	return webGPUPoint{X: p.X, Y: p.Y}
}

func webGPUFrameFromRaylib() webGPUFrameContext {
	return webGPUFrameContext{
		Camera: webGPUCamera{
			Position: webGPUVec3{X: camera.Position.X, Y: camera.Position.Y, Z: camera.Position.Z},
			Target:   webGPUVec3{X: camera.Target.X, Y: camera.Target.Y, Z: camera.Target.Z},
			Up:       webGPUVec3{X: camera.Up.X, Y: camera.Up.Y, Z: camera.Up.Z},
			Fovy:     camera.Fovy,
		},
		Width:  uint32(max(1, rl.GetScreenWidth())),
		Height: uint32(max(1, rl.GetScreenHeight())),
		Time:   float32(rl.GetTime()),
	}
}

func ensureExperimentalWebGPURenderer() error {
	if activeWebGPUWorldRenderer != nil {
		return nil
	}
	if assets == nil {
		return fmt.Errorf("render assets are not initialized")
	}
	frame := webGPUFrameFromRaylib()
	hwnd := uintptr(rl.GetWindowHandle())
	renderer, err := newWebGPUWorldRenderer(hwnd, assets, frame.Width, frame.Height)
	if err != nil {
		return err
	}
	activeWebGPUWorldRenderer = renderer
	return nil
}

func drawExperimentalWebGPUFrame() error {
	if activeWebGPUWorldRenderer == nil {
		return fmt.Errorf("WebGPU world renderer is not initialized")
	}
	return activeWebGPUWorldRenderer.DrawGameplay(world, webGPUFrameFromRaylib(), input)
}

func closeExperimentalWebGPURenderer() {
	if activeWebGPUWorldRenderer == nil {
		return
	}
	closeWebGPUEntityRenderer()
	closeWebGPUTextRenderer()
	closeWebGPUHUDRenderer()
	activeWebGPUWorldRenderer.Close()
	activeWebGPUWorldRenderer = nil
}
