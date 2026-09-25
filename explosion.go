package main

import (
	"fmt"
	"math"
	"time"
)

const tntFuseTicks = 80

type PrimedTNT struct {
	BaseEntity
	Fuse int
	Dead bool
}

func (t *PrimedTNT) Tick(_ *World) {
	if t.Fuse > 0 {
		t.Fuse--
	}
	if t.Fuse == 0 {
		t.Dead = true
	}
}

func (s *Server) igniteTNT(x, y, z, fuse int) bool {
	if s.World.getChunkIfGenerated(divFloor(x, chunkWidth), divFloor(z, chunkWidth)) == nil || s.World.BlockAt(x, y, z) != blockTNT {
		return false
	}
	s.World.SetBlockAt(x, y, z, blockAir)
	s.Broadcast(&PacketBlockChange{X: int32(x), Y: int32(y), Z: int32(z), BlockID: blockAir})
	s.SpawnEntity(&PrimedTNT{BaseEntity: BaseEntity{UUID: fmt.Sprintf("tnt-%d", time.Now().UnixNano()), Type: EntityPrimedTNT, X: float64(x) + .5, Y: float64(y) + .5, Z: float64(z) + .5}, Fuse: fuse})
	return true
}

// explode is called only by the authoritative server loop, outside entitiesMu.
func (s *Server) explode(x, y, z, radius float64, damage int, cause string) {
	if radius <= 0 || radius > 6 {
		return
	}
	// Evaluate cover before changing the terrain. Otherwise a blast would
	// always hurt targets through walls it just removed.
	var hits []mobAttack
	s.World.entitiesMu.Lock()
	for _, e := range s.World.entities {
		ex, ey, ez := e.GetPosition()
		d := math.Sqrt((ex-x)*(ex-x) + (ey-y)*(ey-y) + (ez-z)*(ez-z))
		if d >= radius {
			continue
		}
		exposure := explosionExposure(s.World, x, y, z, e)
		if exposure == 0 {
			continue
		}
		amount := max(1, int(math.Round(float64(damage)*(1-d/radius)*exposure)))
		switch target := e.(type) {
		case *PlayerEntity:
			if !target.dead() && target.GameMode == ModeSurvival {
				hits = append(hits, mobAttack{target, amount, cause})
			}
		case *MobEntity:
			if target.Health > 0 {
				target.Health = max(0, target.Health-amount)
				target.Hurt = 8
				target.Dirty = true
			}
		}
	}
	s.World.entitiesMu.Unlock()
	removed := make([]BlockPos, 0, 256)
	chain := make([]BlockPos, 0, 8)
	minX, maxX := int(math.Floor(x-radius)), int(math.Floor(x+radius))
	minY, maxY := max(0, int(math.Floor(y-radius))), min(chunkHeight-1, int(math.Floor(y+radius)))
	minZ, maxZ := int(math.Floor(z-radius)), int(math.Floor(z+radius))
	for bx := minX; bx <= maxX; bx++ {
		for bz := minZ; bz <= maxZ; bz++ {
			if s.World.getChunkIfGenerated(divFloor(bx, chunkWidth), divFloor(bz, chunkWidth)) == nil {
				continue
			}
			for by := minY; by <= maxY; by++ {
				block := s.World.BlockAt(bx, by, bz)
				if block == blockAir || block == blockBedrock || block == blockObsidian || block == blockWater || block == blockLava {
					continue
				}
				dx, dy, dz := float64(bx)+.5-x, float64(by)+.5-y, float64(bz)+.5-z
				if dx*dx+dy*dy+dz*dz > radius*radius {
					continue
				}
				// Dense stone and ore survive near the rim; loose blocks do not.
				resistance := 0.0
				if GetBlock(block).IsOpaque && block != blockTNT {
					resistance = .8
				}
				if math.Sqrt(dx*dx+dy*dy+dz*dz)+resistance > radius {
					continue
				}
				pos := BlockPos{int32(bx), int32(by), int32(bz)}
				if block == blockTNT {
					chain = append(chain, pos)
				}
				if containerSize(block) > 0 {
					s.breakContainer(pos)
				}
				s.World.SetBlockAt(bx, by, bz, blockAir)
				removed = append(removed, pos)
			}
		}
	}
	s.Broadcast(&PacketExplosion{X: x, Y: y, Z: z, Radius: float32(radius), Removed: removed})
	for _, hit := range hits {
		s.hurtPlayer(hit.player, hit.amount, hit.cause)
	}
	for _, pos := range chain {
		// Already removed above; chain primed TNT without a second block update.
		s.SpawnEntity(&PrimedTNT{BaseEntity: BaseEntity{UUID: fmt.Sprintf("tnt-chain-%d", time.Now().UnixNano()), Type: EntityPrimedTNT, X: float64(pos.X) + .5, Y: float64(pos.Y) + .5, Z: float64(pos.Z) + .5}, Fuse: 8})
	}
}

func explosionExposure(w *World, x, y, z float64, e Entity) float64 {
	ex, ey, ez := e.GetPosition()
	if p, ok := e.(*PlayerEntity); ok {
		ey = p.Y - playerEyeY + .9
	}
	visible := 0
	for _, offset := range []float64{-.55, 0, .55} {
		dx, dy, dz := ex-x, ey+offset-y, ez-z
		distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if distance < .01 {
			visible++
			continue
		}
		hit := w.rayCast(float32(x), float32(y), float32(z), float32(dx/distance), float32(dy/distance), float32(dz/distance), float32(distance))
		if !hit.hit || hit.distance >= float32(distance)-.1 {
			visible++
		}
	}
	return float64(visible) / 3
}
