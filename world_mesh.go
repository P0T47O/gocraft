package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// meshKind is deprecated but kept for minimal changes elsewhere if needed
type meshKind int

type meshSnapshot struct {
	blocks []byte
	light  []byte
	meta   []byte
	sizeX  int
	sizeY  int
	sizeZ  int
	baseX  int
	baseZ  int
	yMin   int // inclusive lower Y bound of copied range
	yMax   int // exclusive upper Y bound of copied range
}

func (s *meshSnapshot) index(wx, wy, wz int) int {
	ix := wx - (s.baseX - 1)
	iz := wz - (s.baseZ - 1)
	iy := wy - s.yMin
	if ix < 0 || ix >= s.sizeX || iz < 0 || iz >= s.sizeZ || iy < 0 || iy >= s.sizeY {
		return -1
	}
	return (ix*s.sizeY+iy)*s.sizeZ + iz
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
	snapshot          *meshSnapshot
	tints             *meshTintCache
	instance, request uint64
	key               chunkKey
	baseX             int
	baseZ             int
	heightMap         [chunkWidth][chunkWidth]int16
	centerCX          int
	centerCZ          int
	neighbors         [3][3]*Chunk
	section           int
	yMin              int
	yMax              int
	version           uint32
}

type meshResult struct {
	instance, request uint64
	key               chunkKey
	results           map[string]map[string][]*MeshBuildData
	section           int
	version           uint32
}

// meshSectionBufferPool provides reusable buffers sized for a single section + 1-block border in Y.
// sizeY = sectionHeight + 2 (1 block above and below the section).
var meshSectionBufferPool = sync.Pool{
	New: func() interface{} {
		sizeX := chunkWidth + 2
		sizeZ := chunkWidth + 2
		sizeY := sectionHeight + 2
		total := sizeX * sizeY * sizeZ
		return make([]byte, total)
	},
}

func (s *meshSnapshot) Release() {
	if s.blocks != nil {
		meshSectionBufferPool.Put(s.blocks)
		s.blocks = nil
	}
	if s.light != nil {
		meshSectionBufferPool.Put(s.light)
		s.light = nil
	}
	if s.meta != nil {
		meshSectionBufferPool.Put(s.meta)
		s.meta = nil
	}
}

func releaseMeshResults(results map[string]map[string][]*MeshBuildData) {
	for _, pass := range results {
		for _, list := range pass {
			for _, d := range list {
				d.Reset()
				meshBuilderPool.Put(d)
			}
		}
	}
}

func (w *World) StartMeshWorkers(assets *RenderAssets, workers int) {
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		w.workers.Add(1)
		go func() {
			defer w.workers.Done()
			for {
				var job meshJob
				select {
				case <-w.done:
					return
				case job = <-w.meshJobs:
				}
				res := meshResult{key: job.key, section: job.section, version: job.version, instance: job.instance, request: job.request}
				func() {
					defer job.snapshot.Release()
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("Mesh %v section %d: %v\n", job.key, job.section, r)
						}
					}()
					select {
					case <-w.done:
						return
					default:
					}
					start := time.Now()
					res.results = assets.buildAllMeshData(&job.heightMap, job.baseX, job.baseZ, job.yMin, job.yMax, job.snapshot.blockAt, job.snapshot.lightAt, job.snapshot.metaAt, w.seed, job.tints)
					perfMon.recordLoading(phaseMeshCPU, start)
				}()
				select {
				case w.meshResults <- res:
				case <-w.done:
					releaseMeshResults(res.results)
					return
				}
			}
		}()
	}
}

