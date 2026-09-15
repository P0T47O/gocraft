package main

import (
	"cmp"
	"gocraft/platform"
	"math"
	"slices"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

const worldRenderRadius = defaultRenderDistance

// Leave most of the view clear, with a soft fade before the circular chunk
// boundary. The margin covers chunk-grid rounding and camera motion in a chunk.
func worldFogRange(radius int) (start, end float32) {
	distance := float32(max(radius, 2) * chunkWidth)
	return distance * 0.70, distance - min(1.5*float32(chunkWidth), distance*0.15)
}

// World owns this cache through its render worldRenderCache field. All scratch
// slices retain capacity, but release chunk references at the end of each frame.
type worldRenderCache struct {
	drawCalls, triangles int
	offsets              []chunkItem
	radius               int
	visible              []visibleSection
	translucent          []translucentDraw
	paths                []string
	state                meshRenderState
}

type visibleSection struct {
	chunk       *Chunk
	cx, cz, sec int
	dist        float32
}

type translucentDraw struct {
	section visibleSection
	glass   bool
}

func (r *worldRenderCache) radiusOffsets(radius int) []chunkItem {
	if r.offsets != nil && r.radius == radius {
		return r.offsets
	}
	r.radius = radius
	r.offsets = r.offsets[:0]
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			dist := dx*dx + dz*dz
			if dist <= radius*radius {
				r.offsets = append(r.offsets, chunkItem{dx: dx, dz: dz, dist: float64(dist)})
			}
		}
	}
	slices.SortFunc(r.offsets, func(a, b chunkItem) int {
		if order := cmp.Compare(a.dist, b.dist); order != 0 {
			return order
		}
		if order := cmp.Compare(a.dz, b.dz); order != 0 {
			return order
		}
		return cmp.Compare(a.dx, b.dx)
	})
	return r.offsets
}

// Explicit coordinate ties give deterministic ordering even when distance ties
// change with camera movement; they do not depend on map iteration order.
func compareVisibleSections(a, b visibleSection) int {
	if order := cmp.Compare(a.dist, b.dist); order != 0 {
		return order
	}
	return compareSectionCoordinates(a, b)
}

func compareSectionCoordinates(a, b visibleSection) int {
	if order := cmp.Compare(a.cz, b.cz); order != 0 {
		return order
	}
	if order := cmp.Compare(a.cx, b.cx); order != 0 {
		return order
	}
	return cmp.Compare(a.sec, b.sec)
}

func compareTranslucentDraws(a, b translucentDraw) int {
	if order := cmp.Compare(b.section.dist, a.section.dist); order != 0 {
		return order
	}
	if order := compareSectionCoordinates(a.section, b.section); order != 0 {
		return order
	}
	if a.glass == b.glass {
		return 0
	}
	if a.glass {
		return 1
	}
	return -1 // Water precedes glass only within the same section/distance.
}

func (r *worldRenderCache) appendVisibleSections(chunk *Chunk, chunkX, chunkZ int, frustum *Frustum, camPos mgl32.Vec3) {
	for sec := 0; sec < sectionCount; sec++ {
		if chunk.sectionBlocks[sec] == 0 {
			continue
		}
		// Blocks are centered at integer positions, so bounds extend half a block
		// beyond the first center (including torches and translucent geometry).
		min := mgl32.Vec3{float32(chunkX*chunkWidth) - 0.5, float32(sec*sectionHeight) - 0.5, float32(chunkZ*chunkWidth) - 0.5}
		max := min.Add(mgl32.Vec3{chunkWidth, sectionHeight, chunkWidth})
		if !frustum.IntersectsAABB(min, max) {
			continue
		}
		center := min.Add(max).Mul(0.5)
		delta := camPos.Sub(center)
		r.visible = append(r.visible, visibleSection{chunk: chunk, cx: chunkX, cz: chunkZ, sec: sec, dist: delta.Dot(delta)})
	}
}

