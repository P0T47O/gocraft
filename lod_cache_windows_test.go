//go:build windows

package main

import (
	"os"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

// Real generated terrain plus edits, captured to disk and reloaded into a
// new client World with no full chunks. Uses temporary caches, never a save.
func TestWebGPUClientLODCachePreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_LOD_CACHE_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_LOD_CACHE_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.ExperimentalLOD = true
	currentSettings.HorizonDistance = 64
	const seed = uint32(1234511)
	root := t.TempDir()
	w := NewClientWorld()
	w.seed = seed
	w.openLODCache(root, "preview")
	key := chunkKey{6, 4}
	var source Chunk
	generateChunkData(seed, key.X, key.Z, &source)
	w.applyChunkPacket(chunkPacket(key, &source))
	// Excavate a visible off-grid rectangular pit, add a stone/sand fill and
	// a floating wooden roof with planted leaves beside it.
	for x := 98; x <= 103; x++ {
		for z := 67; z <= 73; z++ {
			ground := sampleTerrainColumn(seed, x, z).height
			for y := ground - 8; y < ground; y++ {
				w.SetBlockAt(x, y, z, blockAir)
			}
		}
	}
	for x := 105; x <= 109; x++ {
		for z := 68; z <= 72; z++ {
			ground := sampleTerrainColumn(seed, x, z).height
			for y := ground; y < ground+3; y++ {
				w.SetBlockAt(x, y, z, blockSand)
			}
			w.SetBlockAt(x, 120, z, blockPlankOak)
		}
	}
	for y := 95; y < 100; y++ {
		w.SetBlockAt(106, y, 76, blockLogBirch)
	}
	for x := 104; x <= 108; x++ {
		for z := 74; z <= 78; z++ {
			w.SetBlockAt(x, 100, z, blockLeavesBirch)
		}
	}
	// Remove original natural vegetation, so the preview also exposes any
	// incorrectly resurrected procedural trees in this known chunk.
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 0; y < chunkHeight; y++ {
				b := source.blocks.Get(x, y, z)
				if generationIsLog(b) || generationIsLeaf(b) {
					w.SetBlockAt(96+x, y, 64+z, blockAir)
				}
			}
		}
	}
	w.UnloadChunks(100, 100, 8, nil)
	w.Close()
	reopened := NewClientWorld()
	defer reopened.Close()
	reopened.seed = seed
	reopened.TimeTicks = 6000
	reopened.openLODCache(root, "preview")
	frame := webGPUFrameContext{Camera: webGPUCamera{Position: webGPUVec3{103, 160, -90}, Target: webGPUVec3{103, 85, 72}, Up: webGPUVec3{0, 1, 0}, Fovy: 48}, Width: 1280, Height: 720}
	deadline := time.Now().Add(12 * time.Second)
	tileKey := lodTileKey{}
	ready := false
	for time.Now().Before(deadline) {
		reopened.processLODCache()
		if err := r.DrawGameplay(reopened, frame, &InputState{}); err != nil {
			t.Fatal(err)
		}
		if reopened.lodObservedChunks[key] != nil && r.lod != nil && len(r.lod.tiles) >= 48 && r.lod.tiles[tileKey] != nil && r.lod.versions[tileKey] == reopened.lodVersions[tileKey] {
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("cached edited terrain tile not rendered")
	}
	image := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error { return r.lod.drawPreview(pass, r, frame.Camera, reopened) })
	saveNativePreview(t, image, "work/lod-cache-restart.png")
}
