package main

import (
	"bytes"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"time"
)

const (
	maxHealth = 20
	maxAir    = 300
)

type PlayerVitals struct {
	Health                 int
	Air                    int
	FireTicks              int
	Cause                  string
	SpawnX, SpawnY, SpawnZ float64
	FallDistance           float64
	cooldown               int
	dryTicks               int
	lastY                  float64
	grace                  int
}

func (p *PlayerEntity) initVitals() {
	if p.Vitals == nil {
		p.Vitals = &PlayerVitals{Health: maxHealth, Air: maxAir, SpawnX: p.X, SpawnY: p.Y, SpawnZ: p.Z}
	}
	p.Vitals.lastY = p.Y
}
func (p *PlayerEntity) dead() bool { return p.Vitals != nil && p.Vitals.Health <= 0 }

type PacketVitals struct {
	Health, Air, Fire int32
	Cause             string
}

func (*PacketVitals) ID() int32 { return IDVitals }
func (p *PacketVitals) Encode(b *bytes.Buffer) error {
	WriteVarInt(b, p.Health)
	WriteVarInt(b, p.Air)
	WriteVarInt(b, p.Fire)
	return WriteString(b, p.Cause)
}
func (p *PacketVitals) Decode(b *bytes.Buffer) error {
	var err error
	if p.Health, err = ReadVarInt(b); err != nil {
		return err
	}
	if p.Air, err = ReadVarInt(b); err != nil {
		return err
	}
	if p.Fire, err = ReadVarInt(b); err != nil {
		return err
	}
	p.Cause, err = ReadString(b)
	return err
}

type PacketRespawn struct{}

func (*PacketRespawn) ID() int32                  { return IDRespawn }
func (*PacketRespawn) Encode(*bytes.Buffer) error { return nil }
func (*PacketRespawn) Decode(*bytes.Buffer) error { return nil }

func (s *Server) sendVitals(p *PlayerEntity) {
	if p.Vitals == nil {
		return
	}
	v := p.Vitals
	s.BroadcastTo(p.UUID, &PacketVitals{Health: int32(v.Health), Air: int32(v.Air), Fire: int32(v.FireTicks), Cause: v.Cause})
}
func (s *Server) hurtPlayer(p *PlayerEntity, amount int, cause string) {
	v := p.Vitals
	if v == nil || p.dead() || p.GameMode != ModeSurvival || v.grace > 0 || v.cooldown > 0 {
		return
	}
	v.Health = max(0, v.Health-amount)
	v.Cause = cause
	v.cooldown = 20
	v.dryTicks = 0
	if p.dead() {
		items := append([]Item(nil), p.Inventory.Slots[:]...)
		items = append(items, p.CursorItem)
		p.Inventory = Inventory{}
		p.CursorItem = Item{}
		for i, item := range items {
			if item.ID != 0 && item.Count > 0 {
				s.SpawnEntity(&ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("death-%s-%d-%d", p.UUID, time.Now().UnixNano(), i), Type: EntityItem, X: p.X, Y: p.Y - 1, Z: p.Z}, ItemStack: item, PickupDelay: 1.5, Vy: 0.1})
			}
		}
		s.SendInventory(p)
		s.BroadcastTo(p.UUID, &PacketInventoryUpdate{SlotID: -1})
	}
	s.sendVitals(p)
}

// Called on the server's 20 Hz loop; only connected players accrue damage.
func (s *Server) tickPlayerVitals(p *PlayerEntity) {
	if p.Vitals == nil {
		p.initVitals()
	}
	v := p.Vitals
	if p.dead() {
		return
	}
	if v.cooldown > 0 {
		v.cooldown--
	}
	if v.grace > 0 {
		v.grace--
	}
	pos := rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z))
	water := touchesLiquid(s.World, pos, blockWater)
	if p.GameMode == ModeCreative {
		v.Air = maxAir
		v.FireTicks = 0
		v.FallDistance = 0
		v.lastY = p.Y
		return
	}
	before := PacketVitals{Health: int32(v.Health), Air: int32(v.Air), Fire: int32(v.FireTicks), Cause: v.Cause}
	head := blockAtPosition(s.World, p.X, p.Y, p.Z)
	if head == blockWater {
		v.Air = max(0, v.Air-1)
		if v.Air == 0 {
			s.hurtPlayer(p, 2, "Drowned")
		}
	} else {
		v.Air = min(maxAir, v.Air+10)
	}
	if water {
		v.FireTicks = 0
		v.FallDistance = 0
	} else if touchesLiquid(s.World, pos, blockLava) {
		v.FireTicks = 160
		s.hurtPlayer(p, 4, "Burned in lava")
	} else if v.FireTicks > 0 {
		v.FireTicks--
		if v.FireTicks%20 == 0 {
			s.hurtPlayer(p, 1, "Burned")
		}
	}
	if GetBlock(head).IsOpaque && GetBlock(head).IsCollidable {
		s.hurtPlayer(p, 1, "Suffocated")
	}
	if p.Y < -32 {
		v.grace = 0
		s.hurtPlayer(p, maxHealth, "Fell into the void")
	}
	if !water {
		if p.Y < v.lastY {
			v.FallDistance += v.lastY - p.Y
		}
		if feetSupported(s.World, pos) {
			if v.FallDistance > 3 {
				s.hurtPlayer(p, int(math.Ceil(v.FallDistance-3)), "Fell from a height")
			}
			v.FallDistance = 0
		}
	}
	v.lastY = p.Y
	// Temporary slow recovery until the food/hunger loop is added.
	if !p.dead() && head != blockWater && v.FireTicks == 0 && v.cooldown == 0 {
		v.dryTicks++
		if v.dryTicks >= 200 {
			v.Health = min(maxHealth, v.Health+1)
			v.dryTicks = 0
		}
	} else {
		v.dryTicks = 0
	}
	after := PacketVitals{Health: int32(v.Health), Air: int32(v.Air), Fire: int32(v.FireTicks), Cause: v.Cause}
	if before != after {
		s.sendVitals(p)
	}
}

