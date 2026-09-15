//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"runtime"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

func init() {
	// Window/input and native graphics backends are both thread-affine on Windows.
	runtime.LockOSThread()
}

const (
	smokeDepthFormat = gputypes.TextureFormatDepth24Plus
	cameraUniformSize = 64
)

const smokeShader = `
struct Camera {
    view_proj: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;

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
    out.position = camera.view_proj * vec4f(in.position, 1.0);
    let lightDir = normalize(vec3f(0.35, 0.80, 0.45));
    let normalLight = 0.42 + 0.58 * max(dot(normalize(in.normal), lightDir), 0.0);
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
	rl.InitWindow(960, 540, "GoCraft WebGPU 3D smoke (GoGPU)")
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

	pipeline, cameraBindGroup, cameraBuffer, cleanupPipeline, err := createSmokePipeline(device, format)
	if err != nil {
		log.Fatalf("create GoCraft WebGPU 3D pipeline: %v", err)
	}
	defer cleanupPipeline()

	meshBackend := platform.NewWebGPUMeshBackend(device)
	vertices, indices := smokeCube()
	mesh, err := meshBackend.UploadChecked(vertices, indices)
	if err != nil {
		log.Fatalf("upload GoCraft WebGPU smoke cube: %v", err)
	}
	defer mesh.Unload()

	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	var depthTexture *wgpu.Texture
	var depthView *wgpu.TextureView

	reconfigure := func() {
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

		if depthView != nil {
			depthView.Release()
			depthView = nil
		}
		if depthTexture != nil {
			depthTexture.Release()
			depthTexture = nil
		}

		var err error
		depthTexture, depthView, err = createDepthTarget(device, width, height)
		if err != nil {
			log.Fatalf("create WebGPU depth target: %v", err)
		}
	}
	reconfigure()
	defer func() {
		if depthView != nil {
			depthView.Release()
		}
		if depthTexture != nil {
			depthTexture.Release()
		}
	}()

	fmt.Printf("WebGPU 3D smoke ready: HWND=0x%x, format=%v, depth=%v, %dx%d, vertexStride=%d bytes\n", hwnd, format, smokeDepthFormat, width, height, unsafe.Sizeof(platform.Vertex{}))
	fmt.Println("Expected result: a rotating colored 3D cube with perspective and correct hidden-surface depth. Resize the window, then close it normally.")

	started := time.Now()
	for {
		rl.PollInputEvents()
		if rl.WindowShouldClose() {
			break
		}

		newWidth := uint32(max(1, rl.GetScreenWidth()))
		newHeight := uint32(max(1, rl.GetScreenHeight()))
		if newWidth != width || newHeight != height {
			width, height = newWidth, newHeight
			reconfigure()
		}

		angle := float32(time.Since(started).Seconds()) * 0.75
		if err := updateCamera(queue, cameraBuffer, width, height, angle); err != nil {
			log.Fatalf("update WebGPU camera: %v", err)
		}
		if err := drawMeshFrame(device, queue, surface, pipeline, cameraBindGroup, depthView, meshBackend, mesh); err != nil {
			log.Fatalf("render WebGPU 3D smoke frame: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func smokeCube() ([]platform.Vertex, []uint32) {
	const h = float32(0.72)
	face := func(normal [3]float32, color [4]uint8, corners ...[3]float32) []platform.Vertex {
		uvs := [][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}
		out := make([]platform.Vertex, 4)
		for i := range out {
			out[i] = platform.Vertex{Position: corners[i], Texcoord: uvs[i], Color: color, Normal: normal}
		}
		return out
	}

	vertices := make([]platform.Vertex, 0, 24)
	vertices = append(vertices, face([3]float32{0, 0, 1}, [4]uint8{255, 105, 95, 255},
		[3]float32{-h, -h, h}, [3]float32{h, -h, h}, [3]float32{h, h, h}, [3]float32{-h, h, h})...)
	vertices = append(vertices, face([3]float32{0, 0, -1}, [4]uint8{100, 150, 255, 255},
		[3]float32{h, -h, -h}, [3]float32{-h, -h, -h}, [3]float32{-h, h, -h}, [3]float32{h, h, -h})...)
	vertices = append(vertices, face([3]float32{1, 0, 0}, [4]uint8{100, 235, 145, 255},
		[3]float32{h, -h, h}, [3]float32{h, -h, -h}, [3]float32{h, h, -h}, [3]float32{h, h, h})...)
	vertices = append(vertices, face([3]float32{-1, 0, 0}, [4]uint8{255, 210, 90, 255},
		[3]float32{-h, -h, -h}, [3]float32{-h, -h, h}, [3]float32{-h, h, h}, [3]float32{-h, h, -h})...)
	vertices = append(vertices, face([3]float32{0, 1, 0}, [4]uint8{225, 125, 255, 255},
		[3]float32{-h, h, h}, [3]float32{h, h, h}, [3]float32{h, h, -h}, [3]float32{-h, h, -h})...)
	vertices = append(vertices, face([3]float32{0, -1, 0}, [4]uint8{100, 220, 235, 255},
		[3]float32{-h, -h, -h}, [3]float32{h, -h, -h}, [3]float32{h, -h, h}, [3]float32{-h, -h, h})...)

	indices := make([]uint32, 0, 36)
	for faceIndex := uint32(0); faceIndex < 6; faceIndex++ {
		base := faceIndex * 4
		indices = append(indices, base, base+1, base+2, base, base+2, base+3)
	}
	return vertices, indices
}

func createSmokePipeline(device *wgpu.Device, format gputypes.TextureFormat) (*wgpu.RenderPipeline, *wgpu.BindGroup, *wgpu.Buffer, func(), error) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "GoCraft WebGPU 3D smoke shader",
		WGSL:  smokeShader,
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create WGSL shader: %w", err)
	}
	if shader == nil {
		return nil, nil, nil, nil, fmt.Errorf("create WGSL shader: nil module")
	}

	cameraBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft WebGPU camera uniform",
		Size:  cameraUniformSize,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("create camera uniform buffer: %w", err)
	}

	bindGroupLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft WebGPU camera bind group layout",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding:    0,
			Visibility: wgpu.ShaderStageVertex,
			Buffer: &gputypes.BufferBindingLayout{
				Type:           gputypes.BufferBindingTypeUniform,
				MinBindingSize: cameraUniformSize,
			},
		}},
	})
	if err != nil {
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("create camera bind group layout: %w", err)
	}

	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "GoCraft WebGPU 3D smoke layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{bindGroupLayout},
	})
	if err != nil {
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	cameraBindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "GoCraft WebGPU camera bind group",
		Layout: bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{{
			Binding: 0,
			Buffer:  cameraBuffer,
			Size:    cameraUniformSize,
		}},
	})
	if err != nil {
		layout.Release()
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("create camera bind group: %w", err)
	}

	stride := uint64(unsafe.Sizeof(platform.Vertex{}))
	if stride != 36 {
		cameraBindGroup.Release()
		layout.Release()
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("unexpected platform.Vertex stride %d, want 36", stride)
	}

	stencilIgnore := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp:      gputypes.StencilOperationKeep,
	}

	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "GoCraft WebGPU 3D smoke pipeline",
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
		DepthStencil: &wgpu.DepthStencilState{
			Format:              smokeDepthFormat,
			DepthWriteEnabled:   true,
			DepthCompare:        gputypes.CompareFunctionLess,
			StencilFront:        stencilIgnore,
			StencilBack:         stencilIgnore,
			StencilReadMask:     0xFFFFFFFF,
			StencilWriteMask:    0xFFFFFFFF,
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
		cameraBindGroup.Release()
		layout.Release()
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, err
	}
	if pipeline == nil {
		cameraBindGroup.Release()
		layout.Release()
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, fmt.Errorf("WebGPU returned nil render pipeline")
	}

	cleanup := func() {
		pipeline.Release()
		cameraBindGroup.Release()
		layout.Release()
		bindGroupLayout.Release()
		cameraBuffer.Release()
		shader.Release()
	}
	return pipeline, cameraBindGroup, cameraBuffer, cleanup, nil
}

func createDepthTarget(device *wgpu.Device, width, height uint32) (*wgpu.Texture, *wgpu.TextureView, error) {
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "GoCraft WebGPU smoke depth",
		Size: wgpu.Extent3D{
			Width:              width,
			Height:             height,
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        smokeDepthFormat,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil, nil, err
	}
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		return nil, nil, err
	}
	return texture, view, nil
}

func updateCamera(queue *wgpu.Queue, cameraBuffer *wgpu.Buffer, width, height uint32, angle float32) error {
	aspect := float32(width) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(58), aspect, 0.1, 100.0)
	view := mgl32.LookAtV(
		mgl32.Vec3{2.6, 2.0, 3.6},
		mgl32.Vec3{0, 0, 0},
		mgl32.Vec3{0, 1, 0},
	)
	model := mgl32.HomogRotate3DY(angle).Mul4(mgl32.HomogRotate3DX(angle * 0.55))

	// MathGL uses OpenGL's -1..1 clip-space Z. WebGPU uses 0..1, so remap
	// z' = 0.5*z + 0.5*w before feeding the matrix to WGSL.
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0.5, 0,
		0, 0, 0.5, 1,
	}
	mvp := clipCorrection.Mul4(projection).Mul4(view).Mul4(model)

	bytes := make([]byte, cameraUniformSize)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(mvp[i]))
	}
	return queue.WriteBuffer(cameraBuffer, 0, bytes)
}

func drawMeshFrame(device *wgpu.Device, queue *wgpu.Queue, surface *wgpu.Surface, pipeline *wgpu.RenderPipeline, cameraBindGroup *wgpu.BindGroup, depthView *wgpu.TextureView, meshBackend *platform.WebGPUMeshBackend, mesh platform.MeshHandle) error {
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

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft WebGPU 3D smoke encoder"})
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("create command encoder: %w", err)
	}

	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft WebGPU 3D smoke pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0.08, G: 0.18, B: 0.38, A: 1.0},
		}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View:            depthView,
			DepthLoadOp:     gputypes.LoadOpClear,
			DepthStoreOp:    gputypes.StoreOpStore,
			DepthClearValue: 1.0,
			DepthReadOnly:   false,
		},
	})
	if err != nil {
		surface.DiscardTexture()
		return fmt.Errorf("begin render pass: %w", err)
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, cameraBindGroup, nil)
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