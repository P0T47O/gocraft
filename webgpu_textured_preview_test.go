//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"math"
	"os"
	"runtime"
	"sort"
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

const webGPUTexturedPreviewShader = `
struct Camera {
    view_proj: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;
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
    @location(0) uv: vec2f,
    @location(1) color: vec4f,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = camera.view_proj * vec4f(in.position, 1.0);
    out.uv = in.uv;
    out.color = in.color;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4f {
    let texel = textureSample(atlas_tex, atlas_sampler, in.uv);
    let alpha = texel.a * in.color.a;
    if alpha < 0.5 {
        discard;
    }
    return vec4f(texel.rgb * in.color.rgb, alpha);
}
`

// TestWebGPUTexturedPreview is the first milestone that samples GoCraft's real
// block textures in WebGPU. A 5x5 generated halo supplies neighbor data while
// only the inner 3x3 chunks are meshed and rendered. All static block textures
// are packed into a CPU-side atlas whose UV map is handed to the existing chunk
// mesher, so this exercises the same UV decisions used by the OpenGL renderer.
func TestWebGPUTexturedPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_TEXTURE_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_TEXTURE_PREVIEW=1 to run the textured WebGPU terrain preview")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initBlockRegistry()
	const seed uint32 = 12345

	atlasPixels, atlasWidth, atlasHeight, atlasUVs, err := buildWebGPUPreviewAtlas()
	if err != nil {
		t.Fatalf("build preview atlas: %v", err)
	}
	assets := &RenderAssets{atlas: &TextureAtlas{UVs: atlasUVs}}

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

	// Texture validation is easiest from an elevated world-like view. Keep a
	// shallower subsurface slice than the geometry probes so cutaway cave walls
	// cannot dominate the frame.
	yMin := max(0, maxTop-24)
	yMax := min(chunkHeight, maxTop+8)

	type cpuMesh struct {
		cx, cz  int
		results map[string]map[string][]*MeshBuildData
	}
	cpuMeshes := make([]cpuMesh, 0, 9)
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
			cpuMeshes = append(cpuMeshes, cpuMesh{cx: cx, cz: cz, results: results})
		}
	}
	defer func() {
		for _, item := range cpuMeshes {
			releaseMeshResults(item.results)
		}
	}()

	rl.SetTraceLogLevel(rl.LogWarning)
	rl.InitWindow(1280, 800, "GoCraft textured terrain - WebGPU preview")
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

	pipeline, bindGroup, cameraBuffer, atlasTexture, atlasView, atlasSampler, cleanupPipeline, err := createWebGPUTexturedPreviewPipeline(
		device, queue, format, atlasPixels, atlasWidth, atlasHeight,
	)
	if err != nil {
		t.Fatalf("create textured preview pipeline: %v", err)
	}
	defer cleanupPipeline()
	_ = atlasTexture
	_ = atlasView
	_ = atlasSampler

	backend := platform.NewWebGPUMeshBackend(device)
	meshes := make([]platform.MeshHandle, 0, 64)
	verticesTotal, indicesTotal, skippedNonAtlas := 0, 0, 0
	for _, item := range cpuMeshes {
		for _, passName := range []string{"opaque", "cutout"} {
			pass := item.results[passName]
			for path, list := range pass {
				if path != "atlas" {
					for _, data := range list {
						if data != nil && data.vertCount > 0 {
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
					mesh, err := backend.UploadChecked(vertices, data.indices)
					if err != nil {
						t.Fatalf("upload textured chunk (%d,%d) %s mesh: %v", item.cx, item.cz, passName, err)
					}
					meshes = append(meshes, mesh)
					verticesTotal += len(vertices)
					indicesTotal += len(data.indices)
				}
			}
		}
	}
	defer func() {
		for _, mesh := range meshes {
			mesh.Unload()
		}
	}()
	if len(meshes) == 0 {
		t.Fatal("textured preview produced no atlas-backed meshes")
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
		t.Fatalf("configure textured preview: %v", err)
	}
	defer func() {
		if depthView != nil {
			depthView.Release()
		}
		if depthTexture != nil {
			depthTexture.Release()
		}
	}()

	t.Logf("textured terrain ready: atlas=%dx%d textures=%d yRange=[%d,%d) meshes=%d vertices=%d indices=%d triangles=%d skippedNonAtlasVertices=%d",
		atlasWidth, atlasHeight, len(atlasUVs), yMin, yMax, len(meshes), verticesTotal, indicesTotal, indicesTotal/3, skippedNonAtlas)
	t.Log("Expected result: real GoCraft grass/dirt/stone/tree/plant textures sampled through a WebGPU atlas; cutout texels are discarded and existing vertex AO/tint modulates the texture color.")

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
				t.Fatalf("resize textured preview: %v", err)
			}
		}
		angle := float32(time.Since(started).Seconds()) * 0.045
		if err := updateWebGPUTexturedPreviewCamera(queue, cameraBuffer, width, height, maxTop, angle); err != nil {
			t.Fatalf("update textured camera: %v", err)
		}
		if err := drawWebGPUChunkPreviewFrame(device, queue, surface, pipeline, bindGroup, depthView, backend, meshes); err != nil {
			t.Fatalf("draw textured preview: %v", err)
		}
		time.Sleep(time.Second / 120)
	}
}

