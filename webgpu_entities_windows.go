//go:build windows

package main

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"gocraft/platform"
)

const (
	webGPUEntityMaxVertices = 32768
	webGPUEntityMaxIndices  = 49152
)

type webGPUEntityRenderer struct {
	queue        *wgpu.Queue
	vertexBuffer *wgpu.Buffer
	indexBuffer  *wgpu.Buffer
}

var activeWebGPUEntityRenderer *webGPUEntityRenderer

func ensureWebGPUEntityRenderer(worldRenderer *webGPUWorldRenderer) (*webGPUEntityRenderer, error) {
	if activeWebGPUEntityRenderer != nil {
		return activeWebGPUEntityRenderer, nil
	}
	if worldRenderer == nil || worldRenderer.device == nil || worldRenderer.queue == nil {
		return nil, fmt.Errorf("WebGPU world renderer is not ready for entity creation")
	}
	r := &webGPUEntityRenderer{queue: worldRenderer.queue}
	var err error
	r.vertexBuffer, err = worldRenderer.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft WebGPU entity vertices",
		Size:  uint64(webGPUEntityMaxVertices) * uint64(unsafe.Sizeof(platform.Vertex{})),
		Usage: gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create WebGPU entity vertex buffer: %w", err)
	}
	r.indexBuffer, err = worldRenderer.device.CreateBuffer(&wgpu.BufferDescriptor{
		Label: "GoCraft WebGPU entity indices",
		Size:  uint64(webGPUEntityMaxIndices) * 4,
		Usage: gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
	})
	if err != nil {
		r.vertexBuffer.Release()
		return nil, fmt.Errorf("create WebGPU entity index buffer: %w", err)
	}
	activeWebGPUEntityRenderer = r
	return r, nil
}

func closeWebGPUEntityRenderer() {
	if activeWebGPUEntityRenderer == nil {
		return
	}
	activeWebGPUEntityRenderer.Close()
	activeWebGPUEntityRenderer = nil
}

func (r *webGPUEntityRenderer) Close() {
	if r == nil {
		return
	}
	if r.indexBuffer != nil {
		r.indexBuffer.Release()
		r.indexBuffer = nil
	}
	if r.vertexBuffer != nil {
		r.vertexBuffer.Release()
		r.vertexBuffer = nil
	}
}

type webGPUEntityBatch struct {
	vertices []platform.Vertex
	indices  []uint32
}

func newWebGPUEntityBatch() *webGPUEntityBatch {
	return &webGPUEntityBatch{
		vertices: make([]platform.Vertex, 0, 4096),
		indices:  make([]uint32, 0, 6144),
	}
}

func webGPUEntityUV(path string) (float32, float32, float32, float32, bool) {
	if path == "" || assets == nil || assets.atlas == nil {
		return 0, 0, 0, 0, false
	}
	uv, ok := assets.atlas.UVs[path]
	if !ok {
		return 0, 0, 0, 0, false
	}
	return uv.X, uv.Y, uv.X + uv.Width, uv.Y + uv.Height, true
}

func webGPUEntitySubUV(path string, textureSize [2]float32, x, y, w, h float32) ([4]float32, bool) {
	u0, v0, u1, v1, ok := webGPUEntityUV(path)
	if !ok || textureSize[0] <= 0 || textureSize[1] <= 0 {
		return [4]float32{}, false
	}
	du, dv := u1-u0, v1-v0
	return [4]float32{
		u0 + du*x/textureSize[0],
		v0 + dv*y/textureSize[1],
		u0 + du*(x+w)/textureSize[0],
		v0 + dv*(y+h)/textureSize[1],
	}, true
}

func webGPUEntityItemTexture(id byte) string {
	def := GetItem(id)
	if def == nil {
		return ""
	}
	if def.PlaceBlock != 0 {
		block := GetBlock(def.PlaceBlock)
		if block != nil {
			return block.Textures.Top
		}
	}
	return def.Icon
}

func (b *webGPUEntityBatch) addQuad(p0, p1, p2, p3 [3]float32, uv [4]float32, color [4]uint8, normal [3]float32) {
	base := uint32(len(b.vertices))
	b.vertices = append(b.vertices,
		platform.Vertex{Position: p0, Texcoord: [2]float32{uv[0], uv[1]}, Color: color, Normal: normal},
		platform.Vertex{Position: p1, Texcoord: [2]float32{uv[2], uv[1]}, Color: color, Normal: normal},
		platform.Vertex{Position: p2, Texcoord: [2]float32{uv[2], uv[3]}, Color: color, Normal: normal},
		platform.Vertex{Position: p3, Texcoord: [2]float32{uv[0], uv[3]}, Color: color, Normal: normal},
	)
	b.indices = append(b.indices, base, base+1, base+2, base, base+2, base+3)
}

