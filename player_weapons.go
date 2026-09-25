package main

import (
	"fmt"
	"math"
	"time"
)

func swordDamage(id byte) int {
	switch id {
	case itemWoodSword, itemGoldSword:
		return 4
	case itemStoneSword:
		return 5
	case itemIronSword:
		return 6
	case itemDiamondSword:
		return 7
	}
	return 0
}

// The action carries no item ID or aim vector: the server uses the selected
// slot and last accepted player rotation, and consumes a real arrow.
func (s *Server) shootSelectedBow(p *PlayerEntity) bool {
	if p == nil || p.dead() || p.AttackCooldown > 0 || p.SelectedSlot < 0 || p.SelectedSlot >= 9 {
		return false
	}
	slot := &p.Inventory.Slots[p.SelectedSlot]
	if slot.ID != int32(itemBow) || slot.Count != 1 {
		return false
	}
	if p.GameMode == ModeSurvival && !p.Inventory.Consume(int32(itemArrow), 1) {
		return false
	}
	cosPitch := math.Cos(float64(p.Pitch))
	dx, dy, dz := math.Sin(float64(p.Yaw))*cosPitch, math.Sin(float64(p.Pitch)), math.Cos(float64(p.Yaw))*cosPitch
	const speed = 1.7
	s.SpawnEntity(&ArrowEntity{BaseEntity: BaseEntity{UUID: fmt.Sprintf("player-arrow-%s-%d", p.UUID, time.Now().UnixNano()), Type: EntityArrow, X: p.X + dx*.45, Y: p.Y + dy*.45, Z: p.Z + dz*.45, Dirty: true}, Owner: p.UUID, Vx: dx * speed, Vy: dy * speed, Vz: dz * speed, Damage: 4})
	p.AttackCooldown = 10
	if p.GameMode == ModeSurvival {
		slot.Wear(1)
		s.SendInventory(p)
	}
	return true
}
