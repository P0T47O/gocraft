package main

import "testing"

func TestDoorTwoBlockLifecycleAndShape(t *testing.T) {
	s, p := hostileTestServer(t)
	p.GameMode = ModeCreative
	p.Yaw = 1.57 // A non-default facing also exercises paired cleanup.
	s.World.SetBlockAt(8, 70, 8, blockStone)
	place := &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockWoodDoor}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: place})
	if s.World.BlockAt(8, 71, 8) != blockWoodDoor || s.World.BlockAt(8, 72, 8) != blockWoodDoor || s.World.MetaAt(8, 72, 8)&shapeUpper == 0 {
		t.Fatal("door halves were not placed together")
	}
	closed, _ := shapeBoxes(blockWoodDoor, s.World.MetaAt(8, 71, 8))
	passage := Collider{Width: .2, Depth: .2, Height: 1}
	if !colliderHitsCore(s.World, meshVec3(7.6, 70.6, 8), passage) {
		t.Fatal("closed door does not block passage")
	}
	if !s.toggleDoor(BlockPos{8, 72, 8}) {
		t.Fatal("upper door interaction was ignored")
	}
	if s.World.MetaAt(8, 71, 8)&shapeDouble == 0 || s.World.MetaAt(8, 72, 8)&shapeDouble == 0 {
		t.Fatal("door halves disagreed on open state")
	}
	opened, _ := shapeBoxes(blockWoodDoor, s.World.MetaAt(8, 71, 8))
	if opened[0] == closed[0] {
		t.Fatal("opening did not move the collision shape")
	}
	if colliderHitsCore(s.World, meshVec3(7.6, 70.6, 8), passage) {
		t.Fatal("open door still blocks central passage")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockAir}})
	if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8, 72, 8) != blockAir {
		t.Fatal("breaking lower half left a floating upper half")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 71, Z: 8, BlockID: blockWoodDoor}})
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 8, Y: 70, Z: 8, BlockID: blockAir}})
	if s.World.BlockAt(8, 71, 8) != blockAir || s.World.BlockAt(8, 72, 8) != blockAir {
		t.Fatal("breaking support left a floating door")
	}
}

func TestDoorAtChunkEdgeOpensAndUpperBreakRemovesLower(t *testing.T) {
	s, p := hostileTestServer(t)
	p.GameMode, p.X = ModeCreative, 15
	s.World.SetBlockAt(15, 70, 8, blockStone)
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 15, Y: 71, Z: 8, BlockID: blockWoodDoor}})
	if s.World.BlockAt(15, 72, 8) != blockWoodDoor {
		t.Fatal("chunk-edge door upper half missing")
	}
	if !s.toggleDoor(BlockPos{15, 71, 8}) || s.World.MetaAt(15, 72, 8)&shapeDouble == 0 {
		t.Fatal("chunk-edge door toggle was not synchronized")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketBlockChange{X: 15, Y: 72, Z: 8, BlockID: blockAir}})
	if s.World.BlockAt(15, 71, 8) != blockAir || s.World.BlockAt(15, 72, 8) != blockAir {
		t.Fatal("breaking the upper half left the lower half")
	}
}

func TestShearingProducesWoolOnceAndRegrows(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z, p.Pitch = 6, -.45
	m := newMob("sheep", "shear-test", 8, 70.5, 8)
	s.World.entities = append(s.World.entities, m)
	p.Inventory.Slots[0] = Item{ID: int32(itemShears), Count: 1}
	if !s.shearMob(p, m.UUID) || !m.Sheared || m.ShearCooldown != 1200 || p.Inventory.Slots[0].Damage != 1 {
		t.Fatal("first shearing did not produce server-owned state")
	}
	if s.shearMob(p, m.UUID) {
		t.Fatal("sheep was sheared twice without regrowth")
	}
	drops := 0
	for _, entity := range s.World.entities {
		if item, ok := entity.(*ItemEntity); ok && item.ItemStack.ID == int32(blockWhiteWool) {
			drops += int(item.ItemStack.Count)
		}
	}
	if drops < 1 || drops > 3 || !m.snapshot().Sheared {
		t.Fatalf("wool drops or remote state incorrect: %d", drops)
	}
	m.ShearCooldown = 1
	s.tickBreeding()
	if m.Sheared || m.ShearCooldown != 0 {
		t.Fatal("wool did not regrow")
	}
}

func TestRenewableArrowMaterialsRegistered(t *testing.T) {
	if mobDropItems["feather"] != itemFeather || mobContent.Definitions["chicken"].Drops[0].Item != "feather" {
		t.Fatal("feather source missing")
	}
	if RecipeRegistry[72] == nil || RecipeRegistry[72].Result.ID != int32(itemArrow) || RecipeRegistry[72].Result.Count != 4 {
		t.Fatal("arrow recipe missing")
	}
	if GetItem(itemShears).MaxDurability == 0 || GetItem(itemFlint).ID != itemFlint {
		t.Fatal("new items were not registered")
	}
}
