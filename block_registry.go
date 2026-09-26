package main

import (
	"fmt"
)

type RenderType int

const (
	RenderTypeCube RenderType = iota
	RenderTypeTorch
	RenderTypeCross // For grass/flowers
	RenderTypeLiquid
	RenderTypeCutout // For leaves, etc.
	RenderTypeGlass
)

// BlockDef holds all static properties and behaviors for a block type.
type BlockDef struct {
	ID            byte
	Name          string
	Textures      blockFaces
	RenderType    RenderType
	IsTransparent bool
	IsOpaque      bool // Blocks light completely
	LightLevel    byte // 0-15
	IsCollidable  bool

	// Mining / Drops
	NoDrop           bool
	MiningConfigured bool
	Hardness         float32
	EffectiveTool    ToolType
	RequiredMaterial ToolMaterial // Minimum tier required (e.g. Iron Pickaxe for Diamond Ore)
	DropItem         byte         // 0 = Drops self (unless Air)
	DropCount        int          // Defaults to 1 if DropItem != 0

}

var Blocks [256]*BlockDef

func init() {
	// Initialize with air by default
	Blocks[blockAir] = &BlockDef{
		ID:            blockAir,
		Name:          "Air",
		RenderType:    RenderTypeCube,
		IsTransparent: true,
		IsOpaque:      false,
		LightLevel:    0,
		IsCollidable:  false,
	}
}

// RegisterBlock adds a block definition to the registry.
func RegisterBlock(def *BlockDef) {
	if def.ID == blockAir {
		return
	}
	if Blocks[def.ID] != nil {
		fmt.Printf("Warning: Overwriting block ID %d\n", def.ID)
	}
	Blocks[def.ID] = def
}

// GetBlock returns the block definition for a given ID.
func GetBlock(id byte) *BlockDef {
	def := Blocks[id]
	if def == nil {
		return Blocks[blockAir]
	}
	return def
}