func buildMeshSnapshotFromNeighbors(job meshJob) *meshSnapshot {
	sizeX := chunkWidth + 2
	sizeZ := chunkWidth + 2
	// Only copy the Y range needed for this section, plus 1-block border above and below
	snapYMin := job.yMin - 1
	if snapYMin < 0 {
		snapYMin = 0
	}
	snapYMax := job.yMax + 1
	if snapYMax > chunkHeight {
		snapYMax = chunkHeight
	}
	sizeY := snapYMax - snapYMin

	// Allocate from pool
	blocks := meshSectionBufferPool.Get().([]byte)
	light := meshSectionBufferPool.Get().([]byte)
	meta := meshSectionBufferPool.Get().([]byte)

	total := sizeX * sizeY * sizeZ
	// Ensure pool buffers are large enough (they should be, but guard against edge cases)
	if len(blocks) < total {
		blocks = make([]byte, total)
	}
	if len(light) < total {
		light = make([]byte, total)
	}
	if len(meta) < total {
		meta = make([]byte, total)
	}

	// Initialize: light to 15 (skylight default), blocks and meta to 0
	for i := 0; i < total; i++ {
		light[i] = 15
		blocks[i] = 0
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
			ddx := cx - job.centerCX
			ddz := cz - job.centerCZ
			if ddx < -1 || ddx > 1 || ddz < -1 || ddz > 1 {
				continue
			}
			chunk := job.neighbors[ddx+1][ddz+1]
			if chunk == nil {
				continue
			}
			lx := modFloor(wx, chunkWidth)
			lz := modFloor(wz, chunkWidth)
			idx := (ix+1)*sizeY*sizeZ + iz + 1
			chunk.blocks.CopyColumn(blocks, idx, sizeZ, lx, lz, snapYMin, snapYMax, false)
			chunk.meta.CopyColumn(meta, idx, sizeZ, lx, lz, snapYMin, snapYMax, false)
			chunk.skyLight.CopyColumn(light, idx, sizeZ, lx, lz, snapYMin, snapYMax, false)
			chunk.blockLight.CopyColumn(light, idx, sizeZ, lx, lz, snapYMin, snapYMax, true)
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
		sizeY:  sizeY,
		sizeZ:  sizeZ,
		baseX:  baseX,
		baseZ:  baseZ,
		yMin:   snapYMin,
		yMax:   snapYMax,
	}
}

func (w *World) markChunkSectionDirty(cx, cz, section int) {
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return
	}
	chunk.mu.Lock()
	ensureChunkSections(chunk)
	if section < 0 || section >= sectionCount {
		for i := range chunk.sectionDirty {
			chunk.invalidateMeshSection(i)
		}
		chunk.mu.Unlock()
		return
	}
	chunk.invalidateMeshSection(section)
	chunk.mu.Unlock()
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

func unloadMeshPass(meshes map[string][]*ChunkMesh) {
	for _, list := range meshes {
		for _, m := range list {
			m.unload()
		}
	}
}
func clearSectionMeshes(c *Chunk, sec int) {
	unloadMeshPass(c.opaqueMeshes[sec])
	unloadMeshPass(c.waterMeshes[sec])
	unloadMeshPass(c.cutoutMeshes[sec])
	unloadMeshPass(c.glassMeshes[sec])
	c.opaqueMeshes[sec] = nil
	c.waterMeshes[sec] = nil
	c.cutoutMeshes[sec] = nil
	c.glassMeshes[sec] = nil
}
func setMeshPending(c *Chunk, sec int, pending bool) {
	c.pendingOpaque[sec] = pending
	c.pendingWater[sec] = pending
	c.pendingCutout[sec] = pending
	c.pendingGlass[sec] = pending
}

