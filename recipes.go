package main

import "fmt"

// Recipe defines a crafting recipe
type Recipe struct {
	ID          int    // Unique ID for networking
	Ingredients []Item // Required items
	Result      Item   // Output item
	Station     byte   // Required station (0 = Hand, blockCraftingTable, etc)
	Description string // Auto-generated or custom description
}

var RecipeRegistry = []*Recipe{}

// RegisterRecipe adds a recipe at its stable network ID. IDs must never be
// reassigned to a different recipe once shipped.
func RegisterRecipe(id int, r *Recipe) {
	if id < 0 {
		panic(fmt.Sprintf("invalid recipe id %d", id))
	}
	if id >= len(RecipeRegistry) {
		RecipeRegistry = append(RecipeRegistry, make([]*Recipe, id-len(RecipeRegistry)+1)...)
	}
	if RecipeRegistry[id] != nil {
		panic(fmt.Sprintf("duplicate recipe id %d", id))
	}
	r.ID = id
	RecipeRegistry[id] = r
}

// InitRecipes initializes all game recipes
func InitRecipes() {
	if len(RecipeRegistry) > 0 {
		return
	}
	fmt.Println("Initializing Recipes...")

	// 1. Logs -> Planks (Specific). IDs 0-2 are legacy protocol IDs.
	RegisterRecipe(0, &Recipe{
		Ingredients: []Item{{ID: int32(blockLog), Count: 1}},
		Result:      Item{ID: int32(blockPlankOak), Count: 4}, // Default Oak
		Station:     0,
	})
	RegisterRecipe(1, &Recipe{
		Ingredients: []Item{{ID: int32(blockLogBirch), Count: 1}},
		Result:      Item{ID: int32(blockPlankBirch), Count: 4},
		Station:     0,
	})
	RegisterRecipe(2, &Recipe{
		Ingredients: []Item{{ID: int32(blockLogSpruce), Count: 1}},
		Result:      Item{ID: int32(blockPlankSpruce), Count: 4},
		Station:     0,
	})

	// 2. Plank Recipes (Sticks, Workbench, Wood Tools). Each base ID is
	// explicit so reordering plank variants cannot renumber network recipes.
	plankTypes := []struct {
		itemID int32
		baseID int
	}{
		{int32(blockPlank), 3},
		{int32(blockPlankOak), 8},
		{int32(blockPlankBirch), 13},
		{int32(blockPlankSpruce), 18},
	}

	for _, plank := range plankTypes {
		pID := plank.itemID
		RegisterRecipe(plank.baseID, &Recipe{
			Ingredients: []Item{{ID: pID, Count: 2}},
			Result:      Item{ID: int32(itemStick), Count: 4},
			Station:     0,
		})
		RegisterRecipe(plank.baseID+1, &Recipe{
			Ingredients: []Item{{ID: pID, Count: 4}},
			Result:      Item{ID: int32(blockCraftingTable), Count: 1},
			Station:     0,
		})
		RegisterRecipe(plank.baseID+2, &Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 3},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodPickaxe), Count: 1},
			Station: blockCraftingTable,
		})
		RegisterRecipe(plank.baseID+3, &Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 3},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodAxe), Count: 1},
			Station: blockCraftingTable,
		})
		RegisterRecipe(plank.baseID+4, &Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 1},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodShovel), Count: 1},
			Station: blockCraftingTable,
		})
	}

	// 3. Torch (Coal + Stick -> 4 Torches)
	RegisterRecipe(23, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemCoal), Count: 1},
			{ID: int32(itemStick), Count: 1},
		},
		Result:  Item{ID: int32(blockTorch), Count: 4},
		Station: 0,
	})

	// --- STONE TOOLS ---
	RegisterRecipe(24, &Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStonePickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(25, &Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStoneAxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(26, &Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStoneShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- IRON TOOLS ---
	RegisterRecipe(27, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(28, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronAxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(29, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- GOLD TOOLS ---
	RegisterRecipe(30, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(31, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldAxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(32, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- DIAMOND TOOLS ---
	RegisterRecipe(33, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(34, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondAxe), Count: 1},
		Station: blockCraftingTable,
	})
	RegisterRecipe(35, &Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondShovel), Count: 1},
		Station: blockCraftingTable,
	})

	RegisterRecipe(36, &Recipe{Ingredients: []Item{{ID: int32(blockCobblestone), Count: 8}}, Result: Item{ID: int32(blockFurnace), Count: 1}, Station: blockCraftingTable})
	chestRecipes := []struct {
		plankID  byte
		recipeID int
	}{
		{blockPlank, 37},
		{blockPlankOak, 38},
		{blockPlankBirch, 39},
		{blockPlankSpruce, 40},
	}
	for _, chest := range chestRecipes {
		RegisterRecipe(chest.recipeID, &Recipe{Ingredients: []Item{{ID: int32(chest.plankID), Count: 8}}, Result: Item{ID: int32(blockChest), Count: 1}, Station: blockCraftingTable})
	}
	RegisterRecipe(41, &Recipe{
		Ingredients: []Item{{ID: int32(itemGunpowder), Count: 5}, {ID: int32(blockSand), Count: 4}},
		Result:      Item{ID: int32(blockTNT), Count: 1}, Station: blockCraftingTable,
	})
	for i, hoe := range []struct{ material, tool byte }{
		{blockPlankOak, itemWoodHoe}, {blockCobblestone, itemStoneHoe},
		{itemIronIngot, itemIronHoe}, {itemDiamond, itemDiamondHoe}, {itemGoldIngot, itemGoldHoe},
	} {
		RegisterRecipe(42+i, &Recipe{Ingredients: []Item{{ID: int32(hoe.material), Count: 2}, {ID: int32(itemStick), Count: 2}}, Result: Item{ID: int32(hoe.tool), Count: 1}, Station: blockCraftingTable})
	}
	RegisterRecipe(47, &Recipe{Ingredients: []Item{{ID: int32(itemWheat), Count: 3}}, Result: Item{ID: int32(itemBread), Count: 1}, Station: blockCraftingTable})
	for tier, material := range []struct{ ingredient, helmet byte }{
		{itemGoldIngot, itemGoldHelmet}, {itemIronIngot, itemIronHelmet}, {itemDiamond, itemDiamondHelmet},
	} {
		for part, count := range [...]int32{5, 8, 7, 4} {
			RegisterRecipe(48+tier*armorSlotCount+part, &Recipe{Ingredients: []Item{{ID: int32(material.ingredient), Count: count}}, Result: Item{ID: int32(material.helmet) + int32(part), Count: 1}, Station: blockCraftingTable})
		}
	}
	for i, sword := range []struct{ ingredient, result byte }{
		{blockPlankOak, itemWoodSword}, {blockCobblestone, itemStoneSword}, {itemIronIngot, itemIronSword}, {itemDiamond, itemDiamondSword}, {itemGoldIngot, itemGoldSword},
	} {
		RegisterRecipe(60+i, &Recipe{Ingredients: []Item{{ID: int32(sword.ingredient), Count: 2}, {ID: int32(itemStick), Count: 1}}, Result: Item{ID: int32(sword.result), Count: 1}, Station: blockCraftingTable})
	}
	RegisterRecipe(65, &Recipe{Ingredients: []Item{{ID: int32(itemStick), Count: 3}, {ID: int32(itemString), Count: 3}}, Result: Item{ID: int32(itemBow), Count: 1}, Station: blockCraftingTable})
	RegisterRecipe(66, &Recipe{Ingredients: []Item{{ID: int32(blockWhiteWool), Count: 3}, {ID: int32(blockPlankOak), Count: 3}}, Result: Item{ID: int32(blockBed), Count: 1}, Station: blockCraftingTable})
}

// GetCraftableRecipes returns recipes that can be crafted with the given inventory
// and nearby stations.
func GetCraftableRecipes(inv *Inventory, nearbyBlocks []byte) []*Recipe {
	craftable := []*Recipe{}

	// Helper map for station availability
	stations := make(map[byte]bool)
	stations[0] = true // Hand always available
	for _, b := range nearbyBlocks {
		stations[b] = true
	}

	for _, r := range RecipeRegistry {
		if r == nil {
			continue
		}
		// 1. Check Station
		if !stations[r.Station] {
			continue
		}

		// 2. Check Ingredients
		if inv.HasItems(r.Ingredients) {
			craftable = append(craftable, r)
		}
	}
	return craftable
}

// CanCraft checks if the inventory has resources for a specific recipe
func CanCraft(inv *Inventory, recipeID int) *Recipe {
	if recipeID < 0 || recipeID >= len(RecipeRegistry) {
		return nil
	}
	r := RecipeRegistry[recipeID]
	if r != nil && inv.HasItems(r.Ingredients) {
		return r
	}
	return nil
}
