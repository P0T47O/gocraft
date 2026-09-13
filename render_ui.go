package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) drawCrosshair() {
	scale := uiScale()
	cx := int32(rl.GetScreenWidth() / 2)
	cy := int32(rl.GetScreenHeight() / 2)

	// Settings for high visibility
	length := int32(8 * scale)
	thickness := int32(2 * scale)
	if thickness < 2 {
		thickness = 2
	}
	// Make sure thickness is even for perfect centering
	if thickness%2 != 0 {
		thickness++
	}

	// 1. Draw Black Outline (Outer Border)
	border := int32(2)
	// Horizontal Outline
	rl.DrawRectangle(cx-length-border, cy-thickness/2-border, length*2+border*2, thickness+border*2, rl.Black)
	// Vertical Outline
	rl.DrawRectangle(cx-thickness/2-border, cy-length-border, thickness+border*2, length*2+border*2, rl.Black)

	// 2. Draw White Inner Cross
	// Horizontal
	rl.DrawRectangle(cx-length, cy-thickness/2, length*2, thickness, rl.White)
	// Vertical
	rl.DrawRectangle(cx-thickness/2, cy-length, thickness, length*2, rl.White)
}

func (a *RenderAssets) drawHotbar(state *InputState) {
	scale := min(float32(1.5), float32(rl.GetScreenWidth())/1280, float32(rl.GetScreenHeight())/720)
	slot, stride := float32(48)*scale, float32(54)*scale
	width := 9*stride + 10*scale
	x := (float32(rl.GetScreenWidth()) - width) / 2
	y := float32(rl.GetScreenHeight()) - 68*scale
	inventoryBox(rl.NewRectangle(x, y, width, 60*scale), invBackground, invLine)
	for i := 0; i < 9; i++ {
		r := rl.NewRectangle(x+8*scale+float32(i)*stride, y+6*scale, slot, slot)
		inventoryBox(r, invSlot, invLine)
		if i == state.SelectedSlot {
			rl.DrawRectangleLinesEx(r, 2*scale, invAccent)
		}
		item := inventoryFor(client).Slots[i]
		if currentGameMode == ModeCreative {
			item = Item{ID: int32(state.Hotbar[i]), Count: 1}
		}
		a.drawInventoryItem(item, r, scale)
		inventoryText(fmt.Sprint(i+1), r.X+3*scale, r.Y+3*scale, int32(10*scale), invMuted)
	}
	if state.CurrentBlock != blockAir {
		label := GetItemVisual(state.CurrentBlock).Name
		fs := int32(15 * scale)
		tw := float32(rl.MeasureText(label, fs))
		inventoryBox(rl.NewRectangle(float32(rl.GetScreenWidth())/2-tw/2-10*scale, y-30*scale, tw+20*scale, 23*scale), invBackground, invLine)
		inventoryText(label, float32(rl.GetScreenWidth())/2-tw/2, y-26*scale, fs, invText)
	}
}

func (a *RenderAssets) drawInventory(state *InputState) {
	if currentGameMode == ModeSurvival {
		a.drawSurvivalInventory(state)
		return
	}

	layout := inventoryLayout()
	scale := inventoryScale()

	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(9, 17, 22, 190))
	windowRect := rl.NewRectangle(layout.OriginX, layout.OriginY, 176*scale, 196*scale)
	inventoryBox(windowRect, invBackground, invLine)
	rl.DrawRectangleRec(rl.NewRectangle(windowRect.X, windowRect.Y, windowRect.Width, 2*scale), invAccent)
	rl.DrawText("CREATIVE INVENTORY", int32(layout.GridX), int32(layout.OriginY+6*scale), int32(6*scale), invText)
	rl.DrawText("HOTBAR", int32(layout.GridX), int32(layout.HotbarY-10*scale), int32(5*scale), invMuted)
	rl.DrawText(fmt.Sprintf("Page %d  /  scroll to browse", state.InventoryPage+1), int32(layout.GridX), int32(layout.GridY+layout.GridH+7*scale), int32(5*scale), invMuted)

	itemsPerPage := layout.Cols * layout.Rows
	start := state.InventoryPage * itemsPerPage
	end := start + itemsPerPage
	if end > len(allBlocks) {
		end = len(allBlocks)
	}
	drawFrames := true
	a.drawSlotGrid(layout, allBlocks[start:end], drawFrames)
	a.drawHotbarSlots(layout, state, drawFrames)
	if state.CursorItem.ID != 0 {
		a.drawDraggedIcon(byte(state.CursorItem.ID), layout.SlotSize)
		if state.CursorItem.Count > 1 {
			mouse := rl.GetMousePosition()
			rl.DrawText(fmt.Sprintf("%d", state.CursorItem.Count), int32(mouse.X), int32(mouse.Y), 20, rl.White)
		}
	}
}

