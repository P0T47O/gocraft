package main

import (
	"math"
	"runtime"
	"sync"
	"time"
)

type World struct {
	chunks           map[chunkKey]*Chunk
	chunksMu         sync.RWMutex // Protects the chunks map
	dirty            bool
	seed             uint32
	genQueue         chan chunkKey
	genResults       chan chunkGenResult
	pending          map[chunkKey]bool
	waterDraws       []waterDraw
	translucentDraws []translucentDraw
	meshJobs         chan meshJob
	meshResults      chan meshResult
	immediate        map[sectionKey]bool
	chunkPool        *ChunkPool

	// Entity System
	entities   []Entity
	entitiesMu sync.RWMutex
}

type EntityType int

const (
	EntityPlayer EntityType = iota
	EntityPig
)

type Entity interface {
	GetUUID() string
	GetType() EntityType
	GetPosition() (float64, float64, float64)
	SetPosition(x, y, z float64)
	GetRotation() (float32, float32)
	SetRotation(yaw, pitch float32)
	Tick(world *World)
	IsDirty() bool
	ClearDirty()
}

type BaseEntity struct {
	UUID       string
	Type       EntityType
	X, Y, Z    float64
	Yaw, Pitch float32
	Dirty      bool
}

func (e *BaseEntity) GetUUID() string                          { return e.UUID }
func (e *BaseEntity) GetType() EntityType                      { return e.Type }
func (e *BaseEntity) GetPosition() (float64, float64, float64) { return e.X, e.Y, e.Z }
func (e *BaseEntity) SetPosition(x, y, z float64)              { e.X, e.Y, e.Z = x, y, z; e.Dirty = true }
func (e *BaseEntity) GetRotation() (float32, float32)          { return e.Yaw, e.Pitch }
func (e *BaseEntity) SetRotation(yaw, pitch float32)           { e.Yaw, e.Pitch = yaw, pitch; e.Dirty = true }
func (e *BaseEntity) IsDirty() bool                            { return e.Dirty }
func (e *BaseEntity) ClearDirty()                              { e.Dirty = false }
func (e *BaseEntity) Tick(world *World)                        {} // Default empty tick

type PigEntity struct {
	BaseEntity
	moveTimer float32
}

func (p *PigEntity) Tick(world *World) {
	// Simple random movement logic
	p.moveTimer -= 0.05
	if p.moveTimer <= 0 {
		p.moveTimer = 2.0 + (float32(time.Now().UnixNano()%100) / 50.0)
		// Change rotation randomly
		p.Yaw += (float32(time.Now().UnixNano()%360) - 180.0)
		p.Dirty = true
	}

	// Move forward based on Yaw
	rad := float64(p.Yaw) * math.Pi / 180.0
	dx := math.Sin(rad) * 0.05
	dz := math.Cos(rad) * 0.05

	p.X += dx
	p.Z += dz

	// Basic Gravity (keep it on surface for now)
	h := float64(world.HeightAt(int(p.X), int(p.Z)))
	if p.Y > h {
		p.Y -= 0.1
		if p.Y < h {
			p.Y = h
		}
	} else if p.Y < h {
		p.Y = h
	}
	p.Dirty = true
}

type PlayerEntity struct {
	BaseEntity
}

func (p *PlayerEntity) Tick(world *World) {
	// Sync logic...
}

func (w *World) TickEntities() {
	w.entitiesMu.Lock()
	defer w.entitiesMu.Unlock()
	for _, e := range w.entities {
		e.Tick(w)
	}
}

type chunkKey struct {
	X int
	Z int
}

type sectionKey struct {
	X       int
	Z       int
	Section int
}

