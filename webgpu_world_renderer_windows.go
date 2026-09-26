//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"gocraft/platform"
)

const webGPUWorldSceneBytes = 128
const webGPUWorldDepthFormat = gputypes.TextureFormatDepth32Float

const webGPUWorldShader = `
struct Scene {
    view_proj: mat4x4<f32>,
    eye_pos: vec4f,
    fog_range: vec4f,
    fog_color: vec4f,
    daylight: vec4f,
}

@group(0) @binding(0) var<uniform> scene: Scene;
@group(0) @binding(1) var atlas_tex: texture_2d<f32>;
@group(0) @binding(2) var atlas_sampler: sampler;

struct VertexInput {
    @location(0) position: vec3f,
    @location(1) uv: vec2f,
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
    let delta = world_pos - scene.eye_pos.xyz;
    // Streaming is horizontal: altitude must not shorten terrain visibility.
    let boundary = smoothstep(scene.fog_range.x, scene.fog_range.y, length(delta.xz));
    // zw is an independent spherical environmental fog range. Equal endpoints
    // disable it for clear weather, without evaluating undefined smoothstep.
    var environment = 0.0;
    if scene.fog_range.w > scene.fog_range.z {
        environment = smoothstep(scene.fog_range.z, scene.fog_range.w, length(delta));
    }
    let amount = max(boundary, environment);
    return mix(rgb, scene.fog_color.rgb, amount);
}

fn sample_atlas(uv: vec2f) -> vec4f {
    // Anisotropy requires a linear sampler, but magnified pixel art should
    // remain nearest-neighbor. Evaluate derivatives before any varying branch.
    let size = vec2f(textureDimensions(atlas_tex, 0));
    let footprint = max(length(dpdx(uv) * size), length(dpdy(uv) * size));
    // The 16px tile atlas loses its last recognizable detail too early with
    // the hardware's neutral mip choice. Keep half a mip level of detail;
    // anisotropic filtering still handles grazing-angle footprints.
    let filtered = textureSampleBias(atlas_tex, atlas_sampler, uv, -0.5);
    if footprint <= 1.0 {
        let pixel = clamp(vec2i(floor(uv * size)), vec2i(0), vec2i(size) - vec2i(1));
        return textureLoad(atlas_tex, pixel, 0);
    }
    return filtered;
}

@fragment
fn fs_opaque(in: VertexOutput) -> @location(0) vec4f {
    let texel = sample_atlas(in.uv);
    // Opaque/cutout vertex alpha carries retained block-light brightness.
    // Texture alpha alone controls cutout coverage.
    let alpha = texel.a;
    if alpha < 0.5 {
        discard;
    }
    let rgb = fogged(texel.rgb * in.color.rgb * max(scene.daylight.x, in.color.a), in.world_pos);
    return vec4f(rgb, 1.0);
}

@fragment
fn fs_translucent(in: VertexOutput) -> @location(0) vec4f {
    let texel = sample_atlas(in.uv);
    let alpha = texel.a * in.color.a;
    if alpha < 0.01 {
        discard;
    }
    let rgb = fogged(texel.rgb * in.color.rgb * scene.daylight.x, in.world_pos);
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

	format                  gputypes.TextureFormat
	opaquePipeline          *wgpu.RenderPipeline
	solidPipeline           *wgpu.RenderPipeline
	translucentPipeline     *wgpu.RenderPipeline
	bindGroup               *wgpu.BindGroup
	sceneBuffer             *wgpu.Buffer
	shader                  *wgpu.ShaderModule
	layout                  *wgpu.PipelineLayout
	bindGroupLayout         *wgpu.BindGroupLayout
	atlasTexture            *wgpu.Texture
	atlasView               *wgpu.TextureView
	atlasBaseView           *wgpu.TextureView
	atlasSampler            *wgpu.Sampler
	filterMipmaps           bool
	sceneSky                [3]float32
	filterAF                int
	depthTexture            *wgpu.Texture
	depthView               *wgpu.TextureView
	msaaTexture             *wgpu.Texture
	msaaView                *wgpu.TextureView
	worldSamples            uint32
	lod                     *webGPULODRenderer
	width, height           uint32
	surfaceNeedsReconfigure bool
	camera                  webGPUCamera
}

func newWebGPUWorldRenderer(hwnd uintptr, assets *RenderAssets, width, height uint32, samples ...uint32) (*webGPUWorldRenderer, error) {
	if hwnd == 0 {
		return nil, fmt.Errorf("native window handle is null")
	}
	atlas, err := buildWebGPUBlockAtlas()
	if err != nil {
		return nil, fmt.Errorf("build block atlas: %w", err)
	}

	r := &webGPUWorldRenderer{width: max(uint32(1), width), height: max(uint32(1), height), worldSamples: 1}
	if len(samples) > 0 && samples[0] == 4 {
		r.worldSamples = 4
	}
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

	if err := r.configureSurface(); err != nil {
		return fail(err)
	}

	r.backend = platform.EnableExperimentalWebGPU(r.device)
	info := r.adapter.Info()
	fmt.Printf("Experimental WebGPU renderer: %s, backend=%v, deviceType=%v, atlas=%dx%d\n", info.Name, info.Backend, info.DeviceType, atlas.width, atlas.height)
	return r, nil
}

func (r *webGPUWorldRenderer) createPipelineResources(atlas *webGPUBlockAtlas) error {
	if r.worldSamples == 0 {
		r.worldSamples = 1 // GPU-only fixtures construct the renderer without a surface.
	}
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
		Label:         "GoCraft WebGPU block atlas",
		Size:          wgpu.Extent3D{Width: uint32(atlas.width), Height: uint32(atlas.height), DepthOrArrayLayers: 1},
		MipLevelCount: atlasMaxMipLevel + 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("create block atlas texture: %w", err)
	}
	for level, mip := range buildWebGPUMips(atlas.pixels, atlas.width, atlas.height, atlasMaxMipLevel+1) {
		if err := r.queue.WriteTexture(
			&wgpu.ImageCopyTexture{Texture: r.atlasTexture, MipLevel: uint32(level)}, mip.pixels,
			&wgpu.ImageDataLayout{BytesPerRow: uint32(mip.width * 4), RowsPerImage: uint32(mip.height)},
			&wgpu.Extent3D{Width: uint32(mip.width), Height: uint32(mip.height), DepthOrArrayLayers: 1},
		); err != nil {
			return fmt.Errorf("upload block atlas mip %d: %w", level, err)
		}
	}
	r.atlasView, err = r.device.CreateTextureView(r.atlasTexture, nil)
	if err != nil {
		return fmt.Errorf("create block atlas view: %w", err)
	}
	r.atlasBaseView, err = r.device.CreateTextureView(r.atlasTexture, &wgpu.TextureViewDescriptor{Label: "GoCraft atlas mip zero", MipLevelCount: 1, ArrayLayerCount: 1})
	if err != nil {
		return fmt.Errorf("create block atlas base view: %w", err)
	}
	r.filterMipmaps, r.filterAF = webGPUFilterSettings()
	r.atlasSampler, err = r.device.CreateSampler(webGPUAtlasSamplerDescriptor(r.filterMipmaps, r.filterAF))
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
			{Binding: 1, TextureView: r.filteredAtlasView(r.filterMipmaps)},
			{Binding: 2, Sampler: r.atlasSampler},
		},
	})
	if err != nil {
		return fmt.Errorf("create world bind group: %w", err)
	}

	ignoreStencil := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp:      gputypes.StencilOperationKeep,
	}
	vertexState := wgpu.VertexState{
		Module:     r.shader,
		EntryPoint: "vs_main",
		Buffers: []gputypes.VertexBufferLayout{{
			ArrayStride: uint64(unsafe.Sizeof(platform.CompactVertex{})),
			StepMode:    gputypes.VertexStepModeVertex,
			Attributes: []gputypes.VertexAttribute{
				{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
				{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
				{Format: gputypes.VertexFormatUnorm8x4, Offset: 20, ShaderLocation: 3},
			},
		}},
	}
	primitive := gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone}
	multisample := gputypes.MultisampleState{Count: r.worldSamples, Mask: 0xFFFFFFFF}

	opaqueDescriptor := &wgpu.RenderPipelineDescriptor{
		Label:     "GoCraft WebGPU opaque world pipeline",
		Layout:    r.layout,
		Vertex:    vertexState,
		Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUWorldDepthFormat, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionGreater,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment:    &wgpu.FragmentState{Module: r.shader, EntryPoint: "fs_opaque", Targets: []gputypes.ColorTargetState{{Format: r.format, WriteMask: gputypes.ColorWriteMaskAll}}},
	}
	r.opaquePipeline, err = r.device.CreateRenderPipeline(opaqueDescriptor)
	if err != nil {
		return fmt.Errorf("create opaque world pipeline: %w", err)
	}

	opaqueDescriptor.Primitive.CullMode = gputypes.CullModeBack
	opaqueDescriptor.Label = "GoCraft solid backface-culling pipeline"
	r.solidPipeline, err = r.device.CreateRenderPipeline(opaqueDescriptor)
	if err != nil {
		return fmt.Errorf("create solid pipeline: %w", err)
	}
	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	r.translucentPipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:     "GoCraft WebGPU translucent world pipeline",
		Layout:    r.layout,
		Vertex:    vertexState,
		Primitive: primitive,
		DepthStencil: &wgpu.DepthStencilState{
			Format: webGPUWorldDepthFormat, DepthWriteEnabled: false, DepthCompare: gputypes.CompareFunctionGreater,
			StencilFront: ignoreStencil, StencilBack: ignoreStencil, StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: multisample,
		Fragment:    &wgpu.FragmentState{Module: r.shader, EntryPoint: "fs_translucent", Targets: []gputypes.ColorTargetState{{Format: r.format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		return fmt.Errorf("create translucent world pipeline: %w", err)
	}
	return nil
}

func (r *webGPUWorldRenderer) configureSurface() error {
	if err := r.surface.Configure(r.device, &wgpu.SurfaceConfiguration{
		Format:      r.format,
		Usage:       gputypes.TextureUsageRenderAttachment,
		Width:       r.width,
		Height:      r.height,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
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
	if r.msaaView != nil {
		r.msaaView.Release()
		r.msaaView = nil
	}
	if r.msaaTexture != nil {
		r.msaaTexture.Release()
		r.msaaTexture = nil
	}
	var err error
	if r.worldSamples > 1 {
		r.msaaTexture, err = r.device.CreateTexture(&wgpu.TextureDescriptor{
			Label: "GoCraft multisampled world color", Size: wgpu.Extent3D{Width: r.width, Height: r.height, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: r.worldSamples, Dimension: gputypes.TextureDimension2D,
			Format: r.format, Usage: gputypes.TextureUsageRenderAttachment,
		})
		if err != nil {
			return fmt.Errorf("create WebGPU multisampled color: %w", err)
		}
		r.msaaView, err = r.device.CreateTextureView(r.msaaTexture, nil)
		if err != nil {
			return fmt.Errorf("create WebGPU multisampled color view: %w", err)
		}
	}
	r.depthTexture, err = r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "GoCraft WebGPU world depth",
		Size:          wgpu.Extent3D{Width: r.width, Height: r.height, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   r.worldSamples,
		Dimension:     gputypes.TextureDimension2D,
		Format:        webGPUWorldDepthFormat,
		Usage:         gputypes.TextureUsageRenderAttachment,
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

func (r *webGPUWorldRenderer) drawMeshMap(pass *wgpu.RenderPassEncoder, meshes map[string][]*ChunkMesh, cache *worldRenderCache) error {
	for path, list := range meshes {
		if path != "atlas" {
			continue // Special/animated material support is a later parity milestone.
		}
		for _, mesh := range list {
			if mesh == nil || mesh.gpuMesh == nil {
				continue
			}
			if err := r.backend.DrawPass(pass, mesh.gpuMesh); err != nil {
				return err
			}
			cache.drawCalls++
			cache.triangles += int(mesh.gpuMesh.IndexCount()) / 3
		}
	}
	return nil
}

func (r *webGPUWorldRenderer) Close() {
	r.closeResources(true)
}

func (r *webGPUWorldRenderer) closeResources(resetBackend bool) {
	if r.lod != nil {
		r.lod.close()
		r.lod = nil
	}
	if r.solidPipeline != nil {
		r.solidPipeline.Release()
		r.solidPipeline = nil
	}
	if resetBackend {
		platform.SetMeshBackend(nil)
	}
	if r.depthView != nil {
		r.depthView.Release()
		r.depthView = nil
	}
	if r.depthTexture != nil {
		r.depthTexture.Release()
		r.depthTexture = nil
	}
	if r.msaaView != nil {
		r.msaaView.Release()
		r.msaaView = nil
	}
	if r.msaaTexture != nil {
		r.msaaTexture.Release()
		r.msaaTexture = nil
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
	if r.atlasBaseView != nil {
		r.atlasBaseView.Release()
		r.atlasBaseView = nil
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
