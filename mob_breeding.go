package main

import (
	"fmt"
	"math"
	"time"
)

func breedingFood(kind string) byte {
	switch kind {
	case "pig":
		return itemCarrot
	case "sheep":
		return itemWheat
	}
	return 0
}

func (s *Server) feedMob(p *PlayerEntity, target string) bool {
	if p == nil || p.dead() || p.SelectedSlot < 0 || p.SelectedSlot >= 9 {
		return false
	}
	slot := &p.Inventory.Slots[p.SelectedSlot]
	for _, entity := range s.World.entities {
		m, ok := entity.(*MobEntity)
		if !ok || m.UUID != target || m.Health <= 0 || m.BabyTicks > 0 || m.BreedCooldown > 0 || m.LoveTicks > 0 || slot.ID != int32(breedingFood(m.Kind)) || slot.ID == 0 || slot.Count <= 0 {
			continue
		}
		dx, dy, dz := p.X-m.X, p.Y-m.Y, p.Z-m.Z
		if dx*dx+dy*dy+dz*dz > 16 {
			return false
		}
		// Re-check the client's target against the server's last accepted look
		// direction and the block ray. A forged packet cannot feed through walls.
		pitchCos := math.Cos(float64(p.Pitch))
		lookX, lookY, lookZ := math.Sin(float64(p.Yaw))*pitchCos, math.Sin(float64(p.Pitch)), math.Cos(float64(p.Yaw))*pitchCos
		wall := s.World.rayCast(float32(p.X), float32(p.Y), float32(p.Z), float32(lookX), float32(lookY), float32(lookZ), 4)
		limit := 4.0
		if wall.hit {
			limit = float64(wall.distance)
		}
		c := mobContent.Definitions[m.Kind].Collider
		if _, aimed := segmentBoxEntry(p.X, p.Y, p.Z, lookX, lookY, lookZ, limit,
			m.X-float64(c.Width)/2, m.Y, m.Z-float64(c.Depth)/2,
			m.X+float64(c.Width)/2, m.Y+float64(c.Height), m.Z+float64(c.Depth)/2); !aimed {
			return false
		}
		m.LoveTicks = 600
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

// Pairing is bounded by the global mob cap and at most two births per tick.
// Parenthood and cooldowns are server-owned; reconnecting cannot duplicate food.
func (s *Server) tickBreeding() {
	s.World.entitiesMu.Lock()
	mobs := make([]*MobEntity, 0, 64)
	for _, entity := range s.World.entities {
		if m, ok := entity.(*MobEntity); ok {
			if m.LoveTicks > 0 {
				m.LoveTicks--
			}
			if m.BreedCooldown > 0 {
				m.BreedCooldown--
			}
			if m.BabyTicks > 0 {
				m.BabyTicks--
			}
			mobs = append(mobs, m)
		}
	}
	children := make([]*MobEntity, 0, 2)
	for i, a := range mobs {
		if len(children) >= 2 || len(mobs)+len(children) >= 64 {
			break
		}
		if a.Health <= 0 || a.LoveTicks == 0 || a.BabyTicks > 0 || a.BreedCooldown > 0 {
			continue
		}
		for _, b := range mobs[i+1:] {
			if b.Kind != a.Kind || b.Health <= 0 || b.LoveTicks == 0 || b.BabyTicks > 0 || b.BreedCooldown > 0 {
				continue
			}
			dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z
			if dx*dx+dy*dy+dz*dz > 16 {
				continue
			}
			a.LoveTicks, b.LoveTicks = 0, 0
			a.BreedCooldown, b.BreedCooldown = 6000, 6000
			child := newMob(a.Kind, fmt.Sprintf("born-%s-%d-%d", a.Kind, time.Now().UnixNano(), len(children)), (a.X+b.X)/2, (a.Y+b.Y)/2, (a.Z+b.Z)/2)
			child.BabyTicks = 24000
			children = append(children, child)
			break
		}
	}
	s.World.entitiesMu.Unlock()
	for _, child := range children {
		s.SpawnEntity(child)
	}
}
