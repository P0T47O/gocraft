package main

import (
	"bytes"
	"testing"
)

func TestInventoryClickSplitTransferAndCapacity(t *testing.T) {
	var inv Inventory
	var cursor Item
	inv.Slots[0] = Item{ID: int32(blockLog), Count: 1}
	inv.Click(0, 1, &cursor)
	if cursor.Count != 1 || inv.Slots[0].ID != 0 {
		t.Fatal("right click must pick up a single item")
	}
	inv.Click(1, 1, &cursor)
	if cursor.ID != 0 || inv.Slots[1].Count != 1 {
		t.Fatal("right click must place one")
	}
	inv.Slots[0] = Item{ID: int32(blockLog), Count: 5}
	inv.Click(0, 1, &cursor)
	if cursor.Count != 3 || inv.Slots[0].Count != 2 {
		t.Fatal("odd stacks must round up when taking half")
	}
	before := inv
	inv.Click(0, 2, &cursor)
	if inv != before {
		t.Fatal("quick transfer should not alter a held cursor stack")
	}
	inv.Click(0, 0, &cursor)
	inv.Slots[9] = Item{ID: int32(blockLog), Count: 63}
	inv.Click(0, 2, &cursor)
	if inv.Slots[9].Count != 64 || inv.Slots[10].Count != 4 || inv.Slots[0].ID != 0 {
		t.Fatal("quick transfer must merge then fill")
	}
	for i := 0; i < 9; i++ {
		inv.Slots[i] = Item{ID: int32(blockStone), Count: 64}
	}
	before = inv
	inv.Click(10, 2, &cursor)
	if before != inv {
		t.Fatal("full destination lost items")
	}
	inv.Click(-1, 0, &cursor)
	inv.Click(36, 0, &cursor)
	inv.Click(0, 99, &cursor)
	if before != inv {
		t.Fatal("invalid clicks changed inventory")
	}
}

func TestCraftBatchesAreBoundedAndAtomic(t *testing.T) {
	r := &Recipe{Ingredients: []Item{{ID: int32(blockLog), Count: 1}}, Result: Item{ID: int32(blockPlankOak), Count: 4}}
	var inv Inventory
	inv.Add(int32(blockLog), 10)
	if n := inv.Craft(r, 64); n != 10 || inv.CountItem(int32(blockPlankOak)) != 40 || inv.CountItem(int32(blockLog)) != 0 {
		t.Fatal("batch crafting failed")
	}
	for i := range inv.Slots {
		inv.Slots[i] = Item{ID: int32(blockStone), Count: 64}
	}
	inv.Slots[0] = Item{ID: int32(blockLog), Count: 2}
	before := inv
	if n := inv.Craft(r, 1); n != 0 || inv != before {
		t.Fatal("full inventory must preserve ingredients")
	}
	inv.Slots[0].Count = 1
	if n := inv.Craft(r, 1); n != 1 || inv.Slots[0].ID != int32(blockPlankOak) {
		t.Fatal("consuming last ingredient should free output slot")
	}
	before = inv
	for _, n := range []int{-1, 0, 65, 1000000} {
		if inv.Craft(r, n) != 0 || inv != before {
			t.Fatal("invalid batch accepted")
		}
	}
}

func TestRecipeBookVariantsAndStationFilter(t *testing.T) {
	InitRecipes()
	var inv Inventory
	inv.Add(int32(blockPlankBirch), 12)
	inv.Add(int32(itemStick), 4)
	rows := recipeBook(&inv, 0, false)
	seen := map[int32]bool{}
	for _, r := range rows {
		if seen[r.Result.ID] {
			t.Fatal("duplicate output rows")
		}
		seen[r.Result.ID] = true
		if r.Result.ID == int32(itemWoodPickaxe) && r.Ingredients[0].ID != int32(blockPlankBirch) {
			t.Fatal("did not select available wood variant")
		}
	}
	for _, r := range recipeBook(&inv, 0, true) {
		if r.Station != 0 {
			t.Fatal("locked recipe marked ready")
		}
	}
	found := false
	for _, r := range recipeBook(&inv, blockCraftingTable, true) {
		if r.Result.ID == int32(itemWoodPickaxe) {
			found = true
		}
	}
	if !found {
		t.Fatal("workbench recipe missing")
	}
}