func rotateY(x, z, yaw float32) (float32, float32) {
	s, c := float32(math.Sin(float64(yaw))), float32(math.Cos(float64(yaw)))
	return x*c - z*s, x*s + z*c
}

func rotateXVec(v [3]float32, angle float32) [3]float32 {
	s, c := float32(math.Sin(float64(angle))), float32(math.Cos(float64(angle)))
	return [3]float32{v[0], v[1]*c - v[2]*s, v[1]*s + v[2]*c}
}

func rotateZVec(v [3]float32, angle float32) [3]float32 {
	s, c := float32(math.Sin(float64(angle))), float32(math.Cos(float64(angle)))
	return [3]float32{v[0]*c - v[1]*s, v[0]*s + v[1]*c, v[2]}
}

func addVec3(a, c [3]float32) [3]float32 {
	return [3]float32{a[0] + c[0], a[1] + c[1], a[2] + c[2]}
}

func (b *webGPUEntityBatch) addBox(cx, cy, cz, sx, sy, sz, yaw float32, uv [4]float32, color [4]uint8) {
	hx, hy, hz := sx/2, sy/2, sz/2
	point := func(x, y, z float32) [3]float32 {
		rx, rz := rotateY(x, z, yaw)
		return [3]float32{cx + rx, cy + y, cz + rz}
	}
	b.addQuad(point(-hx, hy, hz), point(hx, hy, hz), point(hx, -hy, hz), point(-hx, -hy, hz), uv, color, [3]float32{0, 0, 1})
	b.addQuad(point(hx, hy, -hz), point(-hx, hy, -hz), point(-hx, -hy, -hz), point(hx, -hy, -hz), uv, color, [3]float32{0, 0, -1})
	b.addQuad(point(-hx, hy, -hz), point(hx, hy, -hz), point(hx, hy, hz), point(-hx, hy, hz), uv, color, [3]float32{0, 1, 0})
	b.addQuad(point(-hx, -hy, hz), point(hx, -hy, hz), point(hx, -hy, -hz), point(-hx, -hy, -hz), uv, color, [3]float32{0, -1, 0})
	b.addQuad(point(hx, hy, hz), point(hx, hy, -hz), point(hx, -hy, -hz), point(hx, -hy, hz), uv, color, [3]float32{1, 0, 0})
	b.addQuad(point(-hx, hy, -hz), point(-hx, hy, hz), point(-hx, -hy, hz), point(-hx, -hy, -hz), uv, color, [3]float32{-1, 0, 0})
}

func (b *webGPUEntityBatch) addPlayer(e *RemoteEntity) {
	path := GetBlock(blockIronOre).Textures.Top
	u0, v0, u1, v1, ok := webGPUEntityUV(path)
	if !ok {
		return
	}
	uv := [4]float32{u0, v0, u1, v1}
	yaw := -e.Yaw * math.Pi / 180
	x, y, z := float32(e.X), float32(e.Y), float32(e.Z)
	b.addBox(x, y+0.70, z, 0.60, 0.80, 0.30, float32(yaw), uv, [4]uint8{95, 105, 112, 255})
	b.addBox(x, y+1.30, z, 0.50, 0.50, 0.50, float32(yaw), uv, [4]uint8{190, 198, 202, 255})
}

func (b *webGPUEntityBatch) addItem(e *RemoteEntity, now float32) {
	if e.Metadata == 0 {
		return
	}
	id := byte(e.Metadata & 0xFF)
	path := webGPUEntityItemTexture(id)
	u0, v0, u1, v1, ok := webGPUEntityUV(path)
	if !ok {
		return
	}
	bob := float32(math.Sin(float64(now*2.5))) * 0.10
	yaw := now * 1.57
	x, y, z := float32(e.X), float32(e.Y)+bob+0.25, float32(e.Z)
	uv := [4]float32{u0, v0, u1, v1}
	def := GetItem(id)
	if def != nil && def.PlaceBlock != 0 {
		b.addBox(x, y, z, 0.25, 0.25, 0.25, yaw, uv, [4]uint8{255, 255, 255, 255})
		return
	}
	// Flat standalone item sprites use two crossed quads so they remain visible
	// from most viewing angles without needing a separate billboard pipeline.
	h := float32(0.16)
	for _, angle := range []float32{yaw, yaw + math.Pi/2} {
		dx, dz := rotateY(h, 0, angle)
		p0 := [3]float32{x - dx, y + h, z - dz}
		p1 := [3]float32{x + dx, y + h, z + dz}
		p2 := [3]float32{x + dx, y - h, z + dz}
		p3 := [3]float32{x - dx, y - h, z - dz}
		b.addQuad(p0, p1, p2, p3, uv, [4]uint8{255, 255, 255, 255}, [3]float32{0, 0, 1})
		b.addQuad(p1, p0, p3, p2, uv, [4]uint8{255, 255, 255, 255}, [3]float32{0, 0, -1})
	}
}

