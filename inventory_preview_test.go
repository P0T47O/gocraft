//go:build windows

package main

import (
	"os"
	"testing"
)

// Optional real-render smoke test. Normal test runs require no graphics window.
func TestInventoryPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_INVENTORY_PREVIEW") != "1" {
		t.Skip("opt-in graphical preview")
	}
	r, closePreview := nativePreviewFixture(t)
	defer closePreview()
	saved := localInventory
	defer func() { localInventory = saved }()
	localInventory = Inventory{}
	localInventory.Slots[0] = Item{ID: int32(blockPlankOak), Count: 5}
	localInventory.Slots[1] = Item{ID: int32(blockLog), Count: 61}
	localInventory.Slots[2] = Item{ID: int32(itemStick), Count: 2}
	localInventory.Slots[3] = Item{ID: int32(itemWoodShovel), Count: 1, Damage: 40}
	localInventory.Slots[4] = Item{ID: int32(blockSand), Count: 7}
	localInventory.Slots[5] = Item{ID: int32(blockSandstone), Count: 26}
	state := &InputState{InventoryOpen: true, RecipeSelected: int32(itemWoodPickaxe)}
	if err := os.MkdirAll("work", 0755); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		name          string
		width, height int
		station       byte
	}{{"inventory-field", 1280, 720, 0}, {"inventory-workbench", 1871, 994, blockCraftingTable}, {"inventory-small", 800, 600, 0}} {
		state.CraftingStation = sample.station
		captureNativeUI(t, r, sample.name, sample.width, sample.height, state, nil)
	}
	oldMode := currentGameMode
	defer func() { currentGameMode = oldMode }()
	state.CurrentBlock = blockLog
	for i := 0; i < 9; i++ {
		state.Hotbar[i] = byte(localInventory.Slots[i].ID)
	}
	for _, sample := range []string{"hud-theme", "creative-theme"} {
		state.InventoryOpen = sample == "creative-theme"
		if state.InventoryOpen {
			currentGameMode = ModeCreative
		} else {
			currentGameMode = ModeSurvival
		}
		captureNativeUI(t, r, sample, 1280, 720, state, nil)
	}
}
