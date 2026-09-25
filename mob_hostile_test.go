package main

import (
	"bytes"
	"math"
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

func TestServerTickHostileCombat(t *testing.T) {
	for _, tc := range []struct {
		kind    string
		playerZ float64
		ticks   int
	}{
		{"zombie", 10, 25},
		{"skeleton", 13, 25},
		{"creeper", 10, 40},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			s, p := hostileTestServer(t)
			p.Z = tc.playerZ
			s.World.TimeTicks = 18000
			m := newMob(tc.kind, "combat-"+tc.kind, 8, 70.501, 8)
			s.World.entities = append(s.World.entities, m)
			for i := 0; i < tc.ticks; i++ {
				s.Tick()
			}
			if p.Vitals.Health >= maxHealth {
				t.Fatalf("%s did not damage player through full server loop", tc.kind)
			}
			if tc.kind == "creeper" && (m.Health != 0 || !m.Dropped) {
				t.Fatal("creeper did not self-destruct")
			}
			seenState, seenArrow := false, false
			for len(s.Clients[p.UUID].Send) > 0 {
				packet := <-s.Clients[p.UUID].Send
				var wire bytes.Buffer
				if err := WritePacket(&wire, packet); err != nil {
					t.Fatal(err)
				}
				decoded, err := ReadPacket(&wire)
				if err != nil {
					t.Fatalf("%s network packet: %v", tc.kind, err)
				}
				if state, ok := decoded.(*PacketMobState); ok && state.Kind == tc.kind {
					seenState = true
				}
				if spawn, ok := decoded.(*PacketEntitySpawn); ok && spawn.Type == EntityArrow {
					seenArrow = true
				}
			}
			if !seenState || tc.kind == "skeleton" && !seenArrow {
				t.Fatalf("%s state or arrow was not delivered to client", tc.kind)
			}
		})
	}
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

func TestArrowSegmentHitsWholePlayerBody(t *testing.T) {
	s, p := hostileTestServer(t)
	for _, tc := range []struct {
		name string
		y    float64
		hit  bool
	}{
		{"head", p.Y + .1, true},
		{"legs", p.Y - 1.5, true},
		{"above", p.Y + .35, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &ArrowEntity{BaseEntity: BaseEntity{X: p.X, Y: tc.y, Z: p.Z - 1}, Vz: 2}
			a.Tick(s.World)
			if (a.HitPlayer == p) != tc.hit {
				t.Fatalf("arrow hit=%t, want %t", a.HitPlayer == p, tc.hit)
			}
		})
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

func TestNearbySpawnTriesAnotherLoadedColumn(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z = 8
	s.World.TimeTicks = 18000
	def := mobContent.Definitions["creeper"]
	phase := 2.399
	radius := def.SpawnRadius * .875
	x, z := int(math.Floor(p.X+math.Sin(phase)*radius)), int(math.Floor(p.Z+math.Cos(phase)*radius))
	c := lifecycleChunk(s.World, chunkKey{divFloor(x, 16), divFloor(z, 16)})
	c.mu.Lock()
	c.skyLight.Fill(15)
	c.mu.Unlock()
	s.World.SetBlockAt(x, 70, z, blockGrass)
	s.spawnNearbyMobs()
	for _, e := range s.World.entities {
		if m, ok := e.(*MobEntity); ok && m.Kind == "creeper" && int(m.X) == x && int(m.Z) == z {
			return
		}
	}
	t.Fatal("spawn did not recover from the first unsuitable column")
}

func TestSpiderClimbsWallButNotCliff(t *testing.T) {
	w := mobTestWorld(t)
	for x := 6; x <= 10; x++ {
		for y := 71; y <= 73; y++ {
			w.SetBlockAt(x, y, 8, blockStone)
		}
	}
	spider := newMob("spider", "climber", 8, 70.501, 5)
	maxY := spider.Y
	crossed := false
	for i := 0; i < 110; i++ {
		spider.State, spider.Yaw = "chase", 0
		spider.Tick(w)
		maxY = max(maxY, spider.Y)
		crossed = crossed || spider.Z > 9
	}
	if maxY < 73.4 || !crossed {
		t.Fatalf("spider did not scale three-block wall: height=%.2f z=%.2f", maxY, spider.Z)
	}
	for x := 0; x < 16; x++ {
		for z := 8; z < 16; z++ {
			for y := 70; y <= 73; y++ {
				w.SetBlockAt(x, y, z, blockAir)
			}
		}
	}
	spider = newMob("spider", "cliff", 8, 70.501, 5)
	for i := 0; i < 80; i++ {
		spider.State, spider.Yaw = "chase", 0
		spider.Tick(w)
	}
	if spider.Y > 70.8 {
		t.Fatalf("spider climbed empty cliff: %.2f", spider.Y)
	}
}

func TestServerTickSpiderPursuesOverWall(t *testing.T) {
	s, p := hostileTestServer(t)
	p.Z = 12
	for x := 6; x <= 10; x++ {
		for y := 71; y <= 73; y++ {
			s.World.SetBlockAt(x, y, 8, blockStone)
		}
	}
	spider := newMob("spider", "server-climber", 8, 70.501, 5)
	spider.Target = p.UUID // Previously spotted target is now behind the wall.
	s.World.entities = append(s.World.entities, spider)
	maxY := spider.Y
	for i := 0; i < 100; i++ {
		s.Tick()
		maxY = max(maxY, spider.Y)
	}
	if maxY < 73.4 || spider.Z <= 9 {
		t.Fatalf("server AI failed to pursue across wall: maxY=%.2f z=%.2f state=%s", maxY, spider.Z, spider.State)
	}
}
