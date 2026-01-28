package main

import (
	"fmt"
	"sync"
)

// meshKind is deprecated but kept for minimal changes elsewhere if needed
type meshKind int

type meshSnapshot struct {
	blocks []byte
	light  []byte
	sizeX  int
	sizeZ  int
	baseX  int
	baseZ  int
}

func (s *meshSnapshot) index(wx, wy, wz int) int {
	ix := wx - (s.baseX - 1)
	iz := wz - (s.baseZ - 1)
	if ix < 0 || ix >= s.sizeX || iz < 0 || iz >= s.sizeZ || wy < 0 || wy >= chunkHeight {
		return -1
	}
	return (ix*chunkHeight+wy)*s.sizeZ + iz
}

func (s *meshSnapshot) blockAt(wx, wy, wz int) byte {
	idx := s.index(wx, wy, wz)
	if idx < 0 {
		return blockAir
	}
	return s.blocks[idx]
}

func (s *meshSnapshot) lightAt(wx, wy, wz int) byte {
	idx := s.index(wx, wy, wz)
	if idx < 0 {
		return 15
	}
	return s.light[idx]
}

type meshJob struct {
	key       chunkKey
	baseX     int
	baseZ     int
	heightMap [chunkWidth][chunkWidth]int16
	centerCX  int
	centerCZ  int
	neighbors [3][3]*Chunk
	section   int
	yMin      int
	yMax      int
	version   uint32
}

type meshResult struct {
	key     chunkKey
	results map[string]map[string][]*MeshBuildData
	section int
	version uint32
}

var meshBufferPool = sync.Pool{
	New: func() interface{} {
		sizeX := chunkWidth + 2
		sizeZ := chunkWidth + 2
		total := sizeX * sizeZ * chunkHeight
		return make([]byte, total)
	},
}

func (s *meshSnapshot) Release() {
	if s.blocks != nil {
		meshBufferPool.Put(s.blocks)
		s.blocks = nil
	}
	if s.light != nil {
		meshBufferPool.Put(s.light)
		s.light = nil
	}
}

func (w *World) StartMeshWorkers(assets *RenderAssets, workers int) {
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("MeshWorker Panic: %v\n", r)
				}
			}()
			for job := range w.meshJobs {
				snapshot := buildMeshSnapshotFromNeighbors(job)
				results := assets.buildAllMeshData(&job.heightMap, job.baseX, job.baseZ, job.yMin, job.yMax, snapshot.blockAt, snapshot.lightAt, w.seed)
				snapshot.Release() // Return buffers to pool
				w.meshResults <- meshResult{
					key:     job.key,
					results: results,
					section: job.section,
					version: job.version,
				}
			}
		}()
	}
}

func buildMeshSnapshotFromNeighbors(job meshJob) *meshSnapshot {
	sizeX := chunkWidth + 2
	sizeZ := chunkWidth + 2

	// Allocate from pool
	blocks := meshBufferPool.Get().([]byte)
	light := meshBufferPool.Get().([]byte)

	// Zero out buffers just in case (optional but safer)
	// Actually no need to zero blocks as we overwrite or ignore out of bounds
	// But light needs to be initiated to 15?

	// Initialize light to 15 (Skylight) so unloaded chunks don't cause black chunk borders
	for i := range light {
		light[i] = 15
	}
	// Zero blocks?
	for i := range blocks {
		blocks[i] = 0 // Air
	}

	baseX := job.baseX
	baseZ := job.baseZ

	for dx := 0; dx < 3; dx++ {
		for dz := 0; dz < 3; dz++ {
			chunk := job.neighbors[dx][dz]
			if chunk != nil {
				chunk.mu.RLock()
			}
		}
	}
	for ix := -1; ix <= chunkWidth; ix++ {
		for iz := -1; iz <= chunkWidth; iz++ {
			wx := baseX + ix
			wz := baseZ + iz
			cx := divFloor(wx, chunkWidth)
			cz := divFloor(wz, chunkWidth)
			dx := cx - job.centerCX
			dz := cz - job.centerCZ
			if dx < -1 || dx > 1 || dz < -1 || dz > 1 {
				continue
			}
			chunk := job.neighbors[dx+1][dz+1]
			if chunk == nil {
				continue
			}
			lx := modFloor(wx, chunkWidth)
			lz := modFloor(wz, chunkWidth)
			for y := 0; y < chunkHeight; y++ {
				idx := ((ix+1)*chunkHeight+y)*sizeZ + (iz + 1)
				blocks[idx] = chunk.blocks[lx][y][lz]
				sky := chunk.skyLight[lx][y][lz]
				block := chunk.blockLight[lx][y][lz]
				if block > sky {
					light[idx] = block
				} else {
					light[idx] = sky
				}
			}
		}
	}
	for dx := 0; dx < 3; dx++ {
		for dz := 0; dz < 3; dz++ {
			chunk := job.neighbors[dx][dz]
			if chunk != nil {
				chunk.mu.RUnlock()
			}
		}
	}

	return &meshSnapshot{
		blocks: blocks,
		light:  light,
		sizeX:  sizeX,
		sizeZ:  sizeZ,
		baseX:  baseX,
		baseZ:  baseZ,
	}
}

