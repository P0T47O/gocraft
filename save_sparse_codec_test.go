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
			var decoded, decodedMeta chunkPlane
			if err := decodeSparseChunk(p, b, &decoded, &decodedMeta, m); err != nil {
				t.Fatal(err)
			}
			wantMeta := chunkPlane{}
			if meta != nil {
				wantMeta = *meta
			}
			if !decoded.Equal(&c.blocks) || !decodedMeta.Equal(&wantMeta) {
				t.Fatalf("roundtrip differs: %s meta=%v", mode, withMeta)
			}
			wp, wb, wm := encodeChunk(c.blocks.Dense(), denseMeta)
			if !bytes.Equal(p, wp) || !bytes.Equal(b, wb) || !bytes.Equal(m, wm) {
				t.Fatalf("wire/save format differs: %s meta=%v", mode, withMeta)
			}
		}
	}
}

var saveCodecSink []byte

func TestSparseDecodeRejectsCorruptionAtomically(t *testing.T) {
	valid := []byte{255, 255, 0, 1, 0, 0}
	for _, bad := range [][]byte{{1}, {0, 0, 0}, {1, 0, 0}, {255, 255, 0, 2, 0, 0}, {255, 255, 1, 1, 0, 1}} {
		var blocks, meta chunkPlane
		blocks.Fill(9)
		meta.Fill(3)
		if err := decodeSparseChunk([]byte{7}, bad, &blocks, &meta, nil); err == nil {
			t.Fatal("accepted bad blocks")
		}
		if err := decodeSparseChunk([]byte{7}, valid, &blocks, &meta, []byte{1}); err == nil {
			t.Fatal("accepted bad metadata")
		}
		if blocks.Get(0, 0, 0) != 9 || meta.Get(0, 0, 0) != 3 {
			t.Fatal("mutated on failure")
		}
	}
	var blocks, meta chunkPlane
	meta.Fill(3)
	if err := decodeSparseChunk([]byte{7}, valid, &blocks, &meta, nil); err != nil {
		t.Fatal(err)
	}
	if blocks.Get(15, 255, 15) != 7 || meta.Get(0, 0, 0) != 0 {
		t.Fatal("legacy decode")
	}
}

func BenchmarkSparseSaveDecoding(b *testing.B) {
	initBlockRegistry()
	c := new(Chunk)
	generateChunkData(12345, 0, 0, c)
	p, data, metadata := encodeSparseChunk(&c.blocks, &c.meta)
	for _, sparse := range []bool{false, true} {
		name := "dense-adapter"
		if sparse {
			name = "sparse-direct"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var blocks, meta chunkPlane
				if sparse {
					if err := decodeSparseChunk(p, data, &blocks, &meta, metadata); err != nil {
						b.Fatal(err)
					}
				} else {
					db, dm := new([chunkWidth][chunkHeight][chunkWidth]byte), new([chunkWidth][chunkHeight][chunkWidth]byte)
					if err := decodeChunk(p, data, db, dm, metadata); err != nil {
						b.Fatal(err)
					}
					blocks.FromDense(db)
					meta.FromDense(dm)
				}
			}
		})
	}
}

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
