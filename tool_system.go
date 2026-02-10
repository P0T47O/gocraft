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

// GetMiningSpeedMultiplier calculates the mining speed based on tool and block properties
func GetMiningSpeedMultiplier(toolID byte, blockID byte) float32 {
	if toolID == 0 {
		return 1.0
	}

	toolDef, ok := ToolRegistry[toolID]
	if !ok {
		return 1.0
	}

	blockDef := GetBlock(blockID)
	// If effective tool matches, use tool material speed
	if blockDef.EffectiveTool != ToolNone && blockDef.EffectiveTool == toolDef.Type {
		switch toolDef.Material {
		case MatWood:
			return 2.0
		case MatStone:
			return 4.0
		case MatIron:
			return 6.0
		case MatDiamond:
			return 8.0
		case MatGold:
			return 12.0
		}
	}

	return 1.0
}

// CanHarvest returns true if the tool can harvest the block (drop item)
// In MC this is separate from speed (e.g. Glass breaks fast by hand but drops nothing)
// But for "Effective Tool" check (1.5x vs 5x factor), we usually check if it's the "Correct Tool".
func IsCorrectTool(toolID byte, blockID byte) bool {
	blockDef := GetBlock(blockID)
	// If no tool required/effective, then Hand is "correct" (factor 1.5)
	if blockDef.EffectiveTool == ToolNone {
		return true
	}

	toolDef, ok := ToolRegistry[toolID]
	if !ok {
		return false
	}
	return toolDef.Type == blockDef.EffectiveTool
}