func TestCraftPacketBatchAndLegacy(t *testing.T) {
	var b bytes.Buffer
	want := PacketCraft{RecipeID: 3, Count: 64}
	want.Encode(&b)
	var got PacketCraft
	if err := got.Decode(&b); err != nil || got != want {
		t.Fatal("batch packet round trip", err, got)
	}
	b.Reset()
	WriteVarInt(&b, 3)
	got = PacketCraft{}
	if err := got.Decode(&b); err != nil || got.RecipeID != 3 || got.Count != 0 {
		t.Fatal("legacy packet", err, got)
	}
}

func TestSurvivalLayoutFitsAndSlotsDoNotOverlap(t *testing.T) {
	for _, size := range [][2]float32{{640, 480}, {1280, 720}, {1920, 1080}, {2560, 1080}} {
		l := survivalLayout(size[0], size[1])
		frame := l.Rect(0, 0, 1000, 620)
		if frame.X < 0 || frame.Y < 0 || frame.X+frame.Width > size[0] || frame.Y+frame.Height > size[1] {
			t.Fatal("panel outside viewport", size)
		}
		for i := 0; i < 36; i++ {
			a := l.Slot(i)
			if a.X < frame.X || a.Y < frame.Y || a.X+a.Width > frame.X+frame.Width || a.Y+a.Height > frame.Y+frame.Height {
				t.Fatal("slot outside panel")
			}
			for j := i + 1; j < 36; j++ {
				b := l.Slot(j)
				if a.X < b.X+b.Width && a.X+a.Width > b.X && a.Y < b.Y+b.Height && a.Y+a.Height > b.Y {
					t.Fatal("overlapping slots")
				}
			}
		}
	}
}

func TestServerInventoryAndBatchCraftRequests(t *testing.T) {
	initBlockRegistry()
	InitRecipes()
	w := NewClientWorld()
	defer w.Close()
	lifecycleChunk(w, chunkKey{0, 0})
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "crafter", Type: EntityPlayer, X: 2, Y: 70, Z: 2}, GameMode: ModeSurvival}
	w.entities = []Entity{p}
	s := &Server{World: w, Clients: map[string]*ClientConnection{}}
	p.Inventory.Add(int32(blockLog), 3)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketCraft{RecipeID: 0, Count: 64}})
	if p.Inventory.CountItem(int32(blockPlankOak)) != 12 || p.Inventory.CountItem(int32(blockLog)) != 0 {
		t.Fatal("server batch request failed")
	}
	var source int32
	for i, item := range p.Inventory.Slots {
		if item.ID == int32(blockPlankOak) {
			source = int32(i)
			break
		}
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketClickWindow{SlotID: source, Button: 2}})
	if p.Inventory.Slots[source].ID != 0 || p.Inventory.Slots[9].Count != 12 {
		t.Fatal("server quick transfer failed")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketClickWindow{SlotID: 9, Button: 1}})
	if p.Inventory.Slots[9].Count != 6 || p.CursorItem.Count != 6 {
		t.Fatal("server split failed")
	}
	p.Inventory.Add(int32(itemStick), 2)
	for _, r := range RecipeRegistry {
		if r.Result.ID == int32(itemWoodPickaxe) && r.Ingredients[0].ID == int32(blockPlankOak) {
			before := p.Inventory
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketCraft{RecipeID: int32(r.ID), Count: 64}})
			if p.Inventory != before {
				t.Fatal("batch request bypassed station requirement")
			}
			w.SetBlockAt(3, 70, 2, blockCraftingTable)
			s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketCraft{RecipeID: int32(r.ID), Count: 64}})
			if p.Inventory.CountItem(int32(itemWoodPickaxe)) != 1 {
				t.Fatal("workbench batch failed")
			}
			return
		}
	}
	t.Fatal("pickaxe recipe not found")
}
