//go:build windows

package main

import (
	"fmt"
	"log"
	"runtime"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

func init() {
	// Window/input and native graphics backends are both thread-affine on Windows.
	runtime.LockOSThread()
}

const smokeShader = `
struct VertexInput {
    @location(0) position: vec3f,
    @location(1) uv: vec2f,
    @location(2) normal: vec3f,
    @location(3) color: vec4f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) color: vec4f,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4f(in.position, 1.0);
    let normalLight = 0.70 + 0.30 * max(in.normal.z, 0.0);
    out.color = vec4f(in.color.rgb * normalLight, in.color.a);
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4f {
    return in.color;
}
`

func main() {
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(960, 540, "GoCraft WebGPU mesh smoke (GoGPU)")
	defer rl.CloseWindow()
	rl.SetExitKey(0)

	hwnd := uintptr(rl.GetWindowHandle())
	if hwnd == 0 {
		log.Fatal("raylib returned a null native window handle")
	}

	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		log.Fatalf("create WebGPU instance: %v", err)
	}
	defer instance.Release()

	surface, err := instance.CreateSurfaceUnsafe(wgpu.SurfaceTargetFromWindowsHWND(0, hwnd))
	if err != nil {
		log.Fatalf("create WebGPU surface from raylib HWND: %v", err)
	}
	defer surface.Release()

	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surface,
	})
	if err != nil {
		log.Fatalf("request surface-compatible WebGPU adapter: %v", err)
	}
	defer adapter.Release()

	info := adapter.Info()
	fmt.Printf("WebGPU adapter: %s, backend=%v, deviceType=%v\n", info.Name, info.Backend, info.DeviceType)

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		log.Fatalf("request WebGPU device: %v", err)
	}
	defer device.Release()
	queue := device.Queue()
	if queue == nil {
		log.Fatal("WebGPU device returned a nil queue")
	}

	format := gputypes.TextureFormatBGRA8Unorm
	caps := adapter.GetSurfaceCapabilities(surface)
	if caps != nil && len(caps.Formats) > 0 {
		format = caps.Formats[0]
		for _, candidate := range caps.Formats {
			if candidate == gputypes.TextureFormatBGRA8Unorm {
				format = candidate
				break
			}
		}
	}

	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	configure := func() {
		if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
			Format:      format,
			Usage:       gputypes.TextureUsageRenderAttachment,
			Width:       width,
			Height:      height,
			AlphaMode:   gputypes.CompositeAlphaModeOpaque,
			PresentMode: gputypes.PresentModeFifo,
		}); err != nil {
			log.Fatalf("configure WebGPU surface: %v", err)
		}
	}
	configure()

	pipeline, err := createSmokePipeline(device, format)
	if err != nil {
		log.Fatalf("create GoCraft WebGPU mesh pipeline: %v", err)
	}
	defer pipeline.Release()

	meshBackend := platform.NewWebGPUMeshBackend(device)
	mesh, err := meshBackend.UploadChecked(smokeVertices(), []uint32{0, 1, 2, 2, 3, 0})
	if err != nil {
		log.Fatalf("upload GoCraft WebGPU smoke mesh: %v", err)
	}
	defer mesh.Unload()

	fmt.Printf("WebGPU mesh smoke ready: HWND=0x%x, format=%v, %dx%d, vertexStride=%d bytes\n", hwnd, format, width, height, unsafe.Sizeof(platform.Vertex{}))
	fmt.Println("Expected result: blue background with a four-corner colored quad. Resize the window, then close it normally.")

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

		if err := drawMeshFrame(device, queue, surface, pipeline, meshBackend, mesh); err != nil {
			log.Fatalf("render WebGPU smoke frame: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func smokeVertices() []platform.Vertex {
	return []platform.Vertex{
		{Position: [3]float32{-0.65, -0.55, 0}, Texcoord: [2]float32{0, 1}, Color: [4]uint8{255, 90, 90, 255}, Normal: [3]float32{0, 0, 1}},
		{Position: [3]float32{0.65, -0.55, 0}, Texcoord: [2]float32{1, 1}, Color: [4]uint8{90, 255, 120, 255}, Normal: [3]float32{0, 0, 1}},
		{Position: [3]float32{0.65, 0.55, 0}, Texcoord: [2]float32{1, 0}, Color: [4]uint8{100, 150, 255, 255}, Normal: [3]float32{0, 0, 1}},
		{Position: [3]float32{-0.65, 0.55, 0}, Texcoord: [2]float32{0, 0}, Color: [4]uint8{255, 220, 90, 255}, Normal: [3]float32{0, 0, 1}},
	}
}

func createSmokePipeline(device *wgpu.Device, format gputypes.TextureFormat) (*wgpu.RenderPipeline, error) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "GoCraft WebGPU mesh smoke shader",
		WGSL:  smokeShader,
	})
	if err != nil {
		return nil, fmt.Errorf("create WGSL shader: %w", err)
	}
	if shader == nil {
		return nil, fmt.Errorf("create WGSL shader: nil module")
	}
	defer shader.Release()

	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "GoCraft WebGPU smoke layout"})
	if err != nil {
		return nil, fmt.Errorf("create pipeline layout: %w", err)
	}
	defer layout.Release()

	stride := uint64(unsafe.Sizeof(platform.Vertex{}))
	if stride != 36 {
		return nil, fmt.Errorf("unexpected platform.Vertex stride %d, want 36", stride)
	}

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "GoCraft WebGPU mesh smoke pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: stride,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x3, Offset: 24, ShaderLocation: 2},
					{Format: gputypes.VertexFormatUnorm8x4, Offset: 20, ShaderLocation: 3},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, fmt.Errorf("WebGPU returned nil render pipeline")
	}
	return pipeline, nil
}

func drawMeshFrame(device *wgpu.Device, queue *wgpu.Queue, surface *wgpu.Surface, pipeline *wgpu.RenderPipeline, meshBackend *platform.WebGPUMeshBackend, mesh platform.MeshHandle) error {
	surfaceTexture, _, err := surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	if surfaceTexture == nil {
		return fmt.Errorf("surface returned no texture")
	}

	view, err := surfaceTexture.CreateView(nil)
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("create surface texture view: %w", err)
	}
	defer view.Release()

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft WebGPU mesh smoke encoder"})
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("create command encoder: %w", err)
	}

	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft WebGPU mesh smoke pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0.08, G: 0.18, B: 0.38, A: 1.0},
		}},
	})
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("begin render pass: %w", err)
	}
	pass.SetPipeline(pipeline)
	if err := meshBackend.DrawPass(pass, mesh); err != nil {
		_ = pass.End()
		surface.DiscardTexture()
		return fmt.Errorf("draw mesh: %w", err)
	}
	if err := pass.End(); err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("end render pass: %w", err)
	}

	commandBuffer, err := encoder.Finish()
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("finish command encoder: %w", err)
	}
	defer commandBuffer.Release()

	if _, err := queue.Submit(commandBuffer); err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("submit command buffer: %w", err)
	}
	if err := surface.Present(surfaceTexture); err != nil {
		return fmt.Errorf("present surface: %w", err)
	}
	return nil
}
