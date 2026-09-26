//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"gocraft/platform"
)

const webGPULODShader = `
struct Scene {
    view_proj: mat4x4<f32>, eye_pos: vec4f, fog_range: vec4f,
    fog_color: vec4f, daylight: vec4f,
}
@group(0) @binding(0) var<uniform> scene: Scene;
struct Input { @location(0) position: vec3f, @location(3) color: vec4f, }
struct Output {
    @builtin(position) position: vec4f,
    @location(0) world_pos: vec3f,
    @location(1) color: vec4f,
}
@vertex fn vs_main(input: Input) -> Output {
    var out: Output;
    var pos = input.position;
    let horizontal = length(pos.xz - scene.eye_pos.xz);
    // Keep a shallow underlay beneath real terrain, but retain it when the
    // full chunk stream has not caught up. Fade the offset across the seam.
    if input.color.a > 0.999 {
        pos.y -= 2.5 * (1.0 - smoothstep(max(0.0, scene.daylight.y - 64.0), scene.daylight.y + 64.0, horizontal));
    }
    out.position = scene.view_proj * vec4f(pos, 1.0);
    out.world_pos = pos;
    out.color = input.color;
    return out;
}
@fragment fn fs_main(input: Output) -> @location(0) vec4f {
    let horizontal = length(input.world_pos.xz - scene.eye_pos.xz);
    // Approximate tree crowns must not poke through loaded full-detail trees.
    if input.color.a < 0.999 && horizontal < scene.daylight.y { discard; }
    let fog = smoothstep(scene.fog_range.x, scene.fog_range.y, horizontal);
    return vec4f(mix(input.color.rgb * scene.daylight.x, scene.fog_color.rgb, fog), 1.0);
}
`

type lodBuildResult struct {
	key  lodTileKey
	data lodTileData
}

type webGPULODRenderer struct {
	seed     uint32
	shader   *wgpu.ShaderModule
	pipeline *wgpu.RenderPipeline
	tiles    map[lodTileKey]platform.MeshHandle
	pending  map[lodTileKey]bool
	jobs     chan lodTileKey
	results  chan lodBuildResult
	stop     chan struct{}
	workers  sync.WaitGroup
	offsets  []lodTileKey
	radius   int
}

func newWebGPULODRenderer(r *webGPUWorldRenderer, seed uint32) (*webGPULODRenderer, error) {
	lod := &webGPULODRenderer{
		seed: seed, tiles: make(map[lodTileKey]platform.MeshHandle), pending: make(map[lodTileKey]bool),
		jobs: make(chan lodTileKey, 32), results: make(chan lodBuildResult, 8), stop: make(chan struct{}),
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
		case key := <-lod.jobs:
			select {
			case <-lod.stop:
				return
			default:
			}
			result := lodBuildResult{key: key, data: buildLODTile(lod.seed, key)}
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

func (lod *webGPULODRenderer) draw(pass *wgpu.RenderPassEncoder, r *webGPUWorldRenderer, camera webGPUCamera, cache *worldRenderCache) error {
	center := lodTileKey{int(math.Floor(float64(camera.Position.X) / lodTileSize)), int(math.Floor(float64(camera.Position.Z) / lodTileSize))}
	radius := (horizonDistance()*chunkWidth + lodTileSize - 1) / lodTileSize
	projection, view, _ := webGPUCameraMatrices(camera, r.width, r.height)
	frustum := ExtractFrustum(projection.Mul4(view))
	for key, mesh := range lod.tiles {
		if !lodTileInRange(key, center, radius) {
			mesh.Unload()
			delete(lod.tiles, key)
		}
	}
	for i := 0; i < 4; i++ {
		select {
		case result := <-lod.results:
			delete(lod.pending, result.key)
			if !lodTileInRange(result.key, center, radius) {
				continue
			}
			mesh, err := r.backend.UploadChecked(result.data.vertices, result.data.indices)
			if err != nil {
				return fmt.Errorf("upload distant terrain: %w", err)
			}
			if old := lod.tiles[result.key]; old != nil {
				old.Unload()
			}
			lod.tiles[result.key] = mesh
		default:
			i = 4
		}
	}
	pass.SetPipeline(lod.pipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	scheduled := 0
	for _, offset := range lod.radiusOffsets(radius) {
		key := lodTileKey{center.X + offset.X, center.Z + offset.Z}
		min := mgl32.Vec3{float32(key.X * lodTileSize), 0, float32(key.Z * lodTileSize)}
		max := min.Add(mgl32.Vec3{lodTileSize, chunkHeight, lodTileSize})
		if !frustum.IntersectsAABB(min, max) {
			continue
		}
		if mesh := lod.tiles[key]; mesh != nil {
			if err := r.backend.DrawPass(pass, mesh); err != nil {
				return err
			}
			cache.drawCalls++
			cache.triangles += int(mesh.IndexCount()) / 3
			continue
		}
		if !lod.pending[key] && scheduled < 16 {
			select {
			case lod.jobs <- key:
				lod.pending[key] = true
				scheduled++
			default:
			}
		}
	}
	return nil
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
	return r.lod.draw(pass, r, camera, &world.render)
}
