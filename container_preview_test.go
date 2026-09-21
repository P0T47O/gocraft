//go:build windows

package main

import (
	"os"
	"testing"
)

func TestContainerPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_CONTAINER_PREVIEW") != "1" {
		t.Skip("opt-in rendering")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	saved := localInventory
	defer func() { localInventory = saved }()
	localInventory = Inventory{}
	localInventory.Slots[0] = Item{ID: int32(itemIronPickaxe), Count: 1, Damage: 80}
	localInventory.Slots[1] = Item{ID: int32(blockChest), Count: 2}
	localInventory.Slots[2] = Item{ID: int32(blockFurnace), Count: 1}
	os.MkdirAll("work", 0755)
	for _, sample := range []struct {
		name string
		kind byte
		w, h int32
	}{{"chest", blockChest, 1280, 720}, {"furnace", blockFurnace, 1280, 720}, {"chest-small", blockChest, 800, 600}} {
		c := BlockContainer{Kind: sample.kind, Slots: make([]ItemStack, containerSize(sample.kind))}
		if sample.kind == blockFurnace {
			c.Slots[0] = Item{ID: int32(blockIronOre), Count: 12}
			c.Slots[1] = Item{ID: int32(itemCoal), Count: 4}
			c.Slots[2] = Item{ID: int32(itemIronIngot), Count: 3}
			c.Cook = 125
			c.Burn = 800
			c.BurnTotal = 1600
		} else {
			c.Slots[0] = Item{ID: int32(blockCobblestone), Count: 64}
			c.Slots[1] = Item{ID: int32(blockLog), Count: 32}
			c.Slots[4] = Item{ID: int32(itemWoodPickaxe), Count: 1, Damage: 40}
		}
		state := &InputState{InventoryOpen: true, Container: &PacketContainerState{Token: 1, State: c}}
		captureNativeUI(t, r, sample.name, int(sample.w), int(sample.h), state, nil)
	}
}
