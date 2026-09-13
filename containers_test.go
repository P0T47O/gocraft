package main

import (
	"bytes"
	"reflect"
	"testing"
)

func containerTestServer(t *testing.T) (*Server, *PlayerEntity, BlockPos) {
	t.Helper()
	initBlockRegistry()
	w := NewClientWorld()
	t.Cleanup(w.Close)
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks[2][70][2] = blockChest
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "builder", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
	w.entities = []Entity{p}
	s := &Server{World: w, SavePath: t.TempDir(), Clients: map[string]*ClientConnection{}}
	return s, p, BlockPos{2, 70, 2}
}
func TestChestTransfersAndStaleSessions(t *testing.T) {
	s, p, pos := containerTestServer(t)
	s.openContainer(p, pos)
	c := s.Containers[pos]
	session := s.ContainerSessions[p.UUID]
	tool := Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 23}
	p.Inventory.Slots[0] = tool
	click := func(slot, button int32) {
		s.clickContainer(p, &PacketContainerClick{Token: session.Token, Revision: c.Revision, Slot: slot, Button: button})
	}
	click(27, 2)
	if c.Slots[0] != tool || p.Inventory.Slots[0] != (Item{}) {
		t.Fatal("shift deposit lost item")
	}
	other := &PlayerEntity{BaseEntity: BaseEntity{UUID: "other", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival}
	s.World.entities = append(s.World.entities, other)
	s.openContainer(other, pos)
	stale := c.Revision
	click(0, 0)
	s.clickContainer(other, &PacketContainerClick{Token: s.ContainerSessions[other.UUID].Token, Revision: stale, Slot: 0})
	if other.CursorItem.ID != 0 || p.CursorItem != tool || c.Slots[0].ID != 0 {
		t.Fatal("concurrent take duplicated item")
	}
	click(28, 0)
	if p.Inventory.Slots[1] != tool {
		t.Fatal("cursor transfer lost wear")
	}
	s.closeContainer(p.UUID)
	s.openContainer(p, pos)
	s.clickContainer(p, &PacketContainerClick{Token: session.Token, Revision: c.Revision, Slot: 28, Button: 2})
	if p.Inventory.Slots[1] != tool {
		t.Fatal("old window remained valid")
	}
	p.X = 40
	s.clickContainer(p, &PacketContainerClick{Token: s.ContainerSessions[p.UUID].Token, Revision: c.Revision, Slot: 28, Button: 2})
	if _, ok := s.ContainerSessions[p.UUID]; ok {
		t.Fatal("distant container usable")
	}
}
func TestFurnaceFuelOutputAndSave(t *testing.T) {
	s, _, pos := containerTestServer(t)
	c := &BlockContainer{Pos: pos, Kind: blockFurnace, Slots: []ItemStack{{ID: int32(blockIronOre), Count: 9}, {ID: int32(itemCoal), Count: 1}, {}}}
	s.Containers = map[BlockPos]*BlockContainer{pos: c}
	for i := 0; i < 75; i++ {
		c.tick()
	}
	if c.Cook != 75 || c.Burn <= 0 || c.Slots[1].ID != 0 {
		t.Fatal("fuel/start failed")
	}
	if err := s.saveContainers(); err != nil {
		t.Fatal(err)
	}
	restored := &Server{SavePath: s.SavePath}
	if err := restored.loadContainers(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Containers[pos], c) {
		t.Fatal("furnace state lost")
	}
	c = restored.Containers[pos]
	for i := 75; i < 1600; i++ {
		c.tick()
	}
	if c.Slots[2] != (Item{ID: int32(itemIronIngot), Count: 8}) || c.Slots[0].Count != 1 {
		t.Fatal("coal should smelt eight", c)
	}
	c.tick()
	if c.Cook != 0 || c.Burn != 0 {
		t.Fatal("cooked without fuel")
	}
	c.Slots[2].Count = 64
	c.Slots[1] = Item{ID: int32(itemCoal), Count: 1}
	c.tick()
	if c.Slots[1].Count != 1 {
		t.Fatal("blocked output consumed new fuel")
	}
	c.Slots[2] = Item{}
	c.tick()
	c.Slots[0] = Item{ID: int32(blockGoldOre), Count: 1}
	c.tick()
	if c.Cook != 1 {
		t.Fatal("recipe swap carried progress")
	}
}
func TestFurnaceSlotRestrictionsAndContainerBreak(t *testing.T) {
	s, p, pos := containerTestServer(t)
	s.World.SetBlockAt(2, 70, 2, blockFurnace)
	s.openContainer(p, pos)
	c := s.Containers[pos]
	session := s.ContainerSessions[p.UUID]
	click := func(slot, button int32) {
		s.clickContainer(p, &PacketContainerClick{Token: session.Token, Revision: c.Revision, Slot: slot, Button: button})
	}
	p.CursorItem = Item{ID: int32(blockDirt), Count: 5}
	click(0, 0)
	click(1, 0)
	click(2, 0)
	if p.CursorItem.Count != 5 || c.Slots[0].ID != 0 || c.Slots[1].ID != 0 || c.Slots[2].ID != 0 {
		t.Fatal("invalid furnace insertion")
	}
	p.CursorItem = Item{}
	p.Inventory.Slots[0] = Item{ID: int32(blockIronOre), Count: 3}
	p.Inventory.Slots[1] = Item{ID: int32(itemCoal), Count: 2}
	click(3, 2)
	click(4, 2)
	if c.Slots[0].Count != 3 || c.Slots[1].Count != 2 {
		t.Fatal("shift routing failed")
	}
	c.Slots[2] = Item{ID: int32(itemIronIngot), Count: 4}
	click(2, 1)
	if p.CursorItem.Count != 2 || c.Slots[2].Count != 2 {
		t.Fatal("output extraction failed")
	}
	s.breakContainer(pos)
	s.breakContainer(pos)
	count := int32(0)
	drops := 0
	for _, e := range s.World.entities {
		if d, ok := e.(*ItemEntity); ok {
			count += d.Count
			drops++
		}
	}
	if count != 7 || drops != 3 || len(s.Containers) != 0 || len(s.ContainerSessions) != 0 {
		t.Fatal("break lost or duplicated contents", count, drops)
	}
}
func TestContainerPacketsAndLayout(t *testing.T) {
	initBlockRegistry()
	packet := &PacketContainerState{Token: 7, State: BlockContainer{Kind: blockChest, Slots: make([]ItemStack, 27)}}
	packet.State.Slots[2] = Item{ID: int32(itemIronPickaxe), Count: 1, Damage: 80}
	var buf bytes.Buffer
	if err := WritePacket(&buf, packet); err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadPacket(&buf)
	if err != nil || !reflect.DeepEqual(packet, decoded) {
		t.Fatal("container wire roundtrip", err)
	}
	for _, kind := range []byte{blockChest, blockFurnace} {
		for _, size := range [][2]float32{{800, 600}, {1280, 720}, {1871, 994}} {
			l := survivalLayout(size[0], size[1])
			for i := 0; i < containerSize(kind)+36; i++ {
				r := containerUISlot(l, kind, i)
				if r.X < 0 || r.Y < 0 || r.X+r.Width > size[0] || r.Y+r.Height > size[1] {
					t.Fatal("slot offscreen")
				}
			}
		}
	}
}

