package main

import (
	"fmt"
	"math/rand"
	"time"
)

func isCrop(block byte) bool {
	return block == blockWheatCrop || block == blockPotatoCrop || block == blockCarrotCrop
}

func cropForItem(item byte) byte {
	switch item {
	case itemWheatSeeds:
		return blockWheatCrop
	case itemPotato:
		return blockPotatoCrop
	case itemCarrot:
		return blockCarrotCrop
	}
	return blockAir
}

func cropMaxStage(block byte) byte {
	if block == blockWheatCrop {
		return 7
	}
	if block == blockPotatoCrop || block == blockCarrotCrop {
		return 3
	}
	return 0
}

func cropTexture(block, stage byte) string {
	switch block {
	case blockWheatCrop:
		return fmt.Sprintf("textures/block/wheat_stage%d.png", min(stage, byte(7)))
	case blockPotatoCrop:
		return fmt.Sprintf("textures/block/potatoes_stage%d.png", min(stage, byte(3)))
	case blockCarrotCrop:
		return fmt.Sprintf("textures/block/carrots_stage%d.png", min(stage, byte(3)))
	}
	return ""
}

func (s *Server) setFarmBlock(x, y, z int, block, meta byte) {
	s.World.SetBlockAt(x, y, z, block)
	s.World.SetMetaAt(x, y, z, meta)
	s.Broadcast(&PacketBlockChange{X: int32(x), Y: int32(y), Z: int32(z), BlockID: block, Meta: meta})
}

func (s *Server) nearbyFarmWater(x, y, z int) bool {
	for dx := -4; dx <= 4; dx++ {
		for dz := -4; dz <= 4; dz++ {
			if s.World.BlockAt(x+dx, y, z+dz) == blockWater || s.World.BlockAt(x+dx, y+1, z+dz) == blockWater {
				return true
			}
		}
	}
	return false
}

// Reach has already been checked by HandlePacket. The selected server-side
// inventory slot, not a client-supplied item ID, authorizes both actions.
func (s *Server) interactFarm(p *PlayerEntity, pos BlockPos) bool {
	if pos.Y < 0 || pos.Y >= chunkHeight-1 || p.SelectedSlot < 0 || p.SelectedSlot >= 9 {
		return false
	}
	x, y, z := int(pos.X), int(pos.Y), int(pos.Z)
	block := s.World.BlockAt(x, y, z)
	slot := &p.Inventory.Slots[p.SelectedSlot]
	if slot.Count <= 0 || slot.ID <= 0 || slot.ID > 255 {
		return false
	}
	held := byte(slot.ID)
	if (block == blockDirt || block == blockGrass) && GetItem(held).ToolType == ToolHoe {
		if s.World.BlockAt(x, y+1, z) != blockAir {
			return true
		}
		moist := byte(0)
		if s.nearbyFarmWater(x, y, z) {
			moist = 1
		}
		s.setFarmBlock(x, y, z, blockFarmland, moist)
		if p.GameMode == ModeSurvival {
			slot.Wear(1)
			s.SendInventory(p)
		}
		return true
	}
	if block == blockFarmland {
		crop := cropForItem(held)
		if crop == blockAir {
			return false
		}
		if s.World.BlockAt(x, y+1, z) != blockAir {
			return true
		}
		s.setFarmBlock(x, y+1, z, crop, 0)
		if p.GameMode == ModeSurvival {
			slot.Count--
			if slot.Count == 0 {
				*slot = ItemStack{}
			}
			s.SendInventory(p)
		}
		return true
	}
	return false
}

func (s *Server) dropFarmItem(x, y, z int, id byte, count int32) {
	if count <= 0 {
		return
	}
	s.SpawnEntity(&ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("farm-%d-%d-%d-%d-%d", x, y, z, id, time.Now().UnixNano()), Type: EntityItem, X: float64(x) + .5, Y: float64(y) + .3, Z: float64(z) + .5}, ItemStack: Item{ID: int32(id), Count: count}, PickupDelay: .4, Vy: .1})
}

func (s *Server) dropCrop(x, y, z int, crop, stage byte) {
	switch crop {
	case blockWheatCrop:
		if stage >= 7 {
			s.dropFarmItem(x, y, z, itemWheat, 1)
			s.dropFarmItem(x, y, z, itemWheatSeeds, int32(1+rand.Intn(3)))
		} else {
			s.dropFarmItem(x, y, z, itemWheatSeeds, 1)
		}
	case blockPotatoCrop:
		count := int32(1)
		if stage >= 3 {
			count += int32(rand.Intn(4))
		}
		s.dropFarmItem(x, y, z, itemPotato, count)
	case blockCarrotCrop:
		count := int32(1)
		if stage >= 3 {
			count += int32(rand.Intn(4))
		}
		s.dropFarmItem(x, y, z, itemCarrot, count)
	}
}
