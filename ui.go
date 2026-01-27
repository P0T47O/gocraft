package main

import rl "github.com/gen2brain/raylib-go/raylib"

func uiScale() float32 {
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	scale := float32(1)
	if w > 0 && h > 0 {
		scale = w / 1280
		if h/720 < scale {
			scale = h / 720
		}
	}
	if scale < 1 {
		scale = 1
	}
	return scale
}

func inventoryScale() float32 {
	return uiScale() * 3.2
}

type InventoryLayout struct {
	OriginX  float32
	OriginY  float32
	SlotSize float32
	Stride   float32
	Cols     int
	Rows     int
	GridX    float32
	GridY    float32
	GridW    float32 // Width of the grid area
	GridH    float32 // Height of the grid area
	HotbarX  float32
	HotbarY  float32
}

func inventoryLayout() InventoryLayout {
	scale := inventoryScale()
	// Creative Layout: 9 columns, 6 rows (54 items)
	// Base size approx 9*18 = 162 + padding
	texW := float32(176) * scale
	texH := float32(196) * scale // Taller for more rows
	originX := float32(rl.GetScreenWidth())/2 - texW/2
	originY := float32(rl.GetScreenHeight())/2 - texH/2
	slot := float32(18) * scale
	stride := slot
	// Center the grid in the window
	gridW := float32(9) * stride
	// gridH := float32(6) * stride

	gridX := originX + (texW-gridW)/2
	gridY := originY + float32(18)*scale // Top padding

	// Hotbar is not shown in this creative view usually, or is at bottom.
	// We'll just define it but maybe not draw it or draw it at bottom.
	hotbarX := gridX
	hotbarY := originY + texH - float32(24)*scale

	return InventoryLayout{
		OriginX:  originX,
		OriginY:  originY,
		SlotSize: slot,
		Stride:   stride,
		Cols:     9,
		Rows:     6, // Increased from 3
		GridX:    gridX,
		GridY:    gridY,
		GridW:    gridW,
		GridH:    float32(6) * stride, // Explicit height
		HotbarX:  hotbarX,
		HotbarY:  hotbarY,
	}
}
