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

// Tool properties live only in the block/item registry.
func GetMiningSpeedMultiplier(toolID byte, blockID byte) float32 {
	tool, block := GetItem(toolID), GetBlock(blockID)
	if block.EffectiveTool == ToolNone || tool.ToolType != block.EffectiveTool {
		return 1
	}
	switch tool.ToolMaterial {
	case MatWood:
		return 2
	case MatStone:
		return 4
	case MatIron:
		return 6
	case MatDiamond:
		return 8
	case MatGold:
		return 12
	}
	return 1
}

// CanHarvest is independent of whether the block has an ordinary drop.
func CanHarvest(toolID, blockID byte) bool {
	block, tool := GetBlock(blockID), GetItem(toolID)
	return block.RequiredMaterial == MatNone ||
		(block.EffectiveTool == tool.ToolType && harvestTier(tool.ToolMaterial) >= harvestTier(block.RequiredMaterial))
}

// MiningSeconds returns -1 for unbreakable blocks and 0 for instant blocks.
func MiningSeconds(toolID, blockID byte) float32 {
	d := GetBlock(blockID)
	if d.Hardness <= 0 {
		return d.Hardness
	}
	factor := float32(5)
	if CanHarvest(toolID, blockID) {
		factor = 1.5
	}
	return d.Hardness * factor / GetMiningSpeedMultiplier(toolID, blockID)
}
