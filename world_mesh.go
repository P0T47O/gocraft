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
	meta   []byte
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

func (s *meshSnapshot) metaAt(wx, wy, wz int) byte {
	idx := s.index(wx, wy, wz)
	if idx < 0 {
		return 0
	}
	return s.meta[idx]
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
			for job := range w.meshJobs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("MeshWorker Panic on Chunk %d,%d Section %d: %v\n", job.key.X, job.key.Z, job.section, r)
							// Send empty result to clear pending flags and prevent deadlock
							w.meshResults <- meshResult{
								key:     job.key,
								results: nil, // Partial/Nil result
								section: job.section,
								version: job.version,
							}
						}
					}()

					// Debug logging
					// fmt.Printf("Starting mesh job: %v\n", job.key)
					snapshot := buildMeshSnapshotFromNeighbors(job)
					results := assets.buildAllMeshData(&job.heightMap, job.baseX, job.baseZ, job.yMin, job.yMax, snapshot.blockAt, snapshot.lightAt, snapshot.metaAt, w.seed)
					snapshot.Release() // Return buffers to pool
					w.meshResults <- meshResult{
						key:     job.key,
						results: results,
						section: job.section,
						version: job.version,
					}
				}()
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
	meta := meshBufferPool.Get().([]byte)

	// Initialize light to 15 (Skylight) so unloaded chunks don't cause black chunk borders
	for i := range light {
		light[i] = 15
	}
	// Zero blocks and meta
	for i := range blocks {
		blocks[i] = 0 // Air
	}
	for i := range meta {
		meta[i] = 0
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
				meta[idx] = chunk.meta[lx][y][lz]
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
		meta:   meta,
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
	// We iterate the map directly. To allow safe modification (re-adding if full),
	// we use the safe deletion pattern, but re-adding in loop is tricky.
	// Since immediate updates are critical, we can try to process them all.
	// But let's pull keys first to be safe and deterministic.

	// Optimization: Reuse a static buffer for keys if possible?
	// For now, simple slice is fine.
	keys := make([]sectionKey, 0, len(w.immediate))
	for key := range w.immediate {
		keys = append(keys, key)
	}

	for _, key := range keys {
		if count >= max {
			break
		}
		// We process it, so remove from queue.
		// If we fail to send, we re-add it.
		delete(w.immediate, key)

		chunk := w.getChunkIfGenerated(key.X, key.Z)
		if chunk == nil {
			continue
		}
		ensureChunkSections(chunk)
		if key.Section < 0 || key.Section >= sectionCount {
			continue
		}

		// Check what needs rebuilding
		sec := key.Section
		dirty := chunk.sectionDirty[sec]

		// If perfectly clean and meshes exist, skip
		if !dirty && chunk.opaqueMeshes[sec] != nil && chunk.waterMeshes[sec] != nil &&
			chunk.cutoutMeshes[sec] != nil && chunk.glassMeshes[sec] != nil {
			continue
		}

		// Prepare Neighbors
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

		// Construct Job
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

		// Set Pending Flags (so we know a result is coming)
		chunk.pendingOpaque[sec] = true
		chunk.pendingWater[sec] = true
		chunk.pendingCutout[sec] = true
		chunk.pendingGlass[sec] = true

		// Attempt to Send
		select {
		case w.meshJobs <- job:
			// Success
			count++
		default:
			// Channel Full!
			// Re-queue this job for next frame
			w.immediate[key] = true
			// We can stop processing to prevent thrashing
			return
		}
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

			if res.results == nil {
				// Job failed/panicked
				chunk.meshRetries[res.section]++
				if chunk.meshRetries[res.section] > 5 {
					// Stop trying to mesh this section
					chunk.sectionDirty[res.section] = false // Mark clean so we don't retry

					// Clear pending implies we are done (failed)
					chunk.pendingOpaque[res.section] = false
					chunk.pendingWater[res.section] = false
					chunk.pendingCutout[res.section] = false
					chunk.pendingGlass[res.section] = false
					fmt.Printf("Disabled corrupted Chunk Section %d,%d Sec %d after 5 retries\n", res.key.X, res.key.Z, res.section)
				} else {
					// Retry next frame
					chunk.pendingOpaque[res.section] = false
					chunk.pendingWater[res.section] = false
					chunk.pendingCutout[res.section] = false
					chunk.pendingGlass[res.section] = false
				}
				continue
			}

			// Success - Reset retries
			chunk.meshRetries[res.section] = 0

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
