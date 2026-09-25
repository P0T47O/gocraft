package main

// Click is shared by authoritative server and local inventory interaction.
func (inv *Inventory) Click(index, button int, cursor *Item) {
	if index < 0 || index >= len(inv.Slots) || cursor == nil {
		return
	}
	slot := &inv.Slots[index]
	if index >= armorSlotStart {
		if button == 2 {
			if cursor.ID != 0 || slot.ID == 0 {
				return
			}
			for i := 0; i < armorSlotStart; i++ {
				MoveStack(&inv.Slots[i], slot, 1)
				if slot.ID == 0 {
					return
				}
			}
			return
		}
		if cursor.ID == 0 {
			MoveStack(cursor, slot, 1)
		} else if armorFits(index, *cursor) {
			*slot, *cursor = *cursor, *slot
		}
		return
	}
	if button == 2 {
		if cursor.ID != 0 || slot.ID == 0 {
			return
		}
		if slot.ID > 0 && slot.ID <= 255 {
			part := GetItem(byte(slot.ID)).ArmorPart
			if part != 0 {
				armor := armorSlotStart + int(part) - 1
				if inv.Slots[armor].ID == 0 {
					MoveStack(&inv.Slots[armor], slot, 1)
					return
				}
			}
		}
		start, end := 9, 36
		if index >= 9 {
			start, end = 0, 9
		}
		for pass := 0; pass < 2; pass++ {
			for i := start; i < end; i++ {
				dst := &inv.Slots[i]
				if (pass == 0 && dst.ID == 0) || (pass == 1 && dst.ID != 0) {
					continue
				}
				MoveStack(dst, slot, slot.Count)
				if slot.ID == 0 {
					return
				}
			}
		}
		return
	}
	switch button {
	case 0:
		if cursor.ID == 0 {
			MoveStack(cursor, slot, slot.Count)
		} else if slot.ID == 0 || CanStack(*slot, *cursor) {
			MoveStack(slot, cursor, cursor.Count)
		} else {
			*slot, *cursor = *cursor, *slot
		}
	case 1:
		if cursor.ID == 0 {
			MoveStack(cursor, slot, (slot.Count+1)/2)
		} else {
			MoveStack(slot, cursor, 1)
		}
	}
}

func (inv *Inventory) CountItem(id int32) int32 {
	var total int32
	for _, s := range inv.Slots[:armorSlotStart] {
		if s.ID == id {
			total += s.Count
		}
	}
	return total
}

// Each batch is atomic: full output fits or its ingredients remain untouched.
// Batch requests are bounded to keep network requests cheap.
func (inv *Inventory) Craft(r *Recipe, count int) int {
	if r == nil || count < 1 || count > 64 || r.Result.ID <= 0 || r.Result.Count <= 0 {
		return 0
	}
	completed := 0
	for ; completed < count; completed++ {
		next := *inv
		if !next.ConsumeItems(r.Ingredients) || next.AddStack(r.Result) != 0 {
			break
		}
		*inv = next
	}
	return completed
}

func craftCapacity(inv *Inventory, r *Recipe) int { copy := *inv; return copy.Craft(r, 64) }

// Keep one row per output. Select the ingredient variant the player can use.
func recipeBook(inv *Inventory, station byte, readyOnly bool) []*Recipe {
	rows := []*Recipe{}
	byOutput := map[int32]int{}
	score := func(r *Recipe) int {
		n := craftCapacity(inv, r) * 1000
		for _, ing := range r.Ingredients {
			n += int(min(inv.CountItem(ing.ID), ing.Count))
		}
		return n
	}
	for _, r := range RecipeRegistry {
		if i, ok := byOutput[r.Result.ID]; ok {
			if score(r) > score(rows[i]) {
				rows[i] = r
			}
		} else {
			byOutput[r.Result.ID] = len(rows)
			rows = append(rows, r)
		}
	}
	if readyOnly {
		filtered := []*Recipe{}
		for _, r := range rows {
			if (r.Station == 0 || r.Station == station) && craftCapacity(inv, r) > 0 {
				filtered = append(filtered, r)
			}
		}
		return filtered
	}
	return rows
}
