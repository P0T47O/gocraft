//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/wgpu"
)

// Capture the bed through the normal chunk mesher and WebGPU world pipelines.
// The adjacent full cube is a height/size reference, not part of the bed.
func TestWebGPUBedPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_BED_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_BED_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t)
	defer cleanup()
	chunk := &Chunk{}
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			chunk.blocks.Set(x, 0, z, blockGrass)
			chunk.heightMap[x][z] = 1
		}
	}
	chunk.blocks.Set(8, 1, 8, blockBed)
	chunk.meta.Set(8, 1, 8, faceSouth)
	chunk.blocks.Set(8, 1, 9, blockBed)
	chunk.meta.Set(8, 1, 9, faceSouth|shapeUpper)
	chunk.blocks.Set(11, 1, 8, blockStone)
	chunk.heightMap[8][8] = 2
	chunk.heightMap[8][9] = 2
	chunk.heightMap[11][8] = 2
	initializeChunkLighting(chunk)
	blockAt := func(x, y, z int) byte {
		if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
			return blockAir
		}
		return chunk.blocks.Get(x, y, z)
	}
	metaAt := func(x, y, z int) byte {
		if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
			return 0
		}
		return chunk.meta.Get(x, y, z)
	}
	results := assets.buildAllMeshDataWithLight(&chunk.heightMap, 0, 0, 0, 4, blockAt,
		func(int, int, int) byte { return 15 }, func(int, int, int) byte { return 0 }, metaAt, 42)
	opaque := assets.applyMeshData(results["opaque"])
	cutout := assets.applyMeshData(results["cutout"])
	releaseMeshResults(map[string]map[string][]*MeshBuildData{"glass": results["glass"], "water": results["water"]})
	defer func() {
		for _, meshes := range []map[string][]*ChunkMesh{opaque, cutout} {
			for _, list := range meshes {
				for _, mesh := range list {
					mesh.unload()
				}
			}
		}
	}()
	if len(cutout["atlas"]) == 0 {
		t.Fatal("bed did not enter the cutout mesh")
	}
	draw := func(pass *wgpu.RenderPassEncoder) error {
		pass.SetPipeline(r.solidPipeline)
		pass.SetBindGroup(0, r.bindGroup, nil)
		cache := &worldRenderCache{}
		if err := r.drawMeshMap(pass, opaque, cache); err != nil {
			return err
		}
		pass.SetPipeline(r.opaquePipeline)
		return r.drawMeshMap(pass, cutout, cache)
	}
	for _, view := range []struct {
		name   string
		camera webGPUCamera
	}{
		{"front", webGPUCamera{Position: webGPUVec3{X: 8, Y: 3.8, Z: 13}, Target: webGPUVec3{X: 8.8, Y: 1, Z: 8}, Up: webGPUVec3{Y: 1}, Fovy: 42}},
		{"side", webGPUCamera{Position: webGPUVec3{X: 13, Y: 3.8, Z: 8}, Target: webGPUVec3{X: 8.8, Y: 1, Z: 8}, Up: webGPUVec3{Y: 1}, Fovy: 42}},
	} {
		if err := r.updateSceneCameraAtTime(view.camera, 6000); err != nil {
			t.Fatal(err)
		}
		img := captureNativePreview(t, r, 1280, 720, draw)
		path := filepath.Join("work", "bed-preview-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("bed %s: %s", view.name, path)
	}
	state := &InputState{Hotbar: [9]byte{blockBed}}
	hud, err := ensureWebGPUHUDRenderer(r)
	if err != nil {
		t.Fatal(err)
	}
	img := captureNativePreview(t, r, 1280, 720, func(pass *wgpu.RenderPassEncoder) error {
		if err := draw(pass); err != nil {
			return err
		}
		return hud.Draw(pass, 1280, 720, state)
	})
	saveNativePreview(t, img, filepath.Join("work", "bed-preview-hotbar.png"))
}
