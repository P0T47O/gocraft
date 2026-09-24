package main

import (
	"testing"
	"time"
)

func finishTestMining(s *Server, player *PlayerEntity, pos BlockPos) {
	s.HandlePacket(PacketWrapper{From: player.UUID, Packet: &PacketBlockInteract{X: pos.X, Y: pos.Y, Z: pos.Z, Action: 1}})
	if session, ok := s.miningSessions[player.UUID]; ok {
		seconds := MiningSeconds(session.tool, session.block)
		session.started = time.Now().Add(-time.Duration(float64(seconds+1) * float64(time.Second)))
		s.miningSessions[player.UUID] = session
	}
	s.HandlePacket(PacketWrapper{From: player.UUID, Packet: &PacketBlockChange{X: pos.X, Y: pos.Y, Z: pos.Z, BlockID: blockAir}})
}

func TestMiningRules(t *testing.T) {
	initBlockRegistry()
	for _, tc := range []struct {
		block, tool byte
		harvest     bool
		seconds     float32
	}{
		{blockCobblestone, 0, false, 10},
		{blockCobblestone, itemWoodPickaxe, true, 1.5},
		{blockCobblestone, itemDiamondAxe, false, 10},
		{blockLog, 0, true, 3},
		{blockLog, itemWoodAxe, true, 1.5},
		{blockDirt, 0, true, .75},
		{blockDirt, itemWoodShovel, true, .375},
		{blockDiamondOre, itemGoldPickaxe, false, 1.25},
		{blockDiamondOre, itemIronPickaxe, true, .75},
		{blockObsidian, itemDiamondPickaxe, true, 9.375},
		{blockObsidian, 0, false, 250},
		{blockTorch, 0, true, 0},
		{blockBedrock, 0, true, -1},
	} {
		if CanHarvest(tc.tool, tc.block) != tc.harvest || MiningSeconds(tc.tool, tc.block) != tc.seconds {
			t.Errorf("block %d tool %d: harvest=%v seconds=%v", tc.block, tc.tool, CanHarvest(tc.tool, tc.block), MiningSeconds(tc.tool, tc.block))
		}
	}
	for _, d := range Blocks {
		if d == nil || d.ID == 0 || d.ID >= itemWoodPickaxe {
			continue
		}
		if !d.MiningConfigured {
			t.Errorf("unconfigured %s", d.Name)
		}
		for _, tool := range []byte{itemWoodPickaxe, itemStoneShovel, itemDiamondAxe, itemGoldPickaxe} {
			if MiningSeconds(tool, d.ID) > MiningSeconds(0, d.ID) {
				t.Errorf("tool slower than hand: %s", d.Name)
			}
		}
	}
}

func TestClientMiningStartDoesNotCountUnspentFrame(t *testing.T) {
	initBlockRegistry()
	preserveWindowInput(t)
	oldMode, oldDelta := currentGameMode, gameFrameDelta
	t.Cleanup(func() { currentGameMode, gameFrameDelta = oldMode, oldDelta })
	currentGameMode = ModeSurvival
	setGameFrameTime(0.05)
	windowFrame = windowInputFrame{Width: 1280, Height: 720}
	windowFrame.MouseDown[mouseLeft] = true
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks.Set(2, 70, 4, blockStone)
	camera := gameCamera{Position: newGameVec3(2, 72, 2), Target: newGameVec3(2, 70, 4), Up: newGameVec3(0, 1, 0)}
	state := &InputState{SelectedSlot: 0}
	state.InitFromCamera(camera)
	peer := &Client{Outgoing: make(chan Packet, 8), done: make(chan struct{})}
	hit := HandleInput(w, &camera, state, peer)
	if !hit.hit || hit.x != 2 || hit.y != 70 || hit.z != 4 {
		t.Fatalf("test ray missed stone: %+v", hit)
	}
	if state.MiningProgress != 0 || len(peer.Outgoing) != 1 {
		t.Fatalf("start frame credited mining: progress %.3f, packets %d", state.MiningProgress, len(peer.Outgoing))
	}
	if start, ok := (<-peer.Outgoing).(*PacketBlockInteract); !ok || start.Action != 1 {
		t.Fatalf("first mining packet was not start: %#v", start)
	}
}

func TestServerMiningDrops(t *testing.T) {
	initBlockRegistry()
	for _, tc := range []struct {
		name              string
		block, tool, drop byte
	}{
		{"bare cobble", blockCobblestone, 0, 0},
		{"pick cobble", blockCobblestone, itemWoodPickaxe, blockCobblestone},
		{"axe stone", blockStone, itemDiamondAxe, 0},
		{"stone conversion", blockStone, itemWoodPickaxe, blockCobblestone},
		{"gold insufficient", blockDiamondOre, itemGoldPickaxe, 0},
		{"iron diamond", blockDiamondOre, itemIronPickaxe, itemDiamond},
		{"glass", blockGlass, itemDiamondPickaxe, 0},
		{"ice", blockIce, itemDiamondPickaxe, 0},
		{"hand log", blockLog, 0, blockLog},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewClientWorld()
			defer w.Close()
			c := lifecycleChunk(w, chunkKey{0, 0})
			c.blocks.Set(2, 70, 2, tc.block)
			p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "miner", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
			if tc.tool != 0 {
				p.Inventory.Slots[0] = Item{ID: int32(tc.tool), Count: 1}
			}
			w.entities = []Entity{p}
			s := &Server{World: w, Clients: map[string]*ClientConnection{}}
			finishTestMining(s, p, BlockPos{2, 70, 2})
			if w.BlockAt(2, 70, 2) != blockAir {
				t.Fatal("block not removed")
			}
			count := 0
			for _, e := range w.entities {
				if item, ok := e.(*ItemEntity); ok {
					count++
					if item.ID != int32(tc.drop) || item.Count != 1 {
						t.Fatalf("unexpected drop %+v", item)
					}
				}
			}
			if (tc.drop == 0 && count != 0) || (tc.drop != 0 && count != 1) {
				t.Fatalf("drop entities: %d", count)
			}
		})
	}
}
