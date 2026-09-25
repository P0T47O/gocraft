//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/wgpu"
)

// TestWebGPUFoliagePreview renders the actual block mesher, atlas, mip chain,
// cutout shader and depth pipeline to repeatable near/far PNGs. It is opt-in
// because it needs a real WebGPU adapter and writes diagnostic images.
func TestWebGPUFoliagePreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_FOLIAGE_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_FOLIAGE_PREVIEW=1 to capture foliage")
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
	for _, tree := range []treeAnchor{
		{x: 3, y: 1, z: 8, height: 5, log: blockLog, leaves: blockLeaves},
		{x: 8, y: 1, z: 8, height: 5, log: blockLogBirch, leaves: blockLeavesBirch},
		{x: 13, y: 1, z: 8, height: 7, log: blockLogSpruce, leaves: blockLeavesSpruce, spruce: true},
	} {
		tree.emit(func(x, y, z int, block byte) {
			if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
				return
			}
			chunk.blocks.Set(x, y, z, block)
			chunk.heightMap[x][z] = max(chunk.heightMap[x][z], int16(y+1))
		})
	}
	chunk.blocks.Set(8, 1, 12, blockTorch)
	chunk.heightMap[8][12] = 2
	initializeChunkLighting(chunk)
	getBlock := func(x, y, z int) byte {
		if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
			return blockAir
		}
		return chunk.blocks.Get(x, y, z)
	}
	getLight := func(x, y, z int) byte {
		if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
			return 15
		}
		return max(chunk.skyLight.Get(x, y, z), chunk.blockLight.Get(x, y, z))
	}
	results := assets.buildAllMeshDataWithLight(&chunk.heightMap, 0, 0, 0, 11, getBlock, getLight,
		func(x, y, z int) byte {
			if x < 0 || x >= chunkWidth || z < 0 || z >= chunkWidth || y < 0 || y >= chunkHeight {
				return 0
			}
			return chunk.blockLight.Get(x, y, z)
		}, func(int, int, int) byte { return 0 }, 42)
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
		t.Fatal("foliage produced no atlas cutout mesh")
	}

	const width, height = uint32(1280), uint32(720)
	for _, view := range []struct {
		name   string
		camera webGPUCamera
		ticks  float64
	}{
		{"near", webGPUCamera{Position: webGPUVec3{8, 8, 20}, Target: webGPUVec3{8, 4, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55}, 6000},
		{"far", webGPUCamera{Position: webGPUVec3{8, 13, 48}, Target: webGPUVec3{8, 4, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55}, 6000},
		{"night", webGPUCamera{Position: webGPUVec3{8, 8, 20}, Target: webGPUVec3{8, 4, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55}, 18000},
	} {
		if err := r.updateSceneCameraAtTime(view.camera, view.ticks); err != nil {
			t.Fatal(err)
		}
		img := captureNativePreview(t, r, width, height, func(pass *wgpu.RenderPassEncoder) error {
			pass.SetPipeline(r.solidPipeline)
			pass.SetBindGroup(0, r.bindGroup, nil)
			cache := &worldRenderCache{}
			if err := r.drawMeshMap(pass, opaque, cache); err != nil {
				return err
			}
			pass.SetPipeline(r.opaquePipeline)
			return r.drawMeshMap(pass, cutout, cache)
		})
		foliagePixels := 0
		sky := img.RGBAAt(0, 0)
		for y := 0; y < int(height)/2; y++ {
			for x := int(width) / 3; x < int(width)*2/3; x++ {
				i := y*img.Stride + x*4
				if img.Pix[i] != sky.R || img.Pix[i+1] != sky.G || img.Pix[i+2] != sky.B {
					foliagePixels++
				}
			}
		}
		if foliagePixels < 500 {
			t.Fatalf("%s foliage preview has no canopy (%d colored pixels)", view.name, foliagePixels)
		}
		path := filepath.Join("work", "foliage-"+view.name+".png")
		saveNativePreview(t, img, path)
		t.Logf("%s: %s (%d canopy-region pixels, format %v)", view.name, path, foliagePixels, r.format)
	}
}