func TestCraftFurnaceToIronTool(t *testing.T) {
	s, p, pos := containerTestServer(t)
	InitRecipes()
	p.Inventory.Add(int32(blockCobblestone), 8)
	var furnace, ironPick *Recipe
	for _, r := range RecipeRegistry {
		if r.Result.ID == int32(blockFurnace) {
			furnace = r
		}
		if r.Result.ID == int32(itemIronPickaxe) {
			ironPick = r
		}
	}
	if furnace == nil || ironPick == nil || p.Inventory.Craft(furnace, 1) != 1 {
		t.Fatal("furnace recipe unavailable")
	}
	if !p.Inventory.Consume(int32(blockFurnace), 1) {
		t.Fatal("furnace missing")
	}
	s.World.SetBlockAt(2, 70, 2, blockFurnace)
	s.openContainer(p, pos)
	c := s.Containers[pos]
	c.Slots[0] = Item{ID: int32(blockIronOre), Count: 3}
	c.Slots[1] = Item{ID: int32(itemCoal), Count: 1}
	for i := 0; i < 600; i++ {
		c.tick()
	}
	s.clickContainer(p, &PacketContainerClick{Token: s.ContainerSessions[p.UUID].Token, Revision: c.Revision, Slot: 2, Button: 2})
	p.Inventory.Add(int32(itemStick), 2)
	if p.Inventory.Craft(ironPick, 1) != 1 || p.Inventory.CountItem(int32(itemIronPickaxe)) != 1 {
		t.Fatal("iron progression interrupted")
	}
	tool := Item{ID: int32(itemIronPickaxe), Count: 1, Damage: 61}
	s.World.SetBlockAt(2, 70, 2, blockChest)
	delete(s.Containers, pos)
	s.openContainer(p, pos)
	s.Containers[pos].Slots[0] = tool
	if err := s.saveContainers(); err != nil {
		t.Fatal(err)
	}
	restored := &Server{SavePath: s.SavePath}
	if err := restored.loadContainers(); err != nil || restored.Containers[pos].Slots[0] != tool {
		t.Fatal("chest save lost wear", err)
	}
}