func buildWebGPUPreviewAtlas() ([]byte, int, int, map[string]AtlasRect, error) {
	pathsSet := make(map[string]struct{})
	for _, def := range Blocks {
		if def == nil {
			continue
		}
		faces := def.Textures
		for _, path := range []string{faces.Top, faces.Bottom, faces.North, faces.South, faces.East, faces.West} {
			if path != "" {
				pathsSet[path] = struct{}{}
			}
		}
	}
	// Grass side overlay is emitted directly by the chunk mesher rather than
	// living in BlockDef.Textures.
	pathsSet["textures/block/grass_block_side_overlay.png"] = struct{}{}

	paths := make([]string, 0, len(pathsSet))
	for path := range pathsSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, 0, 0, nil, fmt.Errorf("block registry contains no texture paths")
	}

	side := 1
	for side*side < len(paths) {
		side++
	}
	width := side * atlasCellSize
	height := side * atlasCellSize
	pixels := make([]byte, width*height*4)
	uvs := make(map[string]AtlasRect, len(paths))

	for i, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, 0, 0, nil, fmt.Errorf("open %s: %w", path, err)
		}
		img, _, decodeErr := image.Decode(file)
		_ = file.Close()
		if decodeErr != nil {
			return nil, 0, 0, nil, fmt.Errorf("decode %s: %w", path, decodeErr)
		}

		tile := image.NewNRGBA(image.Rect(0, 0, atlasTileSize, atlasTileSize))
		bounds := img.Bounds()
		if path == "textures/block/torch.png" && bounds.Dx() < atlasTileSize {
			dx := (atlasTileSize - min(atlasTileSize, bounds.Dx())) / 2
			dst := image.Rect(dx, 0, dx+min(atlasTileSize, bounds.Dx()), min(atlasTileSize, bounds.Dy()))
			draw.Draw(tile, dst, img, bounds.Min, draw.Src)
		} else {
			dst := image.Rect(0, 0, min(atlasTileSize, bounds.Dx()), min(atlasTileSize, bounds.Dy()))
			draw.Draw(tile, dst, img, bounds.Min, draw.Src)
		}

		col := i % side
		row := i / side
		for py := 0; py < atlasCellSize; py++ {
			sy := atlasSourceCoordinate(py)
			for px := 0; px < atlasCellSize; px++ {
				sx := atlasSourceCoordinate(px)
				c := tile.NRGBAAt(sx, sy)
				dstX := col*atlasCellSize + px
				dstY := row*atlasCellSize + py
				offset := (dstY*width + dstX) * 4
				pixels[offset+0] = c.R
				pixels[offset+1] = c.G
				pixels[offset+2] = c.B
				pixels[offset+3] = c.A
			}
		}

		uvs[path] = AtlasRect{
			X:      float32(col*atlasCellSize+atlasPadding) / float32(width),
			Y:      float32(row*atlasCellSize+atlasPadding) / float32(height),
			Width:  float32(atlasTileSize) / float32(width),
			Height: float32(atlasTileSize) / float32(height),
		}
	}
	return pixels, width, height, uvs, nil
}

