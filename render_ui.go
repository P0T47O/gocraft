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
	if a.hotbarTex.ID == 0 {
		return
	}
	scale := float32(3) * uiScale()
	hotbarWidth := float32(a.hotbarTex.Width) * scale
	hotbarHeight := float32(a.hotbarTex.Height) * scale
	hbX := float32(rl.GetScreenWidth())/2 - hotbarWidth/2
	hbY := float32(rl.GetScreenHeight()) - hotbarHeight - 10
	rl.DrawTextureEx(a.hotbarTex, rl.NewVector2(hbX, hbY), 0, scale, rl.White)

	slotWidth := float32(a.hotbarTex.Width) / 9
	slotSize := slotWidth * scale
	hotbarBlocks := state.Hotbar[:]
	selectedIndex := state.SelectedSlot
	if a.hotbarSel.ID != 0 {
		selW := float32(a.hotbarSel.Width) * scale
		selH := float32(a.hotbarSel.Height) * scale
		selX := hbX + float32(selectedIndex)*slotWidth*scale + (slotWidth*scale-selW)/2
		selY := hbY - (selH-hotbarHeight)/2
		rl.DrawTextureEx(a.hotbarSel, rl.NewVector2(selX, selY), 0, scale, rl.White)
	}

	for i, b := range hotbarBlocks {
		if currentGameMode == ModeSurvival {
			var item Item
			if client != nil {
				item = client.Inventory.Slots[i]
			} else {
				item = localInventory.Slots[i]
			}
			if item.ID != 0 {
				iconX := hbX + float32(i)*slotWidth*scale + (slotWidth*scale-slotSize)/2
				iconY := hbY + (hotbarHeight-slotSize)/2
				a.drawIcon(byte(item.ID), iconX, iconY, slotSize)
				if item.Count > 1 {
					rl.DrawText(fmt.Sprintf("%d", item.Count), int32(iconX+slotSize-30), int32(iconY+slotSize-25), 20, rl.White)
				}
			}
		} else {
			if b == blockAir {
				continue
			}
			iconX := hbX + float32(i)*slotWidth*scale + (slotWidth*scale-slotSize)/2
			iconY := hbY + (hotbarHeight-slotSize)/2
			a.drawIcon(b, iconX, iconY, slotSize)
		}
	}
}

func (a *RenderAssets) drawInventory(state *InputState) {
	if currentGameMode == ModeSurvival {
		a.drawSurvivalInventory(state)
		return
	}

	layout := inventoryLayout()
	scale := inventoryScale()

	// Draw semi-transparent background for the whole screen
	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.Fade(rl.Black, 0.5))

	// Draw Creative Inventory Window Background
	windowRect := rl.Rectangle{
		X:      layout.OriginX,
		Y:      layout.OriginY,
		Width:  layout.GridW + 16*scale, // Padding
		Height: layout.GridH + 60*scale, // Padding + label space
	}
	// Main background
	rl.DrawRectangleRec(windowRect, rl.NewColor(198, 198, 198, 255))
	// Borders
	rl.DrawRectangleLinesEx(windowRect, 2*scale, rl.NewColor(255, 255, 255, 255))                                                                                                    // Highlight
	rl.DrawRectangleLinesEx(rl.NewRectangle(windowRect.X-2*scale, windowRect.Y-2*scale, windowRect.Width+4*scale, windowRect.Height+4*scale), 2*scale, rl.NewColor(85, 85, 85, 255)) // Shadow

	rl.DrawText("Creative Inventory", int32(layout.GridX), int32(layout.OriginY+8*scale), int32(10*scale), rl.DarkGray)

	itemsPerPage := layout.Cols * layout.Rows
	start := state.InventoryPage * itemsPerPage
	end := start + itemsPerPage
	if end > len(allBlocks) {
		end = len(allBlocks)
	}
	drawFrames := a.inventoryTex.ID == 0
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

func (a *RenderAssets) drawSurvivalInventory(state *InputState) {
	scale := inventoryScale()
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())

	// Darken background
	rl.DrawRectangle(0, 0, int32(w), int32(h), rl.Fade(rl.Black, 0.5))

	// Layout Config
	slotSize := 36 * scale / 2 // Adjust scale as needed, existing scale might be 2x or 3x
	if slotSize < 32 {
		slotSize = 32
	}
	stride := slotSize + 4
	cols := 9
	rows := 3 // Main inventory

	invW := float32(cols)*stride + 20
	invH := float32(rows+1)*stride + 60 // +1 for hotbar, +padding

	startX := (w - invW) / 2
	startY := (h - invH) / 2

	// Window BG
	bgRect := rl.NewRectangle(startX, startY, invW, invH)
	rl.DrawRectangleRec(bgRect, rl.NewColor(198, 198, 198, 255))
	rl.DrawRectangleLinesEx(bgRect, 2, rl.White)
	rl.DrawRectangleLinesEx(rl.NewRectangle(startX-2, startY-2, invW+4, invH+4), 2, rl.NewColor(85, 85, 85, 255))

	rl.DrawText("Survival Inventory", int32(startX+10), int32(startY+10), 20, rl.DarkGray)

	// Draw Inventory Slots (9-35)
	for i := 9; i < 36; i++ {
		idx := i - 9
		r := idx / 9
		c := idx % 9
		x := startX + 10 + float32(c)*stride
		y := startY + 40 + float32(r)*stride

		a.drawSlot(x, y, slotSize)

		var item Item
		if client != nil {
			item = client.Inventory.Slots[i]
		} else {
			item = localInventory.Slots[i]
		}
		if item.ID != 0 {
			a.drawIcon(byte(item.ID), x, y, slotSize)
			if item.Count > 1 {
				rl.DrawText(fmt.Sprintf("%d", item.Count), int32(x+slotSize-30), int32(y+slotSize-25), 20, rl.White)
			}
		}
	}

	// Draw Hotbar Slots (0-8)
	hotbarY := startY + invH - stride - 10
	for i := 0; i < 9; i++ {
		x := startX + 10 + float32(i)*stride
		a.drawSlot(x, hotbarY, slotSize)

		var item Item
		if client != nil {
			item = client.Inventory.Slots[i]
		} else {
			item = localInventory.Slots[i]
		}
		if item.ID != 0 {
			a.drawIcon(byte(item.ID), x, hotbarY, slotSize)
			if item.Count > 1 {
				rl.DrawText(fmt.Sprintf("%d", item.Count), int32(x+slotSize-30), int32(hotbarY+slotSize-25), 20, rl.White)
			}
		}
	}

	if state.CursorItem.ID != 0 {
		a.drawDraggedIcon(byte(state.CursorItem.ID), slotSize)
		if state.CursorItem.Count > 1 {
			mouse := rl.GetMousePosition()
			rl.DrawText(fmt.Sprintf("%d", state.CursorItem.Count), int32(mouse.X), int32(mouse.Y), 20, rl.White)
		}
	}

	a.drawCraftingList(state, startX, startY, invW, invH)
}

