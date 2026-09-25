package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

type PacketMobState struct {
	IDString, Kind, State string
	X, Y, Z               float64
	Yaw                   float32
	Health, Hurt, Death   int
	Baby                  bool
}

func (*PacketMobState) ID() int32 { return 0x1D }
func (p *PacketMobState) Encode(w *bytes.Buffer) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return WriteString(w, string(b))
}
func (p *PacketMobState) Decode(r *bytes.Buffer) error {
	b, err := ReadString(r)
	if err != nil {
		return err
	}
	if len(b) > 2048 {
		return fmt.Errorf("mob packet too large")
	}
	return json.Unmarshal([]byte(b), p)
}
func (m *MobEntity) snapshot() *PacketMobState {
	return &PacketMobState{IDString: m.UUID, Kind: m.Kind, State: m.State, X: m.X, Y: m.Y, Z: m.Z, Yaw: m.Yaw, Health: m.Health, Hurt: m.Hurt, Death: m.Death, Baby: m.BabyTicks > 0}
}

type PacketAttackMob struct{ Target string }

func (*PacketAttackMob) ID() int32                      { return 0x1E }
func (p *PacketAttackMob) Encode(w *bytes.Buffer) error { return WriteString(w, p.Target) }
func (p *PacketAttackMob) Decode(r *bytes.Buffer) error {
	v, e := ReadString(r)
	p.Target = v
	return e
}

func (s *Server) attackMob(p *PlayerEntity, target string) {
	if p == nil || p.dead() || p.AttackCooldown > 0 || p.SelectedSlot < 0 || p.SelectedSlot >= 9 {
		return
	}

	originX, originY, originZ := float32(p.X), float32(p.Y), float32(p.Z)
	cosPitch := float32(math.Cos(float64(p.Pitch)))
	dirX := float32(math.Sin(float64(p.Yaw))) * cosPitch
	dirY := float32(math.Sin(float64(p.Pitch)))
	dirZ := float32(math.Cos(float64(p.Yaw))) * cosPitch

	wall := s.World.rayCast(originX, originY, originZ, dirX, dirY, dirZ, 3.5)
	distance := float32(3.5)
	if wall.hit {
		distance = wall.distance
	}

	// Server-side reach validation must not depend on the rendering library.
	// This slab test mirrors the old Raylib AABB query for the mob collider.
	rayBoxDistance := func(cx, cy, cz float32, c Collider, maxDistance float32) (float32, bool) {
		tMin, tMax := float32(0), maxDistance
		testAxis := func(origin, direction, minValue, maxValue float32) bool {
			if math.Abs(float64(direction)) < 1e-7 {
				return origin >= minValue && origin <= maxValue
			}
			inv := 1 / direction
			t1 := (minValue - origin) * inv
			t2 := (maxValue - origin) * inv
			if t1 > t2 {
				t1, t2 = t2, t1
			}
			if t1 > tMin {
				tMin = t1
			}
			if t2 < tMax {
				tMax = t2
			}
			return tMin <= tMax
		}
		if !testAxis(originX, dirX, cx-c.Width/2, cx+c.Width/2) ||
			!testAxis(originY, dirY, cy, cy+c.Height) ||
			!testAxis(originZ, dirZ, cz-c.Depth/2, cz+c.Depth/2) {
			return 0, false
		}
		if tMin < 0 || tMin > maxDistance {
			return 0, false
		}
		return tMin, true
	}

	var victim *MobEntity
	for _, e := range s.World.entities {
		m, ok := e.(*MobEntity)
		if !ok || m.Health <= 0 {
			continue
		}
		d := mobContent.Definitions[m.Kind]
		hitDistance, hit := rayBoxDistance(float32(m.X), float32(m.Y), float32(m.Z), d.Collider, distance)
		if hit && hitDistance < distance {
			distance = hitDistance
			victim = m
		}
	}
	if victim == nil || victim.UUID != target {
		return
	}
	damage := 1
	slot := &p.Inventory.Slots[p.SelectedSlot]
	tool := GetItem(byte(slot.ID))
	if value := swordDamage(byte(slot.ID)); value > 0 {
		damage = value
	} else if tool.ToolType != ToolNone {
		damage = 2 + harvestTier(tool.ToolMaterial)
		if tool.ToolType == ToolAxe {
			damage++
		}
	}
	if victim.hit(damage, float32(victim.X-p.X), float32(victim.Z-p.Z)) {
		if mobContent.Definitions[victim.Kind].Hostile {
			victim.Target = p.UUID
			victim.LostTicks = 0
		}
		p.AttackCooldown = 10
		if p.GameMode == ModeSurvival {
			slot.Wear(1)
			s.SendInventory(p)
		}
	}
}
