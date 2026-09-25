package main

import (
	"bytes"
	"testing"
)

func TestArmorEquipmentAndDamage(t *testing.T) {
	initBlockRegistry()
	var inv Inventory
	inv.Slots[0] = Item{ID: int32(itemDiamondChestplate), Count: 1}
	inv.Click(0, 2, &Item{})
	if inv.Slots[0].ID != 0 || inv.Slots[armorSlotStart+1].ID != int32(itemDiamondChestplate) || inv.ArmorPoints() != 8 {
		t.Fatal("shift-click did not equip chestplate")
	}
	cursor := Item{ID: int32(blockDirt), Count: 1}
	inv.Click(armorSlotStart+1, 0, &cursor)
	if inv.Slots[armorSlotStart+1].ID != int32(itemDiamondChestplate) || cursor.ID != int32(blockDirt) {
		t.Fatal("non-armor replaced chestplate")
	}
	cursor = Item{ID: int32(itemGoldHelmet), Count: 1}
	inv.Click(armorSlotStart+1, 0, &cursor)
	if inv.Slots[armorSlotStart+1].ID != int32(itemDiamondChestplate) {
		t.Fatal("wrong armor part equipped")
	}
	remaining, worn := inv.absorbDamage(10, "Zombie")
	if remaining != 6 || !worn || inv.Slots[armorSlotStart+1].Damage != 1 {
		t.Fatalf("damage protection: %d %v %+v", remaining, worn, inv.Slots[armorSlotStart+1])
	}
	remaining, worn = inv.absorbDamage(10, "Starved")
	if remaining != 10 || worn || inv.Slots[armorSlotStart+1].Damage != 1 {
		t.Fatal("armor protected starvation or wore")
	}
	inv.Click(armorSlotStart+1, 2, &Item{})
	if inv.ArmorPoints() != 0 || inv.CountItem(int32(itemDiamondChestplate)) != 1 {
		t.Fatal("shift unequip lost armor")
	}
}

func TestArmorDurabilityAndVitals(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Inventory.Slots[armorSlotStart] = Item{ID: int32(itemIronHelmet), Count: 1, Damage: GetItem(itemIronHelmet).MaxDurability - 1}
	p.Vitals.grace, p.Vitals.cooldown = 0, 0
	s.hurtPlayer(p, 10, "Zombie")
	if p.Vitals.Health != 11 || p.Inventory.Slots[armorSlotStart].ID != 0 {
		t.Fatalf("armor breaking hit: health=%d slot=%+v", p.Vitals.Health, p.Inventory.Slots[armorSlotStart])
	}
	if got := vitalsPacket(p); got.Armor != 0 {
		t.Fatalf("broken armor remained in HUD: %d", got.Armor)
	}
	var wire bytes.Buffer
	p.Inventory.Slots[armorSlotStart+2] = Item{ID: int32(itemGoldLeggings), Count: 1}
	if err := WritePacket(&wire, vitalsPacket(p)); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPacket(&wire)
	if err != nil || got.(*PacketVitals).Armor != 3 {
		t.Fatal("armor points lost on wire", err)
	}
}

func TestArmorSaveAndRecipes(t *testing.T) {
	initBlockRegistry()
	InitRecipes()
	for tier, base := range []byte{itemGoldHelmet, itemIronHelmet, itemDiamondHelmet} {
		for part, count := range [...]int32{5, 8, 7, 4} {
			r := RecipeRegistry[48+tier*armorSlotCount+part]
			if r == nil || r.Result.ID != int32(base)+int32(part) || r.Ingredients[0].Count != count || r.Station != blockCraftingTable {
				t.Fatal("armor recipe missing")
			}
		}
	}
	w := lifeTestWorld(t)
	_, p := lifeTestPlayer(w)
	p.Inventory.Slots[armorSlotStart+3] = Item{ID: int32(itemIronBoots), Count: 1, Damage: 11}
	root := t.TempDir()
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if err := loadSurvivalPlayers(root, loaded); err != nil {
		t.Fatal(err)
	}
	got := loaded.entities[0].(*PlayerEntity).Inventory.Slots[armorSlotStart+3]
	if got != p.Inventory.Slots[armorSlotStart+3] {
		t.Fatalf("armor save mismatch: %+v", got)
	}
}

func TestServerArmorClickIsAuthoritative(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Inventory.Slots[5] = Item{ID: int32(itemIronChestplate), Count: 1}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketClickWindow{SlotID: 5, Button: 2}})
	if p.Inventory.Slots[5].ID != 0 || p.Inventory.Slots[armorSlotStart+1].ID != int32(itemIronChestplate) {
		t.Fatal("server did not equip armor")
	}
	seen := false
	for len(s.Clients[p.UUID].Send) > 0 {
		if v, ok := (<-s.Clients[p.UUID].Send).(*PacketVitals); ok && v.Armor == 6 {
			seen = true
		}
	}
	if !seen {
		t.Fatal("server did not sync armor points")
	}
	p.CursorItem = Item{ID: int32(blockDirt), Count: 1}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketClickWindow{SlotID: armorSlotStart + 1, Button: 0}})
	if p.Inventory.Slots[armorSlotStart+1].ID != int32(itemIronChestplate) || p.CursorItem.ID != int32(blockDirt) {
		t.Fatal("server allowed invalid armor")
	}
}

func TestArmorDropsOnDeath(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Inventory.Slots[armorSlotStart] = Item{ID: int32(itemDiamondHelmet), Count: 1, Damage: 4}
	p.Vitals.Health = 2
	s.hurtPlayer(p, 20, "Explosion")
	if !p.dead() || p.Inventory.Slots[armorSlotStart].ID != 0 {
		t.Fatal("death retained armor")
	}
	found := false
	for _, entity := range w.entities {
		if drop, ok := entity.(*ItemEntity); ok && drop.ItemStack.ID == int32(itemDiamondHelmet) && drop.ItemStack.Damage == 5 {
			found = true
		}
	}
	if !found {
		t.Fatal("worn armor was not dropped")
	}
}
