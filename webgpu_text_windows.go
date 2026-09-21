//go:build windows

package main

import (
	"fmt"
	"unicode"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	webGPUFontAtlasWidth  = 128
	webGPUFontAtlasHeight = 64
	webGPUFontCellSize    = 8
	webGPUTextMaxVertices = 65536
)

const webGPUTextShader = `
@group(0) @binding(0) var font_tex: texture_2d<f32>;
@group(0) @binding(1) var font_sampler: sampler;

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
    let coverage = textureSample(font_tex, font_sampler, in.uv).a;
    if coverage < 0.01 {
        discard;
    }
    return vec4f(in.color.rgb, in.color.a * coverage);
}
`

type webGPUTextVertex struct {
	Position [2]float32
	UV       [2]float32
	Color    [4]float32
}

type webGPUTextRenderer struct {
	device          *wgpu.Device
	queue           *wgpu.Queue
	pipeline        *wgpu.RenderPipeline
	shader          *wgpu.ShaderModule
	layout          *wgpu.PipelineLayout
	bindGroupLayout *wgpu.BindGroupLayout
	bindGroup       *wgpu.BindGroup
	fontTexture     *wgpu.Texture
	fontView        *wgpu.TextureView
	fontSampler     *wgpu.Sampler
	uploads         webGPUFrameUploads
}

var activeWebGPUTextRenderer *webGPUTextRenderer

func ensureWebGPUTextRenderer(worldRenderer *webGPUWorldRenderer) (*webGPUTextRenderer, error) {
	if activeWebGPUTextRenderer != nil {
		return activeWebGPUTextRenderer, nil
	}
	if worldRenderer == nil || worldRenderer.device == nil || worldRenderer.queue == nil {
		return nil, fmt.Errorf("WebGPU world renderer is not ready for text creation")
	}

	r := &webGPUTextRenderer{device: worldRenderer.device, queue: worldRenderer.queue}
	fail := func(err error) (*webGPUTextRenderer, error) {
		r.Close()
		return nil, err
	}
	var err error
	r.shader, err = r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{Label: "GoCraft WebGPU text shader", WGSL: webGPUTextShader})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU text shader: %w", err))
	}

	r.fontTexture, err = r.device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "GoCraft built-in pixel font",
		Size:          wgpu.Extent3D{Width: webGPUFontAtlasWidth, Height: webGPUFontAtlasHeight, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU font texture: %w", err))
	}
	pixels := buildWebGPUPixelFontAtlas()
	if err := r.queue.WriteTexture(
		&wgpu.ImageCopyTexture{Texture: r.fontTexture, MipLevel: 0},
		pixels,
		&wgpu.ImageDataLayout{Offset: 0, BytesPerRow: webGPUFontAtlasWidth * 4, RowsPerImage: webGPUFontAtlasHeight},
		&wgpu.Extent3D{Width: webGPUFontAtlasWidth, Height: webGPUFontAtlasHeight, DepthOrArrayLayers: 1},
	); err != nil {
		return fail(fmt.Errorf("upload WebGPU font texture: %w", err))
	}
	r.fontView, err = r.device.CreateTextureView(r.fontTexture, nil)
	if err != nil {
		return fail(fmt.Errorf("create WebGPU font view: %w", err))
	}
	r.fontSampler, err = r.device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "GoCraft WebGPU font sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU font sampler: %w", err))
	}

	r.bindGroupLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "GoCraft WebGPU text BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{Binding: 0, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D}},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Sampler: &gputypes.SamplerBindingLayout{Type: gputypes.SamplerBindingTypeFiltering}},
		},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU text bind group layout: %w", err))
	}
	r.layout, err = r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label: "GoCraft WebGPU text layout", BindGroupLayouts: []*wgpu.BindGroupLayout{r.bindGroupLayout},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU text pipeline layout: %w", err))
	}
	r.bindGroup, err = r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "GoCraft WebGPU text bind group",
		Layout: r.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: r.fontView},
			{Binding: 1, Sampler: r.fontSampler},
		},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU text bind group: %w", err))
	}

	blend := &gputypes.BlendState{
		Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
		Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
	}
	stride := uint64(unsafe.Sizeof(webGPUTextVertex{}))
	r.pipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Label:  "GoCraft WebGPU text pipeline",
		Layout: r.layout,
		Vertex: wgpu.VertexState{
			Module: r.shader, EntryPoint: "vs_main",
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
		Fragment: &wgpu.FragmentState{Module: r.shader, EntryPoint: "fs_main", Targets: []gputypes.ColorTargetState{{
			Format: worldRenderer.format, Blend: blend, WriteMask: gputypes.ColorWriteMaskAll,
		}}},
	})
	if err != nil {
		return fail(fmt.Errorf("create WebGPU text pipeline: %w", err))
	}

	activeWebGPUTextRenderer = r
	return r, nil
}

