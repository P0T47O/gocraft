package main

// MaxStackSize defines the maximum number of items in a single stack
const MaxStackSize = 64

type Item struct {
	ID    int32
	Count int32
}

type Inventory struct {
	Slots [36]Item // 0-8: Hotbar, 9-35: Inventory
}

func (inv *Inventory) Add(id int32, count int32) int32 {
	// 1. Try to stack
	for i := 0; i < 36; i++ {
		if inv.Slots[i].ID == id && inv.Slots[i].Count < MaxStackSize {
			space := MaxStackSize - inv.Slots[i].Count
			if count <= space {
				inv.Slots[i].Count += count
				return 0
			}
			inv.Slots[i].Count += space
			count -= space
		}
	}
	// 2. Try empty slots
	for i := 0; i < 36; i++ {
		if inv.Slots[i].ID == 0 {
			inv.Slots[i].ID = id
			inv.Slots[i].Count = count
			return 0
		}
	}
	return count // Return remaining
}

// Consume attempts to remove 'count' of 'id' from the inventory.
// Returns true if successful (enough items found), false otherwise.
// Prioritizes removing from the hotbar (0-8) first, then main inventory.
func (inv *Inventory) Consume(id int32, count int32) bool {
	// First check if we have enough
	total := int32(0)
	for i := 0; i < 36; i++ {
		if inv.Slots[i].ID == id {
			total += inv.Slots[i].Count
		}
	}
	if total < count {
		return false
	}

	remaining := count

	// Pass 1: Hotbar
	for i := 0; i < 9; i++ {
		if remaining <= 0 {
			break
		}
		if inv.Slots[i].ID == id {
			if inv.Slots[i].Count > remaining {
				inv.Slots[i].Count -= remaining
				remaining = 0
			} else {
				remaining -= inv.Slots[i].Count
				inv.Slots[i] = Item{} // Clear slot
			}
		}
	}

	// Pass 2: Main Inventory
	for i := 9; i < 36; i++ {
		if remaining <= 0 {
			break
		}
		if inv.Slots[i].ID == id {
			if inv.Slots[i].Count > remaining {
				inv.Slots[i].Count -= remaining
				remaining = 0
			} else {
				remaining -= inv.Slots[i].Count
				inv.Slots[i] = Item{} // Clear slot
			}
		}
	}

	return true
}

// HasItems checks if the inventory contains the required items.
// It handles duplicate ingredients correctly (e.g. 2 stacks of 64 logs needed).
func (inv *Inventory) HasItems(items []Item) bool {
	// Create a map of required counts
	required := make(map[int32]int32)
	for _, item := range items {
		required[item.ID] += item.Count
	}

	// Iterate inventory to count available
	// We operate on a copy or just count
	available := make(map[int32]int32)
	for i := 0; i < 36; i++ {
		slot := inv.Slots[i]
		if slot.ID != 0 {
			available[slot.ID] += slot.Count
		}
	}

	// Check sufficiency
	for id, count := range required {
		if available[id] < count {
			return false
		}
	}
	return true
}

// ConsumeItems consumes a list of items from the inventory.
// Returns true if successful (all items consumed), false if not enough items (state unchanged not guaranteed if check skipped).
// It relies on HasItems being called first or implicitly checks.
// To be safe, we check HasItems first.
func (inv *Inventory) ConsumeItems(items []Item) bool {
	if !inv.HasItems(items) {
		return false
	}
	for _, item := range items {
		inv.Consume(item.ID, item.Count)
	}
	return true
}
