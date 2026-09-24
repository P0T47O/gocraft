package main

import (
	"bytes"
	"testing"
	"time"
)

func TestMiningActionPacketRoundTrip(t *testing.T) {
	for _, action := range []int32{0, 1, 2} {
		var wire bytes.Buffer
		if err := WritePacket(&wire, &PacketBlockInteract{X: -23, Y: 70, Z: 42, Action: action}); err != nil {
			t.Fatal(err)
		}
		packet, err := ReadPacket(&wire)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := packet.(*PacketBlockInteract)
		if !ok || got.Action != action || got.X != -23 || got.Y != 70 || got.Z != 42 {
			t.Fatalf("action %d decoded as %#v", action, packet)
		}
	}
}

func testAuthorityServer(t *testing.T) (*Server, *PlayerEntity, *Chunk) {
	t.Helper()
	w := NewClientWorld()
	t.Cleanup(w.Close)
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks.Set(2, 70, 2, blockStone)
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "tester", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
	p.Inventory.Slots[0] = Item{ID: int32(itemWoodPickaxe), Count: 1}
	w.entities = []Entity{p}
	conn := &ClientConnection{Name: p.UUID, Send: make(chan Packet, 128)}
	s := &Server{World: w, Clients: map[string]*ClientConnection{p.UUID: conn}, miningSessions: make(map[string]miningSession)}
	return s, p, c
}

func TestServerModeAndCommandAuthorization(t *testing.T) {
	initBlockRegistry()
	s, p, c := testAuthorityServer(t)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketGameMode{Mode: ModeCreative}})
	if p.GameMode != ModeSurvival {
		t.Fatal("remote client switched itself to Creative")
	}
	select {
	case packet := <-s.Clients[p.UUID].Send:
		mode, ok := packet.(*PacketGameMode)
		if !ok || mode.Mode != ModeSurvival {
			t.Fatalf("denied mode did not receive authoritative correction: %T", packet)
		}
	default:
		t.Fatal("denied mode received no correction")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketChat{Message: "/give 1 64"}})
	if p.Inventory.Slots[1].ID != 0 {
		t.Fatal("dedicated client granted itself items")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketChat{Message: "/tp 500 80 500"}})
	for len(s.Clients[p.UUID].Send) > 0 {
		if packet := <-s.Clients[p.UUID].Send; packet.ID() == IDPlayerMove {
			t.Fatal("dedicated client received unauthorized teleport")
		}
	}
	s.LocalCheats = true
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketGameMode{Mode: ModeCreative}})
	if p.GameMode != ModeCreative {
		t.Fatal("integrated singleplayer mode switch was denied")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockAir}})
	if c.blocks.Get(2, 70, 2) != blockAir {
		t.Fatal("integrated Creative instant mining was rejected")
	}
	c.blocks.Set(10, 70, 2, blockStone)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 10, Y: 70, Z: 2, BlockID: blockAir}})
	if c.blocks.Get(10, 70, 2) != blockAir {
		t.Fatal("Creative's normal eight-block reach was reduced")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketChat{Message: "/give 1 1"}})
	if p.Inventory.Slots[1].ID == 0 {
		t.Fatal("integrated singleplayer command was denied")
	}
}

func TestDedicatedLoginDemotesSavedCreativeMode(t *testing.T) {
	initBlockRegistry()
	s, p, _ := testAuthorityServer(t)
	s.HasSavedPos = true
	s.PendingChunks = make(map[chunkKey][]string)
	p.GameMode = ModeCreative
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketLogin{Username: p.UUID, ProtocolVersion: protocolVersion}})
	if p.GameMode != ModeSurvival {
		t.Fatal("saved Creative mode bypassed dedicated-server policy")
	}
}

func TestServerMiningRequiresStartAndTime(t *testing.T) {
	initBlockRegistry()
	s, p, c := testAuthorityServer(t)
	pos := BlockPos{2, 70, 2}
	breakPacket := PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: pos.X, Y: pos.Y, Z: pos.Z, BlockID: blockAir}}
	start := PacketWrapper{From: p.UUID, Packet: &PacketBlockInteract{X: pos.X, Y: pos.Y, Z: pos.Z, Action: 1}}
	s.HandlePacket(breakPacket)
	if c.blocks.Get(2, 70, 2) != blockStone {
		t.Fatal("break without start was accepted")
	}
	s.HandlePacket(start)
	s.HandlePacket(breakPacket)
	if c.blocks.Get(2, 70, 2) != blockStone || len(s.miningSessions) != 0 {
		t.Fatal("early break was accepted or mining session was reusable")
	}
	s.HandlePacket(start)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketSlotChange{Slot: 0}})
	if _, ok := s.miningSessions[p.UUID]; !ok {
		t.Fatal("unchanged selected slot canceled the mining session")
	}
	session := s.miningSessions[p.UUID]
	session.started = time.Now().Add(-2 * time.Second)
	s.miningSessions[p.UUID] = session
	s.HandlePacket(breakPacket)
	if c.blocks.Get(2, 70, 2) != blockAir {
		t.Fatal("legitimate completed mining was rejected")
	}
	c.blocks.Set(2, 70, 2, blockStone)
	s.HandlePacket(breakPacket)
	if c.blocks.Get(2, 70, 2) != blockStone {
		t.Fatal("completed mining session was replayed")
	}
}

func TestServerMiningRejectsChangedTargetToolAndReach(t *testing.T) {
	initBlockRegistry()
	for _, change := range []string{"block", "tool", "slot", "reach", "cancel"} {
		t.Run(change, func(t *testing.T) {
			s, p, c := testAuthorityServer(t)
			pos := BlockPos{2, 70, 2}
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockInteract{X: pos.X, Y: pos.Y, Z: pos.Z, Action: 1}})
			session := s.miningSessions[p.UUID]
			session.started = time.Now().Add(-2 * time.Second)
			s.miningSessions[p.UUID] = session
			switch change {
			case "block":
				c.blocks.Set(2, 70, 2, blockDirt)
			case "tool":
				p.Inventory.Slots[0] = Item{}
			case "slot":
				s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketSlotChange{Slot: 1}})
			case "reach":
				p.X = 30
			case "cancel":
				s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockInteract{X: pos.X, Y: pos.Y, Z: pos.Z, Action: 2}})
			}
			want := c.blocks.Get(2, 70, 2)
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: pos.X, Y: pos.Y, Z: pos.Z, BlockID: blockAir}})
			if got := c.blocks.Get(2, 70, 2); got != want {
				t.Fatalf("%s change allowed mining: got %d want %d", change, got, want)
			}
		})
	}
}