// Capture mutable voxel data on the owner thread; workers never retain live chunks.
func (w *World) submitMesh(cx, cz, sec int, assets *RenderAssets) bool {
	c := w.getChunkIfGenerated(cx, cz)
	if c == nil || sec < 0 || sec >= sectionCount {
		return false
	}
	ensureChunkSections(c)
	if !c.sectionDirty[sec] || c.pendingOpaque[sec] || c.meshRetries[sec] > 5 {
		return false
	}
	if c.sectionBlocks[sec] == 0 {
		clearSectionMeshes(c, sec)
		c.sectionDirty[sec] = false
		return false
	}
	if len(w.meshJobs) == cap(w.meshJobs) {
		return false
	}
	select {
	case <-w.done:
		return false
	default:
	}
	if c.tints == nil {
		c.tints = assets.buildMeshTintCache(w.seed, cx*chunkWidth, cz*chunkWidth)
	}
	job := meshJob{key: chunkKey{cx, cz}, baseX: cx * chunkWidth, baseZ: cz * chunkWidth, heightMap: c.heightMap, centerCX: cx, centerCZ: cz,
		section: sec, yMin: sec * sectionHeight, yMax: (sec + 1) * sectionHeight, version: c.meshVersion[sec], instance: c.instance, tints: c.tints}
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			job.neighbors[dx+1][dz+1] = w.getChunkIfGenerated(cx+dx, cz+dz)
		}
	}
	job.snapshot = buildMeshSnapshotFromNeighbors(job)
	job.neighbors = [3][3]*Chunk{}
	w.nextMeshID++
	job.request = w.nextMeshID
	select {
	case w.meshJobs <- job:
		c.meshSubmittedVersion[sec] = job.version
		c.meshRequest[sec] = job.request
		setMeshPending(c, sec, true)
		return true
	default:
		job.snapshot.Release()
		return false
	}
}

func (w *World) ProcessImmediateMeshes(assets *RenderAssets, max int) {
	keys := make([]sectionKey, 0, len(w.immediate))
	for key := range w.immediate {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].X != keys[j].X {
			return keys[i].X < keys[j].X
		}
		if keys[i].Z != keys[j].Z {
			return keys[i].Z < keys[j].Z
		}
		return keys[i].Section < keys[j].Section
	})
	deadline := time.Now().Add(time.Millisecond)
	count := 0
	for _, key := range keys {
		if count >= max || time.Now().After(deadline) {
			return
		}
		c := w.getChunkIfGenerated(key.X, key.Z)
		if c == nil || key.Section < 0 || key.Section >= sectionCount || !c.sectionDirty[key.Section] {
			delete(w.immediate, key)
			continue
		}
		if w.submitMesh(key.X, key.Z, key.Section, assets) {
			count++
			delete(w.immediate, key)
		}
	}
}

func (w *World) acceptsMesh(res meshResult) *Chunk {
	c := w.getChunkIfGenerated(res.key.X, res.key.Z)
	if c == nil || c.instance != res.instance || res.section < 0 || res.section >= sectionCount {
		return nil
	}
	if c.meshRequest[res.section] != res.request {
		return nil
	}
	setMeshPending(c, res.section, false)
	if c.meshVersion[res.section] != res.version {
		return nil
	}
	return c
}

// Budget includes stale results, packing and GPU upload. One result is indivisible.
func (w *World) ProcessMeshResults(assets *RenderAssets, maxPerFrame int) {
	deadline := time.Now().Add(2 * time.Millisecond)
	bytes := 0
	for processed := 0; processed < maxPerFrame; processed++ {
		if processed > 0 && (time.Now().After(deadline) || bytes >= 4<<20) {
			return
		}
		select {
		case res := <-w.meshResults:
			c := w.acceptsMesh(res)
			if c == nil {
				if perfMon != nil {
					perfMon.meshStale.Add(1)
				}
				releaseMeshResults(res.results)
				continue
			}
			if res.results == nil {
				c.meshRetries[res.section]++
				continue
			}
			for _, pass := range res.results {
				for _, list := range pass {
					for _, d := range list {
						bytes += d.vertCount*36 + len(d.indices)*4
					}
				}
			}
			sec := res.section
			uploadStart := time.Now()
			clearSectionMeshes(c, sec)
			c.opaqueMeshes[sec] = assets.applyMeshData(res.results["opaque"])
			c.waterMeshes[sec] = assets.applyMeshData(res.results["water"])
			c.cutoutMeshes[sec] = assets.applyMeshData(res.results["cutout"])
			c.glassMeshes[sec] = assets.applyMeshData(res.results["glass"])
			perfMon.recordLoading(phaseUpload, uploadStart)
			c.meshRetries[sec] = 0
			c.sectionDirty[sec] = false
			if w == world {
				recordChunkReady(res.key, c)
			}
			perfMon.IncrementMeshBuild()
		default:
			return
		}
	}
}
