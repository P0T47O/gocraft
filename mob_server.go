package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func (s *Server) updateMobs() {
	var drops []*ItemEntity
	var removed []string
	var packets []*PacketMobState
	s.World.entitiesMu.Lock()
	kept := s.World.entities[:0]
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok && p.AttackCooldown > 0 {
			p.AttackCooldown--
		}
		m, ok := e.(*MobEntity)
		if !ok {
			kept = append(kept, e)
			continue
		}
		if m.Health > 0 && mobContent.Definitions[m.Kind].Hostile && !s.mobNearOnlinePlayer(m, 128) {
			removed = append(removed, m.UUID)
			continue
		}
		if m.Health <= 0 && !m.Dropped {
			d := mobContent.Definitions[m.Kind]
			m.Dropped = true
			for i, entry := range d.Drops {
				if entry.Chance > 0 && float64(m.random()%10000)/10000 >= entry.Chance {
					continue
				}
				count := entry.Min + int32(m.random()%uint32(entry.Max-entry.Min+1))
				if count == 0 {
					continue
				}
				drops = append(drops, &ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("mob-drop-%s-%d-%d", m.UUID, i, time.Now().UnixNano()), Type: EntityItem, X: m.X, Y: m.Y + .3, Z: m.Z}, ItemStack: Item{ID: int32(mobDropItems[entry.Item]), Count: count}, PickupDelay: .5, Vy: .1})
			}
		}
		if m.Death >= 12 {
			removed = append(removed, m.UUID)
			continue
		}
		kept = append(kept, m)
		packets = append(packets, m.snapshot())
	}
	s.World.entities = kept
	s.World.entitiesMu.Unlock()
	for _, d := range drops {
		s.SpawnEntity(d)
	}
	for _, p := range packets {
		s.Broadcast(p)
	}
	for _, id := range removed {
		s.Broadcast(&PacketEntityDespawn{EntityID: id})
		delete(s.LastSentPos, id)
		delete(s.LastSentMeta, id)
	}
	s.MobSpawnTicks++
	if s.MobSpawnTicks%200 == 0 {
		s.spawnNearbyMobs()
	}
}
func (s *Server) spawnNearbyMobs() {
	counts := map[string]int{}
	total := 0
	for _, e := range s.World.entities {
		if m, ok := e.(*MobEntity); ok {
			counts[m.Kind]++
			total++
		}
	}
	if total >= 64 {
		return
	}
	names := []string{}
	for name, d := range mobContent.Definitions {
		if d.SpawnLimit > counts[name] {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	kind := names[(s.MobSpawnTicks/200)%len(names)]
	def := mobContent.Definitions[kind]
	for _, e := range s.World.entities {
		p, ok := e.(*PlayerEntity)
		if !ok || p.dead() {
			continue
		}
		s.ClientsMu.RLock()
		online := s.Clients[p.UUID] != nil
		s.ClientsMu.RUnlock()
		if !online {
			continue
		}
		// Try several directions and radii. A single unsuitable column should
		// not waste the entire ten-second spawn interval.
		for attempt := 0; attempt < 8; attempt++ {
			phase := float64(s.MobSpawnTicks/200)*2.399 + float64(attempt)*2.399
			radius := def.SpawnRadius * (1 - 0.125*float64(attempt%3))
			x, z := int(math.Floor(p.X+math.Sin(phase)*radius)), int(math.Floor(p.Z+math.Cos(phase)*radius))
			if s.World.getChunkIfGenerated(divFloor(x, 16), divFloor(z, 16)) == nil {
				continue
			}
			if spawnMobOnColumn(s, kind, def, x, z) {
				return
			}
		}
	}
}

func spawnMobOnColumn(s *Server, kind string, def MobDefinition, x, z int) bool {
	for y := chunkHeight - 2; y > 0; y-- {
		if s.World.BlockAt(x, y, z) != blockGrass {
			continue
		}
		pos := gameVec3{X: float32(x), Y: float32(y) + .501, Z: float32(z)}
		shape := def.Collider
		if !colliderLoaded(s.World, pos, shape) || colliderHitsCore(s.World, pos, shape) {
			return false
		}
		if !canSpawnMobAt(s.World, def, x, y+1, z) {
			return false
		}
		near := false
		for _, other := range s.World.entities {
			ox, oy, oz := other.GetPosition()
			if (ox-float64(pos.X))*(ox-float64(pos.X))+(oy-float64(pos.Y))*(oy-float64(pos.Y))+(oz-float64(pos.Z))*(oz-float64(pos.Z)) < 9 {
				near = true
				break
			}
		}
		if !near {
			s.SpawnEntity(newMob(kind, fmt.Sprintf("%s-%d", kind, time.Now().UnixNano()), float64(pos.X), float64(pos.Y), float64(pos.Z)))
			return true
		}
		return false
	}
	return false
}

func canSpawnMobAt(w *World, def MobDefinition, x, y, z int) bool {
	if !def.Hostile {
		return true
	}
	light := max(float32(w.LightBlockAt(x, y, z)), float32(w.LightSkyAt(x, y, z))*worldDaylight(w.TimeTicks).Brightness)
	return light <= 7
}

func (s *Server) mobNearOnlinePlayer(m *MobEntity, radius float64) bool {
	for _, e := range s.World.entities {
		p, ok := e.(*PlayerEntity)
		if !ok || p.dead() {
			continue
		}
		s.ClientsMu.RLock()
		online := s.Clients[p.UUID] != nil
		s.ClientsMu.RUnlock()
		if !online {
			continue
		}
		dx, dz := p.X-m.X, p.Z-m.Z
		if dx*dx+dz*dz < radius*radius {
			return true
		}
	}
	return false
}
