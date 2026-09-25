package main

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// Hash generated blocks and light across oceans, caves, negative coordinates
// and multiple seeds. Update this only for deliberate generation changes.
func TestChunkStorageTerrainDigest(t *testing.T) {
	initBlockRegistry()
	h := sha256.New()
	for _, seed := range []uint32{0, 12345, 0xffffffff} {
		for _, pos := range [][2]int{{0, 0}, {-1, -1}, {21, -39}, {64, 73}} {
			c := new(Chunk)
			generateChunkData(seed, pos[0], pos[1], c)
			initializeChunkLighting(c)
			for x := 0; x < chunkWidth; x++ {
				for y := 0; y < chunkHeight; y++ {
					for z := 0; z < chunkWidth; z++ {
						_, _ = h.Write([]byte{c.blocks.Get(x, y, z), c.meta.Get(x, y, z), c.skyLight.Get(x, y, z), c.blockLight.Get(x, y, z)})
					}
				}
			}
		}
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	t.Log(got)
	if got != "1ad89c43fe16ba3d4bd635a4d97b26ea1da9e99202f911c18298785d1fca8d68" {
		t.Fatal("generated voxels or lighting changed unexpectedly")
	}
}

var storageScanSink byte

func BenchmarkChunkStorageScan(b *testing.B) {
	initBlockRegistry()
	c := new(Chunk)
	generateChunkData(12345, 0, 0, c)
	initializeChunkLighting(c)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v byte
		for x := 0; x < chunkWidth; x++ {
			for y := 0; y < chunkHeight; y++ {
				for z := 0; z < chunkWidth; z++ {
					v += c.blocks.Get(x, y, z) + c.meta.Get(x, y, z) + c.skyLight.Get(x, y, z) + c.blockLight.Get(x, y, z)
				}
			}
		}
		storageScanSink = v
	}
}
