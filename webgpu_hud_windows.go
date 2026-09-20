//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const webGPUHUDMaxVertices = 16384

const webGPUHUDShader = `
@group(0) @binding(0) var hud_atlas: texture_2d<f32>;
@group(0) @binding(1) var hud_sampler: sampler;

struct VertexInput {
    @location(0) position: vec2f,
    @location(1) uv: vec2f,
    @location(2) color: vec4f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) uv: vec2f,
    @location(1) color: vec4f,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4f(in.position, 0.0, 1.0);
    out.uv = in.uv;
    out.color = in.color;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4f {
    if in.uv.x < 0.0 {
        return in.color;
    }
    return textureSample(hud_atlas, hud_sampler, in.uv) * in.color;
}
`

type webGPUHUDVertex struct {
	Position [2]float32
	UV       [2]float32
	Color    [4]float32
}

type webGPUHUDRenderer struct {
	device          *wgpu.Device
	queue           *wgpu.Queue
	pipeline        *wgpu.RenderPipeline
	shader          *wgpu.ShaderModule
	layout          *wgpu.PipelineLayout
	bindGroupLayout *wgpu.BindGroupLayout
	bindGroup       *wgpu.BindGroup
	sampler         *wgpu.Sampler
	uploads         webGPUFrameUploads
}

var activeWebGPUHUDRenderer *webGPUHUDRenderer

