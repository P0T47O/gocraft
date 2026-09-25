package main

import "testing"

func TestEatSelectedFoodServerAuthoritative(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Vitals.Food = 10
	p.Inventory.Slots[0] = Item{ID: int32(itemCookedPork), Count: 2}
	p.Inventory.Slots[1] = Item{ID: int32(itemRawPork), Count: 1}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerAction{ActionType: 2, Value: int32(itemRawPork)}})
	if p.Vitals.Food != 18 || p.Inventory.Slots[0].Count != 1 || p.Inventory.Slots[1].Count != 1 {
		t.Fatal("eat must use the selected stack, not the packet value")
	}
	p.Vitals.Food = maxFood
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerAction{ActionType: 2}})
	if p.Inventory.Slots[0].Count != 1 {
		t.Fatal("full food bar consumed a stack")
	}
	p.Vitals.Food = 10
	p.Inventory.Slots[0] = Item{ID: int32(blockStone), Count: 1}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerAction{ActionType: 2}})
	if p.Vitals.Food != 10 || p.Inventory.Slots[0].Count != 1 {
		t.Fatal("non-food item changed hunger")
	}
	p.GameMode = ModeCreative
	p.Inventory.Slots[0] = Item{ID: int32(itemCookedPork), Count: 1}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerAction{ActionType: 2}})
	if p.Inventory.Slots[0].Count != 1 {
		t.Fatal("creative mode consumed food")
	}
}

func TestHungerRecoveryAndStarvation(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	v := p.Vitals
	v.Health, v.Food, v.Saturation, v.dryTicks, v.Exhaustion = 10, 18, 0, 199, 3.5
	s.tickPlayerFood(p)
	if v.Health != 11 || v.Exhaustion < 3 {
		t.Fatal("fed player did not heal with an exhaustion cost")
	}
	root := t.TempDir()
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal("cannot save after food recovery", err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if err := loadSurvivalPlayers(root, loaded); err != nil {
		t.Fatal("cannot load valid post-recovery exhaustion", err)
	}
	v.Health, v.Food, v.dryTicks, v.cooldown = 3, 0, 199, 0
	s.tickPlayerFood(p)
	if v.Health != 2 {
		t.Fatal("empty food bar did not cause starvation")
	}
	v.Health, v.dryTicks, v.cooldown = 1, 199, 0
	s.tickPlayerFood(p)
	if v.Health != 1 {
		t.Fatal("normal-difficulty starvation should stop at one health")
	}
	v.Health, v.Food, v.dryTicks = 0, maxFood, 199
	s.tickPlayerFood(p)
	if v.Health != 0 {
		t.Fatal("food regeneration resurrected a dead player")
	}
}

func TestHungerMovementCostAndIdle(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	v := p.Vitals
	v.Saturation = 0
	for i := 0; i < 100; i++ {
		s.tickPlayerFood(p)
	}
	if v.Exhaustion != 0 || v.Food != maxFood {
		t.Fatal("idle player lost food")
	}
	p.X += .2
	s.tickPlayerFood(p)
	walk := v.Exhaustion
	p.X += .3
	s.tickPlayerFood(p)
	if walk <= 0 || v.Exhaustion-walk <= walk*5 {
		t.Fatalf("fast movement not more exhausting: walk=%f total=%f", walk, v.Exhaustion)
	}
	p.X += 20 // Teleport; excluded from exhaustion.
	before := v.Exhaustion
	s.tickPlayerFood(p)
	if v.Exhaustion != before {
		t.Fatal("teleport consumed hunger")
	}
}
