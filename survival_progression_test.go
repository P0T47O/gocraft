package main

import (
	"bytes"
	"math"
	"testing"
)

func TestSwordAndBowSurvivalCombat(t *testing.T) {
	s, p := hostileTestServer(t)
	for id, damage := range map[byte]int{itemWoodSword: 4, itemStoneSword: 5, itemIronSword: 6, itemDiamondSword: 7, itemGoldSword: 4} {
		if swordDamage(id) != damage || GetItem(id).MaxStack != 1 {
			t.Fatalf("sword %d lacks combat stats", id)
		}
	}
	p.Inventory.Slots[0] = Item{ID: int32(itemBow), Count: 1}
	p.Inventory.Slots[1] = Item{ID: int32(itemArrow), Count: 2}
	if !s.shootSelectedBow(p) || p.Inventory.Slots[1].Count != 1 || p.Inventory.Slots[0].Damage != 1 {
		t.Fatal("bow did not consume arrow and durability")
	}
	if s.shootSelectedBow(p) {
		t.Fatal("bow ignored cooldown")
	}
	p.AttackCooldown = 0
	p.Inventory.Slots[1] = Item{}
	if s.shootSelectedBow(p) {
		t.Fatal("bow fired without arrows")
	}
	seen := false
	for _, e := range s.World.entities {
		if a, ok := e.(*ArrowEntity); ok && a.Owner == p.UUID {
			seen = true
		}
	}
	if !seen {
		t.Fatal("player arrow not spawned")
	}
	m := newMob("zombie", "archery-target", 8, 70.5, 8)
	s.World.entities = append(s.World.entities, m)
	a := &ArrowEntity{BaseEntity: BaseEntity{UUID: "shot", Type: EntityArrow, X: 8, Y: 71, Z: 6}, Owner: p.UUID, Vz: 2, Damage: 4}
	a.Tick(s.World)
	if a.HitMob != m || !a.Dead {
		t.Fatal("arrow missed mob collision box")
	}
}

func TestArrowHitsNearestTarget(t *testing.T) {
	s, p := hostileTestServer(t)
	far := newMob("zombie", "far-target", 8, 70.5, 9)
	near := newMob("zombie", "near-target", 8, 70.5, 7)
	s.World.entities = append(s.World.entities, far, near)
	a := &ArrowEntity{BaseEntity: BaseEntity{UUID: "nearest-shot", Type: EntityArrow, X: 8, Y: 71, Z: 5}, Owner: p.UUID, Vz: 5, Damage: 4}
	a.Tick(s.World)
	if a.HitMob != near || a.HitPlayer != nil {
		t.Fatalf("arrow selected wrong target: mob=%v player=%v", a.HitMob, a.HitPlayer)
	}
}

func TestSheepWoolAndFeedingPair(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z, p.Pitch = 6, -.45
	if mobContent.Definitions["sheep"].Drops[0].Item != "wool" || mobDropItems["wool"] != blockWhiteWool {
		t.Fatal("wool source missing")
	}
	a := newMob("sheep", "sheep-a", 8, 70.5, 8)
	b := newMob("sheep", "sheep-b", 9, 70.5, 8)
	s.World.entities = append(s.World.entities, a, b)
	p.Inventory.Slots[0] = Item{ID: int32(itemWheat), Count: 2}
	if !s.feedMob(p, a.UUID) {
		t.Fatal("first sheep could not be fed")
	}
	p.Yaw = .45
	if !s.feedMob(p, b.UUID) || p.Inventory.Slots[0].ID != 0 {
		t.Fatal("feeding did not consume wheat")
	}
	s.tickBreeding()
	if a.BreedCooldown == 0 || b.BreedCooldown == 0 || len(s.World.entities) != 4 {
		t.Fatal("pair produced no child")
	}
	child := s.World.entities[3].(*MobEntity)
	if child.Kind != "sheep" || child.BabyTicks == 0 || !child.snapshot().Baby {
		t.Fatal("child state missing")
	}
	if s.feedMob(p, a.UUID) {
		t.Fatal("parent immediately bred again")
	}
	var wire bytes.Buffer
	if err := WritePacket(&wire, &PacketInteractMob{Target: a.UUID}); err != nil {
		t.Fatal(err)
	}
	packet, err := ReadPacket(&wire)
	if err != nil || packet.(*PacketInteractMob).Target != a.UUID {
		t.Fatal("interaction packet failed", err)
	}
}

