//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"gocraft/platform"
)

const webGPULODChunkMaskWGSL = `
struct Scene {
    view_proj: mat4x4<f32>, eye_pos: vec4f, fog_range: vec4f,
    fog_color: vec4f, daylight: vec4f,
    mask_origin: vec4i, mask_words: array<vec4u, 521>,
}
@group(0) @binding(0) var<uniform> scene: Scene;
fn lod_chunk_covered(pos: vec3f) -> bool {
    // Full voxels are centered on integer coordinates. Their outer edge is
    // at chunk*16 - 0.5, not at chunk*16; use that edge for the LOD cutout.
    let chunk = vec2i(floor((pos.xz + vec2f(0.5)) / 16.0)) - scene.mask_origin.xy;
    if chunk.x < 0 || chunk.y < 0 || chunk.x >= 258 || chunk.y >= 258 { return false; }
    let bit = u32(chunk.y * 258 + chunk.x);
    let word = scene.mask_words[bit / 128u][(bit / 32u) % 4u];
    return ((word >> (bit % 32u)) & 1u) != 0u;
}
`

const webGPULODShader = webGPULODChunkMaskWGSL + `
struct Input { @location(0) position: vec3f, @location(1) detail: vec2f, @location(3) color: vec4f, }
struct Output {
    @builtin(position) position: vec4f,
    @location(0) world_pos: vec3f,
    @location(1) color: vec4f,
    @location(2) boundary_axis: f32,
}
@vertex fn vs_main(input: Input) -> Output {
    var out: Output;
    var pos = input.position;
    if input.detail.y > 0.0 && input.detail.y < 16.0 {
        // Morph against the same tile-center distance used by the CPU level
        // selector. At the switch, both meshes describe the same terrain.
        let center = (floor(pos.xz / 128.0) + vec2f(0.5)) * 128.0;
        let distance = length(center - scene.eye_pos.xz);
        let full_radius = scene.daylight.y + 32.0;
        var boundary = full_radius + 512.0;
        if input.detail.y < 3.0 { boundary = full_radius + 64.0; }
        else if input.detail.y < 6.0 { boundary = full_radius + 192.0; }
        let width = select(112.0, 48.0, input.detail.y < 3.0);
        let blend = smoothstep(boundary - width, boundary - 16.0, distance);
        pos.y = mix(pos.y, input.detail.x, blend);
    }
    out.position = scene.view_proj * vec4f(pos, 1.0);
    out.world_pos = pos;
    out.color = input.color;
    out.boundary_axis = input.detail.y;
    return out;
}
@fragment fn fs_main(input: Output) -> @location(0) vec4f {
    let horizontal = length(input.world_pos.xz - scene.eye_pos.xz);
    let tree = input.color.a > 0.994 && input.color.a < 0.999;
    let structure = input.color.a < 0.994;
    // Replace the first outside LOD cell with a strip whose inner edge uses
    // the last real voxel column. The strip slopes into the ordinary LOD mesh.
    if input.boundary_axis < 0.0 {
        var inward = vec3f(16.0, 0.0, 0.0);
        if input.boundary_axis < -3.5 { inward = vec3f(0.0, 0.0, -16.0); }
        else if input.boundary_axis < -2.5 { inward = vec3f(-16.0, 0.0, 0.0); }
        else if input.boundary_axis < -1.5 { inward = vec3f(0.0, 0.0, 16.0); }
        let inside = lod_chunk_covered(input.world_pos);
        if inside {
            // The strip extends 0.35 block beneath the last real voxel.
            // Keep this tiny overlap only where its outer neighbor is LOD.
            if lod_chunk_covered(input.world_pos + inward) { discard; }
        } else if !lod_chunk_covered(input.world_pos - inward) { discard; }
    }
    // Keep the ordinary LOD ground beneath transition strips. Cutting that
    // mesh away made subpixel holes where the two triangulations met.
    // A real chunk owns its entire column, including excavations and caves.
    if input.boundary_axis >= 0.0 && lod_chunk_covered(input.world_pos) { discard; }
    // The chunk mask already chooses between real and simplified trees.
    // A second distance fade left a visibly bare ring between the forests.
    var color = input.color.rgb;
    let water = color.b > color.r * 1.5 && color.b > color.g * 1.25;
    if !tree && !structure && !water {
        // A little stable block-scale variation makes the near LOD less like
        // a uniformly painted sheet. It fades out before aliasing at range.
        let block = floor(input.world_pos.xz);
        let grain = fract(sin(dot(block, vec2f(127.1, 311.7))) * 43758.5453);
        let amount = 0.035 * (1.0 - smoothstep(scene.daylight.y + 32.0, scene.daylight.y + 512.0, horizontal));
        color *= 1.0 + amount * (grain * 2.0 - 1.0);
    }
    let fog = smoothstep(scene.fog_range.x, scene.fog_range.y, horizontal);
    return vec4f(mix(color * scene.daylight.x, scene.fog_color.rgb, fog), 1.0);
}
`

