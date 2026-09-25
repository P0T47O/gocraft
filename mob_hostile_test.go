package main

import (
	"reflect"
	"testing"
)

func hostileTestServer(t *testing.T) (*Server, *PlayerEntity) {
	t.Helper()
	w := mobTestWorld(t)
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "target", Type: EntityPlayer, X: 8, Y: 72.1, Z: 10}, GameMode: ModeSurvival}
	p.initVitals()
	w.entities = []Entity{p}
	s := &Server{World: w, Clients: map[string]*ClientConnection{"target": {Send: make(chan Packet, 256)}}, LastSentPos: map[string][3]float64{}, LastSentMeta: map[string]int32{}}
	return s, p
}

func TestHostileContentAndLootTable(t *testing.T) {
	for _, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
		d, ok := mobContent.Definitions[kind]
		if !ok || !d.Hostile || d.Model == "" || d.Animation == "" || len(d.Drops) == 0 {
			t.Fatalf("incomplete hostile definition %s", kind)
		}
		for _, drop := range d.Drops {
			if mobDropItems[drop.Item] == 0 {
				t.Fatalf("missing item for %s drop %s", kind, drop.Item)
			}
		}
	}
	initBlockRegistry()
	initItemRegistry()
	for _, id := range []byte{itemRottenFlesh, itemBone, itemArrow, itemString, itemGunpowder} {
		if Items[id] == nil || Items[id].Icon == "" {
			t.Fatalf("missing item %d", id)
		}
	}
}

func TestZombieChaseAndMelee(t *testing.T) {
	s, p := hostileTestServer(t)
	m := newMob("zombie", "z", 8, 70.501, 8.7)
	s.World.entities = append(s.World.entities, m)
	s.updateMobAI()
	if m.State != "chase" || m.Target != p.UUID || m.AttackCooldown == 0 || p.Vitals.Health != maxHealth-3 {
		t.Fatalf("zombie attack: state=%s target=%s cooldown=%d health=%d", m.State, m.Target, m.AttackCooldown, p.Vitals.Health)
	}
	s.updateMobAI()
	if p.Vitals.Health != maxHealth-3 {
		t.Fatal("zombie bypassed attack cooldown")
	}
}

func TestHostileDoesNotAttackThroughWall(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z = 8.55
	m := newMob("zombie", "wall-zombie", 8, 70.501, 7.45)
	m.Target = p.UUID
	m.AvoidTicks, m.AvoidYaw = 5, 1.25
	s.World.entities = append(s.World.entities, m)
	for y := 71; y <= 73; y++ {
		s.World.SetBlockAt(8, y, 8, blockStone)
	}
	s.updateMobAI()
	if p.Vitals.Health != maxHealth || m.State != "chase" || m.AvoidTicks != 4 || m.Yaw != 1.25 {
		t.Fatalf("wall pursuit failed: health=%d state=%s avoid=%d yaw=%f", p.Vitals.Health, m.State, m.AvoidTicks, m.Yaw)
	}
}

func TestSkeletonArrowHitsAndWallStopsIt(t *testing.T) {
	for _, wall := range []bool{false, true} {
		s, p := hostileTestServer(t)
		p.Z = 13
		m := newMob("skeleton", "s", 8, 70.501, 8)
		s.World.entities = append(s.World.entities, m)
		s.updateMobAI()
		if len(s.World.entities) != 3 {
			t.Fatal("skeleton did not shoot")
		}
		if wall {
			for y := 71; y <= 73; y++ {
				s.World.SetBlockAt(8, y, 10, blockStone)
			}
		}
		for i := 0; i < 20; i++ {
			s.UpdateEntities()
		}
		if wall && p.Vitals.Health != maxHealth || !wall && p.Vitals.Health >= maxHealth {
			t.Fatalf("arrow wall=%t health=%d", wall, p.Vitals.Health)
		}
	}
}