func (w *World) markChunkSectionDirty(cx, cz, section int) {
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return
	}
	ensureChunkSections(chunk)
	if section < 0 || section >= sectionCount {
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
			chunk.meshVersion[i]++
		}
		return
	}
	chunk.sectionDirty[section] = true
	chunk.meshVersion[section]++
}

func (w *World) markNeighborsDirty(cx, cz int) {
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			if dx == 0 && dz == 0 {
				continue
			}
			w.markChunkSectionDirty(cx+dx, cz+dz, -1)
		}
	}
}

func (w *World) requestImmediateMesh(cx, cz, section int) {
	if section < 0 || section >= sectionCount {
		return
	}
	w.immediate[sectionKey{X: cx, Z: cz, Section: section}] = true
}

func (w *World) requestImmediateAllSections(cx, cz int) {
	for sec := 0; sec < sectionCount; sec++ {
		w.immediate[sectionKey{X: cx, Z: cz, Section: sec}] = true
	}
}

func (w *World) ProcessImmediateMeshes(assets *RenderAssets, max int) {
	if max < 1 {
		return
	}
	count := 0
	keys := make([]sectionKey, 0, len(w.immediate))
	for key := range w.immediate {
		keys = append(keys, key)
	}
	for _, key := range keys {
		if count >= max {
			break
		}
		delete(w.immediate, key)
		chunk := w.getChunkIfGenerated(key.X, key.Z)
		if chunk == nil {
			continue
		}
		ensureChunkSections(chunk)
		if key.Section < 0 || key.Section >= sectionCount {
			continue
		}
		rebuildOpaque := chunk.sectionDirty[key.Section] || chunk.opaqueMeshes[key.Section] == nil
		rebuildWater := chunk.sectionDirty[key.Section] || chunk.waterMeshes[key.Section] == nil
		rebuildCutout := chunk.sectionDirty[key.Section] || chunk.cutoutMeshes[key.Section] == nil
		rebuildGlass := chunk.sectionDirty[key.Section] || chunk.glassMeshes[key.Section] == nil
		if !rebuildOpaque && !rebuildWater && !rebuildCutout && !rebuildGlass {
			continue
		}
		var neighbors [3][3]*Chunk
		for dx := -1; dx <= 1; dx++ {
			for dz := -1; dz <= 1; dz++ {
				neighbors[dx+1][dz+1] = w.getChunkIfGenerated(key.X+dx, key.Z+dz)
			}
		}
		baseX := key.X * chunkWidth
		baseZ := key.Z * chunkWidth
		yMin := key.Section * sectionHeight
		yMax := yMin + sectionHeight
		if rebuildOpaque || rebuildWater || rebuildCutout || rebuildGlass {
			job := meshJob{
				key:       chunkKey{X: key.X, Z: key.Z},
				baseX:     baseX,
				baseZ:     baseZ,
				heightMap: chunk.heightMap,
				centerCX:  key.X,
				centerCZ:  key.Z,
				neighbors: neighbors,
				section:   key.Section,
				yMin:      yMin,
				yMax:      yMax,
				version:   chunk.meshVersion[key.Section],
			}
			snapshot := buildMeshSnapshotFromNeighbors(job)
			results := assets.buildAllMeshData(&job.heightMap, baseX, baseZ, yMin, yMax, snapshot.blockAt, snapshot.lightAt, w.seed)
			snapshot.Release()

			passCleanup := func(meshes map[string][]*ChunkMesh) {
				for _, list := range meshes {
					for _, m := range list {
						m.unload()
					}
				}
			}

			passCleanup(chunk.opaqueMeshes[key.Section])
			chunk.opaqueMeshes[key.Section] = assets.applyMeshData(results["opaque"])

			passCleanup(chunk.waterMeshes[key.Section])
			chunk.waterMeshes[key.Section] = assets.applyMeshData(results["water"])

			passCleanup(chunk.cutoutMeshes[key.Section])
			chunk.cutoutMeshes[key.Section] = assets.applyMeshData(results["cutout"])

			passCleanup(chunk.glassMeshes[key.Section])
			chunk.glassMeshes[key.Section] = assets.applyMeshData(results["glass"])
		}
		chunk.pendingOpaque[key.Section] = false
		chunk.pendingWater[key.Section] = false
		chunk.pendingCutout[key.Section] = false
		chunk.pendingGlass[key.Section] = false
		clearSectionDirtyIfReady(chunk, key.Section)
		count++
		perfMon.IncrementMeshBuild()
	}
}

