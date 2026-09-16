//go:build windows

package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type webGPUPoint struct {
	X, Y float32
}

var activeWebGPUWorldRenderer *webGPUWorldRenderer

func webGPUMousePosition() webGPUPoint {
	p := rl.GetMousePosition()
	return webGPUPoint{X: p.X, Y: p.Y}
}

func ensureExperimentalWebGPURenderer() error {
	if activeWebGPUWorldRenderer != nil {
		return nil
	}
	if assets == nil {
		return fmt.Errorf("render assets are not initialized")
	}
	hwnd := uintptr(rl.GetWindowHandle())
	renderer, err := newWebGPUWorldRenderer(hwnd, assets)
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
	return activeWebGPUWorldRenderer.DrawGameplay(world, camera, input)
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
