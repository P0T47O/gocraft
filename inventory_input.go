package main

func inventoryFor(c *Client) *Inventory {
	if c != nil {
		return &c.Inventory
	}
	return &localInventory
}
func selectedBookRecipe(state *InputState, rows []*Recipe) *Recipe {
	for _, r := range rows {
		if r.Result.ID == state.RecipeSelected {
			return r
		}
	}
	if len(rows) > 0 {
		return rows[0]
	}
	return nil
}
func recipeAvailable(state *InputState, r *Recipe) bool {
	return r != nil && (r.Station == 0 || r.Station == state.CraftingStation)
}

func (s *InputState) updateCraftingInput(l SurvivalLayout, c *Client) {
	mouse := inputMousePosition()
	click := inputMousePressed(mouseLeft)
	if click && uiContainsPoint(mouse, l.Rect(938, 22, 38, 32)) {
		s.InventoryOpen = false
		s.CraftingStation = 0
		s.SkipCamera = true
		captureCursor()
		return
	}
	if click && uiContainsPoint(mouse, l.Rect(24, 96, 138, 32)) {
		s.RecipesReadyOnly = false
		s.CraftingScroll = 0
	}
	if click && uiContainsPoint(mouse, l.Rect(170, 96, 138, 32)) {
		s.RecipesReadyOnly = true
		s.CraftingScroll = 0
	}
	inv := inventoryFor(c)
	rows := recipeBook(inv, s.CraftingStation, s.RecipesReadyOnly)
	if uiContainsPoint(mouse, l.Rect(24, 144, 284, 384)) {
		s.CraftingScroll -= int(inputMouseWheel())
	}
	s.CraftingScroll = max(0, min(s.CraftingScroll, max(0, len(rows)-6)))
	for i := 0; i < 6 && i+s.CraftingScroll < len(rows); i++ {
		if click && uiContainsPoint(mouse, l.Row(i)) {
			s.RecipeSelected = rows[i+s.CraftingScroll].Result.ID
		}
	}
	r := selectedBookRecipe(s, rows)
	if !click || !recipeAvailable(s, r) || craftCapacity(inv, r) == 0 {
		return
	}
	count := 0
	if uiContainsPoint(mouse, l.Rect(354, 216, 286, 34)) {
		count = 1
		if inputKeyDown(keyLeftShift) || inputKeyDown(keyRightShift) {
			count = 64
		}
	}
	if uiContainsPoint(mouse, l.Rect(648, 216, 310, 34)) {
		count = 64
	}
	if count > 0 {
		if c != nil {
			c.Send(&PacketCraft{RecipeID: int32(r.ID), Count: int32(count)})
		} else {
			inv.Craft(r, count)
		}
	}
}
