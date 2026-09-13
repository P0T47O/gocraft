package main

import "testing"

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
			c.blocks[2][70][2] = tc.block
			p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "miner", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
			if tc.tool != 0 {
				p.Inventory.Slots[0] = Item{ID: int32(tc.tool), Count: 1}
			}
			w.entities = []Entity{p}
			s := &Server{World: w, Clients: map[string]*ClientConnection{}}
			packet := PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockAir}}
			s.HandlePacket(packet)
			s.HandlePacket(packet)
			if w.BlockAt(2, 70, 2) != blockAir {
				t.Fatal("block not removed")
			}
			count := 0
			for _, e := range w.entities {
				if item, ok := e.(*ItemEntity); ok {
					count++
					if item.ItemID != tc.drop || item.Count != 1 {
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