const webGPULODWaterShader = webGPULODChunkMaskWGSL + `
@group(0) @binding(1) var atlas_tex: texture_2d<f32>;
@group(0) @binding(2) var atlas_sampler: sampler;
struct Input { @location(0) position: vec3f, @location(1) uv: vec2f, @location(3) color: vec4f, }
struct Output {
    @builtin(position) position: vec4f,
    @location(0) world_pos: vec3f,
    @location(1) uv: vec2f,
    @location(2) color: vec4f,
}
@vertex fn vs_main(input: Input) -> Output {
    var out: Output;
    out.position = scene.view_proj * vec4f(input.position, 1.0);
    out.world_pos = input.position;
    out.uv = input.uv;
    out.color = input.color;
    return out;
}
@fragment fn fs_main(input: Output) -> @location(0) vec4f {
    if lod_chunk_covered(input.world_pos) { discard; }
    let horizontal = length(input.world_pos.xz - scene.eye_pos.xz);
    let texel = textureSampleBias(atlas_tex, atlas_sampler, input.uv, -0.5);
    let alpha = texel.a * input.color.a;
    if alpha < 0.01 { discard; }
    let fog = smoothstep(scene.fog_range.x, scene.fog_range.y, horizontal);
    let rgb = mix(texel.rgb * input.color.rgb * scene.daylight.x, scene.fog_color.rgb, fog);
    return vec4f(rgb, alpha);
}
`

type lodBuildResult struct {
	key     lodTileKey
	version uint64
	step    int
	data    lodTileData
}

type lodBuildJob struct {
	key       lodTileKey
	version   uint64
	step      int
	overrides map[lodPoint]lodColumn
}

type lodBuildState struct {
	version uint64
	step    int
}

type webGPULODRenderer struct {
	seed          uint32
	shader        *wgpu.ShaderModule
	pipeline      *wgpu.RenderPipeline
	waterShader   *wgpu.ShaderModule
	waterPipeline *wgpu.RenderPipeline
	tiles         map[lodTileKey]platform.MeshHandle
	waterTiles    map[lodTileKey]platform.MeshHandle
	versions      map[lodTileKey]uint64
	steps         map[lodTileKey]int
	pending       map[lodTileKey]lodBuildState
	covered       map[lodTileKey]bool
	jobs          chan lodBuildJob
	results       chan lodBuildResult
	stop          chan struct{}
	workers       sync.WaitGroup
	offsets       []lodTileKey
	radius        int
}

