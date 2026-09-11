package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Optional real-render smoke test. Normal test runs require no graphics window.
func TestInventoryPreview(t *testing.T) {
	if os.Getenv("GOCRAFT_INVENTORY_PREVIEW") != "1" {
		t.Skip("opt-in graphical preview")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetConfigFlags(rl.FlagWindowHidden)
	rl.InitWindow(1280, 720, "Inventory preview")
	defer rl.CloseWindow()
	initBlockRegistry()
	InitRecipes()
	a := loadRenderAssets()
	defer a.unload()
	saved := localInventory
	defer func() { localInventory = saved }()
	localInventory = Inventory{}
	localInventory.Slots[0] = Item{ID: int32(blockPlankOak), Count: 5}
	localInventory.Slots[1] = Item{ID: int32(blockLog), Count: 61}
	localInventory.Slots[2] = Item{ID: int32(itemStick), Count: 2}
	localInventory.Slots[3] = Item{ID: int32(itemWoodShovel), Count: 1}
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
		rl.SetWindowSize(sample.width, sample.height)
		state.CraftingStation = sample.station
		// Render to a texture for a deterministic capture without desktop interaction.
		target := rl.LoadRenderTexture(int32(sample.width), int32(sample.height))
		rl.BeginTextureMode(target)
		rl.ClearBackground(rl.NewColor(33, 53, 66, 255))
		a.drawSurvivalInventory(state)
		rl.EndTextureMode()
		img := rl.LoadImageFromTexture(target.Texture)
		rl.ImageFlipVertical(img)
		path, _ := filepath.Abs(filepath.Join("work", sample.name+".png"))
		ok := rl.ExportImage(*img, path)
		rl.UnloadImage(img)
		rl.UnloadRenderTexture(target)
		if !ok {
			t.Fatal("preview export failed")
		}
	}
}
