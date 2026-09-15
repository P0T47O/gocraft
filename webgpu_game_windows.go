//go:build windows

package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var activeWebGPUWorldRenderer *webGPUWorldRenderer

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
	return activeWebGPUWorldRenderer.Draw(world, camera)
}

func closeExperimentalWebGPURenderer() {
	if activeWebGPUWorldRenderer == nil {
		return
	}
	activeWebGPUWorldRenderer.Close()
	activeWebGPUWorldRenderer = nil
}
