//go:build windows

package main

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

const webGPUTransparencySceneBytes = 112

const webGPUTransparencyShader = `
struct Scene {
    view_proj: mat4x4<f32>,
    eye_pos: vec4f,
    fog_range: vec4f,
    fog_color: vec4f,
}

@group(0) @binding(0) var<uniform> scene: Scene;
@group(0) @binding(1) var atlas_tex: texture_2d<f32>;
@group(0) @binding(2) var atlas_sampler: sampler;

struct VertexInput {
    @location(0) position: vec3f,
    @location(1) uv: vec2f,
    @location(2) normal: vec3f,
    @location(3) color: vec4f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) world_pos: vec3f,
    @location(1) uv: vec2f,
    @location(2) color: vec4f,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = scene.view_proj * vec4f(in.position, 1.0);
    out.world_pos = in.position;
    out.uv = in.uv;
    out.color = in.color;
    return out;
}

fn fogged(rgb: vec3f, world_pos: vec3f) -> vec3f {
    let d = distance(world_pos, scene.eye_pos.xyz);
    let amount = smoothstep(scene.fog_range.x, scene.fog_range.y, d);
    return mix(rgb, scene.fog_color.rgb, amount);
}

@fragment
fn fs_opaque(in: VertexOutput) -> @location(0) vec4f {
    let texel = textureSample(atlas_tex, atlas_sampler, in.uv);
    let alpha = texel.a * in.color.a;
    if alpha < 0.5 {
        discard;
    }
    let rgb = fogged(texel.rgb * in.color.rgb, in.world_pos);
    return vec4f(rgb, 1.0);
}

@fragment
fn fs_translucent(in: VertexOutput) -> @location(0) vec4f {
    let texel = textureSample(atlas_tex, atlas_sampler, in.uv);
    let alpha = texel.a * in.color.a;
    if alpha < 0.01 {
        discard;
    }
    let rgb = fogged(texel.rgb * in.color.rgb, in.world_pos);
    return vec4f(rgb, alpha);
}
`

type webGPUTransparencyMesh struct {
	mesh   platform.MeshHandle
	cx, cz int
	pass   string
}

