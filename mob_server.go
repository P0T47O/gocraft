package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math"
	"sort"
	"time"
)

func viewDirection(yaw, pitch float32) rl.Vector3 {
	return rl.NewVector3(float32(math.Sin(float64(yaw))*math.Cos(float64(pitch))), float32(math.Sin(float64(pitch))), float32(math.Cos(float64(yaw))*math.Cos(float64(pitch))))
}
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
		if m.Health <= 0 && !m.Dropped {
			d := mobContent.Definitions[m.Kind]
			count := d.DropMin + int32(m.random()%uint32(d.DropMax-d.DropMin+1))
			m.Dropped = true
			drops = append(drops, &ItemEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("mob-drop-%s-%d", m.UUID, time.Now().UnixNano()), Type: EntityItem, X: m.X, Y: m.Y + .3, Z: m.Z}, ItemStack: Item{ID: int32(itemRawPork), Count: count}, PickupDelay: .5, Vy: .1})
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
		phase := float64(s.MobSpawnTicks/200) * 2.399
		x, z := int(math.Floor(p.X+math.Sin(phase)*def.SpawnRadius)), int(math.Floor(p.Z+math.Cos(phase)*def.SpawnRadius))
		if s.World.getChunkIfGenerated(divFloor(x, 16), divFloor(z, 16)) == nil {
			continue
		}
		for y := chunkHeight - 2; y > 0; y-- {
			if s.World.BlockAt(x, y, z) != blockGrass {
				continue
			}
			pos := rl.NewVector3(float32(x), float32(y)+.501, float32(z))
			shape := def.Collider
			if !colliderLoaded(s.World, pos, shape) || colliderHits(s.World, pos, shape) {
				break
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
			}
			return
		}
	}
}