func (a *RenderAssets) drawSlotGrid(layout InventoryLayout, items []byte, drawFrames bool) {
	for row := 0; row < layout.Rows; row++ {
		for col := 0; col < layout.Cols; col++ {
			index := row*layout.Cols + col
			if index >= len(items) {
				continue
			}
			x := layout.GridX + float32(col)*layout.Stride
			y := layout.GridY + float32(row)*layout.Stride
			if drawFrames {
				a.drawSlot(x, y, layout.SlotSize)
			}
			a.drawIcon(items[index], x, y, layout.SlotSize)
		}
	}
}

func (a *RenderAssets) drawHotbarSlots(layout InventoryLayout, state *InputState, drawFrames bool) {
	for col := 0; col < layout.Cols; col++ {
		x := layout.HotbarX + float32(col)*layout.Stride
		y := layout.HotbarY
		if drawFrames {
			a.drawSlot(x, y, layout.SlotSize)
		}
		block := state.Hotbar[col]
		if block != blockAir {
			a.drawIcon(block, x, y, layout.SlotSize)
		}
		if col == state.SelectedSlot {
			rl.DrawRectangleLinesEx(rl.NewRectangle(x, y, layout.SlotSize, layout.SlotSize), 2, invAccent)
		}
	}
}

func (a *RenderAssets) drawSlot(x, y, size float32) {
	r := rl.NewRectangle(x+1, y+1, size-2, size-2)
	fill := invSlot
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), r) {
		fill = rl.NewColor(76, 89, 76, 255)
	}
	inventoryBox(r, fill, invLine)
}

func (a *RenderAssets) drawSlotOverlay(tex rl.Texture2D, x, y, size float32) {
	scale := size / float32(tex.Width)
	rl.DrawTextureEx(tex, rl.NewVector2(x, y), 0, scale, rl.White)
}

func (a *RenderAssets) drawIcon(block byte, x, y, size float32) {
	rt := a.iconRenders[block]
	if rt.ID == 0 {
		return
	}
	offset := a.iconOffsets[block]
	iconScale := (size * 0.9) / float32(rt.Texture.Width)
	iconW := float32(rt.Texture.Width) * iconScale
	iconH := float32(rt.Texture.Height) * iconScale
	iconX := x + (size-iconW)/2 - offset.X*iconScale
	iconY := y + (size-iconH)/2 - offset.Y*iconScale
	src := rl.Rectangle{
		X:      0,
		Y:      0,
		Width:  float32(rt.Texture.Width),
		Height: -float32(rt.Texture.Height),
	}
	dst := rl.Rectangle{
		X:      iconX,
		Y:      iconY,
		Width:  iconW,
		Height: iconH,
	}
	rl.DrawTexturePro(rt.Texture, src, dst, rl.NewVector2(0, 0), 0, rl.White)
}

func (a *RenderAssets) drawDraggedIcon(block byte, size float32) {
	rt := a.iconRenders[block]
	if rt.ID == 0 {
		return
	}
	offset := a.iconOffsets[block]
	mouse := rl.GetMousePosition()
	iconScale := (size * 0.9) / float32(rt.Texture.Width)
	iconW := float32(rt.Texture.Width) * iconScale
	iconH := float32(rt.Texture.Height) * iconScale
	iconX := mouse.X - iconW/2 - offset.X*iconScale
	iconY := mouse.Y - iconH/2 - offset.Y*iconScale
	src := rl.Rectangle{
		X:      0,
		Y:      0,
		Width:  float32(rt.Texture.Width),
		Height: -float32(rt.Texture.Height),
	}
	dst := rl.Rectangle{
		X:      iconX,
		Y:      iconY,
		Width:  iconW,
		Height: iconH,
	}
	rl.DrawTexturePro(rt.Texture, src, dst, rl.NewVector2(0, 0), 0, rl.White)
}