// TestWebGPUTransparencyPreview extends the real-texture milestone with the two
// passes that the game still needs before frame-loop integration: opaque/cutout
// first with depth writes, then water/glass back-to-front with alpha blending and
// depth writes disabled. Both passes share distance fog and the real block atlas.
func TestWebGPUTransparencyPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_TRANSPARENCY_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_TRANSPARENCY_PREVIEW=1 to run the water/glass/fog WebGPU preview")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initBlockRegistry()
	const seed uint32 = 12345

	atlasPixels, atlasWidth, atlasHeight, atlasUVs, err := buildWebGPUPreviewAtlas()
	if err != nil {
		t.Fatalf("build WebGPU preview atlas: %v", err)
	}
	assets := &RenderAssets{atlas: &TextureAtlas{UVs: atlasUVs}}

	// Generate a one-chunk halo around the rendered 3x3 region so every visible
	// boundary chunk gets real neighbor culling data.
	chunks := make(map[chunkKey]*Chunk, 25)
	maxTop := 1
	for cx := -2; cx <= 2; cx++ {
		for cz := -2; cz <= 2; cz++ {
			chunk := &Chunk{}
			generateChunkData(seed, cx, cz, chunk)
			chunk.rebuildHeightMap()
			initializeChunkLighting(chunk)
			chunks[chunkKey{X: cx, Z: cz}] = chunk
			if cx >= -1 && cx <= 1 && cz >= -1 && cz <= 1 {
				for x := 0; x < chunkWidth; x++ {
					for z := 0; z < chunkWidth; z++ {
						if top := int(chunk.heightMap[x][z]); top > maxTop {
							maxTop = top
						}
					}
				}
			}
		}
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

	// Keep the preview world-like and include enough depth to show shore water
	// and shallow caves without turning the test into a full-height geology slab.
	yMin := max(0, maxTop-28)
	yMax := min(chunkHeight, maxTop+8)

	type cpuChunkMesh struct {
		cx, cz  int
		results map[string]map[string][]*MeshBuildData
	}
	cpuMeshes := make([]cpuChunkMesh, 0, 9)
	for cx := -1; cx <= 1; cx++ {
		for cz := -1; cz <= 1; cz++ {
			chunk := chunks[chunkKey{X: cx, Z: cz}]
			results := assets.buildAllMeshData(
				&chunk.heightMap,
				cx*chunkWidth,
				cz*chunkWidth,
				yMin,
				yMax,
				getBlock,
				getLight,
				getMeta,
				seed,
			)
			cpuMeshes = append(cpuMeshes, cpuChunkMesh{cx: cx, cz: cz, results: results})
		}
	}
	defer func() {
		for _, item := range cpuMeshes {
			releaseMeshResults(item.results)
		}
	}()

	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(1280, 800, "GoCraft water + glass + fog - WebGPU preview")
	defer rl.CloseWindow()
	rl.SetExitKey(0)

	hwnd := uintptr(rl.GetWindowHandle())
	if hwnd == 0 {
		t.Fatal("raylib returned a null native window handle")
	}
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

	opaquePipeline, translucentPipeline, bindGroup, sceneBuffer, cleanupPipeline, err := createWebGPUTransparencyPipelines(
		device, queue, format, atlasPixels, atlasWidth, atlasHeight,
	)
	if err != nil {
		t.Fatalf("create WebGPU transparency pipelines: %v", err)
	}
	defer cleanupPipeline()

	backend := platform.NewWebGPUMeshBackend(device)
	opaqueMeshes := make([]webGPUTransparencyMesh, 0, 64)
	translucentMeshes := make([]webGPUTransparencyMesh, 0, 32)
	verticesByPass := map[string]int{}
	indicesByPass := map[string]int{}
	skippedNonAtlas := 0
	for _, item := range cpuMeshes {
		for _, passName := range []string{"opaque", "cutout", "water", "glass"} {
			for path, list := range item.results[passName] {
				if path != "atlas" {
					for _, data := range list {
						if data != nil {
							skippedNonAtlas += data.vertCount
						}
					}
					continue
				}
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
					mesh, uploadErr := backend.UploadChecked(vertices, data.indices)
					if uploadErr != nil {
						t.Fatalf("upload (%d,%d) %s mesh: %v", item.cx, item.cz, passName, uploadErr)
					}
					draw := webGPUTransparencyMesh{mesh: mesh, cx: item.cx, cz: item.cz, pass: passName}
					if passName == "water" || passName == "glass" {
						translucentMeshes = append(translucentMeshes, draw)
					} else {
						opaqueMeshes = append(opaqueMeshes, draw)
					}
					verticesByPass[passName] += len(vertices)
					indicesByPass[passName] += len(data.indices)
				}
			}
		}
	}
	defer func() {
		for _, draw := range opaqueMeshes {
			draw.mesh.Unload()
		}
		for _, draw := range translucentMeshes {
			draw.mesh.Unload()
		}
	}()
	if len(opaqueMeshes) == 0 {
		t.Fatal("transparency preview produced no opaque/cutout meshes")
	}

	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	var depthTexture *wgpu.Texture
	var depthView *wgpu.TextureView
	reconfigure := func() error {
		if err := surface.Configure(device, &wgpu.SurfaceConfiguration{
			Format: format, Usage: gputypes.TextureUsageRenderAttachment,
			Width: width, Height: height,
			AlphaMode: gputypes.CompositeAlphaModeOpaque,
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
		t.Fatalf("configure transparency preview: %v", err)
	}
	defer func() {
		if depthView != nil {
			depthView.Release()
		}
		if depthTexture != nil {
			depthTexture.Release()
		}
	}()

	t.Logf("water/glass/fog ready: atlas=%dx%d textures=%d yRange=[%d,%d) opaqueMeshes=%d translucentMeshes=%d waterTriangles=%d glassTriangles=%d skippedNonAtlasVertices=%d",
		atlasWidth, atlasHeight, len(atlasUVs), yMin, yMax, len(opaqueMeshes), len(translucentMeshes), indicesByPass["water"]/3, indicesByPass["glass"]/3, skippedNonAtlas)
	t.Log("Expected result: textured terrain with shoreline water and any glass rendered translucent; distant terrain fades into the sky color. Transparent chunk batches are sorted back-to-front and do not write depth.")

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
			if err := reconfigure(); err != nil {
				t.Fatalf("resize transparency preview: %v", err)
			}
		}

		angle := float32(time.Since(started).Seconds()) * 0.035
		eye, err := updateWebGPUTransparencyScene(queue, sceneBuffer, width, height, maxTop, angle)
		if err != nil {
			t.Fatalf("update WebGPU transparency scene: %v", err)
		}
		sortWebGPUTransparentMeshes(translucentMeshes, eye, maxTop)
		if err := drawWebGPUTransparencyFrame(
			device, queue, surface, opaquePipeline, translucentPipeline, bindGroup, depthView, backend, opaqueMeshes, translucentMeshes,
		); err != nil {
			t.Fatalf("draw WebGPU transparency preview: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func createWebGPUTransparencyPipelines(
	device *wgpu.Device,
	queue *wgpu.Queue,
	format gputypes.TextureFormat,
	atlasPixels []byte,
	atlasWidth, atlasHeight int,
) (*wgpu.RenderPipeline, *wgpu.RenderPipeline, *wgpu.BindGroup, *wgpu.Buffer, func(), error) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft water/glass/fog shader", WGSL: webGPUTransparencyShader})
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	sceneBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft water/glass/fog scene",
		Size:  webGPUTransparencySceneBytes,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "GoCraft water/glass/fog atlas",
		Size: wgpu.Extent3D{Width: uint32(atlasWidth), Height: uint32(atlasHeight), DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D,
		Format: gputypes.TextureFormatRGBA8Unorm,
		Usage: gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	if err := queue.WriteTexture(
		&wgpu.ImageCopyTexture{Texture: texture, MipLevel: 0}, atlasPixels,
		&wgpu.ImageDataLayout{Offset: 0, BytesPerRow: uint32(atlasWidth * 4), RowsPerImage: uint32(atlasHeight)},
		&wgpu.Extent3D{Width: uint32(atlasWidth), Height: uint32(atlasHeight), DepthOrArrayLayers: 1},
	); err != nil {
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label: "GoCraft water/glass/fog atlas sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		MagFilter: gputypes.FilterModeNearest,
		MinFilter: gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
	})
	if err != nil {
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft water/glass/fog BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: webGPUTransparencySceneBytes}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "GoCraft water/glass/fog layout", BindGroupLayouts: []*wgpu.BindGroupLayout{bgl}})
	if err != nil {
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label: "GoCraft water/glass/fog bind group", Layout: bgl,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: sceneBuffer, Size: webGPUTransparencySceneBytes},
			{Binding: 1, TextureView: view},
			{Binding: 2, Sampler: sampler},
		},
	})
	if err != nil {
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}

	ignoreStencil := wgpu.StencilFaceState{
		Compare: gputypes.CompareFunctionAlways,
		FailOp: gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp: gputypes.StencilOperationKeep,
	}
	stride := uint64(unsafe.Sizeof(platform.Vertex{}))
	vertexState := wgpu.VertexState{
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
	}
	primitive := gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone}
	multisample := gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF}

	opaquePipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft fogged opaque pipeline", Layout: layout,
		Vertex: vertexState, Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUChunkPreviewDepthFormat, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionLess,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment: &wgpu.FragmentState{Module: shader, EntryPoint: "fs_opaque", Targets: []gputypes.ColorTargetState{{Format: format, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}

	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	translucentPipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft fogged translucent pipeline", Layout: layout,
		Vertex: vertexState, Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUChunkPreviewDepthFormat, DepthWriteEnabled: false, DepthCompare: gputypes.CompareFunctionLess,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment: &wgpu.FragmentState{Module: shader, EntryPoint: "fs_translucent", Targets: []gputypes.ColorTargetState{{Format: format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		opaquePipeline.Release()
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, err
	}

	cleanup := func() {
		translucentPipeline.Release()
		opaquePipeline.Release()
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		sceneBuffer.Release()
		shader.Release()
	}
	return opaquePipeline, translucentPipeline, bindGroup, sceneBuffer, cleanup, nil
}

func updateWebGPUTransparencyScene(queue *wgpu.Queue, buffer *wgpu.Buffer, width, height uint32, maxTop int, angle float32) (mgl32.Vec3, error) {
	aspect := float32(width) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(52), aspect, 0.1, 1000.0)
	center := mgl32.Vec3{float32(chunkWidth) * 0.5, float32(maxTop) - 7, float32(chunkWidth) * 0.5}
	radius := float32(chunkWidth) * 5.0
	eye := mgl32.Vec3{
		center.X() + float32(math.Cos(float64(angle)))*radius,
		float32(maxTop) + float32(chunkWidth)*3.6,
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
	fogStart := float32(chunkWidth) * 5.5
	fogEnd := float32(chunkWidth) * 10.0
	fogColor := [4]float32{0.52, 0.72, 0.92, 1.0}

	bytes := make([]byte, webGPUTransparencySceneBytes)
	put := func(offset int, value float32) {
		binary.LittleEndian.PutUint32(bytes[offset:], math.Float32bits(value))
	}
	for i := 0; i < 16; i++ {
		put(i*4, vp[i])
	}
	put(64, eye.X())
	put(68, eye.Y())
	put(72, eye.Z())
	put(76, 1)
	put(80, fogStart)
	put(84, fogEnd)
	put(88, 0)
	put(92, 0)
	for i, value := range fogColor {
		put(96+i*4, value)
	}
	if err := queue.WriteBuffer(buffer, 0, bytes); err != nil {
		return eye, fmt.Errorf("write WebGPU transparency scene: %w", err)
	}
	return eye, nil
}

func sortWebGPUTransparentMeshes(meshes []webGPUTransparencyMesh, eye mgl32.Vec3, maxTop int) {
	slices.SortStableFunc(meshes, func(a, b webGPUTransparencyMesh) int {
		distance := func(item webGPUTransparencyMesh) float32 {
			center := mgl32.Vec3{
				float32(item.cx*chunkWidth) + float32(chunkWidth)*0.5,
				float32(maxTop) - 8,
				float32(item.cz*chunkWidth) + float32(chunkWidth)*0.5,
			}
			delta := eye.Sub(center)
			return delta.Dot(delta)
		}
		if order := cmp.Compare(distance(b), distance(a)); order != 0 {
			return order
		}
		// Match the game's deterministic tie-breaking philosophy. Water before
		// glass within the same batch/distance keeps ordering stable frame-to-frame.
		if a.cz != b.cz {
			return cmp.Compare(a.cz, b.cz)
		}
		if a.cx != b.cx {
			return cmp.Compare(a.cx, b.cx)
		}
		return cmp.Compare(a.pass, b.pass)
	})
}

func drawWebGPUTransparencyFrame(
	device *wgpu.Device,
	queue *wgpu.Queue,
	surface *wgpu.Surface,
	opaquePipeline, translucentPipeline *wgpu.RenderPipeline,
	bindGroup *wgpu.BindGroup,
	depthView *wgpu.TextureView,
	backend *platform.WebGPUMeshBackend,
	opaqueMeshes, translucentMeshes []webGPUTransparencyMesh,
) error {
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
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft water/glass/fog encoder"})
	if err != nil {
		surface.DiscardTexture()
		return err
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft water/glass/fog pass",
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

	pass.SetBindGroup(0, bindGroup, nil)
	pass.SetPipeline(opaquePipeline)
	for _, draw := range opaqueMeshes {
		if err := backend.DrawPass(pass, draw.mesh); err != nil {
			_ = pass.End()
			surface.DiscardTexture()
			return err
		}
	}
	pass.SetPipeline(translucentPipeline)
	for _, draw := range translucentMeshes {
		if err := backend.DrawPass(pass, draw.mesh); err != nil {
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
