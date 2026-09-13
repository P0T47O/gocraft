package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type PacketMobState struct {
	IDString, Kind, State string
	X, Y, Z               float64
	Yaw                   float32
	Health, Hurt, Death   int
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
	return &PacketMobState{IDString: m.UUID, Kind: m.Kind, State: m.State, X: m.X, Y: m.Y, Z: m.Z, Yaw: m.Yaw, Health: m.Health, Hurt: m.Hurt, Death: m.Death}
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
	ray := rl.Ray{Position: rl.NewVector3(float32(p.X), float32(p.Y), float32(p.Z)), Direction: viewDirection(p.Yaw, p.Pitch)}
	wall := s.World.HitTest(ray, 3.5)
	distance := float32(3.5)
	if wall.hit {
		distance = wall.distance
	}
	var victim *MobEntity
	for _, e := range s.World.entities {
		m, ok := e.(*MobEntity)
		if !ok || m.Health <= 0 {
			continue
		}
		d := mobContent.Definitions[m.Kind]
		hit := rl.GetRayCollisionBox(ray, mobBox(rl.NewVector3(float32(m.X), float32(m.Y), float32(m.Z)), d.Collider))
		if hit.Hit && hit.Distance < distance {
			distance = hit.Distance
			victim = m
		}
	}
	if victim == nil || victim.UUID != target {
		return
	}
	damage := 1
	slot := &p.Inventory.Slots[p.SelectedSlot]
	tool := GetItem(byte(slot.ID))
	if tool.ToolType != ToolNone {
		damage = 2 + harvestTier(tool.ToolMaterial)
		if tool.ToolType == ToolAxe {
			damage++
		}
	}
	away := rl.Vector3Normalize(rl.NewVector3(float32(victim.X-p.X), 0, float32(victim.Z-p.Z)))
	if victim.hit(damage, away) {
		p.AttackCooldown = 10
		if p.GameMode == ModeSurvival {
			slot.Wear(1)
			s.SendInventory(p)
		}
	}
}