func (r *worldRenderCache) collectTranslucent() {
	clear(r.translucent)
	r.translucent = r.translucent[:0]
	for _, section := range r.visible {
		if len(section.chunk.waterMeshes[section.sec]) != 0 {
			r.translucent = append(r.translucent, translucentDraw{section: section})
		}
		if len(section.chunk.glassMeshes[section.sec]) != 0 {
			r.translucent = append(r.translucent, translucentDraw{section: section, glass: true})
		}
	}
	slices.SortFunc(r.translucent, compareTranslucentDraws)
}

func (r *worldRenderCache) drawMeshes(meshes map[string][]*ChunkMesh, assets *RenderAssets, viewProj mgl32.Mat4) {
	if len(meshes) == 0 {
		return
	}
	paths := r.paths[:0]
	for path, list := range meshes {
		if len(list) != 0 {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	for _, path := range paths {
		var texture uint32
		if assets.isAnimated(path) {
			texture = assets.currentTexture(path).ID
		}
		for _, mesh := range meshes[path] {
			if mesh == nil {
				continue
			}
			r.state.draw(mesh, mesh.material.Shader.ID, viewProj, texture)
			if mesh.glMesh != nil {
				r.drawCalls++
				r.triangles += int(mesh.glMesh.IndexCount()) / 3
			}
		}
	}
	clear(paths)
	r.paths = paths[:0]
}

func (w *World) Draw(assets *RenderAssets, camera rl.Camera3D) {
	aspect := float32(rl.GetScreenWidth()) / float32(rl.GetScreenHeight())
	proj := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, 1000.0)
	camPos := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	view := mgl32.LookAtV(camPos, mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z}, mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z})
	viewProj := proj.Mul4(view)
	frustum := ExtractFrustum(viewProj)

	// Flush Raylib before issuing direct GL commands and invalidate cached bindings.
	rl.DrawRenderBatchActive()
	r := &w.render
	r.drawCalls, r.triangles = 0, 0
	r.state.reset()
	// Update Fog Shader Uniforms (if active)
	if assets.fogShader.ID != 0 {
		sid := assets.fogShader.ID
		platform.UseProgram(sid)

		skyColor := rl.NewColor(180, 210, 255, 255) // Nice sky blue
		platform.Uniform4f(platform.GetUniformLocation(sid, "fogColor"), float32(skyColor.R)/255.0, float32(skyColor.G)/255.0, float32(skyColor.B)/255.0, 1.0)

		fogStart, fogEnd := worldFogRange(renderDistance())
		platform.Uniform1f(platform.GetUniformLocation(sid, "fogStart"), fogStart)
		platform.Uniform1f(platform.GetUniformLocation(sid, "fogEnd"), fogEnd)

		platform.Uniform3f(platform.GetUniformLocation(sid, "viewPos"), camera.Position.X, camera.Position.Y, camera.Position.Z)

		// Critical: Set matModel to Identity because chunk vertices are already in world space.
		// If explicit matModel isn't set, it might default to 0 (all vertices collapse to origin for fog calc),
		// causing fog to increase as you walk away from 0,0,0.
		ident := mgl32.Ident4()
		platform.UniformMatrix4fv(platform.GetUniformLocation(sid, "matModel"), 1, false, &ident[0])
	}

	cx := int(math.Floor(float64(camera.Position.X) / float64(chunkWidth)))
	cz := int(math.Floor(float64(camera.Position.Z) / float64(chunkWidth)))
	r.visible = r.visible[:0]
	for _, offset := range r.radiusOffsets(renderDistance()) {
		chunkX, chunkZ := cx+offset.dx, cz+offset.dz
		chunk := w.getChunkIfGenerated(chunkX, chunkZ)
		if chunk == nil {
			continue
		}
		ensureChunkSections(chunk)
		r.appendVisibleSections(chunk, chunkX, chunkZ, &frustum, camPos)
	}
	slices.SortFunc(r.visible, compareVisibleSections)

	// Bound submission work separately from drawing. submitMesh owns eligibility,
	// pending flags, and nonblocking queue handling; only accepted jobs count.
	deadline := time.Now().Add(time.Millisecond)
	submissions := 0
	for _, section := range r.visible {
		if submissions == 8 || !time.Now().Before(deadline) {
			break
		}
		if section.chunk.meshRetries[section.sec] <= 5 && w.submitMesh(section.cx, section.cz, section.sec, assets) {
			submissions++
		}
	}

	for _, section := range r.visible {
		r.drawMeshes(section.chunk.opaqueMeshes[section.sec], assets, viewProj)
	}
	rl.DisableBackfaceCulling()
	platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
	platform.PolygonOffset(-1, -1)
	for _, section := range r.visible {
		r.drawMeshes(section.chunk.cutoutMeshes[section.sec], assets, viewProj)
	}
	platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
	rl.EnableBackfaceCulling()

	// Raylib owns torch drawing. Flush it before resuming the cached direct-GL
	// renderer, since the batch may change the program and texture bindings.
	platform.UseProgram(0)
	for _, section := range r.visible {
		for _, torch := range section.chunk.torches {
			if torch.y/sectionHeight != section.sec {
				continue
			}
			worldX, worldZ := section.cx*chunkWidth+torch.x, section.cz*chunkWidth+torch.z
			pos := rl.NewVector3(float32(worldX), float32(torch.y), float32(worldZ))
			assets.drawBlock(blockTorch, pos, w.BlockAt, w.LightAt, w.MetaAt, worldX, torch.y, worldZ)
		}
	}
	rl.DrawRenderBatchActive()
	r.state.reset()

	r.collectTranslucent()
	rl.DisableDepthMask()
	for _, item := range r.translucent {
		section := item.section
		if item.glass {
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
			rl.DisableBackfaceCulling()
			r.drawMeshes(section.chunk.glassMeshes[section.sec], assets, viewProj)
		} else {
			rl.EnableBackfaceCulling()
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1, -1)
			r.drawMeshes(section.chunk.waterMeshes[section.sec], assets, viewProj)
		}
	}
	platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
	platform.UseProgram(0)
	r.state.reset()
	clear(r.visible)
	r.visible = r.visible[:0]
	clear(r.translucent)
	r.translucent = r.translucent[:0]
}

