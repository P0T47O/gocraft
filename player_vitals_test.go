package main

import (
	"bytes"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"testing"
)

func lifeTestWorld(t *testing.T) *World {
	t.Helper()
	initBlockRegistry()
	w := NewClientWorld()
	t.Cleanup(w.Close)
	c := lifecycleChunk(w, chunkKey{0, 0})
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.blocks[x][70][z] = blockStone
		}
	}
	return w
}
func lifeTestPlayer(w *World) (*Server, *PlayerEntity) {
	p := &PlayerEntity{BaseEntity: BaseEntity{UUID: "survivor", Type: EntityPlayer, X: 8, Y: 72.125, Z: 8}, GameMode: ModeSurvival}
	p.initVitals()
	w.entities = []Entity{p}
	return &Server{World: w, Clients: map[string]*ClientConnection{}}, p
}
func TestMovementWalkSprintAndSneakEdge(t *testing.T) {
	w := lifeTestWorld(t)
	start := rl.NewVector3(8, 72.125, 3)
	var walk, run InputState
	p, q := start, start
	for i := 0; i < 60; i++ {
		p = walk.StepMovement(w, p, 1.0/60, MovementControls{Forward: 1}, false)
		q = run.StepMovement(w, q, 1.0/60, MovementControls{Forward: 1, Sprint: true}, false)
	}
	if math.Abs(float64((q.Z-start.Z)/(p.Z-start.Z))-1.5) > 0.05 {
		t.Fatal("sprint speed", p, q)
	}
	if p.Y < 72.11 || q.Y < 72.11 {
		t.Fatal("fell through floor")
	}
	c := w.getChunkIfGenerated(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			c.blocks[x][70][z] = blockAir
		}
	}
	c.blocks[8][70][8] = blockStone
	p = rl.NewVector3(8, 72.125, 8)
	var sneak InputState
	for i := 0; i < 180; i++ {
		p = sneak.StepMovement(w, p, 1.0/60, MovementControls{Forward: 1, Sneak: true}, false)
	}
	if p.Z > 8.81 || p.Y < 72.11 {
		t.Fatal("sneak walked off edge", p)
	}
}
func TestWaterMovementAndSweptCollision(t *testing.T) {
	w := lifeTestWorld(t)
	c := w.getChunkIfGenerated(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 71; y <= 74; y++ {
				c.blocks[x][y][z] = blockWater
			}
		}
	}
	start := rl.NewVector3(8, 73, 8)
	up, down := start, start
	var a, b InputState
	for i := 0; i < 30; i++ {
		up = a.StepMovement(w, up, 1.0/60, MovementControls{Jump: true}, false)
		down = b.StepMovement(w, down, 1.0/60, MovementControls{Sneak: true}, false)
	}
	if up.Y <= start.Y || down.Y >= start.Y || !a.IsSwimming {
		t.Fatal("swim controls", up, down)
	}
	p := resolveCollision(w, rl.NewVector3(8, 90, 8), rl.NewVector3(0, -40, 0))
	if p.Y < 72.11 || p.Y > 72.14 {
		t.Fatal("fast fall tunneled through floor", p)
	}
}
func TestFallDamageWaterAndCreativeImmunity(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Y = 80
	p.initVitals()
	p.Y = 72.125
	s.tickPlayerVitals(p)
	if p.Vitals.Health != 15 {
		t.Fatal("fall damage", p.Vitals.Health)
	}
	p.Vitals = nil
	p.Y = 90
	p.initVitals()
	w.getChunkIfGenerated(0, 0).blocks[8][71][8] = blockWater
	p.Y = 72.125
	s.tickPlayerVitals(p)
	if p.Vitals.Health != 20 || p.Vitals.FallDistance != 0 {
		t.Fatal("water did not cushion fall")
	}
	p.GameMode = ModeCreative
	p.Y = -80
	s.tickPlayerVitals(p)
	if p.Vitals.Health != 20 {
		t.Fatal("creative took damage")
	}
}
func TestDrowningLavaAndSuffocation(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	c := w.getChunkIfGenerated(0, 0)
	c.blocks[8][72][8] = blockWater
	for i := 0; i < maxAir+1; i++ {
		s.tickPlayerVitals(p)
	}
	if p.Vitals.Air != 0 || p.Vitals.Health >= 20 {
		t.Fatal("drowning missing")
	}
	c.blocks[8][72][8] = blockAir
	s.tickPlayerVitals(p)
	if p.Vitals.Air == 0 {
		t.Fatal("air not replenished")
	}
	p.Vitals.cooldown = 0
	c.blocks[8][71][8] = blockLava
	s.tickPlayerVitals(p)
	if p.Vitals.FireTicks == 0 {
		t.Fatal("lava did not ignite")
	}
	c.blocks[8][71][8] = blockWater
	s.tickPlayerVitals(p)
	if p.Vitals.FireTicks != 0 {
		t.Fatal("water did not extinguish")
	}
	p.Vitals.cooldown = 0
	c.blocks[8][72][8] = blockStone
	before := p.Vitals.Health
	s.tickPlayerVitals(p)
	if p.Vitals.Health >= before {
		t.Fatal("suffocation missing")
	}
}
func TestDeathDropsOnceBlocksActionsAndRespawns(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Inventory.Slots[0] = Item{ID: int32(blockLog), Count: 12}
	p.CursorItem = Item{ID: int32(blockDirt), Count: 3}
	s.hurtPlayer(p, 20, "Test death")
	count := 0
	for _, e := range w.entities {
		if d, ok := e.(*ItemEntity); ok {
			count += d.Count
		}
	}
	if count != 15 || !p.dead() || p.CursorItem.ID != 0 || p.Inventory.Slots[0].ID != 0 {
		t.Fatal("death inventory handling")
	}
	n := len(w.entities)
	s.hurtPlayer(p, 20, "Again")
	if len(w.entities) != n {
		t.Fatal("duplicate drops")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketPlayerMove{X: 99, Y: 99, Z: 99}})
	if p.X != 8 {
		t.Fatal("dead player moved")
	}
	s.HandlePacket(PacketWrapper{From: p.UUID, Packet: &PacketRespawn{}})
	if p.dead() || p.Vitals.Health != 20 || p.Vitals.Air != maxAir || collides(w, rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))) {
		t.Fatal("unsafe respawn")
	}
	if p.Inventory.Slots[0].ID != 0 {
		t.Fatal("respawn duplicated inventory")
	}
}
func TestVitalsPersistAndPacketRoundTrip(t *testing.T) {
	w := lifeTestWorld(t)
	_, p := lifeTestPlayer(w)
	p.Vitals.Health = 7
	p.Vitals.Air = 90
	p.Vitals.FireTicks = 100
	p.Vitals.FallDistance = 2
	root := t.TempDir()
	if err := saveSurvivalPlayers(root, w); err != nil {
		t.Fatal(err)
	}
	other := NewClientWorld()
	defer other.Close()
	if err := loadSurvivalPlayers(root, other); err != nil {
		t.Fatal(err)
	}
	loaded := other.entities[0].(*PlayerEntity)
	if loaded.Vitals.Health != 7 || loaded.Vitals.Air != 90 || loaded.Vitals.SpawnX != p.Vitals.SpawnX || loaded.Vitals.FallDistance != 2 {
		t.Fatal("vitals lost")
	}
	var b bytes.Buffer
	want := &PacketVitals{Health: 7, Air: 90, Fire: 100, Cause: "Lava"}
	if err := WritePacket(&b, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPacket(&b)
	if err != nil || *got.(*PacketVitals) != *want {
		t.Fatal("vitals packet", err)
	}
	b.Reset()
	WritePacket(&b, &PacketRespawn{})
	if _, err := ReadPacket(&b); err != nil {
		t.Fatal(err)
	}
	// Legacy player records initialize on login without treating missing health as death.
	legacy := &PlayerEntity{BaseEntity: BaseEntity{X: 4, Y: 70, Z: 5}}
	legacy.initVitals()
	if legacy.dead() || legacy.Vitals.Health != 20 {
		t.Fatal("legacy player initialized dead")
	}
}

func TestDestroyedSpawnGetsSafeLanding(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	for cx := -1; cx <= 1; cx++ {
		for cz := -1; cz <= 1; cz++ {
			c := lifecycleChunk(w, chunkKey{cx, cz})
			c.blocks = [chunkWidth][chunkHeight][chunkWidth]byte{}
		}
	}
	p.Vitals.Health = 0
	s.respawnPlayer(p)
	pos := rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
	if p.dead() || collides(w, pos) || !feetSupported(w, pos) {
		t.Fatal("destroyed spawn did not get a safe landing")
	}
}

func TestRecoveryAndPausedOrOfflinePlayers(t *testing.T) {
	w := lifeTestWorld(t)
	s, p := lifeTestPlayer(w)
	p.Vitals.Health = 10
	for i := 0; i < 200; i++ {
		s.tickPlayerVitals(p)
	}
	if p.Vitals.Health != 11 {
		t.Fatal("slow recovery failed")
	}
	p.Y = -100
	s.updatePlayerVitals()
	if p.Vitals.Health != 11 {
		t.Fatal("offline player took damage")
	}
	s.Paused.Store(true)
	s.Tick()
	if p.Vitals.Health != 11 {
		t.Fatal("paused server advanced damage")
	}
}

func TestSwimmingCanExitOntoBank(t *testing.T) {
	w := lifeTestWorld(t)
	c := w.getChunkIfGenerated(0, 0)
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			for y := 71; y <= 75; y++ {
				if z >= 10 {
					c.blocks[x][y][z] = blockStone
				} else if y <= 74 {
					c.blocks[x][y][z] = blockWater
				}
			}
		}
	}
	p := rl.NewVector3(8, 75.4, 9)
	var state InputState
	for i := 0; i < 100; i++ {
		p = state.StepMovement(w, p, 1.0/60, MovementControls{Forward: 1, Jump: true}, false)
	}
	if p.Z <= 10.3 || p.Y < 77.11 {
		t.Fatal("could not swim onto bank", p)
	}
}