type Chunk struct {
	mu            sync.RWMutex
	blocks        [chunkWidth][chunkHeight][chunkWidth]byte
	meta          [chunkWidth][chunkHeight][chunkWidth]byte
	heightMap     [chunkWidth][chunkWidth]int16
	opaqueMeshes  []map[string][]*ChunkMesh
	waterMeshes   []map[string][]*ChunkMesh
	cutoutMeshes  []map[string][]*ChunkMesh
	glassMeshes   []map[string][]*ChunkMesh
	skyLight      [chunkWidth][chunkHeight][chunkWidth]byte
	blockLight    [chunkWidth][chunkHeight][chunkWidth]byte
	dirty         bool
	generated     bool
	sectionDirty  []bool
	pendingOpaque []bool
	pendingWater  []bool
	pendingCutout []bool
	pendingGlass  []bool
	meshVersion   []uint32
	torchCount    int
}

func NewFlatWorld() *World {
	w := &World{
		chunks:      make(map[chunkKey]*Chunk),
		seed:        uint32(time.Now().UnixNano()),
		genQueue:    make(chan chunkKey, 1024),
		genResults:  make(chan chunkGenResult, 1024),
		pending:     make(map[chunkKey]bool),
		meshJobs:    make(chan meshJob, 1024),
		meshResults: make(chan meshResult, 1024),
		immediate:   make(map[sectionKey]bool),
		chunkPool:   NewChunkPool(1024),
	}
	return w
}

func (w *World) StartBackend() {
	// init gen workers
	for i := 0; i < runtime.NumCPU(); i++ {
		go w.genWorker()
	}
}

func NewClientWorld() *World {
	world := &World{
		chunks:      map[chunkKey]*Chunk{},
		seed:        uint32(time.Now().UnixNano()),
		genQueue:    make(chan chunkKey, 512),
		genResults:  make(chan chunkGenResult, 512),
		pending:     map[chunkKey]bool{},
		meshJobs:    make(chan meshJob, 128),
		meshResults: make(chan meshResult, 128),
		immediate:   map[sectionKey]bool{},
		chunkPool:   NewChunkPool(1024),
	}
	// Client world does not spawn generation workers.
	// It relies on receiving chunk data from the server.
	return world
}

func (w *World) allocChunk() *Chunk {
	// fmt.Println("AllocChunk")
	return w.chunkPool.Get()
}

func (w *World) freeChunk(c *Chunk) {
	// fmt.Println("FreeChunk")
	w.chunkPool.Put(c)
}

func (w *World) RemoveBlock(x, y, z int) {
	if y < 0 || y >= chunkHeight {
		return
	}
	if w.BlockAt(x, y, z) == blockAir {
		return
	}
	w.SetBlockAt(x, y, z, blockAir)

	// Check if block above needs support
	up := w.BlockAt(x, y+1, z)
	if up != blockAir {
		def := GetBlock(up)
		// If it's a plant or cactus, check if it still has support (which it likely doesn't since we just removed it)
		// Simpler: Just check if it IS a plant, if so, remove it.
		// Standard MC: If support is gone, break it.
		if def.RenderType == RenderTypeCross || up == blockCactus {
			// Recursive removal
			w.RemoveBlock(x, y+1, z)
		}
	}
}