// DrawBlockCrack renders the mining progress crack overlay
func (w *World) DrawBlockCrack(assets *RenderAssets, camera rl.Camera3D, input *InputState) {
	if input.MiningTarget == nil || input.MiningProgress <= 0 {
		return
	}

	// Calculate stage (0 to 9)
	stage := int(input.MiningProgress * 10.0)
	if stage < 0 {
		stage = 0
	}
	if stage > 9 {
		stage = 9
	}

	tex := assets.CrackTextures[stage]
	if tex.ID == 0 {
		return
	}

	hit := input.MiningTarget
	pos := rl.NewVector3(float32(hit.x), float32(hit.y), float32(hit.z))

	// Scale up slightly to avoid z-fighting
	scale := float32(1.002)
	// Block centers are at integer coordinates (e.g. 0,0,0 for block 0).
	// So we draw centered at pos.
	drawPos := pos

	rl.BeginMode3D(camera)

	// Use the pre-generated unit cube model
	// We need to set the texture on the model's material
	materials := assets.crackModel.GetMaterials()
	if len(materials) > 0 {
		rl.SetMaterialTexture(&materials[0], rl.MapDiffuse, tex)
	}

	platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
	platform.PolygonOffset(-2.0, -2.0)

	// DrawModel handles texture binding correctly
	// Tint with DarkGray to make cracks dark (if texture is white) or just opaque
	rl.DrawModel(assets.crackModel, drawPos, scale, rl.Fade(rl.DarkGray, 0.8))

	platform.Disable(platform.GL_POLYGON_OFFSET_FILL)

	rl.EndMode3D()
}