func closeWebGPUTextRenderer() {
	if activeWebGPUTextRenderer == nil {
		return
	}
	activeWebGPUTextRenderer.Close()
	activeWebGPUTextRenderer = nil
}

func (r *webGPUTextRenderer) Close() {
	if r != nil {
		r.uploads.close()
	}
	if r == nil {
		return
	}
	if r.pipeline != nil {
		r.pipeline.Release()
		r.pipeline = nil
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
	if r.fontSampler != nil {
		r.fontSampler.Release()
		r.fontSampler = nil
	}
	if r.fontView != nil {
		r.fontView.Release()
		r.fontView = nil
	}
	if r.fontTexture != nil {
		r.fontTexture.Release()
		r.fontTexture = nil
	}
	if r.shader != nil {
		r.shader.Release()
		r.shader = nil
	}
}

type webGPUTextBatch struct {
	vertices      []webGPUTextVertex
	width, height float32
}

func newWebGPUTextBatch(width, height uint32) *webGPUTextBatch {
	return &webGPUTextBatch{vertices: make([]webGPUTextVertex, 0, 4096), width: float32(width), height: float32(height)}
}

func (b *webGPUTextBatch) point(x, y float32) [2]float32 {
	return [2]float32{x/b.width*2 - 1, 1 - y/b.height*2}
}

func (b *webGPUTextBatch) glyph(x, y, size float32, code byte, color [4]float32) {
	cellWidth := size * 0.72
	p0, p1 := b.point(x, y), b.point(x+cellWidth, y+size)
	col := float32(code % 16)
	row := float32(code / 16)
	u0 := col / 16.0
	v0 := row / 8.0
	u1 := (col + 1) / 16.0
	v1 := (row + 1) / 8.0
	b.vertices = append(b.vertices,
		webGPUTextVertex{Position: [2]float32{p0[0], p0[1]}, UV: [2]float32{u0, v0}, Color: color},
		webGPUTextVertex{Position: [2]float32{p1[0], p0[1]}, UV: [2]float32{u1, v0}, Color: color},
		webGPUTextVertex{Position: [2]float32{p1[0], p1[1]}, UV: [2]float32{u1, v1}, Color: color},
		webGPUTextVertex{Position: [2]float32{p0[0], p0[1]}, UV: [2]float32{u0, v0}, Color: color},
		webGPUTextVertex{Position: [2]float32{p1[0], p1[1]}, UV: [2]float32{u1, v1}, Color: color},
		webGPUTextVertex{Position: [2]float32{p0[0], p1[1]}, UV: [2]float32{u0, v1}, Color: color},
	)
}

func webGPUTextRuneCode(r rune) byte {
	if r >= 32 && r <= 126 {
		return byte(r)
	}
	return byte('?')
}

func (b *webGPUTextBatch) text(x, y, size float32, text string, color [4]float32) {
	originX := x
	advance := size * 0.66
	lineHeight := size * 1.18
	for _, r := range text {
		if r == '\n' {
			x = originX
			y += lineHeight
			continue
		}
		if r == '\t' {
			x += advance * 4
			continue
		}
		b.glyph(x, y, size, webGPUTextRuneCode(r), color)
		x += advance
	}
}

func (b *webGPUTextBatch) shadowText(x, y, size float32, text string, color [4]float32) {
	offset := max(float32(1), size*0.08)
	b.text(x+offset, y+offset, size, text, [4]float32{0, 0, 0, color[3] * 0.85})
	b.text(x, y, size, text, color)
}

func webGPUTextWidth(size float32, text string) float32 {
	maxRunes, current := 0, 0
	for _, r := range text {
		if r == '\n' {
			maxRunes = max(maxRunes, current)
			current = 0
			continue
		}
		current++
	}
	maxRunes = max(maxRunes, current)
	return float32(maxRunes) * size * 0.66
}

func (b *webGPUTextBatch) centered(y, size float32, text string, color [4]float32) {
	b.shadowText((b.width-webGPUTextWidth(size, text))/2, y, size, text, color)
}

func webGPUHotbarItem(state *InputState, slot int) (byte, int32) {
	if state == nil || slot < 0 || slot >= len(state.Hotbar) {
		return 0, 0
	}
	if currentGameMode == ModeCreative || client == nil {
		return state.Hotbar[slot], 1
	}
	if slot >= len(client.Inventory.Slots) {
		return 0, 0
	}
	stack := client.Inventory.Slots[slot]
	if stack.ID <= 0 || stack.ID > 255 {
		return 0, 0
	}
	return byte(stack.ID), stack.Count
}

func (b *webGPUTextBatch) addHotbarText(state *InputState, scale float32) {
	if state == nil || state.InventoryOpen {
		return
	}
	slot, stride := float32(48)*scale, float32(54)*scale
	width := 9*stride + 10*scale
	x := (b.width - width) / 2
	y := b.height - 68*scale
	muted := [4]float32{0.68, 0.72, 0.68, 1}
	white := [4]float32{0.94, 0.96, 0.94, 1}
	for i := 0; i < 9; i++ {
		rx := x + 8*scale + float32(i)*stride
		ry := y + 6*scale
		b.shadowText(rx+3*scale, ry+2*scale, max(float32(9)*scale, 8), fmt.Sprint(i+1), muted)
		_, count := webGPUHotbarItem(state, i)
		if count > 1 {
			label := fmt.Sprint(count)
			size := max(float32(11)*scale, 9)
			b.shadowText(rx+slot-webGPUTextWidth(size, label)-3*scale, ry+slot-size-2*scale, size, label, white)
		}
	}
	id, _ := webGPUHotbarItem(state, state.SelectedSlot)
	if id == 0 {
		return
	}
	name := GetItem(id).Name
	if name == "" {
		return
	}
	size := max(float32(15)*scale, 11)
	b.centered(y-29*scale, size, name, white)
}

func (b *webGPUTextBatch) addVitalsText(state *InputState, scale float32) {
	if state == nil || !state.VitalsReady || currentGameMode != ModeSurvival || state.InventoryOpen {
		return
	}
	x := b.width/2 - 240*scale
	y := b.height - 126*scale
	white := [4]float32{0.94, 0.96, 0.94, 1}
	muted := [4]float32{0.62, 0.72, 0.68, 1}
	size := max(float32(11)*scale, 9)
	b.shadowText(x+184*scale, y+1*scale, size, fmt.Sprintf("%d/20", state.Vitals.Health), white)
	if state.Vitals.Air < maxAir {
		b.shadowText(b.width/2+20*scale, y+1*scale, size, "AIR", muted)
	}
	statusY := y - 24*scale
	switch {
	case state.Vitals.Fire > 0:
		b.shadowText(x, statusY, size, "BURNING - FIND WATER", [4]float32{1, 0.55, 0.32, 1})
	case state.IsSwimming:
		b.shadowText(x, statusY, size, "SWIM  SPACE:UP  SHIFT:DOWN", muted)
	case state.IsSneaking:
		b.shadowText(x, statusY, size, "SNEAKING", [4]float32{0.58, 0.78, 0.38, 1})
	case state.IsRunning:
		b.shadowText(x, statusY, size, "SPRINTING", [4]float32{0.58, 0.78, 0.38, 1})
	}
}

func (b *webGPUTextBatch) addChat(scale float32) {
	if len(chatHistory) == 0 && !isChatOpen {
		return
	}
	size := max(float32(14)*scale, 11)
	lineHeight := size * 1.25
	count := min(6, len(chatHistory))
	bottom := b.height - 108*scale
	top := bottom - float32(count)*lineHeight
	white := [4]float32{0.94, 0.96, 0.94, 1}
	for i, msg := range chatHistory[len(chatHistory)-count:] {
		b.shadowText(20*scale, top+float32(i)*lineHeight, size, msg, white)
	}
	if isChatOpen {
		input := "> " + chatInput + "_"
		maxChars := max(8, int((b.width-40*scale)/(size*0.66)))
		runes := []rune(input)
		if len(runes) > maxChars {
			runes = runes[len(runes)-maxChars:]
		}
		b.shadowText(20*scale, bottom+5*scale, size, string(runes), white)
	}
}

func (b *webGPUTextBatch) addDebug(state *InputState, scale float32) {
	if state == nil || !state.ShowDebug || perfMon == nil {
		return
	}
	m := perfMon.Metrics
	fps := float32(0)
	if dt := gameFrameTime(); dt > 0 {
		fps = 1 / dt
	}
	lines := []string{
		fmt.Sprintf("%.0f FPS", fps),
		fmt.Sprintf("POS %.1f %.1f %.1f", camera.Position.X, camera.Position.Y, camera.Position.Z),
		fmt.Sprintf("CHUNKS %d  MESH %d", len(world.chunks), m.ActiveMeshes),
		fmt.Sprintf("DRAWS %d  TRI %d", m.DrawCalls, m.Triangles),
		fmt.Sprintf("MESH Q %d/%d  UPD %d/S", m.MeshJobs, m.MeshResults, m.MeshesPerSec),
		fmt.Sprintf("FRAME P95/P99 %.1f/%.1f MS", m.FrameP95, m.FrameP99),
		fmt.Sprintf("MEM %d MB  GC %d", m.HeapAllocMB, m.NumGC),
	}
	size := max(float32(12)*scale, 10)
	white := [4]float32{0.94, 0.96, 0.94, 1}
	for i, line := range lines {
		b.shadowText(10*scale, 10*scale+float32(i)*size*1.2, size, line, white)
	}
}

func (b *webGPUTextBatch) addPauseAndDeath(state *InputState, scale float32) {
	if nativeWindowActive {
		return
	} // The full menu overlay owns these states.
	if state == nil {
		return
	}
	white := [4]float32{0.96, 0.97, 0.96, 1}
	accent := [4]float32{0.67, 0.86, 0.44, 1}
	warning := [4]float32{0.95, 0.45, 0.38, 1}
	if state.isDead() {
		b.centered(b.height*0.34, max(float32(34)*scale, 26), "YOU DIED", warning)
		if state.Vitals.Cause != "" {
			b.centered(b.height*0.34+48*scale, max(float32(14)*scale, 11), state.Vitals.Cause, white)
		}
		label := "PRESS R TO RESPAWN"
		if state.RespawnWaiting {
			label = "PREPARING A SAFE SPAWN..."
		}
		b.centered(b.height*0.34+82*scale, max(float32(15)*scale, 12), label, accent)
		return
	}
	if isPaused {
		b.centered(b.height*0.38, max(float32(34)*scale, 26), "PAUSED", white)
		b.centered(b.height*0.38+48*scale, max(float32(14)*scale, 11), "ESC TO RESUME", accent)
	}
}

func (r *webGPUTextRenderer) Draw(pass *wgpu.RenderPassEncoder, width, height uint32, state *InputState) error {
	if r == nil || pass == nil || state == nil || width == 0 || height == 0 {
		return nil
	}
	batch := newWebGPUTextBatch(width, height)
	scale := webGPUHUDScale(width, height)
	batch.addHotbarText(state, scale)
	batch.addVitalsText(state, scale)
	batch.addChat(scale)
	batch.addDebug(state, scale)
	batch.addPauseAndDeath(state, scale)
	if len(batch.vertices) == 0 {
		return nil
	}
	if len(batch.vertices) > webGPUTextMaxVertices {
		return fmt.Errorf("WebGPU text vertex budget exceeded: %d > %d", len(batch.vertices), webGPUTextMaxVertices)
	}
	byteLen := len(batch.vertices) * int(unsafe.Sizeof(webGPUTextVertex{}))
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&batch.vertices[0])), byteLen)
	buffer, err := r.uploads.write(r.device, r.queue, bytes, gputypes.BufferUsageVertex)
	if err != nil {
		return fmt.Errorf("upload WebGPU text vertices: %w", err)
	}
	pass.SetPipeline(r.pipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	pass.SetVertexBuffer(0, buffer, 0)
	pass.Draw(gputypes.DrawArgs{VertexCount: uint32(len(batch.vertices)), InstanceCount: 1})
	return nil
}

