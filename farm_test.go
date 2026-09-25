package main

import (
	"testing"
	"time"
)

func TestFarmTillPlantGrowHarvestAndSave(t *testing.T) {
	s, p := hostileTestServer(t)
	initItemRegistry()
	c := s.World.getChunkIfGenerated(0, 0)
	c.mu.Lock()
	c.skyLight.Fill(15)
	c.mu.Unlock()
	s.World.SetBlockAt(8, 70, 8, blockDirt)
	s.World.SetBlockAt(9, 70, 8, blockWater)
	p.SelectedSlot = 0
	p.Inventory.Slots[0] = Item{ID: int32(itemWoodHoe), Count: 1}
	if !s.interactFarm(p, BlockPos{8, 70, 8}) || s.World.BlockAt(8, 70, 8) != blockFarmland || s.World.MetaAt(8, 70, 8) != 1 {
		t.Fatal("hoe did not make moist farmland")
	}
	if p.Inventory.Slots[0].Damage != 1 {
		t.Fatal("hoe did not wear")
	}
	p.SelectedSlot = 1
	p.Inventory.Slots[1] = Item{ID: int32(itemWheatSeeds), Count: 2}
	if !s.interactFarm(p, BlockPos{8, 70, 8}) || s.World.BlockAt(8, 71, 8) != blockWheatCrop || p.Inventory.Slots[1].Count != 1 {
		t.Fatal("wheat planting did not consume a seed")
	}
	for i := 0; i < 500 && s.World.MetaAt(8, 71, 8) < 7; i++ {
		s.growCropAt(8, 71, 8)
	}
	if s.World.MetaAt(8, 71, 8) != 7 {
		t.Fatal("wheat did not mature")
	}
	root := t.TempDir()
	if err := SaveChunk(root, c, 0, 0); err != nil {
		t.Fatal(err)
	}
	loaded := &Chunk{}
	if !TryLoadChunk(root, loaded, 0, 0) || loaded.meta.Get(8, 71, 8) != 7 || loaded.blocks.Get(8, 71, 8) != blockWheatCrop {
		t.Fatal("mature crop stage was not saved")
	}
	s.dropCrop(8, 71, 8, blockWheatCrop, 7)
	wheat, seeds := false, false
	for _, e := range s.World.entities {
		if item, ok := e.(*ItemEntity); ok {
			wheat = wheat || item.ID == int32(itemWheat)
			seeds = seeds || item.ID == int32(itemWheatSeeds)
		}
	}
	if !wheat || !seeds {
		t.Fatal("mature wheat did not yield grain and seeds")
	}
}

func TestPotatoCarrotFoodAndRecipes(t *testing.T) {
	initBlockRegistry()
	initItemRegistry()
	InitRecipes()
	for _, item := range []byte{itemPotato, itemCarrot} {
		food, _ := foodValue(int32(item))
		if cropForItem(item) == blockAir || food == 0 {
			t.Fatalf("crop %d cannot be planted/eaten", item)
		}
	}
	if smeltResult(int32(itemPotato)) != int32(itemBakedPotato) {
		t.Fatal("potato cannot be baked")
	}
	if RecipeRegistry[47].Result.ID != int32(itemBread) || RecipeRegistry[42].Result.ID != int32(itemWoodHoe) {
		t.Fatal("farm recipes missing")
	}
	if GetItem(itemWheatSeeds).ID == 0 || GetBlock(blockFarmland).ID == 0 {
		t.Fatal("farm content not registered")
	}
}

func TestCoveredFarmColumnStillGrows(t *testing.T) {
	s, _ := hostileTestServer(t)
	s.World.SetBlockAt(8, 60, 8, blockFarmland)
	s.World.SetBlockAt(8, 61, 8, blockCarrotCrop)
	s.World.SetBlockAt(8, 70, 8, blockStone)
	s.World.SetBlockAt(9, 60, 8, blockWater)
	c := s.World.getChunkIfGenerated(0, 0)
	c.mu.Lock()
	c.blockLight.Set(8, 61, 8, 15)
	c.mu.Unlock()
	for i := 0; i < 100 && s.World.MetaAt(8, 61, 8) == 0; i++ {
		s.sampleFarmColumn(8, 8)
	}
	if s.World.MetaAt(8, 61, 8) == 0 {
		t.Fatal("covered farm column was ignored")
	}
}

func TestFarmBreakDropsCropAndRejectsDirectCropPlacement(t *testing.T) {
	s, p := hostileTestServer(t)
	initItemRegistry()
	p.GameMode = ModeCreative
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockWheatCrop}})
	if s.World.BlockAt(8, 71, 8) != blockAir {
		t.Fatal("client placed crop without farmland or seeds")
	}
	p.GameMode = ModeSurvival
	s.World.SetBlockAt(8, 70, 8, blockFarmland)
	s.World.SetBlockAt(8, 71, 8, blockWheatCrop)
	s.World.SetMetaAt(8, 71, 8, 7)
	pos := BlockPos{8, 70, 8}
	s.beginMining(p, pos, time.Now().Add(-5*time.Second))
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: pos.X, Y: pos.Y, Z: pos.Z, BlockID: blockAir}})
	if s.World.BlockAt(8, 70, 8) != blockAir || s.World.BlockAt(8, 71, 8) != blockAir {
		t.Fatal("broken farmland left floating crop")
	}
	wheat := false
	for _, e := range s.World.entities {
		if item, ok := e.(*ItemEntity); ok && item.ID == int32(itemWheat) {
			wheat = true
		}
	}
	if !wheat {
		t.Fatal("crop on broken farmland lost harvest")
	}
}

func TestCropMeshUsesSavedGrowthStage(t *testing.T) {
	initBlockRegistry()
	stage0, stage7 := cropTexture(blockWheatCrop, 0), cropTexture(blockWheatCrop, 7)
	rect := AtlasRect{X: .5, Y: .25, Width: .0625, Height: .0625}
	a := &RenderAssets{atlas: &TextureAtlas{UVs: map[string]AtlasRect{stage0: {X: .1, Y: .1, Width: .0625, Height: .0625}, stage7: rect}}}
	var heights [chunkWidth][chunkWidth]int16
	data := a.buildAllMeshData(&heights, 0, 0, 8, 9,
		func(x, y, z int) byte {
			if x == 8 && y == 8 && z == 8 {
				return blockWheatCrop
			}
			return blockAir
		},
		func(x, y, z int) byte { return 15 },
		func(x, y, z int) byte { return 7 }, 42)
	defer releaseMeshResults(data)
	if len(data["cutout"]["atlas"]) != 1 {
		t.Fatal("crop was not rendered as cutout atlas geometry")
	}
	for i := 0; i < len(data["cutout"]["atlas"][0].texcoords); i += 2 {
		u, v := data["cutout"]["atlas"][0].texcoords[i], data["cutout"]["atlas"][0].texcoords[i+1]
		if u < rect.X-1e-6 || u > rect.X+rect.Width+1e-6 || v < rect.Y-1e-6 || v > rect.Y+rect.Height+1e-6 {
			t.Fatalf("mature crop used wrong atlas stage: %v,%v", u, v)
		}
	}
}