func (w *World) PlaceAdjacent(hit hitInfo, block byte) (int, int, int, bool) {
	if !hit.hit {
		return 0, 0, 0, false
	}
	nx := hit.x + int(math.Round(float64(hit.normal.X)))
	ny := hit.y + int(math.Round(float64(hit.normal.Y)))
	nz := hit.z + int(math.Round(float64(hit.normal.Z)))
	if ny < 0 || ny >= chunkHeight {
		return 0, 0, 0, false
	}
	if w.BlockAt(nx, ny, nz) != blockAir {
		return 0, 0, 0, false
	}
	if block == blockTorch {
		if hit.normal.Y < 0 {
			return 0, 0, 0, false
		}
		if hit.normal.Y > 0 {
			support := w.BlockAt(nx, ny-1, nz)
			if support == blockTorch || !isOpaqueBlock(support) {
				return 0, 0, 0, false
			}
			w.SetBlockAt(nx, ny, nz, blockTorch)
			w.SetMetaAt(nx, ny, nz, 0)
			return nx, ny, nz, true
		}
		support := w.BlockAt(hit.x, hit.y, hit.z)
		if support == blockTorch || !isOpaqueBlock(support) {
			return 0, 0, 0, false
		}
		meta := byte(0)
		switch {
		case hit.normal.Z < 0:
			meta = 1
		case hit.normal.Z > 0:
			meta = 2
		case hit.normal.X < 0:
			meta = 3
		case hit.normal.X > 0:
			meta = 4
		}
		w.SetBlockAt(nx, ny, nz, blockTorch)
		w.SetMetaAt(nx, ny, nz, meta)
		return nx, ny, nz, true
	}
	// Plant validation logic
	def := GetBlock(block)
	if def.RenderType == RenderTypeCross || block == blockCactus {
		support := w.BlockAt(nx, ny-1, nz)
		validSoil := false
		if block == blockCactus || block == blockDeadBush {
			validSoil = (support == blockSand)
		} else {
			// Flowers, Grass
			validSoil = (support == blockGrass || support == blockDirt)
		}
		if !validSoil {
			return 0, 0, 0, false
		}
	}

	w.SetBlockAt(nx, ny, nz, block)
	return nx, ny, nz, true
}

func (w *World) IsDirty() bool {
	return w.dirty
}

func (w *World) ClearDirty() {
	w.dirty = false
}

func (w *World) BlockAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return blockAir
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if chunk.generated {
		return chunk.blocks[lx][y][lz]
	}
	return blockAtProcedural(w.seed, x, y, z)
}

func (w *World) MetaAt(x, y, z int) byte {
	if y < 0 || y >= chunkHeight {
		return 0
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if chunk.generated {
		return chunk.meta[lx][y][lz]
	}
	return 0
}

func (w *World) SetMetaAt(x, y, z int, meta byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if !chunk.generated {
		generateChunkData(w.seed, cx, cz, chunk)
		chunk.mu.Lock()
		// blocks and heightMap are already filled in chunk
		ensureChunkSections(chunk)
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
			chunk.meshVersion[i]++
		}
		chunk.mu.Unlock()
		chunk.generated = true
		w.chunksMu.Lock()
		delete(w.pending, chunkKey{X: cx, Z: cz})
		w.chunksMu.Unlock()
	}
	chunk.mu.Lock()
	if chunk.meta[lx][y][lz] == meta {
		chunk.mu.Unlock()
		return
	}
	chunk.meta[lx][y][lz] = meta
	chunk.dirty = true
	ensureChunkSections(chunk)
	sec := sectionIndexForY(y)
	chunk.sectionDirty[sec] = true
	chunk.meshVersion[sec]++
	chunk.mu.Unlock()
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			ncx := cx + dx
			ncz := cz + dz
			w.markChunkAllSectionsDirty(ncx, ncz)
			w.requestImmediateAllSections(ncx, ncz)
		}
	}
}