type webGPUMobPose struct {
	joint  [3]float32
	angleX float32
}

func webGPUMobFaceUVs(model MobModel, bone MobBone) ([6][4]float32, bool) {
	w, h, d := bone.Size[0]*16, bone.Size[1]*16, bone.Size[2]*16
	if bone.UVAxis == "z" {
		h, d = d, h
	}
	u, v := bone.UV[0], bone.UV[1]
	regions := [6][4]float32{
		{u + d, v + d, w, h},         // front
		{u + 2*d + w, v + d, w, h},   // back
		{u + d, v, w, d},             // top
		{u + d + w, v, w, d},         // bottom
		{u, v + d, d, h},             // right
		{u + d + w, v + d, d, h},     // left
	}
	var out [6][4]float32
	for i, r := range regions {
		uv, ok := webGPUEntitySubUV(model.Texture, model.TextureSize, r[0], r[1], r[2], r[3])
		if !ok {
			return [6][4]float32{}, false
		}
		out[i] = uv
	}
	return out, true
}

func (b *webGPUEntityBatch) addMobBone(center, size [3]float32, angleX, yaw, deathAngle float32, root [3]float32, uvs [6][4]float32, color [4]uint8) {
	hx, hy, hz := size[0]/2, size[1]/2, size[2]/2
	point := func(x, y, z float32) [3]float32 {
		p := rotateXVec([3]float32{x, y, z}, angleX)
		p = addVec3(p, center)
		rx, rz := rotateY(p[0], p[2], yaw)
		p = [3]float32{rx, p[1], rz}
		p = rotateZVec(p, deathAngle)
		return addVec3(p, root)
	}
	b.addQuad(point(-hx, hy, hz), point(hx, hy, hz), point(hx, -hy, hz), point(-hx, -hy, hz), uvs[0], color, [3]float32{0, 0, 1})
	b.addQuad(point(hx, hy, -hz), point(-hx, hy, -hz), point(-hx, -hy, -hz), point(hx, -hy, -hz), uvs[1], color, [3]float32{0, 0, -1})
	b.addQuad(point(-hx, hy, -hz), point(hx, hy, -hz), point(hx, hy, hz), point(-hx, hy, hz), uvs[2], color, [3]float32{0, 1, 0})
	b.addQuad(point(-hx, -hy, hz), point(hx, -hy, hz), point(hx, -hy, -hz), point(-hx, -hy, -hz), uvs[3], color, [3]float32{0, -1, 0})
	b.addQuad(point(hx, hy, hz), point(hx, hy, -hz), point(hx, -hy, -hz), point(hx, -hy, hz), uvs[4], color, [3]float32{1, 0, 0})
	b.addQuad(point(-hx, hy, -hz), point(-hx, hy, hz), point(-hx, -hy, hz), point(-hx, -hy, -hz), uvs[5], color, [3]float32{-1, 0, 0})
}

func (b *webGPUEntityBatch) addMob(e *RemoteEntity) {
	definition, ok := mobContent.Definitions[e.MobKind]
	if !ok {
		return
	}
	model, ok := mobContent.Models[definition.Model]
	if !ok {
		return
	}
	anim := mobContent.Animations[definition.Animation]
	angles := make(map[string]float32, len(anim.Channels))
	for _, ch := range anim.Channels {
		angles[ch.Bone] = float32(math.Sin(float64(e.AnimPhase+ch.Phase))) * anim.Amplitude * e.AnimBlend
	}
	death := float32(0)
	if anim.DeathSeconds > 0 {
		death = min(e.DeathTime/anim.DeathSeconds, float32(1))
	}
	root := [3]float32{float32(e.X), float32(e.Y) + death*0.32, float32(e.Z)}
	deathAngle := death * float32(math.Pi/2)
	poses := make(map[string]webGPUMobPose, len(model.Bones))
	color := [4]uint8{255, 255, 255, 255}
	if e.MobHurt > 0 {
		color = [4]uint8{255, 115, 115, 255}
	}
	for _, bone := range model.Bones {
		parent := webGPUMobPose{}
		if bone.Parent != "" {
			parent = poses[bone.Parent]
		}
		pivot := rotateXVec(bone.Pivot, parent.angleX)
		joint := addVec3(parent.joint, pivot)
		angleX := parent.angleX + angles[bone.Name]
		center := addVec3(joint, rotateXVec(bone.Offset, angleX))
		poses[bone.Name] = webGPUMobPose{joint: joint, angleX: angleX}
		uvs, ok := webGPUMobFaceUVs(model, bone)
		if !ok {
			continue
		}
		b.addMobBone(center, bone.Size, angleX, e.Yaw, deathAngle, root, uvs, color)
	}
}

