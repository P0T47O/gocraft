//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

const webGPUWorldSceneBytes = 112
const webGPUWorldDepthFormat = gputypes.TextureFormatDepth24Plus

const webGPUWorldShader = `
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

type webGPUWorldRenderer struct {
	instance *wgpu.Instance
	surface  *wgpu.Surface
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue
	backend  *platform.WebGPUMeshBackend

	format              gputypes.TextureFormat
	opaquePipeline      *wgpu.RenderPipeline
	translucentPipeline *wgpu.RenderPipeline
	bindGroup           *wgpu.BindGroup
	sceneBuffer         *wgpu.Buffer
	shader              *wgpu.ShaderModule
	layout              *wgpu.PipelineLayout
	bindGroupLayout     *wgpu.BindGroupLayout
	atlasTexture        *wgpu.Texture
	atlasView           *wgpu.TextureView
	atlasSampler        *wgpu.Sampler
	depthTexture        *wgpu.Texture
	depthView           *wgpu.TextureView
	width, height       uint32
}

func newWebGPUWorldRenderer(hwnd uintptr, assets *RenderAssets) (*webGPUWorldRenderer, error) {
	if hwnd == 0 {
		return nil, fmt.Errorf("raylib returned a null native window handle")
	}
	atlas, err := buildWebGPUBlockAtlas()
	if err != nil {
		return nil, fmt.Errorf("build block atlas: %w", err)
	}

	r := &webGPUWorldRenderer{}
	fail := func(err error) (*webGPUWorldRenderer, error) {
		r.closeResources(false)
		return nil, err
	}

	r.instance, err = wgpu.CreateInstance(nil)
	if err != nil {
		return fail(fmt.Errorf("create WebGPU instance: %w", err))
	}
	r.surface, err = r.instance.CreateSurfaceUnsafe(wgpu.SurfaceTargetFromWindowsHWND(0, hwnd))
	if err != nil {
		return fail(fmt.Errorf("create WebGPU surface: %w", err))
	}
	r.adapter, err = r.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: r.surface,
	})
	if err != nil {
		return fail(fmt.Errorf("request WebGPU adapter: %w", err))
	}
	r.device, err = r.adapter.RequestDevice(nil)
	if err != nil {
		return fail(fmt.Errorf("request WebGPU device: %w", err))
	}
	r.queue = r.device.Queue()
	if r.queue == nil {
		return fail(fmt.Errorf("WebGPU device returned nil queue"))
	}

	r.format = gputypes.TextureFormatBGRA8Unorm
	caps := r.adapter.GetSurfaceCapabilities(r.surface)
	if caps != nil && len(caps.Formats) > 0 {
		r.format = caps.Formats[0]
		for _, candidate := range caps.Formats {
			if candidate == gputypes.TextureFormatBGRA8Unorm {
				r.format = candidate
				break
			}
		}
	}

	if err := r.createPipelineResources(atlas); err != nil {
		return fail(err)
	}
	if assets.atlas == nil {
		assets.atlas = &TextureAtlas{}
	}
	assets.atlas.UVs = atlas.uvs

	r.width = uint32(max(1, rl.GetScreenWidth()))
	r.height = uint32(max(1, rl.GetScreenHeight()))
	if err := r.configureSurface(); err != nil {
		return fail(err)
	}

	r.backend = platform.EnableExperimentalWebGPU(r.device)
	info := r.adapter.Info()
	fmt.Printf("Experimental WebGPU renderer: %s, backend=%v, deviceType=%v, atlas=%dx%d\n", info.Name, info.Backend, info.DeviceType, atlas.width, atlas.height)
	return r, nil
}

func (r *webGPUWorldRenderer) createPipelineResources(atlas *webGPUBlockAtlas) error {
	var err error
	r.shader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft WebGPU world shader", WGSL: webGPUWorldShader})
	if err != nil {
		return fmt.Errorf("create world shader: %w", err)
	}
	r.sceneBuffer, err = r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft WebGPU world scene",
		Size:  webGPUWorldSceneBytes,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("create world scene buffer: %w", err)
	}
	r.atlasTexture, err = r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "GoCraft WebGPU block atlas",
		Size:  wgpu.Extent3D{Width: uint32(atlas.width), Height: uint32(atlas.height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("create block atlas texture: %w", err)
	}
	if err := r.queue.WriteTexture(
		&wgpu.ImageCopyTexture{Texture: r.atlasTexture, MipLevel: 0},
		atlas.pixels,
		&wgpu.ImageDataLayout{Offset: 0, BytesPerRow: uint32(atlas.width * 4), RowsPerImage: uint32(atlas.height)},
		&wgpu.Extent3D{Width: uint32(atlas.width), Height: uint32(atlas.height), DepthOrArrayLayers: 1},
	); err != nil {
		return fmt.Errorf("upload block atlas: %w", err)
	}
	r.atlasView, err = r.device.CreateTextureView(r.atlasTexture, nil)
	if err != nil {
		return fmt.Errorf("create block atlas view: %w", err)
	}
	r.atlasSampler, err = r.device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:          "GoCraft WebGPU block atlas sampler",
		AddressModeU:   gputypes.AddressModeClampToEdge,
		AddressModeV:   gputypes.AddressModeClampToEdge,
		MagFilter:      gputypes.FilterModeNearest,
		MinFilter:      gputypes.FilterModeNearest,
		MipmapFilter:   gputypes.FilterModeNearest,
	})
	if err != nil {
		return fmt.Errorf("create block atlas sampler: %w", err)
	}

	r.bindGroupLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft WebGPU world BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: webGPUWorldSceneBytes}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return fmt.Errorf("create world bind group layout: %w", err)
	}
	r.layout, err = r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "GoCraft WebGPU world layout", BindGroupLayouts: []*wgpu.BindGroupLayout{r.bindGroupLayout},
	})
	if err != nil {
		return fmt.Errorf("create world pipeline layout: %w", err)
	}
	r.bindGroup, err = r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "GoCraft WebGPU world bind group",
		Layout: r.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: r.sceneBuffer, Size: webGPUWorldSceneBytes},
			{Binding: 1, TextureView: r.atlasView},
			{Binding: 2, Sampler: r.atlasSampler},
		},
	})
	if err != nil {
		return fmt.Errorf("create world bind group: %w", err)
	}

	ignoreStencil := wgpu.StencilFaceState{
		Compare: gputypes.CompareFunctionAlways,
		FailOp: gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp: gputypes.StencilOperationKeep,
	}
	vertexState := wgpu.VertexState{
		Module: r.shader,
		EntryPoint: "vs_main",
		Buffers: []gputypes.VertexBufferLayout{{
			ArrayStride: uint64(unsafe.Sizeof(platform.Vertex{})),
			StepMode: gputypes.VertexStepModeVertex,
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

	r.opaquePipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft WebGPU opaque world pipeline",
		Layout: r.layout,
		Vertex: vertexState,
		Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUWorldDepthFormat, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionLess,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment: &wgpu.FragmentState{Module: r.shader, EntryPoint: "fs_opaque", Targets: []gputypes.ColorTargetState{{Format: r.format, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		return fmt.Errorf("create opaque world pipeline: %w", err)
	}

	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	r.translucentPipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft WebGPU translucent world pipeline",
		Layout: r.layout,
		Vertex: vertexState,
		Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUWorldDepthFormat, DepthWriteEnabled: false, DepthCompare: gputypes.CompareFunctionLess,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment: &wgpu.FragmentState{Module: r.shader, EntryPoint: "fs_translucent", Targets: []gputypes.ColorTargetState{{Format: r.format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		return fmt.Errorf("create translucent world pipeline: %w", err)
	}
	return nil
}

func (r *webGPUWorldRenderer) configureSurface() error {
	if err := r.surface.Configure(r.device, &wgpu.SurfaceConfiguration{
		Format: r.format,
		Usage: gputypes.TextureUsageRenderAttachment,
		Width: r.width,
		Height: r.height,
		AlphaMode: gputypes.CompositeAlphaModeOpaque,
		PresentMode: gputypes.PresentModeFifo,
	}); err != nil {
		return fmt.Errorf("configure WebGPU surface: %w", err)
	}
	if r.depthView != nil {
		r.depthView.Release()
		r.depthView = nil
	}
	if r.depthTexture != nil {
		r.depthTexture.Release()
		r.depthTexture = nil
	}
	var err error
	r.depthTexture, err = r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "GoCraft WebGPU world depth",
		Size: wgpu.Extent3D{Width: r.width, Height: r.height, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount: 1,
		Dimension: gputypes.TextureDimension2D,
		Format: webGPUWorldDepthFormat,
		Usage: gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		return fmt.Errorf("create WebGPU depth texture: %w", err)
	}
	r.depthView, err = r.device.CreateTextureView(r.depthTexture, nil)
	if err != nil {
		return fmt.Errorf("create WebGPU depth view: %w", err)
	}
	return nil
}

func (r *webGPUWorldRenderer) updateScene(camera rl.Camera3D) error {
	aspect := float32(r.width) / float32(max(uint32(1), r.height))
	projection := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, 1000.0)
	eye := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	view := mgl32.LookAtV(eye, mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z}, mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z})
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0.5, 0,
		0, 0, 0.5, 1,
	}
	vp := clipCorrection.Mul4(projection).Mul4(view)
	fogStart, fogEnd := worldFogRange(renderDistance())
	fogColor := [4]float32{180.0 / 255.0, 210.0 / 255.0, 1.0, 1.0}

	bytes := make([]byte, webGPUWorldSceneBytes)
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
	return r.queue.WriteBuffer(r.sceneBuffer, 0, bytes)
}

func (r *webGPUWorldRenderer) collectVisible(world *World, camera rl.Camera3D) {
	aspect := float32(r.width) / float32(max(uint32(1), r.height))
	projection := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, 1000.0)
	camPos := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	view := mgl32.LookAtV(camPos, mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z}, mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z})
	frustum := ExtractFrustum(projection.Mul4(view))

	cache := &world.render
	cache.drawCalls, cache.triangles = 0, 0
	cache.visible = cache.visible[:0]
	cx := int(math.Floor(float64(camera.Position.X) / float64(chunkWidth)))
	cz := int(math.Floor(float64(camera.Position.Z) / float64(chunkWidth)))
	for _, offset := range cache.radiusOffsets(renderDistance()) {
		chunkX, chunkZ := cx+offset.dx, cz+offset.dz
		chunk := world.getChunkIfGenerated(chunkX, chunkZ)
		if chunk == nil {
			continue
		}
		ensureChunkSections(chunk)
		cache.appendVisibleSections(chunk, chunkX, chunkZ, &frustum, camPos)
	}
	slices.SortFunc(cache.visible, compareVisibleSections)

	deadline := time.Now().Add(time.Millisecond)
	submissions := 0
	for _, section := range cache.visible {
		if submissions == 8 || !time.Now().Before(deadline) {
			break
		}
		if section.chunk.meshRetries[section.sec] <= 5 && world.submitMesh(section.cx, section.cz, section.sec, assets) {
			submissions++
		}
	}
	cache.collectTranslucent()
}

func (r *webGPUWorldRenderer) drawMeshMap(pass *wgpu.RenderPassEncoder, meshes map[string][]*ChunkMesh, cache *worldRenderCache) error {
	for path, list := range meshes {
		if path != "atlas" {
			continue // Special/animated material support is a later parity milestone.
		}
		for _, mesh := range list {
			if mesh == nil || mesh.glMesh == nil {
				continue
			}
			if err := r.backend.DrawPass(pass, mesh.glMesh); err != nil {
				return err
			}
			cache.drawCalls++
			cache.triangles += int(mesh.glMesh.IndexCount()) / 3
		}
	}
	return nil
}

func (r *webGPUWorldRenderer) Draw(world *World, camera rl.Camera3D) error {
	if world == nil {
		return nil
	}
	width := uint32(max(1, rl.GetScreenWidth()))
	height := uint32(max(1, rl.GetScreenHeight()))
	if width != r.width || height != r.height {
		r.width, r.height = width, height
		if err := r.configureSurface(); err != nil {
			return err
		}
	}
	if err := r.updateScene(camera); err != nil {
		return fmt.Errorf("update WebGPU world scene: %w", err)
	}
	r.collectVisible(world, camera)
	cache := &world.render
	defer func() {
		clear(cache.visible)
		cache.visible = cache.visible[:0]
		clear(cache.translucent)
		cache.translucent = cache.translucent[:0]
	}()

	surfaceTexture, _, err := r.surface.GetCurrentTexture()
	if err != nil {
		return err
	}
	if surfaceTexture == nil {
		return fmt.Errorf("surface returned no texture")
	}
	view, err := surfaceTexture.CreateView(nil)
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	defer view.Release()
	encoder, err := r.device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoCraft WebGPU world encoder"})
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoCraft WebGPU world pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View: view,
			LoadOp: gputypes.LoadOpClear,
			StoreOp: gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 180.0 / 255.0, G: 210.0 / 255.0, B: 1.0, A: 1.0},
		}},
		DepthStencilAttachment: &wgpu.RenderPassDepthStencilAttachment{
			View: r.depthView,
			DepthLoadOp: gputypes.LoadOpClear,
			DepthStoreOp: gputypes.StoreOpStore,
			DepthClearValue: 1,
			DepthReadOnly: false,
		},
	})
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}

	pass.SetPipeline(r.opaquePipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	for _, section := range cache.visible {
		if err := r.drawMeshMap(pass, section.chunk.opaqueMeshes[section.sec], cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
		if err := r.drawMeshMap(pass, section.chunk.cutoutMeshes[section.sec], cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
	}

	pass.SetPipeline(r.translucentPipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	for _, item := range cache.translucent {
		section := item.section
		meshes := section.chunk.waterMeshes[section.sec]
		if item.glass {
			meshes = section.chunk.glassMeshes[section.sec]
		}
		if err := r.drawMeshMap(pass, meshes, cache); err != nil {
			_ = pass.End()
			r.surface.DiscardTexture()
			return err
		}
	}
	if err := pass.End(); err != nil {
		r.surface.DiscardTexture()
		return err
	}
	commandBuffer, err := encoder.Finish()
	if err != nil {
		r.surface.DiscardTexture()
		return err
	}
	defer commandBuffer.Release()
	if _, err := r.queue.Submit(commandBuffer); err != nil {
		r.surface.DiscardTexture()
		return err
	}
	return r.surface.Present(surfaceTexture)
}

func (r *webGPUWorldRenderer) Close() {
	r.closeResources(true)
}

func (r *webGPUWorldRenderer) closeResources(resetBackend bool) {
	if resetBackend {
		platform.SetMeshBackend(platform.OpenGLMeshBackend{})
	}
	if r.depthView != nil {
		r.depthView.Release()
		r.depthView = nil
	}
	if r.depthTexture != nil {
		r.depthTexture.Release()
		r.depthTexture = nil
	}
	if r.translucentPipeline != nil {
		r.translucentPipeline.Release()
		r.translucentPipeline = nil
	}
	if r.opaquePipeline != nil {
		r.opaquePipeline.Release()
		r.opaquePipeline = nil
	}
	if r.bindGroup != nil {
		r.bindGroup.Release()
		r.bindGroup = nil
	}
	if r.layout != nil {
		r.layout.Release()
		r.layout = nil
	}
	if r.bindGroupLayout != nil {
		r.bindGroupLayout.Release()
		r.bindGroupLayout = nil
	}
	if r.atlasSampler != nil {
		r.atlasSampler.Release()
		r.atlasSampler = nil
	}
	if r.atlasView != nil {
		r.atlasView.Release()
		r.atlasView = nil
	}
	if r.atlasTexture != nil {
		r.atlasTexture.Release()
		r.atlasTexture = nil
	}
	if r.sceneBuffer != nil {
		r.sceneBuffer.Release()
		r.sceneBuffer = nil
	}
	if r.shader != nil {
		r.shader.Release()
		r.shader = nil
	}
	if r.device != nil {
		r.device.Release()
		r.device = nil
	}
	if r.adapter != nil {
		r.adapter.Release()
		r.adapter = nil
	}
	if r.surface != nil {
		r.surface.Release()
		r.surface = nil
	}
	if r.instance != nil {
		r.instance.Release()
		r.instance = nil
	}
}