func (w *World) SetBlockAt(x, y, z int, block byte) {
	if y < 0 || y >= chunkHeight {
		return
	}
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if !chunk.generated {
		generateChunkData(w.seed, cx, cz, chunk)
		chunk.mu.Lock()
		// blocks and heightMap are already filled in chunk
		ensureChunkSections(chunk)
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
			chunk.meshVersion[i]++
		}
		chunk.mu.Unlock()
		chunk.generated = true
		w.chunksMu.Lock()
		delete(w.pending, chunkKey{X: cx, Z: cz})
		w.chunksMu.Unlock()
	}
	chunk.mu.Lock()
	oldBlock := chunk.blocks[lx][y][lz]
	if oldBlock == block {
		chunk.mu.Unlock()
		return
	}
	chunk.blocks[lx][y][lz] = block
	if oldBlock == blockTorch {
		chunk.torchCount--
		if chunk.torchCount < 0 {
			chunk.torchCount = 0
		}
	}
	if block == blockTorch {
		chunk.torchCount++
	}
	chunk.meta[lx][y][lz] = 0
	chunk.updateHeightMap(lx, lz, y)
	chunk.dirty = true
	ensureChunkSections(chunk)
	sec := sectionIndexForY(y)
	chunk.sectionDirty[sec] = true
	chunk.meshVersion[sec]++
	chunk.mu.Unlock()
	if y%sectionHeight == 0 && sec > 0 {
		w.markChunkSectionDirty(cx, cz, sec-1)
		w.requestImmediateMesh(cx, cz, sec-1)
	}
	if y%sectionHeight == sectionHeight-1 && sec < sectionCount-1 {
		w.markChunkSectionDirty(cx, cz, sec+1)
		w.requestImmediateMesh(cx, cz, sec+1)
	}
	if lx == 0 {
		w.markChunkSectionDirty(cx-1, cz, sec)
		w.requestImmediateMesh(cx-1, cz, sec)
	}
	if lx == chunkWidth-1 {
		w.markChunkSectionDirty(cx+1, cz, sec)
		w.requestImmediateMesh(cx+1, cz, sec)
	}
	if lz == 0 {
		w.markChunkSectionDirty(cx, cz-1, sec)
		w.requestImmediateMesh(cx, cz-1, sec)
	}
	if lz == chunkWidth-1 {
		w.markChunkSectionDirty(cx, cz+1, sec)
		w.requestImmediateMesh(cx, cz+1, sec)
	}
	w.requestImmediateMesh(cx, cz, sec)
	opacityChanged := isOpaqueBlock(oldBlock) != isOpaqueBlock(block)
	if emitsLight(oldBlock) || emitsLight(block) || opacityChanged {
		w.updateBlockLight(x, y, z, oldBlock, block)
	}
	if opacityChanged {
		w.updateSkyLight(x, y, z)
	}
	w.dirty = true
}

func (w *World) HeightAt(x, z int) int {
	cx := divFloor(x, chunkWidth)
	cz := divFloor(z, chunkWidth)
	lx := modFloor(x, chunkWidth)
	lz := modFloor(z, chunkWidth)
	chunk := w.requestChunk(cx, cz)
	if chunk.generated {
		return int(chunk.heightMap[lx][lz])
	}
	return terrainTopY(w.seed, x, z)
}

func (w *World) markChunkAllSectionsDirty(cx, cz int) {
	chunk := w.getChunkIfGenerated(cx, cz)
	if chunk == nil {
		return
	}
	chunk.mu.Lock()
	defer chunk.mu.Unlock()
	ensureChunkSections(chunk)
	for i := range chunk.sectionDirty {
		chunk.sectionDirty[i] = true
		chunk.meshVersion[i]++
	}
}

func (w *World) requestChunk(cx, cz int) *Chunk {
	key := chunkKey{X: cx, Z: cz}

	w.chunksMu.Lock()
	chunk := w.chunks[key]
	if chunk == nil {
		chunk = w.allocChunk()
		w.chunks[key] = chunk
		w.chunksMu.Unlock()
		w.queueChunkGen(key)
		return chunk
	}
	w.chunksMu.Unlock()

	if !chunk.generated {
		w.queueChunkGen(key)
	}
	return chunk
}

func (w *World) getChunkIfGenerated(cx, cz int) *Chunk {
	if w == nil {
		return nil
	}
	w.chunksMu.RLock()
	defer w.chunksMu.RUnlock()

	if chunk, ok := w.chunks[chunkKey{X: cx, Z: cz}]; ok && chunk.generated {
		return chunk
	}
	return nil
}

func (w *World) ensureChunk(cx, cz int) *Chunk {
	key := chunkKey{X: cx, Z: cz}
	w.chunksMu.Lock()
	defer w.chunksMu.Unlock()

	if chunk, ok := w.chunks[key]; ok {
		return chunk
	}
	chunk := &Chunk{}
	w.chunks[key] = chunk
	return chunk
}

