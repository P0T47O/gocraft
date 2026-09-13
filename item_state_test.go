package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyGroundTools(t *testing.T) {
	initBlockRegistry()
	var buf bytes.Buffer
	buf.WriteString(entityMagic)
	buf.WriteByte(7)
	writeUint32(&buf, 1)
	WriteString(&buf, "old-drop")
	buf.WriteByte(byte(EntityItem))
	for i := 0; i < 3; i++ {
		binary.Write(&buf, binary.LittleEndian, float64(0))
	}
	for i := 0; i < 2; i++ {
		binary.Write(&buf, binary.LittleEndian, float32(0))
	}
	buf.WriteByte(itemWoodPickaxe)
	binary.Write(&buf, binary.LittleEndian, int32(3))
	binary.Write(&buf, binary.LittleEndian, float32(12))
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, entityFile), buf.Bytes(), 0600)
	w := NewClientWorld()
	defer w.Close()
	if _, err := LoadEntities(root, w); err != nil {
		t.Fatal(err)
	}
	if len(w.entities) != 3 {
		t.Fatal("legacy ground stack not split")
	}
	for _, e := range w.entities {
		d := e.(*ItemEntity)
		if d.Count != 1 || d.Damage != 0 || d.Age != 12 {
			t.Fatal("legacy ground state changed")
		}
	}
	if err := SaveEntities(root, w); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEntities(root, w); err != nil || len(w.entities) != 3 {
		t.Fatal("migrated ground save", err)
	}
}

func TestGroundMergeAndDeathKeepDamage(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks[2][69][2] = blockStone
	a := &ItemEntity{BaseEntity: BaseEntity{UUID: "a", Type: EntityItem, X: 2, Y: 70, Z: 2}, ItemStack: Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 10}}
	b := &ItemEntity{BaseEntity: BaseEntity{UUID: "b", Type: EntityItem, X: 2, Y: 70, Z: 2}, ItemStack: Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 10}}
	w.entities = []Entity{a, b}
	a.Tick(w)
	if a.Count != 1 || b.Dead {
		t.Fatal("ground tools merged")
	}
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "victim", Type: EntityPlayer}, GameMode: ModeSurvival}
	p.initVitals()
	p.Inventory.Slots[0] = a.ItemStack
	p.CursorItem = Item{ID: int32(itemWoodAxe), Count: 1, Damage: 21}
	w.entities = []Entity{p}
	s := &Server{World: w, Clients: map[string]*ClientConnection{}}
	p.Vitals.grace = 0
	s.hurtPlayer(p, maxHealth, "test")
	var damage int32
	for _, e := range w.entities {
		if d, ok := e.(*ItemEntity); ok {
			damage += d.Damage
		}
	}
	if damage != 31 {
		t.Fatal("death lost wear", damage)
	}
}

func TestItemDefinitionsAndTransfers(t *testing.T) {
	initBlockRegistry()
	if Blocks[itemWoodPickaxe] != nil || GetItem(itemWoodPickaxe).MaxDurability != 59 || StackLimit(int32(itemWoodPickaxe)) != 1 {
		t.Fatal("tool definition not separated")
	}
	if GetItem(blockStone).PlaceBlock != blockStone {
		t.Fatal("missing placeable item mapping")
	}
	tool := Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 20}
	var inv Inventory
	if inv.AddStack(tool) != 0 || inv.AddStack(tool) != 0 || inv.Slots[0].Count != 1 || inv.Slots[1] != tool {
		t.Fatal("tools stacked or damage lost")
	}
	var cursor Item
	inv.Click(0, 1, &cursor)
	if cursor != tool || inv.Slots[0] != (Item{}) {
		t.Fatal("split lost state")
	}
	inv.Click(2, 1, &cursor)
	if inv.Slots[2] != tool || cursor != (Item{}) {
		t.Fatal("place lost state")
	}
	inv.Click(2, 2, &cursor)
	if inv.Slots[9] != tool {
		t.Fatal("shift lost state")
	}
	cursor = Item{ID: tool.ID, Count: 1, Damage: 30}
	inv.Click(9, 0, &cursor)
	if cursor != tool || inv.Slots[9].Damage != 30 {
		t.Fatal("different tools were merged")
	}
	tool.Wear(38)
	if tool.Damage != 58 {
		t.Fatal("wrong wear")
	}
	tool.Wear(1)
	if tool != (Item{}) {
		t.Fatal("broken tool retained")
	}
}