func newWebGPULODRenderer(r *webGPUWorldRenderer, seed uint32) (*webGPULODRenderer, error) {
	lod := &webGPULODRenderer{
		seed: seed, tiles: make(map[lodTileKey]platform.MeshHandle), waterTiles: make(map[lodTileKey]platform.MeshHandle), versions: make(map[lodTileKey]uint64), steps: make(map[lodTileKey]int), pending: make(map[lodTileKey]lodBuildState),
		jobs: make(chan lodBuildJob, 32), results: make(chan lodBuildResult, 8), stop: make(chan struct{}),
	}
	var err error
	lod.shader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft distant terrain", WGSL: webGPULODShader})
	if err != nil {
		return nil, fmt.Errorf("create LOD shader: %w", err)
	}
	lod.pipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft distant terrain", Layout: r.layout,
		Vertex: wgpu.VertexState{Module: lod.shader, EntryPoint: "vs_main", Buffers: []gputypes.VertexBufferLayout{{
			ArrayStride: uint64(unsafe.Sizeof(platform.CompactVertex{})), StepMode: gputypes.VertexStepModeVertex,
			Attributes: []gputypes.VertexAttribute{
				{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
				{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
				{Format: gputypes.VertexFormatUnorm8x4, Offset: 20, ShaderLocation: 3},
			},
		}}},
		Primitive:    gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeBack},
		DepthStencil: &wgpu.DepthStencilState{Format: webGPUWorldDepthFormat, DepthWriteEnabled: true, DepthCompare: gputypes.CompareFunctionGreater},
		Multisample:  gputypes.MultisampleState{Count: r.worldSamples, Mask: 0xFFFFFFFF},
		Fragment:     &wgpu.FragmentState{Module: lod.shader, EntryPoint: "fs_main", Targets: []gputypes.ColorTargetState{{Format: r.format, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		lod.shader.Release()
		return nil, fmt.Errorf("create LOD pipeline: %w", err)
	}
	lod.waterShader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft distant water", WGSL: webGPULODWaterShader})
	if err != nil {
		lod.close()
		return nil, fmt.Errorf("create LOD water shader: %w", err)
	}
	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	lod.waterPipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label: "GoCraft distant water", Layout: r.layout,
		Vertex: wgpu.VertexState{Module: lod.waterShader, EntryPoint: "vs_main", Buffers: []gputypes.VertexBufferLayout{{
			ArrayStride: uint64(unsafe.Sizeof(platform.CompactVertex{})), StepMode: gputypes.VertexStepModeVertex,
			Attributes: []gputypes.VertexAttribute{
				{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
				{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
				{Format: gputypes.VertexFormatUnorm8x4, Offset: 20, ShaderLocation: 3},
			},
		}}},
		Primitive:    gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeBack},
		DepthStencil: &wgpu.DepthStencilState{Format: webGPUWorldDepthFormat, DepthWriteEnabled: false, DepthCompare: gputypes.CompareFunctionGreater},
		Multisample:  gputypes.MultisampleState{Count: r.worldSamples, Mask: 0xFFFFFFFF},
		Fragment:     &wgpu.FragmentState{Module: lod.waterShader, EntryPoint: "fs_main", Targets: []gputypes.ColorTargetState{{Format: r.format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll}}},
	})
	if err != nil {
		lod.close()
		return nil, fmt.Errorf("create LOD water pipeline: %w", err)
	}
	for range 2 {
		lod.workers.Add(1)
		go lod.buildLoop()
	}
	return lod, nil
}

func (lod *webGPULODRenderer) buildLoop() {
	defer lod.workers.Done()
	for {
		select {
		case <-lod.stop:
			return
		case job := <-lod.jobs:
			select {
			case <-lod.stop:
				return
			default:
			}
			start := time.Now()
			result := lodBuildResult{key: job.key, version: job.version, step: job.step, data: buildLODTileAtStep(lod.seed, job.key, job.overrides, job.step)}
			perfMon.recordLoading(phaseLODBuild, start)
			select {
			case lod.results <- result:
			case <-lod.stop:
				return
			}
		}
	}
}

func (lod *webGPULODRenderer) close() {
	if lod == nil {
		return
	}
	close(lod.stop)
	lod.workers.Wait()
	for _, mesh := range lod.tiles {
		mesh.Unload()
	}
	for _, mesh := range lod.waterTiles {
		mesh.Unload()
	}
	if lod.waterPipeline != nil {
		lod.waterPipeline.Release()
	}
	if lod.waterShader != nil {
		lod.waterShader.Release()
	}
	if lod.pipeline != nil {
		lod.pipeline.Release()
	}
	if lod.shader != nil {
		lod.shader.Release()
	}
}

func (lod *webGPULODRenderer) radiusOffsets(radius int) []lodTileKey {
	if lod.radius == radius && lod.offsets != nil {
		return lod.offsets
	}
	lod.radius = radius
	lod.offsets = lod.offsets[:0]
	for z := -radius; z <= radius; z++ {
		for x := -radius; x <= radius; x++ {
			if x*x+z*z <= (radius+1)*(radius+1) {
				lod.offsets = append(lod.offsets, lodTileKey{x, z})
			}
		}
	}
	sort.Slice(lod.offsets, func(i, j int) bool {
		a, b := lod.offsets[i], lod.offsets[j]
		return a.X*a.X+a.Z*a.Z < b.X*b.X+b.Z*b.Z
	})
	return lod.offsets
}

func lodTileInRange(key, center lodTileKey, radius int) bool {
	dx, dz := key.X-center.X, key.Z-center.Z
	return dx*dx+dz*dz <= (radius+1)*(radius+1)
}

// Hysteresis prevents repeated rebuilding when the camera hovers near a level
// boundary. The full-detail radius is followed by 2, 4, 8, then 16 blocks
// per cell. Missing chunks close to the camera also need 2-block fallback.
func lodStepForDistance(distance, fullRadius float32, previous int) int {
	if distance < fullRadius-96 {
		if distance < 12*chunkWidth {
			return lodNearCellSize
		}
		return lodTransitionCellSize
	}
	nearLimit := fullRadius + 4*chunkWidth
	transitionLimit := fullRadius + 12*chunkWidth
	midLimit := fullRadius + 32*chunkWidth
	const hysteresis = 16
	switch previous {
	case lodNearCellSize:
		nearLimit += hysteresis
	case lodTransitionCellSize:
		nearLimit -= hysteresis
		transitionLimit += hysteresis
	case lodCellSize:
		transitionLimit -= hysteresis
		midLimit += hysteresis
	case lodFarCellSize:
		midLimit -= hysteresis
	}
	if distance < float32(nearLimit) {
		return lodNearCellSize
	}
	if distance < float32(transitionLimit) {
		return lodTransitionCellSize
	}
	if distance < float32(midLimit) {
		return lodCellSize
	}
	return lodFarCellSize
}

func lodStepForTile(key lodTileKey, camera webGPUCamera, fullRadius float32, previous int) int {
	x := float64(key.X*lodTileSize+lodTileSize/2) - float64(camera.Position.X)
	z := float64(key.Z*lodTileSize+lodTileSize/2) - float64(camera.Position.Z)
	return lodStepForDistance(float32(math.Hypot(x, z)), fullRadius, previous)
}

func (lod *webGPULODRenderer) draw(pass *wgpu.RenderPassEncoder, r *webGPUWorldRenderer, camera webGPUCamera, world *World) error {
	if lod.covered == nil {
		lod.covered = make(map[lodTileKey]bool)
	}
	clear(lod.covered)
	cache := &world.render
	center := lodTileKey{int(math.Floor(float64(camera.Position.X) / lodTileSize)), int(math.Floor(float64(camera.Position.Z) / lodTileSize))}
	radius := (horizonDistance()*chunkWidth + lodTileSize - 1) / lodTileSize
	fullRadius := float32(renderDistance() * chunkWidth)
	projection, view, _ := webGPUCameraMatrices(camera, r.width, r.height)
	frustum := ExtractFrustum(projection.Mul4(view))
	for key, mesh := range lod.tiles {
		if !lodTileInRange(key, center, radius) {
			mesh.Unload()
			if water := lod.waterTiles[key]; water != nil {
				water.Unload()
				delete(lod.waterTiles, key)
			}
			delete(lod.tiles, key)
			delete(lod.versions, key)
			delete(lod.steps, key)
		}
	}
	for i := 0; i < 4; i++ {
		select {
		case result := <-lod.results:
			if pending, ok := lod.pending[result.key]; ok && pending == (lodBuildState{result.version, result.step}) {
				delete(lod.pending, result.key)
			}
			if !lodTileInRange(result.key, center, radius) || result.version != world.lodVersions[result.key] ||
				result.step != lodStepForTile(result.key, camera, fullRadius, lod.steps[result.key]) {
				continue
			}
			mesh, err := r.backend.UploadChecked(result.data.vertices, result.data.indices)
			if err != nil {
				return fmt.Errorf("upload distant terrain: %w", err)
			}
			var water platform.MeshHandle
			if len(result.data.waterIndices) > 0 {
				uv, ok := assets.getAtlasUV("textures/block/water_still.png")
				if !ok {
					mesh.Unload()
					return fmt.Errorf("distant water texture is absent from atlas")
				}
				for i := range result.data.waterVertices {
					coord := &result.data.waterVertices[i].Texcoord
					coord[0] = uv.X + coord[0]*uv.Width
					coord[1] = uv.Y + coord[1]*uv.Height
				}
				water, err = r.backend.UploadChecked(result.data.waterVertices, result.data.waterIndices)
				if err != nil {
					mesh.Unload()
					return fmt.Errorf("upload distant water: %w", err)
				}
			}
			if old := lod.tiles[result.key]; old != nil {
				old.Unload()
			}
			if old := lod.waterTiles[result.key]; old != nil {
				old.Unload()
			}
			lod.tiles[result.key] = mesh
			if water == nil {
				delete(lod.waterTiles, result.key)
			} else {
				lod.waterTiles[result.key] = water
			}
			lod.versions[result.key] = result.version
			lod.steps[result.key] = result.step
		default:
			i = 4
		}
	}
	pass.SetPipeline(lod.pipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	scheduled := 0
	snapshotDeadline := time.Now().Add(time.Millisecond)
	for _, offset := range lod.radiusOffsets(radius) {
		key := lodTileKey{center.X + offset.X, center.Z + offset.Z}
		min := mgl32.Vec3{float32(key.X * lodTileSize), 0, float32(key.Z * lodTileSize)}
		max := min.Add(mgl32.Vec3{lodTileSize, chunkHeight, lodTileSize})
		if !frustum.IntersectsAABB(min, max) {
			continue
		}
		if lodTileFullyCovered(world, key, camera) {
			lod.covered[key] = true
			continue
		}
		world.requestLODCacheTile(key)
		step := lodStepForTile(key, camera, fullRadius, lod.steps[key])
		if mesh := lod.tiles[key]; mesh != nil {
			if err := r.backend.DrawPass(pass, mesh); err != nil {
				return err
			}
			cache.drawCalls++
			cache.triangles += int(mesh.IndexCount()) / 3
			if lod.versions[key] == world.lodVersions[key] && lod.steps[key] == step {
				continue
			}
		}
		version := world.lodVersions[key]
		state := lodBuildState{version, step}
		// A pending job owns this tile until its result is drained. New versions
		// coalesce on World; never queue multiple obsolete builds for one tile.
		if _, pending := lod.pending[key]; !pending && scheduled < 2 && time.Now().Before(snapshotDeadline) && len(lod.jobs) < cap(lod.jobs) {
			start := time.Now()
			overrides := world.snapshotLODTile(key)
			perfMon.recordLoading(phaseLODSnapshot, start)
			select {
			case lod.jobs <- lodBuildJob{key: key, version: version, step: step, overrides: overrides}:
				lod.pending[key] = state
				scheduled++
			default:
			}
		}
	}
	return nil
}

func (lod *webGPULODRenderer) drawWater(pass *wgpu.RenderPassEncoder, r *webGPUWorldRenderer, camera webGPUCamera, cache *worldRenderCache) error {
	projection, view, _ := webGPUCameraMatrices(camera, r.width, r.height)
	frustum := ExtractFrustum(projection.Mul4(view))
	pass.SetPipeline(lod.waterPipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	for key, mesh := range lod.waterTiles {
		if lod.covered[key] {
			continue
		}
		min := mgl32.Vec3{float32(key.X * lodTileSize), float32(seaLevel) - 1, float32(key.Z * lodTileSize)}
		max := min.Add(mgl32.Vec3{lodTileSize, 2, lodTileSize})
		if !frustum.IntersectsAABB(min, max) {
			continue
		}
		if err := r.backend.DrawPass(pass, mesh); err != nil {
			return err
		}
		cache.drawCalls++
		cache.triangles += int(mesh.IndexCount()) / 3
	}
	return nil
}

// Match the GPU mask, with a whole chunk halo for crowns and seam overhangs.
// Fully hidden tiles need neither a draw nor a rebuild on each packet update.
func lodTileFullyCovered(world *World, key lodTileKey, camera webGPUCamera) bool {
	cx := int(math.Floor(float64(camera.Position.X) / chunkWidth))
	cz := int(math.Floor(float64(camera.Position.Z) / chunkWidth))
	radius := renderDistance()
	x0, x1, z0, z1 := key.X*8-1, key.X*8+8, key.Z*8-1, key.Z*8+8
	for _, p := range [4]chunkKey{{x0, z0}, {x0, z1}, {x1, z0}, {x1, z1}} {
		dx, dz := p.X-cx, p.Z-cz
		if dx*dx+dz*dz > radius*radius {
			return false
		}
	}
	world.chunksMu.RLock()
	defer world.chunksMu.RUnlock()
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			c := world.chunks[chunkKey{x, z}]
			if c == nil || !c.generated || !c.lodOccludes {
				return false
			}
		}
	}
	return true
}

func (lod *webGPULODRenderer) drawPreview(pass *wgpu.RenderPassEncoder, r *webGPUWorldRenderer, camera webGPUCamera, world *World) error {
	if err := lod.draw(pass, r, camera, world); err != nil {
		return err
	}
	return lod.drawWater(pass, r, camera, &world.render)
}

func (r *webGPUWorldRenderer) drawDistantTerrain(pass *wgpu.RenderPassEncoder, world *World, camera webGPUCamera) error {
	if horizonDistance() <= renderDistance() {
		if r.lod != nil {
			r.lod.close()
			r.lod = nil
		}
		return nil
	}
	if r.lod == nil || r.lod.seed != world.seed {
		if r.lod != nil {
			r.lod.close()
		}
		var err error
		r.lod, err = newWebGPULODRenderer(r, world.seed)
		if err != nil {
			return err
		}
	}
	return r.lod.draw(pass, r, camera, world)
}
