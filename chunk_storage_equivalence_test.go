package main

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// Hash pre-migration byte order across oceans, caves, negative coordinates and
// multiple seeds. The expected digest is captured before changing storage.
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
	if got != "6a03ca4e703e039da1917ef7045d0d5e4bd4559ef1f3511e1555ed89e0719b4d" {
		t.Fatal("storage migration changed generated voxels or lighting")
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
