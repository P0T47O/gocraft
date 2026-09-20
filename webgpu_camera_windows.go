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
	projection := mgl32.Perspective(mgl32.DegToRad(camera.Fovy), aspect, 0.01, 1000.0)
	eye := mgl32.Vec3{camera.Position.X, camera.Position.Y, camera.Position.Z}
	view := mgl32.LookAtV(
		eye,
		mgl32.Vec3{camera.Target.X, camera.Target.Y, camera.Target.Z},
		mgl32.Vec3{camera.Up.X, camera.Up.Y, camera.Up.Z},
	)
	return projection, view, eye
}

func (r *webGPUWorldRenderer) updateSceneCamera(camera webGPUCamera) error {
	r.camera = camera
	projection, view, eye := webGPUCameraMatrices(camera, r.width, r.height)
	clipCorrection := mgl32.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 0.5, 0,
		0, 0, 0.5, 1,
	}
	vp := clipCorrection.Mul4(projection).Mul4(view)
	fogStart, fogEnd := worldFogRange(renderDistance())
	fogColor := [4]float32{180.0 / 255.0, 210.0 / 255.0, 1.0, 1.0}

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
	put(88, 0)
	put(92, 0)
	for i, value := range fogColor {
		put(96+i*4, value)
	}
	return r.queue.WriteBuffer(r.sceneBuffer, 0, bytes)
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

	deadline := time.Now().Add(time.Millisecond)
	submissions := 0
	for _, section := range cache.visible {
		if submissions == 8 || !time.Now().Before(deadline) {
			break
		}
		if section.chunk.meshRetries[section.sec] <= 5 && world.submitMesh(section.cx, section.cz, section.sec, assets) {
			submissions++
		}
	}
	cache.collectTranslucent()
}
