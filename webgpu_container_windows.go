//go:build windows

package main

import (
	"fmt"

	"github.com/gogpu/wgpu"
)

// drawWebGPUContainerOverlay mirrors the existing chest/furnace presentation
// while keeping the authoritative click handling in updateContainerInput.
func drawWebGPUContainerOverlay(pass *wgpu.RenderPassEncoder, worldRenderer *webGPUWorldRenderer, state *InputState) error {
	if pass == nil || worldRenderer == nil || state == nil || state.Container == nil {
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

	view := state.Container
	container := &view.State
	layout := survivalLayout(float32(worldRenderer.width), float32(worldRenderer.height))
	scale := layout.S
	shapes := newWebGPUHUDBuilder(worldRenderer.width, worldRenderer.height)
	labels := newWebGPUTextBatch(worldRenderer.width, worldRenderer.height)

	shapes.rect(0, 0, float32(worldRenderer.width), float32(worldRenderer.height), webGPUUIDim)
	panel := layout.Rect(0, 0, 1000, 620)
	shapes.rect(panel.X, panel.Y, panel.Width, panel.Height, webGPUUIPanel)
	shapes.border(panel.X, panel.Y, panel.Width, panel.Height, max(scale, 1), webGPUUILine)
	shapes.rect(panel.X, panel.Y, panel.Width, 3*scale, webGPUUIAccent)

	title := "CHEST"
	if container.Kind == blockFurnace {
		title = "FURNACE"
	}
	labels.shadowText(layout.X+24*scale, layout.Y+23*scale, max(float32(24)*scale, 15), title, webGPUUIText)
	closeRect := layout.Rect(938, 22, 38, 32)
	shapes.rect(closeRect.X, closeRect.Y, closeRect.Width, closeRect.Height, webGPUUISlot)
	shapes.border(closeRect.X, closeRect.Y, closeRect.Width, closeRect.Height, max(scale, 1), webGPUUILine)
	labels.shadowText(closeRect.X+12*scale, closeRect.Y+7*scale, max(float32(14)*scale, 10), "X", webGPUUIMuted)
	labels.shadowText(layout.X+198*scale, layout.Y+279*scale, max(float32(15)*scale, 10), "INVENTORY", webGPUUIMuted)

	if container.Kind == blockFurnace {
		drawWebGPUFurnaceStatus(shapes, labels, layout, container)
	}

	inv := inventoryFor(client)
	mouse := webGPUMousePosition()
	hovered := ItemStack{}
	hasHover := false
	for i := 0; i < len(container.Slots)+36; i++ {
		stack := ItemStack{}
		if i < len(container.Slots) {
			stack = container.Slots[i]
		} else {
			stack = inv.Slots[i-len(container.Slots)]
		}
		r := containerUISlot(layout, container.Kind, i)
		shapes.inventorySlot(r.X, r.Y, r.Width, r.Height, scale, webGPUUISlot)
		if mouse.X >= r.X && mouse.X <= r.X+r.Width && mouse.Y >= r.Y && mouse.Y <= r.Y+r.Height {
			shapes.border(r.X-2*scale, r.Y-2*scale, r.Width+4*scale, r.Height+4*scale, max(2*scale, 2), webGPUUIAccent)
			hovered = stack
			hasHover = true
		}
		iconSize := min(r.Width, r.Height)
		shapes.addItemStack(stack, r.X+(r.Width-iconSize)/2, r.Y+(r.Height-iconSize)/2, iconSize)
		webGPUAddStackText(labels, stack, r.X+(r.Width-iconSize)/2, r.Y+(r.Height-iconSize)/2, iconSize, scale)
		if i >= len(container.Slots) && i-len(container.Slots) < 9 {
			labels.shadowText(r.X+4*scale, r.Y+3*scale, max(float32(10)*scale, 8), fmt.Sprint(i-len(container.Slots)+1), webGPUUIMuted)
		}
	}

	labels.shadowText(layout.X+198*scale, layout.Y+589*scale, max(float32(13)*scale, 9), "LMB MOVE   RMB SPLIT / PLACE ONE   SHIFT-CLICK QUICK MOVE", webGPUUIMuted)

	if state.CursorItem.ID > 0 && state.CursorItem.ID <= 255 {
		size := 50 * scale
		x, y := mouse.X-size/2, mouse.Y-size/2
		shapes.addItemStack(state.CursorItem, x, y, size)
		webGPUAddStackText(labels, state.CursorItem, x, y, size, scale)
	} else if hasHover && hovered.ID > 0 && hovered.ID <= 255 {
		label := GetItem(byte(hovered.ID)).Name
		if maxDurability := GetItem(byte(hovered.ID)).MaxDurability; maxDurability > 0 {
			label += fmt.Sprintf("  %d / %d", maxDurability-hovered.Damage, maxDurability)
		}
		fontSize := max(float32(14)*scale, 10)
		boxWidth := max(webGPUTextWidth(fontSize, label)+20*scale, 150*scale)
		boxHeight := 30 * scale
		x := min(mouse.X+15*scale, float32(worldRenderer.width)-boxWidth-4)
		y := min(mouse.Y+15*scale, float32(worldRenderer.height)-boxHeight-4)
		shapes.rect(x, y, boxWidth, boxHeight, webGPUUIPanel)
		shapes.border(x, y, boxWidth, boxHeight, max(scale, 1), webGPUUILine)
		labels.shadowText(x+8*scale, y+7*scale, fontSize, label, webGPUUIText)
	}

	if err := hud.drawPrepared(pass, shapes); err != nil {
		return err
	}
	return text.drawPrepared(pass, labels)
}

func drawWebGPUFurnaceStatus(shapes *webGPUHUDBuilder, labels *webGPUTextBatch, layout SurvivalLayout, container *BlockContainer) {
	if shapes == nil || labels == nil || container == nil {
		return
	}
	scale := layout.S
	labels.shadowText(layout.X+330*scale, layout.Y+57*scale, max(float32(13)*scale, 9), "INPUT", webGPUUIMuted)
	labels.shadowText(layout.X+330*scale, layout.Y+148*scale, max(float32(13)*scale, 9), "FUEL", webGPUUIMuted)
	labels.shadowText(layout.X+600*scale, layout.Y+96*scale, max(float32(13)*scale, 9), "OUTPUT", webGPUUIMuted)

	progress := max(float32(0), min(float32(1), float32(container.Cook)/200.0))
	track := layout.Rect(425, 139, 140, 20)
	shapes.rect(track.X, track.Y, track.Width, track.Height, webGPUUISlot)
	shapes.border(track.X, track.Y, track.Width, track.Height, max(scale, 1), webGPUUILine)
	if progress > 0 {
		shapes.rect(track.X+2*scale, track.Y+2*scale, (track.Width-4*scale)*progress, track.Height-4*scale, webGPUUIAccent)
	}
	labels.shadowText(layout.X+469*scale, layout.Y+170*scale, max(float32(16)*scale, 10), fmt.Sprintf("%d%%", container.Cook/2), webGPUUIText)

	burn := float32(0)
	if container.BurnTotal > 0 {
		burn = max(float32(0), min(float32(1), float32(container.Burn)/float32(container.BurnTotal)))
	}
	burnTrack := layout.Rect(406, 183, 10, 48)
	shapes.rect(burnTrack.X, burnTrack.Y, burnTrack.Width, burnTrack.Height, webGPUUISlot)
	if burn > 0 {
		fillHeight := burnTrack.Height * burn
		shapes.rect(burnTrack.X, burnTrack.Y+burnTrack.Height-fillHeight, burnTrack.Width, fillHeight, [4]float32{0.91, 0.61, 0.29, 1})
	}

	status := "ADD ORE AND FUEL"
	statusColor := webGPUUIMuted
	if container.Burn > 0 {
		status = "BURNING"
		statusColor = webGPUUIAccent
	}
	if len(container.Slots) >= 3 && container.Slots[0].ID != 0 {
		result := smeltResult(container.Slots[0].ID)
		if result != 0 && container.Slots[2].Count >= StackLimit(result) {
			status = "OUTPUT FULL"
			statusColor = webGPUUIWarning
		}
	}
	labels.shadowText(layout.X+600*scale, layout.Y+211*scale, max(float32(14)*scale, 10), status, statusColor)
}
