//go:build windows

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogpu/wgpu"
)

func TestLODChunkMaskKeepsFallbackOnlyForUnmeshedChunks(t *testing.T) {
	ready := &Chunk{generated: true}
	ensureChunkSections(ready)
	ready.heightMap[0][0] = 1
	ready.sectionBlocks[0] = 1
	ready.opaqueMeshes[0] = map[string][]*ChunkMesh{"terrain": {{}}}
	ready.sectionDirty[0] = false
	claimLODColumnIfReady(ready)
	loading := &Chunk{generated: true}
	w := &World{chunks: map[chunkKey]*Chunk{{-1, -1}: ready, {0, -1}: loading}}
	camera := webGPUCamera{Position: webGPUVec3{-8, 100, -8}}
	mask := lodChunkMask(w, camera)
	originX := int(int32(binary.LittleEndian.Uint32(mask[0:])))
	originZ := int(int32(binary.LittleEndian.Uint32(mask[4:])))
	if originX != -130 || originZ != -130 {
		t.Fatalf("negative camera mask origin = (%d,%d)", originX, originZ)
	}
	covered := func(cx, cz int) bool {
		bit := (cz-originZ)*lodChunkMaskSide + cx - originX
		return mask[16+bit/8]&(1<<(bit%8)) != 0
	}
	if !covered(-1, -1) || covered(0, -1) || covered(-1, 0) {
		t.Fatal("mask must hide LOD only under a generated, meshed chunk")
	}
	ready.opaqueMeshes[0] = nil // Digging the last visible column must not revive LOD.
	mask = lodChunkMask(w, camera)
	if !covered(-1, -1) {
		t.Fatal("an excavated chunk exposed the stale distant terrain")
	}
}

func TestLODChunkMaskWaitsForSurfaceAndClipsCachedChunks(t *testing.T) {
	previous := currentSettings
	currentSettings = &GameSettings{RenderDistance: 32, HorizonDistance: 96}
	defer func() { currentSettings = previous }()
	partial := &Chunk{generated: true}
	ensureChunkSections(partial)
	partial.heightMap[0][0] = 81
	partial.sectionBlocks[0], partial.sectionBlocks[5] = 1, 1
	partial.opaqueMeshes[0] = map[string][]*ChunkMesh{"underground": {{}}}
	partial.sectionDirty[0] = false
	cached := &Chunk{generated: true, lodOccludes: true}
	w := &World{chunks: map[chunkKey]*Chunk{{0, 0}: partial, {24, 24}: cached, {32, 0}: cached}}
	covered := func(key chunkKey) bool {
		mask := lodChunkMask(w, webGPUCamera{})
		x := key.X - int(int32(binary.LittleEndian.Uint32(mask[0:])))
		z := key.Z - int(int32(binary.LittleEndian.Uint32(mask[4:])))
		bit := z*lodChunkMaskSide + x
		return mask[16+bit/8]&(1<<(bit%8)) != 0
	}
	if covered(chunkKey{}) || covered(chunkKey{24, 24}) || !covered(chunkKey{32, 0}) {
		t.Fatal("mask must wait for the surface, exclude cache outside render disk, and include its positive endpoint")
	}
	partial.opaqueMeshes[5] = map[string][]*ChunkMesh{"terrain": {{}}}
	partial.sectionDirty[5] = false
	claimLODColumnIfReady(partial)
	if !covered(chunkKey{}) || !partial.lodOccludes {
		t.Fatal("complete surface did not claim the LOD column")
	}
	partial.opaqueMeshes[5] = nil
	partial.sectionDirty[5] = true
	if !covered(chunkKey{}) {
		t.Fatal("editing a claimed column exposed stale LOD")
	}
}

func TestLODTileRangeIncludesNearUnderlay(t *testing.T) {
	center := lodTileKey{0, 0}
	if !lodTileInRange(center, center, 4) || !lodTileInRange(lodTileKey{4, 0}, center, 4) {
		t.Fatal("LOD must cover the near field while full chunks are streaming")
	}
	if lodTileInRange(lodTileKey{7, 0}, center, 4) {
		t.Fatal("LOD tile outside horizon retained")
	}
}

