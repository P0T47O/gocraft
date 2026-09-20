//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

var (
	webGPUUIPanel   = [4]float32{0.055, 0.075, 0.075, 0.97}
	webGPUUISlot    = [4]float32{0.105, 0.13, 0.115, 0.98}
	webGPUUILine    = [4]float32{0.29, 0.35, 0.31, 1}
	webGPUUIAccent  = [4]float32{0.58, 0.78, 0.38, 1}
	webGPUUIText    = [4]float32{0.94, 0.96, 0.94, 1}
	webGPUUIMuted   = [4]float32{0.62, 0.68, 0.64, 1}
	webGPUUIWarning = [4]float32{0.92, 0.42, 0.35, 1}
	webGPUUIDim     = [4]float32{0.02, 0.035, 0.045, 0.73}
)

func (h *webGPUHUDRenderer) drawPrepared(pass *wgpu.RenderPassEncoder, b *webGPUHUDBuilder) error {
	if h == nil || pass == nil || b == nil || len(b.vertices) == 0 {
		return nil
	}
	if len(b.vertices) > webGPUHUDMaxVertices {
		return fmt.Errorf("WebGPU HUD vertex budget exceeded: %d > %d", len(b.vertices), webGPUHUDMaxVertices)
	}
	byteLen := len(b.vertices) * int(unsafe.Sizeof(webGPUHUDVertex{}))
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&b.vertices[0])), byteLen)
	buffer, err := h.uploads.write(h.device, h.queue, bytes, gputypes.BufferUsageVertex)
	if err != nil {
		return fmt.Errorf("upload WebGPU UI vertices: %w", err)
	}
	pass.SetPipeline(h.pipeline)
	pass.SetBindGroup(0, h.bindGroup, nil)
	pass.SetVertexBuffer(0, buffer, 0)
	pass.Draw(gputypes.DrawArgs{VertexCount: uint32(len(b.vertices)), InstanceCount: 1})
	return nil
}

func (r *webGPUTextRenderer) drawPrepared(pass *wgpu.RenderPassEncoder, b *webGPUTextBatch) error {
	if r == nil || pass == nil || b == nil || len(b.vertices) == 0 {
		return nil
	}
	if len(b.vertices) > webGPUTextMaxVertices {
		return fmt.Errorf("WebGPU text vertex budget exceeded: %d > %d", len(b.vertices), webGPUTextMaxVertices)
	}
	byteLen := len(b.vertices) * int(unsafe.Sizeof(webGPUTextVertex{}))
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&b.vertices[0])), byteLen)
	buffer, err := r.uploads.write(r.device, r.queue, bytes, gputypes.BufferUsageVertex)
	if err != nil {
		return fmt.Errorf("upload WebGPU inventory text: %w", err)
	}
	pass.SetPipeline(r.pipeline)
	pass.SetBindGroup(0, r.bindGroup, nil)
	pass.SetVertexBuffer(0, buffer, 0)
	pass.Draw(gputypes.DrawArgs{VertexCount: uint32(len(b.vertices)), InstanceCount: 1})
	return nil
}

func webGPUItemUV(id byte) (float32, float32, float32, float32, bool) {
	if id == 0 || assets == nil || assets.atlas == nil {
		return 0, 0, 0, 0, false
	}
	def := GetItem(id)
	path := def.Icon
	if def.PlaceBlock != 0 {
		block := GetBlock(def.PlaceBlock)
		if block != nil {
			path = block.Textures.Top
		}
	}
	if path == "" {
		return 0, 0, 0, 0, false
	}
	uv, ok := assets.atlas.UVs[path]
	if !ok {
		return 0, 0, 0, 0, false
	}
	return uv.X, uv.Y, uv.X + uv.Width, uv.Y + uv.Height, true
}

func (b *webGPUHUDBuilder) addItemIcon(id byte, x, y, size float32) {
	u0, v0, u1, v1, ok := webGPUItemUV(id)
	if !ok {
		return
	}
	pad := size * 0.10
	b.texturedRect(x+pad, y+pad, size-pad*2, size-pad*2, u0, v0, u1, v1, [4]float32{1, 1, 1, 1})
}

