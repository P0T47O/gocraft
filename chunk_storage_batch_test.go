package main

import "testing"

func TestCopyColumnMatchesScalar(t *testing.T) {
	var p chunkPlane
	p.Fill(7)
	for y := 16; y < 48; y++ {
		p.Set(3, y, 5, byte(y%16))
	}
	for _, span := range [][2]int{{0, 1}, {0, 256}, {15, 33}, {31, 49}, {255, 256}} {
		for _, merge := range []bool{false, true} {
			dst := make([]byte, 256*18+5)
			for i := range dst {
				dst[i] = 9
			}
			p.CopyColumn(dst, 3, 18, 3, 5, span[0], span[1], merge)
			for i, v := range dst {
				want := byte(9)
				if i >= 3 && (i-3)%18 == 0 && (i-3)/18 < span[1]-span[0] {
					want = p.Get(3, span[0]+(i-3)/18, 5)
					if merge {
						want = max(want, 9)
					}
				}
				if v != want {
					t.Fatalf("span=%v merge=%v index=%d: %d != %d", span, merge, i, v, want)
				}
			}
		}
	}
}

func TestFreshPacketMetadataMatchesRebuild(t *testing.T) {
	initBlockRegistry()
	source := new(Chunk)
	generateChunkData(12345, 0, 0, source)
	source.blocks.Set(3, 255, 5, blockTorch)
	source.blocks.Set(1, 0, 2, blockTorch)
	initializeChunkLighting(source)
	w := NewClientWorld()
	defer w.Close()
	if !w.applyChunkPacket(chunkPacket(chunkKey{}, source)) {
		t.Fatal("packet rejected")
	}
	c := w.getChunkIfGenerated(0, 0)
	heights, sections, count := c.heightMap, c.sectionBlocks, c.torchCount
	torches := append([]blockPos(nil), c.torches...)
	c.rebuildHeightMap()
	c.rebuildTorchCount()
	if heights != c.heightMap || sections != c.sectionBlocks || count != c.torchCount || len(torches) != len(c.torches) {
		t.Fatal("derived metadata differs")
	}
	for i, p := range torches {
		if p != c.torches[i] {
			t.Fatal("torch order differs")
		}
	}
}