func initBlockRegistry() {
	air := Blocks[blockAir]
	Blocks = [256]*BlockDef{}
	Blocks[blockAir] = air
	defer configureBasicMining()
	// Define and register all block types
	RegisterBlock(&BlockDef{
		ID: blockTNT, Name: "TNT",
		Textures:   blockFaces{Top: "textures/block/tnt_top.png", Bottom: "textures/block/tnt_bottom.png", North: "textures/block/tnt_side.png", South: "textures/block/tnt_side.png", East: "textures/block/tnt_side.png", West: "textures/block/tnt_side.png"},
		RenderType: RenderTypeCube, IsOpaque: true, IsCollidable: true,
	})
	RegisterBlock(&BlockDef{ID: blockFarmland, Name: "Farmland", Textures: blockFaces{Top: "textures/block/farmland.png", Bottom: "textures/block/dirt.png", North: "textures/block/dirt.png", South: "textures/block/dirt.png", East: "textures/block/dirt.png", West: "textures/block/dirt.png"}, RenderType: RenderTypeCube, IsOpaque: true, IsCollidable: true, DropItem: blockDirt})
	RegisterBlock(&BlockDef{ID: blockWhiteWool, Name: "White Wool", Textures: blockFaces{Top: "textures/block/white_wool.png", Bottom: "textures/block/white_wool.png", North: "textures/block/white_wool.png", South: "textures/block/white_wool.png", East: "textures/block/white_wool.png", West: "textures/block/white_wool.png"}, RenderType: RenderTypeCube, IsOpaque: true, IsCollidable: true})
	RegisterBlock(&BlockDef{ID: blockWoodDoor, Name: "Oak Door", Textures: blockFaces{Top: "textures/block/oak_door_bottom.png", Bottom: "textures/block/oak_door_top.png", North: "textures/block/oak_door_bottom.png", South: "textures/block/oak_door_bottom.png", East: "textures/block/oak_door_bottom.png", West: "textures/block/oak_door_bottom.png"}, RenderType: RenderTypeCube, IsTransparent: true, IsCollidable: true})
	for _, shaped := range []struct {
		id            byte
		name, texture string
	}{
		{blockOakSlab, "Oak Slab", "textures/block/oak_planks.png"},
		{blockOakStairs, "Oak Stairs", "textures/block/oak_planks.png"},
		{blockStoneSlab, "Stone Slab", "textures/block/stone.png"},
		{blockCobbleStairs, "Cobblestone Stairs", "textures/block/cobblestone.png"},
	} {
		faces := blockFaces{Top: shaped.texture, Bottom: shaped.texture, North: shaped.texture, South: shaped.texture, East: shaped.texture, West: shaped.texture}
		RegisterBlock(&BlockDef{ID: shaped.id, Name: shaped.name, Textures: faces, RenderType: RenderTypeCube, IsTransparent: true, IsCollidable: true})
	}
	RegisterBlock(&BlockDef{ID: blockBed, Name: "Bed", Textures: blockFaces{Top: "textures/block/red_bed_head_up.png", Bottom: "textures/block/bed_down.png", North: "textures/block/red_bed_head_east.png", South: "textures/block/red_bed_head_west.png", East: "textures/block/red_bed_head_east.png", West: "textures/block/red_bed_head_west.png"}, RenderType: RenderTypeCube, IsTransparent: true, IsCollidable: false})
	for _, crop := range []struct {
		id            byte
		name, texture string
	}{
		{blockWheatCrop, "Wheat Crop", "textures/block/wheat_stage0.png"},
		{blockPotatoCrop, "Potato Crop", "textures/block/potatoes_stage0.png"},
		{blockCarrotCrop, "Carrot Crop", "textures/block/carrots_stage0.png"},
	} {
		RegisterBlock(&BlockDef{ID: crop.id, Name: crop.name, Textures: blockFaces{North: crop.texture}, RenderType: RenderTypeCross, IsTransparent: true, IsCollidable: false, NoDrop: true})
	}
	RegisterBlock(&BlockDef{
		ID:           blockGrass,
		Name:         "Grass Block",
		Textures:     blockFaces{Top: "textures/block/grass_block_top.png", Bottom: "textures/block/dirt.png", North: "textures/block/grass_block_side.png", South: "textures/block/grass_block_side.png", East: "textures/block/grass_block_side.png", West: "textures/block/grass_block_side.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockStone,
		Name:         "Stone",
		Textures:     blockFaces{Top: "textures/block/stone.png", Bottom: "textures/block/stone.png", North: "textures/block/stone.png", South: "textures/block/stone.png", East: "textures/block/stone.png", West: "textures/block/stone.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockDirt,
		Name:         "Dirt",
		Textures:     blockFaces{Top: "textures/block/dirt.png", Bottom: "textures/block/dirt.png", North: "textures/block/dirt.png", South: "textures/block/dirt.png", East: "textures/block/dirt.png", West: "textures/block/dirt.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockCobblestone,
		Name:         "Cobblestone",
		Textures:     blockFaces{Top: "textures/block/cobblestone.png", Bottom: "textures/block/cobblestone.png", North: "textures/block/cobblestone.png", South: "textures/block/cobblestone.png", East: "textures/block/cobblestone.png", West: "textures/block/cobblestone.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockSand,
		Name:         "Sand",
		Textures:     blockFaces{Top: "textures/block/sand.png", Bottom: "textures/block/sand.png", North: "textures/block/sand.png", South: "textures/block/sand.png", East: "textures/block/sand.png", West: "textures/block/sand.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockGravel,
		Name:         "Gravel",
		Textures:     blockFaces{Top: "textures/block/gravel.png", Bottom: "textures/block/gravel.png", North: "textures/block/gravel.png", South: "textures/block/gravel.png", East: "textures/block/gravel.png", West: "textures/block/gravel.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockLeaves,
		Name:          "Oak Leaves",
		Textures:      blockFaces{Top: "textures/block/oak_leaves.png", Bottom: "textures/block/oak_leaves.png", North: "textures/block/oak_leaves.png", South: "textures/block/oak_leaves.png", East: "textures/block/oak_leaves.png", West: "textures/block/oak_leaves.png"},
		RenderType:    RenderTypeCutout,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockWater,
		Name:          "Water",
		Textures:      blockFaces{Top: "textures/block/water_still.png", Bottom: "textures/block/water_still.png", North: "textures/block/water_flow.png", South: "textures/block/water_flow.png", East: "textures/block/water_flow.png", West: "textures/block/water_flow.png"},
		RenderType:    RenderTypeLiquid,
		IsTransparent: true,
		IsCollidable:  false,
	})
	RegisterBlock(&BlockDef{
		ID:           blockLava,
		Name:         "Lava",
		Textures:     blockFaces{Top: "textures/block/lava_still.png", Bottom: "textures/block/lava_still.png", North: "textures/block/lava_flow.png", South: "textures/block/lava_flow.png", East: "textures/block/lava_flow.png", West: "textures/block/lava_flow.png"},
		RenderType:   RenderTypeLiquid,
		IsOpaque:     true,
		LightLevel:   15,
		IsCollidable: false,
	})
	RegisterBlock(&BlockDef{
		ID:            blockTorch,
		Name:          "Torch",
		Textures:      blockFaces{Top: "textures/block/torch.png", Bottom: "textures/block/torch.png", North: "textures/block/torch.png", South: "textures/block/torch.png", East: "textures/block/torch.png", West: "textures/block/torch.png"},
		RenderType:    RenderTypeTorch,
		IsTransparent: true,
		LightLevel:    14,
		IsCollidable:  false,
	})
	RegisterBlock(&BlockDef{
		ID:           blockCoalOre,
		Name:         "Coal Ore",
		Textures:     blockFaces{Top: "textures/block/coal_ore.png", Bottom: "textures/block/coal_ore.png", North: "textures/block/coal_ore.png", South: "textures/block/coal_ore.png", East: "textures/block/coal_ore.png", West: "textures/block/coal_ore.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockIronOre,
		Name:         "Iron Ore",
		Textures:     blockFaces{Top: "textures/block/iron_ore.png", Bottom: "textures/block/iron_ore.png", North: "textures/block/iron_ore.png", South: "textures/block/iron_ore.png", East: "textures/block/iron_ore.png", West: "textures/block/iron_ore.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockGoldOre,
		Name:         "Gold Ore",
		Textures:     blockFaces{Top: "textures/block/gold_ore.png", Bottom: "textures/block/gold_ore.png", North: "textures/block/gold_ore.png", South: "textures/block/gold_ore.png", East: "textures/block/gold_ore.png", West: "textures/block/gold_ore.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockDiamondOre,
		Name:         "Diamond Ore",
		Textures:     blockFaces{Top: "textures/block/diamond_ore.png", Bottom: "textures/block/diamond_ore.png", North: "textures/block/diamond_ore.png", South: "textures/block/diamond_ore.png", East: "textures/block/diamond_ore.png", West: "textures/block/diamond_ore.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockLapisOre,
		Name:         "Lapis Ore",
		Textures:     blockFaces{Top: "textures/block/lapis_ore.png", Bottom: "textures/block/lapis_ore.png", North: "textures/block/lapis_ore.png", South: "textures/block/lapis_ore.png", East: "textures/block/lapis_ore.png", West: "textures/block/lapis_ore.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockBedrock,
		Name:         "Bedrock",
		Textures:     blockFaces{Top: "textures/block/bedrock.png", Bottom: "textures/block/bedrock.png", North: "textures/block/bedrock.png", South: "textures/block/bedrock.png", East: "textures/block/bedrock.png", West: "textures/block/bedrock.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockLog,
		Name:         "Oak Log",
		Textures:     blockFaces{Top: "textures/block/oak_log_top.png", Bottom: "textures/block/oak_log_top.png", North: "textures/block/oak_log.png", South: "textures/block/oak_log.png", East: "textures/block/oak_log.png", West: "textures/block/oak_log.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockPlank,
		Name:         "Oak Planks",
		Textures:     blockFaces{Top: "textures/block/oak_planks.png", Bottom: "textures/block/oak_planks.png", North: "textures/block/oak_planks.png", South: "textures/block/oak_planks.png", East: "textures/block/oak_planks.png", West: "textures/block/oak_planks.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockGlass,
		Name:          "Glass",
		Textures:      blockFaces{Top: "textures/block/glass.png", Bottom: "textures/block/glass.png", North: "textures/block/glass.png", South: "textures/block/glass.png", East: "textures/block/glass.png", West: "textures/block/glass.png"},
		RenderType:    RenderTypeGlass,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockGlowstone,
		Name:         "Glowstone",
		Textures:     blockFaces{Top: "textures/block/glowstone.png", Bottom: "textures/block/glowstone.png", North: "textures/block/glowstone.png", South: "textures/block/glowstone.png", East: "textures/block/glowstone.png", West: "textures/block/glowstone.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		LightLevel:   15,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockCraftingTable,
		Name:         "Crafting Table",
		Textures:     blockFaces{Top: "textures/block/crafting_table_top.png", Bottom: "textures/block/oak_planks.png", North: "textures/block/crafting_table_front.png", South: "textures/block/crafting_table_side.png", East: "textures/block/crafting_table_side.png", West: "textures/block/crafting_table_side.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockObsidian,
		Name:         "Obsidian",
		Textures:     blockFaces{Top: "textures/block/obsidian.png", Bottom: "textures/block/obsidian.png", North: "textures/block/obsidian.png", South: "textures/block/obsidian.png", East: "textures/block/obsidian.png", West: "textures/block/obsidian.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockDiamondBlock,
		Name:         "Diamond Block",
		Textures:     blockFaces{Top: "textures/block/diamond_block.png", Bottom: "textures/block/diamond_block.png", North: "textures/block/diamond_block.png", South: "textures/block/diamond_block.png", East: "textures/block/diamond_block.png", West: "textures/block/diamond_block.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockGoldBlock,
		Name:         "Gold Block",
		Textures:     blockFaces{Top: "textures/block/gold_block.png", Bottom: "textures/block/gold_block.png", North: "textures/block/gold_block.png", South: "textures/block/gold_block.png", East: "textures/block/gold_block.png", West: "textures/block/gold_block.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockIronBlock,
		Name:         "Iron Block",
		Textures:     blockFaces{Top: "textures/block/iron_block.png", Bottom: "textures/block/iron_block.png", North: "textures/block/iron_block.png", South: "textures/block/iron_block.png", East: "textures/block/iron_block.png", West: "textures/block/iron_block.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockCoalBlock,
		Name:         "Coal Block",
		Textures:     blockFaces{Top: "textures/block/coal_block.png", Bottom: "textures/block/coal_block.png", North: "textures/block/coal_block.png", South: "textures/block/coal_block.png", East: "textures/block/coal_block.png", West: "textures/block/coal_block.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockLogBirch,
		Name:         "Birch Log",
		Textures:     blockFaces{Top: "textures/block/birch_log_top.png", Bottom: "textures/block/birch_log_top.png", North: "textures/block/birch_log.png", South: "textures/block/birch_log.png", East: "textures/block/birch_log.png", West: "textures/block/birch_log.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockLogSpruce,
		Name:         "Spruce Log",
		Textures:     blockFaces{Top: "textures/block/spruce_log_top.png", Bottom: "textures/block/spruce_log_top.png", North: "textures/block/spruce_log.png", South: "textures/block/spruce_log.png", East: "textures/block/spruce_log.png", West: "textures/block/spruce_log.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockLeavesBirch,
		Name:          "Birch Leaves",
		Textures:      blockFaces{Top: "textures/block/birch_leaves.png", Bottom: "textures/block/birch_leaves.png", North: "textures/block/birch_leaves.png", South: "textures/block/birch_leaves.png", East: "textures/block/birch_leaves.png", West: "textures/block/birch_leaves.png"},
		RenderType:    RenderTypeCutout,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockLeavesSpruce,
		Name:          "Spruce Leaves",
		Textures:      blockFaces{Top: "textures/block/spruce_leaves.png", Bottom: "textures/block/spruce_leaves.png", North: "textures/block/spruce_leaves.png", South: "textures/block/spruce_leaves.png", East: "textures/block/spruce_leaves.png", West: "textures/block/spruce_leaves.png"},
		RenderType:    RenderTypeCutout,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockPlankOak,
		Name:         "Oak Planks",
		Textures:     blockFaces{Top: "textures/block/oak_planks.png", Bottom: "textures/block/oak_planks.png", North: "textures/block/oak_planks.png", South: "textures/block/oak_planks.png", East: "textures/block/oak_planks.png", West: "textures/block/oak_planks.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockPlankBirch,
		Name:         "Birch Planks",
		Textures:     blockFaces{Top: "textures/block/birch_planks.png", Bottom: "textures/block/birch_planks.png", North: "textures/block/birch_planks.png", South: "textures/block/birch_planks.png", East: "textures/block/birch_planks.png", West: "textures/block/birch_planks.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockPlankSpruce,
		Name:         "Spruce Planks",
		Textures:     blockFaces{Top: "textures/block/spruce_planks.png", Bottom: "textures/block/spruce_planks.png", North: "textures/block/spruce_planks.png", South: "textures/block/spruce_planks.png", East: "textures/block/spruce_planks.png", West: "textures/block/spruce_planks.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockSandstone,
		Name:         "Sandstone",
		Textures:     blockFaces{Top: "textures/block/sandstone_top.png", Bottom: "textures/block/sandstone_bottom.png", North: "textures/block/sandstone.png", South: "textures/block/sandstone.png", East: "textures/block/sandstone.png", West: "textures/block/sandstone.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:           blockSnow,
		Name:         "Snow Block",
		Textures:     blockFaces{Top: "textures/block/snow.png", Bottom: "textures/block/snow.png", North: "textures/block/snow.png", South: "textures/block/snow.png", East: "textures/block/snow.png", West: "textures/block/snow.png"},
		RenderType:   RenderTypeCube,
		IsOpaque:     true,
		IsCollidable: true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockIce,
		Name:          "Ice",
		Textures:      blockFaces{Top: "textures/block/ice.png", Bottom: "textures/block/ice.png", North: "textures/block/ice.png", South: "textures/block/ice.png", East: "textures/block/ice.png", West: "textures/block/ice.png"},
		RenderType:    RenderTypeGlass,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockCactus,
		Name:          "Cactus",
		Textures:      blockFaces{Top: "textures/block/cactus_top.png", Bottom: "textures/block/cactus_bottom.png", North: "textures/block/cactus_side.png", South: "textures/block/cactus_side.png", East: "textures/block/cactus_side.png", West: "textures/block/cactus_side.png"},
		RenderType:    RenderTypeCube, // Custom rendering handled in render_mesh.go
		IsOpaque:      false,
		IsTransparent: true,
		IsCollidable:  true,
	})
	RegisterBlock(&BlockDef{
		ID:            blockRose,
		Name:          "Rose",
		Textures:      blockFaces{Top: "textures/block/poppy.png", Bottom: "textures/block/poppy.png", North: "textures/block/poppy.png", South: "textures/block/poppy.png", East: "textures/block/poppy.png", West: "textures/block/poppy.png"},
		RenderType:    RenderTypeCross,
		IsTransparent: true,
		IsCollidable:  false,
	})
	RegisterBlock(&BlockDef{
		ID:            blockDandelion,
		Name:          "Dandelion",
		Textures:      blockFaces{Top: "textures/block/dandelion.png", Bottom: "textures/block/dandelion.png", North: "textures/block/dandelion.png", South: "textures/block/dandelion.png", East: "textures/block/dandelion.png", West: "textures/block/dandelion.png"},
		RenderType:    RenderTypeCross,
		IsTransparent: true,
		IsCollidable:  false,
	})
	RegisterBlock(&BlockDef{
		ID:            blockDeadBush,
		Name:          "Dead Bush",
		Textures:      blockFaces{Top: "textures/block/dead_bush.png", Bottom: "textures/block/dead_bush.png", North: "textures/block/dead_bush.png", South: "textures/block/dead_bush.png", East: "textures/block/dead_bush.png", West: "textures/block/dead_bush.png"},
		RenderType:    RenderTypeCross,
		IsTransparent: true,
		IsCollidable:  false,
	})
	RegisterBlock(&BlockDef{
		ID:            blockTallGrass,
		Name:          "Tall Grass",
		Textures:      blockFaces{Top: "textures/block/short_grass.png", Bottom: "textures/block/short_grass.png", North: "textures/block/short_grass.png", South: "textures/block/short_grass.png", East: "textures/block/short_grass.png", West: "textures/block/short_grass.png"},
		RenderType:    RenderTypeCross,
		IsTransparent: true,
		IsCollidable:  false,
	})

	RegisterBlock(&BlockDef{ID: blockChest, Name: "Chest", Textures: blockFaces{Top: "textures/block/barrel_top.png", Bottom: "textures/block/barrel_bottom.png", North: "textures/block/barrel_top.png", South: "textures/block/barrel_side.png", East: "textures/block/barrel_side.png", West: "textures/block/barrel_side.png"}, IsOpaque: true, IsCollidable: true})
	RegisterBlock(&BlockDef{ID: blockFurnace, Name: "Furnace", Textures: blockFaces{Top: "textures/block/furnace_top.png", Bottom: "textures/block/furnace_top.png", North: "textures/block/furnace_front.png", South: "textures/block/furnace_side.png", East: "textures/block/furnace_side.png", West: "textures/block/furnace_side.png"}, IsOpaque: true, IsCollidable: true})
	initItemRegistry()
}