func (b *webGPUHUDBuilder) addItemStack(stack ItemStack, x, y, size float32) {
	if stack.ID <= 0 || stack.ID > 255 {
		return
	}
	b.addItemIcon(byte(stack.ID), x, y, size)
	def := GetItem(byte(stack.ID))
	if def.MaxDurability > 0 && stack.Damage > 0 {
		fraction := float32(def.MaxDurability-stack.Damage) / float32(def.MaxDurability)
		fraction = max(float32(0), min(float32(1), fraction))
		barX, barY := x+size*0.12, y+size*0.84
		barW, barH := size*0.76, max(float32(2), size*0.055)
		b.rect(barX, barY, barW, barH, [4]float32{0.02, 0.02, 0.02, 0.95})
		color := webGPUUIAccent
		if fraction < 0.25 {
			color = webGPUUIWarning
		} else if fraction < 0.5 {
			color = [4]float32{0.90, 0.72, 0.24, 1}
		}
		b.rect(barX, barY, barW*fraction, barH, color)
	}
}

func webGPUAddStackText(t *webGPUTextBatch, stack ItemStack, x, y, size, scale float32) {
	if stack.ID <= 0 || stack.Count <= 1 {
		return
	}
	label := fmt.Sprint(stack.Count)
	fontSize := max(float32(10)*scale, 8)
	t.shadowText(x+size-webGPUTextWidth(fontSize, label)-3*scale, y+size-fontSize-2*scale, fontSize, label, webGPUUIText)
}

func drawWebGPUInventoryOverlay(pass *wgpu.RenderPassEncoder, worldRenderer *webGPUWorldRenderer, state *InputState) error {
	if pass == nil || worldRenderer == nil || state == nil {
		return nil
	}
	hud, err := ensureWebGPUHUDRenderer(worldRenderer)
	if err != nil {
		return err
	}
	text, err := ensureWebGPUTextRenderer(worldRenderer)
	if err != nil {
		return err
	}

	// The ordinary HUD already knows how to draw block items. Overlay standalone
	// item sprites so tools are visible in the hotbar before the full inventory is open.
	if !state.InventoryOpen {
		shapes := newWebGPUHUDBuilder(worldRenderer.width, worldRenderer.height)
		scale := webGPUHUDScale(worldRenderer.width, worldRenderer.height)
		slot, stride := float32(48)*scale, float32(54)*scale
		barWidth := 9*stride + 10*scale
		barX := (float32(worldRenderer.width) - barWidth) / 2
		barY := float32(worldRenderer.height) - 68*scale
		for i := 0; i < 9; i++ {
			id, _ := webGPUHotbarItem(state, i)
			if id == 0 || GetItem(id).PlaceBlock != 0 || GetItem(id).Icon == "" {
				continue
			}
			rx := barX + 8*scale + float32(i)*stride
			ry := barY + 6*scale
			shapes.addItemIcon(id, rx, ry, slot)
		}
		return hud.drawPrepared(pass, shapes)
	}

	// Container screens get their own layout in the next migration step; avoid
	// drawing the backpack underneath while the authoritative container is open.
	if state.Container != nil {
		return nil
	}
	if currentGameMode == ModeCreative {
		return drawWebGPUCreativeInventory(pass, hud, text, state, worldRenderer.width, worldRenderer.height)
	}
	return drawWebGPUSurvivalInventory(pass, hud, text, state, worldRenderer.width, worldRenderer.height)
}