func (b *webGPUEntityBatch) addMobPlaceholder(e *RemoteEntity) {
	path := GetBlock(blockDirt).Textures.Top
	u0, v0, u1, v1, ok := webGPUEntityUV(path)
	if !ok {
		return
	}
	color := [4]uint8{210, 120, 125, 255}
	if e.MobHurt > 0 {
		color = [4]uint8{255, 70, 70, 255}
	}
	b.addBox(float32(e.X), float32(e.Y)+0.55, float32(e.Z), 0.9, 0.9, 1.25, e.Yaw, [4]float32{u0, v0, u1, v1}, color)
}

func (b *webGPUEntityBatch) addMiningCrack(state *InputState) {
	if state == nil || state.MiningTarget == nil || state.MiningProgress <= 0 {
		return
	}
	stage := int(state.MiningProgress * 10)
	stage = max(0, min(stage, 9))
	path := fmt.Sprintf("textures/block/destroy_stage_%d.png", stage)
	u0, v0, u1, v1, ok := webGPUEntityUV(path)
	if !ok {
		return
	}
	hit := state.MiningTarget
	b.addBox(float32(hit.x), float32(hit.y), float32(hit.z), 1.006, 1.006, 1.006, 0, [4]float32{u0, v0, u1, v1}, [4]uint8{120, 120, 120, 255})
}

func (r *webGPUEntityRenderer) Draw(pass *wgpu.RenderPassEncoder, worldRenderer *webGPUWorldRenderer, state *InputState, now float32) error {
	if r == nil || pass == nil || worldRenderer == nil {
		return nil
	}
	batch := newWebGPUEntityBatch()
	for id, e := range remoteEntities {
		if e == nil || id == *username {
			continue
		}
		switch {
		case e.Type == EntityPlayer:
			batch.addPlayer(e)
		case e.Type == EntityItem:
			batch.addItem(e, now)
		case e.MobKind != "":
			batch.addMob(e)
		default:
			batch.addMobPlaceholder(e)
		}
	}
	batch.addMiningCrack(state)
	if len(batch.vertices) == 0 || len(batch.indices) == 0 {
		return nil
	}
	if len(batch.vertices) > webGPUEntityMaxVertices || len(batch.indices) > webGPUEntityMaxIndices {
		return fmt.Errorf("WebGPU entity batch exceeded budget: %d vertices, %d indices", len(batch.vertices), len(batch.indices))
	}
	vertexBytes := unsafe.Slice((*byte)(unsafe.Pointer(&batch.vertices[0])), len(batch.vertices)*int(unsafe.Sizeof(platform.Vertex{})))
	if err := r.queue.WriteBuffer(r.vertexBuffer, 0, vertexBytes); err != nil {
		return fmt.Errorf("upload WebGPU entity vertices: %w", err)
	}
	indexBytes := unsafe.Slice((*byte)(unsafe.Pointer(&batch.indices[0])), len(batch.indices)*4)
	if err := r.queue.WriteBuffer(r.indexBuffer, 0, indexBytes); err != nil {
		return fmt.Errorf("upload WebGPU entity indices: %w", err)
	}
	pass.SetPipeline(worldRenderer.opaquePipeline)
	pass.SetBindGroup(0, worldRenderer.bindGroup, nil)
	pass.SetVertexBuffer(0, r.vertexBuffer, 0)
	pass.SetIndexBuffer(r.indexBuffer, gputypes.IndexFormatUint32, 0)
	pass.DrawIndexed(gputypes.DrawIndexedArgs{IndexCount: uint32(len(batch.indices)), InstanceCount: 1})
	return nil
}