func TestLODStepBandsAndHysteresis(t *testing.T) {
	full := float32(16 * chunkWidth)
	for _, tc := range []struct {
		distance float32
		previous int
		want     int
	}{
		{100, 0, lodNearCellSize},
		{300, 0, lodNearCellSize},
		{400, 0, lodTransitionCellSize},
		{500, 0, lodCellSize},
		{900, 0, lodFarCellSize},
		{330, lodNearCellSize, lodNearCellSize},
		{330, lodTransitionCellSize, lodTransitionCellSize},
		{455, lodTransitionCellSize, lodTransitionCellSize},
		{455, lodCellSize, lodCellSize},
		{740, lodCellSize, lodCellSize},
		{740, lodFarCellSize, lodCellSize},
		{760, lodFarCellSize, lodFarCellSize},
		{650, lodFarCellSize, lodCellSize},
	} {
		if got := lodStepForDistance(tc.distance, full, tc.previous); got != tc.want {
			t.Fatalf("distance %.0f previous=%d: got %d, want %d", tc.distance, tc.previous, got, tc.want)
		}
	}
}

func TestWebGPUDistantTerrainPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_WEBGPU_LOD_PREVIEW") != "1" {
		t.Skip("set GOCRAFT_WEBGPU_LOD_PREVIEW=1")
	}
	r, cleanup := nativePreviewFixture(t, 4)
	defer cleanup()
	currentSettings.RenderDistance = 8
	currentSettings.HorizonDistance = 64
	world := &World{seed: 1234511, TimeTicks: 6000}
	frame := webGPUFrameContext{
		Camera: webGPUCamera{Position: webGPUVec3{0, 130, 0}, Target: webGPUVec3{600, 70, 600}, Up: webGPUVec3{0, 1, 0}, Fovy: 65},
		Width:  1280, Height: 720,
	}
	state := &InputState{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := r.DrawGameplay(world, frame, state); err != nil {
			t.Fatal(err)
		}
		if r.lod != nil && len(r.lod.tiles) >= 64 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if r.lod == nil || len(r.lod.tiles) == 0 || world.render.drawCalls == 0 {
		t.Fatalf("no distant terrain drawn: renderer=%v drawCalls=%d", r.lod != nil, world.render.drawCalls)
	}
	img := captureNativePreview(t, r, frame.Width, frame.Height, func(pass *wgpu.RenderPassEncoder) error {
		return r.lod.drawPreview(pass, r, frame.Camera, world)
	})
	path := filepath.Join("work", "lod-terrain.png")
	saveNativePreview(t, img, path)
	t.Logf("distant terrain preview: %s; ready tiles=%d", path, len(r.lod.tiles))
	changedTile := lodTileKey{1, 1}
	world.lodColumns = map[lodPoint]lodColumn{{128, 128}: {height: 110, top: blockCobblestone}}
	world.lodVersions = map[lodTileKey]uint64{changedTile: 1}
	for time.Now().Before(deadline.Add(5*time.Second)) && r.lod.versions[changedTile] != 1 {
		if err := r.DrawGameplay(world, frame, state); err != nil {
			t.Fatal(err)
		}
	}
	if r.lod.versions[changedTile] != 1 {
		t.Fatal("authoritative terrain edit did not rebuild its distant tile")
	}
	world.seed = 42
	if err := r.DrawGameplay(world, frame, state); err != nil {
		t.Fatal(err)
	}
	if r.lod == nil || r.lod.seed != 42 {
		t.Fatal("old-seed LOD cache survived a world change")
	}
	currentSettings.HorizonDistance = 0
	if err := r.DrawGameplay(world, frame, state); err != nil {
		t.Fatal(err)
	}
	if r.lod != nil {
		t.Fatal("disabling the horizon retained GPU meshes or workers")
	}
}