func drawWebGPUCreativeInventory(pass *wgpu.RenderPassEncoder, hud *webGPUHUDRenderer, text *webGPUTextRenderer, state *InputState, width, height uint32) error {
	layout := inventoryLayout()
	scale := inventoryScale()
	shapes := newWebGPUHUDBuilder(width, height)
	labels := newWebGPUTextBatch(width, height)
	shapes.rect(0, 0, float32(width), float32(height), webGPUUIDim)
	winW, winH := float32(176)*scale, float32(196)*scale
	shapes.rect(layout.OriginX, layout.OriginY, winW, winH, webGPUUIPanel)
	shapes.border(layout.OriginX, layout.OriginY, winW, winH, max(scale, 1), webGPUUILine)
	shapes.rect(layout.OriginX, layout.OriginY, winW, 3*scale, webGPUUIAccent)

	itemsPerPage := layout.Cols * layout.Rows
	start := state.InventoryPage * itemsPerPage
	for row := 0; row < layout.Rows; row++ {
		for col := 0; col < layout.Cols; col++ {
			index := start + row*layout.Cols + col
			x := layout.GridX + float32(col)*layout.Stride
			y := layout.GridY + float32(row)*layout.Stride
			shapes.rect(x, y, layout.SlotSize, layout.SlotSize, webGPUUISlot)
			shapes.border(x, y, layout.SlotSize, layout.SlotSize, max(scale*0.35, 1), webGPUUILine)
			if index < len(allBlocks) {
				shapes.addItemIcon(allBlocks[index], x, y, layout.SlotSize)
			}
		}
	}
	inv := inventoryFor(client)
	for col := 0; col < layout.Cols; col++ {
		x := layout.HotbarX + float32(col)*layout.Stride
		y := layout.HotbarY
		shapes.rect(x, y, layout.SlotSize, layout.SlotSize, webGPUUISlot)
		shapes.border(x, y, layout.SlotSize, layout.SlotSize, max(scale*0.35, 1), webGPUUILine)
		if col == state.SelectedSlot {
			shapes.border(x-1.5*scale, y-1.5*scale, layout.SlotSize+3*scale, layout.SlotSize+3*scale, max(scale*0.65, 1), webGPUUIAccent)
		}
		stack := inv.Slots[col]
		shapes.addItemStack(stack, x, y, layout.SlotSize)
		webGPUAddStackText(labels, stack, x, y, layout.SlotSize, scale)
	}
	labels.shadowText(layout.OriginX+7*scale, layout.OriginY+4*scale, max(float32(8)*scale, 9), "CREATIVE INVENTORY", webGPUUIText)
	totalPages := max(1, (len(allBlocks)+itemsPerPage-1)/itemsPerPage)
	labels.shadowText(layout.OriginX+105*scale, layout.OriginY+4*scale, max(float32(7)*scale, 8), fmt.Sprintf("PAGE %d/%d", state.InventoryPage+1, totalPages), webGPUUIMuted)
	labels.shadowText(layout.OriginX+7*scale, layout.OriginY+177*scale, max(float32(7)*scale, 8), "E / ESC CLOSE   WHEEL PAGE", webGPUUIMuted)
	if state.CursorItem.ID > 0 && state.CursorItem.ID <= 255 {
		mouse := webGPUMousePosition()
		size := float32(18) * scale
		shapes.addItemStack(state.CursorItem, mouse.X-size/2, mouse.Y-size/2, size)
		webGPUAddStackText(labels, state.CursorItem, mouse.X-size/2, mouse.Y-size/2, size, scale)
	}
	if err := hud.drawPrepared(pass, shapes); err != nil {
		return err
	}
	return text.drawPrepared(pass, labels)
}