func (s *Server) updatePlayerVitals() {
	s.World.entitiesMu.RLock()
	players := []*PlayerEntity{}
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok {
			players = append(players, p)
		}
	}
	s.World.entitiesMu.RUnlock()
	for _, p := range players {
		s.ClientsMu.RLock()
		connected := s.Clients[p.UUID] != nil
		s.ClientsMu.RUnlock()
		if connected {
			s.tickPlayerVitals(p)
		}
	}
}

// Search actual loaded blocks near the saved anchor, never teleport into a
// procedural approximation of an edited or still-loading chunk.
func (s *Server) safeRespawn(v *PlayerVitals) (float64, float64, float64, bool) {
	ax, az := int(math.Floor(v.SpawnX+0.5)), int(math.Floor(v.SpawnZ+0.5))
	pending := false
	for radius := 0; radius <= 16; radius++ {
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if radius > 0 && absInt(dx) != radius && absInt(dz) != radius {
					continue
				}
				x, z := ax+dx, az+dz
				if s.World.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth)) == nil {
					pending = true
					s.World.requestChunk(divFloor(x, chunkWidth), divFloor(z, chunkWidth))
					continue
				}
				for y := chunkHeight - 3; y >= 0; y-- {
					id := s.World.BlockAt(x, y, z)
					if GetBlock(id).IsCollidable && id != blockCactus && id != blockIce && s.World.BlockAt(x, y+1, z) == blockAir && s.World.BlockAt(x, y+2, z) == blockAir {
						return float64(x), float64(y) + 0.5 + playerEyeY + 0.005, float64(z), true
					}
				}
			}
		}
	}
	if pending {
		return 0, 0, 0, false
	}
	// If the entire anchor area has become water/lava/void, provide a small
	// last-resort landing rather than leaving the death screen stuck forever.
	top := 62
	for y := chunkHeight - 1; y >= 0; y-- {
		if s.World.BlockAt(ax, y, az) != blockAir {
			top = y
			break
		}
	}
	y := min(top+1, chunkHeight-3)
	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			s.World.SetBlockAt(ax+dx, y, az+dz, blockCobblestone)
			s.World.SetBlockAt(ax+dx, y+1, az+dz, blockAir)
			s.World.SetBlockAt(ax+dx, y+2, az+dz, blockAir)
		}
	}
	return float64(ax), float64(y) + 0.5 + playerEyeY + 0.005, float64(az), true
}
func (s *Server) respawnPlayer(p *PlayerEntity) {
	if !p.dead() {
		return
	}
	x, y, z, ok := s.safeRespawn(p.Vitals)
	if !ok {
		s.sendVitals(p)
		return
	}
	old := p.Vitals
	p.Vitals = &PlayerVitals{Health: maxHealth, Air: maxAir, SpawnX: old.SpawnX, SpawnY: old.SpawnY, SpawnZ: old.SpawnZ, lastY: y, grace: 60}
	p.SetPosition(x, y, z)
	s.BroadcastTo(p.UUID, &PacketSpawnPoint{X: x, Y: y, Z: z})
	s.sendVitals(p)
	s.ClientsMu.RLock()
	c := s.Clients[p.UUID]
	s.ClientsMu.RUnlock()
	if c != nil {
		c.LastChunkX = divFloor(int(x), chunkWidth)
		c.LastChunkZ = divFloor(int(z), chunkWidth)
		s.SendChunksAround(p.UUID, c.LastChunkX, c.LastChunkZ, 16)
	}
}
