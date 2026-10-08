package main

import (
	"math"
	"testing"
)

func TestBedPlacementFacingAndPairedRemoval(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaw  float32
		face byte
	}{
		{"south", 0, faceSouth},
		{"east", math.Pi / 2, faceEast},
		{"north", math.Pi, faceNorth},
		{"west", -math.Pi / 2, faceWest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := hostileTestServer(t)
			p.GameMode, p.Yaw = ModeCreative, tc.yaw
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockBed, Meta: faceNorth}})
			dx, dz := faceOffset(tc.face)
			if s.World.BlockAt(8, 71, 8) != blockBed || s.World.MetaAt(8, 71, 8) != tc.face || s.World.BlockAt(8+dx, 71, 8+dz) != blockBed || s.World.MetaAt(8+dx, 71, 8+dz) != tc.face|shapeUpper {
				t.Fatalf("bed not placed as a %s-facing pair", tc.name)
			}
			if !bedPartsMatch(s.World, 8, 71, 8, tc.face) || !bedPartsMatch(s.World, 8+dx, 71, 8+dz, tc.face|shapeUpper) {
				t.Fatal("bed halves do not point to each other")
			}
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: int32(8 + dx), Y: 71, Z: int32(8 + dz), BlockID: blockAir}})
			if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8+dx, 71, 8+dz) != blockAir {
				t.Fatal("breaking the head left the foot behind")
			}
		})
	}
}

func TestBedRequiresTwoSupportedFreeLoadedBlocks(t *testing.T) {
	s, p := hostileTestServer(t)
	p.GameMode, p.Yaw = ModeSurvival, 0
	p.Inventory.Slots[0] = Item{ID: int32(blockBed), Count: 1}
	s.World.SetBlockAt(8, 71, 9, blockStone)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockBed, Meta: faceSouth}})
	if s.World.BlockAt(8, 71, 8) != blockAir || p.Inventory.Slots[0].Count != 1 {
		t.Fatal("obstructed bed placement consumed item or placed foot")
	}
	s.World.SetBlockAt(8, 71, 9, blockAir)
	s.World.SetBlockAt(8, 70, 9, blockAir)
	if s.World.canPlaceBed(8, 71, 8, faceSouth) {
		t.Fatal("bed accepted an unsupported head")
	}
	s.World.SetBlockAt(8, 70, 9, blockStone)
	if !s.World.canPlaceBed(8, 71, 8, faceSouth) {
		t.Fatal("valid two-block bed was rejected")
	}
	if s.World.canPlaceBed(15, 71, 8, faceEast) {
		t.Fatal("bed accepted an unloaded head chunk")
	}
	neighbor := lifecycleChunk(s.World, chunkKey{1, 0})
	neighbor.blocks.Set(0, 70, 8, blockStone)
	if !s.World.canPlaceBed(15, 71, 8, faceEast) {
		t.Fatal("bed rejected a supported head across a loaded chunk border")
	}
}

func TestBedClientPredictionAndBlastRemoveBothHalves(t *testing.T) {
	w := lifeTestWorld(t)
	hit := hitInfo{hit: true, x: 8, y: 70, z: 8, normal: gameVec3{Y: 1}}
	if _, _, _, ok := w.PlaceAdjacent(hit, blockBed, 0); !ok || w.BlockAt(8, 71, 8) != blockBed || w.BlockAt(8, 71, 9) != blockBed {
		t.Fatal("client failed to predict both halves")
	}
	w.RemoveBlock(8, 71, 9)
	if w.BlockAt(8, 71, 8) != blockAir || w.BlockAt(8, 71, 9) != blockAir {
		t.Fatal("client left a ghost half after breaking head")
	}
	s, _ := hostileTestServer(t)
	s.World.SetBlockAt(8, 71, 8, blockBed)
	s.World.SetMetaAt(8, 71, 8, faceSouth)
	s.World.SetBlockAt(8, 71, 9, blockBed)
	s.World.SetMetaAt(8, 71, 9, faceSouth|shapeUpper)
	s.explode(8.5, 71.5, 8.5, .65, 0, "test")
	if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8, 71, 9) != blockAir {
		t.Fatal("blast left a bed half behind")
	}
}

func TestBedLosesBothHalvesWhenSupportIsBroken(t *testing.T) {
	s, p := hostileTestServer(t)
	p.GameMode, p.Yaw = ModeCreative, 0
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockBed}})
	if s.World.BlockAt(8, 71, 9) != blockBed {
		t.Fatal("bed head not placed")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 70, Z: 9, BlockID: blockAir}})
	if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8, 71, 9) != blockAir {
		t.Fatal("unsupported bed half remained after its floor was mined")
	}
}

func TestRejectedBedMineRestoresBothClientHalves(t *testing.T) {
	s, p := hostileTestServer(t)
	s.World.SetBlockAt(8, 71, 8, blockBed)
	s.World.SetMetaAt(8, 71, 8, faceSouth)
	s.World.SetBlockAt(8, 71, 9, blockBed)
	s.World.SetMetaAt(8, 71, 9, faceSouth|shapeUpper)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 9, BlockID: blockAir}})
	if s.World.BlockAt(8, 71, 8) != blockBed || s.World.BlockAt(8, 71, 9) != blockBed {
		t.Fatal("unearned mining removed a bed half")
	}
	corrected := map[BlockPos]bool{}
	for len(s.Clients[p.UUID].Send) > 0 {
		if packet, ok := (<-s.Clients[p.UUID].Send).(*PacketBlockChange); ok && packet.BlockID == blockBed {
			corrected[BlockPos{packet.X, packet.Y, packet.Z}] = true
		}
	}
	if !corrected[BlockPos{8, 71, 8}] || !corrected[BlockPos{8, 71, 9}] {
		t.Fatal("server did not restore both predicted bed halves")
	}
}
