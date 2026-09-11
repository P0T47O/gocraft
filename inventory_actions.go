package main

// Click is shared by authoritative server and local inventory interaction.
func (inv *Inventory) Click(index, button int, cursor *Item) {
	if index < 0 || index >= len(inv.Slots) || cursor == nil {
		return
	}
	slot := &inv.Slots[index]
	if button == 2 {
		if cursor.ID != 0 || slot.ID == 0 {
			return
		}
		start, end := 9, 36
		if index >= 9 {
			start, end = 0, 9
		}
		for pass := 0; pass < 2; pass++ {
			for i := start; i < end; i++ {
				dst := &inv.Slots[i]
				if (pass == 0 && dst.ID != slot.ID) || (pass == 1 && dst.ID != 0) {
					continue
				}
				n := min(slot.Count, int32(MaxStackSize)-dst.Count)
				if n <= 0 {
					continue
				}
				dst.ID = slot.ID
				dst.Count += n
				slot.Count -= n
				if slot.Count == 0 {
					*slot = Item{}
					return
				}
			}
		}
		return
	}
	switch button {
	case 0:
		if cursor.ID == 0 {
			*cursor = *slot
			*slot = Item{}
		} else if slot.ID == cursor.ID {
			n := min(cursor.Count, int32(MaxStackSize)-slot.Count)
			slot.Count += n
			cursor.Count -= n
		} else {
			*slot, *cursor = *cursor, *slot
		}
	case 1:
		if cursor.ID == 0 && slot.ID != 0 {
			n := (slot.Count + 1) / 2
			*cursor = Item{ID: slot.ID, Count: n}
			slot.Count -= n
		} else if cursor.ID != 0 && (slot.ID == 0 || slot.ID == cursor.ID) && slot.Count < MaxStackSize {
			slot.ID = cursor.ID
			slot.Count++
			cursor.Count--
		}
	}
	if slot.Count == 0 {
		*slot = Item{}
	}
	if cursor.Count == 0 {
		*cursor = Item{}
	}
}

func (inv *Inventory) CountItem(id int32) int32 {
	var total int32
	for _, s := range inv.Slots {
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
		if !next.ConsumeItems(r.Ingredients) || next.Add(r.Result.ID, r.Result.Count) != 0 {
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
