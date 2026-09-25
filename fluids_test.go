package main

import (
	"bytes"
	"testing"
)

// A flat stone shelf, a hole into a lower basin, and a second chunk make
// deterministic terrain fixtures without running the procedural generator.
func fluidScene(t *testing.T) *Server {
	t.Helper()
	s, _ := hostileTestServer(t)
	c := lifecycleChunk(s.World, chunkKey{1, 0})
	for x := 0; x < chunkWidth; x++ {
		for z := 0; z < chunkWidth; z++ {
			c.blocks.Set(x, 70, z, blockStone)
		}
	}
	return s
}

func fluidTicks(t *testing.T, s *Server, count int) {
	t.Helper()
	for range count {
		s.tickFluids()
		if len(s.fluidChanges) > fluidMaxChangesPerTick {
			t.Fatalf("fluid edit budget exceeded: %d", len(s.fluidChanges))
		}
		if len(s.fluidQueue)-s.fluidHead > fluidMaxQueued {
			t.Fatal("fluid queue exceeded budget")
		}
	}
}

func TestFluidFlatSpreadAndRecession(t *testing.T) {
	s := fluidScene(t)
	w := s.World
	w.SetBlockAt(8, 71, 8, blockWater)
	s.scheduleFluidAround(BlockPos{8, 71, 8})
	fluidTicks(t, s, 50)
	if w.BlockAt(9, 71, 8) != blockWater || w.MetaAt(9, 71, 8) != 1 {
		t.Fatal("source did not spread one level across flat shelf")
	}
	if w.BlockAt(8, 71, 15) != blockWater || w.BlockAt(8, 71, 16) == blockWater {
		t.Fatal("water exceeded seven-block horizontal range")
	}
	w.SetBlockAt(8, 71, 8, blockAir)
	s.scheduleFluidAround(BlockPos{8, 71, 8})
	fluidTicks(t, s, 100)
	if w.BlockAt(9, 71, 8) != blockAir || w.BlockAt(8, 71, 15) != blockAir {
		t.Fatal("unsupported water did not recede")
	}
}

func TestFluidWaterfallAndChunkSeam(t *testing.T) {
	s := fluidScene(t)
	w := s.World
	w.SetBlockAt(15, 74, 8, blockWater)
	s.scheduleFluidAround(BlockPos{15, 74, 8})
	fluidTicks(t, s, 50)
	if w.BlockAt(15, 73, 8) != blockWater || w.MetaAt(15, 73, 8) != fluidFalling {
		t.Fatal("water did not fall into lower basin")
	}
	if w.BlockAt(16, 71, 8) != blockWater {
		t.Fatal("flow did not cross loaded chunk boundary")
	}
}

func TestFluidLavaDelayAndSolidification(t *testing.T) {
	s := fluidScene(t)
	w := s.World
	w.SetBlockAt(8, 71, 8, blockLava)
	s.scheduleFluidAround(BlockPos{8, 71, 8})
	fluidTicks(t, s, 3)
	if w.BlockAt(9, 71, 8) != blockAir {
		t.Fatal("lava spread before scheduled delay")
	}
	fluidTicks(t, s, 1)
	if w.BlockAt(9, 71, 8) != blockLava {
		t.Fatal("lava failed to spread")
	}
	w.SetBlockAt(8, 71, 7, blockWater)
	s.scheduleFluidAround(BlockPos{8, 71, 7})
	fluidTicks(t, s, 6)
	if w.BlockAt(8, 71, 8) != blockObsidian {
		t.Fatal("lava source did not solidify to obsidian")
	}
}

func TestFluidPacketBoundsAndRoundTrip(t *testing.T) {
	p := &PacketFluidDelta{Changes: []FluidChange{{1, 70, -3, blockWater, 7}, {2, 71, 4, blockLava, 8}}}
	var wire bytes.Buffer
	if err := WritePacket(&wire, p); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPacket(&wire)
	if err != nil {
		t.Fatal(err)
	}
	changes := got.(*PacketFluidDelta).Changes
	if len(changes) != 2 || changes[0] != p.Changes[0] || changes[1] != p.Changes[1] {
		t.Fatal("fluid packet round trip mismatch")
	}
	p.Changes = make([]FluidChange, 257)
	if err := p.Encode(&bytes.Buffer{}); err == nil {
		t.Fatal("oversize fluid packet accepted")
	}
}

func TestFluidChangeBudgetUnderBroadFront(t *testing.T) {
	s := fluidScene(t)
	for x := 1; x < 15; x++ {
		for z := 1; z < 15; z++ {
			if (x+z)%3 == 0 {
				s.World.SetBlockAt(x, 71, z, blockWater)
				s.scheduleFluidAround(BlockPos{int32(x), 71, int32(z)})
			}
		}
	}
	fluidTicks(t, s, 20)
}

func TestFluidMeshHasSteppedSurfaceAndOnlyOneLip(t *testing.T) {
	initBlockRegistry()
	var heights [chunkWidth][chunkWidth]int16
	for _, tc := range []struct {
		block byte
		pass  string
	}{{blockWater, "water"}, {blockLava, "opaque"}} {
		a := &RenderAssets{}
		data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
			func(x, y, z int) byte {
				if y == 8 && z == 8 && (x == 8 || x == 9) {
					return tc.block
				}
				return blockAir
			},
			func(int, int, int) byte { return 15 },
			func(x, y, z int) byte {
				if x == 9 {
					return 4
				}
				return 0
			}, 42)
		faces, topFull, topLow, lips := 0, 0, 0, 0
		for _, meshes := range data[tc.pass] {
			for _, mesh := range meshes {
				faces += mesh.vertCount / 4
				for i := 0; i < mesh.vertCount; i += 4 {
					n := mesh.normals[i*3 : i*3+3]
					v := mesh.vertices[i*3 : i*3+12]
					if almostMesh(n[1], 1) && almostMesh(v[1], 8.5) {
						topFull++
					}
					if almostMesh(n[1], 1) && almostMesh(v[1], 8.1) {
						topLow++
					}
					if almostMesh(n[0], 1) && almostMesh(v[0], 8.5) && almostMesh(v[1], 8.5) && almostMesh(v[7], 8.1) {
						lips++
					}
				}
			}
		}
		releaseMeshResults(data)
		if faces != 11 || topFull != 1 || topLow != 1 || lips != 1 {
			t.Fatalf("fluid %d: faces=%d full=%d low=%d lips=%d", tc.block, faces, topFull, topLow, lips)
		}
	}
}
