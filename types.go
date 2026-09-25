package main

const (
	chunkWidth    = 16
	chunkHeight   = 256
	sectionHeight = 16
	sectionCount  = chunkHeight / sectionHeight
)

// Block IDs are persisted in chunk saves and sent over the network. Keep these
// values explicit: inserting or reordering constants must never renumber old
// content.
const (
	blockAir           byte = 0
	blockGrass         byte = 1
	blockStone         byte = 2
	blockDirt          byte = 3
	blockCobblestone   byte = 4
	blockSand          byte = 5
	blockLeaves        byte = 6
	blockWater         byte = 7
	blockLava          byte = 8
	blockGravel        byte = 9
	blockTorch         byte = 10
	blockCoalOre       byte = 11
	blockIronOre       byte = 12
	blockGoldOre       byte = 13
	blockDiamondOre    byte = 14
	blockLapisOre      byte = 15
	blockBedrock       byte = 16
	blockLog           byte = 17
	blockPlank         byte = 18
	blockGlass         byte = 19
	blockGlowstone     byte = 20
	blockObsidian      byte = 21
	blockDiamondBlock  byte = 22
	blockGoldBlock     byte = 23
	blockIronBlock     byte = 24
	blockCoalBlock     byte = 25
	blockLogBirch      byte = 26
	blockLogSpruce     byte = 27
	blockLeavesBirch   byte = 28
	blockLeavesSpruce  byte = 29
	blockPlankOak      byte = 30
	blockPlankBirch    byte = 31
	blockPlankSpruce   byte = 32
	blockSandstone     byte = 33
	blockCactus        byte = 34
	blockDeadBush      byte = 35
	blockSnow          byte = 36
	blockIce           byte = 37
	blockRose          byte = 38
	blockDandelion     byte = 39
	blockTallGrass     byte = 40
	blockCraftingTable byte = 41
	blockChest         byte = 42
	blockFurnace       byte = 43
	blockTNT           byte = 44
	blockFarmland      byte = 45
	blockWheatCrop     byte = 46
	blockPotatoCrop    byte = 47
	blockCarrotCrop    byte = 48
)

// Item IDs share the byte namespace with blocks and are also persisted and
// transmitted. 100+ is reserved for non-block items; keep every assignment
// explicit for save/protocol compatibility.
const (
	itemWoodPickaxe    byte = 100
	itemStonePickaxe   byte = 101
	itemIronPickaxe    byte = 102
	itemDiamondPickaxe byte = 103
	itemGoldPickaxe    byte = 104

	itemWoodShovel    byte = 105
	itemStoneShovel   byte = 106
	itemIronShovel    byte = 107
	itemDiamondShovel byte = 108
	itemGoldShovel    byte = 109

	itemWoodAxe    byte = 110
	itemStoneAxe   byte = 111
	itemIronAxe    byte = 112
	itemDiamondAxe byte = 113
	itemGoldAxe    byte = 114

	itemCoal        byte = 115
	itemIronIngot   byte = 116
	itemGoldIngot   byte = 117
	itemDiamond     byte = 118
	itemStick       byte = 119
	itemRawPork     byte = 120
	itemCookedPork  byte = 121
	itemRottenFlesh byte = 122
	itemBone        byte = 123
	itemArrow       byte = 124
	itemString      byte = 125
	itemGunpowder   byte = 126
	itemWheatSeeds  byte = 127
	itemWheat       byte = 128
	itemBread       byte = 129
	itemPotato      byte = 130
	itemCarrot      byte = 131
	itemWoodHoe     byte = 132
	itemStoneHoe    byte = 133
	itemIronHoe     byte = 134
	itemDiamondHoe  byte = 135
	itemGoldHoe     byte = 136
	itemBakedPotato byte = 137
)

type hitInfo struct {
	x      int
	y      int
	z      int
	normal struct {
		X, Y, Z float32
	}
	distance float32
	hit      bool
}

type blockFaces struct {
	Top    string
	Bottom string
	North  string
	South  string
	East   string
	West   string
}

func inBounds(x, y, z int) bool {
	return x >= 0 && x < chunkWidth &&
		y >= 0 && y < chunkHeight &&
		z >= 0 && z < chunkWidth
}

func divFloor(v, d int) int {
	if d == 0 {
		return 0
	}
	if v >= 0 {
		return v / d
	}
	return -(((-v) + d - 1) / d)
}

func modFloor(v, d int) int {
	if d == 0 {
		return 0
	}
	m := v % d
	if m < 0 {
		m += d
	}
	return m
}
