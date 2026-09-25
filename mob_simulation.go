package main

import "math"

type MobEntity struct {
	BaseEntity
	Kind                                      string
	Health                                    int
	Velocity                                  gameVec3
	State                                     string
	Timer, Flee, Hurt, Death                  int
	RNG                                       uint32
	Dropped                                   bool
	Target                                    string
	LostTicks, AttackCooldown, Fuse, SunTicks int
	AvoidTicks                                int
	AvoidYaw                                  float32
}

func newMob(kind, id string, x, y, z float64) *MobEntity {
	d := mobContent.Definitions[kind]
	seed := uint32(2166136261)
	for _, b := range []byte(id) {
		seed = (seed ^ uint32(b)) * 16777619
	}
	return &MobEntity{BaseEntity: BaseEntity{UUID: id, Type: EntityPig, X: x, Y: y, Z: z, Dirty: true}, Kind: kind, Health: d.Health, State: "idle", Timer: d.IdleTicks, RNG: seed}
}
func (m *MobEntity) random() uint32 {
	if m.RNG == 0 {
		m.RNG = 1
	}
	m.RNG ^= m.RNG << 13
	m.RNG ^= m.RNG >> 17
	m.RNG ^= m.RNG << 5
	return m.RNG
}
func (m *MobEntity) hasBehavior(name string) bool {
	for _, b := range mobContent.Definitions[m.Kind].Behaviors {
		if b == name {
			return true
		}
	}
	return false
}
func colliderLoaded(w *World, pos gameVec3, c Collider) bool {
	for _, x := range []float32{pos.X - c.Width/2, pos.X + c.Width/2} {
		for _, z := range []float32{pos.Z - c.Depth/2, pos.Z + c.Depth/2} {
			if w.getChunkIfGenerated(divFloor(blockIndexFromCoord(x), chunkWidth), divFloor(blockIndexFromCoord(z), chunkWidth)) == nil {
				return false
			}
		}
	}
	return true
}
func (m *MobEntity) Tick(w *World) {
	if m.Kind == "" {
		legacy := newMob("pig", m.UUID, m.X, m.Y, m.Z)
		legacy.Yaw = m.Yaw
		*m = *legacy
	}
	d, ok := mobContent.Definitions[m.Kind]
	if !ok {
		return
	}
	if m.Health <= 0 {
		m.State = "dead"
		m.Death++
		m.Dirty = true
		return
	}
	if m.Hurt > 0 {
		m.Hurt--
	}
	pos := gameVec3{X: float32(m.X), Y: float32(m.Y), Z: float32(m.Z)}
	if !colliderLoaded(w, pos, d.Collider) {
		return
	}
	if m.Flee > 0 && m.hasBehavior("flee") {
		m.Flee--
		m.State = "flee"
	} else if m.State != "chase" && m.State != "aim" && m.State != "fuse" {
		m.Timer--
		if m.Timer <= 0 {
			if m.State == "idle" && m.hasBehavior("wander") {
				m.State = "walk"
				m.Timer = d.WalkTicks + int(m.random()%40)
				m.Yaw = float32(m.random()%6283) / 1000
			} else {
				m.State = "idle"
				m.Timer = d.IdleTicks + int(m.random()%40)
			}
		}
	}
	speed := float32(0)
	if m.State == "walk" {
		speed = d.Speed
	}
	if m.State == "flee" {
		speed = d.FleeSpeed
	}
	if m.State == "chase" {
		speed = d.Speed
	}
	delta := gameVec3{X: float32(math.Sin(float64(m.Yaw))) * speed * .05, Z: float32(math.Cos(float64(m.Yaw))) * speed * .05}
	delta.X += m.Velocity.X * .05
	delta.Z += m.Velocity.Z * .05
	m.Velocity.X *= .8
	m.Velocity.Z *= .8
	grounded := colliderHitsCore(w, offsetAxis(pos, 1, -.06), d.Collider)
	next := moveColliderCore(w, pos, delta, d.Collider)
	blocked := abs32(next.X-pos.X)+abs32(next.Z-pos.Z) < (abs32(delta.X)+abs32(delta.Z))*.4
	if blocked && grounded && d.Collider.StepHeight > 0 {
		raised := moveColliderCore(w, pos, gameVec3{Y: d.Collider.StepHeight + .02}, d.Collider)
		stepped := moveColliderCore(w, raised, delta, d.Collider)
		landed := moveColliderCore(w, stepped, gameVec3{Y: -d.Collider.StepHeight - .02}, d.Collider)
		if abs32(stepped.X-pos.X)+abs32(stepped.Z-pos.Z) > (abs32(delta.X)+abs32(delta.Z))*.8 && colliderHitsCore(w, offsetAxis(landed, 1, -.06), d.Collider) {
			next = landed
			blocked = false
		}
	}
	// Avoid unloaded terrain and drops over one block during intentional movement.
	if !colliderLoaded(w, next, d.Collider) || (grounded && !colliderHitsCore(w, offsetAxis(next, 1, -1.05), d.Collider)) {
		next = pos
		blocked = true
	}
	if blocked {
		if d.Hostile {
			turn := float32(1.25)
			if m.random()&1 == 0 {
				turn = -turn
			}
			m.AvoidYaw = m.Yaw + turn
			m.AvoidTicks = 16 + int(m.random()%20)
			m.Yaw = m.AvoidYaw
		} else {
			m.Yaw += 1.5
		}
		m.Timer = 20
	}
	wet := blockAtPosition(w, float64(next.X), float64(next.Y+.4), float64(next.Z)) == blockWater
	if wet {
		m.Velocity.Y += (float32(1) - m.Velocity.Y) * .25
	} else {
		m.Velocity.Y = max(m.Velocity.Y-20*.05, float32(-30))
	}
	oldY := next.Y
	next = moveColliderCore(w, next, gameVec3{Y: m.Velocity.Y * .05}, d.Collider)
	if abs32(next.Y-oldY) < abs32(m.Velocity.Y*.05)*.5 {
		m.Velocity.Y = 0
	}
	m.X, m.Y, m.Z = float64(next.X), float64(next.Y), float64(next.Z)
	m.Dirty = true
}

func (m *MobEntity) hit(damage int, awayX, awayZ float32) bool {
	if m.Health <= 0 || m.Hurt > 0 {
		return false
	}
	m.Health = max(0, m.Health-damage)
	m.Hurt = 10
	if m.hasBehavior("flee") {
		m.Flee = mobContent.Definitions[m.Kind].FleeTicks
	}
	m.Timer = 1
	if m.hasBehavior("flee") {
		m.State = "flee"
	} else {
		m.State = "chase"
	}
	length := float32(math.Sqrt(float64(awayX*awayX + awayZ*awayZ)))
	if length > 0 {
		awayX /= length
		awayZ /= length
	}
	m.Yaw = float32(math.Atan2(float64(awayX), float64(awayZ)))
	m.Velocity = gameVec3{X: awayX * 3, Y: 3, Z: awayZ * 3}
	m.Dirty = true
	if m.Health == 0 {
		m.State = "dead"
	}
	return true
}
