//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/wgpu"
)

// Opt-in GPU acceptance image for shaped blocks and their actual hotbar icons.
// The fixture owns the native window, atlas, GPU and output under ignored work/.
func TestWebGPUShapedBlocksPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_SHAPES_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_SHAPES_PREVIEW=1")
	}
	samples := uint32(1)
	if os.Getenv("GOCRAFT_WEBGPU_MSAA_PREVIEW") == "1" {
		samples = 4
	}
	r, cleanup := nativePreviewFixture(t, samples)
	defer cleanup()
	if _, ok := assets.atlas.UVs[GetItem(blockWoodDoor).Icon]; !ok {
		t.Fatal("oak door item sprite missing from GPU atlas")
	}
	chunk := &Chunk{}
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			chunk.blocks.Set(x, 0, z, blockGrass)
			chunk.heightMap[x][z] = 1
		}
	}
	put := func(x, y, z int, block, meta byte) {
		chunk.blocks.Set(x, y, z, block)
		chunk.meta.Set(x, y, z, meta)
		chunk.heightMap[x][z] = max(chunk.heightMap[x][z], int16(y+1))
	}
	put(3, 1, 8, blockOakSlab, 0)
	put(5, 1, 8, blockStoneSlab, shapeUpper)
	put(7, 1, 8, blockOakStairs, faceSouth)
	put(7, 1, 9, blockOakStairs, faceEast) // Inner corner seen from the camera.
	put(8, 1, 8, blockOakStairs, faceSouth)
	put(10, 1, 8, blockCobbleStairs, faceSouth|shapeUpper)
	put(12, 1, 8, blockWoodDoor, faceNorth)
	put(12, 2, 8, blockWoodDoor, faceNorth|shapeUpper)
	put(14, 1, 8, blockWoodDoor, faceEast|shapeDouble)
	put(14, 2, 8, blockWoodDoor, faceEast|shapeDouble|shapeUpper)
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
		t.Fatal("transparent door was not emitted in the cutout pass")
	}
	if err := r.updateSceneCameraAtTime(webGPUCamera{Position: webGPUVec3{8, 6, 20}, Target: webGPUVec3{8, 1, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55}, 6000); err != nil {
		t.Fatal(err)
	}
	state := &InputState{Hotbar: [9]byte{blockOakSlab, blockStoneSlab, blockOakStairs, blockCobbleStairs, blockWoodDoor}}
	const width, height = uint32(1280), uint32(720)
	draw := func(pass *wgpu.RenderPassEncoder) error {
		pass.SetPipeline(r.solidPipeline)
		pass.SetBindGroup(0, r.bindGroup, nil)
		cache := &worldRenderCache{}
		if err := r.drawMeshMap(pass, opaque, cache); err != nil {
			return err
		}
		pass.SetPipeline(r.opaquePipeline)
		if err := r.drawMeshMap(pass, cutout, cache); err != nil {
			return err
		}
		if r.worldSamples > 1 {
			return nil // UI has a separate single-sample pass in live gameplay.
		}
		hud, err := ensureWebGPUHUDRenderer(r)
		if err != nil {
			return err
		}
		return hud.Draw(pass, width, height, state)
	}
	img := captureNativePreview(t, r, width, height, draw)
	name := "shaped-blocks.png"
	if r.worldSamples > 1 {
		name = "shaped-blocks-msaa4.png"
	}
	path := filepath.Join("work", name)
	saveNativePreview(t, img, path)
	t.Logf("shaped block preview: %s", path)
	if os.Getenv("GOCRAFT_WEBGPU_FILTER_DIAG") == "1" {
		for _, variant := range []struct {
			name string
			mips bool
			af   int
		}{
			{"mip-on-af1", true, 1},
			{"mip-off-af1", false, 1},
			{"mip-off-af8", false, 8},
		} {
			currentSettings.Mipmaps, currentSettings.Anisotropy = variant.mips, variant.af
			if err := r.updateFiltering(); err != nil {
				t.Fatal(err)
			}
			other := captureNativePreview(t, r, width, height, draw)
			saveNativePreview(t, other, filepath.Join("work", "filter-"+variant.name+".png"))
			changed, totalDifference := 0, 0
			// Near grass only: fixed camera, no foliage or HUD in this crop.
			for y := 470; y < 640; y++ {
				for x := 180; x < 1100; x++ {
					i := y*img.Stride + x*4
					diff := 0
					for channel := 0; channel < 3; channel++ {
						a, b := int(img.Pix[i+channel]), int(other.Pix[i+channel])
						diff += max(a-b, b-a)
					}
					if diff > 3 {
						changed++
					}
					totalDifference += diff
				}
			}
			t.Logf("%s: changed near pixels %d/%d, mean RGB absolute delta %.3f", variant.name, changed, 920*170, float64(totalDifference)/float64(920*170*3))
		}
		currentSettings.Mipmaps, currentSettings.Anisotropy = true, 8
		if err := r.updateFiltering(); err != nil {
			t.Fatal(err)
		}
		if err := r.updateSceneCameraAtTime(webGPUCamera{Position: webGPUVec3{8, 12, 30}, Target: webGPUVec3{8, 1, 8}, Up: webGPUVec3{0, 1, 0}, Fovy: 55}, 6000); err != nil {
			t.Fatal(err)
		}
		farBase := captureNativePreview(t, r, width, height, draw)
		saveNativePreview(t, farBase, filepath.Join("work", "filter-far-mip-on-af8.png"))
		for _, variant := range []struct {
			name string
			mips bool
			af   int
		}{
			{"mip-on-af1", true, 1},
			{"mip-off-af1", false, 1},
			{"mip-off-af8", false, 8},
		} {
			currentSettings.Mipmaps, currentSettings.Anisotropy = variant.mips, variant.af
			if err := r.updateFiltering(); err != nil {
				t.Fatal(err)
			}
			other := captureNativePreview(t, r, width, height, draw)
			saveNativePreview(t, other, filepath.Join("work", "filter-far-"+variant.name+".png"))
			changed := 0
			for y := 160; y < 630; y++ {
				for x := 0; x < int(width); x++ {
					i := y*farBase.Stride + x*4
					for channel := 0; channel < 3; channel++ {
						if farBase.Pix[i+channel] != other.Pix[i+channel] {
							changed++
							break
						}
					}
				}
			}
			t.Logf("%s: changed far pixels %d", variant.name, changed)
		}
	}
}