func ensureWebGPUHUDRenderer(worldRenderer *webGPUWorldRenderer) (*webGPUHUDRenderer, error) {
	if activeWebGPUHUDRenderer != nil {
		return activeWebGPUHUDRenderer, nil
	}
	if worldRenderer == nil || worldRenderer.device == nil || worldRenderer.atlasView == nil || worldRenderer.atlasSampler == nil {
		return nil, fmt.Errorf("WebGPU world renderer is not ready for HUD creation")
	}

	h := &webGPUHUDRenderer{device: worldRenderer.device, queue: worldRenderer.queue}
	fail := func(err error) (*webGPUHUDRenderer, error) {
		h.Close()
		return nil, err
	}
	var err error
	h.shader, err = h.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft WebGPU HUD shader", WGSL: webGPUHUDShader})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU HUD shader: %w", err))
	}
	h.bindGroupLayout, err = h.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft WebGPU HUD BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU HUD bind group layout: %w", err))
	}
	h.layout, err = h.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "GoCraft WebGPU HUD layout", BindGroupLayouts: []*wgpu.BindGroupLayout{h.bindGroupLayout},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU HUD pipeline layout: %w", err))
	}
	h.sampler, err = h.device.CreateSampler(webGPUAtlasSamplerDescriptor(false, 1))
	if err != nil {
		return fail(err)
	}
	h.bindGroup, err = h.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "GoCraft WebGPU HUD bind group",
		Layout: h.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: worldRenderer.atlasBaseView},
			{Binding: 1, Sampler: h.sampler},
		},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU HUD bind group: %w", err))
	}

	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	stride := uint64(unsafe.Sizeof(webGPUHUDVertex{}))
	h.pipeline, err = h.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "GoCraft WebGPU HUD pipeline",
		Layout: h.layout,
		Vertex: wgpu.VertexState{
			Module: h.shader, EntryPoint: "vs_main",
			Buffers: []gputypes.VertexBufferLayout{{
				ArrayStride: stride,
				StepMode:    gputypes.VertexStepModeVertex,
				Attributes: []gputypes.VertexAttribute{
					{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
					{Format: gputypes.VertexFormatFloat32x2, Offset: 8, ShaderLocation: 1},
					{Format: gputypes.VertexFormatFloat32x4, Offset: 16, ShaderLocation: 2},
				},
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList, FrontFace: gputypes.FrontFaceCCW, CullMode: gputypes.CullModeNone},
		Multisample: gputypes.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &wgpu.FragmentState{Module: h.shader, EntryPoint: "fs_main", Targets: []gputypes.ColorTargetState{{
			Format: worldRenderer.format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll,
		}}},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU HUD pipeline: %w", err))
	}
	activeWebGPUHUDRenderer = h
	return h, nil
}

func closeWebGPUHUDRenderer() {
	if activeWebGPUHUDRenderer == nil {
		return
	}
	activeWebGPUHUDRenderer.Close()
	activeWebGPUHUDRenderer = nil
}

func (h *webGPUHUDRenderer) Close() {
	if h != nil {
		h.uploads.close()
	}
	if h == nil {
		return
	}
	if h.pipeline != nil {
		h.pipeline.Release()
		h.pipeline = nil
	}
	if h.bindGroup != nil {
		h.bindGroup.Release()
		h.bindGroup = nil
	}
	if h.sampler != nil {
		h.sampler.Release()
		h.sampler = nil
	}
	if h.layout != nil {
		h.layout.Release()
		h.layout = nil
	}
	if h.bindGroupLayout != nil {
		h.bindGroupLayout.Release()
		h.bindGroupLayout = nil
	}
	if h.shader != nil {
		h.shader.Release()
		h.shader = nil
	}
}

type webGPUHUDBuilder struct {
	vertices      []webGPUHUDVertex
	width, height float32
}

func newWebGPUHUDBuilder(width, height uint32) *webGPUHUDBuilder {
	return &webGPUHUDBuilder{vertices: make([]webGPUHUDVertex, 0, 2048), width: float32(width), height: float32(height)}
}

func (b *webGPUHUDBuilder) point(x, y float32) [2]float32 {
	return [2]float32{x/b.width*2 - 1, 1 - y/b.height*2}
}

func (b *webGPUHUDBuilder) rect(x, y, w, h float32, color [4]float32) {
	b.texturedRect(x, y, w, h, -1, -1, -1, -1, color)
}

func (b *webGPUHUDBuilder) texturedRect(x, y, w, h, u0, v0, u1, v1 float32, color [4]float32) {
	p0, p1 := b.point(x, y), b.point(x+w, y+h)
	verts := [6]webGPUHUDVertex{
		{Position: [2]float32{p0[0], p0[1]}, UV: [2]float32{u0, v0}, Color: color},
		{Position: [2]float32{p1[0], p0[1]}, UV: [2]float32{u1, v0}, Color: color},
		{Position: [2]float32{p1[0], p1[1]}, UV: [2]float32{u1, v1}, Color: color},
		{Position: [2]float32{p0[0], p0[1]}, UV: [2]float32{u0, v0}, Color: color},
		{Position: [2]float32{p1[0], p1[1]}, UV: [2]float32{u1, v1}, Color: color},
		{Position: [2]float32{p0[0], p1[1]}, UV: [2]float32{u0, v1}, Color: color},
	}
	b.vertices = append(b.vertices, verts[:]...)
}

func (b *webGPUHUDBuilder) border(x, y, w, h, thickness float32, color [4]float32) {
	b.rect(x, y, w, thickness, color)
	b.rect(x, y+h-thickness, w, thickness, color)
	b.rect(x, y+thickness, thickness, h-2*thickness, color)
	b.rect(x+w-thickness, y+thickness, thickness, h-2*thickness, color)
}

func webGPUHUDScale(width, height uint32) float32 {
	return min(float32(1.5), float32(width)/1280, float32(height)/720)
}

func (b *webGPUHUDBuilder) addCrosshair(scale float32) {
	cx, cy := b.width/2, b.height/2
	length := max(float32(8)*scale, 6)
	thickness := max(float32(2)*scale, 2)
	border := float32(2)
	black := [4]float32{0, 0, 0, 0.9}
	white := [4]float32{1, 1, 1, 1}
	b.rect(cx-length-border, cy-thickness/2-border, length*2+border*2, thickness+border*2, black)
	b.rect(cx-thickness/2-border, cy-length-border, thickness+border*2, length*2+border*2, black)
	b.rect(cx-length, cy-thickness/2, length*2, thickness, white)
	b.rect(cx-thickness/2, cy-length, thickness, length*2, white)
}

func webGPUHUDHotbarBlock(state *InputState, slot int) byte {
	if state == nil || slot < 0 || slot >= len(state.Hotbar) {
		return blockAir
	}
	if currentGameMode == ModeCreative || client == nil {
		return state.Hotbar[slot]
	}
	item := client.Inventory.Slots[slot]
	if item.ID <= 0 || item.ID > 255 {
		return blockAir
	}
	return byte(item.ID)
}

func (b *webGPUHUDBuilder) addBlockIcon(block byte, x, y, size float32) {
	if block == blockAir || assets == nil || assets.atlas == nil {
		return
	}
	def := GetBlock(block)
	if def == nil || def.ID == blockAir || def.Textures.Top == "" {
		return
	}
	uv, ok := assets.atlas.UVs[def.Textures.Top]
	if !ok {
		return
	}
	pad := size * 0.13
	b.texturedRect(x+pad, y+pad, size-pad*2, size-pad*2, uv.X, uv.Y, uv.X+uv.Width, uv.Y+uv.Height, [4]float32{1, 1, 1, 1})
}

func (b *webGPUHUDBuilder) addHotbar(state *InputState, scale float32) {
	if state == nil {
		return
	}
	slot, stride := float32(48)*scale, float32(54)*scale
	width := 9*stride + 10*scale
	x := (b.width - width) / 2
	y := b.height - 68*scale
	background := [4]float32{0.055, 0.085, 0.085, 0.88}
	line := [4]float32{0.30, 0.36, 0.33, 0.95}
	slotColor := [4]float32{0.12, 0.15, 0.13, 0.94}
	accent := [4]float32{0.58, 0.78, 0.38, 1}
	b.rect(x, y, width, 60*scale, background)
	b.border(x, y, width, 60*scale, max(scale, 1), line)
	for i := 0; i < 9; i++ {
		rx := x + 8*scale + float32(i)*stride
		ry := y + 6*scale
		b.rect(rx, ry, slot, slot, slotColor)
		b.border(rx, ry, slot, slot, max(scale, 1), line)
		if i == state.SelectedSlot {
			b.border(rx-2*scale, ry-2*scale, slot+4*scale, slot+4*scale, max(2*scale, 2), accent)
		}
		b.addBlockIcon(webGPUHUDHotbarBlock(state, i), rx, ry, slot)
	}
}

func (b *webGPUHUDBuilder) addVitals(state *InputState, scale float32) {
	if state == nil || !state.VitalsReady || currentGameMode != ModeSurvival {
		return
	}
	x := b.width/2 - 240*scale
	y := b.height - 126*scale
	background := [4]float32{0.055, 0.085, 0.085, 0.88}
	line := [4]float32{0.30, 0.36, 0.33, 0.95}
	heart := [4]float32{0.85, 0.44, 0.39, 1}
	air := [4]float32{0.54, 0.76, 0.84, 1}
	b.rect(x-8*scale, y-7*scale, 242*scale, 29*scale, background)
	b.border(x-8*scale, y-7*scale, 242*scale, 29*scale, max(scale, 1), line)
	mask := []string{"0110110", "1111111", "1111111", "0111110", "0011100", "0001000"}
	for i := 0; i < 10; i++ {
		hp := int(state.Vitals.Health) - i*2
		for row, pattern := range mask {
			for col, bit := range pattern {
				if bit != '1' {
					continue
				}
				color := line
				if hp >= 2 || (hp == 1 && col < 3) {
					color = heart
				}
				b.rect(x+float32(i)*18*scale+float32(col)*2*scale, y+float32(row)*2*scale, 2*scale, 2*scale, color)
			}
		}
	}
	if state.Vitals.Air < maxAir {
		bx := b.width/2 + 12*scale
		b.rect(bx, y-7*scale, 226*scale, 29*scale, background)
		b.border(bx, y-7*scale, 226*scale, 29*scale, max(scale, 1), line)
		for i := 0; i < 10; i++ {
			color := line
			if state.Vitals.Air > int32(i*30) {
				color = air
			}
			b.rect(bx+(48+float32(i)*16)*scale-4*scale, y+2*scale, 8*scale, 8*scale, color)
		}
	}
	if state.HurtFlash > 0 {
		alpha := min(state.HurtFlash*2, float32(1))
		b.border(0, 0, b.width, b.height, 8*scale, [4]float32{0.85, 0.31, 0.24, alpha})
	}
}

func (h *webGPUHUDRenderer) Draw(pass *wgpu.RenderPassEncoder, width, height uint32, state *InputState) error {
	if h == nil || pass == nil || state == nil || width == 0 || height == 0 {
		return nil
	}
	b := newWebGPUHUDBuilder(width, height)
	scale := webGPUHUDScale(width, height)
	if !state.InventoryOpen {
		b.addHotbar(state, scale)
		b.addVitals(state, scale)
	}
	if !state.InventoryOpen && !isPaused && !state.isDead() {
		b.addCrosshair(scale)
	}
	if len(b.vertices) == 0 {
		return nil
	}
	if len(b.vertices) > webGPUHUDMaxVertices {
		return fmt.Errorf("WebGPU HUD vertex budget exceeded: %d > %d", len(b.vertices), webGPUHUDMaxVertices)
	}
	byteLen := len(b.vertices) * int(unsafe.Sizeof(webGPUHUDVertex{}))
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&b.vertices[0])), byteLen)
	buffer, err := h.uploads.write(h.device, h.queue, bytes, gputypes.BufferUsageVertex)
	if err != nil {
		return fmt.Errorf("upload WebGPU HUD vertices: %w", err)
	}
	pass.SetPipeline(h.pipeline)
	pass.SetBindGroup(0, h.bindGroup, nil)
	pass.SetVertexBuffer(0, buffer, 0)
	pass.Draw(gputypes.DrawArgs{VertexCount: uint32(len(b.vertices)), InstanceCount: 1})
	return nil
}