func createWebGPUTexturedPreviewPipeline(
	device *wgpu.Device,
	queue *wgpu.Queue,
	format gputypes.TextureFormat,
	atlasPixels []byte,
	atlasWidth, atlasHeight int,
) (*wgpu.RenderPipeline, *wgpu.BindGroup, *wgpu.Buffer, *wgpu.Texture, *wgpu.TextureView, *wgpu.Sampler, func(), error) {
	shader, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft textured preview shader", WGSL: webGPUTexturedPreviewShader})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	cameraBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft textured preview camera", Size: webGPUChunkPreviewCameraBytes,
		Usage: gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label: "GoCraft WebGPU block atlas",
		Size: wgpu.Extent3D{Width: uint32(atlasWidth), Height: uint32(atlasHeight), DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: gputypes.TextureDimension2D,
		Format: gputypes.TextureFormatRGBA8Unorm,
		Usage: gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	if err := queue.WriteTexture(
		&wgpu.ImageCopyTexture{Texture: texture, MipLevel: 0},
		atlasPixels,
		&wgpu.ImageDataLayout{Offset: 0, BytesPerRow: uint32(atlasWidth * 4), RowsPerImage: uint32(atlasHeight)},
		&wgpu.Extent3D{Width: uint32(atlasWidth), Height: uint32(atlasHeight), DepthOrArrayLayers: 1},
	); err != nil {
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	view, err := device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	sampler, err := device.CreateSampler(&wgpu.SamplerDescriptor{
		Label: "GoCraft block atlas nearest sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		MagFilter: gputypes.FilterModeNearest,
		MinFilter: gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
	})
	if err != nil {
		view.Release()
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	bgl, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft textured preview BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageVertex, Buffer: &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform, MinBindingSize: webGPUChunkPreviewCameraBytes}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		sampler.Release()
		view.Release()
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{Label: "GoCraft textured preview layout", BindGroupLayouts: []*wgpu.BindGroupLayout{bgl}})
	if err != nil {
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	bindGroup, err := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label: "GoCraft textured preview BG", Layout: bgl,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: cameraBuffer, Size: webGPUChunkPreviewCameraBytes},
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
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	ignoreStencil := wgpu.StencilFaceState{
		Compare: gputypes.CompareFunctionAlways,
		FailOp: gputypes.StencilOperationKeep,
		DepthFailOp: gputypes.StencilOperationKeep,
		PassOp: gputypes.StencilOperationKeep,
	}
	stride := uint64(unsafe.Sizeof(platform.Vertex{}))
	pipeline, err := device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft textured terrain pipeline", Layout: layout,
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
			StencilFront: ignoreStencil, StencilBack: ignoreStencil,
			StencilReadMask: 0, StencilWriteMask: 0,
		},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{
			Module: shader, EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{Format: format, WriteMask: gputypes.ColorWriteMaskAll}},
		},
	})
	if err != nil {
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
		return nil, nil, nil, nil, nil, nil, nil, err
	}

	cleanup := func() {
		pipeline.Release()
		bindGroup.Release()
		layout.Release()
		bgl.Release()
		sampler.Release()
		view.Release()
		texture.Release()
		cameraBuffer.Release()
		shader.Release()
	}
	return pipeline, bindGroup, cameraBuffer, texture, view, sampler, cleanup, nil
}

func updateWebGPUTexturedPreviewCamera(queue *wgpu.Queue, buffer *wgpu.Buffer, width, height uint32, maxTop int, angle float32) error {
	aspect := float32(width) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(50), aspect, 0.1, 1000.0)
	center := mgl32.Vec3{float32(chunkWidth) * 0.5, float32(maxTop) - 5, float32(chunkWidth) * 0.5}
	radius := float32(chunkWidth) * 5.2
	eye := mgl32.Vec3{
		center.X() + float32(math.Cos(float64(angle)))*radius,
		float32(maxTop) + float32(chunkWidth)*5.0,
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
	if err := queue.WriteBuffer(buffer, 0, bytes); err != nil {
		return fmt.Errorf("write textured camera uniform: %w", err)
	}
	return nil
}