func buildWebGPUPixelFontAtlas() []byte {
	pixels := make([]byte, webGPUFontAtlasWidth*webGPUFontAtlasHeight*4)
	for code := 32; code <= 126; code++ {
		glyph := webGPUPixelGlyphs[unicode.ToUpper(rune(code))]
		if glyph == 0 && code != ' ' {
			glyph = webGPUPixelGlyphs['?']
		}
		cellX := (code % 16) * webGPUFontCellSize
		cellY := (code / 16) * webGPUFontCellSize
		for bit := 0; bit < 35; bit++ {
			if glyph&(uint64(1)<<uint(34-bit)) == 0 {
				continue
			}
			x := bit % 5
			y := bit / 5
			px := cellX + 1 + x
			py := cellY + y
			offset := (py*webGPUFontAtlasWidth + px) * 4
			pixels[offset+0] = 255
			pixels[offset+1] = 255
			pixels[offset+2] = 255
			pixels[offset+3] = 255
		}
	}
	return pixels
}

var webGPUPixelGlyphs = map[rune]uint64{
	' ': 0x000000000, 'A': 0x3A31FC631, 'B': 0x7A31F463E, 'C': 0x3A308422E,
	'D': 0x7A318C63E, 'E': 0x7E10F421F, 'F': 0x7E10F4210, 'G': 0x3A30BC62F,
	'H': 0x4631FC631, 'I': 0x7C842109F, 'J': 0x1C4214A4C, 'K': 0x4654C5251,
	'L': 0x42108421F, 'M': 0x4775AC631, 'N': 0x47359C631, 'O': 0x3A318C62E,
	'P': 0x7A31F4210, 'Q': 0x3A318D64D, 'R': 0x7A31F5251, 'S': 0x3E107043E,
	'T': 0x7C8421084, 'U': 0x46318C62E, 'V': 0x46318C544, 'W': 0x4631AD6AA,
	'X': 0x462A22A31, 'Y': 0x462A21084, 'Z': 0x7C222221F, '0': 0x3A33AE62E,
	'1': 0x11942109F, '2': 0x3A211111F, '3': 0x78217043E, '4': 0x08CA97C42,
	'5': 0x7E10F043E, '6': 0x3A10F462E, '7': 0x7C2222108, '8': 0x3A317462E,
	'9': 0x3A317842E, '?': 0x3A2111004, '!': 0x108421004, '.': 0x00000018C,
	',': 0x000003188, ':': 0x018C03180, ';': 0x018C03188, '-': 0x0000F8000,
	'_': 0x00000001F, '/': 0x044222110, '\\': 0x410820841, '+': 0x0084F9080,
	'=': 0x03E0F8000, '(': 0x088842082, ')': 0x208210888, '[': 0x39084210E,
	']': 0x38421084E, '<': 0x088882082, '>': 0x208208888, '%': 0x674222173,
	'#': 0x295F57D4A, '@': 0x3A37ADE0E, '\'': 0x108800000, '"': 0x295400000,
	'*': 0x02AEFBAA0, '|': 0x108421084,
}