func TestDurabilityPacketAndSaves(t *testing.T) {
	initBlockRegistry()
	stack := Item{ID: int32(itemDiamondPickaxe), Count: 1, Damage: 456}
	pkt := &PacketInventoryUpdate{SlotID: 4, ItemID: stack.ID, Count: stack.Count, Damage: stack.Damage}
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	var decoded PacketInventoryUpdate
	if err := decoded.Decode(&buf); err != nil || decoded != *pkt {
		t.Fatal("packet lost damage", err)
	}
	w := NewClientWorld()
	defer w.Close()
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "owner", Type: EntityPlayer}, GameMode: ModeSurvival, CursorItem: stack}
	p.Inventory.Slots[2] = stack
	w.entities = []Entity{p, &ItemEntity{BaseEntity: BaseEntity{UUID: "drop", Type: EntityItem}, ItemStack: stack}}
	root := t.TempDir()
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	if err := SaveEntities(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if _, err := LoadEntities(root, loaded); err != nil {
		t.Fatal(err)
	}
	if err := loadSurvivalPlayers(root, loaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.entities[0], p) || loaded.entities[1].(*ItemEntity).ItemStack != stack {
		t.Fatal("save lost item state")
	}
}

func TestLegacyToolOverflowMigration(t *testing.T) {
	initBlockRegistry()
	p := PlayerEntity{BaseEntity: BaseEntity{UUID: "legacy", Type: EntityPlayer}, GameMode: ModeSurvival}
	for i := range p.Inventory.Slots {
		p.Inventory.Slots[i] = Item{ID: int32(blockDirt), Count: 64}
	}
	p.Inventory.Slots[0] = Item{ID: int32(itemWoodPickaxe), Count: 64}
	p.CursorItem = Item{ID: int32(itemWoodAxe), Count: 2}
	data, _ := json.Marshal(survivalPlayerSave{Version: 1, Players: map[string]PlayerEntity{"legacy": p}})
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "players.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	w := NewClientWorld()
	defer w.Close()
	if err := loadSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	got := w.entities[0].(*PlayerEntity)
	if got.Inventory.Slots[0].Count != 1 || got.CursorItem.Count != 1 || len(got.PendingItems) != 2 || got.PendingItems[0].Count != 63 {
		t.Fatal("legacy tools lost", got.PendingItems)
	}
	got.Inventory.Slots[1] = Item{}
	got.claimPendingItems()
	if got.Inventory.Slots[1].ID != int32(itemWoodPickaxe) || got.Inventory.Slots[1].Damage != 0 || got.PendingItems[0].Count != 62 {
		t.Fatal("pending tools not claimed")
	}
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	if err := loadSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedToolDropPickupAndWear(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks[2][70][2] = blockStone
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "miner", Type: EntityPlayer, X: 2, Y: 71, Z: 2}, GameMode: ModeSurvival, SelectedSlot: 1}
	p.Inventory.Slots[0] = Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 10}
	p.Inventory.Slots[1] = Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 57}
	w.entities = []Entity{p}
	s := &Server{World: w, LastSentPos: make(map[string][3]float64), LastSentMeta: make(map[string]int32), Clients: map[string]*ClientConnection{}}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockAir}})
	if p.Inventory.Slots[1].Damage != 58 {
		t.Fatal("mining did not wear tool")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerAction{Value: int32(itemWoodPickaxe) | 1<<8}})
	if p.Inventory.Slots[0].Damage != 10 || p.Inventory.Slots[1] != (Item{}) {
		t.Fatal("wrong tool dropped")
	}
	var tool *ItemEntity
	for _, e := range w.entities {
		if d, ok := e.(*ItemEntity); ok && d.ID == int32(itemWoodPickaxe) {
			tool = d
		}
	}
	if tool == nil || tool.Damage != 58 {
		t.Fatal("drop lost damage")
	}
	tool.X, tool.Y, tool.Z = p.X, p.Y, p.Z
	tool.PickupDelay = 0
	s.UpdateEntities()
	if p.Inventory.Slots[1].Damage != 58 {
		t.Fatal("pickup reset damage")
	}
	c.blocks[2][70][2] = blockStone
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 2, Y: 70, Z: 2, BlockID: blockAir}})
	if p.Inventory.Slots[1] != (Item{}) {
		t.Fatal("tool did not break")
	}
}
