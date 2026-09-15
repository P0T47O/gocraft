//go:build windows

package main

import (
	"fmt"
	"log"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-webgpu/webgpu/wgpu"
)

func main() {
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(960, 540, "GoCraft WebGPU smoke")
	defer rl.CloseWindow()
	rl.SetExitKey(0)

	hwnd := uintptr(rl.GetWindowHandle())
	if hwnd == 0 {
		log.Fatal("raylib returned a null native window handle")
	}

	if err := wgpu.Init(); err != nil {
		log.Fatalf("initialize wgpu-native: %v", err)
	}

	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		log.Fatalf("create WebGPU instance: %v", err)
	}
	defer instance.Release()

	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		log.Fatalf("request WebGPU adapter: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		log.Fatalf("request WebGPU device: %v", err)
	}
	defer device.Release()
	queue := device.Queue()

	surface, err := instance.CreateSurfaceFromWindowsHWND(0, hwnd)
	if err != nil {
		log.Fatalf("create WebGPU surface from raylib HWND: %v", err)
	}
	defer surface.Release()

	caps, err := surface.GetCapabilities(adapter)
	if err != nil {
		log.Fatalf("query surface capabilities: %v", err)
	}
	if len(caps.Formats) == 0 {
		log.Fatal("WebGPU surface reports no supported formats")
	}
	format := caps.Formats[0]
	for _, candidate := range caps.Formats {
		if candidate == wgpu.TextureFormatBGRA8Unorm {
			format = candidate
			break
		}
	}

	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	configure := func() {
		if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
			Format:      format,
			Usage:       wgpu.TextureUsageRenderAttachment,
			Width:       width,
			Height:      height,
			AlphaMode:   wgpu.CompositeAlphaModeOpaque,
			PresentMode: wgpu.PresentModeFifo,
		}); err != nil {
			log.Fatalf("configure WebGPU surface: %v", err)
		}
	}
	configure()

	fmt.Printf("WebGPU smoke ready: HWND=0x%x, format=%v, %dx%d\n", hwnd, format, width, height)
	fmt.Println("Expected result: this window should stay WebGPU-blue. Close it normally to finish the probe.")

	for {
		rl.PollInputEvents()
		if rl.WindowShouldClose() {
			break
		}

		newWidth := uint32(max(1, rl.GetScreenWidth()))
		newHeight := uint32(max(1, rl.GetScreenHeight()))
		if newWidth != width || newHeight != height {
			width, height = newWidth, newHeight
			configure()
		}

		if err := drawClearFrame(device, queue, surface); err != nil {
			if err == wgpu.ErrSurfaceLost || err == wgpu.ErrSurfaceNeedsReconfigure {
				configure()
				continue
			}
			log.Fatalf("render WebGPU smoke frame: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func drawClearFrame(device *wgpu.Device, queue *wgpu.Queue, surface *wgpu.Surface) error {
	surfaceTexture, _, err := surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	if surfaceTexture == nil || surfaceTexture.Texture == nil {
		return fmt.Errorf("surface returned no texture")
	}
	defer surfaceTexture.Texture.Release()

	view, err := surfaceTexture.Texture.CreateView(nil)
	if err != nil {
		return fmt.Errorf("create surface texture view: %w", err)
	}
	defer view.Release()

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft WebGPU smoke encoder"})
	if err != nil {
		return fmt.Errorf("create command encoder: %w", err)
	}

	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft WebGPU smoke clear pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     wgpu.LoadOpClear,
			StoreOp:    wgpu.StoreOpStore,
			ClearValue: wgpu.Color{R: 0.08, G: 0.18, B: 0.38, A: 1.0},
		}},
	})
	if err != nil {
		encoder.Release()
		return fmt.Errorf("begin render pass: %w", err)
	}
	pass.End()
	pass.Release()

	commandBuffer, err := encoder.Finish()
	encoder.Release()
	if err != nil {
		return fmt.Errorf("finish command encoder: %w", err)
	}
	defer commandBuffer.Release()

	if _, err := queue.Submit(commandBuffer); err != nil {
		return fmt.Errorf("submit command buffer: %w", err)
	}
	if err := surface.Present(); err != nil {
		return fmt.Errorf("present surface: %w", err)
	}
	return nil
}