func ensureChunkSections(chunk *Chunk) {
	if chunk.opaqueMeshes == nil {
		chunk.opaqueMeshes = make([]map[string][]*ChunkMesh, sectionCount)
	}
	if chunk.waterMeshes == nil {
		chunk.waterMeshes = make([]map[string][]*ChunkMesh, sectionCount)
	}
	if chunk.cutoutMeshes == nil {
		chunk.cutoutMeshes = make([]map[string][]*ChunkMesh, sectionCount)
	}
	if chunk.glassMeshes == nil {
		chunk.glassMeshes = make([]map[string][]*ChunkMesh, sectionCount)
	}
	if chunk.sectionDirty == nil {
		chunk.sectionDirty = make([]bool, sectionCount)
		for i := range chunk.sectionDirty {
			chunk.sectionDirty[i] = true
		}
	}
	if chunk.pendingOpaque == nil {
		chunk.pendingOpaque = make([]bool, sectionCount)
	}
	if chunk.pendingWater == nil {
		chunk.pendingWater = make([]bool, sectionCount)
	}
	if chunk.pendingCutout == nil {
		chunk.pendingCutout = make([]bool, sectionCount)
	}
	if chunk.pendingGlass == nil {
		chunk.pendingGlass = make([]bool, sectionCount)
	}
	if chunk.meshVersion == nil {
		chunk.meshVersion = make([]uint32, sectionCount)
	}
}

func sectionIndexForY(y int) int {
	if y < 0 {
		return 0
	}
	sec := y / sectionHeight
	if sec < 0 {
		return 0
	}
	if sec >= sectionCount {
		return sectionCount - 1
	}
	return sec
}

func (c *Chunk) updateHeightMap(x, z, y int) {
	current := int(c.heightMap[x][z])
	if y >= current {
		if c.blocks[x][y][z] != blockAir {
			c.heightMap[x][z] = int16(y + 1)
		}
		return
	}
	if y == current-1 && c.blocks[x][y][z] == blockAir {
		for ny := y - 1; ny >= 0; ny-- {
			if c.blocks[x][ny][z] != blockAir {
				c.heightMap[x][z] = int16(ny + 1)
				return
			}
		}
		c.heightMap[x][z] = 0
	}
}

func (c *Chunk) rebuildHeightMap() {
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			h := 0
			for y := chunkHeight - 1; y >= 0; y-- {
				if c.blocks[x][y][z] != blockAir {
					h = y + 1
					break
				}
			}
			c.heightMap[x][z] = int16(h)
		}
	}
}

func (c *Chunk) rebuildTorchCount() {
	count := 0
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if c.blocks[x][y][z] == blockTorch {
					count++
				}
			}
		}
	}
	c.torchCount = count
}

func (w *World) UnloadChunks(playerX, playerZ, radius int, onUnload func(int, int)) {
	radiusSq := radius * radius
	toRemove := make([]chunkKey, 0)

	w.chunksMu.RLock()
	for key := range w.chunks {
		dx := key.X - playerX
		dz := key.Z - playerZ
		distSq := dx*dx + dz*dz

		if distSq > radiusSq {
			if onUnload != nil {
				onUnload(key.X, key.Z)
			}
			toRemove = append(toRemove, key)
		}
	}
	w.chunksMu.RUnlock()

	if len(toRemove) > 0 {
		w.chunksMu.Lock()
		for _, key := range toRemove {
			chunk := w.chunks[key]
			delete(w.chunks, key)
			w.freeChunk(chunk)
			perfMon.IncrementChunkUnload()
		}
		w.chunksMu.Unlock()
	}
}
func findLandSpawn(world *World, startX, startZ, radius int) (int, int, bool) {
	bestX, bestZ := startX, startZ
	found := false
	bestHeight := -1
	seaLevel := 62
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			x := startX + dx
			z := startZ + dz
			h := world.HeightAt(x, z)
			if h <= seaLevel {
				continue
			}
			if world.BlockAt(x, h-1, z) == blockWater {
				continue
			}
			if h > bestHeight {
				bestHeight = h
				bestX = x
				bestZ = z
				found = true
			}
		}
	}
	return bestX, bestZ, found
}