func TestSpiderPounceAndCreeperFuse(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z = 12
	spider := newMob("spider", "sp", 8, 70.501, 8)
	s.World.entities = append(s.World.entities, spider)
	s.updateMobAI()
	if spider.Velocity.Y <= 0 || spider.AttackCooldown == 0 {
		t.Fatal("spider did not pounce")
	}
	s.World.entities = []Entity{p}
	p.Z = 10
	creeper := newMob("creeper", "cr", 8, 70.501, 8)
	s.World.entities = append(s.World.entities, creeper)
	for i := 0; i < mobContent.Definitions["creeper"].AttackTicks; i++ {
		s.updateMobAI()
	}
	if creeper.Health != 0 || !creeper.Dropped || p.Vitals.Health >= maxHealth {
		t.Fatalf("creeper fuse failed: health=%d dropped=%t target=%d", creeper.Health, creeper.Dropped, p.Vitals.Health)
	}
}

func TestHostileLootAndTransientArrowSave(t *testing.T) {
	s, _ := hostileTestServer(t)
	s.World.entities = nil
	for _, kind := range []string{"zombie", "skeleton", "spider", "creeper"} {
		m := newMob(kind, kind, 8, 70.501, 8)
		m.Health = 0
		s.World.entities = append(s.World.entities, m)
	}
	s.updateMobs()
	for _, e := range s.World.entities {
		item, ok := e.(*ItemEntity)
		if !ok {
			continue
		}
		if !validStack(item.ItemStack) {
			t.Fatalf("invalid mob drop %+v", item.ItemStack)
		}
	}
	m := newMob("skeleton", "saved", 8, 70.501, 8)
	s.World.entities = []Entity{m, &ArrowEntity{BaseEntity: BaseEntity{UUID: "transient", Type: EntityArrow}}}
	root := t.TempDir()
	if err := SaveEntities(root, s.World); err != nil {
		t.Fatal(err)
	}
	loaded := NewClientWorld()
	defer loaded.Close()
	if _, err := LoadEntities(root, loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.entities) != 1 || !reflect.DeepEqual(m, loaded.entities[0]) {
		t.Fatal("mob persistence or transient-arrow filtering failed")
	}
}

func TestHostileSpawnLightAndDespawn(t *testing.T) {
	s, _ := hostileTestServer(t)
	c := s.World.getChunkIfGenerated(0, 0)
	c.mu.Lock()
	c.skyLight.Fill(15)
	c.mu.Unlock()
	zombie := mobContent.Definitions["zombie"]
	s.World.TimeTicks = 6000
	if canSpawnMobAt(s.World, zombie, 8, 71, 8) {
		t.Fatal("hostile spawned in daylight")
	}
	s.World.TimeTicks = 18000
	if !canSpawnMobAt(s.World, zombie, 8, 71, 8) {
		t.Fatal("hostile did not spawn at night")
	}
	c.mu.Lock()
	c.blockLight.Set(8, 71, 8, 15)
	c.mu.Unlock()
	if canSpawnMobAt(s.World, zombie, 8, 71, 8) {
		t.Fatal("hostile spawned beside a torch")
	}
	m := newMob("zombie", "distant", 150, 70.501, 8)
	s.World.entities = append(s.World.entities, m)
	s.updateMobs()
	for _, e := range s.World.entities {
		if e.GetUUID() == m.UUID {
			t.Fatal("faraway hostile was not despawned")
		}
	}
}

func TestNearbyHostileActuallySpawnsAtNight(t *testing.T) {
	for _, ticks := range []float64{6000, 18000} {
		s, p := hostileTestServer(t)
		p.Z = 8
		for _, cz := range []int{1, 2} {
			c := lifecycleChunk(s.World, chunkKey{0, cz})
			c.mu.Lock()
			c.skyLight.Fill(15)
			c.mu.Unlock()
		}
		s.World.SetBlockAt(8, 70, 32, blockGrass)
		s.World.TimeTicks = ticks
		// Sorted definitions pick creeper at index zero; phase zero targets
		// precisely (playerX, playerZ + spawnRadius).
		s.MobSpawnTicks = 0
		s.spawnNearbyMobs()
		spawned := false
		for _, e := range s.World.entities {
			if m, ok := e.(*MobEntity); ok && m.Kind == "creeper" {
				spawned = true
			}
		}
		if spawned != (ticks == 18000) {
			t.Fatalf("time %.0f: hostile spawn=%t", ticks, spawned)
		}
	}
}