func (w *World) ProcessMeshResults(assets *RenderAssets, maxPerFrame int) {
	processed := 0
	for {
		if processed >= maxPerFrame {
			return
		}
		select {
		case res := <-w.meshResults:
			chunk := w.getChunkIfGenerated(res.key.X, res.key.Z)
			if chunk == nil {
				continue
			}
			ensureChunkSections(chunk)
			if res.section < 0 || res.section >= sectionCount {
				continue
			}
			if chunk.meshVersion[res.section] != res.version {
				chunk.pendingOpaque[res.section] = false
				chunk.pendingWater[res.section] = false
				chunk.pendingCutout[res.section] = false
				chunk.pendingGlass[res.section] = false
				continue
			}

			// Clean up old meshes
			passCleanup := func(meshes map[string][]*ChunkMesh) {
				for _, list := range meshes {
					for _, m := range list {
						m.unload()
					}
				}
			}

			// Apply all passes
			passCleanup(chunk.opaqueMeshes[res.section])
			chunk.opaqueMeshes[res.section] = assets.applyMeshData(res.results["opaque"])
			chunk.pendingOpaque[res.section] = false

			passCleanup(chunk.waterMeshes[res.section])
			chunk.waterMeshes[res.section] = assets.applyMeshData(res.results["water"])
			chunk.pendingWater[res.section] = false

			passCleanup(chunk.cutoutMeshes[res.section])
			chunk.cutoutMeshes[res.section] = assets.applyMeshData(res.results["cutout"])
			chunk.pendingCutout[res.section] = false

			passCleanup(chunk.glassMeshes[res.section])
			chunk.glassMeshes[res.section] = assets.applyMeshData(res.results["glass"])
			chunk.pendingGlass[res.section] = false

			clearSectionDirtyIfReady(chunk, res.section)
			processed++
		default:
			return
		}
	}
}

func clearSectionDirtyIfReady(chunk *Chunk, section int) {
	if section < 0 || section >= sectionCount {
		return
	}
	if !chunk.sectionDirty[section] {
		return
	}
	if chunk.pendingOpaque[section] || chunk.pendingWater[section] || chunk.pendingCutout[section] || chunk.pendingGlass[section] {
		return
	}
	if chunk.opaqueMeshes[section] == nil || chunk.waterMeshes[section] == nil || chunk.cutoutMeshes[section] == nil || chunk.glassMeshes[section] == nil {
		return
	}
	chunk.sectionDirty[section] = false
}
