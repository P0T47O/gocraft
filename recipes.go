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

// RegisterRecipe adds a recipe to the registry
func RegisterRecipe(r *Recipe) {
	r.ID = len(RecipeRegistry)
	RecipeRegistry = append(RecipeRegistry, r)
}

// InitRecipes initializes all game recipes
func InitRecipes() {
	if len(RecipeRegistry) > 0 {
		return
	}
	fmt.Println("Initializing Recipes...")

	// 1. Logs -> Planks (Specific)
	RegisterRecipe(&Recipe{
		Ingredients: []Item{{ID: int32(blockLog), Count: 1}},
		Result:      Item{ID: int32(blockPlankOak), Count: 4}, // Default Oak
		Station:     0,
	})
	RegisterRecipe(&Recipe{
		Ingredients: []Item{{ID: int32(blockLogBirch), Count: 1}},
		Result:      Item{ID: int32(blockPlankBirch), Count: 4},
		Station:     0,
	})
	RegisterRecipe(&Recipe{
		Ingredients: []Item{{ID: int32(blockLogSpruce), Count: 1}},
		Result:      Item{ID: int32(blockPlankSpruce), Count: 4},
		Station:     0,
	})

	// 2. Plank Recipes (Sticks, Workbench, Wood Tools)
	// Supports all plank types
	plankTypes := []int32{
		int32(blockPlank),
		int32(blockPlankOak),
		int32(blockPlankBirch),
		int32(blockPlankSpruce),
	}

	for _, pID := range plankTypes {
		// Planks -> Sticks
		RegisterRecipe(&Recipe{
			Ingredients: []Item{{ID: pID, Count: 2}},
			Result:      Item{ID: int32(itemStick), Count: 4},
			Station:     0,
		})

		// Planks -> Crafting Table
		RegisterRecipe(&Recipe{
			Ingredients: []Item{{ID: pID, Count: 4}},
			Result:      Item{ID: int32(blockCraftingTable), Count: 1},
			Station:     0,
		})

		// Wood Pickaxe
		RegisterRecipe(&Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 3},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodPickaxe), Count: 1},
			Station: blockCraftingTable,
		})

		// Wood Axe
		RegisterRecipe(&Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 3},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodAxe), Count: 1},
			Station: blockCraftingTable,
		})

		// Wood Shovel
		RegisterRecipe(&Recipe{
			Ingredients: []Item{
				{ID: pID, Count: 1},
				{ID: int32(itemStick), Count: 2},
			},
			Result:  Item{ID: int32(itemWoodShovel), Count: 1},
			Station: blockCraftingTable,
		})
	}

	// 3. Torch (Coal + Stick -> 4 Torches)
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemCoal), Count: 1},
			{ID: int32(itemStick), Count: 1},
		},
		Result:  Item{ID: int32(blockTorch), Count: 4},
		Station: 0,
	})

	// --- STONE TOOLS ---
	// Stone Pickaxe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStonePickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Stone Axe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStoneAxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Stone Shovel
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(blockCobblestone), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemStoneShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- IRON TOOLS ---
	// Iron Pickaxe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Iron Axe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronAxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Iron Shovel
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemIronIngot), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemIronShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- GOLD TOOLS ---
	// Gold Pickaxe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Gold Axe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldAxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Gold Shovel
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemGoldIngot), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemGoldShovel), Count: 1},
		Station: blockCraftingTable,
	})

	// --- DIAMOND TOOLS ---
	// Diamond Pickaxe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondPickaxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Diamond Axe
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 3},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondAxe), Count: 1},
		Station: blockCraftingTable,
	})
	// Diamond Shovel
	RegisterRecipe(&Recipe{
		Ingredients: []Item{
			{ID: int32(itemDiamond), Count: 1},
			{ID: int32(itemStick), Count: 2},
		},
		Result:  Item{ID: int32(itemDiamondShovel), Count: 1},
		Station: blockCraftingTable,
	})
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
	if inv.HasItems(r.Ingredients) {
		return r
	}
	return nil
}
