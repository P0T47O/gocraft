package main

import (
	"sync"
)

// ChunkPool manages a pool of reusable Chunk objects to reduce GC pressure
type ChunkPool struct {
	pool sync.Pool
}

func NewChunkPool(initialCap int) *ChunkPool {
	return &ChunkPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Chunk{}
			},
		},
	}
}

// Get retrieves a chunk from the pool and resets it
func (p *ChunkPool) Get() *Chunk {
	c := p.pool.Get().(*Chunk)
	c.Reset()
	return c
}

// Put returns a chunk to the pool
func (p *ChunkPool) Put(c *Chunk) {
	c.Reset() // Clear data before returning to pool to prevent leaks
	p.pool.Put(c)
}

// Reset clears the chunk data so it can be reused
func (c *Chunk) Reset() {
	// Re-initialize arrays to zero
	// Note: Go arrays are value types, so assigning a zero-value array clears them
	c.blocks = [chunkWidth][chunkHeight][chunkWidth]byte{}
	c.meta = [chunkWidth][chunkHeight][chunkWidth]byte{}
	c.heightMap = [chunkWidth][chunkWidth]int16{}
	c.skyLight = [chunkWidth][chunkHeight][chunkWidth]byte{}
	c.blockLight = [chunkWidth][chunkHeight][chunkWidth]byte{}

	// Unload and clear meshes
	// We must unload meshes properly to free GPU resources
	for i := range c.opaqueMeshes {
		for _, meshes := range c.opaqueMeshes[i] {
			for _, m := range meshes {
				m.unload()
			}
		}
		c.opaqueMeshes[i] = nil
	}
	for i := range c.waterMeshes {
		for _, meshes := range c.waterMeshes[i] {
			for _, m := range meshes {
				m.unload()
			}
		}
		c.waterMeshes[i] = nil
	}
	for i := range c.cutoutMeshes {
		for _, meshes := range c.cutoutMeshes[i] {
			for _, m := range meshes {
				m.unload()
			}
		}
		c.cutoutMeshes[i] = nil
	}
	for i := range c.glassMeshes {
		for _, meshes := range c.glassMeshes[i] {
			for _, m := range meshes {
				m.unload()
			}
		}
		c.glassMeshes[i] = nil
	}

	// Re-initialize slices if they are nil or too small (though usually fixed size)
	// But simply setting them to nil/default is safer for a general reset
	if len(c.sectionDirty) != sectionCount {
		c.sectionDirty = make([]bool, sectionCount)
		c.pendingOpaque = make([]bool, sectionCount)
		c.pendingWater = make([]bool, sectionCount)
		c.pendingCutout = make([]bool, sectionCount)
		c.pendingGlass = make([]bool, sectionCount)
		c.meshVersion = make([]uint32, sectionCount)

		c.opaqueMeshes = make([]map[string][]*ChunkMesh, sectionCount)
		c.waterMeshes = make([]map[string][]*ChunkMesh, sectionCount)
		c.cutoutMeshes = make([]map[string][]*ChunkMesh, sectionCount)
		c.glassMeshes = make([]map[string][]*ChunkMesh, sectionCount)
	} else {
		for i := 0; i < sectionCount; i++ {
			c.sectionDirty[i] = false
			c.pendingOpaque[i] = false
			c.pendingWater[i] = false
			c.pendingCutout[i] = false
			c.pendingGlass[i] = false
			c.meshVersion[i] = 0
			// Maps are already nilled above
		}
	}

	c.dirty = false
	c.generated = false
	c.torchCount = 0
}
