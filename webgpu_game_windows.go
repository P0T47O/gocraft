//go:build windows

package main

import (
	"fmt"
)

var activeWebGPUWorldRenderer *webGPUWorldRenderer

func webGPUMousePosition() webGPUPoint {
	p := inputMousePosition()
	return webGPUPoint{X: p.X, Y: p.Y}
}

func webGPUFrameFromWindow() webGPUFrameContext {
	return webGPUFrameContext{
		Camera: webGPUCamera{
			Position: webGPUVec3{X: camera.Position.X, Y: camera.Position.Y, Z: camera.Position.Z},
			Target:   webGPUVec3{X: camera.Target.X, Y: camera.Target.Y, Z: camera.Target.Z},
			Up:       webGPUVec3{X: camera.Up.X, Y: camera.Up.Y, Z: camera.Up.Z},
			Fovy:     camera.Fovy,
		},
		Width:  uint32(max(1, windowWidth())),
		Height: uint32(max(1, windowHeight())),
		Time:   float32(inputTime()),
	}
}

func ensureExperimentalWebGPURenderer() error {
	samples := uint32(normalizeMSAASamples(LoadSettings().MSAASamples))
	if activeWebGPUWorldRenderer != nil {
		if activeWebGPUWorldRenderer.worldSamples == samples || world != nil {
			return nil
		}
		// The renderer survives returning to the main menu. Recreate it only
		// after the previous world released its GPU meshes.
		closeExperimentalWebGPURenderer()
	}
	if assets == nil {
		return fmt.Errorf("render assets are not initialized")
	}
	frame := webGPUFrameFromWindow()
	if nativeGameWindow == nil {
		return fmt.Errorf("native WebGPU window is not initialized")
	}
	hwnd := nativeGameWindow.hwnd
	renderer, err := newWebGPUWorldRenderer(hwnd, assets, frame.Width, frame.Height, samples)
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
	return activeWebGPUWorldRenderer.DrawGameplay(world, webGPUFrameFromWindow(), input)
}

func closeExperimentalWebGPURenderer() {
	if activeWebGPUWorldRenderer == nil {
		resetWebGPUAtlasAnimations()
		return
	}
	closeWebGPUEntityRenderer()
	closeWebGPUTextRenderer()
	closeWebGPUHUDRenderer()
	activeWebGPUWorldRenderer.Close()
	activeWebGPUWorldRenderer = nil
	resetWebGPUAtlasAnimations()
}
