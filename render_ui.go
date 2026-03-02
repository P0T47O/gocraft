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

	// Calculate approximate crafting list width to center both
	maxIngCount := float32(4)
	iconSize := slotSize
	arrowWidth := 16 * scale
	listW := float32(10) + maxIngCount*(iconSize+4*scale) + arrowWidth + float32(10) + iconSize + float32(10)

	totalW := invW + 10 + listW

	startX := (w - totalW) / 2
	if startX < 10 {
		startX = 10
	}
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
	screenW := float32(rl.GetScreenWidth())

	// Tighter Card Layout
	slotSize := 36 * scale / 2
	if slotSize < 32 {
		slotSize = 32
	}

	cardHeight := slotSize + 4*scale // Narrower border
	cardPadding := 2 * scale         // Tighter spacing

	// Dynamic list width based on max ingredients (assuming max 4 for now)
	maxIngCount := float32(4)
	iconSize := slotSize // Same size as inventory
	arrowWidth := 16 * scale
	scrollBarW := 8 * scale

	// N * (ingredient icon + padding) + arrow + result icon + scrollbar + card padding
	listW := float32(10) + maxIngCount*(iconSize+2*scale) + arrowWidth + float32(8) + iconSize + scrollBarW + float32(10)
	listH := invH

	// Smart Positioning
	// We centered the total width in drawSurvivalInventory, so just append to the right
	listX := invX + invW + 10
	if listX+listW > screenW-10 {
		listX = screenW - listW - 10
	}

	listY := invY

	// Panel Background
	bgRect := rl.NewRectangle(listX, listY, listW, listH)
	rl.DrawRectangleRec(bgRect, rl.NewColor(210, 210, 210, 240))
	rl.DrawRectangleLinesEx(bgRect, 2, rl.White)
	rl.DrawRectangleLinesEx(rl.NewRectangle(listX-2, listY-2, listW+4, listH+4), 2, rl.NewColor(50, 50, 50, 255))

	rl.DrawText("Crafting", int32(listX+10), int32(listY+10), 20, rl.DarkGray)

	mouse := rl.GetMousePosition()

	yStart := listY + 36
	visibleH := listH - 36
	visibleCount := int(visibleH / (cardHeight + cardPadding))

	maxScroll := len(recipes) - visibleCount
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Handle Scrolling
	if rl.CheckCollisionPointRec(mouse, bgRect) {
		wheel := rl.GetMouseWheelMove()
		if wheel > 0 {
			state.CraftingScroll--
		} else if wheel < 0 {
			state.CraftingScroll++
		}

		// Clamp scrolling
		if state.CraftingScroll > maxScroll {
			state.CraftingScroll = maxScroll
		}
		if state.CraftingScroll < 0 {
			state.CraftingScroll = 0
		}
	}

	// 3. Render Items
	y := yStart

	for i, r := range recipes {
		if i < state.CraftingScroll {
			continue // Skip items scrolled out of view at the top
		}
		if y+cardHeight > listY+listH {
			break // Skip items that overflow the bottom
		}

		cardRect := rl.NewRectangle(listX+5, y, listW-10-scrollBarW, cardHeight)
		isHover := rl.CheckCollisionPointRec(mouse, cardRect)

		bgColor := rl.NewColor(160, 160, 160, 255)
		if isHover {
			bgColor = rl.NewColor(190, 190, 190, 255)
			if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
				if client != nil {
					client.Send(&PacketCraft{RecipeID: int32(r.ID)})
				}
			}
			// Lightweight tooltip
			def := GetBlock(byte(r.Result.ID))
			rl.DrawText(def.Name, int32(mouse.X+10), int32(mouse.Y-20), 20, rl.Black)
		}

		rl.DrawRectangleRec(cardRect, bgColor)
		rl.DrawRectangleLinesEx(cardRect, 1, rl.NewColor(100, 100, 100, 255))

		// Draw Ingredients (Left)
		currX := listX + 8
		for _, ing := range r.Ingredients {
			if currX+iconSize > listX+listW-scrollBarW-5 {
				break
			}
			iy := y + (cardHeight-iconSize)/2

			a.drawSlot(currX, iy, iconSize)
			a.drawIcon(byte(ing.ID), currX, iy, iconSize)
			if ing.Count > 1 {
				txt := fmt.Sprintf("%d", ing.Count)
				// Use 18 for font size, adjust text position relative to right and bottom edges to avoid clipping
				textW := rl.MeasureText(txt, 18)
				rl.DrawText(txt, int32(currX+iconSize-float32(textW)-6), int32(iy+iconSize-18), 18, rl.White)
			}
			currX += iconSize + 2*scale // Tighter spacing
		}

		// Draw Graphical Arrow
		// We center the arrow dynamically based on ingredients used so it adjusts properly
		arrowX := currX + 2*scale
		arrowY := y + cardHeight/2
		a.drawUIArrow(arrowX, arrowY, 12*scale)

		// Draw Result (Right)
		resX := arrowX + arrowWidth + 6*scale
		resY := y + (cardHeight-iconSize)/2

		// Draw a slot-like background for the result
		a.drawSlot(resX, resY, iconSize)
		a.drawIcon(byte(r.Result.ID), resX, resY, iconSize)
		if r.Result.Count > 1 {
			txt := fmt.Sprintf("%d", r.Result.Count)
			textW := rl.MeasureText(txt, 18)
			rl.DrawText(txt, int32(resX+iconSize-float32(textW)-6), int32(resY+iconSize-18), 18, rl.White)
		}

		y += cardHeight + cardPadding
	}

	// Draw Scrollbar (if needed)
	if maxScroll > 0 {
		trackW := scrollBarW - 2*scale
		if trackW < 4 {
			trackW = 4
		}
		trackX := listX + listW - trackW - 4*scale
		trackY := yStart
		trackH := visibleH - 4*scale

		// Scroll track
		rl.DrawRectangle(int32(trackX), int32(trackY), int32(trackW), int32(trackH), rl.NewColor(120, 120, 120, 150))

		// Scroll thumb
		thumbH := trackH * float32(visibleCount) / float32(len(recipes))
		if thumbH < 15*scale {
			thumbH = 15 * scale
		}

		scrollFraction := float32(state.CraftingScroll) / float32(maxScroll)
		thumbY := trackY + scrollFraction*(trackH-thumbH)

		rl.DrawRectangle(int32(trackX+1), int32(thumbY+1), int32(trackW-2), int32(thumbH-2), rl.NewColor(220, 220, 220, 255))
	}
}

func (a *RenderAssets) drawUIArrow(x, y, length float32) {
	thickness := length / 3
	rl.DrawRectangleV(rl.NewVector2(x, y-thickness/2), rl.NewVector2(length*0.6, thickness), rl.DarkGray)

	p1 := rl.NewVector2(x+length*0.5, y-thickness)
	p2 := rl.NewVector2(x+length*0.5, y+thickness)
	p3 := rl.NewVector2(x+length, y)
	rl.DrawTriangle(p1, p2, p3, rl.DarkGray)
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
