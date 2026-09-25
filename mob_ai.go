package main

import (
	"math"
	"strconv"
)

type mobAttack struct {
	player *PlayerEntity
	amount int
	cause  string
}

type mobShot struct {
	mob    *MobEntity
	target *PlayerEntity
	damage int
}

// All decisions are server-owned. World.TickEntities applies the resulting
// state/velocity on the same tick; clients only receive mob snapshots.
func (s *Server) updateMobAI() {
	s.World.entitiesMu.Lock()
	players := make([]*PlayerEntity, 0, 4)
	for _, e := range s.World.entities {
		if p, ok := e.(*PlayerEntity); ok && !p.dead() {
			s.ClientsMu.RLock()
			online := s.Clients[p.UUID] != nil
			s.ClientsMu.RUnlock()
			if online {
				players = append(players, p)
			}
		}
	}
	var attacks []mobAttack
	var shots []mobShot
	var explosions [][3]float64
	day := worldDaylight(s.World.TimeTicks).Brightness > .8
	for _, e := range s.World.entities {
		m, ok := e.(*MobEntity)
		if !ok || m.Health <= 0 {
			continue
		}
		d := mobContent.Definitions[m.Kind]
		if !d.Hostile {
			continue
		}
		if m.AttackCooldown > 0 {
			m.AttackCooldown--
		}
		if day && (m.Kind == "zombie" || m.Kind == "skeleton") && s.World.LightSkyAt(int(math.Floor(m.X)), int(math.Floor(m.Y+float64(d.Collider.Height))), int(math.Floor(m.Z))) >= 14 {
			m.SunTicks++
			if m.SunTicks >= 20 {
				m.SunTicks = 0
				m.Health = max(0, m.Health-1)
				m.Hurt = 8
				m.Dirty = true
			}
		} else {
			m.SunTicks = 0
		}
		if m.Health <= 0 {
			continue
		}
		target, dist, visible := nearestMobTarget(s.World, m, d, players)
		if target == nil {
			m.LostTicks++
			m.Fuse = max(0, m.Fuse-2)
			if m.State == "fuse" || m.State == "aim" {
				m.State = "chase"
			}
			if m.LostTicks > 60 {
				m.Target = ""
				m.Fuse = 0
				if m.State == "chase" || m.State == "aim" || m.State == "fuse" {
					m.State, m.Timer = "idle", d.IdleTicks
				}
			}
			if m.AvoidTicks > 0 {
				m.Yaw = m.AvoidYaw
				m.AvoidTicks--
			}
			continue
		}
		m.Target = target.UUID
		if visible {
			m.LostTicks = 0
		} else {
			m.LostTicks++
		}
		if m.LostTicks > 100 {
			m.Target, m.State, m.Timer = "", "idle", d.IdleTicks
			continue
		}
		if m.AvoidTicks > 0 && (!visible || dist > d.AttackRange) {
			m.Yaw = m.AvoidYaw
			m.AvoidTicks--
		} else {
			m.Yaw = float32(math.Atan2(target.X-m.X, target.Z-m.Z))
		}
		m.State = "chase"
		if m.hasBehavior("ranged") {
			// Keep a little distance while firing. The ray check below makes the
			// shot authoritative and prevents damage through terrain.
			if visible && dist <= d.AttackRange {
				m.State = "aim"
				if m.AttackCooldown == 0 {
					shots = append(shots, mobShot{m, target, d.Damage})
					m.AttackCooldown = d.AttackTicks
				}
			}
		} else if m.hasBehavior("fuse") {
			if visible && dist <= d.AttackRange {
				m.State = "fuse"
				m.Fuse++
				if m.Fuse >= d.AttackTicks {
					explosions = append(explosions, [3]float64{m.X, m.Y + .8, m.Z})
					m.Health, m.Dropped, m.State = 0, true, "dead"
				}
			} else {
				m.Fuse = max(0, m.Fuse-2)
			}
		} else {
			if visible && m.hasBehavior("pounce") && dist > d.AttackRange && dist < 6 && m.AttackCooldown == 0 {
				m.Velocity.X += float32((target.X-m.X)/dist) * 3
				m.Velocity.Z += float32((target.Z-m.Z)/dist) * 3
				m.Velocity.Y = 5
				m.AttackCooldown = 45
			}
			if visible && dist <= d.AttackRange && m.AttackCooldown == 0 {
				attacks = append(attacks, mobAttack{target, d.Damage, "Slain by " + m.Kind})
				m.AttackCooldown = d.AttackTicks
			}
		}
		m.Dirty = true
	}
	s.World.entitiesMu.Unlock()
	for _, center := range explosions {
		s.explode(center[0], center[1], center[2], 3.5, mobContent.Definitions["creeper"].Damage, "Creeper explosion")
	}
	for _, attack := range attacks {
		s.hurtPlayer(attack.player, attack.amount, attack.cause)
	}
	for _, shot := range shots {
		x, y, z := shot.mob.X, shot.mob.Y+1.45, shot.mob.Z
		dx, dy, dz := shot.target.X-x, shot.target.Y-playerEyeY*.45-y, shot.target.Z-z
		distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if distance < .01 {
			continue
		}
		const speed = .75
		s.SpawnEntity(&ArrowEntity{BaseEntity: BaseEntity{UUID: "arrow-" + shot.mob.UUID + "-" + shot.target.UUID + "-" + strconv.FormatUint(uint64(shot.mob.random()), 10), Type: EntityArrow, X: x, Y: y, Z: z, Dirty: true}, Owner: shot.mob.UUID, Vx: dx / distance * speed, Vy: dy/distance*speed + .08, Vz: dz / distance * speed, Damage: shot.damage})
	}
}

func nearestMobTarget(w *World, m *MobEntity, d MobDefinition, players []*PlayerEntity) (*PlayerEntity, float64, bool) {
	var nearest *PlayerEntity
	best := d.Detection
	seen := false
	for _, p := range players {
		if p.GameMode != ModeSurvival || math.Abs(p.Y-m.Y) > 5 {
			continue
		}
		dist := math.Hypot(p.X-m.X, p.Z-m.Z)
		visible := mobSeesPlayer(w, m, d, p)
		if !visible && (m.Target != p.UUID || dist > d.Detection*1.5) {
			continue
		}
		if dist >= best && !(m.Target == p.UUID && nearest == nil) {
			continue
		}
		nearest, best, seen = p, dist, visible
	}
	return nearest, best, seen
}

func mobSeesPlayer(w *World, m *MobEntity, d MobDefinition, p *PlayerEntity) bool {
	ox, oy, oz := float32(m.X), float32(m.Y)+d.Collider.Height*.78, float32(m.Z)
	dx, dy, dz := float32(p.X)-ox, float32(p.Y)-oy, float32(p.Z)-oz
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
	if dist < .01 {
		return true
	}
	hit := w.rayCast(ox, oy, oz, dx/dist, dy/dist, dz/dist, dist)
	return !hit.hit || hit.distance >= dist-.1
}
