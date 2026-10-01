//go:build windows

package main

import (
	"encoding/binary"
	"math"
	"slices"
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

func webGPUCameraMatrices(camera webGPUCamera, width, height uint32) (mgl32.Mat4, mgl32.Mat4, mgl32.Vec3) {
	aspect := float32(max(uint32(1), width)) / float32(max(uint32(1), height))
	projection := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, worldFarPlane(max(renderDistance(), horizonDistance())))
	eye := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	view := mgl32.LookAtV(
		eye,
		mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z},
		mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z},
	)
	return projection, view, eye
}

func (r *webGPUWorldRenderer) updateSceneCamera(camera webGPUCamera) error {
	return r.updateSceneCameraAtTime(camera, 6000)
}

func (r *webGPUWorldRenderer) updateSceneCameraAtTime(camera webGPUCamera, ticks float64) error {
	r.camera = camera
	projection, view, eye := webGPUCameraMatrices(camera, r.width, r.height)
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, -0.5, 0,
		0, 0, 0.5, 1,
	}
	// Reverse the [0,1] depth range. Together with a floating-point depth
	// attachment this preserves separation between distant water and terrain.
	vp := clipCorrection.Mul4(projection).Mul4(view)
	fogStart, fogEnd := worldFogRange(max(renderDistance(), horizonDistance()))
	daylight := worldDaylight(ticks)
	r.sceneSky = daylight.Sky

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
	// Independent environmental fog disabled in clear air. Do not couple it
	// to render distance; water/weather can supply their own range later.
	put(88, 0)
	put(92, 0)
	for i, value := range daylight.Sky {
		put(96+i*4, value)
	}
	put(108, 1)
	put(112, daylight.Brightness)
	put(116, float32(max(0, renderDistance()-2)*chunkWidth))
	return r.queue.WriteBuffer(r.sceneBuffer, 0, bytes)
}

// Covers the entire legal 128-chunk render radius, including its positive edge.
const lodChunkMaskSide = 2*maxRenderDistance + 2
const lodChunkMaskWords = (lodChunkMaskSide*lodChunkMaskSide + 127) / 128

// Keep the fallback only in chunk columns whose real opaque mesh is not ready.
// The mask follows the camera and covers the playable near field, including
// newly excavated holes. Cached chunks outside the real draw disk never own LOD.
func lodChunkMask(world *World, camera webGPUCamera) [16 + lodChunkMaskWords*16]byte {
	var data [16 + lodChunkMaskWords*16]byte
	centerX := int(math.Floor(float64(camera.Position.X) / chunkWidth))
	centerZ := int(math.Floor(float64(camera.Position.Z) / chunkWidth))
	originX := centerX - lodChunkMaskSide/2
	originZ := centerZ - lodChunkMaskSide/2
	radius := renderDistance()
	binary.LittleEndian.PutUint32(data[0:], uint32(int32(originX)))
	binary.LittleEndian.PutUint32(data[4:], uint32(int32(originZ)))
	world.chunksMu.RLock()
	defer world.chunksMu.RUnlock()
	for key, chunk := range world.chunks {
		x, z := key.X-originX, key.Z-originZ
		dx, dz := key.X-centerX, key.Z-centerZ
		if x < 0 || z < 0 || x >= lodChunkMaskSide || z >= lodChunkMaskSide || dx*dx+dz*dz > radius*radius || !chunk.generated {
			continue
		}
		if chunk.lodOccludes {
			bit := z*lodChunkMaskSide + x
			data[16+bit/8] |= 1 << (bit % 8)
		}
	}
	return data
}

func (r *webGPUWorldRenderer) updateLODChunkMask(world *World, camera webGPUCamera) error {
	data := lodChunkMask(world, camera)
	return r.queue.WriteBuffer(r.sceneBuffer, 128, data[:])
}

func (r *webGPUWorldRenderer) collectVisibleCamera(world *World, camera webGPUCamera) {
	projection, view, camPos := webGPUCameraMatrices(camera, r.width, r.height)
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
	cache.visibleSections, cache.readySections = len(cache.visible), 0
	for _, section := range cache.visible {
		if !section.chunk.sectionDirty[section.sec] {
			cache.readySections++
		}
	}

	deadline := time.Now().Add(2 * time.Millisecond)
	submissions := 0
	visibleWaiting := false
	for _, section := range cache.visible {
		if !section.chunk.sectionDirty[section.sec] || section.chunk.pendingOpaque[section.sec] || section.chunk.meshRetries[section.sec] > 5 {
			continue
		}
		if submissions == 16 || !time.Now().Before(deadline) {
			visibleWaiting = true
			break
		}
		if section.chunk.meshRetries[section.sec] <= 5 && world.submitMesh(section.cx, section.cz, section.sec, assets) {
			submissions++
		}
	}
	if !visibleWaiting {
		world.prewarmMeshes(cx, cz, renderDistance(), assets, time.Now().Add(time.Millisecond))
	}
	cache.collectTranslucent()
}
