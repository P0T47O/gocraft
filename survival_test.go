package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSurvivalInventoryRoundTrip(t *testing.T) {
	initBlockRegistry()
	root := t.TempDir()
	w := NewClientWorld()
	defer w.Close()
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "builder", Type: EntityPlayer, X: 12, Y: 70, Z: -8}, GameMode: ModeSurvival, SelectedSlot: 7, CursorItem: Item{ID: int32(blockLog), Count: 5}}
	p.Inventory.Slots[0] = Item{ID: int32(blockDirt), Count: 63}
	p.Inventory.Slots[35] = Item{ID: int32(itemStonePickaxe), Count: 1}
	q := &PlayerEntity{BaseEntity: BaseEntity{UUID: "other", Type: EntityPlayer}, GameMode: ModeCreative}
	w.entities = []Entity{p, q}
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	// Replacing an existing save must work on Windows too.
	p.Inventory.Slots[0].Count = 17
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	loaded.entities = []Entity{&PlayerEntity{BaseEntity: BaseEntity{UUID: "builder", Type: EntityPlayer}}}
	if err := loadSurvivalPlayers(root, loaded); err != nil {
		t.Fatal(err)
	}
	s := &Server{World: loaded}
	if !reflect.DeepEqual(s.findPlayerEntity("builder"), p) || !reflect.DeepEqual(s.findPlayerEntity("other"), q) {
		t.Fatal("player state changed after save/load")
	}
	if len(loaded.entities) != 2 {
		t.Fatal("duplicate player on restore")
	}
	if err := os.WriteFile(filepath.Join(root, "players.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadSurvivalPlayers(root, loaded); err == nil {
		t.Fatal("corrupt save accepted")
	}
}

func TestInventoryOverflowConservesItems(t *testing.T) {
	var inv Inventory
	if rem := inv.Add(int32(blockDirt), 130); rem != 0 {
		t.Fatal(rem)
	}
	if inv.Slots[0].Count != 64 || inv.Slots[1].Count != 64 || inv.Slots[2].Count != 2 {
		t.Fatal(inv)
	}
	for i := 3; i < 36; i++ {
		inv.Slots[i] = Item{ID: int32(blockStone), Count: 64}
	}
	if rem := inv.Add(int32(blockDirt), 70); rem != 8 {
		t.Fatalf("remaining %d", rem)
	}
	before := inv
	if inv.Consume(int32(blockDirt), -3) || inv.Consume(int32(blockDirt), 1000) || inv != before {
		t.Fatal("invalid consume changed inventory")
	}
}

func TestBasicMiningCraftingAndPlacement(t *testing.T) {
	initBlockRegistry()
	InitRecipes()
	stableBlocks := []struct {
		name string
		got  byte
		want byte
	}{
		{"air", blockAir, 0}, {"torch", blockTorch, 10}, {"bedrock", blockBedrock, 16},
		{"oak_plank", blockPlankOak, 30}, {"crafting_table", blockCraftingTable, 41},
		{"chest", blockChest, 42}, {"furnace", blockFurnace, 43},
	}
	for _, tc := range stableBlocks {
		if tc.got != tc.want {
			t.Fatalf("stable block id changed: %s=%d want %d", tc.name, tc.got, tc.want)
		}
	}
	stableItems := []struct {
		name string
		got  byte
		want byte
	}{
		{"wood_pickaxe", itemWoodPickaxe, 100}, {"gold_pickaxe", itemGoldPickaxe, 104},
		{"wood_shovel", itemWoodShovel, 105}, {"wood_axe", itemWoodAxe, 110},
		{"coal", itemCoal, 115}, {"stick", itemStick, 119}, {"cooked_pork", itemCookedPork, 121},
	}
	for _, tc := range stableItems {
		if tc.got != tc.want {
			t.Fatalf("stable item id changed: %s=%d want %d", tc.name, tc.got, tc.want)
		}
	}
	if EntityPlayer != 0 || EntityPig != 1 || EntityItem != 2 {
		t.Fatalf("stable entity ids changed: player=%d pig=%d item=%d", EntityPlayer, EntityPig, EntityItem)
	}
	stableRecipes := []struct {
		id     int
		result int32
	}{
		{0, int32(blockPlankOak)},
		{23, int32(blockTorch)},
		{36, int32(blockFurnace)},
		{40, int32(blockChest)},
	}
	for _, tc := range stableRecipes {
		if tc.id >= len(RecipeRegistry) || RecipeRegistry[tc.id] == nil || RecipeRegistry[tc.id].ID != tc.id || RecipeRegistry[tc.id].Result.ID != tc.result {
			t.Fatalf("stable recipe id changed: id=%d", tc.id)
		}
	}
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks.Set(2, 70, 2, blockLog)
	c.blocks.Set(3, 70, 2, blockCraftingTable)
	c.blocks.Set(4, 70, 2, blockBedrock)
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "builder", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
	w.entities = []Entity{p}
	s := &Server{World: w, Clients: map[string]*ClientConnection{}}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockAir}})
	if w.BlockAt(2, 70, 2) != blockAir {
		t.Fatal("log not mined")
	}
	var drop *ItemEntity
	for _, e := range w.entities {
		if i, ok := e.(*ItemEntity); ok {
			drop = i
		}
	}
	if drop == nil || drop.ID != int32(blockLog) || drop.Count != 1 {
		t.Fatal("missing log drop")
	}
	drop.X = p.X
	drop.Y = p.Y
	drop.Z = p.Z
	drop.PickupDelay = 0
	s.UpdateEntities()
	if p.Inventory.Slots[0] != (Item{ID: int32(blockLog), Count: 1}) {
		t.Fatal("pickup failed")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketCraft{RecipeID: 0}})
	if p.Inventory.Slots[0] != (Item{ID: int32(blockPlankOak), Count: 4}) {
		t.Fatal("log recipe failed")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockPlankOak}})
	if w.BlockAt(2, 70, 2) != blockPlankOak || p.Inventory.Slots[0].Count != 3 {
		t.Fatal("placement failed to consume exactly one")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockPlankOak}})
	if p.Inventory.Slots[0].Count != 3 {
		t.Fatal("occupied placement consumed item")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 4, Y: 70, Z: 2, BlockID: blockAir}})
	if w.BlockAt(4, 70, 2) != blockBedrock {
		t.Fatal("bedrock mined")
	}
	if !s.hasCraftingStation(p, blockCraftingTable) {
		t.Fatal("nearby station unavailable")
	}
	p.X = 30
	if s.hasCraftingStation(p, blockCraftingTable) {
		t.Fatal("distant station available")
	}
	if harvestTier(MatGold) >= harvestTier(MatIron) {
		t.Fatal("gold harvest tier too high")
	}
}
