package main

// Redundant blockFacesByType removed. Managed in block_registry.go.

var allBlocks = []byte{
	blockGrass,
	blockDirt,
	blockStone,
	blockCobblestone,
	blockSand,
	blockSandstone,
	blockGravel,
	blockSnow,
	blockIce,

	// Wood
	blockLog,
	blockLogBirch,
	blockLogSpruce,
	blockLeaves,
	blockLeavesBirch,
	blockLeavesSpruce,
	blockPlank,
	blockPlankOak,
	blockPlankBirch,
	blockPlankSpruce,

	// Vegetation
	blockCactus,
	blockDeadBush,
	blockRose,
	blockDandelion,
	blockTallGrass,

	// Misc
	blockGlass,
	blockTorch,
	blockBedrock,

	// Liquids
	blockWater,
	blockLava,

	// Ores
	blockCoalOre,
	blockIronOre,
	blockGoldOre,
	blockDiamondOre,
	blockLapisOre,

	// Special Blocks
	blockGlowstone,
	blockObsidian,
	blockDiamondBlock,
	blockGoldBlock,
	blockIronBlock,
	blockCoalBlock,
}
