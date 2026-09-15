package main

import (
	"bytes"
	"testing"
)

func TestChunkLightRoundTripAndApply(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{-2, 3})
	c.blocks[3][20][4] = blockStone
	c.rebuildTorchCount()
	clear(c.sectionDirty)
	source := new(Chunk)
	source.skyLight[3][20][4], source.blockLight[3][20][4] = 7, 11
	p := chunkLightPacket(chunkKey{-2, 3}, source, 1<<1)
	if len(p.Data) != 4096 {
		t.Fatal("light packet unexpectedly includes whole chunk")
	}
	var wire bytes.Buffer
	if err := WritePacket(&wire, p); err != nil {
		t.Fatal(err)
	}
	r, err := ReadPacket(&wire)
	if err != nil {
		t.Fatal(err)
	}
	if !w.applyChunkLight(r.(*PacketChunkLight)) {
		t.Fatal("delta rejected")
	}
	if c.skyLight[3][20][4] != 7 || c.blockLight[3][20][4] != 11 || c.blocks[3][20][4] != blockStone {
		t.Fatal("light update changed geometry or lost light")
	}
	versions := append([]uint32(nil), c.meshVersion...)
	w.applyChunkLight(p)
	for sec, v := range versions {
		if c.meshVersion[sec] != v {
			t.Fatal("duplicate light invalidated mesh")
		}
	}
	p.CX = 100
	if w.applyChunkLight(p) {
		t.Fatal("accepted delta before initial chunk")
	}
}

func TestChunkLightValidation(t *testing.T) {
	for _, mask := range []uint16{1, 0x8000, 0xffff} {
		p := &PacketChunkLight{CX: -123, CZ: 456, Sections: mask, Data: make([]byte, chunkLightSize(mask))}
		var b bytes.Buffer
		if err := p.Encode(&b); err != nil {
			t.Fatal(err)
		}
		data := append([]byte(nil), b.Bytes()...)
		for _, bad := range [][]byte{data[:len(data)-1], append(append([]byte(nil), data...), 0)} {
			if err := new(PacketChunkLight).Decode(bytes.NewBuffer(bad)); err == nil {
				t.Fatal("accepted truncated/trailing data")
			}
		}
	}
	if err := new(PacketChunkLight).Encode(new(bytes.Buffer)); err == nil {
		t.Fatal("accepted empty mask")
	}
}

func TestLightQueueMergeAndFullRequestWins(t *testing.T) {
	s := &Server{PendingChunks: make(map[chunkKey][]string)}
	key := chunkKey{}
	s.queueChunkLightFor(key, "a", 1)
	s.queueChunkLightFor(key, "a", 2)
	if s.pendingLights[key]["a"] != 3 || len(s.PendingChunks[key]) != 1 {
		t.Fatal("light sections not merged")
	}
	s.queueChunkFor(key, "a")
	s.queueChunkLightFor(key, "a", 4)
	if s.pendingLights[key]["a"] != 0 || len(s.PendingChunks[key]) != 1 {
		t.Fatal("full request downgraded")
	}
}

func TestServerSendsLightOnlyToKnownChunks(t *testing.T) {
	initBlockRegistry()
	w := NewFlatWorld()
	defer w.Close()
	c := w.ensureChunk(0, 0)
	c.generated = true
	c.lightDirtySections = 1 << 2
	c.skyLight[1][35][1] = 8
	key := chunkKey{}
	w.lightChanged[key] = true
	known := &ClientConnection{KnownChunks: map[chunkKey]bool{key: true}, Send: make(chan Packet, 8)}
	fresh := &ClientConnection{KnownChunks: map[chunkKey]bool{}, Send: make(chan Packet, 8)}
	s := &Server{World: w, Clients: map[string]*ClientConnection{"known": known, "fresh": fresh}, PendingChunks: make(map[chunkKey][]string)}
	s.queueChunkFor(key, "fresh")
	s.processPendingChunks()
	select {
	case p := <-known.Send:
		if p.ID() != IDChunkLight {
			t.Fatal("known peer got full snapshot")
		}
	default:
		t.Fatal("no light packet")
	}
	select {
	case p := <-fresh.Send:
		if p.ID() != IDChunkData {
			t.Fatal("fresh peer got delta")
		}
	default:
		t.Fatal("no full packet")
	}
	if len(s.PendingChunks) != 0 || len(s.pendingLights) != 0 || c.lightDirtySections != 0 {
		t.Fatal("pending light state retained")
	}
}
