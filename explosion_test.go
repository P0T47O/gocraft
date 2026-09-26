package main

import (
	"bytes"
	"testing"
)

func TestTNTIgnitionExplosionAndChain(t *testing.T) {
	s, p := hostileTestServer(t)
	s.World.SetBlockAt(8, 71, 8, blockTNT)
	s.World.SetBlockAt(9, 71, 8, blockTNT)
	s.World.SetBlockAt(8, 71, 9, blockStone)
	s.World.SetBlockAt(7, 71, 8, blockObsidian)
	s.World.SetBlockAt(8, 69, 8, blockBedrock)
	if !s.igniteTNT(8, 71, 8, 1) {
		t.Fatal("TNT did not ignite")
	}
	if s.World.BlockAt(8, 71, 8) != blockAir {
		t.Fatal("primed TNT stayed as a block")
	}
	s.UpdateEntities()
	if p.Vitals.Health >= maxHealth {
		t.Fatal("explosion did not hurt nearby player")
	}
	if s.World.BlockAt(8, 71, 9) != blockAir || s.World.BlockAt(9, 71, 8) != blockAir {
		t.Fatal("blast did not remove blocks")
	}
	if s.World.BlockAt(7, 71, 8) != blockObsidian || s.World.BlockAt(8, 69, 8) != blockBedrock {
		t.Fatal("blast removed protected blocks")
	}
	chain := 0
	for _, e := range s.World.entities {
		if _, ok := e.(*PrimedTNT); ok {
			chain++
		}
	}
	if chain != 1 {
		t.Fatalf("chain TNT count=%d", chain)
	}
	found := false
	for len(s.Clients[p.UUID].Send) > 0 {
		pkt := <-s.Clients[p.UUID].Send
		if event, ok := pkt.(*PacketExplosion); ok {
			found = true
			if len(event.Removed) == 0 {
				t.Fatal("explosion packet omitted terrain delta")
			}
			var wire bytes.Buffer
			if err := WritePacket(&wire, event); err != nil {
				t.Fatal(err)
			}
			decoded, err := ReadPacket(&wire)
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded.(*PacketExplosion).Removed) != len(event.Removed) {
				t.Fatal("terrain delta changed in transit")
			}
		}
	}
	if !found {
		t.Fatal("explosion event not sent")
	}
}

func TestExplosionPacketRejectsTruncatedBlocks(t *testing.T) {
	p := &PacketExplosion{X: 1, Y: 2, Z: 3, Radius: 4, Removed: []BlockPos{{1, 2, 3}}}
	var payload bytes.Buffer
	if err := p.Encode(&payload); err != nil {
		t.Fatal(err)
	}
	data := payload.Bytes()
	if err := (&PacketExplosion{}).Decode(bytes.NewBuffer(data[:len(data)-1])); err == nil {
		t.Fatal("truncated explosion accepted")
	}
}

func TestCreeperFuseUsesExplosionEvent(t *testing.T) {
	s, p := hostileTestServer(t)
	s.World.SetBlockAt(9, 71, 8, blockDirt)
	m := newMob("creeper", "exploding-creeper", 8, 70.501, 8)
	s.World.entities = append(s.World.entities, m)
	for i := 0; i < mobContent.Definitions["creeper"].AttackTicks; i++ {
		s.updateMobAI()
	}
	if m.Health != 0 || s.World.BlockAt(9, 71, 8) != blockAir {
		t.Fatal("creeper did not destroy nearby terrain")
	}
	for len(s.Clients[p.UUID].Send) > 0 {
		if _, ok := (<-s.Clients[p.UUID].Send).(*PacketExplosion); ok {
			return
		}
	}
	t.Fatal("creeper did not send explosion effect")
}

func TestExplosionCoverBlocksDamage(t *testing.T) {
	s, p := hostileTestServer(t)
	for y := 70; y <= 73; y++ {
		s.World.SetBlockAt(8, y, 9, blockObsidian)
	}
	s.explode(8.5, 71.5, 8.5, 4, 16, "TNT explosion")
	if p.Vitals.Health != maxHealth {
		t.Fatal("explosion damaged player through obsidian cover")
	}
}

func TestExplosionRemovesDoorOutsideBlastWhenSupportFalls(t *testing.T) {
	s, p := hostileTestServer(t)
	s.World.SetBlockAt(8, 70, 8, blockStone)
	s.World.SetBlockAt(8, 71, 8, blockWoodDoor)
	s.World.SetMetaAt(8, 71, 8, faceNorth)
	s.World.SetBlockAt(8, 72, 8, blockWoodDoor)
	s.World.SetMetaAt(8, 72, 8, faceNorth|shapeUpper)
	// The support is inside the radius; the door itself is outside it.
	s.explode(8.5, 68.5, 8.5, 2.9, 0, "test")
	for y := 70; y <= 72; y++ {
		if got := s.World.BlockAt(8, y, 8); got != blockAir {
			t.Fatalf("floating block at y=%d: %d", y, got)
		}
	}
	var event *PacketExplosion
	for len(s.Clients[p.UUID].Send) > 0 {
		if packet, ok := (<-s.Clients[p.UUID].Send).(*PacketExplosion); ok {
			event = packet
		}
	}
	if event == nil {
		t.Fatal("explosion event missing")
	}
	seen := make(map[BlockPos]bool)
	for _, pos := range event.Removed {
		if seen[pos] {
			t.Fatalf("duplicate removed block: %+v", pos)
		}
		seen[pos] = true
	}
	for y := int32(70); y <= 72; y++ {
		if !seen[BlockPos{8, y, 8}] {
			t.Fatalf("client was not told to remove y=%d", y)
		}
	}
	var wire bytes.Buffer
	if err := WritePacket(&wire, event); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPacket(&wire); err != nil {
		t.Fatal(err)
	}
}

func TestExplosionPairsDoorWhenOnlyUpperHalfIsHit(t *testing.T) {
	s, _ := hostileTestServer(t)
	s.World.SetBlockAt(8, 70, 8, blockObsidian)
	s.World.SetBlockAt(8, 71, 8, blockWoodDoor)
	s.World.SetBlockAt(8, 72, 8, blockWoodDoor)
	s.World.SetMetaAt(8, 72, 8, shapeUpper)
	s.explode(8.5, 73.5, 8.5, 1.1, 0, "test")
	if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8, 72, 8) != blockAir || s.World.BlockAt(8, 70, 8) != blockObsidian {
		t.Fatal("partial blast left a half-door or destroyed protected support")
	}
}

func TestPrimedTNTPersistsFuse(t *testing.T) {
	w := mobTestWorld(t)
	w.entities = []Entity{&PrimedTNT{BaseEntity: BaseEntity{UUID: "fuse", Type: EntityPrimedTNT, X: 8.5, Y: 71.5, Z: 8.5}, Fuse: 27}}
	root := t.TempDir()
	if err := SaveEntities(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if _, err := LoadEntities(root, loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.entities) != 1 {
		t.Fatalf("loaded entities=%d", len(loaded.entities))
	}
	primed, ok := loaded.entities[0].(*PrimedTNT)
	if !ok || primed.Fuse != 27 {
		t.Fatalf("saved TNT fuse: %#v", loaded.entities[0])
	}
}
