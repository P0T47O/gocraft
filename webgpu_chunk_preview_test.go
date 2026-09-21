//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

const webGPUChunkPreviewDepthFormat = gputypes.TextureFormatDepth24Plus
const webGPUChunkPreviewCameraBytes = 64

const webGPUChunkPreviewShader = `
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
    let light_dir = normalize(vec3f(0.35, 0.80, 0.45));
    let light = 0.58 + 0.42 * max(dot(normalize(in.normal), light_dir), 0.0);
    out.color = vec4f(in.color.rgb * light, 1.0);
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4f {
    return in.color;
}
`

func TestWebGPUChunkPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_CHUNK_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_CHUNK_PREVIEW=1 to run the interactive WebGPU chunk preview")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initBlockRegistry()
	const seed uint32 = 12345

	chunks := make(map[chunkKey]*Chunk, 9)
	for cx := -1; cx <= 1; cx++ {
		for cz := -1; cz <= 1; cz++ {
			chunk := &Chunk{}
			generateChunkData(seed, cx, cz, chunk)
			chunk.rebuildHeightMap()
			initializeChunkLighting(chunk)
			chunks[chunkKey{X: cx, Z: cz}] = chunk
		}
	}
	center := chunks[chunkKey{}]
	if center == nil {
		t.Fatal("generated center chunk is nil")
	}

	getChunk := func(wx, wz int) (*Chunk, int, int) {
		cx := divFloor(wx, chunkWidth)
		cz := divFloor(wz, chunkWidth)
		chunk := chunks[chunkKey{X: cx, Z: cz}]
		if chunk == nil {
			return nil, 0, 0
		}
		return chunk, modFloor(wx, chunkWidth), modFloor(wz, chunkWidth)
	}
	getBlock := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return blockAir
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return blockAir
		}
		return chunk.blocks[lx][wy][lz]
	}
	getLight := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return 15
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return 15
		}
		sky := chunk.skyLight[lx][wy][lz]
		block := chunk.blockLight[lx][wy][lz]
		if block > sky {
			return block
		}
		return sky
	}
	getMeta := func(wx, wy, wz int) byte {
		if wy < 0 || wy >= chunkHeight {
			return 0
		}
		chunk, lx, lz := getChunk(wx, wz)
		if chunk == nil {
			return 0
		}
		return chunk.meta[lx][wy][lz]
	}

	assets := &RenderAssets{}
	results := assets.buildAllMeshData(&center.heightMap, 0, 0, 0, chunkHeight, getBlock, getLight, getMeta, seed)
	defer releaseMeshResults(results)

	preview, closePreview := newNativePreviewWindow(t, 1100, 720)
	defer closePreview()
	hwnd := preview.window.hwnd

	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		t.Fatalf("create WebGPU instance: %v", err)
	}
	defer instance.Release()

	surface, err := instance.CreateSurfaceUnsafe(wgpu.SurfaceTargetFromWindowsHWND(0, hwnd))
	if err != nil {
		t.Fatalf("create WebGPU surface: %v", err)
	}
	defer surface.Release()

	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surface,
	})
	if err != nil {
		t.Fatalf("request WebGPU adapter: %v", err)
	}
	defer adapter.Release()
	info := adapter.Info()
	t.Logf("WebGPU adapter: %s, backend=%v, deviceType=%v", info.Name, info.Backend, info.DeviceType)

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("request WebGPU device: %v", err)
	}
	defer device.Release()
	queue := device.Queue()
	if queue == nil {
		t.Fatal("WebGPU device returned nil queue")
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

	pipeline, cameraBindGroup, cameraBuffer, cleanupPipeline, err := createWebGPUChunkPreviewPipeline(device, format)
	if err != nil {
		t.Fatalf("create chunk preview pipeline: %v", err)
	}
	defer cleanupPipeline()

	backend := platform.NewWebGPUMeshBackend(device)
	var meshes []platform.MeshHandle
	var verticesTotal, indicesTotal int
	for _, passName := range []string{"opaque", "cutout"} {
		for _, list := range results[passName] {
			for _, data := range list {
				if data == nil || data.vertCount == 0 || len(data.indices) == 0 {
					continue
				}
				vertices := make([]platform.Vertex, data.vertCount)
				for i := 0; i < data.vertCount; i++ {
					vertices[i] = platform.Vertex{
						Position: [3]float32{data.vertices[i*3], data.vertices[i*3+1], data.vertices[i*3+2]},
						Texcoord: [2]float32{data.texcoords[i*2], data.texcoords[i*2+1]},
						Color:    [4]uint8{data.colors[i*4], data.colors[i*4+1], data.colors[i*4+2], data.colors[i*4+3]},
						Normal:   [3]float32{data.normals[i*3], data.normals[i*3+1], data.normals[i*3+2]},
					}
				}
				mesh, err := backend.UploadChecked(vertices, data.indices)
				if err != nil {
					t.Fatalf("upload %s chunk mesh: %v", passName, err)
				}
				meshes = append(meshes, mesh)
				verticesTotal += len(vertices)
				indicesTotal += len(data.indices)
			}
		}
	}
	defer func() {
		for _, mesh := range meshes {
			mesh.Unload()
		}
	}()
	if len(meshes) == 0 {
		t.Fatal("real chunk mesher produced no opaque/cutout meshes")
	}

	width := uint32(max(1, preview.window.width))
	height := uint32(max(1, preview.window.height))
	var depthTexture *wgpu.Texture
	var depthView *wgpu.TextureView
	reconfigure := func() error {
		if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
			Format:      format,
			Usage:       gputypes.TextureUsageRenderAttachment,
			Width:       width,
			Height:      height,
			AlphaMode:   gputypes.CompositeAlphaModeOpaque,
			PresentMode: gputypes.PresentModeFifo,
		}); err != nil {
			return err
		}
		if depthView != nil {
			depthView.Release()
			depthView = nil
		}
		if depthTexture != nil {
			depthTexture.Release()
			depthTexture = nil
		}
		depthTexture, depthView, err = createWebGPUChunkPreviewDepth(device, width, height)
		return err
	}
	if err := reconfigure(); err != nil {
		t.Fatalf("configure chunk preview surface: %v", err)
	}
	defer func() {
		if depthView != nil {
			depthView.Release()
		}
		if depthTexture != nil {
			depthTexture.Release()
		}
	}()

	centerY := float32(center.heightMap[chunkWidth/2][chunkWidth/2])
	if centerY < 24 {
		centerY = 24
	}
	t.Logf("real chunk mesh ready: meshes=%d vertices=%d indices=%d triangles=%d centerY=%.1f", len(meshes), verticesTotal, indicesTotal, indicesTotal/3, centerY)
	t.Log("Expected result: the actual generated GoCraft center chunk rotates slowly; textures are intentionally omitted in this milestone, so vertex lighting/tint shows the terrain shape.")

	started := time.Now()
	for preview.nextFrame() {
		newWidth := uint32(max(1, preview.window.width))
		newHeight := uint32(max(1, preview.window.height))
		if newWidth != width || newHeight != height {
			width, height = newWidth, newHeight
			if err := reconfigure(); err != nil {
				t.Fatalf("resize chunk preview: %v", err)
			}
		}

		angle := float32(time.Since(started).Seconds()) * 0.12
		if err := updateWebGPUChunkPreviewCamera(queue, cameraBuffer, width, height, centerY, angle); err != nil {
			t.Fatalf("update chunk preview camera: %v", err)
		}
		if err := drawWebGPUChunkPreviewFrame(device, queue, surface, pipeline, cameraBindGroup, depthView, backend, meshes); err != nil {
			t.Fatalf("draw chunk preview: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func createWebGPUChunkPreviewPipeline(device *wgpu.Device, format gputypes.TextureFormat) (*wgpu.RenderPipeline, *wgpu.BindGroup, *wgpu.Buffer, func(), error) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft real chunk shader", WGSL: webGPUChunkPreviewShader})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	cameraBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft real chunk camera",
		Size:  webGPUChunkPreviewCameraBytes,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		shader.Release()
		return nil, nil, nil, nil, err
	}
	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft real chunk camera BGL",
		Entries: []gputypes.BindGroupLayoutEntry{{
			Binding: 0, Visibility: wgpu.ShaderStageVertex,
			Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: webGPUChunkPreviewCameraBytes},
		}},
	})
	if err != nil {
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, err
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "GoCraft real chunk layout", BindGroupLayouts: []*wgpu.BindGroupLayout{bgl}})
	if err != nil {
		bgl.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, err
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label: "GoCraft real chunk camera BG", Layout: bgl,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, Buffer: cameraBuffer, Size: webGPUChunkPreviewCameraBytes}},
	})
	if err != nil {
		layout.Release()
		bgl.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, err
	}

	ignoreStencil := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp:      gputypes.StencilOperationKeep,
	}
	stride := uint64(unsafe.Sizeof(platform.Vertex{}))
	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "GoCraft real chunk pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module: shader, EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: stride, StepMode: gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x3, Offset: 24, ShaderLocation: 2},
					{Format: gputypes.VertexFormatUnorm8x4, Offset: 20, ShaderLocation: 3},
				},
			}},
		},
		Primitive: gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUChunkPreviewDepthFormat, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionLess,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment:    &wgpu.FragmentState{Module: shader, EntryPoint: "fs_main", Targets: []gputypes.ColorTargetState{{Format: format, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, err
	}
	cleanup := func() {
		pipeline.Release()
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		cameraBuffer.Release()
		shader.Release()
	}
	return pipeline, bindGroup, cameraBuffer, cleanup, nil
}

func createWebGPUChunkPreviewDepth(device *wgpu.Device, width, height uint32) (*wgpu.Texture, *wgpu.TextureView, error) {
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "GoCraft real chunk depth",
		Size:          wgpu.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D,
		Format: webGPUChunkPreviewDepthFormat, Usage: gputypes.TextureUsageRenderAttachment,
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

func updateWebGPUChunkPreviewCamera(queue *wgpu.Queue, buffer *wgpu.Buffer, width, height uint32, centerY, angle float32) error {
	aspect := float32(width) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(58), aspect, 0.1, 600.0)
	center := mgl32.Vec3{float32(chunkWidth) * 0.5, centerY * 0.72, float32(chunkWidth) * 0.5}
	radius := float32(42)
	eye := mgl32.Vec3{
		center.X() + float32(math.Cos(float64(angle)))*radius,
		center.Y() + 24,
		center.Z() + float32(math.Sin(float64(angle)))*radius,
	}
	view := mgl32.LookAtV(eye, center, mgl32.Vec3{0, 1, 0})
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0.5, 0,
		0, 0, 0.5, 1,
	}
	vp := clipCorrection.Mul4(projection).Mul4(view)
	bytes := make([]byte, webGPUChunkPreviewCameraBytes)
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(bytes[i*4:], math.Float32bits(vp[i]))
	}
	return queue.WriteBuffer(buffer, 0, bytes)
}

