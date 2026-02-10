package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	chunkWidth    = 16
	chunkHeight   = 256
	sectionHeight = 16
	sectionCount  = chunkHeight / sectionHeight
)

const (
	blockAir byte = iota
	blockGrass
	blockStone
	blockDirt
	blockCobblestone
	blockSand
	blockLeaves
	blockWater
	blockLava
	blockGravel
	blockTorch
	blockCoalOre
	blockIronOre
	blockGoldOre
	blockDiamondOre
	blockLapisOre
	blockBedrock
	blockLog
	blockPlank
	blockGlass
	blockGlowstone
	blockObsidian
	blockDiamondBlock
	blockGoldBlock
	blockIronBlock
	blockCoalBlock
	blockLogBirch
	blockLogSpruce
	blockLeavesBirch
	blockLeavesSpruce
	blockPlankOak
	blockPlankBirch
	blockPlankSpruce
	blockSandstone
	blockCactus
	blockDeadBush
	blockSnow
	blockIce
	blockRose
	blockDandelion
	blockTallGrass
	blockCraftingTable
)

const (
	// Tools (Start at 100 to avoid block collision)
	itemWoodPickaxe byte = 100 + iota
	itemStonePickaxe
	itemIronPickaxe
	itemDiamondPickaxe
	itemGoldPickaxe

	itemWoodShovel
	itemStoneShovel
	itemIronShovel
	itemDiamondShovel
	itemGoldShovel

	itemWoodAxe
	itemStoneAxe
	itemIronAxe
	itemDiamondAxe
	itemGoldAxe

	// Resources
	itemCoal
	itemIronIngot
	itemGoldIngot
	itemDiamond
	itemStick
)

type hitInfo struct {
	x        int
	y        int
	z        int
	normal   rl.Vector3
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

type faceMesh struct {
	mesh      rl.Mesh
	vertices  []float32
	normals   []float32
	texcoords []float32
	indices   []uint16
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

func findHit(blocks *[chunkWidth][chunkHeight][chunkWidth]byte, ray rl.Ray) hitInfo {
	best := hitInfo{distance: float32(math.MaxFloat32)}
	for x := 0; x < chunkWidth; x++ {
		for y := 0; y < chunkHeight; y++ {
			for z := 0; z < chunkWidth; z++ {
				if blocks[x][y][z] == blockAir {
					continue
				}
				min := rl.NewVector3(float32(x)-0.5, float32(y)-0.5, float32(z)-0.5)
				max := rl.NewVector3(float32(x)+0.5, float32(y)+0.5, float32(z)+0.5)
				collision := rl.GetRayCollisionBox(ray, rl.BoundingBox{Min: min, Max: max})
				if collision.Hit && collision.Distance < best.distance {
					best = hitInfo{
						x:        x,
						y:        y,
						z:        z,
						normal:   collision.Normal,
						distance: collision.Distance,
						hit:      true,
					}
				}
			}
		}
	}
	return best
}
