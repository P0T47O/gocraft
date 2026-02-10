package main

import (
	"gocraft/platform"
	"math"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"
	"github.com/go-gl/mathgl/mgl32"
)

type translucentDraw struct {
	chunk *Chunk
	dist  float32
	dx    int
	dz    int
}

type waterDraw struct {
	chunk *Chunk
	dist  float32
	dx    int
	dz    int
}

func (w *World) Draw(assets *RenderAssets, camera rl.Camera3D) {
	// Calculate MVP matrices manually for PureGL
	aspect := float32(rl.GetScreenWidth()) / float32(rl.GetScreenHeight())
	proj := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, 1000.0)

	camPos := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	camTarget := mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z}
	camUp := mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z}
	view := mgl32.LookAtV(camPos, camTarget, camUp)

	// Pre-calculate ViewProj Matrix (Optimization)
	viewProj := proj.Mul4(view)
	frustum := ExtractFrustum(viewProj)

	// Update Fog Shader Uniforms (if active)
	if assets.fogShader.ID != 0 {
		sid := assets.fogShader.ID
		platform.UseProgram(sid)

		skyColor := rl.NewColor(180, 210, 255, 255) // Nice sky blue
		platform.Uniform4f(platform.GetUniformLocation(sid, "fogColor"), float32(skyColor.R)/255.0, float32(skyColor.G)/255.0, float32(skyColor.B)/255.0, 1.0)

		fogDensity := float32(0.005)
		platform.Uniform1f(platform.GetUniformLocation(sid, "fogDensity"), fogDensity)
		platform.Uniform1i(platform.GetUniformLocation(sid, "fogMode"), 2) // Exp2

		platform.Uniform3f(platform.GetUniformLocation(sid, "viewPos"), camera.Position.X, camera.Position.Y, camera.Position.Z)

		// Critical: Set matModel to Identity because chunk vertices are already in world space.
		// If explicit matModel isn't set, it might default to 0 (all vertices collapse to origin for fog calc),
		// causing fog to increase as you walk away from 0,0,0.
		ident := mgl32.Ident4()
		platform.UniformMatrix4fv(platform.GetUniformLocation(sid, "matModel"), 1, false, &ident[0])
	}

	cx := int(math.Floor(float64(camera.Position.X) / 16.0))
	cz := int(math.Floor(float64(camera.Position.Z) / 16.0))
	playerSec := int(math.Floor(float64(camera.Position.Y) / 16.0))

	// Sorting logic
	renderRadius := 16
	type chunkItem struct {
		dx, dz int
		dist   float64
	}
	var items []chunkItem
	var backlog []chunkItem

	for dz := -renderRadius; dz <= renderRadius; dz++ {
		for dx := -renderRadius; dx <= renderRadius; dx++ {
			dist := float64(dx*dx + dz*dz)
			if dist > float64(renderRadius*renderRadius) {
				continue
			}
			item := chunkItem{dx, dz, dist}
			items = append(items, item)

			// Prioritize chunks closer to player or in backlog
			if dist <= 2*2 {
				// High priority, already in items
			} else {
				backlog = append(backlog, item)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].dist < items[j].dist
	})

	// meshBuildsUsed := 0
	// maxMeshBuildsPerFrame := 10
	// boostBuilds := false

	// Define helper capture function to include view/proj
	drawSection := func(dx, dz, secMin, secMax int) {
		chunk := w.getChunkIfGenerated(cx+dx, cz+dz)
		if chunk == nil {
			return
		}
		ensureChunkSections(chunk)
		baseX := (cx + dx) * chunkWidth
		baseZ := (cz + dz) * chunkWidth
		if secMin < 0 {
			secMin = 0
		}
		if secMax > sectionCount {
			secMax = sectionCount
		}
		for sec := secMin; sec < secMax; sec++ {
			// Frustum Culling
			yMin := sec * sectionHeight
			yMax := yMin + sectionHeight
			min := mgl32.Vec3{float32(baseX), float32(yMin), float32(baseZ)}
			max := mgl32.Vec3{float32(baseX + chunkWidth), float32(yMax), float32(baseZ + chunkWidth)}
			if !frustum.IntersectsAABB(min, max) {
				continue
			}

			// Skip broken chunks
			if chunk.meshRetries[sec] > 5 {
				continue
			}

			rebuildWater := (chunk.sectionDirty[sec] || chunk.waterMeshes[sec] == nil) && !chunk.pendingWater[sec]
			rebuildOpaque := (chunk.sectionDirty[sec] || chunk.opaqueMeshes[sec] == nil) && !chunk.pendingOpaque[sec]
			rebuildCutout := (chunk.sectionDirty[sec] || chunk.cutoutMeshes[sec] == nil) && !chunk.pendingCutout[sec]
			rebuildGlass := (chunk.sectionDirty[sec] || chunk.glassMeshes[sec] == nil) && !chunk.pendingGlass[sec]
			if rebuildWater || rebuildOpaque || rebuildCutout || rebuildGlass {
				var neighbors [3][3]*Chunk
				for ndx := -1; ndx <= 1; ndx++ {
					for ndz := -1; ndz <= 1; ndz++ {
						neighbors[ndx+1][ndz+1] = w.getChunkIfGenerated(cx+dx+ndx, cz+dz+ndz)
					}
				}
				job := meshJob{
					key:       chunkKey{X: cx + dx, Z: cz + dz},
					baseX:     baseX,
					baseZ:     baseZ,
					heightMap: chunk.heightMap,
					centerCX:  cx + dx,
					centerCZ:  cz + dz,
					neighbors: neighbors,
					section:   sec,
					yMin:      yMin,
					yMax:      yMax,
					version:   chunk.meshVersion[sec],
				}
				select {
				case w.meshJobs <- job:
					chunk.pendingOpaque[sec] = true
					chunk.pendingWater[sec] = true
					chunk.pendingCutout[sec] = true
					chunk.pendingGlass[sec] = true
				default:
					// If channel is full, we'll try again next frame.
				}
			}
			for path, meshes := range chunk.opaqueMeshes[sec] {
				var texID uint32
				if assets.isAnimated(path) {
					tex := assets.currentTexture(path)
					texID = tex.ID
				}
				for _, mesh := range meshes {
					mesh.Draw(mesh.material.Shader.ID, viewProj, texID)
				}
			}
			rl.DisableBackfaceCulling() // Grass overlay needs double-sided due to internal culling logic? Or just consistent state. But definitely Enable Offset.
			platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
			platform.PolygonOffset(-1.0, -1.0)
			for _, meshes := range chunk.cutoutMeshes[sec] {
				for _, mesh := range meshes {
					mesh.Draw(mesh.material.Shader.ID, viewProj, 0)
				}
			}
			platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
			rl.EnableBackfaceCulling()
		}
	}

	nearMin := playerSec - 1
	nearMax := playerSec + 2
	for ndx := -1; ndx <= 1; ndx++ {
		for ndz := -1; ndz <= 1; ndz++ {
			drawSection(ndx, ndz, nearMin, nearMax)
		}
	}
	for _, item := range items {
		drawSection(item.dx, item.dz, 0, sectionCount)
	}
	// Removed legacy backlog and budget logic.
	// Mesh requests are now handled by drawSection -> requestImmediateMesh.

	// Draw Torches (Raylib legacy, or needs porting? Keeps usage of assets.drawBlock which uses Raylib)
	// We'll leave it for now, it matches existing behavior.
	for _, item := range items {
		dx := item.dx
		dz := item.dz
		chunk := w.requestChunk(cx+dx, cz+dz)
		if !chunk.generated {
			continue
		}
		if chunk.torchCount == 0 {
			continue
		}
		baseX := (cx + dx) * chunkWidth
		baseZ := (cz + dz) * chunkWidth
		for x := 0; x < chunkWidth; x++ {
			for z := 0; z < chunkWidth; z++ {
				maxY := int(chunk.heightMap[x][z])
				for y := 0; y < maxY; y++ {
					block := chunk.blocks[x][y][z]
					if block != blockTorch {
						continue
					}
					worldX := baseX + x
					worldZ := baseZ + z
					pos := rl.NewVector3(float32(worldX), float32(y), float32(worldZ))
					assets.drawBlock(block, pos, w.BlockAt, w.LightAt, w.MetaAt, worldX, y, worldZ)
				}
			}
		}
	}

	// Transparent Pass (Water/Glass)
	// Needs to collect draws and sort by distance to camera
	waterDraws := w.waterDraws[:0]
	translucent := w.translucentDraws[:0]

	for _, item := range items {
		dx := item.dx
		dz := item.dz
		chunk := w.requestChunk(cx+dx, cz+dz)
		if !chunk.generated {
			continue
		}
		ensureChunkSections(chunk)

		// Check if we need to draw transparents
		hasWater := false
		hasGlass := false
		for sec := 0; sec < sectionCount; sec++ {
			if chunk.waterMeshes[sec] != nil {
				hasWater = true
			}
			if chunk.glassMeshes[sec] != nil {
				hasGlass = true
			}
		}

		if hasWater || hasGlass {
			centerX := float32((cx+dx)*chunkWidth + chunkWidth/2)
			centerZ := float32((cz+dz)*chunkWidth + chunkWidth/2)
			camDX := camera.Position.X - centerX
			camDZ := camera.Position.Z - centerZ
			dist := camDX*camDX + camDZ*camDZ

			if hasWater {
				waterDraws = append(waterDraws, waterDraw{chunk: chunk, dist: dist, dx: dx, dz: dz})
			}
			if hasGlass {
				translucent = append(translucent, translucentDraw{chunk: chunk, dist: dist, dx: dx, dz: dz})
			}
		}
	}

	w.waterDraws = waterDraws
	w.translucentDraws = translucent

	// Sort far to near
	sort.Slice(waterDraws, func(i, j int) bool { return waterDraws[i].dist > waterDraws[j].dist })
	sort.Slice(translucent, func(i, j int) bool { return translucent[i].dist > translucent[j].dist })

	// Draw Water
	rl.DisableDepthMask()
	platform.ActiveTexture(platform.GL_TEXTURE0)

	// Apply Polygon Offset to prevent Z-fighting with solid blocks (especially bottom faces if drawn)
	platform.Enable(platform.GL_POLYGON_OFFSET_FILL)
	platform.PolygonOffset(-1.0, -1.0)

	for _, item := range waterDraws {
		for sec := 0; sec < sectionCount; sec++ {
			// Sort keys for deterministic order to prevent z-fighting flicker
			paths := make([]string, 0, len(item.chunk.waterMeshes[sec]))
			for path := range item.chunk.waterMeshes[sec] {
				paths = append(paths, path)
			}
			sort.Strings(paths)

			for _, path := range paths {
				meshes := item.chunk.waterMeshes[sec][path]
				var texID uint32
				if assets.isAnimated(path) {
					// Remove optimization: Always update texture for now to ensure animation works
					// near := absInt(item.dx) <= 2 && absInt(item.dz) <= 2
					tex := assets.currentTexture(path)
					texID = tex.ID
				}
				for _, mesh := range meshes {
					mesh.Draw(mesh.material.Shader.ID, viewProj, texID)
				}
			}
		}
	}
	platform.Disable(platform.GL_POLYGON_OFFSET_FILL)
	rl.EnableDepthMask() // Reset for safety, though disabled again below if needed
	// Actually Glass needs it Disabled too.
	rl.DisableDepthMask()
	rl.EnableBackfaceCulling()
	// Keep DepthMask Disabled for Glass loop which follows...

	// Draw Glass
	// DepthMask is disabled from above.
	rl.DisableBackfaceCulling() // Often used for glass
	for _, item := range translucent {
		for sec := 0; sec < sectionCount; sec++ {
			for _, meshes := range item.chunk.glassMeshes[sec] {
				for _, mesh := range meshes {
					mesh.Draw(mesh.material.Shader.ID, viewProj, 0)
				}
			}
		}
	}
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
	platform.UseProgram(0) // Reset shader state to avoid interfering with Raylib
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