func drawWebGPUChunkPreviewFrame(device *wgpu.Device, queue *wgpu.Queue, surface *wgpu.Surface, pipeline *wgpu.RenderPipeline, cameraBindGroup *wgpu.BindGroup, depthView *wgpu.TextureView, backend *platform.WebGPUMeshBackend, meshes []platform.MeshHandle) error {
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
		return err
	}
	defer view.Release()

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft real chunk encoder"})
	if err != nil {
		surface.DiscardTexture()
		return err
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft real chunk pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0.52, G: 0.72, B: 0.92, A: 1},
		}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View: depthView, DepthLoadOp: gputypes.LoadOpClear, DepthStoreOp: gputypes.StoreOpStore,
			DepthClearValue: 1, DepthReadOnly: false,
		},
	})
	if err != nil {
		surface.DiscardTexture()
		return err
	}
	pass.SetPipeline(pipeline)
	pass.SetBindGroup(0, cameraBindGroup, nil)
	for _, mesh := range meshes {
		if err := backend.DrawPass(pass, mesh); err != nil {
			_ = pass.End()
			surface.DiscardTexture()
			return err
		}
	}
	if err := pass.End(); err != nil {
		surface.DiscardTexture()
		return err
	}
	commandBuffer, err := encoder.Finish()
	if err != nil {
		surface.DiscardTexture()
		return err
	}
	defer commandBuffer.Release()
	if _, err := queue.Submit(commandBuffer); err != nil {
		surface.DiscardTexture()
		return err
	}
	return surface.Present(surfaceTexture)
}
