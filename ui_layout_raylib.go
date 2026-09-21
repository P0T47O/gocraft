package main

import rl "github.com/gen2brain/raylib-go/raylib"

// Legacy draw paths read their window dimensions here, never from shared layout.
func uiScale() float32 {
	return uiScaleFor(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
}
func inventoryScale() float32 {
	return inventoryScaleFor(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
}
func inventoryLayout() InventoryLayout {
	return inventoryLayoutFor(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
}
