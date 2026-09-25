package main

// Mining profiles are applied once at registration. Runtime lookup remains O(1).
// Every physical block must appear here, including intentionally instant blocks.
func configureBasicMining() {
	set := func(h float32, tool ToolType, tier ToolMaterial, noDrop bool, ids ...byte) {
		for _, id := range ids {
			d := GetBlock(id)
			d.Hardness, d.EffectiveTool, d.RequiredMaterial, d.NoDrop = h, tool, tier, noDrop
			d.MiningConfigured = true
		}
	}
	set(2, ToolAxe, MatNone, false, blockLog, blockLogBirch, blockLogSpruce, blockPlank, blockPlankOak, blockPlankBirch, blockPlankSpruce)
	set(2.5, ToolAxe, MatNone, false, blockCraftingTable, blockChest)
	set(.5, ToolShovel, MatNone, false, blockDirt, blockSand, blockGravel)
	set(.6, ToolShovel, MatNone, false, blockGrass)
	set(.2, ToolShovel, MatWood, false, blockSnow)
	set(1.5, ToolPickaxe, MatWood, false, blockStone)
	set(2, ToolPickaxe, MatWood, false, blockCobblestone)
	set(3.5, ToolPickaxe, MatWood, false, blockFurnace)
	set(0, ToolNone, MatNone, false, blockTNT)
	set(.8, ToolNone, MatNone, false, blockWhiteWool, blockBed)
	set(.6, ToolShovel, MatNone, false, blockFarmland)
	set(0, ToolNone, MatNone, true, blockWheatCrop, blockPotatoCrop, blockCarrotCrop)
	set(.8, ToolPickaxe, MatWood, false, blockSandstone)
	set(3, ToolPickaxe, MatWood, false, blockCoalOre)
	set(3, ToolPickaxe, MatStone, false, blockIronOre, blockLapisOre)
	set(3, ToolPickaxe, MatIron, false, blockGoldOre, blockDiamondOre)
	set(5, ToolPickaxe, MatWood, false, blockCoalBlock)
	set(5, ToolPickaxe, MatStone, false, blockIronBlock)
	set(3, ToolPickaxe, MatIron, false, blockGoldBlock)
	set(5, ToolPickaxe, MatIron, false, blockDiamondBlock)
	set(50, ToolPickaxe, MatDiamond, false, blockObsidian)
	set(.3, ToolNone, MatNone, true, blockGlass)
	set(.5, ToolPickaxe, MatNone, true, blockIce)
	set(.2, ToolNone, MatNone, true, blockLeaves, blockLeavesBirch, blockLeavesSpruce)
	set(.3, ToolNone, MatNone, false, blockGlowstone)
	set(.4, ToolNone, MatNone, false, blockCactus)
	set(0, ToolNone, MatNone, false, blockTorch, blockRose, blockDandelion, blockDeadBush)
	set(0, ToolNone, MatNone, true, blockTallGrass)
	set(-1, ToolNone, MatNone, true, blockBedrock, blockWater, blockLava)
	GetBlock(blockGrass).DropItem = blockDirt
	GetBlock(blockStone).DropItem = blockCobblestone
	GetBlock(blockCoalOre).DropItem = itemCoal
	GetBlock(blockDiamondOre).DropItem = itemDiamond
	GetBlock(blockDeadBush).DropItem = itemStick
	GetBlock(blockDeadBush).DropCount = 2
	for _, d := range Blocks {
		if d != nil && d.ID != blockAir && d.ID < itemWoodPickaxe && !d.MiningConfigured {
			panic("missing mining profile: " + d.Name)
		}
	}
}
