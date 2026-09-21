package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) drawContainer(state *InputState) {
	view := state.Container
	if view == nil {
		return
	}
	c := &view.State
	l := legacySurvivalLayoutFor(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(9, 17, 22, 190))
	inventoryBox(l.Rect(0, 0, 1000, 620), invBackground, invLine)
	rl.DrawRectangleRec(l.Rect(0, 0, 1000, 3), invAccent)
	title := "CHEST"
	if c.Kind == blockFurnace {
		title = "FURNACE"
	}
	l.Text(title, 24, 23, 24, invText)
	inventoryBox(l.Rect(938, 22, 38, 32), invSlot, invLine)
	l.Text("X", 950, 29, 14, invMuted)
	l.Text("INVENTORY", 198, 279, 15, invMuted)
	if c.Kind == blockFurnace {
		l.Text("INPUT", 330, 57, 13, invMuted)
		l.Text("FUEL", 330, 148, 13, invMuted)
		l.Text("OUTPUT", 600, 96, 13, invMuted)
		inventoryBox(l.Rect(425, 139, 140, 20), invSlot, invLine)
		rl.DrawRectangleRec(l.Rect(427, 141, 136*float32(c.Cook)/200, 16), invAccent)
		l.Text(fmt.Sprintf("%d%%", c.Cook/2), 469, 170, 16, invText)
		burn := float32(0)
		if c.BurnTotal > 0 {
			burn = float32(c.Burn) / float32(c.BurnTotal)
		}
		rl.DrawRectangleRec(l.Rect(406, 183, 10, 48), invSlot)
		rl.DrawRectangleRec(l.Rect(406, 183+48*(1-burn), 10, 48*burn), rl.NewColor(233, 157, 75, 255))
		status := "Add ore and fuel"
		if c.Burn > 0 {
			status = "Burning"
		}
		if c.Slots[0].ID != 0 && smeltResult(c.Slots[0].ID) != 0 && c.Slots[2].Count >= StackLimit(smeltResult(c.Slots[0].ID)) {
			status = "Output full"
		}
		l.Text(status, 600, 211, 14, invMuted)
	}
	inv := inventoryFor(client)
	mouse := rl.GetMousePosition()
	var hovered ItemStack
	for i := 0; i < len(c.Slots)+36; i++ {
		slot := ItemStack{}
		if i < len(c.Slots) {
			slot = c.Slots[i]
		} else {
			slot = inv.Slots[i-len(c.Slots)]
		}
		r := raylibRect(containerUISlot(l.SurvivalLayout, c.Kind, i))
		inventoryBox(r, invSlot, invLine)
		if rl.CheckCollisionPointRec(mouse, r) {
			rl.DrawRectangleLinesEx(r, 2*l.S, invAccent)
			hovered = slot
		}
		a.drawInventoryItem(slot, r, l.S)
	}
	l.Text("LMB Move   RMB Split / place one   Shift-click Quick move", 198, 589, 13, invMuted)
	if state.CursorItem.ID != 0 {
		a.drawInventoryItem(state.CursorItem, rl.NewRectangle(mouse.X-25*l.S, mouse.Y-25*l.S, 50*l.S, 50*l.S), l.S)
	} else if hovered.ID != 0 {
		label := GetItem(byte(hovered.ID)).Name
		if max := GetItem(byte(hovered.ID)).MaxDurability; max > 0 {
			label += fmt.Sprintf("  %d / %d", max-hovered.Damage, max)
		}
		fs := int32(14 * l.S)
		width := float32(rl.MeasureText(label, fs)) + 16*l.S
		x := min(mouse.X+15*l.S, float32(rl.GetScreenWidth())-width-4)
		y := min(mouse.Y+15*l.S, float32(rl.GetScreenHeight())-30*l.S)
		inventoryBox(rl.NewRectangle(x, y, width, 26*l.S), invBackground, invLine)
		inventoryText(label, x+8*l.S, y+5*l.S, fs, invText)
	}
}
