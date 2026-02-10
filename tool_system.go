package main

type ToolType int

const (
	ToolNone ToolType = iota
	ToolPickaxe
	ToolShovel
	ToolAxe
)

type ToolMaterial int

const (
	MatNone ToolMaterial = iota
	MatWood
	MatStone
	MatIron
	MatDiamond
	MatGold
)

// ToolDef defines a tool's properties
type ToolDef struct {
	Type     ToolType
	Material ToolMaterial
}

var ToolRegistry = map[byte]ToolDef{
	itemWoodPickaxe:    {ToolPickaxe, MatWood},
	itemStonePickaxe:   {ToolPickaxe, MatStone},
	itemIronPickaxe:    {ToolPickaxe, MatIron},
	itemDiamondPickaxe: {ToolPickaxe, MatDiamond},
	itemGoldPickaxe:    {ToolPickaxe, MatGold},

	itemWoodShovel:    {ToolShovel, MatWood},
	itemStoneShovel:   {ToolShovel, MatStone},
	itemIronShovel:    {ToolShovel, MatIron},
	itemDiamondShovel: {ToolShovel, MatDiamond},
	itemGoldShovel:    {ToolShovel, MatGold},

	itemWoodAxe:    {ToolAxe, MatWood},
	itemStoneAxe:   {ToolAxe, MatStone},
	itemIronAxe:    {ToolAxe, MatIron},
	itemDiamondAxe: {ToolAxe, MatDiamond},
	itemGoldAxe:    {ToolAxe, MatGold},
}

// BlockToolReq maps blocks to their best tool
var BlockToolReq = map[byte]ToolType{
	blockStone:        ToolPickaxe,
	blockCobblestone:  ToolPickaxe,
	blockCoalOre:      ToolPickaxe,
	blockIronOre:      ToolPickaxe,
	blockGoldOre:      ToolPickaxe,
	blockDiamondOre:   ToolPickaxe,
	blockLapisOre:     ToolPickaxe,
	blockIronBlock:    ToolPickaxe,
	blockGoldBlock:    ToolPickaxe,
	blockDiamondBlock: ToolPickaxe,
	blockObsidian:     ToolPickaxe,
	blockSandstone:    ToolPickaxe,
	blockIce:          ToolPickaxe, // Actually pickaxe breaks ice fast but doesn't drop it without Silk Touch

	blockDirt:   ToolShovel,
	blockGrass:  ToolShovel,
	blockSand:   ToolShovel,
	blockGravel: ToolShovel,
	blockSnow:   ToolShovel,

	blockLog:         ToolAxe,
	blockLogBirch:    ToolAxe,
	blockLogSpruce:   ToolAxe,
	blockPlank:       ToolAxe,
	blockPlankOak:    ToolAxe,
	blockPlankBirch:  ToolAxe,
	blockPlankSpruce: ToolAxe,
}

// GetMiningSpeedMultiplier returns the speed multiplier for the given tool and block
func GetMiningSpeedMultiplier(toolID byte, blockID byte) float32 {
	tool, isTool := ToolRegistry[toolID]
	reqTool, needsTool := BlockToolReq[blockID]

	// 1. Check if tool matches block requirement
	isBestTool := false
	if needsTool {
		if isTool && tool.Type == reqTool {
			isBestTool = true
		}
	} else {
		// If block doesn't require a tool (e.g. leaves, glass), check if tool helps
		// For now simple logic: usually no tool helps unless specific cases (shears for leaves)
		// But in MC, Axes help with wood-based things even if not strictly required
		// For simplicity, let's say NO multiplier unless mapped.
	}

	// 2. Determine Material Multiplier
	multiplier := float32(1.0) // Hand speed

	if isBestTool {
		switch tool.Material {
		case MatWood:
			multiplier = 2.0
		case MatStone:
			multiplier = 4.0
		case MatIron:
			multiplier = 6.0
		case MatDiamond:
			multiplier = 8.0
		case MatGold:
			multiplier = 12.0
		}
	}

	return multiplier
}

// CanHarvest returns true if the tool can harvest the block (drop item)
// For now we always drop items in this engine, but this affects speed calculation in MC physics
func CanHarvest(toolID byte, blockID byte) bool {
	// Simple logic for now: all tools can harvest everything
	// In real MC, you need Iron Pickaxe for Diamond Ore, etc.
	// Implementing Tier check:

	tool, isTool := ToolRegistry[toolID]

	if blockID == blockObsidian {
		return isTool && tool.Type == ToolPickaxe && tool.Material == MatDiamond
	}
	if blockID == blockDiamondOre || blockID == blockGoldOre {
		return isTool && tool.Type == ToolPickaxe && (tool.Material == MatIron || tool.Material == MatDiamond)
	}
	if blockID == blockIronOre || blockID == blockLapisOre {
		return isTool && tool.Type == ToolPickaxe && (tool.Material == MatStone || tool.Material == MatIron || tool.Material == MatDiamond)
	}
	if blockID == blockStone || blockID == blockCobblestone || blockID == blockCoalOre {
		return isTool && tool.Type == ToolPickaxe // Wooden or better
	}

	// Most other blocks can be harvested by hand
	return true
}