func drawWebGPUSurvivalInventory(pass *wgpu.RenderPassEncoder, hud *webGPUHUDRenderer, text *webGPUTextRenderer, state *InputState, width, height uint32) error {
	layout := survivalLayout(float32(width), float32(height))
	s := layout.S
	shapes := newWebGPUHUDBuilder(width, height)
	labels := newWebGPUTextBatch(width, height)
	shapes.rect(0, 0, float32(width), float32(height), webGPUUIDim)
	panel := layout.Rect(0, 0, 1000, 620)
	shapes.rect(panel.X, panel.Y, panel.Width, panel.Height, webGPUUIPanel)
	shapes.border(panel.X, panel.Y, panel.Width, panel.Height, max(s, 1), webGPUUILine)
	shapes.rect(panel.X, panel.Y, panel.Width, 3*s, webGPUUIAccent)

	labels.shadowText(layout.X+24*s, layout.Y+23*s, max(float32(24)*s, 15), "INVENTORY", webGPUUIText)
	station := "FIELD CRAFTING"
	if state.CraftingStation != 0 {
		station = "WORKBENCH"
	}
	labels.shadowText(layout.X+354*s, layout.Y+30*s, max(float32(14)*s, 10), station, webGPUUIAccent)
	labels.shadowText(layout.X+24*s, layout.Y+77*s, max(float32(12)*s, 9), "RECIPE BOOK", webGPUUIMuted)

	// Recipe filter buttons line up exactly with the existing Raylib hit testing.
	for _, button := range []struct {
		x, w float32
		name string
		on   bool
	}{{24, 138, "ALL RECIPES", !state.RecipesReadyOnly}, {170, 138, "READY TO CRAFT", state.RecipesReadyOnly}} {
		r := layout.Rect(button.x, 96, button.w, 32)
		fill := webGPUUISlot
		if button.on {
			fill = [4]float32{0.18, 0.25, 0.18, 1}
		}
		shapes.rect(r.X, r.Y, r.Width, r.Height, fill)
		shapes.border(r.X, r.Y, r.Width, r.Height, max(s, 1), webGPUUILine)
		if button.on {
			shapes.rect(r.X, r.Y+r.Height-3*s, r.Width, 3*s, webGPUUIAccent)
		}
		labels.shadowText(r.X+8*s, r.Y+9*s, max(float32(11)*s, 8), button.name, webGPUUIText)
	}

	inv := inventoryFor(client)
	rows := recipeBook(inv, state.CraftingStation, state.RecipesReadyOnly)
	offset := max(0, min(state.CraftingScroll, max(0, len(rows)-6)))
	chosen := selectedBookRecipe(state, rows)
	for i := 0; i < 6 && offset+i < len(rows); i++ {
		recipe := rows[offset+i]
		r := layout.Row(i)
		fill := webGPUUISlot
		selected := chosen != nil && chosen.Result.ID == recipe.Result.ID
		if selected {
			fill = [4]float32{0.18, 0.24, 0.18, 1}
		}
		shapes.rect(r.X, r.Y, r.Width, r.Height, fill)
		shapes.border(r.X, r.Y, r.Width, r.Height, max(s, 1), webGPUUILine)
		if selected {
			shapes.rect(r.X, r.Y, 3*s, r.Height, webGPUUIAccent)
		}
		icon := ItemStack{ID: recipe.Result.ID, Count: recipe.Result.Count}
		shapes.addItemStack(icon, r.X+7*s, r.Y+6*s, 46*s)
		labels.shadowText(r.X+64*s, r.Y+10*s, max(float32(14)*s, 9), GetItem(byte(recipe.Result.ID)).Name, webGPUUIText)
		status := "MISSING MATERIALS"
		color := webGPUUIMuted
		if !recipeAvailable(state, recipe) {
			status = "REQUIRES WORKBENCH"
		} else if n := craftCapacity(inv, recipe); n > 0 {
			status = fmt.Sprintf("READY / %d BATCHES", n)
			color = webGPUUIAccent
		} else if inv.HasItems(recipe.Ingredients) {
			status = "INVENTORY FULL"
		}
		labels.shadowText(r.X+64*s, r.Y+34*s, max(float32(10)*s, 8), status, color)
	}
	labels.shadowText(layout.X+24*s, layout.Y+544*s, max(float32(11)*s, 8), fmt.Sprintf("%d RECIPES / WHEEL TO BROWSE", len(rows)), webGPUUIMuted)

	// Selected recipe summary and the two existing craft hit targets.
	detail := layout.Rect(342, 84, 628, 174)
	shapes.rect(detail.X, detail.Y, detail.Width, detail.Height, [4]float32{0.075, 0.10, 0.085, 1})
	shapes.border(detail.X, detail.Y, detail.Width, detail.Height, max(s, 1), webGPUUILine)
	if chosen != nil {
		shapes.addItemStack(chosen.Result, layout.X+354*s, layout.Y+96*s, 48*s)
		labels.shadowText(layout.X+416*s, layout.Y+98*s, max(float32(19)*s, 12), GetItem(byte(chosen.Result.ID)).Name, webGPUUIText)
		labels.shadowText(layout.X+416*s, layout.Y+128*s, max(float32(11)*s, 8), fmt.Sprintf("MAKES %d", chosen.Result.Count), webGPUUIMuted)
		for i, ing := range chosen.Ingredients {
			x := float32(354 + i*306)
			r := layout.Rect(x, 156, 298, 48)
			shapes.rect(r.X, r.Y, r.Width, r.Height, webGPUUISlot)
			shapes.border(r.X, r.Y, r.Width, r.Height, max(s, 1), webGPUUILine)
			shapes.addItemIcon(byte(ing.ID), layout.X+(x+4)*s, layout.Y+160*s, 38*s)
			labels.shadowText(layout.X+(x+50)*s, layout.Y+160*s, max(float32(11)*s, 8), GetItem(byte(ing.ID)).Name, webGPUUIText)
			have := inv.CountItem(ing.ID)
			color := webGPUUIAccent
			if have < ing.Count {
				color = webGPUUIWarning
			}
			labels.shadowText(layout.X+(x+50)*s, layout.Y+181*s, max(float32(10)*s, 8), fmt.Sprintf("%d / %d", have, ing.Count), color)
		}
		n := craftCapacity(inv, chosen)
		enabled := n > 0 && recipeAvailable(state, chosen)
		for _, button := range []struct {
			x, w  float32
			label string
		}{{354, 286, "CRAFT"}, {648, 310, fmt.Sprintf("CRAFT MAX (%d)", n)}} {
			r := layout.Rect(button.x, 216, button.w, 34)
			fill := webGPUUISlot
			color := webGPUUIMuted
			if enabled {
				fill = [4]float32{0.18, 0.25, 0.18, 1}
				color = webGPUUIText
			}
			shapes.rect(r.X, r.Y, r.Width, r.Height, fill)
			shapes.border(r.X, r.Y, r.Width, r.Height, max(s, 1), webGPUUILine)
			labels.shadowText(r.X+10*s, r.Y+10*s, max(float32(11)*s, 8), button.label, color)
		}
	}

	labels.shadowText(layout.X+354*s, layout.Y+272*s, max(float32(11)*s, 8), "BACKPACK", webGPUUIMuted)
	used := 0
	for _, stack := range inv.Slots {
		if stack.ID != 0 {
			used++
		}
	}
	labels.shadowText(layout.X+850*s, layout.Y+272*s, max(float32(10)*s, 8), fmt.Sprintf("%d/36", used), webGPUUIMuted)
	for i := 0; i < 36; i++ {
		r := layout.Slot(i)
		shapes.rect(r.X, r.Y, r.Width, r.Height, webGPUUISlot)
		shapes.border(r.X, r.Y, r.Width, r.Height, max(s, 1), webGPUUILine)
		if i == state.SelectedSlot {
			shapes.border(r.X-2*s, r.Y-2*s, r.Width+4*s, r.Height+4*s, max(2*s, 2), webGPUUIAccent)
		}
		stack := inv.Slots[i]
		shapes.addItemStack(stack, r.X, r.Y, r.Width)
		webGPUAddStackText(labels, stack, r.X, r.Y, r.Width, s)
		if i < 9 {
			labels.shadowText(r.X+4*s, r.Y+3*s, max(float32(9)*s, 7), fmt.Sprint(i+1), webGPUUIMuted)
		}
	}
	labels.shadowText(layout.X+354*s, layout.Y+594*s, max(float32(10)*s, 8), "LMB MOVE  RMB SPLIT  SHIFT+CLICK QUICK MOVE", webGPUUIMuted)

	if state.CursorItem.ID > 0 && state.CursorItem.ID <= 255 {
		mouse := webGPUMousePosition()
		size := 50 * s
		shapes.addItemStack(state.CursorItem, mouse.X-size/2, mouse.Y-size/2, size)
		webGPUAddStackText(labels, state.CursorItem, mouse.X-size/2, mouse.Y-size/2, size, s)
	}
	if err := hud.drawPrepared(pass, shapes); err != nil {
		return err
	}
	return text.drawPrepared(pass, labels)
}
