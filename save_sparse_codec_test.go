package main

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestSparseSaveEncodingMatchesDense(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, mode := range []string{"zero", "uniform", "random", "terrain"} {
		c := new(Chunk)
		switch mode {
		case "uniform":
			c.blocks.Fill(7)
			c.meta.Fill(2)
		case "random":
			for x := 0; x < 16; x++ {
				for y := 0; y < 256; y++ {
					for z := 0; z < 16; z++ {
						c.blocks.Set(x, y, z, byte(rng.Intn(256)))
						c.meta.Set(x, y, z, byte(rng.Intn(16)))
					}
				}
			}
		case "terrain":
			initBlockRegistry()
			generateChunkData(12345, 0, 0, c)
		}
		for _, withMeta := range []bool{false, true} {
			var meta *chunkPlane
			var denseMeta *[chunkWidth][chunkHeight][chunkWidth]byte
			if withMeta {
				meta = &c.meta
				denseMeta = c.meta.Dense()
			}
			p, b, m := encodeSparseChunk(&c.blocks, meta)
			wp, wb, wm := encodeChunk(c.blocks.Dense(), denseMeta)
			if !bytes.Equal(p, wp) || !bytes.Equal(b, wb) || !bytes.Equal(m, wm) {
				t.Fatalf("wire/save format differs: %s meta=%v", mode, withMeta)
			}
		}
	}
}

var saveCodecSink []byte

func BenchmarkSparseSaveEncoding(b *testing.B) {
	initBlockRegistry()
	c := new(Chunk)
	generateChunkData(12345, 0, 0, c)
	for _, sparse := range []bool{false, true} {
		name := "dense-adapter"
		if sparse {
			name = "sparse-direct"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if sparse {
					_, saveCodecSink, _ = encodeSparseChunk(&c.blocks, &c.meta)
				} else {
					_, saveCodecSink, _ = encodeChunk(c.blocks.Dense(), c.meta.Dense())
				}
			}
		})
	}
}
