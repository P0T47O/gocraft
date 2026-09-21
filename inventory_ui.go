package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func (a *RenderAssets) drawInventoryItem(item Item, r rl.Rectangle, s float32) {
	if item.ID == 0 {
		return
	}
	a.drawIcon(byte(item.ID), r.X+3*s, r.Y+3*s, r.Width-6*s)
	if max := GetItem(byte(item.ID)).MaxDurability; max > 0 && item.Damage > 0 {
		fraction := float32(max-item.Damage) / float32(max)
		bar := rl.NewRectangle(r.X+5*s, r.Y+r.Height-7*s, r.Width-10*s, 3*s)
		rl.DrawRectangleRec(bar, rl.Black)
		bar.Width *= fraction
		color := invAccent
		if fraction < .25 {
			color = rl.NewColor(235, 85, 75, 255)
		} else if fraction < .5 {
			color = rl.NewColor(230, 185, 65, 255)
		}
		rl.DrawRectangleRec(bar, color)
	}
	if item.Count > 1 {
		label := fmt.Sprint(item.Count)
		fs := int32(14 * s)
		x := r.X + r.Width - float32(rl.MeasureText(label, fs)) - 4*s
		y := r.Y + r.Height - float32(fs) - 3*s
		inventoryText(label, x+s, y+s, fs, rl.Black)
		inventoryText(label, x, y, fs, invText)
	}
}
func (a *RenderAssets) drawSurvivalInventory(state *InputState) {
	l := legacySurvivalLayoutFor(float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight()))
	inv := inventoryFor(client)
	mouse := rl.GetMousePosition()
	rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.NewColor(9, 17, 22, 190))
	rl.DrawRectangleRec(l.Rect(7, 9, 1000, 620), rl.Fade(rl.Black, 0.3))
	inventoryBox(l.Rect(0, 0, 1000, 620), invBackground, invLine)
	rl.DrawRectangleRec(l.Rect(0, 0, 1000, 3), invAccent)
	l.Text("INVENTORY", 24, 23, 24, invText)
	station := "FIELD CRAFTING"
	if state.CraftingStation != 0 {
		station = "WORKBENCH"
	}
	l.Text(station, 354, 30, 14, invAccent)
	inventoryButton(l.Rect(938, 22, 38, 32), "X", true, false, l.S)
	rl.DrawRectangleRec(l.Rect(24, 68, 952, 1), invLine)
	l.Text("RECIPE BOOK", 24, 77, 12, invMuted)
	inventoryButton(l.Rect(24, 96, 138, 32), "All recipes", true, !state.RecipesReadyOnly, l.S)
	inventoryButton(l.Rect(170, 96, 138, 32), "Ready to craft", true, state.RecipesReadyOnly, l.S)
	rows := recipeBook(inv, state.CraftingStation, state.RecipesReadyOnly)
	offset := max(0, min(state.CraftingScroll, max(0, len(rows)-6)))
	chosen := selectedBookRecipe(state, rows)
	if len(rows) == 0 {
		l.Text("Nothing ready yet.", 36, 168, 16, invText)
		l.Text("Collect wood or switch", 36, 200, 14, invMuted)
		l.Text("to All recipes for a guide.", 36, 222, 14, invMuted)
	}
	for i := 0; i < 6 && offset+i < len(rows); i++ {
		r := rows[offset+i]
		rect := l.Row(i)
		fill := invPanel
		selected := chosen != nil && chosen.Result.ID == r.Result.ID
		if selected {
			fill = rl.NewColor(57, 72, 57, 255)
		} else if rl.CheckCollisionPointRec(mouse, rect) {
			fill = invSlot
		}
		inventoryBox(rect, fill, invLine)
		if selected {
			rl.DrawRectangleRec(rl.NewRectangle(rect.X, rect.Y, 3*l.S, rect.Height), invAccent)
		}
		a.drawInventoryItem(r.Result, rl.NewRectangle(rect.X+7*l.S, rect.Y+6*l.S, 46*l.S, 46*l.S), l.S)
		inventoryText(GetItemVisual(byte(r.Result.ID)).Name, rect.X+64*l.S, rect.Y+11*l.S, int32(15*l.S), invText)
		status := "Missing materials"
		color := invMuted
		if !recipeAvailable(state, r) {
			status = "Requires workbench"
		} else if n := craftCapacity(inv, r); n > 0 {
			status = fmt.Sprintf("Ready  /  %d batches", n)
			color = invAccent
		} else if inv.HasItems(r.Ingredients) {
			status = "Inventory full"
		}
		inventoryText(status, rect.X+64*l.S, rect.Y+34*l.S, int32(12*l.S), color)
	}
	if len(rows) > 6 {
		track := l.Rect(316, 144, 3, 378)
		rl.DrawRectangleRec(track, invSlot)
		thumb := track.Height * 6 / float32(len(rows))
		track.Y += (track.Height - thumb) * float32(offset) / float32(len(rows)-6)
		track.Height = thumb
		rl.DrawRectangleRec(track, invAccent)
	}
	l.Text(fmt.Sprintf("%d recipes  /  scroll to browse", len(rows)), 24, 544, 12, invMuted)
	l.Text("Select a recipe to see", 24, 579, 13, invMuted)
	l.Text("materials and quantities.", 24, 597, 13, invMuted)
	rl.DrawRectangleRec(l.Rect(330, 84, 1, 492), invLine)
	inventoryBox(l.Rect(342, 84, 628, 174), invPanel, invLine)
	if chosen != nil {
		a.drawInventoryItem(chosen.Result, l.Rect(354, 96, 48, 48), l.S)
		l.Text(GetItemVisual(byte(chosen.Result.ID)).Name, 416, 99, 21, invText)
		l.Text(fmt.Sprintf("Makes %d  /  %s", chosen.Result.Count, map[bool]string{true: "Workbench recipe", false: "Hand recipe"}[chosen.Station != 0]), 416, 128, 13, invMuted)
		for i, ing := range chosen.Ingredients {
			x := float32(354 + i*306)
			inventoryBox(l.Rect(x, 156, 298, 48), invBackground, invLine)
			a.drawInventoryItem(Item{ID: ing.ID, Count: 1}, l.Rect(x+4, 160, 38, 38), l.S)
			l.Text(GetItemVisual(byte(ing.ID)).Name, x+50, 161, 13, invText)
			have := inv.CountItem(ing.ID)
			color := invAccent
			if have < ing.Count {
				color = invWarning
			}
			l.Text(fmt.Sprintf("%d / %d available", have, ing.Count), x+50, 181, 12, color)
		}
		n := craftCapacity(inv, chosen)
		enabled := n > 0 && recipeAvailable(state, chosen)
		label := fmt.Sprintf("Craft %d", chosen.Result.Count)
		if !recipeAvailable(state, chosen) {
			label = "Open a workbench to craft"
		} else if n == 0 {
			label = "Need more materials"
			if inv.HasItems(chosen.Ingredients) {
				label = "Make room in inventory"
			}
		}
		inventoryButton(l.Rect(354, 216, 286, 34), label, enabled, enabled, l.S)
		bulkLabel := "Craft max"
		if enabled {
			bulkLabel = fmt.Sprintf("Craft max  (%d items)", int(chosen.Result.Count)*n)
		}
		inventoryButton(l.Rect(648, 216, 310, 34), bulkLabel, enabled, false, l.S)
	} else {
		l.Text("Select a recipe from All recipes", 366, 124, 18, invMuted)
	}
	used := 0
	for _, s := range inv.Slots {
		if s.ID != 0 {
			used++
		}
	}
	l.Text("BACKPACK", 354, 272, 12, invMuted)
	l.Text(fmt.Sprintf("%d / 36 slots", used), 860, 272, 12, invMuted)
	hover := -1
	for i := 0; i < 36; i++ {
		r := l.Slot(i)
		fill := invSlot
		if rl.CheckCollisionPointRec(mouse, r) {
			fill = rl.NewColor(76, 89, 76, 255)
			hover = i
		}
		inventoryBox(r, fill, invLine)
		rl.DrawRectangleRec(rl.NewRectangle(r.X+1, r.Y+1, r.Width-2, 2*l.S), rl.NewColor(23, 29, 30, 255))
		if i == state.SelectedSlot {
			rl.DrawRectangleLinesEx(r, 2*l.S, invAccent)
		}
		a.drawInventoryItem(inv.Slots[i], r, l.S)
		if i < 9 {
			inventoryText(fmt.Sprint(i+1), r.X+4*l.S, r.Y+3*l.S, int32(10*l.S), invMuted)
		}
	}
	l.Text("HOTBAR", 354, 498, 12, invMuted)
	l.Text("LMB  Move    RMB  Split / place one    Shift-click  Quick move", 354, 594, 12, invMuted)
	if state.CursorItem.ID != 0 {
		a.drawInventoryItem(state.CursorItem, rl.NewRectangle(mouse.X-25*l.S, mouse.Y-25*l.S, 50*l.S, 50*l.S), l.S)
	} else if hover >= 0 && inv.Slots[hover].ID != 0 {
		item := inv.Slots[hover]
		name := GetItem(byte(item.ID)).Name
		if max := GetItem(byte(item.ID)).MaxDurability; max > 0 {
			name += fmt.Sprintf("  %d / %d", max-item.Damage, max)
		}
		fs := int32(16 * l.S)
		width := max(float32(rl.MeasureText(name, fs))+24*l.S, 160*l.S)
		x := min(mouse.X+16*l.S, float32(rl.GetScreenWidth())-width-8)
		y := min(mouse.Y+20*l.S, float32(rl.GetScreenHeight())-60*l.S)
		inventoryBox(rl.NewRectangle(x, y, width, 54*l.S), invBackground, invAccent)
		inventoryText(name, x+10*l.S, y+8*l.S, fs, invText)
		inventoryText(fmt.Sprintf("%d items  /  stack limit 64", item.Count), x+10*l.S, y+32*l.S, int32(11*l.S), invMuted)
	}
}
