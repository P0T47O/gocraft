package main

import (
	"math/rand"
	"reflect"
	"testing"
	"unsafe"
)

func TestChunkPlaneDenseEquivalence(t *testing.T) {
	var p chunkPlane
	var dense [chunkWidth][chunkHeight][chunkWidth]byte
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100000; i++ {
		x, y, z, v := rng.Intn(16), rng.Intn(256), rng.Intn(16), byte(rng.Intn(256))
		p.Set(x, y, z, v)
		dense[x][y][z] = v
		if i%1000 == 0 {
			p.Compact()
			if *p.Dense() != dense {
				t.Fatal("random writes differ")
			}
		}
	}
	copy := p.Clone()
	before := p.Get(0, 0, 0)
	copy.Set(0, 0, 0, before+1)
	if p.Get(0, 0, 0) != before {
		t.Fatal("clone aliases")
	}
	p.Fill(15)
	p.Set(1, 17, 3, 7)
	if p.Get(2, 17, 3) != 15 {
		t.Fatal("uniform expansion lost other voxels")
	}
	p.Set(1, 17, 3, 15)
	p.Compact()
	for _, s := range p.sections {
		if s.data != nil || s.uniform != 15 {
			t.Fatal("did not collapse")
		}
	}
	p = chunkPlane{}
	if p.Get(1, 17, 3) != 0 {
		t.Fatal("reset retained data")
	}
}

func TestChunkPlaneWireImport(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	data := make([]byte, chunkWidth*chunkHeight*chunkWidth)
	_, _ = rng.Read(data)
	for _, shift := range []uint{0, 4} {
		var p chunkPlane
		p.FromWire(data, shift, 15)
		for x := 0; x < 16; x++ {
			for y := 0; y < 256; y++ {
				for z := 0; z < 16; z++ {
					if p.Get(x, y, z) != (data[(x*256+y)*16+z]>>shift)&15 {
						t.Fatal("wire stride differs")
					}
				}
			}
		}
		p.FromWire(nil, shift, 15)
		for _, s := range p.sections {
			if s.data != nil || s.uniform != 0 {
				t.Fatal("nil wire did not reset")
			}
		}
	}
}

func BenchmarkChunkStorageMesh(b *testing.B) {
	a, c := storageMeshFixture()
	db, dm, ds, dl := c.blocks.Dense(), c.meta.Dense(), c.skyLight.Dense(), c.blockLight.Dense()
	valid := func(x, y, z int) bool { return x >= 0 && x < 16 && y >= 0 && y < 256 && z >= 0 && z < 16 }
	for _, sparse := range []bool{false, true} {
		name := "dense"
		if sparse {
			name = "sparse"
		}
		b.Run(name, func(b *testing.B) {
			block := func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				if sparse {
					return c.blocks.Get(x, y, z)
				}
				return db[x][y][z]
			}
			light := func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 15
				}
				if sparse {
					return max(c.skyLight.Get(x, y, z), c.blockLight.Get(x, y, z))
				}
				return max(ds[x][y][z], dl[x][y][z])
			}
			meta := func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				if sparse {
					return c.meta.Get(x, y, z)
				}
				return dm[x][y][z]
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := a.buildAllMeshData(&c.heightMap, 0, 0, 0, 256, block, light, meta, 12345, nil)
				releaseMeshResults(result)
			}
		})
	}
}

func storageBytes(c *Chunk) uintptr {
	size := unsafe.Sizeof(*c)
	for _, p := range []*chunkPlane{&c.blocks, &c.meta, &c.skyLight, &c.blockLight} {
		for _, s := range p.sections {
			if s.data != nil {
				size += unsafe.Sizeof(*s.data)
			}
		}
	}
	return size
}

func TestChunkStorageFootprint(t *testing.T) {
	initBlockRegistry()
	var total, maximum uintptr
	const count = 64
	for i := 0; i < count; i++ {
		c := new(Chunk)
		generateChunkData(12345, i%8-4, i/8-4, c)
		initializeChunkLighting(c)
		n := storageBytes(c)
		total += n
		maximum = max(maximum, n)
	}
	old := unsafe.Sizeof(Chunk{}) - 4*unsafe.Sizeof(chunkPlane{}) + 4*16*256*16
	t.Logf("empty Chunk=%dB dense baseline=%dB generated mean=%dB max=%dB saved=%.1f%% (excludes slice/map backing allocations)", unsafe.Sizeof(Chunk{}), old, total/count, maximum, 100*(1-float64(total)/float64(count*old)))
	if total >= count*old {
		t.Fatal("no resident voxel saving")
	}
}

func storageMeshFixture() (*RenderAssets, *Chunk) {
	initBlockRegistry()
	c := new(Chunk)
	generateChunkData(12345, 0, 0, c)
	initializeChunkLighting(c)
	return &RenderAssets{}, c
}

func TestChunkStorageMeshEquivalence(t *testing.T) {
	a, c := storageMeshFixture()
	blocks, meta, sky, light := c.blocks.Dense(), c.meta.Dense(), c.skyLight.Dense(), c.blockLight.Dense()
	valid := func(x, y, z int) bool { return x >= 0 && x < 16 && y >= 0 && y < 256 && z >= 0 && z < 16 }
	for sec := 0; sec < sectionCount; sec++ {
		sparse := a.buildAllMeshData(&c.heightMap, 0, 0, sec*16, (sec+1)*16,
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				return c.blocks.Get(x, y, z)
			},
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 15
				}
				return max(c.skyLight.Get(x, y, z), c.blockLight.Get(x, y, z))
			},
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				return c.meta.Get(x, y, z)
			}, 12345, nil)
		dense := a.buildAllMeshData(&c.heightMap, 0, 0, sec*16, (sec+1)*16,
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				return blocks[x][y][z]
			},
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 15
				}
				return max(sky[x][y][z], light[x][y][z])
			},
			func(x, y, z int) byte {
				if !valid(x, y, z) {
					return 0
				}
				return meta[x][y][z]
			}, 12345, nil)
		equal := reflect.DeepEqual(sparse, dense)
		releaseMeshResults(sparse)
		releaseMeshResults(dense)
		if !equal {
			t.Fatalf("section %d mesh differs", sec)
		}
	}
}