func TestFeedingRejectsTargetsBehindWall(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z, p.Pitch = 6, -.45
	m := newMob("sheep", "walled-sheep", 8, 70.5, 8)
	s.World.entities = append(s.World.entities, m)
	p.Inventory.Slots[0] = Item{ID: int32(itemWheat), Count: 1}
	s.World.SetBlockAt(8, 71, 7, blockStone)
	if s.feedMob(p, m.UUID) || m.LoveTicks != 0 || p.Inventory.Slots[0].Count != 1 {
		t.Fatal("feeding reached through a wall")
	}
}

func TestBedSleepAndRespawnFallback(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	w.SetBlockAt(8, 71, 8, blockBed)
	w.TimeTicks = 18000
	if !s.useBed(p, BlockPos{8, 71, 8}) || !p.Vitals.HasHome || math.Mod(w.TimeTicks, 24000) != 0 {
		t.Fatal("bed did not set home and skip night")
	}
	x, y, z, ok := s.safeRespawn(p.Vitals)
	if !ok || x != 9 || z != 8 || math.Abs(y-72.13) > .02 {
		t.Fatalf("unsafe bed spawn: %v %v %v %v", x, y, z, ok)
	}
	w.SetBlockAt(8, 71, 8, blockAir)
	x, _, z, ok = s.safeRespawn(p.Vitals)
	if !ok || x != 8 || z != 8 {
		t.Fatal("destroyed bed did not fall back to original spawn")
	}
	InitRecipes()
	if r := RecipeRegistry[66]; r == nil || r.Result.ID != int32(blockBed) || r.Ingredients[0].ID != int32(blockWhiteWool) {
		t.Fatal("bed recipe missing")
	}
}

func TestBedRespawnWaitsForHomeChunk(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Vitals.HasHome, p.Vitals.HomeX, p.Vitals.HomeY, p.Vitals.HomeZ = true, 20, 71, 8
	if _, _, _, ok := s.safeRespawn(p.Vitals); ok {
		t.Fatal("unloaded bed chunk silently fell back to world spawn")
	}
	c := lifecycleChunk(w, chunkKey{1, 0})
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.blocks.Set(x, 70, z, blockStone)
		}
	}
	w.SetBlockAt(20, 71, 8, blockBed)
	x, _, z, ok := s.safeRespawn(p.Vitals)
	if !ok || x != 21 || z != 8 {
		t.Fatalf("loaded bed was not used: %v %v %v", x, z, ok)
	}
}

func TestBedRequiresOnlineSurvivorsToSleep(t *testing.T) {
	w := lifeTestWorld(t)
	s, first := lifeTestPlayer(w)
	second := &PlayerEntity{BaseEntity: BaseEntity{UUID: "second", Type: EntityPlayer, X: 10, Y: 72.125, Z: 8}, GameMode: ModeSurvival}
	second.initVitals()
	w.entities = append(w.entities, second)
	s.Clients[first.UUID] = &ClientConnection{Send: make(chan Packet, 8)}
	s.Clients[second.UUID] = &ClientConnection{Send: make(chan Packet, 8)}
	w.SetBlockAt(8, 71, 8, blockBed)
	w.SetBlockAt(10, 71, 8, blockBed)
	w.TimeTicks = 18000
	if !s.useBed(first, BlockPos{8, 71, 8}) || w.TimeTicks != 18000 {
		t.Fatal("one sleeper skipped a multiplayer night")
	}
	if !s.useBed(second, BlockPos{10, 71, 8}) || int64(w.TimeTicks)%24000 != 0 {
		t.Fatal("all sleepers did not skip the night")
	}
	if len(s.sleeping) != 0 {
		t.Fatal("sleep votes survived sunrise")
	}
}

func TestBedHomeAndBabySurviveSave(t *testing.T) {
	w := lifeTestWorld(t)
	_, p := lifeTestPlayer(w)
	p.Vitals.HasHome, p.Vitals.HomeX, p.Vitals.HomeY, p.Vitals.HomeZ = true, 8, 71, 8
	root := t.TempDir()
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	w.entities = nil
	if err := loadSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	loaded := w.entities[0].(*PlayerEntity)
	if !loaded.Vitals.HasHome || loaded.Vitals.HomeX != 8 || loaded.Vitals.HomeY != 71 || loaded.Vitals.HomeZ != 8 {
		t.Fatal("bed spawn was lost on reload")
	}
	child := newMob("sheep", "saved-child", 9, 71, 9)
	child.BabyTicks = 12000
	w.entities = []Entity{child}
	if err := SaveEntities(root, w); err != nil {
		t.Fatal(err)
	}
	w.entities = nil
	if _, err := LoadEntities(root, w); err != nil {
		t.Fatal(err)
	}
	if got := w.entities[0].(*MobEntity); got.Kind != "sheep" || got.BabyTicks != 12000 {
		t.Fatal("baby sheep state was lost on reload")
	}
}
