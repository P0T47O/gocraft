package main

const (
	armorSlotStart = 36
	armorSlotCount = 4
)

func registerArmorItems() {
	for _, material := range []struct {
		name, texture string
		base          byte
		points        [4]int
		durability    [4]int32
	}{
		{"Golden", "golden", itemGoldHelmet, [4]int{2, 5, 3, 1}, [4]int32{77, 112, 105, 91}},
		{"Iron", "iron", itemIronHelmet, [4]int{2, 6, 5, 2}, [4]int32{165, 240, 225, 195}},
		{"Diamond", "diamond", itemDiamondHelmet, [4]int{3, 8, 6, 3}, [4]int32{363, 528, 495, 429}},
	} {
		for part, label := range [...]string{"Helmet", "Chestplate", "Leggings", "Boots"} {
			icon := [...]string{"helmet", "chestplate", "leggings", "boots"}[part]
			id := material.base + byte(part)
			Items[id] = &ItemDef{ID: id, Name: material.name + " " + label, Icon: "textures/item/" + material.texture + "_" + icon + ".png", MaxStack: 1, MaxDurability: material.durability[part], ArmorPart: byte(part + 1), ArmorPoints: material.points[part]}
		}
	}
}

func armorFits(slot int, stack ItemStack) bool {
	if slot < armorSlotStart || slot >= armorSlotStart+armorSlotCount || stack.Count != 1 || stack.ID <= 0 || stack.ID > 255 {
		return false
	}
	return GetItem(byte(stack.ID)).ArmorPart == byte(slot-armorSlotStart+1)
}

func (inv *Inventory) ArmorPoints() int {
	points := 0
	for i := armorSlotStart; i < armorSlotStart+armorSlotCount; i++ {
		if armorFits(i, inv.Slots[i]) && validStack(inv.Slots[i]) {
			points += GetItem(byte(inv.Slots[i].ID)).ArmorPoints
		}
	}
	return min(points, 20)
}

func armorProtects(cause string) bool {
	switch cause {
	case "Starved", "Drowned", "Fell into the void", "Fell from a height", "Suffocated":
		return false
	}
	return true
}

// Armor reduces eligible damage by four percent per point. Each equipped
// piece takes durability wear on a protected hit, including the breaking hit.
func (inv *Inventory) absorbDamage(amount int, cause string) (int, bool) {
	points := inv.ArmorPoints()
	if points == 0 || !armorProtects(cause) {
		return amount, false
	}
	remaining := max(1, (amount*(20-points)+19)/20)
	worn := false
	for i := armorSlotStart; i < armorSlotStart+armorSlotCount; i++ {
		if armorFits(i, inv.Slots[i]) {
			inv.Slots[i].Wear(1)
			worn = true
		}
	}
	return remaining, worn
}