func (a *RenderAssets) drawCraftingList(state *InputState, invX, invY, invW, invH float32) {
	// 1. Get Recipes
	nearby := []byte{}
	if state.CraftingStation != 0 {
		nearby = append(nearby, state.CraftingStation)
	}

	var inv *Inventory
	if client != nil {
		inv = &client.Inventory
	} else {
		inv = &localInventory
	}

	recipes := GetCraftableRecipes(inv, nearby)

	// 2. Layout
	scale := inventoryScale()

	// Card Layout Config
	cardHeight := 48 * scale
	cardPadding := 4 * scale

	// List width - User said 200 was too wide, reducing to 170
	listW := float32(170) * scale
	listH := invH

	// Default to Left side
	listX := invX - listW - 10

	// If it doesn't fit on the left, try the right
	if listX < 0 {
		listX = invX + invW + 10
	}

	listY := invY

	// Background
	bgRect := rl.NewRectangle(listX, listY, listW, listH)
	rl.DrawRectangleRec(bgRect, rl.NewColor(220, 220, 220, 255)) // Lighter BG
	rl.DrawRectangleLinesEx(bgRect, 2, rl.White)
	rl.DrawRectangleLinesEx(rl.NewRectangle(listX-2, listY-2, listW+4, listH+4), 2, rl.NewColor(85, 85, 85, 255))

	rl.DrawText("Crafting", int32(listX+10), int32(listY+10), 20, rl.DarkGray)

	// 3. Render Items
	y := listY + 40
	mouse := rl.GetMousePosition()

	// Icon sizes
	resultSize := 32 * scale
	ingSize := 24 * scale

	for _, r := range recipes {
		if y+cardHeight > listY+listH {
			break
		}

		// Card Background
		cardRect := rl.NewRectangle(listX+5, y, listW-10, cardHeight)

		// Interaction
		isHover := rl.CheckCollisionPointRec(mouse, cardRect)
		bgColor := rl.NewColor(180, 180, 180, 255)
		if isHover {
			bgColor = rl.NewColor(200, 200, 200, 255)

			// Click to Craft
			if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
				if client != nil {
					client.Send(&PacketCraft{RecipeID: int32(r.ID)})
				}
			}

			// Tooltip
			resultDef := GetBlock(byte(r.Result.ID))
			rl.DrawText(resultDef.Name, int32(mouse.X+15), int32(mouse.Y), 20, rl.Black)
		}
		rl.DrawRectangleRec(cardRect, bgColor)
		// Card Border
		rl.DrawRectangleLinesEx(cardRect, 1, rl.Gray)

		// Layout within Card
		currX := listX + 10

		// 1. Result Icon (Left)
		a.drawIcon(byte(r.Result.ID), currX, y+(cardHeight-resultSize)/2, resultSize)
		if r.Result.Count > 1 {
			rl.DrawText(fmt.Sprintf("%d", r.Result.Count), int32(currX+resultSize-15), int32(y+cardHeight-20), 20, rl.White)
		}
		currX += resultSize + 5

		// 2. Arrow (Middle)
		rl.DrawText("->", int32(currX), int32(y+cardHeight/2-10), 20, rl.DarkGray)
		currX += 25

		// 3. Ingredients (Right)
		for _, ing := range r.Ingredients {
			if currX+ingSize > listX+listW-5 {
				break
			}
			a.drawIcon(byte(ing.ID), currX, y+(cardHeight-ingSize)/2, ingSize)

			// Count (Larger text: 20px)
			if ing.Count > 1 {
				rl.DrawText(fmt.Sprintf("%d", ing.Count), int32(currX+ingSize-12), int32(y+cardHeight-18), 20, rl.White)
			}

			currX += ingSize + 2
		}

		y += cardHeight + cardPadding
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
		if col == state.SelectedSlot && a.slotSelect.ID != 0 {
			a.drawSlotOverlay(a.slotSelect, x, y, layout.SlotSize)
		}
	}
}

func (a *RenderAssets) drawSlot(x, y, size float32) {
	if a.slotTex.ID == 0 {
		rl.DrawRectangle(int32(x), int32(y), int32(size), int32(size), rl.NewColor(60, 60, 60, 220))
		return
	}
	a.drawSlotOverlay(a.slotTex, x, y, size)
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
